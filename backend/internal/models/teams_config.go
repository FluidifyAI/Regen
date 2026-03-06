package models

import (
	"time"

	"github.com/google/uuid"
)

// TeamsConfig holds the Microsoft Teams integration configuration for the entire instance.
// Only one row exists (enforced by CHECK id = 1).
type TeamsConfig struct {
	ID          int        `gorm:"primaryKey;default:1"`
	AppID       string     `gorm:"column:app_id"`
	AppPassword string     `gorm:"column:app_password"`
	TenantID    string     `gorm:"column:tenant_id"`
	TeamID      string     `gorm:"column:team_id"`
	BotUserID   string     `gorm:"column:bot_user_id"`
	ServiceURL  string     `gorm:"column:service_url"`
	TeamName    string     `gorm:"column:team_name"`
	ConnectedAt time.Time  `gorm:"column:connected_at;autoCreateTime"`
	ConnectedBy *uuid.UUID `gorm:"column:connected_by"`
}

func (TeamsConfig) TableName() string { return "teams_config" }
