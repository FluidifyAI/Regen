package webhooks

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// GenericWebhookPayload represents the Fluidify Regen-native webhook format.
//
// This is a simple, documented format for teams that want to send alerts from:
//   - Custom monitoring scripts
//   - AWS Lambda functions
//   - Internal tools and services
//   - Manual curl commands for testing
//
// Design principles:
//   - Only "title" is required
//   - All other fields have sensible defaults
//   - external_id is auto-generated if not provided
//   - Schema is self-documenting (GET /webhooks/generic/schema)
type GenericWebhookPayload struct {
	Alerts []GenericAlert `json:"alerts" binding:"required,min=1,max=100,dive"` // Array of alerts (1-100)
}

// GenericAlert represents a single alert in the generic webhook format.
//
// Minimal example:
//   {"title": "High CPU on web-01"}
//
// Full example:
//   {
//     "title": "High Error Rate",
//     "description": "Error rate > 5% on api-gateway",
//     "severity": "critical",
//     "status": "firing",
//     "external_id": "custom-123",
//     "labels": {"service": "api-gateway", "env": "production"},
//     "annotations": {"runbook_url": "https://wiki.example.com/runbooks/error-rate"},
//     "started_at": "2024-01-01T00:00:00Z",
//     "ended_at": "2024-01-01T00:05:00Z"
//   }
type GenericAlert struct {
	// Title is the only required field - a short summary of the alert
	Title string `json:"title" binding:"required,min=1,max=500"`

	// Description provides detailed context (optional)
	Description string `json:"description" binding:"max=5000"`

	// Severity must be "critical", "warning", or "info" (defaults to "warning")
	Severity string `json:"severity" binding:"omitempty,oneof=critical warning info"`

	// Status must be "firing" or "resolved" (defaults to "firing")
	Status string `json:"status" binding:"omitempty,oneof=firing resolved"`

	// ExternalID is a unique identifier for deduplication (auto-generated if not provided)
	// If provided, it must be unique within this source. If omitted, we generate a stable
	// ID from SHA256(title + sorted labels).
	ExternalID string `json:"external_id" binding:"omitempty,max=255"`

	// Labels are key-value pairs for filtering, grouping, and routing (optional)
	Labels map[string]string `json:"labels"`

	// Annotations are additional metadata not used for routing (optional)
	Annotations map[string]string `json:"annotations"`

	// StartedAt is when the alert started firing (defaults to current time)
	StartedAt *time.Time `json:"started_at"`

	// EndedAt is when the alert was resolved (nil for firing alerts)
	// Only valid if Status == "resolved"
	EndedAt *time.Time `json:"ended_at"`
}

// GenericProvider implements WebhookProvider for the Fluidify Regen-native generic webhook format.
//
// Authentication (optional): HMAC-SHA256 signature verification
//   - Server-side secret configured via WEBHOOK_SECRET environment variable
//   - Client sends signature in X-Webhook-Signature header
//   - Signature = HMAC-SHA256(webhook_secret, request_body)
//   - Format: "sha256=<hex-encoded-signature>"
//
// If no webhook secret is configured, authentication is disabled (URL secrecy only).
type GenericProvider struct {
	// WebhookSecret is the shared secret for HMAC verification (optional)
	// If empty, signature verification is disabled
	WebhookSecret string
}

// Source returns "generic"
func (g *GenericProvider) Source() string {
	return "generic"
}

