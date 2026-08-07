package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DeviceToken stores an FCM registration token for a user's device.
// Token is tagged json:"-" and must never appear in any API response or log line.
type DeviceToken struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID     uuid.UUID `gorm:"type:uuid;not null"                             json:"user_id"`
	Token      string    `gorm:"type:text;not null"                             json:"-"` // never serialized
	Platform   string    `gorm:"type:varchar(10);not null;default:'web'"        json:"platform"`
	AppVersion string    `gorm:"type:varchar(50);not null;default:''"           json:"app_version"`
	CreatedAt  time.Time `gorm:"not null;default:now()"                         json:"created_at"`
	LastSeenAt time.Time `gorm:"not null;default:now()"                         json:"last_seen_at"`
}

// TableName returns the DB table name for GORM.
func (DeviceToken) TableName() string { return "device_tokens" }

// BeforeCreate sets a UUID when none is provided, matching all other models.
func (d *DeviceToken) BeforeCreate(_ *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}
