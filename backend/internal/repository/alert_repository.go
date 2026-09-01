package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AlertRepository defines the interface for alert data access.
//
// Create, GetByExternalID, and Update take ctx (REG-157): they sit on the
// alert-webhook -> incident-create path, so threading ctx into db.WithContext
// lets the gorm tracing plugin (REG-9) attach a real child span. GetByID and
// List do not yet — same mixed pattern already established by
// user_repository.go's Upsert, not a uniform migration of every method.
type AlertRepository interface {
	Create(ctx context.Context, alert *models.Alert) error
	GetByID(id uuid.UUID) (*models.Alert, error)
	GetByExternalID(ctx context.Context, source, externalID string) (*models.Alert, error)
	List(filters AlertFilters, pagination Pagination) ([]models.Alert, int64, error)
	Update(ctx context.Context, alert *models.Alert) error
}

// AlertFilters holds filter options for listing alerts
type AlertFilters struct {
	Source   string
	Status   models.AlertStatus
	Severity models.AlertSeverity
}

// Pagination holds pagination parameters
type Pagination struct {
	Page     int
	PageSize int
}

// alertRepository implements AlertRepository
type alertRepository struct {
	db *gorm.DB
}

// NewAlertRepository creates a new alert repository
func NewAlertRepository(db *gorm.DB) AlertRepository {
	return &alertRepository{db: db}
}

// Create inserts a new alert
func (r *alertRepository) Create(ctx context.Context, alert *models.Alert) error {
	if err := r.db.WithContext(ctx).Create(alert).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return &AlreadyExistsError{
				Resource: "alert",
				Field:    "source,external_id",
				Value:    fmt.Sprintf("%s,%s", alert.Source, alert.ExternalID),
			}
		}
		return &DatabaseError{Op: "create alert", Err: err}
	}
	return nil
}

// GetByID retrieves an alert by UUID
func (r *alertRepository) GetByID(id uuid.UUID) (*models.Alert, error) {
	var alert models.Alert
	if err := r.db.Where("id = ?", id).First(&alert).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &NotFoundError{Resource: "alert", ID: id}
		}
		return nil, &DatabaseError{Op: "get alert by id", Err: err}
	}
	return &alert, nil
}

// GetByExternalID retrieves an alert by source and external ID
func (r *alertRepository) GetByExternalID(ctx context.Context, source, externalID string) (*models.Alert, error) {
	var alert models.Alert
	if err := r.db.WithContext(ctx).Where("source = ? AND external_id = ?", source, externalID).First(&alert).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &NotFoundError{
				Resource: "alert",
				ID:       fmt.Sprintf("source=%s,external_id=%s", source, externalID),
			}
		}
		return nil, &DatabaseError{Op: "get alert by external id", Err: err}
	}
	return &alert, nil
}

// List retrieves alerts with filtering and pagination
func (r *alertRepository) List(filters AlertFilters, pagination Pagination) ([]models.Alert, int64, error) {
	var alerts []models.Alert
	var total int64

	query := r.db.Model(&models.Alert{})

	// Apply filters
	if filters.Source != "" {
		query = query.Where("source = ?", filters.Source)
	}
	if filters.Status != "" {
		query = query.Where("status = ?", filters.Status)
	}
	if filters.Severity != "" {
		query = query.Where("severity = ?", filters.Severity)
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, &DatabaseError{Op: "count alerts", Err: err}
	}

	// Apply pagination
	offset := (pagination.Page - 1) * pagination.PageSize
	query = query.Offset(offset).Limit(pagination.PageSize)

	// Order by received_at descending
	query = query.Order("received_at DESC")

	if err := query.Find(&alerts).Error; err != nil {
		return nil, 0, &DatabaseError{Op: "list alerts", Err: err}
	}

	return alerts, total, nil
}

// Update updates mutable fields of an alert
func (r *alertRepository) Update(ctx context.Context, alert *models.Alert) error {
	// Only allow updating specific mutable fields
	updates := map[string]interface{}{
		"status":               alert.Status,
		"title":                alert.Title,
		"description":          alert.Description,
		"ended_at":             alert.EndedAt,
		"escalation_policy_id": alert.EscalationPolicyID,
	}

	if err := r.db.WithContext(ctx).Model(alert).Updates(updates).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &NotFoundError{Resource: "alert", ID: alert.ID}
		}
		return &DatabaseError{Op: "update alert", Err: err}
	}

	return nil
}
