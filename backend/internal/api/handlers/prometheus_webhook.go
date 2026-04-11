package handlers

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/FluidifyAI/Regen/backend/internal/models/webhooks"
	"github.com/FluidifyAI/Regen/backend/internal/services"
)

// PrometheusWebhook handles incoming Prometheus Alertmanager webhooks
func PrometheusWebhook(alertSvc services.AlertService) gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		// Step 1: Parse JSON payload
		var payload webhooks.AlertmanagerPayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			slog.Error("invalid webhook payload", "error", err, "source", "prometheus")
			c.JSON(400, gin.H{
				"error":   "invalid payload",
				"details": err.Error(),
			})
			return
		}

		// Step 2: Validate basic requirements
		if len(payload.Alerts) == 0 {
			slog.Warn("webhook received with no alerts", "source", "prometheus")
			c.JSON(400, gin.H{
				"error": "no alerts in payload",
			})
			return
		}

		// Step 3: Process alerts through service layer
		result, err := alertSvc.ProcessAlertmanagerPayload(&payload)
		if err != nil {
			slog.Error("webhook processing failed",
				"error", err,
				"source", "prometheus",
				"payload_status", payload.Status,
			)
			c.JSON(500, gin.H{
				"error": "internal server error",
			})
			return
		}

		// Step 4: Log metrics with structured logging
		duration := time.Since(startTime)
		slog.Info("webhook processed",
			"source", "prometheus",
			"received", result.Received,
			"created", result.Created,
			"updated", result.Updated,
			"incidents_created", result.IncidentsCreated,
			"duration_ms", duration.Milliseconds(),
			"payload_status", payload.Status,
		)

		// Step 5: Return success response
		c.JSON(200, gin.H{
			"received":          result.Received,
			"incidents_created": result.IncidentsCreated,
		})
	}
}
