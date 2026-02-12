package webhooks

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// AlertmanagerPayload represents the top-level webhook payload sent by Prometheus Alertmanager
// Spec: https://prometheus.io/docs/alerting/latest/configuration/#webhook_config
type AlertmanagerPayload struct {
	Version  string              `json:"version" binding:"max=10"`                          // Alertmanager version (e.g., "4")
	GroupKey string              `json:"groupKey" binding:"max=500"`                        // Unique key for alert group
	Status   string              `json:"status" binding:"required,oneof=firing resolved"`   // "firing" or "resolved"
	Receiver string              `json:"receiver" binding:"max=200"`                        // Name of configured receiver
	Alerts   []AlertmanagerAlert `json:"alerts" binding:"required,min=1,max=100,dive"`      // Array of alerts in this notification (max 100 per webhook)
}

// AlertmanagerAlert represents a single alert within an Alertmanager webhook payload
type AlertmanagerAlert struct {
	Status       string            `json:"status" binding:"required,oneof=firing resolved"` // "firing" or "resolved"
	Labels       map[string]string `json:"labels" binding:"required"`                       // Alert labels (alertname, severity, instance, etc.)
	Annotations  map[string]string `json:"annotations"`                                     // Alert annotations (summary, description, etc.)
	StartsAt     time.Time         `json:"startsAt" binding:"required"`                     // ISO8601 timestamp when alert started
	EndsAt       time.Time         `json:"endsAt"`                                          // ISO8601 timestamp when alert ended (or zero for firing)
	GeneratorURL string            `json:"generatorURL" binding:"max=2048"`                 // Link to Prometheus expression browser
	Fingerprint  string            `json:"fingerprint" binding:"required,max=64"`           // Unique identifier for this alert (used for deduplication)
}

// PrometheusProvider implements WebhookProvider for Prometheus Alertmanager webhooks.
//
// Prometheus uses webhook URL secrecy for authentication (no signature verification).
// The fingerprint field provides stable deduplication across alert fires/resolves.
//
// Field Mapping:
//   - Title: labels["alertname"]
//   - Description: annotations["summary"] or annotations["description"]
//   - Severity: labels["severity"] (defaults to "warning" if missing)
//   - Status: "firing" or "resolved"
//   - ExternalID: fingerprint field
//   - Labels: All labels from Alertmanager
//   - Annotations: All annotations from Alertmanager
type PrometheusProvider struct{}

// Source returns "prometheus"
func (p *PrometheusProvider) Source() string {
	return "prometheus"
}

// ValidatePayload performs no validation for Prometheus.
//
// Prometheus Alertmanager does not sign webhook payloads. Security relies on:
//   - Webhook URL secrecy (long random path)
//   - Network-level access controls (firewall, VPN)
//   - Optional TLS client certificates (not implemented here)
func (p *PrometheusProvider) ValidatePayload(body []byte, headers http.Header) error {
	return nil // No validation for Prometheus
}

// ParsePayload converts an Alertmanager webhook payload to NormalizedAlerts.
//
// This method contains all Prometheus-specific field mapping logic that was previously
// in alert_service.normalizeAlert(). Now it's encapsulated in the provider.
func (p *PrometheusProvider) ParsePayload(body []byte) ([]NormalizedAlert, error) {
	var payload AlertmanagerPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("invalid alertmanager payload: %w", err)
	}

	if len(payload.Alerts) == 0 {
		return nil, fmt.Errorf("alertmanager payload contains no alerts")
	}

	alerts := make([]NormalizedAlert, 0, len(payload.Alerts))
	for _, amAlert := range payload.Alerts {
		normalized, err := p.normalizePrometheusAlert(&amAlert)
		if err != nil {
			return nil, fmt.Errorf("failed to normalize alert %s: %w", amAlert.Fingerprint, err)
		}
		alerts = append(alerts, *normalized)
	}

	return alerts, nil
}

// normalizePrometheusAlert converts a single Alertmanager alert to NormalizedAlert.
//
// This is the Prometheus-specific mapping logic extracted from alert_service.go.
func (p *PrometheusProvider) normalizePrometheusAlert(amAlert *AlertmanagerAlert) (*NormalizedAlert, error) {
	// Extract title from alertname label
	title := amAlert.Labels["alertname"]
	if title == "" {
		title = "Unknown Alert"
	}

	// Extract description from annotations (prefer summary, fall back to description)
	description := amAlert.Annotations["summary"]
	if description == "" {
		description = amAlert.Annotations["description"]
	}

	// Parse severity from labels (will default to "warning" in normalizedAlertToModel)
	severity := amAlert.Labels["severity"]
	if severity == "" {
		severity = "warning"
	}

	// Determine status
	status := "firing"
	if amAlert.Status == "resolved" {
		status = "resolved"
	}

	// Handle ended_at (only set if alert is resolved and has valid timestamp)
	var endedAt *time.Time
	if status == "resolved" && !amAlert.EndsAt.IsZero() {
		// Alertmanager sends 0001-01-01 for firing alerts
		if amAlert.EndsAt.Year() > 1900 {
			endedAt = &amAlert.EndsAt
		}
	}

	// Marshal the individual alert as raw payload
	rawPayloadBytes, err := json.Marshal(amAlert)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal alert for raw payload: %w", err)
	}

	return &NormalizedAlert{
		ExternalID:  amAlert.Fingerprint,
		Source:      "prometheus",
		Status:      status,
		Severity:    severity,
		Title:       title,
		Description: description,
		Labels:      amAlert.Labels,
		Annotations: amAlert.Annotations,
		RawPayload:  json.RawMessage(rawPayloadBytes),
		StartedAt:   amAlert.StartsAt,
		EndedAt:     endedAt,
	}, nil
}
