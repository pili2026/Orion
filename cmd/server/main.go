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
	"github.com/hill/orion/internal/repository"
	"github.com/hill/orion/internal/service"
)

func main() {
	// ── 1. Logger ────────────────────────────────────────────────────────────
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// ── 2. Config ────────────────────────────────────────────────────────────
	config.Init()

	// ── 3. Database ──────────────────────────────────────────────────────────
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

	// ── 4. MQTT ──────────────────────────────────────────────────────────────
	mqttClient, err := service.InitMQTT()
	if err != nil {
		slog.Error("Failed to initialise MQTT client", slog.Any("error", err))
		os.Exit(1)
	}
	defer mqttClient.Disconnect(500)

	// ── 5. Wire dependencies ─────────────────────────────────────────────────
	//
	//   repository  ←  dbManager.GormDB
	//   dynsec      ←  mqttClient
	//   service     ←  repository + dynsec
	//   handler     ←  dbManager + mqttClient + service
	//
	gatewayRepo := repository.NewGatewayRepository(dbManager.GormDB)
	dynsec := service.NewDynsecService(mqttClient)
	gatewaySvc := service.NewGatewayService(gatewayRepo, dynsec)

	telemetryRepo := repository.NewTelemetryRepository(dbManager.PgxPool)
	telemetrySvc := service.NewTelemetryService(telemetryRepo, dbManager.GormDB)

	siteRepo := repository.NewSiteRepository(dbManager.GormDB)
	siteSvc := service.NewSiteService(siteRepo)

	zoneRepo := repository.NewZoneRepository(dbManager.GormDB)
	zoneSvc := service.NewZoneService(zoneRepo, siteRepo)

	h := handler.NewHandler(dbManager, mqttClient, gatewaySvc, telemetrySvc, siteSvc, zoneSvc)

	// Re-subscribe on every (re)connect so subscriptions survive broker restarts.
	h.SetupMQTTSubscribers()

	// ── 6. HTTP server ───────────────────────────────────────────────────────
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

	// ── 7. Graceful shutdown ─────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("Shutdown signal received", slog.String("signal", sig.String()))

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server forced to shut down", slog.Any("error", err))
	}

	slog.Info("Orion shut down cleanly")
}
