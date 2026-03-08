// cmd/etl-meta/main.go
//
// etl-meta scans the legacy ima_thing database (MariaDB), parses all table names,
// and seeds the Orion metadata tables (sites, gateways, devices, etc.).
//
// Usage:
//
//	go run cmd/etl-meta/main.go
//
// Required environment variables (in addition to the standard Orion .env):
//
//	SRC_DB_HOST     legacy MariaDB host (LAN IP)
//	SRC_DB_PORT     legacy MariaDB port (default 3306)
//	SRC_DB_USER     legacy MariaDB user
//	SRC_DB_PASSWORD legacy MariaDB password
//	SRC_DB_NAME     legacy database name (e.g. ima_thing)
//	SRC_DB_SCHEMA   legacy schema name   (same as DB name in MySQL/MariaDB)
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

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

	// ── Connect to legacy MariaDB (source) ───────────────────────────────────
	srcDB, err := connectMariaDB(
		getenv("SRC_DB_HOST", "localhost"),
		getenv("SRC_DB_PORT", "3306"),
		mustenv("SRC_DB_USER"),
		mustenv("SRC_DB_PASSWORD"),
		mustenv("SRC_DB_NAME"),
	)
	if err != nil {
		slog.Error("Failed to connect to source DB (MariaDB)", slog.Any("error", err))
		os.Exit(1)
	}
	slog.Info("Connected to source DB (MariaDB)",
		slog.String("host", getenv("SRC_DB_HOST", "localhost")),
		slog.String("db", mustenv("SRC_DB_NAME")),
	)

	// ── Connect to Orion PostgreSQL (destination) ────────────────────────────
	dstDB, err := connectPostgres(
		mustenv("DB_HOST"),
		getenv("DB_PORT", "5432"),
		mustenv("DB_USER"),
		mustenv("DB_PASSWORD"),
		mustenv("DB_NAME"),
	)
	if err != nil {
		slog.Error("Failed to connect to destination DB (PostgreSQL)", slog.Any("error", err))
		os.Exit(1)
	}
	slog.Info("Connected to destination DB (PostgreSQL)",
		slog.String("host", mustenv("DB_HOST")),
		slog.String("db", mustenv("DB_NAME")),
	)

	// ── Run seeder ───────────────────────────────────────────────────────────
	// In MySQL/MariaDB, schema = database name.
	srcSchema := getenv("SRC_DB_SCHEMA", mustenv("SRC_DB_NAME"))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	seeder := etl.NewMetaSeeder(srcDB, dstDB)
	if err := seeder.Run(ctx, srcSchema); err != nil {
		slog.Error("MetaSeeder failed", slog.Any("error", err))
		os.Exit(1)
	}

	slog.Info("etl-meta completed — review etl_table_map before running etl-telemetry")
}

// ── DB connection helpers ─────────────────────────────────────────────────────

// connectMariaDB opens a GORM connection to a MariaDB / MySQL database.
// DSN format: user:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True&loc=UTC
func connectMariaDB(host, port, user, password, dbname string) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=UTC",
		user, password, host, port, dbname,
	)
	return gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
}

// connectPostgres opens a GORM connection to a PostgreSQL database.
func connectPostgres(host, port, user, password, dbname string) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname,
	)
	return gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
}

// ── env helpers ───────────────────────────────────────────────────────────────

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
