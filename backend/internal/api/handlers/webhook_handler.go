package handlers

import (
	"io"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openincident/openincident/internal/models/webhooks"
	"github.com/openincident/openincident/internal/services"
)

// WebhookHandler is a unified handler for all webhook providers (v0.3+).
//
// Instead of having separate handlers for Prometheus, Grafana, CloudWatch, and Generic,
// we use a single handler that delegates to the WebhookProvider interface. This makes
// the handler code source-agnostic.
//
// Flow:
//  1. Read raw request body (needed for signature validation)
//  2. Provider.ValidatePayload() - check authentication
//  3. Provider.ParsePayload() - normalize to []NormalizedAlert
//  4. AlertService.ProcessNormalizedAlerts() - create/update alerts and incidents
//  5. Return success response with statistics
//
// Each webhook route (e.g., POST /webhooks/grafana) instantiates this handler with
// the appropriate provider (e.g., &webhooks.GrafanaProvider{}).
type WebhookHandler struct {
	provider     webhooks.WebhookProvider
	alertService services.AlertService
}

// NewWebhookHandler creates a handler for a specific webhook provider.
//
// Example usage in routes.go:
//   grafanaHandler := NewWebhookHandler(&webhooks.GrafanaProvider{}, alertService)
//   router.POST("/webhooks/grafana", grafanaHandler.Handle)
func NewWebhookHandler(provider webhooks.WebhookProvider, alertService services.AlertService) *WebhookHandler {
	return &WebhookHandler{
		provider:     provider,
		alertService: alertService,
	}
}

// Handle processes an incoming webhook request from any monitoring source.
//
// This is the unified entry point for Prometheus, Grafana, CloudWatch, and Generic webhooks.
// The provider parameter determines how the payload is validated and parsed.
func (h *WebhookHandler) Handle(c *gin.Context) {
	startTime := time.Now()
	source := h.provider.Source()

	// Step 1: Read raw request body
	// We need the raw bytes for signature validation (some providers sign the body)
	// Use LimitReader to prevent memory exhaustion from malicious large payloads
	// Note: The middleware.BodySizeLimit already restricts this to 1MB for webhooks
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		slog.Error("failed to read request body",
			"error", err,
			"source", source,
		)
		c.JSON(400, gin.H{
			"error":  "failed to read request body",
			"source": source,
		})
		return
	}

	// Step 2: Validate authentication
	// This is provider-specific:
	//   - Prometheus: No validation (URL secrecy)
	//   - Grafana: No validation (URL secrecy)
	//   - CloudWatch: SNS signature verification
	//   - Generic: HMAC-SHA256 signature (if configured)
	if err := h.provider.ValidatePayload(body, c.Request.Header); err != nil {
		slog.Warn("webhook authentication failed",
			"error", err,
			"source", source,
			"remote_addr", c.ClientIP(),
		)
		c.JSON(401, gin.H{
			"error":  "authentication failed",
			"source": source,
			"detail": err.Error(),
		})
		return
	}

	// Step 3: Parse and normalize alerts
	// Provider converts source-specific format to []NormalizedAlert
	alerts, err := h.provider.ParsePayload(body)
	if err != nil {
		slog.Error("webhook payload parsing failed",
			"error", err,
			"source", source,
		)
		c.JSON(400, gin.H{
			"error":  "invalid payload",
			"source": source,
			"detail": err.Error(),
		})
		return
	}

	// Special case: CloudWatch SNS subscription confirmation returns empty alerts array
	// This is not an error - we confirmed the subscription and should return 200 OK
	if len(alerts) == 0 {
		duration := time.Since(startTime)
		slog.Info("webhook processed (no alerts)",
			"source", source,
			"duration_ms", duration.Milliseconds(),
			"note", "SNS subscription confirmation or empty batch",
		)
		c.JSON(200, gin.H{
			"source":            source,
			"received":          0,
			"created":           0,
			"updated":           0,
			"incidents_created": 0,
		})
		return
	}

	// Step 4: Process normalized alerts through service layer
	// This is where deduplication, grouping, routing, and incident creation happen
	result, err := h.alertService.ProcessNormalizedAlerts(alerts)
	if err != nil {
		slog.Error("webhook processing failed",
			"error", err,
			"source", source,
			"received", len(alerts),
		)
		c.JSON(500, gin.H{
			"error":  "processing failed",
			"source": source,
		})
		return
	}

	// Step 5: Log success metrics with structured logging
	duration := time.Since(startTime)
	slog.Info("webhook processed",
		"source", source,
		"received", result.Received,
		"created", result.Created,
		"updated", result.Updated,
		"incidents_created", result.IncidentsCreated,
		"duration_ms", duration.Milliseconds(),
	)

	// Step 6: Return success response with statistics
	c.JSON(200, gin.H{
		"source":            source,
		"received":          result.Received,
		"created":           result.Created,
		"updated":           result.Updated,
		"incidents_created": result.IncidentsCreated,
	})
}

// HandleSchema returns the JSON Schema for the generic webhook format.
//
// This is only used for the Generic webhook provider to provide self-documenting API.
// Served at GET /api/v1/webhooks/generic/schema
func (h *WebhookHandler) HandleSchema(c *gin.Context) {
	schema := webhooks.GetJSONSchema()
	c.JSON(200, schema)
}
