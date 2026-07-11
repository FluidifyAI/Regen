package models

import (
	"time"

	"github.com/google/uuid"
)

// RoutingRule defines how alerts should be routed after grouping.
//
// Routing rules are evaluated in priority order (lowest number = highest priority).
// The first matching rule determines what happens to the alert.
//
// match_criteria JSONB schema:
//
//	{
//	  "source":   ["prometheus", "grafana"],   // optional; empty = all sources
//	  "severity": ["critical", "warning"],     // optional; empty = all severities
//	  "labels": {                              // optional; alert label matching
//	    "env":       "prod",                   // exact string match
//	    "alertname": "DiskUsage.*",            // RE2 regex
//	    "svc":       "*"                       // * = key present, any value
//	  },
//	  "annotations": {                         // optional; alert annotation matching
//	    "summary": ".*connection refused.*"    // RE2 regex
//	  },
//	  "title":       ".*disk.*",               // optional; RE2 regex on alert title
//	  "description": ".*OOM.*"                // optional; RE2 regex on alert description
//	}
//
// All criteria are ANDed — every specified field must match.
// source and severity use exact list matching. labels, annotations, title, and
// description values use RE2 regex (no lookaheads; Go regexp package).
// Use * as a label/annotation value to match any non-empty value (key-exists check).
//
// How title and description are derived per source:
//
//	Source      title                    description
//	----------  -----------------------  -----------------------
//	Prometheus  labels.alertname         annotations.summary
//	Grafana     rule name                annotations.summary
//	Generic     title field              description field
//
// actions JSONB schema:
//
//	{
//	  "create_incident":   true,        // default action
//	  "suppress":          true,        // alert stored, no incident created
//	  "severity_override": "critical",  // override alert severity
//	  "channel_override":  "db-oncall"  // override Slack channel name suffix
//	}
type RoutingRule struct {
	// ID is the unique identifier for this routing rule
	ID uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`

	// Name is a human-readable identifier for this rule
	Name string `gorm:"type:varchar(255);not null" json:"name"`

	// Description explains what this rule does and why it exists
	Description string `gorm:"type:text;default:''" json:"description"`

	// Enabled controls whether this rule is active
	Enabled bool `gorm:"not null;default:true" json:"enabled"`

	// Priority determines evaluation order (lower number = higher priority).
	// UNIQUE constraint ensures no two rules share the same priority.
	Priority int `gorm:"not null;uniqueIndex:idx_routing_rules_priority" json:"priority"`

	// MatchCriteria defines which alerts this rule applies to (JSONB).
	// Empty map {} matches all alerts.
	MatchCriteria JSONB `gorm:"type:jsonb;not null;default:'{}'" json:"match_criteria"`

	// Actions defines what to do when this rule matches (JSONB).
	// Supported keys: create_incident, suppress, severity_override, channel_override.
	Actions JSONB `gorm:"type:jsonb;not null;default:'{}'" json:"actions"`

	// CreatedAt is when this rule was created (immutable)
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`

	// UpdatedAt is when this rule was last modified
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

// TableName specifies the database table name
func (RoutingRule) TableName() string {
	return "routing_rules"
}
