package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AlertStatus represents the current state of an alert
type AlertStatus string

const (
	AlertStatusFiring   AlertStatus = "firing"
	AlertStatusResolved AlertStatus = "resolved"
)

// AlertSeverity represents the severity level of an alert
type AlertSeverity string

const (
	AlertSeverityCritical AlertSeverity = "critical"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityInfo     AlertSeverity = "info"
)

// JSONB is a custom type for PostgreSQL JSONB fields
type JSONB map[string]interface{}

// Value implements the driver.Valuer interface for JSONB
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface for JSONB
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("failed to unmarshal JSONB value: unsupported type")
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bytes, &result); err != nil {
		return err
	}

	*j = result
	return nil
}

// JSONBArray is a custom type for PostgreSQL JSONB array fields
type JSONBArray []string

// Value implements the driver.Valuer interface for JSONBArray
func (j JSONBArray) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface for JSONBArray
func (j *JSONBArray) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("failed to unmarshal JSONBArray value: unsupported type")
	}

	var result []string
	if err := json.Unmarshal(bytes, &result); err != nil {
		return err
	}

	*j = result
	return nil
}

// Alert represents an alert from a monitoring system
type Alert struct {
	ID          uuid.UUID     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ExternalID  string        `gorm:"type:varchar(255);not null" json:"external_id"`
	Source      string        `gorm:"type:varchar(100);not null" json:"source"`
	Fingerprint string        `gorm:"type:varchar(255);index" json:"fingerprint"`
	Status      AlertStatus   `gorm:"type:varchar(50);not null;default:'firing'" json:"status"`
	Severity    AlertSeverity `gorm:"type:varchar(50);not null;default:'info'" json:"severity"`
	Title       string        `gorm:"type:varchar(500);not null" json:"title"`
	Description string        `gorm:"type:text" json:"description"`
	Labels      JSONB         `gorm:"type:jsonb;default:'{}'" json:"labels"`
	Annotations JSONB         `gorm:"type:jsonb;default:'{}'" json:"annotations"`
	RawPayload  JSONB         `gorm:"type:jsonb" json:"raw_payload"`
	StartedAt   time.Time     `gorm:"type:timestamptz;not null" json:"started_at"`
	EndedAt     *time.Time    `gorm:"type:timestamptz" json:"ended_at"`
	ReceivedAt  time.Time     `gorm:"type:timestamptz;not null;default:now()" json:"received_at"`
	CreatedAt   time.Time     `gorm:"type:timestamptz;not null;default:now()" json:"created_at"`

	// EscalationPolicyID links this alert to the escalation policy assigned by
	// the routing engine.  Nil means no escalation is configured for this alert.
	EscalationPolicyID *uuid.UUID `gorm:"type:uuid;index" json:"escalation_policy_id,omitempty"`

	// AcknowledgmentStatus is the alert-level ack state driven by the escalation
	// engine. Denormalized from escalation_states for fast querying without a join.
	AcknowledgmentStatus AcknowledgmentStatus `gorm:"type:varchar(50);not null;default:'pending'" json:"acknowledgment_status"`
}

// TableName specifies the table name for the Alert model
func (Alert) TableName() string {
	return "alerts"
}

// BeforeCreate is a GORM hook that runs before creating an alert
// It ensures ReceivedAt is set if not already specified (immutability)
func (a *Alert) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	if a.ReceivedAt.IsZero() {
		a.ReceivedAt = time.Now()
	}
	return nil
}
