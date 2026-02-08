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

	// Initialize services
	incidentSvc := services.NewIncidentService(incidentRepo, timelineRepo, alertRepo, chatService, db, cfg.SlackAutoInviteUserIDs)
	alertSvc := services.NewAlertService(alertRepo, incidentSvc)

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
		webhooks := v1.Group("/webhooks")
		webhooks.Use(middleware.BodySizeLimit(middleware.WebhookMaxBodySize))
		{
			// Prometheus Alertmanager webhook (v0.1)
			webhooks.POST("/prometheus", handlers.PrometheusWebhook(alertSvc))

			// Future webhooks (to be implemented in v0.3)
			webhooks.POST("/grafana", func(c *gin.Context) {
				c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
			})
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
	}
}
