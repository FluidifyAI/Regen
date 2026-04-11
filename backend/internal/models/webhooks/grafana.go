package webhooks

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// GrafanaWebhookPayload represents the top-level webhook payload sent by Grafana Unified Alerting
// Spec: https://grafana.com/docs/grafana/latest/alerting/configure-notifications/manage-contact-points/integrations/webhook-notifier/
//
// Grafana Unified Alerting (v9+) intentionally uses a format similar to Prometheus Alertmanager
// for easier migration and interoperability. The main differences:
//   - Adds "values" field with query results
//   - May include "orgId" in some contexts
//   - Fingerprint generation may differ slightly from Prometheus
type GrafanaWebhookPayload struct {
	Receiver          string            `json:"receiver"`                                        // Name of contact point
	Status            string            `json:"status" binding:"required,oneof=firing resolved"` // "firing" or "resolved"
	Alerts            []GrafanaAlert    `json:"alerts" binding:"required,min=1,max=100,dive"`    // Array of alerts (max 100)
	GroupLabels       map[string]string `json:"groupLabels"`                                     // Labels used for grouping
	CommonLabels      map[string]string `json:"commonLabels"`                                    // Labels common to all alerts
	CommonAnnotations map[string]string `json:"commonAnnotations"`                               // Annotations common to all alerts
	ExternalURL       string            `json:"externalURL"`                                     // Base URL of Grafana instance
}

// GrafanaAlert represents a single alert within a Grafana webhook payload
type GrafanaAlert struct {
	Status       string            `json:"status" binding:"required,oneof=firing resolved"` // "firing" or "resolved"
	Labels       map[string]string `json:"labels" binding:"required"`                       // Alert labels (alertname, grafana_folder, etc.)
	Annotations  map[string]string `json:"annotations"`                                     // Alert annotations (summary, description, etc.)
	StartsAt     time.Time         `json:"startsAt" binding:"required"`                     // When alert started firing
	EndsAt       time.Time         `json:"endsAt"`                                          // When alert resolved (or zero for firing)
	GeneratorURL string            `json:"generatorURL"`                                    // Link to alert rule in Grafana
	Fingerprint  string            `json:"fingerprint" binding:"max=64"`                    // Unique identifier (may be empty)

	// Grafana-specific fields not present in Alertmanager
	Values      map[string]float64 `json:"values"`      // Query results (e.g., {"A": 95.2, "B": 100})
	ValueString string             `json:"valueString"` // Human-readable query result

	// Optional fields for fingerprint derivation when fingerprint field is empty
	// (Present in some Grafana versions/configurations)
	OrgID   int64  `json:"orgId,omitempty"`   // Grafana organization ID
	RuleUID string `json:"ruleUID,omitempty"` // Unique ID of alert rule
}

// GrafanaProvider implements WebhookProvider for Grafana Unified Alerting webhooks.
//
// Grafana Unified Alerting (v9+) is the successor to legacy Grafana alerting and uses a
// webhook format intentionally similar to Prometheus Alertmanager for easier migration.
//
// Authentication: Like Prometheus, Grafana relies on webhook URL secrecy (no signature verification).
//
// Field Mapping:
//   - Title: labels["alertname"] or labels["alert_name"] (Grafana may use either)
//   - Description: annotations["summary"] or annotations["description"]
//   - Severity: labels["severity"] (defaults to "warning" if missing)
//   - Status: "firing" or "resolved"
//   - ExternalID: fingerprint field, or derived from orgId+ruleUID if fingerprint is empty
//   - Labels: All labels from Grafana
//   - Annotations: All annotations from Grafana (including valueString if present)
type GrafanaProvider struct{}

// Source returns "grafana"
func (g *GrafanaProvider) Source() string {
	return "grafana"
}

// ValidatePayload performs no validation for Grafana.
//
// Grafana Unified Alerting does not sign webhook payloads. Security relies on:
//   - Webhook URL secrecy (configured in contact point)
//   - Network-level access controls (firewall, VPN)
//   - Optional basic auth (configured in contact point URL, not implemented here)
func (g *GrafanaProvider) ValidatePayload(body []byte, headers http.Header) error {
	return nil // No validation for Grafana
}

