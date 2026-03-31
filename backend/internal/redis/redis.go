package redis

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client is the global Redis client instance.
// Backed by either a single-node client or a Sentinel failover client —
// callers use the same interface either way.
var Client *redis.Client

// Config holds Redis configuration.
// If SentinelAddrs is non-empty, Sentinel (HA) mode is used and URL is ignored.
type Config struct {
	URL            string // single-instance URL (redis://[:password@]host:port/db)
	SentinelAddrs  string // comma-separated sentinel host:port list
	SentinelMaster string // sentinel master name (default: "mymaster")
	Password       string // shared password for both modes
}

// Connect establishes a connection to Redis.
// When cfg.SentinelAddrs is set it connects via Sentinel for HA failover.
// Otherwise it falls back to the single-instance URL.
func Connect(cfg Config) error {
	if cfg.SentinelAddrs != "" {
		return connectSentinel(cfg)
	}
	return connectSingle(cfg)
}

func connectSingle(cfg Config) error {
	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return fmt.Errorf("invalid redis URL: %w", err)
	}
	if cfg.Password != "" {
		opts.Password = cfg.Password
	}

	Client = redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := Client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to ping redis: %w", err)
	}

	slog.Info("redis connection established",
		"mode", "single",
		"addr", opts.Addr,
		"db", opts.DB,
	)
	return nil
}

func connectSentinel(cfg Config) error {
	master := cfg.SentinelMaster
	if master == "" {
		master = "mymaster"
	}

	addrs := strings.Split(cfg.SentinelAddrs, ",")
	for i, a := range addrs {
		addrs[i] = strings.TrimSpace(a)
	}

	Client = redis.NewFailoverClient(&redis.FailoverOptions{
		MasterName:    master,
		SentinelAddrs: addrs,
		Password:      cfg.Password,
		// Sentinel itself may also require auth in production deployments.
		// SentinelPassword can be added here if needed.
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := Client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to ping redis via sentinel: %w", err)
	}

	slog.Info("redis connection established",
		"mode", "sentinel",
		"master", master,
		"sentinels", addrs,
	)
	return nil
}

// Close closes the Redis connection gracefully.
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

// Health checks the Redis connection health.
func Health() error {
	if Client == nil {
		return fmt.Errorf("redis not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := Client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}

	stats := Client.PoolStats()
	slog.Debug("redis health check",
		"total_conns", stats.TotalConns,
		"idle_conns", stats.IdleConns,
		"stale_conns", stats.StaleConns,
	)

	return nil
}
