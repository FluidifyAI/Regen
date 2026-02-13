package models

import (
	"time"

	"github.com/google/uuid"
)

// RotationType defines the cadence of a schedule layer's rotation.
type RotationType string

const (
	// RotationTypeDaily rotates participants every day.
	RotationTypeDaily RotationType = "daily"
	// RotationTypeWeekly rotates participants every week.
	RotationTypeWeekly RotationType = "weekly"
	// RotationTypeCustom rotates based on shift_duration_seconds.
	RotationTypeCustom RotationType = "custom"
)

// Schedule is the top-level on-call schedule entity.
//
// A schedule contains one or more layers. During evaluation the layer with
// the lowest order_index that has a non-empty participant slot for the
// queried time wins (fallback chain). Overrides are checked before layers.
type Schedule struct {
	// ID is the unique identifier for this schedule.
	ID uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`

	// Name is a human-readable label.
	// Example: "Platform Team Primary"
	Name string `gorm:"type:varchar(255);not null" json:"name"`

	// Description explains the purpose of this schedule.
	Description string `gorm:"type:text;default:''" json:"description"`

	// Timezone is an IANA timezone string used for shift boundary calculations.
	// Example: "America/New_York", "UTC"
	Timezone string `gorm:"type:varchar(100);not null;default:'UTC'" json:"timezone"`

	// NotificationChannel is an optional Slack channel or other destination
	// to post shift-change notifications to. Free-form string; may be empty.
	// Example: "#oncall-platform"
	NotificationChannel string `gorm:"type:varchar(255);default:''" json:"notification_channel"`

	// CreatedAt is when this schedule was created (immutable, server-generated).
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`

	// UpdatedAt is when this schedule was last modified.
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`

	// Layers are loaded via GetWithLayers — not auto-loaded by GORM.
	// Populated only when explicitly fetched.
	Layers []ScheduleLayer `gorm:"-" json:"layers,omitempty"`
}

// TableName specifies the database table name.
func (Schedule) TableName() string {
	return "schedules"
}

// ScheduleLayer defines one rotation layer within a schedule.
//
// Layers are stacked by order_index. The evaluator walks layers 0, 1, 2, …
// and the first layer that yields a non-empty user for the requested time wins.
// This models primary/secondary/tertiary on-call without special-casing.
type ScheduleLayer struct {
	// ID is the unique identifier for this layer.
	ID uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`

	// ScheduleID is the parent schedule.
	ScheduleID uuid.UUID `gorm:"type:uuid;not null;index" json:"schedule_id"`

	// Name is a human-readable label.
	// Example: "Primary", "Secondary"
	Name string `gorm:"type:varchar(255);not null" json:"name"`

	// OrderIndex determines evaluation precedence. Lower wins.
	// 0 = primary, 1 = secondary, etc.
	OrderIndex int `gorm:"not null;default:0" json:"order_index"`

	// RotationType defines the cadence: "daily", "weekly", or "custom".
	RotationType RotationType `gorm:"type:varchar(50);not null;default:'weekly'" json:"rotation_type"`

	// RotationStart is the epoch from which shift slots are computed.
	// Defaults to midnight UTC on the day the layer is created.
	// All slot boundaries are: RotationStart + N * ShiftDurationSeconds.
	RotationStart time.Time `gorm:"not null" json:"rotation_start"`

	// ShiftDurationSeconds is the length of one shift in seconds.
	// For "daily" this is 86400, for "weekly" 604800.
	// For "custom" the caller sets it explicitly.
	ShiftDurationSeconds int `gorm:"not null;default:604800" json:"shift_duration_seconds"`

	// CreatedAt is when this layer was created (immutable, server-generated).
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`

	// Participants are loaded alongside the layer — not auto-loaded by GORM.
	Participants []ScheduleParticipant `gorm:"-" json:"participants,omitempty"`
}

// TableName specifies the database table name.
func (ScheduleLayer) TableName() string {
	return "schedule_layers"
}

// ScheduleParticipant is a single user slot within a layer.
//
// Participants are ordered by order_index to define rotation order.
// The on-call user at time T is: participants[slotIndex % len(participants)]
// where slotIndex = floor((T - RotationStart) / ShiftDuration).
type ScheduleParticipant struct {
	// ID is the unique identifier for this participant slot.
	ID uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`

	// LayerID is the parent layer.
	LayerID uuid.UUID `gorm:"type:uuid;not null;index" json:"layer_id"`

	// UserName is the display name or identifier for the on-call person.
	// Free-text; not a foreign key. Examples: "alice", "@alice", "Alice Smith".
	UserName string `gorm:"type:varchar(255);not null" json:"user_name"`

	// OrderIndex determines the rotation order within the layer.
	// Slot 0 is on-call first from RotationStart, then slot 1, etc.
	OrderIndex int `gorm:"not null;default:0" json:"order_index"`

	// CreatedAt is when this participant was added (immutable, server-generated).
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
}

// TableName specifies the database table name.
func (ScheduleParticipant) TableName() string {
	return "schedule_participants"
}

// ScheduleOverride temporarily replaces the computed on-call user for a
// specific time range within a schedule.
//
// Overrides are checked before layers: if any override covers the queried
// time, its user is returned without consulting layers.
type ScheduleOverride struct {
	// ID is the unique identifier for this override.
	ID uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`

	// ScheduleID is the parent schedule.
	ScheduleID uuid.UUID `gorm:"type:uuid;not null;index" json:"schedule_id"`

	// OverrideUser is the user taking over on-call during this window.
	OverrideUser string `gorm:"type:varchar(255);not null" json:"override_user"`

	// StartTime is the beginning of the override window (inclusive).
	StartTime time.Time `gorm:"not null" json:"start_time"`

	// EndTime is the end of the override window (exclusive).
	EndTime time.Time `gorm:"not null" json:"end_time"`

	// CreatedBy is the user_name of whoever created this override.
	CreatedBy string `gorm:"type:varchar(255);not null;default:'system'" json:"created_by"`

	// CreatedAt is when this override was created (immutable, server-generated).
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
}

// TableName specifies the database table name.
func (ScheduleOverride) TableName() string {
	return "schedule_overrides"
}
