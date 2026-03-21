// cmd/migrate/main.go
// Runs database migrations using golang-migrate.
//
// Usage:
//
//	go run cmd/migrate/main.go up           # apply all pending migrations
//	go run cmd/migrate/main.go down         # roll back the last migration
//	go run cmd/migrate/main.go version      # show current migration version
//	go run cmd/migrate/main.go force <ver>  # force-set version (fix dirty state)
package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env if present (same as the main server)
	if err := godotenv.Load(); err != nil {
		slog.Warn("No .env file found, using system environment variables")
	}

	if len(os.Args) < 2 {
		fmt.Println("Usage: go run cmd/migrate/main.go [up|down|version|force <version>]")
		os.Exit(1)
	}

	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)

	m, err := migrate.New("file://migrations", dsn)
	if err != nil {
		slog.Error("Failed to initialise migrate", slog.Any("error", err))
		os.Exit(1)
	}
	defer m.Close()

	command := os.Args[1]

	switch command {
	case "up":
		if err := m.Up(); errors.Is(err, migrate.ErrNoChange) {
			slog.Info("No new migrations to apply")
		} else if err != nil {
			slog.Error("Migration up failed", slog.Any("error", err))
			os.Exit(1)
		} else {
			slog.Info("Migrations applied successfully")
		}

	case "down":
		if err := m.Steps(-1); err != nil {
			slog.Error("Migration down failed", slog.Any("error", err))
			os.Exit(1)
		}
		slog.Info("Rolled back one migration")

	case "version":
		version, dirty, err := m.Version()
		if err != nil {
			slog.Error("Failed to get migration version", slog.Any("error", err))
			os.Exit(1)
		}
		slog.Info("Current migration version",
			slog.Uint64("version", uint64(version)),
			slog.Bool("dirty", dirty),
		)

	case "force":
		if len(os.Args) < 3 {
			fmt.Println("Usage: go run cmd/migrate/main.go force <version>")
			os.Exit(1)
		}
		v, err := strconv.Atoi(os.Args[2])
		if err != nil {
			slog.Error("Invalid version number", slog.String("input", os.Args[2]))
			os.Exit(1)
		}
		if err := m.Force(v); err != nil {
			slog.Error("Force version failed", slog.Any("error", err))
			os.Exit(1)
		}
		slog.Info("Forced migration version", slog.Int("version", v))

	default:
		fmt.Printf("Unknown command %q — use up, down, version, or force <version>\n", command)
		os.Exit(1)
	}
}
