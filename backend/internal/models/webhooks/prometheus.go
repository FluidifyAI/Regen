package webhooks

import "time"

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
