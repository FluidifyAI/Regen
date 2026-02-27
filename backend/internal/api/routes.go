package api

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/crewjam/saml/samlsp"
	"github.com/gin-gonic/gin"
	"github.com/openincident/openincident/internal/api/handlers"
	"github.com/openincident/openincident/internal/api/middleware"
	"github.com/openincident/openincident/internal/config"
	"github.com/openincident/openincident/internal/enterprise"
	"github.com/openincident/openincident/internal/metrics"
	"github.com/openincident/openincident/internal/models/webhooks"
	"github.com/openincident/openincident/internal/repository"
	"github.com/openincident/openincident/internal/services"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"
)

// SetupRoutes configures all application routes.
// teamsSvc may be nil when Teams integration is disabled.
// samlMiddleware may be nil when SSO is not configured (all routes open).
// localAuth may be nil when local auth is not configured.
// hooks contains enterprise extension points; OSS callers pass enterprise.NewNoOp().
func SetupRoutes(router *gin.Engine, db *gorm.DB, cfg *config.Config, teamsSvc *services.TeamsService, samlMiddleware *samlsp.Middleware, hooks enterprise.Hooks, localAuth services.LocalAuthService) {
	// Initialize repositories
	alertRepo := repository.NewAlertRepository(db)
	incidentRepo := repository.NewIncidentRepository(db)
	timelineRepo := repository.NewTimelineRepository(db)
	groupingRuleRepo := repository.NewGroupingRuleRepository(db)
	routingRuleRepo := repository.NewRoutingRuleRepository(db)
	escalationPolicyRepo := repository.NewEscalationPolicyRepository(db)

	// Initialize Slack service (optional - graceful degradation if not configured)
	var chatService services.ChatService
	slackToken := os.Getenv("SLACK_BOT_TOKEN")
	if slackToken != "" {
		// Validate Slack configuration
		validator := services.NewSlackValidator(slackToken)
		if err := validator.ValidateToken(); err != nil {
			slog.Error("slack token validation failed", "error", err)
			slog.Warn("continuing without slack integration - incidents will be created but no slack channels will be created")
		} else if err := validator.ValidateScopes(); err != nil {
			slog.Error("slack scope validation failed", "error", err)
			slog.Warn("continuing without slack integration - please check bot permissions")
		} else {
			// Token and scopes validated - initialize Slack service
			var err error
			chatService, err = services.NewSlackService(slackToken)
			if err != nil {
				slog.Error("failed to initialize slack service", "error", err)
				slog.Warn("continuing without slack integration")
			} else {
				slog.Info("slack integration enabled")
			}
		}
	} else {
		slog.Warn("SLACK_BOT_TOKEN not set - running in degraded mode without slack integration")
	}

	// Initialize grouping engine (for alert deduplication and grouping)
	groupingEngine := services.NewGroupingEngine(groupingRuleRepo, incidentRepo, db)
	slog.Info("grouping engine initialized")

	// Initialize routing engine (for alert routing decisions)
	routingEngine := services.NewRoutingEngine(routingRuleRepo)
	slog.Info("routing engine initialized")

	// Initialize schedule evaluator (for on-call schedule evaluation)
	scheduleRepo := repository.NewScheduleRepository(db)
	scheduleEvaluator := services.NewScheduleEvaluator(scheduleRepo)
	slog.Info("schedule evaluator initialized")

	// Initialize escalation engine (v0.5+)
	escalationEngine := services.NewEscalationEngine(escalationPolicyRepo, scheduleEvaluator, nil)
	slog.Info("escalation engine initialized")

	// Initialize AI service (v0.6+) — noop if OPENAI_API_KEY is not set
	aiSvc := services.NewAIService(cfg.OpenAIAPIKey, cfg.OpenAIModel, cfg.OpenAIMaxTokens, cfg.OpenAIPostMortemMaxTokens)
	if aiSvc.IsEnabled() {
		slog.Info("AI service enabled", "model", cfg.OpenAIModel)
	} else {
		slog.Warn("AI service disabled — set OPENAI_API_KEY to enable AI features")
	}

	// Post-mortem repositories (v0.7+)
	postMortemTemplateRepo := repository.NewPostMortemTemplateRepository(db)
	pmRepo := repository.NewPostMortemRepository(db)

	// Initialize services
	incidentSvc := services.NewIncidentService(incidentRepo, timelineRepo, alertRepo, chatService, db, cfg.SlackAutoInviteUserIDs)
	services.SetAIService(incidentSvc, aiSvc)
	alertSvc := services.NewAlertService(alertRepo, incidentSvc)
	alertSvc.SetGroupingEngine(groupingEngine)
	alertSvc.SetRoutingEngine(routingEngine)
	alertSvc.SetEscalationEngine(escalationEngine)

	// Post-mortem service (v0.7+)
	postMortemSvc := services.NewPostMortemService(pmRepo, postMortemTemplateRepo, incidentSvc, aiSvc)

	// Wire Teams service into incident service (v0.8+).
	// teamsSvc is constructed once in serve.go and injected here.
	if teamsSvc != nil {
		services.SetTeamsService(incidentSvc, teamsSvc)
	}

	// Start Slack Socket Mode event handler (bidirectional sync)
	// Requires SLACK_APP_TOKEN in addition to SLACK_BOT_TOKEN
	if cfg.SlackAppToken != "" && chatService != nil {
		eventHandler, err := services.NewSlackEventHandler(
			cfg.SlackAppToken,
			cfg.SlackBotToken,
			incidentSvc,
			chatService,
		)
		if err != nil {
			slog.Error("failed to initialize slack socket mode", "error", err)
			slog.Warn("bidirectional Slack sync disabled - Slack will be one-way only")
		} else {
			eventHandler.Start()
		}
	} else if cfg.SlackAppToken == "" && chatService != nil {
		slog.Warn("SLACK_APP_TOKEN not set - bidirectional Slack sync disabled (one-way only)")
	}

	// Middleware
	router.Use(middleware.RequestID())       // Must be first for request tracing
	router.Use(middleware.SecurityHeaders()) // Security headers on all responses
	router.Use(middleware.CORS())
	router.Use(middleware.Recovery())
	router.Use(middleware.Logger())
	router.Use(metrics.Middleware())                    // Prometheus metrics
	router.Use(middleware.AuditLog(hooks.Audit))        // Enterprise audit trail (no-op in OSS)

	// Health check endpoints (always open — liveness/readiness probes)
	router.GET("/health", handlers.Health(db))
	router.GET("/ready", handlers.Ready(db))

	// Metrics endpoint (always open — scraped by Prometheus via network policy)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Auth endpoints (always open — these ARE the login flow)
	if samlMiddleware != nil {
		// crewjam/saml's ServeHTTP handles /saml/metadata and /saml/acs internally,
		// but falls through to its inner (nil) handler for all other paths.
		// We dispatch /saml/login to HandleStartAuthFlow ourselves — we can't
		// register it as a separate Gin route alongside the *action wildcard
		// (httprouter conflict), so we intercept inside the handler instead.
		samlHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/saml/login" || r.URL.Path == "/saml/login/" {
				samlMiddleware.HandleStartAuthFlow(w, r)
				return
			}
			samlMiddleware.ServeHTTP(w, r)
		})
		samlGroup := router.Group("/saml", middleware.RateLimitAuth())
		samlGroup.Any("/*action", gin.WrapH(samlHandler))
		slog.Info("SAML SSO routes registered", "login", "/saml/login", "metadata", "/saml/metadata")
	} else {
		slog.Warn("SAML SSO disabled — set SAML_IDP_METADATA_URL to enable authentication")
	}
	router.GET("/auth/logout", handlers.Logout(samlMiddleware, localAuth))

	// SCIM 2.0 provisioning routes — enterprise only, no-op stub returns 501 in OSS.
	scimGroup := router.Group("/scim/v2")
	hooks.SCIM.RegisterRoutes(scimGroup)

	// API v1 routes
	// Webhooks are machine-to-machine and intentionally excluded from RequireAuth.
	// All other /api/v1 routes require a valid session (no-op when auth disabled).
	v1 := router.Group("/api/v1")
	{
		// Webhooks (with 1MB body size limit + rate limit: 300 req/min per IP)
		webhooksGroup := v1.Group("/webhooks")
		webhooksGroup.Use(middleware.BodySizeLimit(middleware.WebhookMaxBodySize))
		webhooksGroup.Use(middleware.RateLimitWebhooks())
		{
			// v0.1: Prometheus Alertmanager webhook (legacy handler for backwards compatibility)
			webhooksGroup.POST("/prometheus", handlers.PrometheusWebhook(alertSvc))

			// v0.3: Multi-source webhooks using unified handler with WebhookProvider interface
			// Each webhook route uses the same handler with a different provider implementation

			// Grafana Unified Alerting (v9+) webhook
			// Example: curl -X POST http://localhost:8080/api/v1/webhooks/grafana -d @grafana-payload.json
			grafanaHandler := handlers.NewWebhookHandler(&webhooks.GrafanaProvider{}, alertSvc)
			webhooksGroup.POST("/grafana", grafanaHandler.Handle)

			// CloudWatch alarms via SNS (handles subscription confirmation automatically)
			// Example: Configure SNS topic subscription with this endpoint URL
			cloudwatchHandler := handlers.NewWebhookHandler(&webhooks.CloudWatchProvider{}, alertSvc)
			webhooksGroup.POST("/cloudwatch", cloudwatchHandler.Handle)

			// Generic webhook with optional HMAC authentication
			// Supports any custom monitoring tool or script
			// Example: curl -X POST http://localhost:8080/api/v1/webhooks/generic \
			//            -H "Content-Type: application/json" \
			//            -d '{"alerts":[{"title":"High CPU on web-01"}]}'
			webhookSecret := os.Getenv("WEBHOOK_SECRET") // Optional HMAC secret
			genericHandler := handlers.NewWebhookHandler(&webhooks.GenericProvider{WebhookSecret: webhookSecret}, alertSvc)
			webhooksGroup.POST("/generic", genericHandler.Handle)

			// Generic webhook JSON Schema endpoint (self-documenting API)
			// Example: curl http://localhost:8080/api/v1/webhooks/generic/schema
			webhooksGroup.GET("/generic/schema", genericHandler.HandleSchema)

			// Teams Bot Framework webhook (v0.8+)
			// Microsoft routes all bot messages here. Protected by Bot Framework JWT auth.
			// Configure this URL as the bot endpoint in your Azure Bot registration.
			if teamsSvc != nil {
				teamsEventHandler := services.NewTeamsEventHandler(cfg.TeamsAppID, incidentSvc, incidentRepo, timelineRepo, teamsSvc)
				teamsGroup := webhooksGroup.Group("/teams", middleware.TeamsAuth(cfg.TeamsAppID))
				teamsGroup.POST("", handlers.TeamsWebhook(teamsEventHandler))
			}
		}

		// Local login/logout endpoints (always open — these ARE the auth actions)
		if localAuth != nil {
			v1.POST("/auth/login", middleware.RateLimitAuth(), handlers.Login(localAuth))
			// Bootstrap: allow first user creation without auth (only works when user count is 0)
			v1.POST("/auth/bootstrap", middleware.RateLimitAuth(), handlers.CreateFirstUser(localAuth))
		}
		// Logout is always registered — handles both local and SAML sessions.
		// POST is the safe form (SPA calls this via fetch); the GET at /auth/logout
		// is kept for SAML redirect flows and deep-link compatibility.
		v1.POST("/auth/logout", handlers.APILogout(samlMiddleware, localAuth))

		// Auth identity endpoint — no RequireAuth so unauthenticated callers can
		// read ssoEnabled to show the SSO button. InjectSAMLSession populates the
		// SAML session in context without aborting, so authenticated SAML users
		// are also correctly identified.
		v1.GET("/auth/me",
			middleware.InjectSAMLSession(samlMiddleware),
			handlers.GetCurrentUser(localAuth, samlMiddleware != nil),
		)

		// Protected routes — require session (no-op when auth disabled).
		// RBAC middleware runs after auth; the OSS no-op allows all requests through.
		protected := v1.Group("", middleware.RequireAuth(samlMiddleware, localAuth), hooks.RBAC.Middleware("api", "access"), middleware.RateLimitAPI())

		// Incidents
		protected.GET("/incidents", handlers.ListIncidents(incidentSvc))
		protected.GET("/incidents/:id", handlers.GetIncident(incidentSvc))
		protected.POST("/incidents", handlers.CreateIncident(incidentSvc))
		protected.PATCH("/incidents/:id", handlers.UpdateIncident(incidentSvc))
		protected.GET("/incidents/:id/timeline", handlers.GetIncidentTimeline(incidentSvc))
		protected.POST("/incidents/:id/timeline", handlers.CreateTimelineEntry(incidentSvc))

		// Alerts
		protected.GET("/alerts", handlers.ListAlerts(alertRepo))
		protected.GET("/alerts/:id", handlers.GetAlert(alertRepo))
		protected.POST("/alerts/:id/acknowledge", handlers.AcknowledgeAlert(alertRepo, escalationEngine, incidentRepo, timelineRepo))

		// AI (v0.6+)
		protected.POST("/incidents/:id/summarize", handlers.SummarizeIncident(incidentSvc, aiSvc))
		protected.POST("/incidents/:id/handoff-digest", handlers.GenerateHandoffDigest(incidentSvc, aiSvc))
		protected.GET("/settings/ai", handlers.GetAISettings(aiSvc))
		protected.GET("/settings/teams", handlers.GetTeamsSettings(teamsSvc))

		// Post-Mortem Templates (v0.7+)
		protected.GET("/post-mortem-templates", handlers.ListPostMortemTemplates(postMortemSvc))
		protected.POST("/post-mortem-templates", handlers.CreatePostMortemTemplate(postMortemSvc))
		protected.GET("/post-mortem-templates/:id", handlers.GetPostMortemTemplate(postMortemSvc))
		protected.PATCH("/post-mortem-templates/:id", handlers.UpdatePostMortemTemplate(postMortemSvc))
		protected.DELETE("/post-mortem-templates/:id", handlers.DeletePostMortemTemplate(postMortemSvc))

		// Post-Mortems (v0.7+)
		protected.GET("/incidents/:id/postmortem", handlers.GetPostMortem(incidentSvc, postMortemSvc))
		protected.POST("/incidents/:id/postmortem/generate", handlers.GeneratePostMortem(incidentSvc, postMortemSvc, aiSvc))
		protected.PATCH("/incidents/:id/postmortem", handlers.UpdatePostMortem(incidentSvc, postMortemSvc))
		protected.GET("/incidents/:id/postmortem/export", handlers.ExportPostMortem(incidentSvc, postMortemSvc))
		protected.POST("/incidents/:id/postmortem/action-items", handlers.CreateActionItem(incidentSvc, postMortemSvc))
		protected.PATCH("/incidents/:id/postmortem/action-items/:itemId", handlers.UpdateActionItem(incidentSvc, postMortemSvc))
		protected.DELETE("/incidents/:id/postmortem/action-items/:itemId", handlers.DeleteActionItem(incidentSvc, postMortemSvc))

		// Grouping Rules (v0.3)
		protected.GET("/grouping-rules", handlers.ListGroupingRules(groupingRuleRepo))
		protected.GET("/grouping-rules/:id", handlers.GetGroupingRule(groupingRuleRepo))
		onGroupingRuleMutate := func() { groupingEngine.RefreshRules() }
		protected.POST("/grouping-rules", handlers.CreateGroupingRule(groupingRuleRepo, onGroupingRuleMutate))
		protected.PUT("/grouping-rules/:id", handlers.UpdateGroupingRule(groupingRuleRepo, onGroupingRuleMutate))
		protected.DELETE("/grouping-rules/:id", handlers.DeleteGroupingRule(groupingRuleRepo, onGroupingRuleMutate))

		// Routing Rules (v0.3)
		protected.GET("/routing-rules", handlers.ListRoutingRules(routingRuleRepo))
		protected.GET("/routing-rules/:id", handlers.GetRoutingRule(routingRuleRepo))
		onRoutingRuleMutate := func() { routingEngine.RefreshRules() }
		protected.POST("/routing-rules", handlers.CreateRoutingRule(routingRuleRepo, onRoutingRuleMutate))
		protected.PATCH("/routing-rules/:id", handlers.UpdateRoutingRule(routingRuleRepo, onRoutingRuleMutate))
		protected.DELETE("/routing-rules/:id", handlers.DeleteRoutingRule(routingRuleRepo, onRoutingRuleMutate))

		// Schedules (v0.4)
		protected.GET("/schedules", handlers.ListSchedules(scheduleRepo))
		protected.POST("/schedules", handlers.CreateSchedule(scheduleRepo))
		protected.GET("/schedules/:id", handlers.GetSchedule(scheduleRepo))
		protected.PATCH("/schedules/:id", handlers.UpdateSchedule(scheduleRepo))
		protected.DELETE("/schedules/:id", handlers.DeleteSchedule(scheduleRepo))

		protected.POST("/schedules/:id/layers", handlers.CreateLayer(scheduleRepo))
		protected.DELETE("/schedules/:id/layers/:layer_id", handlers.DeleteLayer(scheduleRepo))

		protected.GET("/schedules/:id/oncall", handlers.GetOnCall(scheduleRepo, scheduleEvaluator))
		protected.GET("/schedules/:id/oncall/timeline", handlers.GetOnCallTimeline(scheduleEvaluator))

		protected.GET("/schedules/:id/overrides", handlers.ListOverrides(scheduleRepo))
		protected.POST("/schedules/:id/overrides", handlers.CreateOverride(scheduleRepo))
		protected.DELETE("/schedules/:id/overrides/:override_id", handlers.DeleteOverride(scheduleRepo))

		// Escalation Policies (v0.5)
		protected.GET("/escalation-policies", handlers.ListEscalationPolicies(escalationPolicyRepo))
		protected.POST("/escalation-policies", handlers.CreateEscalationPolicy(escalationPolicyRepo))
		protected.GET("/escalation-policies/:id", handlers.GetEscalationPolicy(escalationPolicyRepo))
		protected.PATCH("/escalation-policies/:id", handlers.UpdateEscalationPolicy(escalationPolicyRepo))
		protected.DELETE("/escalation-policies/:id", handlers.DeleteEscalationPolicy(escalationPolicyRepo))

		protected.POST("/escalation-policies/:id/tiers", handlers.CreateEscalationTier(escalationPolicyRepo))
		protected.PATCH("/escalation-policies/:id/tiers/:tier_id", handlers.UpdateEscalationTier(escalationPolicyRepo))
		protected.DELETE("/escalation-policies/:id/tiers/:tier_id", handlers.DeleteEscalationTier(escalationPolicyRepo))

		// Settings — admin only
		settingsGroup := protected.Group("/settings", middleware.RequireAdmin())
		{
			settingsGroup.GET("/users", handlers.ListUsers(localAuth))
			settingsGroup.POST("/users", handlers.CreateUser(localAuth))
			settingsGroup.PATCH("/users/:id", handlers.UpdateUser(localAuth))
			settingsGroup.DELETE("/users/:id", handlers.DeactivateUser(localAuth))
			settingsGroup.POST("/users/:id/reset-password", handlers.ResetUserPassword(localAuth))
		}
	}
}