// ParsePayload converts a Grafana webhook payload to NormalizedAlerts.
//
// Handles both payloads with fingerprint field and those without (deriving from orgId+ruleUID).
// Preserves Grafana-specific fields (values, valueString) in annotations for display in UI.
func (g *GrafanaProvider) ParsePayload(body []byte) ([]NormalizedAlert, error) {
	var payload GrafanaWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("invalid grafana webhook payload: %w", err)
	}

	if len(payload.Alerts) == 0 {
		return nil, fmt.Errorf("grafana webhook payload contains no alerts")
	}

	alerts := make([]NormalizedAlert, 0, len(payload.Alerts))
	for _, grafanaAlert := range payload.Alerts {
		normalized, err := g.normalizeGrafanaAlert(&grafanaAlert)
		if err != nil {
			return nil, fmt.Errorf("failed to normalize grafana alert: %w", err)
		}
		alerts = append(alerts, *normalized)
	}

	return alerts, nil
}

// normalizeGrafanaAlert converts a single Grafana alert to NormalizedAlert.
//
// This handles Grafana-specific quirks:
//   - Fingerprint may be empty (derive from orgId+ruleUID or generate from labels)
//   - labels["alertname"] vs labels["alert_name"] (Grafana may use either)
//   - Query results in "values" field (add to annotations as "query_results")
func (g *GrafanaProvider) normalizeGrafanaAlert(grafanaAlert *GrafanaAlert) (*NormalizedAlert, error) {
	// Extract title from alertname label (try both common variants)
	title := grafanaAlert.Labels["alertname"]
	if title == "" {
		title = grafanaAlert.Labels["alert_name"] // Some Grafana versions use this
	}
	if title == "" {
		title = "Unknown Grafana Alert"
	}

	// Extract description from annotations (prefer summary, fall back to description)
	description := grafanaAlert.Annotations["summary"]
	if description == "" {
		description = grafanaAlert.Annotations["description"]
	}

	// Parse severity from labels (default to "warning" if missing)
	severity := grafanaAlert.Labels["severity"]
	if severity == "" {
		severity = "warning"
	}

	// Determine status
	status := "firing"
	if grafanaAlert.Status == "resolved" {
		status = "resolved"
	}

	// Handle ended_at (only set if alert is resolved and has valid timestamp)
	var endedAt *time.Time
	if status == "resolved" && !grafanaAlert.EndsAt.IsZero() {
		// Grafana may send 0001-01-01 for firing alerts (similar to Alertmanager)
		if grafanaAlert.EndsAt.Year() > 1900 {
			endedAt = &grafanaAlert.EndsAt
		}
	}

	// Determine external_id (fingerprint or derived)
	externalID := grafanaAlert.Fingerprint
	if externalID == "" {
		// Fingerprint not provided - derive from orgId+ruleUID if available
		if grafanaAlert.OrgID != 0 && grafanaAlert.RuleUID != "" {
			externalID = fmt.Sprintf("org%d-%s", grafanaAlert.OrgID, grafanaAlert.RuleUID)
		} else {
			// Fall back to generating from labels (deterministic)
			externalID = GenerateExternalID(title, grafanaAlert.Labels)
		}
	}

	// Merge annotations with Grafana-specific metadata
	annotations := make(map[string]string)
	for k, v := range grafanaAlert.Annotations {
		annotations[k] = v
	}

	// Add Grafana-specific fields to annotations for UI display
	if grafanaAlert.ValueString != "" {
		annotations["grafana_query_result"] = grafanaAlert.ValueString
	}
	if grafanaAlert.GeneratorURL != "" {
		annotations["grafana_url"] = grafanaAlert.GeneratorURL
	}

	// Marshal the individual alert as raw payload
	rawPayloadBytes, err := json.Marshal(grafanaAlert)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal grafana alert for raw payload: %w", err)
	}

	return &NormalizedAlert{
		ExternalID:  externalID,
		Source:      "grafana",
		Status:      status,
		Severity:    severity,
		Title:       title,
		Description: description,
		Labels:      grafanaAlert.Labels,
		Annotations: annotations,
		RawPayload:  json.RawMessage(rawPayloadBytes),
		StartedAt:   grafanaAlert.StartsAt,
		EndedAt:     endedAt,
	}, nil
}
