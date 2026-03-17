package models

import (
	"time"

	"github.com/google/uuid"
)

// TelegramConfig holds the Telegram bot integration configuration.
// Only one row exists (enforced by CHECK id = 1).
type TelegramConfig struct {
	ID          int        `gorm:"primaryKey;default:1"`
	BotToken    string     `gorm:"column:bot_token"`
	ChatID      string     `gorm:"column:chat_id"`
	ChatName    string     `gorm:"column:chat_name"`
	ConnectedAt time.Time  `gorm:"column:connected_at;autoCreateTime"`
	ConnectedBy *uuid.UUID `gorm:"column:connected_by"`
}

func (TelegramConfig) TableName() string { return "telegram_config" }
