package api

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/openincident/openincident/internal/api/handlers"
	"github.com/openincident/openincident/internal/api/middleware"
	"github.com/openincident/openincident/internal/config"
	"github.com/openincident/openincident/internal/metrics"
	"github.com/openincident/openincident/internal/models/webhooks"
	"github.com/openincident/openincident/internal/repository"
	"github.com/openincident/openincident/internal/services"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"
)

// SetupRoutes configures all application routes
func SetupRoutes(router *gin.Engine, db *gorm.DB, cfg *config.Config) {
	// Initialize repositories
	alertRepo := repository.NewAlertRepository(db)
	incidentRepo := repository.NewIncidentRepository(db)
	timelineRepo := repository.NewTimelineRepository(db)
	groupingRuleRepo := repository.NewGroupingRuleRepository(db)
	routingRuleRepo := repository.NewRoutingRuleRepository(db)

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

	// Initialize services
	incidentSvc := services.NewIncidentService(incidentRepo, timelineRepo, alertRepo, chatService, db, cfg.SlackAutoInviteUserIDs)
	alertSvc := services.NewAlertService(alertRepo, incidentSvc)
	alertSvc.SetGroupingEngine(groupingEngine)
	alertSvc.SetRoutingEngine(routingEngine)

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
	router.Use(metrics.Middleware()) // Prometheus metrics

	// Health check endpoints
	router.GET("/health", handlers.Health(db))
	router.GET("/ready", handlers.Ready(db))

	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Webhooks (with 1MB body size limit)
		webhooksGroup := v1.Group("/webhooks")
		webhooksGroup.Use(middleware.BodySizeLimit(middleware.WebhookMaxBodySize))
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
		}

		// Incidents
		v1.GET("/incidents", handlers.ListIncidents(incidentSvc))
		v1.GET("/incidents/:id", handlers.GetIncident(incidentSvc))
		v1.POST("/incidents", handlers.CreateIncident(incidentSvc))
		v1.PATCH("/incidents/:id", handlers.UpdateIncident(incidentSvc))
		v1.GET("/incidents/:id/timeline", handlers.GetIncidentTimeline(incidentSvc))
		v1.POST("/incidents/:id/timeline", handlers.CreateTimelineEntry(incidentSvc))

		// Alerts (to be implemented)
		v1.GET("/alerts", func(c *gin.Context) {
			c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
		})

		// Grouping Rules (v0.3)
		v1.GET("/grouping-rules", handlers.ListGroupingRules(groupingRuleRepo))
		v1.GET("/grouping-rules/:id", handlers.GetGroupingRule(groupingRuleRepo))
		onGroupingRuleMutate := func() { groupingEngine.RefreshRules() }
		v1.POST("/grouping-rules", handlers.CreateGroupingRule(groupingRuleRepo, onGroupingRuleMutate))
		v1.PUT("/grouping-rules/:id", handlers.UpdateGroupingRule(groupingRuleRepo, onGroupingRuleMutate))
		v1.DELETE("/grouping-rules/:id", handlers.DeleteGroupingRule(groupingRuleRepo, onGroupingRuleMutate))

		// Routing Rules (v0.3)
		v1.GET("/routing-rules", handlers.ListRoutingRules(routingRuleRepo))
		v1.GET("/routing-rules/:id", handlers.GetRoutingRule(routingRuleRepo))
		onRoutingRuleMutate := func() { routingEngine.RefreshRules() }
		v1.POST("/routing-rules", handlers.CreateRoutingRule(routingRuleRepo, onRoutingRuleMutate))
		v1.PATCH("/routing-rules/:id", handlers.UpdateRoutingRule(routingRuleRepo, onRoutingRuleMutate))
		v1.DELETE("/routing-rules/:id", handlers.DeleteRoutingRule(routingRuleRepo, onRoutingRuleMutate))

		// Schedules (v0.4)
		v1.GET("/schedules", handlers.ListSchedules(scheduleRepo))
		v1.POST("/schedules", handlers.CreateSchedule(scheduleRepo))
		v1.GET("/schedules/:id", handlers.GetSchedule(scheduleRepo))
		v1.PATCH("/schedules/:id", handlers.UpdateSchedule(scheduleRepo))
		v1.DELETE("/schedules/:id", handlers.DeleteSchedule(scheduleRepo))

		v1.POST("/schedules/:id/layers", handlers.CreateLayer(scheduleRepo))
		v1.DELETE("/schedules/:id/layers/:layer_id", handlers.DeleteLayer(scheduleRepo))

		v1.GET("/schedules/:id/oncall", handlers.GetOnCall(scheduleRepo, scheduleEvaluator))
		v1.GET("/schedules/:id/oncall/timeline", handlers.GetOnCallTimeline(scheduleEvaluator))

		v1.GET("/schedules/:id/overrides", handlers.ListOverrides(scheduleRepo))
		v1.POST("/schedules/:id/overrides", handlers.CreateOverride(scheduleRepo))
		v1.DELETE("/schedules/:id/overrides/:override_id", handlers.DeleteOverride(scheduleRepo))
	}
}