// ValidatePayload verifies the HMAC-SHA256 signature if a webhook secret is configured.
//
// Header format: X-Webhook-Signature: sha256=<hex-encoded-hmac>
//
// Algorithm:
//  1. Extract signature from X-Webhook-Signature header
//  2. Compute HMAC-SHA256(webhook_secret, request_body)
//  3. Compare computed signature with provided signature (constant-time comparison)
//
// Returns nil if:
//   - No webhook secret configured (signature verification disabled)
//   - Signature is valid
//
// Returns error if:
//   - Webhook secret is configured but signature header is missing
//   - Signature format is invalid (not "sha256=...")
//   - Signature does not match
func (g *GenericProvider) ValidatePayload(body []byte, headers http.Header) error {
	// If no webhook secret configured, skip validation
	if g.WebhookSecret == "" {
		return nil
	}

	// Extract signature from header
	signatureHeader := headers.Get("X-Webhook-Signature")
	if signatureHeader == "" {
		return fmt.Errorf("missing X-Webhook-Signature header (webhook secret is configured)")
	}

	// Parse signature format: "sha256=<hex-signature>"
	if len(signatureHeader) < 7 || signatureHeader[:7] != "sha256=" {
		return fmt.Errorf("invalid signature format (expected 'sha256=<hex>')")
	}
	providedSignature := signatureHeader[7:]

	// Compute expected signature
	mac := hmac.New(sha256.New, []byte(g.WebhookSecret))
	mac.Write(body)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	// Constant-time comparison to prevent timing attacks
	if !hmac.Equal([]byte(providedSignature), []byte(expectedSignature)) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

// ParsePayload converts a generic webhook payload to NormalizedAlerts.
//
// This method applies sensible defaults:
//   - external_id: Generated from SHA256(title + labels) if not provided
//   - severity: "warning" if not provided
//   - status: "firing" if not provided
//   - started_at: Current time if not provided
//   - labels: Empty map if not provided
//   - annotations: Empty map if not provided
func (g *GenericProvider) ParsePayload(body []byte) ([]NormalizedAlert, error) {
	var payload GenericWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("invalid generic webhook payload: %w", err)
	}

	if len(payload.Alerts) == 0 {
		return nil, fmt.Errorf("generic webhook payload contains no alerts")
	}

	alerts := make([]NormalizedAlert, 0, len(payload.Alerts))
	for i, genericAlert := range payload.Alerts {
		normalized, err := g.normalizeGenericAlert(&genericAlert)
		if err != nil {
			return nil, fmt.Errorf("failed to normalize alert at index %d: %w", i, err)
		}
		alerts = append(alerts, *normalized)
	}

	return alerts, nil
}

// normalizeGenericAlert converts a single generic alert to NormalizedAlert.
//
// Applies defaults for all optional fields to ensure valid NormalizedAlert output.
func (g *GenericProvider) normalizeGenericAlert(genericAlert *GenericAlert) (*NormalizedAlert, error) {
	// Title is required (validated by binding tag, but check anyway)
	if genericAlert.Title == "" {
		return nil, fmt.Errorf("title is required")
	}

	// Apply severity default and validate
	severity := genericAlert.Severity
	if severity == "" {
		severity = "warning"
	} else {
		// Validate severity is one of allowed values
		if severity != "critical" && severity != "warning" && severity != "info" {
			return nil, fmt.Errorf("invalid severity %q: must be critical, warning, or info", severity)
		}
	}

	// Apply status default and validate
	status := genericAlert.Status
	if status == "" {
		status = "firing"
	} else {
		// Validate status is one of allowed values
		if status != "firing" && status != "resolved" {
			return nil, fmt.Errorf("invalid status %q: must be firing or resolved", status)
		}
	}

	// Validate status/ended_at consistency
	if status == "firing" && genericAlert.EndedAt != nil {
		return nil, fmt.Errorf("ended_at cannot be set for firing alerts")
	}

	// Apply started_at default
	startedAt := time.Now()
	if genericAlert.StartedAt != nil {
		startedAt = *genericAlert.StartedAt
	}

	// Initialize empty maps if not provided
	labels := genericAlert.Labels
	if labels == nil {
		labels = make(map[string]string)
	}

	annotations := genericAlert.Annotations
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// Generate external_id if not provided
	externalID := genericAlert.ExternalID
	if externalID == "" {
		externalID = GenerateExternalID(genericAlert.Title, labels)
	}

	// Marshal the individual alert as raw payload
	rawPayloadBytes, err := json.Marshal(genericAlert)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal generic alert for raw payload: %w", err)
	}

	return &NormalizedAlert{
		ExternalID:  externalID,
		Source:      "generic",
		Status:      status,
		Severity:    severity,
		Title:       genericAlert.Title,
		Description: genericAlert.Description,
		Labels:      labels,
		Annotations: annotations,
		RawPayload:  json.RawMessage(rawPayloadBytes),
		StartedAt:   startedAt,
		EndedAt:     genericAlert.EndedAt,
	}, nil
}

