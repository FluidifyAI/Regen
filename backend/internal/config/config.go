package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all application configuration
type Config struct {
	Port        string `default:"8080"`
	Environment string `default:"development"`
	LogLevel    string `default:"info"`

	// Database
	DatabaseURL    string `default:"postgresql://openincident:secret@localhost:5432/openincident?sslmode=disable"`
	DBMaxOpenConns int    `default:"25"`
	DBMaxIdleConns int    `default:"5"`
	DBConnMaxLife  string `default:"5m"`

	// Redis
	RedisURL string `default:"redis://localhost:6379"`

	// Slack
	SlackBotToken          string
	SlackSigningSecret     string
	SlackAppToken          string
	SlackAutoInviteUserIDs []string

	// OpenAI (optional — AI features disabled if APIKey is empty)
	OpenAIAPIKey    string
	OpenAIModel     string `default:"gpt-4o-mini"`
	OpenAIMaxTokens int    `default:"1000"`
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists (ignore error if not found)
	_ = godotenv.Load()

	cfg := &Config{
		Port:        getEnv("PORT", "8080"),
		Environment: getEnv("APP_ENV", "development"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),

		// Database
		DatabaseURL:    getEnv("DATABASE_URL", "postgresql://openincident:secret@localhost:5432/openincident?sslmode=disable"),
		DBMaxOpenConns: getEnvAsInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns: getEnvAsInt("DB_MAX_IDLE_CONNS", 5),
		DBConnMaxLife:  getEnv("DB_CONN_MAX_LIFE", "5m"),

		// Redis
		RedisURL: getEnv("REDIS_URL", "redis://localhost:6379"),

		// Slack
		SlackBotToken:          getEnv("SLACK_BOT_TOKEN", ""),
		SlackSigningSecret:     getEnv("SLACK_SIGNING_SECRET", ""),
		SlackAppToken:          getEnv("SLACK_APP_TOKEN", ""),
		SlackAutoInviteUserIDs: getEnvAsSlice("SLACK_AUTO_INVITE_USER_IDS", []string{}),

		// OpenAI
		OpenAIAPIKey:    getEnv("OPENAI_API_KEY", ""),
		OpenAIModel:     getEnv("OPENAI_MODEL", "gpt-4o-mini"),
		OpenAIMaxTokens: getEnvAsInt("OPENAI_MAX_TOKENS", 1000),
	}

	// Validate required configuration
	if cfg.Environment == "production" {
		if cfg.DatabaseURL == "" {
			return nil, fmt.Errorf("DATABASE_URL is required in production")
		}
	}

	return cfg, nil
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
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

// getEnvAsSlice parses a comma-separated environment variable into a string slice.
// Trims whitespace from each value and filters out empty strings.
func getEnvAsSlice(key string, defaultValue []string) []string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	if len(result) == 0 {
		return defaultValue
	}
	return result
}
