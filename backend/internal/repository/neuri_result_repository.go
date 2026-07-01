package repository

import (
	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type NeuriResultRepository interface {
	Create(result *models.NeuriResult) error
	ListByIncidentID(incidentID uuid.UUID) ([]models.NeuriResult, error)
}

type neuriResultRepository struct{ db *gorm.DB }

func NewNeuriResultRepository(db *gorm.DB) NeuriResultRepository {
	return &neuriResultRepository{db: db}
}

func (r *neuriResultRepository) Create(result *models.NeuriResult) error {
	if err := r.db.Create(result).Error; err != nil {
		return &DatabaseError{Op: "create neuri_result", Err: err}
	}
	return nil
}

func (r *neuriResultRepository) ListByIncidentID(incidentID uuid.UUID) ([]models.NeuriResult, error) {
	var results []models.NeuriResult
	if err := r.db.Where("incident_id = ?", incidentID).Order("created_at desc").Find(&results).Error; err != nil {
		return nil, &DatabaseError{Op: "list neuri_results", Err: err}
	}
	return results, nil
}
