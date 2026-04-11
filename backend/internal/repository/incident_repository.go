package repository

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/FluidifyAI/Regen/backend/internal/models"
	"gorm.io/gorm"
)

// IncidentRepository defines the interface for incident data access
type IncidentRepository interface {
	Create(incident *models.Incident) error
	GetByID(id uuid.UUID) (*models.Incident, error)
	GetByNumber(number int) (*models.Incident, error)
	GetBySlackChannelID(channelID string) (*models.Incident, error)
	GetByTeamsChannelID(channelID string) (*models.Incident, error)
	GetByTeamsConversationID(conversationID string) (*models.Incident, error)
	List(filters IncidentFilters, pagination Pagination) ([]models.Incident, int64, error)
	Update(incident *models.Incident) error
	UpdateStatus(id uuid.UUID, status models.IncidentStatus) error
	UpdateSlackChannel(id uuid.UUID, channelID, channelName string) error
	UpdateSlackMessageTS(id uuid.UUID, messageTS string) error
	UpdateTeamsChannel(id uuid.UUID, channelID, channelName string) error
	UpdateTeamsConversationID(id uuid.UUID, conversationID string) error
	UpdateTeamsActivityID(id uuid.UUID, activityID string) error
	UpdateTeamsPostingIDs(id uuid.UUID, conversationID, activityID string) error
	LinkAlert(incidentID, alertID uuid.UUID, linkedByType, linkedByID string) error
	GetAlerts(incidentID uuid.UUID) ([]models.Alert, error)
	GetIncidentByAlertID(alertID uuid.UUID) (*models.Incident, error)
	UpdateAISummary(id uuid.UUID, summary string, generatedAt time.Time) error
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

// GetBySlackChannelID retrieves an incident by its Slack channel ID
func (r *incidentRepository) GetBySlackChannelID(channelID string) (*models.Incident, error) {
	var incident models.Incident
	if err := r.db.Where("slack_channel_id = ?", channelID).First(&incident).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // Not found is not an error — caller checks nil
		}
		return nil, &DatabaseError{Op: "get incident by slack channel id", Err: err}
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
		"teams_channel_id":      incident.TeamsChannelID,
		"teams_channel_name":    incident.TeamsChannelName,
		"teams_conversation_id": incident.TeamsConversationID,
		"teams_activity_id":     incident.TeamsActivityID,
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

// UpdateSlackChannel updates the Slack channel information for an incident
func (r *incidentRepository) UpdateSlackChannel(id uuid.UUID, channelID, channelName string) error {
	updates := map[string]interface{}{
		"slack_channel_id":   channelID,
		"slack_channel_name": channelName,
	}

	if err := r.db.Model(&models.Incident{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &NotFoundError{Resource: "incident", ID: id}
		}
		return &DatabaseError{Op: "update incident slack channel", Err: err}
	}

	return nil
}

// UpdateSlackMessageTS stores the Slack message timestamp for the incident's pinned card.
func (r *incidentRepository) UpdateSlackMessageTS(id uuid.UUID, messageTS string) error {
	if err := r.db.Model(&models.Incident{}).Where("id = ?", id).
		Update("slack_message_ts", messageTS).Error; err != nil {
		return &DatabaseError{Op: "update incident slack message ts", Err: err}
	}
	return nil
}

// GetByTeamsChannelID retrieves an incident by its Teams channel ID.
// Returns nil (not an error) when not found — caller checks nil.
func (r *incidentRepository) GetByTeamsChannelID(channelID string) (*models.Incident, error) {
	var incident models.Incident
	if err := r.db.Where("teams_channel_id = ?", channelID).First(&incident).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, &DatabaseError{Op: "get incident by teams channel id", Err: err}
	}
	return &incident, nil
}

// GetByTeamsConversationID retrieves an incident by its Bot Framework conversation ID.
// Returns nil (not an error) when not found — caller checks nil.
// Bot Framework inbound activities use Conversation.ID (the conversation ID, format a:xxx),
// which is distinct from the Teams channel ID (19:xxx@thread.tacv2) stored in teams_channel_id.
func (r *incidentRepository) GetByTeamsConversationID(conversationID string) (*models.Incident, error) {
	var incident models.Incident
	if err := r.db.Where("teams_conversation_id = ?", conversationID).First(&incident).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, &DatabaseError{Op: "get incident by teams conversation id", Err: err}
	}
	return &incident, nil
}

