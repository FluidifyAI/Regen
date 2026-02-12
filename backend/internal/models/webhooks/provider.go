package webhooks

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"
)

// WebhookProvider defines the interface that all monitoring source webhook handlers must implement.
//
// Each monitoring system (Prometheus, Grafana, CloudWatch, etc.) has its own payload format and
// authentication mechanism. WebhookProvider normalizes these differences so the AlertService can
// process alerts uniformly regardless of their source.
//
// Example flow:
//  1. HTTP webhook received → WebhookHandler
//  2. ValidatePayload() checks signatures/authentication
//  3. ParsePayload() converts source-specific JSON → []NormalizedAlert
//  4. AlertService.ProcessNormalizedAlerts() handles storage and incident creation
//
// Adding a new monitoring source requires only implementing this three-method interface.
type WebhookProvider interface {
	// Source returns the identifier for this monitoring source.
	// Used as the alert.source field for storage and deduplication.
	//
	// Examples: "prometheus", "grafana", "cloudwatch", "generic"
	//
	// MUST be lowercase alphanumeric with optional hyphens.
	// MUST be globally unique across all providers.
	Source() string

	// ValidatePayload verifies the authenticity of the webhook request.
	//
	// This method is called BEFORE parsing to prevent wasting CPU on forged requests.
	// Validation mechanisms vary by source:
	//   - Prometheus: No validation (relies on URL secrecy)
	//   - Grafana: No validation (relies on URL secrecy)
	//   - CloudWatch: SNS message signature verification
	//   - Generic: HMAC-SHA256 signature via X-Webhook-Secret header
	//
	// Parameters:
	//   - body: Raw webhook request body (for signature validation)
	//   - headers: HTTP request headers (for signature extraction)
	//
	// Returns nil if valid or no validation configured.
	// Returns error if authentication fails (handler will return 401).
	ValidatePayload(body []byte, headers http.Header) error

	// ParsePayload converts the source-specific webhook payload into normalized alerts.
	//
	// This is where provider-specific field mapping happens:
	//   - Extract title, description, severity from source-specific fields
	//   - Map source-specific status values to "firing" or "resolved"
	//   - Derive a stable external_id for deduplication
	//   - Convert source-specific labels/tags to normalized map[string]string
	//   - Preserve complete original payload in RawPayload for debugging
	//
	// Parameters:
	//   - body: Raw webhook request body (validated via ValidatePayload)
	//
	// Returns:
	//   - []NormalizedAlert: One or more normalized alerts (webhooks often contain multiple)
	//   - error: Parse error (handler will return 400)
	//
	// Implementation notes:
	//   - MUST return at least one NormalizedAlert or an error (not both empty)
	//   - SHOULD store complete original payload in NormalizedAlert.RawPayload
	//   - SHOULD handle malformed JSON gracefully with descriptive error messages
	ParsePayload(body []byte) ([]NormalizedAlert, error)
}

