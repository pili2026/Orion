package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hill/orion/internal/config"
	"github.com/hill/orion/internal/database"
	"github.com/hill/orion/internal/handler"
	"github.com/hill/orion/internal/middleware"
	"github.com/hill/orion/internal/repository"
	"github.com/hill/orion/internal/service"
)

func main() {
	// ── 1. Logger ────────────────────────────────────────────────────────────
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// ── 2. Config ────────────────────────────────────────────────────────────
	config.Init()

	// ── 3. Root context (cancelled on SIGTERM / SIGINT) ───────────────────────
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	// ── 4. Database ──────────────────────────────────────────────────────────
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer dbCancel()

	dbManager, err := database.InitDB(dbCtx, database.Config{
		Host:     os.Getenv("DB_HOST"),
		Port:     os.Getenv("DB_PORT"),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		DBName:   os.Getenv("DB_NAME"),
	})
	if err != nil {
		slog.Error("Failed to initialise database", slog.Any("error", err))
		os.Exit(1)
	}
	defer dbManager.Close()

	// ── 5. Wire dependencies (before MQTT so h is ready for the callback) ────
	//
	//   repository   ←  dbManager.GormDB / PgxPool
	//   dynsec       ←  mqttClient  (wired after MQTT init below)
	//   service      ←  repository
	//   ingestSvc    ←  dbManager.GormDB + telemetryRepo
	//   handler      ←  dbManager + mqttClient + services
	//
	telemetryRepo := repository.NewTelemetryRepository(dbManager.PgxPool)
	telemetrySvc := service.NewTelemetryService(telemetryRepo, dbManager.GormDB)

	siteRepo := repository.NewSiteRepository(dbManager.GormDB)
	siteSvc := service.NewSiteService(siteRepo)

	zoneRepo := repository.NewZoneRepository(dbManager.GormDB)

	ingestSvc := service.NewMQTTIngestService(dbManager.GormDB, telemetryRepo, service.NewInMemoryDLQ(service.DLQBufferSize))
	ingestSvc.Start(rootCtx)

	// ── 6. MQTT ──────────────────────────────────────────────────────────────
	// h is declared here so the onConnect closure can reference it.
	// It will be assigned after NewHandler() below.
	var h *handler.Handler

	mqttClient, err := service.InitMQTT(func() {
		// Called on every (re)connect — restores subscriptions after broker restart.
		if h != nil {
			h.SetupMQTTSubscribers()
		}
	})
	if err != nil {
		slog.Error("Failed to initialise MQTT client", slog.Any("error", err))
		os.Exit(1)
	}
	defer mqttClient.Disconnect(500)

	// ── 7. Finish wiring (services that depend on mqttClient) ────────────────
	gatewayRepo := repository.NewGatewayRepository(dbManager.GormDB)
	dynsec := service.NewDynsecService(mqttClient)
	pkiSvc := service.NewPKIService(dbManager.GormDB)
	gatewaySvc := service.NewGatewayService(gatewayRepo, zoneRepo, dynsec, pkiSvc)

	zoneSvc := service.NewZoneService(zoneRepo, siteRepo, gatewayRepo)

	deviceRepo := repository.NewDeviceRepository(dbManager.GormDB)
	deviceSvc := service.NewDeviceService(deviceRepo, gatewayRepo, zoneRepo)

	authn := middleware.NewStubAuthenticator()
	h = handler.NewHandler(dbManager, mqttClient, gatewaySvc, telemetrySvc, siteSvc, zoneSvc, ingestSvc, deviceSvc, authn)

	// SetupMQTTSubscribers is now driven by the OnConnect callback above.
	// Calling it once here guards against a race where the initial connect
	// fires before h is assigned (extremely unlikely but safe to have).
	h.SetupMQTTSubscribers()

	// ── 8. HTTP server ───────────────────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      h.SetupRouter(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("HTTP server starting", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP server error", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	// ── 9. Graceful shutdown ─────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("Shutdown signal received", slog.String("signal", sig.String()))

	rootCancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server forced to shut down", slog.Any("error", err))
	}

	ingestSvc.Stop()

	slog.Info("Orion shut down cleanly")
}
