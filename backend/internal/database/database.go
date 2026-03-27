package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB is the global database instance
var DB *gorm.DB

// Config holds database configuration
type Config struct {
	URL          string
	MaxOpenConns int
	MaxIdleConns int
	ConnMaxLife  string
	LogLevel     string
}

// Connect establishes a connection to the database with retry logic
func Connect(cfg Config) error {
	var err error
	maxRetries := 5
	retryDelay := time.Second

	for i := 0; i < maxRetries; i++ {
		DB, err = connectOnce(cfg)
		if err == nil {
			slog.Info("database connection established",
				"max_open_conns", cfg.MaxOpenConns,
				"max_idle_conns", cfg.MaxIdleConns,
				"conn_max_life", cfg.ConnMaxLife,
			)
			return nil
		}

		if i < maxRetries-1 {
			slog.Warn("failed to connect to database, retrying",
				"attempt", i+1,
				"max_retries", maxRetries,
				"retry_in", retryDelay,
				"error", err,
			)
			time.Sleep(retryDelay)
			retryDelay *= 2 // exponential backoff
		}
	}

	return fmt.Errorf("failed to connect to database after %d attempts: %w", maxRetries, err)
}

// connectOnce attempts a single database connection
func connectOnce(cfg Config) (*gorm.DB, error) {
	// Configure GORM logger
	var gormLogger logger.Interface
	if cfg.LogLevel == "debug" {
		gormLogger = logger.Default.LogMode(logger.Info)
	} else {
		gormLogger = logger.Default.LogMode(logger.Silent)
	}

	// Open database connection
	db, err := gorm.Open(postgres.Open(cfg.URL), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Get underlying *sql.DB
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	// Parse connection max lifetime
	connMaxLife, err := time.ParseDuration(cfg.ConnMaxLife)
	if err != nil {
		slog.Warn("invalid connection max lifetime, using default 5m",
			"value", cfg.ConnMaxLife,
			"error", err,
		)
		connMaxLife = 5 * time.Minute
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(connMaxLife)

	// Test connection
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// Close closes the database connection gracefully
func Close() error {
	if DB == nil {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}

	slog.Info("closing database connection")
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}

	return nil
}

// Health checks the database connection health
func Health() error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}

	// Test connection with context timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Get connection stats
	stats := sqlDB.Stats()
	slog.Debug("database health check",
		"open_connections", stats.OpenConnections,
		"in_use", stats.InUse,
		"idle", stats.Idle,
	)

	return nil
}
