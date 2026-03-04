package models

import (
	"time"

	"github.com/google/uuid"
)

// SlackConfig holds the Slack integration configuration for the entire instance.
// Only one row exists (enforced by CHECK id = 1).
type SlackConfig struct {
	ID                int        `gorm:"primaryKey;default:1"`
	BotToken          string     `gorm:"column:bot_token"`
	SigningSecret     string     `gorm:"column:signing_secret"`
	AppToken          string     `gorm:"column:app_token"`
	WorkspaceID       string     `gorm:"column:workspace_id"`
	WorkspaceName     string     `gorm:"column:workspace_name"`
	BotUserID         string     `gorm:"column:bot_user_id"`
	OAuthClientID     string     `gorm:"column:oauth_client_id"`
	OAuthClientSecret string     `gorm:"column:oauth_client_secret"`
	ConnectedAt       time.Time  `gorm:"column:connected_at;autoCreateTime"`
	ConnectedBy       *uuid.UUID `gorm:"column:connected_by"`
}

func (SlackConfig) TableName() string { return "slack_config" }
