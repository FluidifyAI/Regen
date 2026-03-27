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
	"github.com/fluidify/regen/internal/api"
	"github.com/fluidify/regen/internal/auth"
	"github.com/fluidify/regen/internal/config"
	"github.com/fluidify/regen/internal/coordinator"
	"github.com/fluidify/regen/internal/coordinator/agents"
	"github.com/fluidify/regen/internal/database"
	"github.com/fluidify/regen/internal/enterprise"
	"github.com/fluidify/regen/internal/metrics"
	"github.com/fluidify/regen/internal/redis"
	"github.com/fluidify/regen/internal/repository"
	"github.com/fluidify/regen/internal/services"
	"github.com/fluidify/regen/internal/worker"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the Fluidify Regen HTTP server",
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

	slog.Info("starting Fluidify Regen",
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

	// Seed AI agent user accounts (idempotent — safe to call on every startup).
	if err := coordinator.SeedAgents(repository.NewUserRepository(database.DB)); err != nil {
		slog.Error("failed to seed AI agents", "error", err)
		// Non-fatal: agents won't run until next restart when DB is healthy.
	}

	// Create the application lifecycle context before any service that needs it.
	// Cancelled on SIGTERM, which propagates to all in-flight Graph API calls.
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	// Initialize TeamsService once — shared by API routes and background workers.
	// DB config takes precedence over env vars so admins can configure Teams via
	// the setup wizard without restarting the server.
	var teamsSvc *services.TeamsService
	{
		teamsAppID := cfg.TeamsAppID
		teamsAppPassword := cfg.TeamsAppPassword
		teamsTenantID := cfg.TeamsTenantID
		teamsTeamID := cfg.TeamsTeamID
		teamsBotUserID := cfg.TeamsBotUserID
		teamsServiceURL := cfg.TeamsServiceURL

		dbTeamsCfg, err := repository.NewTeamsConfigRepository(database.DB).Get()
		if err != nil {
			slog.Warn("failed to load teams config from DB, falling back to env vars", "error", err)
		} else if dbTeamsCfg != nil && dbTeamsCfg.AppID != "" {
			slog.Info("using Teams config from database")
			teamsAppID = dbTeamsCfg.AppID
			teamsAppPassword = dbTeamsCfg.AppPassword
			teamsTenantID = dbTeamsCfg.TenantID
			teamsTeamID = dbTeamsCfg.TeamID
			teamsBotUserID = dbTeamsCfg.BotUserID
			teamsServiceURL = dbTeamsCfg.ServiceURL
		}

		if teamsAppID != "" {
			teamsSvc, err = services.NewTeamsService(appCtx, teamsAppID, teamsAppPassword, teamsTenantID, teamsTeamID, teamsBotUserID, teamsServiceURL)
			if err != nil {
				slog.Error("teams service initialization failed", "error", err)
				slog.Warn("continuing without Teams integration — check Teams config in Settings or TEAMS_* env vars")
			} else {
				slog.Info("Teams integration enabled")
			}
		} else {
			slog.Warn("Teams app_id not configured — Teams integration disabled")
		}
	}

	// Initialize user and session repositories for local auth and SAML JIT provisioning.
	userRepo := repository.NewUserRepository(database.DB)
	sessionRepo := repository.NewLocalSessionRepository(database.DB)
	localAuthSvc := services.NewLocalAuthService(userRepo, sessionRepo)

	// Periodic cleanup of expired local sessions. Daily is sufficient — sessions
	// are already rejected at read time (expires_at > NOW()), so this is purely
	// a table housekeeping operation with no security impact.
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := sessionRepo.DeleteExpired(); err != nil {
					slog.Warn("session cleanup failed", "error", err)
				}
			case <-appCtx.Done():
				return
			}
		}
	}()

	// Initialize SAML middleware (optional — nil when SAML_IDP_METADATA_URL is not set).
	// Must run after migrations so the users table exists for JIT provisioning.
	samlMiddleware, err := auth.NewSAMLMiddleware(cfg)
	if err != nil {
		return fmt.Errorf("saml: %w", err)
	}
	if samlMiddleware != nil {
		// Wrap the default CookieSessionProvider with JIT user provisioning.
		authSvc := services.NewAuthService(userRepo)
		samlMiddleware.Session = auth.NewProvisioningSessionProvider(samlMiddleware.Session, authSvc)
		slog.Info("SAML SSO enabled", "base_url", cfg.SAMLBaseURL)
	} else {
		slog.Warn("SAML SSO disabled — set SAML_IDP_METADATA_URL to enable authentication")
	}

	// Enterprise hooks — no-op stubs in the OSS build.
	// Replace enterprise.NewNoOp() with the real implementation in the
	// enterprise binary to unlock SCIM, audit log export, RBAC, and retention.
	enterpriseHooks := enterprise.NewNoOp()

	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	api.SetupRoutes(router, database.DB, cfg, teamsSvc, samlMiddleware, enterpriseHooks, localAuthSvc)

	worker.StartAll(appCtx, database.DB, cfg, teamsSvc, enterpriseHooks)

	// Start the AI coordinator.
	pmAgentUser, pmAgentErr := userRepo.GetByEmail(coordinator.PostMortemAgentEmail)
	if pmAgentErr != nil {
		slog.Warn("post-mortem agent user not found, AI coordinator not starting", "error", pmAgentErr)
	} else if pmAgentUser.Active {
		var agentSlackSvc services.ChatService
		if cfg.SlackBotToken != "" {
			if slackSvc, slackErr := services.NewSlackService(cfg.SlackBotToken); slackErr == nil {
				agentSlackSvc = slackSvc
			} else {
				slog.Warn("AI coordinator: slack service init failed", "error", slackErr)
			}
		}

		activeChatSvcs := make([]services.ChatService, 0, 2)
		if agentSlackSvc != nil {
			activeChatSvcs = append(activeChatSvcs, agentSlackSvc)
		}
		if teamsSvc != nil {
			activeChatSvcs = append(activeChatSvcs, teamsSvc)
		}
		var multiChat services.ChatService
		switch len(activeChatSvcs) {
		case 1:
			multiChat = activeChatSvcs[0]
		case 2:
			multiChat = services.NewMultiChatService(activeChatSvcs...)
		}

		aiSvc := services.NewAIService(cfg.OpenAIAPIKey, cfg.OpenAIModel, cfg.OpenAIMaxTokens, cfg.OpenAIPostMortemMaxTokens)
		pmRepo := repository.NewPostMortemRepository(database.DB)
		pmTemplateRepo := repository.NewPostMortemTemplateRepository(database.DB)
		agentIncidentRepo := repository.NewIncidentRepository(database.DB)
		agentTimelineRepo := repository.NewTimelineRepository(database.DB)
		agentAlertRepo := repository.NewAlertRepository(database.DB)
		agentIncidentSvc := services.NewIncidentService(
			agentIncidentRepo, agentTimelineRepo,
			agentAlertRepo,
			agentSlackSvc, database.DB,
		)
		services.SetTeamsService(agentIncidentSvc, teamsSvc)
		agentCommentRepo := repository.NewPostMortemCommentRepository(database.DB)
		pmSvc := services.NewPostMortemService(pmRepo, pmTemplateRepo, agentCommentRepo, agentIncidentSvc, aiSvc)

		pmAgent := agents.NewPostMortemAgent(agents.PostMortemAgentDeps{
			AgentUserID:   pmAgentUser.ID,
			AISvc:         aiSvc,
			IncidentRepo:  agentIncidentRepo,
			PostMortemSvc: pmSvc,
			TimelineRepo:  agentTimelineRepo,
			SlackSvc:      agentSlackSvc,
			TeamsSvc:      teamsSvc,
			MultiChat:     multiChat,
			UserRepo:      userRepo,
			FrontendURL:   cfg.FrontendURL,
			WaitDuration:  agents.DefaultWaitDuration,
		})

		coord := coordinator.New(redis.Client, pmAgent)
		go coord.Start(appCtx)
		slog.Info("AI coordinator started")
	}

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
