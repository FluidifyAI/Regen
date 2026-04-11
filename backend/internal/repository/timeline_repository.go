package repository

import (
	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TimelineRepository defines the interface for timeline entry data access
type TimelineRepository interface {
	Create(entry *models.TimelineEntry) error
	CreateBulk(entries []models.TimelineEntry) error
	GetByIncidentID(incidentID uuid.UUID, pagination Pagination) ([]models.TimelineEntry, int64, error)
}

// timelineRepository implements TimelineRepository
type timelineRepository struct {
	db *gorm.DB
}

// NewTimelineRepository creates a new timeline repository
func NewTimelineRepository(db *gorm.DB) TimelineRepository {
	return &timelineRepository{db: db}
}

// Create inserts a new timeline entry (append-only)
func (r *timelineRepository) Create(entry *models.TimelineEntry) error {
	if err := r.db.Create(entry).Error; err != nil {
		return &DatabaseError{Op: "create timeline entry", Err: err}
	}
	return nil
}

// CreateBulk inserts multiple timeline entries (bulk import)
func (r *timelineRepository) CreateBulk(entries []models.TimelineEntry) error {
	if len(entries) == 0 {
		return nil
	}

	if err := r.db.CreateInBatches(entries, 100).Error; err != nil {
		return &DatabaseError{Op: "bulk create timeline entries", Err: err}
	}

	return nil
}

// GetByIncidentID retrieves timeline entries for an incident, ordered by timestamp
func (r *timelineRepository) GetByIncidentID(incidentID uuid.UUID, pagination Pagination) ([]models.TimelineEntry, int64, error) {
	var entries []models.TimelineEntry
	var total int64

	query := r.db.Model(&models.TimelineEntry{}).Where("incident_id = ?", incidentID)

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, &DatabaseError{Op: "count timeline entries", Err: err}
	}

	// Apply pagination
	offset := (pagination.Page - 1) * pagination.PageSize
	query = query.Offset(offset).Limit(pagination.PageSize)

	// Order by timestamp ascending (chronological)
	query = query.Order("timestamp ASC")

	if err := query.Find(&entries).Error; err != nil {
		return nil, 0, &DatabaseError{Op: "get timeline entries", Err: err}
	}

	return entries, total, nil
}
