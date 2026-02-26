package models

import (
	"time"

	"github.com/google/uuid"
)

// LocalSession stores a session token for locally-authenticated users.
type LocalSession struct {
	Token     string    `gorm:"type:text;primaryKey"           json:"-"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index"        json:"-"`
	ExpiresAt time.Time `gorm:"not null"                        json:"-"`
	CreatedAt time.Time `gorm:"not null;default:now()"          json:"-"`
}

func (LocalSession) TableName() string { return "local_sessions" }
