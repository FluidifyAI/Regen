package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Incident represents an incident
type Incident struct {
	ID             uuid.UUID        `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	IncidentNumber int              `gorm:"autoIncrement;unique;not null" json:"incident_number"`
	Title          string           `gorm:"type:varchar(500);not null" json:"title"`
	Slug           string           `gorm:"type:varchar(100);not null" json:"slug"`
	Status         IncidentStatus   `gorm:"type:varchar(20);not null;default:'triggered';index:idx_incidents_status" json:"status"`
	Severity       IncidentSeverity `gorm:"type:varchar(20);not null;default:'medium';index:idx_incidents_severity" json:"severity"`
	Summary        string           `gorm:"type:text" json:"summary,omitempty"`

	// Slack integration
	SlackChannelID   string `gorm:"type:varchar(50)" json:"slack_channel_id,omitempty"`
	SlackChannelName string `gorm:"type:varchar(100)" json:"slack_channel_name,omitempty"`
	SlackMessageTS   string `gorm:"type:varchar(64)" json:"slack_message_ts,omitempty"`

	// Timestamps (created_at and triggered_at are immutable)
	CreatedAt      time.Time  `gorm:"not null;default:now()" json:"created_at"`
	TriggeredAt    time.Time  `gorm:"not null;default:now();index:idx_incidents_triggered_at" json:"triggered_at"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`

	// Ownership
	CreatedByType string     `gorm:"type:varchar(20);not null" json:"created_by_type"`
	CreatedByID   string     `gorm:"type:varchar(100)" json:"created_by_id,omitempty"`
	CommanderID   *uuid.UUID `gorm:"type:uuid" json:"commander_id,omitempty"`

	// Metadata
	Labels       JSONB `gorm:"type:jsonb;default:'{}'" json:"labels"`
	CustomFields JSONB `gorm:"type:jsonb;default:'{}'" json:"custom_fields"`

	// Grouping (v0.3+)
	// GroupKey is a hash derived from alert labels according to grouping rules
	// Alerts with the same group_key (within time window) are grouped into this incident
	// NULL for manually created incidents or incidents created before v0.3
	GroupKey *string `gorm:"type:varchar(64);index:idx_incidents_group_key_status_created" json:"group_key,omitempty"`

	// Relationships (not in database, loaded via joins)
	Alerts []Alert `gorm:"many2many:incident_alerts;" json:"alerts,omitempty"`
	// TimelineEntries will be added when TimelineEntry model is created
	// TimelineEntries  []TimelineEntry  `gorm:"foreignKey:IncidentID" json:"timeline_entries,omitempty"`
}

// IncidentStatus represents the status of an incident
type IncidentStatus string

const (
	IncidentStatusTriggered    IncidentStatus = "triggered"
	IncidentStatusAcknowledged IncidentStatus = "acknowledged"
	IncidentStatusResolved     IncidentStatus = "resolved"
	IncidentStatusCanceled     IncidentStatus = "canceled"
)

// IncidentSeverity represents the severity of an incident
type IncidentSeverity string

const (
	IncidentSeverityCritical IncidentSeverity = "critical"
	IncidentSeverityHigh     IncidentSeverity = "high"
	IncidentSeverityMedium   IncidentSeverity = "medium"
	IncidentSeverityLow      IncidentSeverity = "low"
)

// BeforeCreate hook
func (i *Incident) BeforeCreate(tx *gorm.DB) error {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	if i.TriggeredAt.IsZero() {
		i.TriggeredAt = time.Now()
	}
	return nil
}

// TableName specifies the table name for Incident
func (Incident) TableName() string {
	return "incidents"
}
