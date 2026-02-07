package redis

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client is the global Redis client instance
var Client *redis.Client

// Config holds Redis configuration
type Config struct {
	URL string
}

// Connect establishes a connection to Redis
func Connect(cfg Config) error {
	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return fmt.Errorf("invalid redis URL: %w", err)
	}

	Client = redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := Client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to ping redis: %w", err)
	}

	slog.Info("redis connection established",
		"addr", opts.Addr,
		"db", opts.DB,
	)

	return nil
}

// Close closes the Redis connection gracefully
func Close() error {
	if Client == nil {
		return nil
	}

	slog.Info("closing redis connection")
	if err := Client.Close(); err != nil {
		return fmt.Errorf("failed to close redis: %w", err)
	}

	return nil
}

// Health checks the Redis connection health
func Health() error {
	if Client == nil {
		return fmt.Errorf("redis not initialized")
	}

	// Test connection with context timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := Client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}

	// Get connection stats
	stats := Client.PoolStats()
	slog.Debug("redis health check",
		"total_conns", stats.TotalConns,
		"idle_conns", stats.IdleConns,
		"stale_conns", stats.StaleConns,
	)

	return nil
}
