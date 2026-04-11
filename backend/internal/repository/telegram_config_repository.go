package repository

import (
	"errors"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TelegramConfigRepository manages Telegram bot configuration.
type TelegramConfigRepository interface {
	Get() (*models.TelegramConfig, error) // nil, nil if not configured
	Save(cfg *models.TelegramConfig) error
	Delete() error
}

type telegramConfigRepository struct{ db *gorm.DB }

func NewTelegramConfigRepository(db *gorm.DB) TelegramConfigRepository {
	return &telegramConfigRepository{db: db}
}

func (r *telegramConfigRepository) Get() (*models.TelegramConfig, error) {
	var cfg models.TelegramConfig
	err := r.db.First(&cfg, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *telegramConfigRepository) Save(cfg *models.TelegramConfig) error {
	cfg.ID = 1
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"bot_token", "chat_id", "chat_name", "connected_at", "connected_by",
		}),
	}).Create(cfg).Error
}

func (r *telegramConfigRepository) Delete() error {
	return r.db.Delete(&models.TelegramConfig{}, 1).Error
}