// UpdateTeamsChannel stores the Teams channel ID and name for an incident.
func (r *incidentRepository) UpdateTeamsChannel(id uuid.UUID, channelID, channelName string) error {
	if err := r.db.Model(&models.Incident{}).Where("id = ?", id).Updates(map[string]interface{}{
		"teams_channel_id":   channelID,
		"teams_channel_name": channelName,
	}).Error; err != nil {
		return &DatabaseError{Op: "update incident teams channel", Err: err}
	}
	return nil
}

// UpdateTeamsConversationID stores the Bot Framework conversation ID for an incident channel.
// This is separate from the Teams channel ID and is required for proactive messaging (v0.9+).
func (r *incidentRepository) UpdateTeamsConversationID(id uuid.UUID, conversationID string) error {
	if err := r.db.Model(&models.Incident{}).Where("id = ?", id).
		Update("teams_conversation_id", conversationID).Error; err != nil {
		return &DatabaseError{Op: "update incident teams conversation id", Err: err}
	}
	return nil
}

// UpdateTeamsPostingIDs atomically stores both the Bot Framework conversation ID and the
// root activity ID in a single UPDATE statement. Avoids a partial-write state where
// conversationID is stored but activityID is not, which would silently prevent card updates.
func (r *incidentRepository) UpdateTeamsPostingIDs(id uuid.UUID, conversationID, activityID string) error {
	if err := r.db.Model(&models.Incident{}).Where("id = ?", id).Updates(map[string]interface{}{
		"teams_conversation_id": conversationID,
		"teams_activity_id":     activityID,
	}).Error; err != nil {
		return &DatabaseError{Op: "update incident teams posting ids", Err: err}
	}
	return nil
}

// UpdateTeamsActivityID stores the Teams root activity ID (used to update the incident card).
func (r *incidentRepository) UpdateTeamsActivityID(id uuid.UUID, activityID string) error {
	if err := r.db.Model(&models.Incident{}).Where("id = ?", id).
		Update("teams_activity_id", activityID).Error; err != nil {
		return &DatabaseError{Op: "update incident teams activity id", Err: err}
	}
	return nil
}

// UpdateAISummary persists an AI-generated summary and its generation timestamp.
func (r *incidentRepository) UpdateAISummary(id uuid.UUID, summary string, generatedAt time.Time) error {
	if err := r.db.Model(&models.Incident{}).Where("id = ?", id).Updates(map[string]interface{}{
		"ai_summary":              summary,
		"ai_summary_generated_at": generatedAt,
	}).Error; err != nil {
		return &DatabaseError{Op: "update incident ai summary", Err: err}
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

// GetAlerts retrieves alerts linked to an incident, most recent first, capped at 500.
// Incidents can accumulate thousands of alerts during large outages; an unbounded
// query would load all of them into memory and slow the response significantly.
func (r *incidentRepository) GetAlerts(incidentID uuid.UUID) ([]models.Alert, error) {
	var alerts []models.Alert

	if err := r.db.
		Joins("JOIN incident_alerts ON incident_alerts.alert_id = alerts.id").
		Where("incident_alerts.incident_id = ?", incidentID).
		Order("alerts.received_at ASC").
		Limit(500).
		Find(&alerts).Error; err != nil {
		return nil, &DatabaseError{Op: "get incident alerts", Err: err}
	}

	return alerts, nil
}

// GetIncidentByAlertID finds the incident that a given alert is linked to.
// Returns NotFoundError if the alert is not linked to any incident.
func (r *incidentRepository) GetIncidentByAlertID(alertID uuid.UUID) (*models.Incident, error) {
	var incident models.Incident
	err := r.db.
		Joins("JOIN incident_alerts ON incident_alerts.incident_id = incidents.id").
		Where("incident_alerts.alert_id = ?", alertID).
		First(&incident).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &NotFoundError{Resource: "incident", ID: alertID.String()}
		}
		return nil, &DatabaseError{Op: "get incident by alert id", Err: err}
	}
	return &incident, nil
}
