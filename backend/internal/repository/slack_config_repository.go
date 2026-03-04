package repository

import (
	"errors"

	"github.com/openincident/openincident/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SlackConfigRepository manages Slack integration configuration.
type SlackConfigRepository interface {
	Get() (*models.SlackConfig, error) // nil, nil if not configured
	Save(cfg *models.SlackConfig) error
	Delete() error
}

type slackConfigRepository struct{ db *gorm.DB }

// NewSlackConfigRepository creates a new SlackConfigRepository.
func NewSlackConfigRepository(db *gorm.DB) SlackConfigRepository {
	return &slackConfigRepository{db: db}
}

func (r *slackConfigRepository) Get() (*models.SlackConfig, error) {
	var cfg models.SlackConfig
	err := r.db.First(&cfg, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *slackConfigRepository) Save(cfg *models.SlackConfig) error {
	cfg.ID = 1
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"bot_token", "signing_secret", "app_token", "workspace_id", "workspace_name", "bot_user_id", "oauth_client_id", "oauth_client_secret", "connected_at", "connected_by"}),
	}).Create(cfg).Error
}

func (r *slackConfigRepository) Delete() error {
	return r.db.Delete(&models.SlackConfig{}, 1).Error
}
