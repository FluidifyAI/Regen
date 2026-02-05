package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all application configuration
type Config struct {
	App      AppConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Slack    SlackConfig
	OpenAI   OpenAIConfig
}

// AppConfig holds application-level configuration
type AppConfig struct {
	Env      string
	LogLevel string
	Port     int
}

// DatabaseConfig holds PostgreSQL configuration
type DatabaseConfig struct {
	URL string
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	URL string
}

// SlackConfig holds Slack integration configuration
type SlackConfig struct {
	BotToken      string
	SigningSecret string
	AppToken      string
}

// OpenAIConfig holds OpenAI API configuration
type OpenAIConfig struct {
	APIKey string
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists (ignore error if not found)
	_ = godotenv.Load()

	cfg := &Config{
		App: AppConfig{
			Env:      getEnv("APP_ENV", "development"),
			LogLevel: getEnv("LOG_LEVEL", "info"),
			Port:     getEnvAsInt("PORT", 8080),
		},
		Database: DatabaseConfig{
			URL: getEnv("DATABASE_URL", "postgresql://openincident:secret@localhost:5432/openincident?sslmode=disable"),
		},
		Redis: RedisConfig{
			URL: getEnv("REDIS_URL", "redis://localhost:6379"),
		},
		Slack: SlackConfig{
			BotToken:      getEnv("SLACK_BOT_TOKEN", ""),
			SigningSecret: getEnv("SLACK_SIGNING_SECRET", ""),
			AppToken:      getEnv("SLACK_APP_TOKEN", ""),
		},
		OpenAI: OpenAIConfig{
			APIKey: getEnv("OPENAI_API_KEY", ""),
		},
	}

	// Validate required configuration
	if cfg.App.Env == "production" {
		if cfg.Database.URL == "" {
			return nil, fmt.Errorf("DATABASE_URL is required in production")
		}
	}

	return cfg, nil
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.App.Env == "development"
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return c.App.Env == "production"
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
