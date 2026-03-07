// Package database provides a DBManager that encapsulates both GORM and pgxpool connections.
package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DBManager wraps two database engines so the Service layer
// can access both GORM and pgx instances.
type DBManager struct {
	GormDB  *gorm.DB
	PgxPool *pgxpool.Pool
}

// Config represents the database connection configuration.
type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

// InitDB initializes the database connections and returns a DBManager
// containing both GORM and pgxpool instances.
func InitDB(ctx context.Context, cfg Config) (*DBManager, error) {
	// Create a logger dedicated to this module with a component tag
	log := slog.Default().With(slog.String("component", "DBManager"))

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName,
	)

	// 1. Initialize pgxpool (used for high-speed Telemetry writes)
	pgxConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Error("Failed to parse pgx config", slog.Any("error", err))
		return nil, fmt.Errorf("failed to parse pgx config: %w", err)
	}

	// Tune the connection pool for high-concurrency IoT write workloads
	pgxConfig.MaxConns = 50
	pgxConfig.MinConns = 10
	pgxConfig.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, pgxConfig)
	if err != nil {
		log.Error("Failed to create pgxpool", slog.Any("error", err))
		return nil, fmt.Errorf("failed to create pgxpool: %w", err)
	}

	// 2. Initialize GORM (used for Metadata CRUD operations)
	// Note: the logger here is GORM's internal SQL query logger.
	// Whether SQL queries should be printed can be configured per environment.
	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Error("Failed to establish GORM connection", slog.Any("error", err))
		return nil, fmt.Errorf("failed to establish GORM connection: %w", err)
	}

	// Configure the connection pool for GORM's underlying sql.DB
	sqlDB, err := gormDB.DB()
	if err != nil {
		log.Error("Failed to obtain sql.DB", slog.Any("error", err))
		return nil, fmt.Errorf("failed to obtain sql.DB: %w", err)
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetConnMaxLifetime(time.Hour)

	log.Info("Database connected successfully (GORM + pgxpool)")

	return &DBManager{
		GormDB:  gormDB,
		PgxPool: pool,
	}, nil
}

// Close gracefully shuts down all database connections.
func (m *DBManager) Close() {
	log := slog.Default().With(slog.String("component", "DBManager"))

	if m.PgxPool != nil {
		m.PgxPool.Close()
		log.Info("pgxpool connection closed")
	}

	if m.GormDB != nil {
		sqlDB, err := m.GormDB.DB()
		if err != nil {
			log.Error("Failed to obtain underlying sql.DB from GORM; unable to close connection properly", slog.Any("error", err))
			return
		}

		if err := sqlDB.Close(); err != nil {
			// Fixes errcheck warning
			log.Error("Error occurred while closing GORM sql.DB connection", slog.Any("error", err))
		} else {
			log.Info("GORM sql.DB connection closed")
		}
	}

	log.Info("Database connection shutdown sequence completed")
}
