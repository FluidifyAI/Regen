package repository

import (
	"errors"

	"github.com/openincident/openincident/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TeamsConfigRepository manages Teams integration configuration.
type TeamsConfigRepository interface {
	Get() (*models.TeamsConfig, error) // nil, nil if not configured
	Save(cfg *models.TeamsConfig) error
	Delete() error
}

type teamsConfigRepository struct{ db *gorm.DB }

func NewTeamsConfigRepository(db *gorm.DB) TeamsConfigRepository {
	return &teamsConfigRepository{db: db}
}

func (r *teamsConfigRepository) Get() (*models.TeamsConfig, error) {
	var cfg models.TeamsConfig
	err := r.db.First(&cfg, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *teamsConfigRepository) Save(cfg *models.TeamsConfig) error {
	cfg.ID = 1
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"app_id", "app_password", "tenant_id", "team_id",
			"bot_user_id", "service_url", "team_name", "connected_at", "connected_by",
		}),
	}).Create(cfg).Error
}

func (r *teamsConfigRepository) Delete() error {
	return r.db.Delete(&models.TeamsConfig{}, 1).Error
}
