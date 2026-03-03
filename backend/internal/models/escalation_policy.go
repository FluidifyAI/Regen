package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EscalationTargetType defines how a tier resolves its notification targets.
type EscalationTargetType string

const (
	// EscalationTargetSchedule notifies the currently on-call user from the
	// tier's schedule_id.
	EscalationTargetSchedule EscalationTargetType = "schedule"

	// EscalationTargetUsers notifies every user_name in the tier's user_names list.
	EscalationTargetUsers EscalationTargetType = "users"

	// EscalationTargetBoth notifies the schedule's on-call user AND all listed users.
	EscalationTargetBoth EscalationTargetType = "both"
)

// EscalationStateStatus tracks the lifecycle of a single alert's escalation.
type EscalationStateStatus string

const (
	// EscalationStatePending means the escalation was triggered but no
	// notification has been sent yet (first worker poll hasn't run).
	EscalationStatePending EscalationStateStatus = "pending"

	// EscalationStateNotified means the worker has sent at least one DM for
	// the current tier and is waiting for acknowledgment or timeout.
	EscalationStateNotified EscalationStateStatus = "notified"

	// EscalationStateAcknowledged means a user has acknowledged the alert;
	// no further tier escalation will occur.
	EscalationStateAcknowledged EscalationStateStatus = "acknowledged"

	// EscalationStateCompleted means the escalation ended (alert resolved
	// before acknowledgment, or max tiers exhausted).
	EscalationStateCompleted EscalationStateStatus = "completed"
)

// AcknowledgmentVia identifies how an acknowledgment was received.
type AcknowledgmentVia string

const (
	AcknowledgmentViaSlack AcknowledgmentVia = "slack"
	AcknowledgmentViaAPI   AcknowledgmentVia = "api"
	AcknowledgmentViaCLI   AcknowledgmentVia = "cli"
)

// AcknowledgmentStatus is the alert-level ack state, denormalized from
// escalation_states for fast querying without a join.
type AcknowledgmentStatus string

const (
	AcknowledgmentStatusPending      AcknowledgmentStatus = "pending"
	AcknowledgmentStatusAcknowledged AcknowledgmentStatus = "acknowledged"
	AcknowledgmentStatusTimedOut     AcknowledgmentStatus = "timed_out"
)

// EscalationPolicy is a named, ordered chain of notification tiers.
//
// When a critical alert fires and matches a routing rule that references this
// policy, the escalation engine:
//  1. Creates an EscalationState row for the alert.
//  2. Notifies tier 0 targets immediately (on next worker poll).
//  3. Polls every 30 s; if the alert is unacknowledged for tier.TimeoutSeconds,
//     advances to the next tier and notifies its targets.
//
// Deleting a policy cascades to all its tiers.
type EscalationPolicy struct {
	// ID is the unique identifier for this policy.
	ID uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`

	// Name is a human-readable label.
	// Example: "Platform Team Default"
	Name string `gorm:"type:varchar(255);not null" json:"name"`

	// Description explains when and why this policy is used.
	Description string `gorm:"type:text;default:''" json:"description"`

	// Enabled controls whether this policy is evaluated by the engine.
	// Disabled policies are never triggered even if a routing rule references them.
	Enabled bool `gorm:"not null;default:true" json:"enabled"`

	// CreatedAt is when this policy was created (immutable, server-generated).
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`

	// UpdatedAt is when this policy was last modified.
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`

	// Tiers are loaded via repository.GetPolicyWithTiers — not auto-loaded by GORM.
	Tiers []EscalationTier `gorm:"-" json:"tiers,omitempty"`
}

// TableName specifies the database table name.
func (EscalationPolicy) TableName() string {
	return "escalation_policies"
}

// BeforeCreate sets the ID if not already set.
func (p *EscalationPolicy) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// EscalationTier is one step in an escalation policy.
//
// Tiers are evaluated in ascending tier_index order. The engine notifies
// the tier's targets, then waits TimeoutSeconds before advancing to
// tier_index+1.  If no higher tier exists, the escalation is marked completed.
//
// target_type determines which targets are notified:
//   - "schedule": on-call user resolved from ScheduleID at notification time
//   - "users":    every name in UserNames
//   - "both":     union of schedule on-call and UserNames
type EscalationTier struct {
	// ID is the unique identifier for this tier.
	ID uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`

	// PolicyID is the parent escalation policy.
	PolicyID uuid.UUID `gorm:"type:uuid;not null;index" json:"policy_id"`

	// TierIndex is the 0-based position in the escalation chain.
	// A UNIQUE constraint on (policy_id, tier_index) is enforced by the DB.
	TierIndex int `gorm:"not null;default:0" json:"tier_index"`

	// TimeoutSeconds is how long to wait at this tier before advancing.
	// Minimum 1 second; typical values are 300 (5 min) or 600 (10 min).
	TimeoutSeconds int `gorm:"not null;default:300" json:"timeout_seconds"`

	// TargetType defines which targets are notified.
	TargetType EscalationTargetType `gorm:"type:varchar(50);not null;default:'schedule'" json:"target_type"`

	// ScheduleID is the schedule whose on-call user will be notified.
	// Nullable: nil means no schedule target (only relevant when TargetType is
	// "schedule" or "both").  Set to nil if the referenced schedule is deleted.
	ScheduleID *uuid.UUID `gorm:"type:uuid;index" json:"schedule_id,omitempty"`

	// UserNames is a list of free-text user identifiers to notify directly.
	// Mirrors the user_name convention used in schedule_participants.
	// Example: ["alice", "@bob", "Carol Smith"]
	UserNames JSONBArray `gorm:"type:jsonb;not null;default:'[]'" json:"user_names"`

	// CreatedAt is when this tier was created (immutable, server-generated).
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
}