// GetJSONSchema returns a JSON Schema document describing the generic webhook format.
//
// This endpoint is served at GET /api/v1/webhooks/generic/schema to provide
// self-documenting API for users integrating with the generic webhook.
//
// The schema can be used for:
//   - Documentation generation
//   - Client-side validation
//   - IDE autocomplete
func GetJSONSchema() map[string]interface{} {
	return map[string]interface{}{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"$id":     "https://fluidifyregen.io/schemas/generic-webhook.json",
		"title":   "Fluidify Regen Generic Webhook",
		"description": "Schema for sending alerts to Fluidify Regen via the generic webhook endpoint",
		"type": "object",
		"required": []string{"alerts"},
		"properties": map[string]interface{}{
			"alerts": map[string]interface{}{
				"type": "array",
				"description": "Array of 1-100 alerts to create/update",
				"minItems": 1,
				"maxItems": 100,
				"items": map[string]interface{}{
					"type": "object",
					"required": []string{"title"},
					"properties": map[string]interface{}{
						"title": map[string]interface{}{
							"type": "string",
							"description": "Short summary of the alert (required)",
							"minLength": 1,
							"maxLength": 500,
							"examples": []string{"High CPU on web-01", "API error rate exceeded"},
						},
						"description": map[string]interface{}{
							"type": "string",
							"description": "Detailed context about the alert (optional)",
							"maxLength": 5000,
							"examples": []string{"CPU utilization is 95%, threshold is 80%"},
						},
						"severity": map[string]interface{}{
							"type": "string",
							"description": "Alert severity (defaults to 'warning')",
							"enum": []string{"critical", "warning", "info"},
							"default": "warning",
						},
						"status": map[string]interface{}{
							"type": "string",
							"description": "Alert status (defaults to 'firing')",
							"enum": []string{"firing", "resolved"},
							"default": "firing",
						},
						"external_id": map[string]interface{}{
							"type": "string",
							"description": "Unique identifier for deduplication. Auto-generated from title+labels if not provided.",
							"maxLength": 255,
							"examples": []string{"custom-123", "prod-api-cpu-alert-001"},
						},
						"labels": map[string]interface{}{
							"type": "object",
							"description": "Key-value pairs for filtering, grouping, and routing",
							"additionalProperties": map[string]interface{}{"type": "string"},
							"examples": []interface{}{
								map[string]string{"service": "api-gateway", "env": "production", "team": "backend"},
							},
						},
						"annotations": map[string]interface{}{
							"type": "object",
							"description": "Additional metadata not used for routing",
							"additionalProperties": map[string]interface{}{"type": "string"},
							"examples": []interface{}{
								map[string]string{"runbook_url": "https://wiki.example.com/runbooks/cpu"},
							},
						},
						"started_at": map[string]interface{}{
							"type": "string",
							"format": "date-time",
							"description": "When the alert started firing (ISO 8601). Defaults to current time.",
							"examples": []string{"2024-01-01T00:00:00Z"},
						},
						"ended_at": map[string]interface{}{
							"type": "string",
							"format": "date-time",
							"description": "When the alert was resolved (ISO 8601). Only valid if status is 'resolved'.",
							"examples": []string{"2024-01-01T00:05:00Z"},
						},
					},
				},
			},
		},
		"examples": []interface{}{
			// Minimal example
			map[string]interface{}{
				"alerts": []interface{}{
					map[string]interface{}{
						"title": "High CPU on web-01",
					},
				},
			},
			// Full example
			map[string]interface{}{
				"alerts": []interface{}{
					map[string]interface{}{
						"title":       "High Error Rate",
						"description": "Error rate > 5% on api-gateway",
						"severity":    "critical",
						"status":      "firing",
						"external_id": "custom-123",
						"labels": map[string]string{
							"service": "api-gateway",
							"env":     "production",
							"team":    "backend",
						},
						"annotations": map[string]string{
							"runbook_url": "https://wiki.example.com/runbooks/error-rate",
						},
						"started_at": "2024-01-01T00:00:00Z",
					},
				},
			},
		},
	}
}
