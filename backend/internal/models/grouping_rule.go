package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GroupingRule defines how alerts should be grouped into a single incident.
//
// Grouping rules are evaluated in priority order (lowest number = highest priority).
// The first matching rule determines the grouping behavior for an alert.
//
// Example use cases:
//   - "Group all alerts with same alertname within 5 minutes"
//   - "Group alerts with same service label within 10 minutes"
//   - "Group critical alerts from same region within 1 minute"
//
// Grouping prevents alert fatigue by creating one incident for related alerts
// instead of separate incidents for each alert.
type GroupingRule struct {
	// ID is the unique identifier for this grouping rule
	ID uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`

	// Name is a human-readable identifier for this rule
	// Example: "Group by service and environment"
	Name string `gorm:"type:varchar(255);not null" json:"name"`

	// Description explains what this rule does and why it exists
	// Example: "Groups alerts from the same service in the same environment..."
	Description string `gorm:"type:text;default:''" json:"description"`

	// Enabled controls whether this rule is active
	// Disabled rules are skipped during alert processing
	Enabled bool `gorm:"not null" json:"enabled"`

	// Priority determines evaluation order (lower number = higher priority)
	// Rules are evaluated in ascending priority order until one matches
	// Example: Priority 1 is evaluated before Priority 100
	// UNIQUE constraint ensures no two rules have the same priority
	Priority int `gorm:"not null;uniqueIndex:idx_grouping_rules_priority" json:"priority"`

	// MatchLabels defines which alerts this rule applies to
	// Stored as JSONB map[string]string
	//
	// Matching logic:
	//   - Empty map {} matches all alerts
	//   - {"alertname": "*"} matches all alerts (wildcard)
	//   - {"alertname": "HighCPU"} matches alerts with exactly that alertname
	//   - {"service": "api", "env": "prod"} matches alerts with both labels
	//
	// Example: {"service": "*", "severity": "critical"} matches all critical alerts
	// that have a service label (regardless of value)
	MatchLabels JSONB `gorm:"type:jsonb;not null;default:'{}'" json:"match_labels"`

	// TimeWindowSeconds defines the grouping time window in seconds
	//
	// Alerts are grouped if:
	//   1. They match the same rule
	//   2. They have the same group key (derived from labels)
	//   3. An open incident exists within this time window
	//
	// Example: 300 (5 minutes) means alerts within 5 minutes are grouped together
	//
	// Use cases:
	//   - Short window (60s): Group rapid-fire alerts from same issue
	//   - Medium window (300s): Group related alerts during incident
	//   - Long window (3600s): Group slow-developing issues
	TimeWindowSeconds int `gorm:"not null;default:300" json:"time_window_seconds"`

	// CrossSourceLabels enables grouping alerts from different monitoring sources
	// Stored as JSONB []string
	//
	// When specified, alerts from different sources (prometheus, grafana, cloudwatch)
	// can be grouped together if they share the same values for these labels.
	//
	// Example: ["service", "env"] means:
	//   - Prometheus alert: {service: "api", env: "prod"}
	//   - Grafana alert: {service: "api", env: "prod"}
	//   → Both alerts grouped into same incident
	//
	// Use cases:
	//   - Cross-monitoring correlation: Same service failing in both Prometheus and Grafana
	//   - Multi-region deployments: Correlate alerts across CloudWatch regions
	//
	// Default: [] (empty array) means no cross-source grouping
	CrossSourceLabels JSONBArray `gorm:"type:jsonb;default:'[]'" json:"cross_source_labels"`

	// CreatedAt is when this rule was created (immutable)
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`

	// UpdatedAt is when this rule was last modified
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

// TableName specifies the database table name
func (GroupingRule) TableName() string {
	return "grouping_rules"
}

// BeforeCreate generates a UUID for new grouping rules when none is set.
// This ensures compatibility with both PostgreSQL (which uses gen_random_uuid() as default)
// and SQLite (used in tests, which does not support that function).
func (gr *GroupingRule) BeforeCreate(tx *gorm.DB) error {
	if gr.ID == uuid.Nil {
		gr.ID = uuid.New()
	}
	return nil
}

// GroupKey generates a unique key for grouping alerts based on this rule.
//
// The group key is derived from alert labels according to the rule's configuration.
// Alerts with the same group key (and within the time window) are grouped together.
//
// Algorithm:
//  1. Extract relevant label keys from MatchLabels (non-wildcard keys)
//  2. Sort keys alphabetically for deterministic ordering
//  3. Build string: "key1=value1|key2=value2|..."
//  4. Hash with SHA256 for consistent, compact key
//
// Example:
//   Rule: MatchLabels = {"service": "*", "env": "*"}
//   Alert: {service: "api", env: "prod", instance: "web-01"}
//   GroupKey: SHA256("env=prod|service=api") → "a1b2c3..."
//
// Note: instance label is NOT included because it's not in MatchLabels.
// This ensures all alerts from the same service+env are grouped.
func (gr *GroupingRule) GroupKey(alertLabels map[string]string) string {
	// Implementation will be in grouping_engine.go
	// This is a placeholder method signature
	return ""
}

// Matches checks if this rule applies to an alert based on its labels.
//
// Returns true if all labels in MatchLabels match the alert's labels.
//
// Matching logic:
//   - Empty MatchLabels {} matches all alerts
//   - Wildcard "*" matches any value for that key
//   - Exact value must match exactly
//   - If MatchLabels requires a key, alert must have that key
//
// Examples:
//   Rule: {"alertname": "HighCPU"} matches alert with alertname=HighCPU
//   Rule: {"severity": "*"} matches any alert with a severity label
//   Rule: {} matches all alerts
func (gr *GroupingRule) Matches(alertLabels map[string]string) bool {
	// Implementation will be in grouping_engine.go
	// This is a placeholder method signature
	return false
}

// IsValid validates the grouping rule configuration.
//
// Returns error if:
//   - Name is empty
//   - Priority is negative
//   - TimeWindowSeconds is <= 0
//   - MatchLabels is invalid JSON
//
// This should be called before saving a rule to the database.
func (gr *GroupingRule) IsValid() error {
	// Implementation will be added when needed
	// This is a placeholder method signature
	return nil
}