// TableName specifies the database table name.
func (EscalationTier) TableName() string {
	return "escalation_tiers"
}

// BeforeCreate sets the ID if not already set.
func (t *EscalationTier) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

// EscalationState tracks the live escalation progress for a single alert or
// manually escalated incident.
//
// Exactly one of AlertID / IncidentID is set, discriminated by SourceType.
// For alert-sourced states the legacy UNIQUE index on alert_id still applies.
// The worker queries for active states (acknowledged_at IS NULL) and
// advances the tier when last_notified_at + tier.timeout_seconds < NOW().
type EscalationState struct {
	// ID is the unique identifier for this state row.
	ID uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`

	// AlertID is set for alert-sourced escalations (nullable for incident escalations).
	AlertID *uuid.UUID `gorm:"type:uuid;uniqueIndex" json:"alert_id,omitempty"`

	// IncidentID is set for incident-sourced (manual) escalations.
	IncidentID *uuid.UUID `gorm:"type:uuid;index" json:"incident_id,omitempty"`

	// SourceType distinguishes alert vs. incident escalations.
	SourceType string `gorm:"type:varchar(20);not null;default:'alert'" json:"source_type"`

	// PolicyID is the escalation policy driving this escalation.
	PolicyID uuid.UUID `gorm:"type:uuid;not null;index" json:"policy_id"`

	// CurrentTierIndex is the tier that was most recently notified.
	// Starts at 0; incremented by the worker on timeout.
	CurrentTierIndex int `gorm:"not null;default:0" json:"current_tier_index"`

	// Status reflects the current lifecycle stage of this escalation.
	Status EscalationStateStatus `gorm:"type:varchar(50);not null;default:'pending'" json:"status"`

	// LastNotifiedAt is when the current tier's notifications were last sent.
	// Nil means tier 0 has not yet been notified (status == pending).
	LastNotifiedAt *time.Time `gorm:"type:timestamptz" json:"last_notified_at,omitempty"`

	// AcknowledgedAt is when the escalation was stopped by a user.
	// Nil means not yet acknowledged.
	AcknowledgedAt *time.Time `gorm:"type:timestamptz" json:"acknowledged_at,omitempty"`

	// AcknowledgedBy is the user_name of whoever acknowledged the alert.
	AcknowledgedBy *string `gorm:"type:varchar(255)" json:"acknowledged_by,omitempty"`

	// AcknowledgedVia records the channel used: "slack", "api", or "cli".
	AcknowledgedVia *AcknowledgmentVia `gorm:"type:varchar(50)" json:"acknowledged_via,omitempty"`

	// CreatedAt is when this escalation was triggered (immutable, server-generated).
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
}

// TableName specifies the database table name.
func (EscalationState) TableName() string {
	return "escalation_states"
}

// BeforeCreate sets the ID if not already set.
func (s *EscalationState) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

// IsActive returns true if this escalation is still in progress (not yet
// acknowledged or completed).
func (s *EscalationState) IsActive() bool {
	return s.AcknowledgedAt == nil &&
		s.Status != EscalationStateCompleted
}