// NormalizedAlert is the canonical internal representation of an alert.
//
// All webhook providers produce NormalizedAlerts. AlertService only consumes this type.
// This separation allows adding new monitoring sources without modifying core alert processing logic.
//
// Field Mapping Philosophy:
//   - Title: Human-readable alert name (e.g., "High CPU on web-01")
//   - Description: Detailed context (e.g., "CPU utilization is 95%, threshold 80%")
//   - Severity: critical | warning | info (normalized across all sources)
//   - Status: firing | resolved (normalized across all sources)
//   - Labels: Structured key-value metadata for filtering/grouping (e.g., service, env, instance)
//   - Annotations: Additional metadata not used for routing (e.g., runbook_url, dashboard_url)
//   - RawPayload: Complete original webhook payload for debugging and future processing
type NormalizedAlert struct {
	// ExternalID is the source-specific unique identifier for deduplication.
	//
	// Examples:
	//   - Prometheus: fingerprint field (e.g., "a3c4e8f1234567")
	//   - Grafana: fingerprint or orgId+ruleId combo
	//   - CloudWatch: AlarmArn (e.g., "arn:aws:cloudwatch:us-east-1:123:alarm:HighCPU")
	//   - Generic: User-provided or SHA256(title + sorted labels)
	//
	// Combined with Source, forms the composite deduplication key: (source, external_id).
	// When the same alert fires again, it updates the existing alert instead of creating a duplicate.
	ExternalID string `json:"external_id"`

	// Source identifies which monitoring system sent this alert.
	// Matches the value returned by WebhookProvider.Source().
	//
	// Examples: "prometheus", "grafana", "cloudwatch", "generic"
	Source string `json:"source"`

	// Status represents the current state of the alert.
	// MUST be one of: "firing" or "resolved"
	//
	// Mapping guidelines:
	//   - Alertmanager "firing" → "firing"
	//   - Alertmanager "resolved" → "resolved"
	//   - Grafana "alerting" → "firing"
	//   - Grafana "ok" / "normal" → "resolved"
	//   - CloudWatch "ALARM" → "firing"
	//   - CloudWatch "OK" → "resolved"
	Status string `json:"status"`

	// Severity represents the urgency level of the alert.
	// MUST be one of: "critical", "warning", or "info"
	//
	// Severity affects incident auto-creation:
	//   - "critical" → Always creates incident
	//   - "warning"  → Always creates incident
	//   - "info"     → Alert stored but no incident created
	//
	// Mapping guidelines:
	//   - Map the most severe source value to "critical"
	//   - Map medium severity to "warning"
	//   - Map informational/low severity to "info"
	//   - Default to "warning" if source doesn't specify severity
	Severity string `json:"severity"`

	// Title is a short, human-readable summary of the alert.
	// Displayed in incident lists and Slack notifications.
	//
	// Examples:
	//   - "High CPU usage on web-01"
	//   - "API endpoint /users returning 5xx errors"
	//   - "Database connection pool exhausted"
	//
	// Field mapping:
	//   - Prometheus: labels["alertname"]
	//   - Grafana: title field
	//   - CloudWatch: AlarmName
	//   - Generic: title field (required)
	Title string `json:"title"`

	// Description provides detailed context about the alert.
	// Displayed in incident detail pages.
	//
	// Examples:
	//   - "CPU utilization is 95%, threshold 80%"
	//   - "Error rate: 12% (120/1000 requests in last 5 minutes)"
	//
	// Field mapping:
	//   - Prometheus: annotations["summary"] or annotations["description"]
	//   - Grafana: message field
	//   - CloudWatch: AlarmDescription + NewStateReason
	//   - Generic: description field (optional)
	Description string `json:"description"`

	// Labels are structured key-value pairs used for filtering, grouping, and routing.
	//
	// Common labels across sources:
	//   - alertname: Name of the alert rule
	//   - severity: Severity level (may differ from normalized Severity field)
	//   - instance: Server/pod/container instance
	//   - service: Application or service name
	//   - env: Environment (prod, staging, dev)
	//   - team: Owning team
	//   - region: Geographic region or availability zone
	//
	// Labels are used by:
	//   - Grouping rules: "group alerts with same service label"
	//   - Routing rules: "route alerts with team=db to #db-oncall"
	//   - Deduplication: Optional cross-source correlation
	Labels map[string]string `json:"labels"`

	// Annotations are additional metadata NOT used for routing or grouping.
	//
	// Common annotations:
	//   - summary: Short description
	//   - description: Detailed explanation
	//   - runbook_url: Link to runbook/playbook
	//   - dashboard_url: Link to monitoring dashboard
	//   - graph_url: Link to query/graph
	//
	// Annotations are displayed in the UI but don't affect alert processing logic.
	Annotations map[string]string `json:"annotations"`

	// RawPayload stores the complete original webhook payload.
	//
	// Purpose:
	//   - Debugging: Inspect exact payload when alerts don't behave as expected
	//   - Future processing: New features can extract additional fields without changing providers
	//   - Audit trail: Complete record of what the monitoring system sent
	//
	// Stored as JSONB in the database for efficient querying.
	RawPayload json.RawMessage `json:"raw_payload"`

	// StartedAt is when the alert first started firing.
	//
	// For "firing" alerts: timestamp when condition became true
	// For "resolved" alerts: original start time (not resolution time)
	//
	// Field mapping:
	//   - Prometheus: StartsAt field
	//   - Grafana: StartsAt field
	//   - CloudWatch: StateChangeTime (for ALARM state)
	//   - Generic: started_at field or current time if not provided
	StartedAt time.Time `json:"started_at"`

	// EndedAt is when the alert was resolved (nil for firing alerts).
	//
	// Only set when Status == "resolved".
	// For "firing" alerts: MUST be nil
	//
	// Field mapping:
	//   - Prometheus: EndsAt (but only if > year 1900, Alertmanager sends 0001-01-01 for firing)
	//   - Grafana: EndsAt
	//   - CloudWatch: StateChangeTime (for OK state)
	//   - Generic: ended_at field or nil if not provided
	EndedAt *time.Time `json:"ended_at,omitempty"`
}

// Fingerprint generates a stable deduplication key from source and external_id.
//
// The fingerprint is used for quick lookups and caching.
// The actual deduplication happens via database unique constraint on (source, external_id).
//
// Returns: SHA256 hash of "{source}:{external_id}"
func (n *NormalizedAlert) Fingerprint() string {
	data := fmt.Sprintf("%s:%s", n.Source, n.ExternalID)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// GenerateExternalID creates a stable external_id from title and labels when not provided.
//
// Used by the Generic webhook provider when users don't specify an external_id.
// The generated ID is deterministic: same title + labels → same external_id.
//
// Algorithm:
//  1. Sort label keys alphabetically
//  2. Create string: "title:{title}|labels:{k1}={v1}|{k2}={v2}..."
//  3. SHA256 hash the string
//  4. Return first 32 characters of hex hash
//
// Example:
//   Title: "High CPU"
//   Labels: {"service": "api", "env": "prod"}
//   Result: "abc123..." (SHA256 of "title:High CPU|labels:env=prod|service=api")
func GenerateExternalID(title string, labels map[string]string) string {
	// Sort label keys for deterministic output
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build string: title + sorted labels
	data := fmt.Sprintf("title:%s", title)
	for _, k := range keys {
		data += fmt.Sprintf("|%s=%s", k, labels[k])
	}

	// Hash and return first 32 chars
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)[:32]
}
