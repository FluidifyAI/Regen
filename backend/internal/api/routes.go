package api

import (
	"log/slog"
	"net/http"
	"net/url"
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
	"github.com/openincident/openincident/ui"
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
	systemSettingsRepo := repository.NewSystemSettingsRepository(db)
	userRepo := repository.NewUserRepository(db)
	slackConfigRepo := repository.NewSlackConfigRepository(db)
	teamsConfigRepo := repository.NewTeamsConfigRepository(db)
	telegramConfigRepo := repository.NewTelegramConfigRepository(db)

	// Slack is initialized lazily: config is read from the DB on each use and
	// cached until the bot_token changes. This means Slack can be configured or
	// updated through the Integrations page without restarting the server.
	chatService := services.NewLazySlackService(slackConfigRepo)

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
		// Check if a key was previously saved via the UI
		if dbKey, err := systemSettingsRepo.GetString(repository.KeyOpenAIAPIKey); err == nil && dbKey != "" {
			aiSvc.Reload(dbKey)
			slog.Info("AI service enabled from DB key", "model", cfg.OpenAIModel)
		} else {
			slog.Warn("AI service disabled — set OPENAI_API_KEY or configure via Settings → System")
		}
	}

	// Post-mortem repositories (v0.7+)
	postMortemTemplateRepo := repository.NewPostMortemTemplateRepository(db)
	pmRepo := repository.NewPostMortemRepository(db)

	// Initialize services
	incidentSvc := services.NewIncidentService(incidentRepo, timelineRepo, alertRepo, chatService, db)
	services.SetAIService(incidentSvc, aiSvc)
	services.SetCommanderDeps(incidentSvc, userRepo, scheduleRepo, scheduleEvaluator)
	alertSvc := services.NewAlertService(alertRepo, incidentSvc)
	alertSvc.SetGroupingEngine(groupingEngine)
	alertSvc.SetRoutingEngine(routingEngine)
	alertSvc.SetEscalationEngine(escalationEngine)
	alertSvc.SetEscalationRepos(escalationPolicyRepo, systemSettingsRepo)

	// Post-mortem service (v0.7+)
	commentRepo := repository.NewPostMortemCommentRepository(db)
	postMortemSvc := services.NewPostMortemService(pmRepo, postMortemTemplateRepo, commentRepo, incidentSvc, aiSvc)

	// Wire Teams service into incident service (v0.8+).
	// teamsSvc is constructed once in serve.go and injected here.
	if teamsSvc != nil {
		services.SetTeamsService(incidentSvc, teamsSvc)
	}

	// Bootstrap Telegram service from DB config (optional)
	if tgCfg, err := telegramConfigRepo.Get(); err == nil && tgCfg != nil && tgCfg.BotToken != "" {
		if tgSvc := services.NewTelegramServiceFromConfig(tgCfg, cfg.FrontendURL); tgSvc != nil {
			services.SetTelegramService(incidentSvc, tgSvc)
		}
	}

	// Start Slack Socket Mode event handler (bidirectional sync) if app token is configured.
	if slackCfgForSocket, err := slackConfigRepo.Get(); err == nil &&
		slackCfgForSocket != nil && slackCfgForSocket.AppToken != "" && chatService != nil {
		eventHandler, err := services.NewSlackEventHandler(
			slackCfgForSocket.AppToken,
			slackCfgForSocket.BotToken,
			incidentSvc,
			chatService,
			userRepo,
		)
		if err != nil {
			slog.Error("failed to initialize slack socket mode", "error", err)
			slog.Warn("bidirectional Slack sync disabled - Slack will be one-way only")
		} else {
			eventHandler.Start()
		}
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
		// After SAML auth the IdP POSTs to /saml/acs, which redirects the browser
		// to the URI stored in the crewjam request tracker (set during
		// HandleStartAuthFlow). By default that URI is the /saml/login path on
		// the backend (port 8080) — which has no frontend to serve. We fix this
		// by cloning the request with the frontend URL before calling
		// HandleStartAuthFlow, so crewjam tracks the frontend root as the
		// post-auth destination. SAML_REDIRECT_URL should be the externally
		// reachable frontend URL (e.g. http://localhost:3000 in dev, https://app.example.com in prod).
		samlRedirectURL := os.Getenv("SAML_REDIRECT_URL")
		if samlRedirectURL == "" {
			samlRedirectURL = cfg.SAMLBaseURL // prod default: same origin serves the frontend
		}
		parsedRedirect, _ := url.Parse(samlRedirectURL)

		samlHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/saml/login" || r.URL.Path == "/saml/login/" {
				// Redirect to the frontend root after auth, not back to /saml/login.
				r2 := r.Clone(r.Context())
				r2.URL = &url.URL{Scheme: parsedRedirect.Scheme, Host: parsedRedirect.Host, Path: "/"}
				r2.Host = parsedRedirect.Host
				samlMiddleware.HandleStartAuthFlow(w, r2)
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

		// Slack OAuth login (always open — these ARE the login flow)
		v1.GET("/auth/slack", handlers.InitSlackOAuth(slackConfigRepo))
		v1.GET("/auth/slack/callback", handlers.SlackOAuthCallback(slackConfigRepo, localAuth))
		// Public: is Slack OAuth login enabled? (LoginPage uses this to show/hide the button)
		v1.GET("/auth/slack/config", handlers.GetSlackOAuthConfig(slackConfigRepo))

		// Local login/logout endpoints (always open — these ARE the auth actions)
		if localAuth != nil {
			v1.POST("/auth/login", middleware.RateLimitAuth(), handlers.Login(localAuth))
			v1.POST("/auth/login/setup-token", middleware.RateLimitAuth(), handlers.SetupTokenLogin(localAuth))
			// Bootstrap: allow first user creation without auth (only works when user count is 0)
			v1.POST("/auth/bootstrap", middleware.RateLimitAuth(), handlers.CreateFirstUser(localAuth))
			// Forgot password: generates a one-time reset link shown on-screen (no SMTP required)
			v1.POST("/auth/forgot-password", middleware.RateLimitAuth(), handlers.ForgotPassword(localAuth))
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
			middleware.InjectSAMLSession(samlMiddleware, localAuth),
			handlers.GetCurrentUser(localAuth, samlMiddleware != nil),
		)

		// Protected routes — require session (no-op when auth disabled).
		// RBAC middleware runs after auth; the OSS no-op allows all requests through.
		protected := v1.Group("", middleware.RequireAuth(samlMiddleware, localAuth), hooks.RBAC.Middleware("api", "access"), middleware.RateLimitAPI())

		// Update own profile (name/password) — requires auth but not admin
		protected.PATCH("/auth/me", handlers.UpdateMe(localAuth))

		// Users — available to all authenticated users for commander assignment
		protected.GET("/users", handlers.ListUsersForAssignment(userRepo))

		// Incidents
		protected.GET("/incidents", handlers.ListIncidents(incidentSvc))
		protected.GET("/incidents/:id", handlers.GetIncident(incidentSvc, userRepo))
		protected.POST("/incidents", handlers.CreateIncident(incidentSvc))
		protected.PATCH("/incidents/:id", handlers.UpdateIncident(incidentSvc))
		protected.GET("/incidents/:id/timeline", handlers.GetIncidentTimeline(incidentSvc, userRepo))
		protected.POST("/incidents/:id/timeline", handlers.CreateTimelineEntry(incidentSvc, userRepo))
		protected.POST("/incidents/:id/escalate", handlers.EscalateIncident(escalationEngine))

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
		protected.POST("/incidents/:id/postmortem", handlers.CreatePostMortem(incidentSvc, postMortemSvc))
		protected.POST("/incidents/:id/postmortem/enhance", handlers.EnhancePostMortem(incidentSvc, postMortemSvc, aiSvc))
		protected.GET("/incidents/:id/postmortem/comments", handlers.ListPostMortemComments(incidentSvc, postMortemSvc))
		protected.POST("/incidents/:id/postmortem/comments", handlers.CreatePostMortemComment(incidentSvc, postMortemSvc))
		protected.DELETE("/incidents/:id/postmortem/comments/:commentId", handlers.DeletePostMortemComment(incidentSvc, postMortemSvc))

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
		// Mutations are admin-only — members can view but not configure alert routing.
		protected.POST("/routing-rules", middleware.RequireAdmin(), handlers.CreateRoutingRule(routingRuleRepo, onRoutingRuleMutate))
		protected.PUT("/routing-rules/reorder", middleware.RequireAdmin(), handlers.ReorderRoutingRules(routingRuleRepo, onRoutingRuleMutate))
		protected.PATCH("/routing-rules/:id", middleware.RequireAdmin(), handlers.UpdateRoutingRule(routingRuleRepo, onRoutingRuleMutate))
		protected.DELETE("/routing-rules/:id", middleware.RequireAdmin(), handlers.DeleteRoutingRule(routingRuleRepo, onRoutingRuleMutate))

		// Schedules (v0.4)
		protected.GET("/schedules", handlers.ListSchedules(scheduleRepo))
		protected.GET("/schedules/:id", handlers.GetSchedule(scheduleRepo))
		protected.GET("/schedules/:id/oncall", handlers.GetOnCall(scheduleRepo, scheduleEvaluator))
		protected.GET("/schedules/:id/oncall/timeline", handlers.GetOnCallTimeline(scheduleEvaluator))
		protected.GET("/schedules/:id/layer-timelines", handlers.GetLayerTimelines(scheduleEvaluator))
		protected.GET("/schedules/:id/overrides", handlers.ListOverrides(scheduleRepo))

		// Schedule mutations — admin only
		protected.POST("/schedules", middleware.RequireAdmin(), handlers.CreateSchedule(scheduleRepo))
		protected.PATCH("/schedules/:id", middleware.RequireAdmin(), handlers.UpdateSchedule(scheduleRepo))
		protected.DELETE("/schedules/:id", middleware.RequireAdmin(), handlers.DeleteSchedule(scheduleRepo))
		protected.POST("/schedules/:id/layers", middleware.RequireAdmin(), handlers.CreateLayer(scheduleRepo))
		protected.PATCH("/schedules/:id/layers/:layer_id", middleware.RequireAdmin(), handlers.UpdateLayer(scheduleRepo))
		protected.DELETE("/schedules/:id/layers/:layer_id", middleware.RequireAdmin(), handlers.DeleteLayer(scheduleRepo))
		protected.POST("/schedules/:id/overrides", middleware.RequireAdmin(), handlers.CreateOverride(scheduleRepo))
		protected.DELETE("/schedules/:id/overrides/:override_id", middleware.RequireAdmin(), handlers.DeleteOverride(scheduleRepo))

		// Escalation Policies (v0.5) — reads open to all, mutations admin-only
		protected.GET("/escalation-policies", handlers.ListEscalationPolicies(escalationPolicyRepo))
		// Severity rules registered before /:id to prevent Gin matching "severity-rules" as :id.
		protected.GET("/escalation-policies/severity-rules", handlers.ListSeverityRules(escalationPolicyRepo))
		protected.GET("/escalation-policies/:id", handlers.GetEscalationPolicy(escalationPolicyRepo))

		protected.POST("/escalation-policies", middleware.RequireAdmin(), handlers.CreateEscalationPolicy(escalationPolicyRepo))
		protected.PUT("/escalation-policies/severity-rules/:severity", middleware.RequireAdmin(), handlers.UpsertSeverityRule(escalationPolicyRepo))
		protected.DELETE("/escalation-policies/severity-rules/:severity", middleware.RequireAdmin(), handlers.DeleteSeverityRule(escalationPolicyRepo))
		protected.PATCH("/escalation-policies/:id", middleware.RequireAdmin(), handlers.UpdateEscalationPolicy(escalationPolicyRepo))
		protected.DELETE("/escalation-policies/:id", middleware.RequireAdmin(), handlers.DeleteEscalationPolicy(escalationPolicyRepo))
		protected.POST("/escalation-policies/:id/tiers", middleware.RequireAdmin(), handlers.CreateEscalationTier(escalationPolicyRepo))
		protected.PATCH("/escalation-policies/:id/tiers/:tier_id", middleware.RequireAdmin(), handlers.UpdateEscalationTier(escalationPolicyRepo))
		protected.DELETE("/escalation-policies/:id/tiers/:tier_id", middleware.RequireAdmin(), handlers.DeleteEscalationTier(escalationPolicyRepo))

		// Agent management (AI agents — enable/disable)
		agentsHandler := handlers.NewAgentsHandler(userRepo)
		protected.GET("/agents", agentsHandler.List)
		protected.PATCH("/agents/:id/status", middleware.RequireAdmin(), agentsHandler.SetStatus)

		// Settings — admin only
		settingsGroup := protected.Group("/settings", middleware.RequireAdmin())
		{
			settingsGroup.GET("/users", handlers.ListUsers(localAuth))
			settingsGroup.POST("/users", handlers.CreateUser(localAuth))
			settingsGroup.PATCH("/users/:id", handlers.UpdateUser(localAuth))
			settingsGroup.DELETE("/users/:id", handlers.DeactivateUser(localAuth))
			settingsGroup.POST("/users/:id/reset-password", handlers.ResetUserPassword(localAuth))

			// Global escalation settings (global fallback policy)
			settingsGroup.GET("/escalation", handlers.GetEscalationSettings(systemSettingsRepo))
			settingsGroup.PUT("/escalation", handlers.UpdateEscalationSettings(systemSettingsRepo))

			// Slack integration config (admin only)
			settingsGroup.GET("/slack", handlers.GetSlackConfig(slackConfigRepo))
			settingsGroup.POST("/slack", handlers.SaveSlackConfig(slackConfigRepo))
			settingsGroup.POST("/slack/test", handlers.TestSlackConfig())
			settingsGroup.DELETE("/slack", handlers.DeleteSlackConfig(slackConfigRepo))
			settingsGroup.GET("/slack/members", handlers.ListSlackMembers(slackConfigRepo, userRepo))

			// Teams integration config (admin only)
			settingsGroup.GET("/teams/config", handlers.GetTeamsConfig(teamsConfigRepo))
			settingsGroup.PUT("/teams/config", handlers.SaveTeamsConfig(teamsConfigRepo))
			settingsGroup.POST("/teams/config/test", handlers.TestTeamsConfig())
			settingsGroup.DELETE("/teams/config", handlers.DeleteTeamsConfig(teamsConfigRepo))
			settingsGroup.GET("/teams/members", handlers.ListTeamsMembers(teamsConfigRepo, userRepo))

			// Telegram config (notification gateway)
			settingsGroup.GET("/telegram", handlers.GetTelegramConfig(telegramConfigRepo))
			settingsGroup.POST("/telegram", handlers.SaveTelegramConfig(telegramConfigRepo))
			settingsGroup.POST("/telegram/test", handlers.TestTelegramConfig())
			settingsGroup.POST("/telegram/fetch-chat-id", handlers.FetchTelegramChatID())
			settingsGroup.DELETE("/telegram", handlers.DeleteTelegramConfig(telegramConfigRepo))

			// System settings (OPE-26/27)
			settingsGroup.GET("/system", handlers.GetSystemSettings(systemSettingsRepo, aiSvc))
			settingsGroup.PATCH("/system", handlers.UpdateSystemSettings(systemSettingsRepo, aiSvc))
			settingsGroup.POST("/system/ai/test", handlers.TestOpenAIKey(systemSettingsRepo))
		}
	}

	// ── Embedded frontend (production) ───────────────────────────────────────
	// Serve the pre-built React SPA from the same origin as the API.
	// This eliminates CORS entirely for self-hosted deployments.
	//
	// ui.Files() returns nil when the frontend has not been built (e.g. local
	// development using `npm run dev`), in which case we skip static serving
	// so the API remains fully functional on its own.
	if staticFS := ui.Files(); staticFS != nil {
		slog.Info("serving embedded frontend")
		// Serve /assets/*, /favicon.ico, etc. directly (long cache headers are
		// set by Vite's build hashing so we can cache aggressively here).
		router.StaticFS("/assets", staticFS)
		router.StaticFileFS("/favicon.ico", "favicon.ico", staticFS)
		// SPA fallback: any path not matched above serves index.html so that
		// client-side routing (React Router) works on hard refresh / deep links.
		router.NoRoute(func(c *gin.Context) {
			c.FileFromFS("index.html", staticFS)
		})
	} else {
		slog.Info("no embedded frontend found — serving API only (use `npm run dev` for the UI)")
	}
}
