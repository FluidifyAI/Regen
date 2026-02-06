package repository

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/models"
	"gorm.io/gorm"
)

// IncidentRepository defines the interface for incident data access
type IncidentRepository interface {
	Create(incident *models.Incident) error
	GetByID(id uuid.UUID) (*models.Incident, error)
	GetByNumber(number int) (*models.Incident, error)
	List(filters IncidentFilters, pagination Pagination) ([]models.Incident, int64, error)
	Update(incident *models.Incident) error
	UpdateStatus(id uuid.UUID, status models.IncidentStatus) error
	LinkAlert(incidentID, alertID uuid.UUID, linkedByType, linkedByID string) error
	GetAlerts(incidentID uuid.UUID) ([]models.Alert, error)
}

// IncidentFilters holds filter options for listing incidents
type IncidentFilters struct {
	Status    models.IncidentStatus
	Severity  models.IncidentSeverity
	StartDate *time.Time
	EndDate   *time.Time
}

// incidentRepository implements IncidentRepository
type incidentRepository struct {
	db *gorm.DB
}

// NewIncidentRepository creates a new incident repository
func NewIncidentRepository(db *gorm.DB) IncidentRepository {
	return &incidentRepository{db: db}
}

// Create inserts a new incident and returns it with the generated incident_number
func (r *incidentRepository) Create(incident *models.Incident) error {
	if err := r.db.Create(incident).Error; err != nil {
		return &DatabaseError{Op: "create incident", Err: err}
	}
	return nil
}

// GetByID retrieves an incident by UUID
func (r *incidentRepository) GetByID(id uuid.UUID) (*models.Incident, error) {
	var incident models.Incident
	if err := r.db.Where("id = ?", id).First(&incident).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &NotFoundError{Resource: "incident", ID: id}
		}
		return nil, &DatabaseError{Op: "get incident by id", Err: err}
	}
	return &incident, nil
}

// GetByNumber retrieves an incident by incident_number
func (r *incidentRepository) GetByNumber(number int) (*models.Incident, error) {
	var incident models.Incident
	if err := r.db.Where("incident_number = ?", number).First(&incident).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &NotFoundError{Resource: "incident", ID: number}
		}
		return nil, &DatabaseError{Op: "get incident by number", Err: err}
	}
	return &incident, nil
}

// List retrieves incidents with filtering and pagination
func (r *incidentRepository) List(filters IncidentFilters, pagination Pagination) ([]models.Incident, int64, error) {
	var incidents []models.Incident
	var total int64

	query := r.db.Model(&models.Incident{})

	// Apply filters
	if filters.Status != "" {
		query = query.Where("status = ?", filters.Status)
	}
	if filters.Severity != "" {
		query = query.Where("severity = ?", filters.Severity)
	}
	if filters.StartDate != nil {
		query = query.Where("triggered_at >= ?", filters.StartDate)
	}
	if filters.EndDate != nil {
		query = query.Where("triggered_at <= ?", filters.EndDate)
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, &DatabaseError{Op: "count incidents", Err: err}
	}

	// Apply pagination
	offset := (pagination.Page - 1) * pagination.PageSize
	query = query.Offset(offset).Limit(pagination.PageSize)

	// Order by triggered_at descending
	query = query.Order("triggered_at DESC")

	if err := query.Find(&incidents).Error; err != nil {
		return nil, 0, &DatabaseError{Op: "list incidents", Err: err}
	}

	return incidents, total, nil
}

// Update updates mutable fields of an incident
func (r *incidentRepository) Update(incident *models.Incident) error {
	// Only update mutable fields
	updates := map[string]interface{}{
		"title":              incident.Title,
		"slug":               incident.Slug,
		"status":             incident.Status,
		"severity":           incident.Severity,
		"summary":            incident.Summary,
		"slack_channel_id":   incident.SlackChannelID,
		"slack_channel_name": incident.SlackChannelName,
		"acknowledged_at":    incident.AcknowledgedAt,
		"resolved_at":        incident.ResolvedAt,
		"commander_id":       incident.CommanderID,
		"labels":             incident.Labels,
		"custom_fields":      incident.CustomFields,
	}

	if err := r.db.Model(incident).Updates(updates).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &NotFoundError{Resource: "incident", ID: incident.ID}
		}
		return &DatabaseError{Op: "update incident", Err: err}
	}

	return nil
}

// UpdateStatus updates the incident status and sets appropriate timestamps
func (r *incidentRepository) UpdateStatus(id uuid.UUID, status models.IncidentStatus) error {
	updates := map[string]interface{}{
		"status": status,
	}

	// Set timestamps based on status transition
	now := time.Now()
	switch status {
	case models.IncidentStatusAcknowledged:
		updates["acknowledged_at"] = now
	case models.IncidentStatusResolved, models.IncidentStatusCanceled:
		updates["resolved_at"] = now
	}

	if err := r.db.Model(&models.Incident{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &NotFoundError{Resource: "incident", ID: id}
		}
		return &DatabaseError{Op: "update incident status", Err: err}
	}

	return nil
}

// LinkAlert creates an incident_alert association
func (r *incidentRepository) LinkAlert(incidentID, alertID uuid.UUID, linkedByType, linkedByID string) error {
	link := map[string]interface{}{
		"incident_id":    incidentID,
		"alert_id":       alertID,
		"linked_by_type": linkedByType,
		"linked_by_id":   linkedByID,
	}

	if err := r.db.Table("incident_alerts").Create(link).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return &AlreadyExistsError{
				Resource: "incident_alert",
				Field:    "incident_id,alert_id",
				Value:    "",
			}
		}
		return &DatabaseError{Op: "link alert to incident", Err: err}
	}

	return nil
}

// GetAlerts retrieves all alerts linked to an incident
func (r *incidentRepository) GetAlerts(incidentID uuid.UUID) ([]models.Alert, error) {
	var alerts []models.Alert

	if err := r.db.
		Joins("JOIN incident_alerts ON incident_alerts.alert_id = alerts.id").
		Where("incident_alerts.incident_id = ?", incidentID).
		Order("alerts.received_at ASC").
		Find(&alerts).Error; err != nil {
		return nil, &DatabaseError{Op: "get incident alerts", Err: err}
	}

	return alerts, nil
}
