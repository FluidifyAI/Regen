package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all application configuration
type Config struct {
	Port        string `default:"8080"`
	Environment string `default:"development"`
	LogLevel    string `default:"info"`

	// Database
	DatabaseURL    string `default:"postgresql://regen:secret@localhost:5432/regen?sslmode=disable"`
	DBMaxOpenConns int    `default:"25"`
	DBMaxIdleConns int    `default:"5"`
	DBConnMaxLife  string `default:"5m"`

	// Redis — single instance (default) or Sentinel (HA)
	// Set REDIS_SENTINEL_ADDRS to a comma-separated list of sentinel addresses
	// (e.g. "sentinel1:26379,sentinel2:26379,sentinel3:26379") to enable Sentinel mode.
	// REDIS_URL is ignored when Sentinel is active.
	RedisURL            string `default:"redis://localhost:6379"`
	RedisSentinelAddrs  string // REDIS_SENTINEL_ADDRS — comma-separated sentinel host:port list
	RedisSentinelMaster string // REDIS_SENTINEL_MASTER — master name (default: "mymaster")
	RedisPassword       string // REDIS_PASSWORD — used in both single and Sentinel mode

	// Slack
	SlackBotToken      string
	SlackSigningSecret string
	SlackAppToken      string

	// OpenAI (optional — AI features disabled if APIKey is empty)
	OpenAIAPIKey              string
	OpenAIModel               string `default:"gpt-4o-mini"`
	OpenAIMaxTokens           int    `default:"1000"`
	OpenAIPostMortemMaxTokens int    `default:"3000"`

	// Microsoft Teams (optional — Teams features disabled if AppID is empty)
	TeamsAppID       string
	TeamsAppPassword string
	TeamsTenantID    string
	TeamsTeamID      string // ID of the Team where incident channels are created
	TeamsBotUserID   string // AAD object ID of the bot user; required for direct messages
	TeamsServiceURL  string // Bot Framework relay URL for this tenant (e.g. https://smba.trafficmanager.net/amer/)

	// OSS team size limit — max active human users (AI agents and deactivated
	// users never count). Override via OSS_USER_LIMIT env var (e.g. set higher
	// for a Pro/SaaS deployment). Default: 7.
	OSSUserLimit int

	// Frontend URL (used by agents to build deep links)
	FrontendURL string `env:"FRONTEND_URL" envDefault:"http://localhost:3000"`

	// Pro licence key (optional — OSS mode when absent)
	LicenceKey string // REGEN_LICENCE_KEY

	// Telemetry — enabled by default; set REGEN_NO_TELEMETRY=1 to disable globally
	TelemetryDisabled bool // REGEN_NO_TELEMETRY=1

	// SAML SSO (optional — SSO disabled if SAMLIDPMetadataURL is empty)
	// When disabled all routes are open (backwards-compatible with existing deployments).
	SAMLIDPMetadataURL    string // SAML_IDP_METADATA_URL — IdP metadata endpoint
	SAMLEntityID          string // SAML_ENTITY_ID — SP EntityID (defaults to <base_url>/saml/metadata)
	SAMLBaseURL           string // SAML_BASE_URL — externally reachable base URL of this instance
	SAMLCertFile          string // SAML_CERT_FILE — path to SP certificate PEM (self-signed generated if empty)
	SAMLKeyFile           string // SAML_KEY_FILE — path to SP private key PEM
	SAMLAllowIDPInitiated bool   // SAML_ALLOW_IDP_INITIATED — allow IdP-initiated flows (Okta tile click)
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
		DatabaseURL:    getEnv("DATABASE_URL", "postgresql://regen:secret@localhost:5432/regen?sslmode=disable"),
		DBMaxOpenConns: getEnvAsInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns: getEnvAsInt("DB_MAX_IDLE_CONNS", 5),
		DBConnMaxLife:  getEnv("DB_CONN_MAX_LIFE", "5m"),

		// Redis
		RedisURL:            getEnv("REDIS_URL", "redis://localhost:6379"),
		RedisSentinelAddrs:  getEnv("REDIS_SENTINEL_ADDRS", ""),
		RedisSentinelMaster: getEnv("REDIS_SENTINEL_MASTER", "mymaster"),
		RedisPassword:       getEnv("REDIS_PASSWORD", ""),

		// Slack
		SlackBotToken:      getEnv("SLACK_BOT_TOKEN", ""),
		SlackSigningSecret: getEnv("SLACK_SIGNING_SECRET", ""),
		SlackAppToken:      getEnv("SLACK_APP_TOKEN", ""),

		// OpenAI
		OpenAIAPIKey:              getEnv("OPENAI_API_KEY", ""),
		OpenAIModel:               getEnv("OPENAI_MODEL", "gpt-4o-mini"),
		OpenAIMaxTokens:           getEnvAsInt("OPENAI_MAX_TOKENS", 1000),
		OpenAIPostMortemMaxTokens: getEnvAsInt("OPENAI_POSTMORTEM_MAX_TOKENS", 3000),

		// Microsoft Teams
		TeamsAppID:       getEnv("TEAMS_APP_ID", ""),
		TeamsAppPassword: getEnv("TEAMS_APP_PASSWORD", ""),
		TeamsTenantID:    getEnv("TEAMS_TENANT_ID", ""),
		TeamsTeamID:      getEnv("TEAMS_TEAM_ID", ""),
		TeamsBotUserID:   getEnv("TEAMS_BOT_USER_ID", ""),
		TeamsServiceURL:  getEnv("TEAMS_SERVICE_URL", "https://smba.trafficmanager.net/amer/"),

		// OSS user limit
		OSSUserLimit: getEnvAsInt("OSS_USER_LIMIT", 7),

		// Frontend
		FrontendURL: getEnv("FRONTEND_URL", "http://localhost:3000"),

		// Pro
		LicenceKey: getEnv("REGEN_LICENCE_KEY", ""),

		// Telemetry
		TelemetryDisabled: getEnvAsBool("REGEN_NO_TELEMETRY", false),

		// SAML SSO
		SAMLIDPMetadataURL:    getEnv("SAML_IDP_METADATA_URL", ""),
		SAMLEntityID:          getEnv("SAML_ENTITY_ID", ""),
		SAMLBaseURL:           getEnv("SAML_BASE_URL", "http://localhost:8080"),
		SAMLCertFile:          getEnv("SAML_CERT_FILE", ""),
		SAMLKeyFile:           getEnv("SAML_KEY_FILE", ""),
		SAMLAllowIDPInitiated: getEnvAsBool("SAML_ALLOW_IDP_INITIATED", false),
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

func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
