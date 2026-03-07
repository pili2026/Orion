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
	"github.com/hill/orion/internal/service"
)

func main() {
	// ── 1. Structured logger ─────────────────────────────────────────────────
	// JSON format is machine-readable in production; swap to TextHandler locally.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// ── 2. Configuration ─────────────────────────────────────────────────────
	config.Init()

	// ── 3. Database ──────────────────────────────────────────────────────────
	// Use a short timeout for the initial connection attempt so a
	// misconfigured DB doesn't block startup indefinitely.
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

	// ── 4. MQTT client ───────────────────────────────────────────────────────
	mqttClient, err := service.InitMQTT()
	if err != nil {
		slog.Error("Failed to initialise MQTT client", slog.Any("error", err))
		os.Exit(1)
	}
	defer mqttClient.Disconnect(500) // allow 500 ms to flush in-flight messages

	// ── 5. Handler & subscriptions ───────────────────────────────────────────
	h := handler.NewHandler(dbManager, mqttClient)

	// Re-subscribe every time the MQTT connection (re)establishes.
	// This covers both the initial connection and automatic reconnects,
	// because Paho only restores the TCP session — not application subscriptions.
	mqttClient.AddRoute("", nil) // ensure internal route table is initialised
	h.SetupMQTTSubscribers()

	// ── 6. HTTP server ───────────────────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: h.SetupRouter(),
		// Prevent Slowloris and similar attacks by enforcing read/write deadlines.
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Run the server in a goroutine so it doesn't block the shutdown logic below.
	go func() {
		slog.Info("HTTP server starting", slog.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP server error", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	// ── 7. Graceful shutdown ─────────────────────────────────────────────────
	// Block until we receive SIGINT (Ctrl-C) or SIGTERM (Docker/K8s stop).
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("Shutdown signal received", slog.String("signal", sig.String()))

	// Give in-flight HTTP requests up to 10 seconds to finish.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server forced to shut down", slog.Any("error", err))
	}

	// dbManager.Close() and mqttClient.Disconnect() are called via defer above.
	slog.Info("Orion shut down cleanly")
}
