package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TimelineEntry represents an immutable timeline entry for an incident
type TimelineEntry struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	IncidentID uuid.UUID `gorm:"type:uuid;not null;index:idx_timeline_incident_timestamp" json:"incident_id"`
	Timestamp  time.Time `gorm:"not null;default:now();index:idx_timeline_incident_timestamp" json:"timestamp"`
	Type       string    `gorm:"type:varchar(50);not null" json:"type"`
	ActorType  string    `gorm:"type:varchar(20);not null" json:"actor_type"`
	ActorID    string    `gorm:"type:varchar(100)" json:"actor_id,omitempty"`
	Content    JSONB     `gorm:"type:jsonb;not null" json:"content"`
	CreatedAt  time.Time `gorm:"not null;default:now()" json:"created_at"`

	// Relationship
	Incident   *Incident `gorm:"foreignKey:IncidentID" json:"incident,omitempty"`
}

// TimelineEntryType constants
const (
	TimelineTypeIncidentCreated   = "incident_created"
	TimelineTypeStatusChanged     = "status_changed"
	TimelineTypeSeverityChanged   = "severity_changed"
	TimelineTypeAlertLinked       = "alert_linked"
	TimelineTypeMessage           = "message"
	TimelineTypeResponderAdded    = "responder_added"
	TimelineTypeEscalated         = "escalated"
	TimelineTypeSummaryGenerated  = "summary_generated"
	TimelineTypePostmortemCreated = "postmortem_created"
)

// BeforeCreate hook
func (t *TimelineEntry) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	if t.Timestamp.IsZero() {
		t.Timestamp = time.Now()
	}
	return nil
}

// TableName specifies the table name for TimelineEntry
func (TimelineEntry) TableName() string {
	return "timeline_entries"
}
