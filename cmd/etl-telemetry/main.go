// cmd/etl-telemetry/main.go
//
// etl-telemetry reads the etl_table_map built by etl-meta and migrates
// historical time-series data from the legacy MariaDB tables into the
// Orion TimescaleDB hypertables.
//
// It supports checkpoint/resume: interrupted runs can be safely restarted.
//
// Usage:
//
//	go run cmd/etl-telemetry/main.go
//
// Required environment variables (same as etl-meta, plus):
//
//	ETL_WORKERS   number of concurrent table workers (default 4)
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/hill/orion/internal/etl"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := godotenv.Load(); err != nil {
		slog.Warn("No .env file found, using system environment variables")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── Connect to legacy MariaDB (source) ───────────────────────────────────
	srcDB, err := connectMariaDB(
		getenv("SRC_DB_HOST", "localhost"),
		getenv("SRC_DB_PORT", "3306"),
		mustenv("SRC_DB_USER"),
		mustenv("SRC_DB_PASSWORD"),
		mustenv("SRC_DB_NAME"),
	)
	if err != nil {
		slog.Error("Failed to connect to source DB", slog.Any("error", err))
		os.Exit(1)
	}
	slog.Info("Connected to source DB (MariaDB)",
		slog.String("host", getenv("SRC_DB_HOST", "localhost")),
		slog.String("db", mustenv("SRC_DB_NAME")),
	)

	// ── Connect to Orion PostgreSQL (destination) — GORM for checkpoints ────
	dstGorm, err := connectPostgres(
		mustenv("DB_HOST"),
		getenv("DB_PORT", "5432"),
		mustenv("DB_USER"),
		mustenv("DB_PASSWORD"),
		mustenv("DB_NAME"),
	)
	if err != nil {
		slog.Error("Failed to connect to destination DB (GORM)", slog.Any("error", err))
		os.Exit(1)
	}

	// ── Connect to Orion PostgreSQL — pgxpool for fast CopyFrom ─────────────
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		mustenv("DB_HOST"), getenv("DB_PORT", "5432"),
		mustenv("DB_USER"), mustenv("DB_PASSWORD"), mustenv("DB_NAME"),
	)
	dstPool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		slog.Error("Failed to create pgxpool", slog.Any("error", err))
		os.Exit(1)
	}
	defer dstPool.Close()

	slog.Info("Connected to destination DB (PostgreSQL)",
		slog.String("host", mustenv("DB_HOST")),
		slog.String("db", mustenv("DB_NAME")),
	)

	// ── Parse run parameters ─────────────────────────────────────────────────
	workers := parseInt(getenv("ETL_WORKERS", "1"))
	batchSleepMs := parseInt(getenv("ETL_BATCH_SLEEP_MS", "200"))

	// ETL_FROM / ETL_TO: date range to migrate (default: last 1 year)
	// Format: 2006-01-02  or  2006-01-02T15:04:05
	now := time.Now().UTC()
	from := parseTime(getenv("ETL_FROM", now.AddDate(-1, 0, 0).Format("2006-01-02")))
	to := parseTime(getenv("ETL_TO", now.Format("2006-01-02")))

	slog.Info("ETL parameters",
		slog.Int("workers", workers),
		slog.Int("batch_sleep_ms", batchSleepMs),
		slog.Time("from", from),
		slog.Time("to", to),
	)

	cfg := etl.TelemetryConfig{
		Workers:      workers,
		BatchSleepMs: batchSleepMs,
		From:         from,
		To:           to,
	}

	// ── Run worker ────────────────────────────────────────────────────────────
	worker := etl.NewTelemetryWorker(srcDB, dstPool, dstGorm, cfg)

	start := time.Now()
	if err := worker.Run(ctx); err != nil {
		slog.Error("etl-telemetry failed", slog.Any("error", err))
		os.Exit(1)
	}
	slog.Info("etl-telemetry completed", slog.Duration("elapsed", time.Since(start)))
}

// ── DB connection helpers ─────────────────────────────────────────────────────

func connectMariaDB(host, port, user, password, dbname string) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=UTC",
		user, password, host, port, dbname,
	)
	return gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
}

func connectPostgres(host, port, user, password, dbname string) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname,
	)
	return gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
}

// ── env / parse helpers ───────────────────────────────────────────────────────

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustenv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("Required environment variable not set", slog.String("key", key))
		os.Exit(1)
	}
	return v
}

func parseInt(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 1
	}
	return n
}

func parseTime(s string) time.Time {
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return t
		}
	}
	slog.Warn("Could not parse time, using zero", slog.String("value", s))
	return time.Time{}
}
