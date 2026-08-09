package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WebPushSubscription stores a Web Push subscription for a user's browser.
// Endpoint, P256DH, and Auth are tagged json:"-" and must never appear in
// any API response or log line.
type WebPushSubscription struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"  json:"id"`
	UserID     uuid.UUID `gorm:"type:uuid;not null"                              json:"user_id"`
	Endpoint   string    `gorm:"type:text;not null;column:endpoint"              json:"-"` // never serialized
	P256DH     string    `gorm:"type:text;not null;column:p256dh"                json:"-"` // never serialized
	Auth       string    `gorm:"type:text;not null;column:auth"                  json:"-"` // never serialized
	CreatedAt  time.Time `gorm:"not null;default:now()"                          json:"created_at"`
	LastSeenAt time.Time `gorm:"not null;default:now()"                          json:"last_seen_at"`
}

// TableName returns the DB table name for GORM.
func (WebPushSubscription) TableName() string { return "web_push_subscriptions" }

// BeforeCreate sets a UUID when none is provided, matching all other models.
func (s *WebPushSubscription) BeforeCreate(_ *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}
