package commands

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/openincident/openincident/internal/api"
	"github.com/openincident/openincident/internal/auth"
	"github.com/openincident/openincident/internal/config"
	"github.com/openincident/openincident/internal/database"
	"github.com/openincident/openincident/internal/metrics"
	"github.com/openincident/openincident/internal/redis"
	"github.com/openincident/openincident/internal/repository"
	"github.com/openincident/openincident/internal/services"
	"github.com/openincident/openincident/internal/worker"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the OpenIncident HTTP server",
		RunE:  runServe,
	}
}

func runServe(_ *cobra.Command, _ []string) error {
	if err := godotenv.Load(); err != nil {
		slog.Warn("no .env file found")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	setupLogging(cfg.LogLevel)

	slog.Info("starting OpenIncident",
		"version", "0.9.0",
		"environment", cfg.Environment,
		"port", cfg.Port,
	)

	dbLogLevel := "info"
	if cfg.Environment == "production" {
		dbLogLevel = "warn"
	}
	dbConfig := database.Config{
		URL:          cfg.DatabaseURL,
		MaxOpenConns: cfg.DBMaxOpenConns,
		MaxIdleConns: cfg.DBMaxIdleConns,
		ConnMaxLife:  cfg.DBConnMaxLife,
		LogLevel:     dbLogLevel,
	}
	if err := database.Connect(dbConfig); err != nil {
		return err
	}
	defer database.Close()

	if err := redis.Connect(redis.Config{URL: cfg.RedisURL}); err != nil {
		return err
	}
	defer redis.Close()

	slog.Info("running database migrations...")
	if err := database.RunMigrations(database.DB, "./migrations"); err != nil {
		return err
	}

	// Create the application lifecycle context before any service that needs it.
	// Cancelled on SIGTERM, which propagates to all in-flight Graph API calls.
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	// Initialize TeamsService once — shared by API routes and background workers.
	// Constructing it twice wastes two OAuth2 token sources and two startup validation calls.
	var teamsSvc *services.TeamsService
	if cfg.TeamsAppID != "" {
		var err error
		teamsSvc, err = services.NewTeamsService(appCtx, cfg.TeamsAppID, cfg.TeamsAppPassword, cfg.TeamsTenantID, cfg.TeamsTeamID, cfg.TeamsBotUserID, cfg.TeamsServiceURL)
		if err != nil {
			slog.Error("teams service initialization failed", "error", err)
			slog.Warn("continuing without Teams integration — check TEAMS_* env vars and Azure app permissions")
		} else {
			slog.Info("Teams integration enabled")
		}
	} else {
		slog.Warn("TEAMS_APP_ID not set — Teams integration disabled")
	}

	// Initialize SAML middleware (optional — nil when SAML_IDP_METADATA_URL is not set).
	// Must run after migrations so the users table exists for JIT provisioning.
	samlMiddleware, err := auth.NewSAMLMiddleware(cfg)
	if err != nil {
		return fmt.Errorf("saml: %w", err)
	}
	if samlMiddleware != nil {
		// Wrap the default CookieSessionProvider with JIT user provisioning.
		userRepo := repository.NewUserRepository(database.DB)
		authSvc := services.NewAuthService(userRepo)
		samlMiddleware.Session = auth.NewProvisioningSessionProvider(samlMiddleware.Session, authSvc)
		slog.Info("SAML SSO enabled", "base_url", cfg.SAMLBaseURL)
	} else {
		slog.Warn("SAML SSO disabled — set SAML_IDP_METADATA_URL to enable authentication")
	}

	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	api.SetupRoutes(router, database.DB, cfg, teamsSvc, samlMiddleware)

	worker.StartAll(appCtx, database.DB, cfg, teamsSvc)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		metrics.UpdateBusinessMetrics(database.DB)
		for range ticker.C {
			metrics.UpdateBusinessMetrics(database.DB)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")
	appCancel()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func setupLogging(level string) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	slog.SetDefault(slog.New(handler))
}
