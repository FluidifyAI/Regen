package services

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/repository"
	"gorm.io/gorm"
)

// CreateIncidentParams holds parameters for creating a manual incident
type CreateIncidentParams struct {
	Title       string
	Severity    models.IncidentSeverity
	Description string
	CreatedBy   string // "user", "system", "api"
}

// UpdateIncidentParams holds parameters for updating an incident
type UpdateIncidentParams struct {
	Status    models.IncidentStatus
	Severity  models.IncidentSeverity
	Summary   string
	UpdatedBy string
	ClientIP  string // For audit logging
}

// CreateTimelineEntryParams holds parameters for creating a timeline entry
type CreateTimelineEntryParams struct {
	IncidentID uuid.UUID
	Type       string
	Content    models.JSONB
	ActorType  string
	ActorID    string
}

// IncidentService defines the interface for incident operations
type IncidentService interface {
	// Alert-triggered incident creation
	CreateIncidentFromAlert(alert *models.Alert) (*models.Incident, error)
	ShouldCreateIncident(severity models.AlertSeverity) bool
	CreateSlackChannelForIncident(incident *models.Incident, alerts []models.Alert) error

	// API operations
	ListIncidents(filters repository.IncidentFilters, pagination repository.Pagination) ([]models.Incident, int64, error)
	GetIncident(id uuid.UUID, number int) (*models.Incident, error)
	CreateIncident(params *CreateIncidentParams) (*models.Incident, error)
	UpdateIncident(id uuid.UUID, params *UpdateIncidentParams) (*models.Incident, error)
	GetIncidentAlerts(incidentID uuid.UUID) ([]models.Alert, error)
	GetIncidentTimeline(incidentID uuid.UUID, pagination repository.Pagination) ([]models.TimelineEntry, int64, error)
	CreateTimelineEntry(params *CreateTimelineEntryParams) (*models.TimelineEntry, error)

	// Slack notifications
	PostStatusUpdateToSlack(incident *models.Incident, previousStatus, newStatus models.IncidentStatus) error
}

// incidentService implements IncidentService
type incidentService struct {
	incidentRepo   repository.IncidentRepository
	timelineRepo   repository.TimelineRepository
	alertRepo      repository.AlertRepository
	chatService    ChatService          // Optional - can be nil if Slack not configured
	messageBuilder *SlackMessageBuilder // Optional - can be nil if Slack not configured
	db             *gorm.DB             // For transaction management
}

// NewIncidentService creates a new incident service
func NewIncidentService(
	incidentRepo repository.IncidentRepository,
	timelineRepo repository.TimelineRepository,
	alertRepo repository.AlertRepository,
	chatService ChatService, // Optional - pass nil if Slack not configured
	db *gorm.DB,
) IncidentService {
	var messageBuilder *SlackMessageBuilder
	if chatService != nil {
		messageBuilder = NewSlackMessageBuilder()
	}

	return &incidentService{
		incidentRepo:   incidentRepo,
		timelineRepo:   timelineRepo,
		alertRepo:      alertRepo,
		chatService:    chatService,
		messageBuilder: messageBuilder,
		db:             db,
	}
}

// ShouldCreateIncident determines if an alert should trigger incident creation
func (s *incidentService) ShouldCreateIncident(severity models.AlertSeverity) bool {
	switch severity {
	case models.AlertSeverityCritical, models.AlertSeverityWarning:
		return true
	case models.AlertSeverityInfo:
		return false
	default:
		return false
	}
}

// CreateIncidentFromAlert creates an incident from an alert with full transaction support
func (s *incidentService) CreateIncidentFromAlert(alert *models.Alert) (*models.Incident, error) {
	// Map alert severity to incident severity
	incidentSeverity := mapAlertSeverityToIncident(alert.Severity)

	// Generate slug from title
	slug := generateSlug(alert.Title)

	// Create incident object
	incident := &models.Incident{
		ID:            uuid.New(),
		Title:         alert.Title,
		Slug:          slug,
		Status:        models.IncidentStatusTriggered,
		Severity:      incidentSeverity,
		Summary:       alert.Description,
		CreatedByType: "system",
		CreatedByID:   "alertmanager",
		TriggeredAt:   time.Now(),
		Labels:        make(models.JSONB),
		CustomFields:  make(models.JSONB),
	}

	// Execute all operations in a transaction for atomicity
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Step 1: Create the incident
		if err := s.incidentRepo.Create(incident); err != nil {
			return fmt.Errorf("failed to create incident: %w", err)
		}

		// Step 2: Link the alert to the incident
		if err := s.incidentRepo.LinkAlert(incident.ID, alert.ID, "system", "alertmanager"); err != nil {
			return fmt.Errorf("failed to link alert to incident: %w", err)
		}

		// Step 3: Create timeline entry for incident creation
		timelineEntry := &models.TimelineEntry{
			ID:         uuid.New(),
			IncidentID: incident.ID,
			Timestamp:  time.Now(),
			Type:       "incident_created",
			ActorType:  "system",
			ActorID:    "alertmanager",
			Content: models.JSONB{
				"trigger":  "alert",
				"alert_id": alert.ID.String(),
				"source":   alert.Source,
			},
		}

		if err := s.timelineRepo.Create(timelineEntry); err != nil {
			return fmt.Errorf("failed to create timeline entry: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	// Create Slack channel asynchronously (non-blocking)
	if s.chatService != nil {
		go func() {
			alerts := []models.Alert{*alert}
			if err := s.CreateSlackChannelForIncident(incident, alerts); err != nil {
				slog.Error("failed to create slack channel",
					"incident_id", incident.ID,
					"incident_number", incident.IncidentNumber,
					"error", err)
			}
		}()
	}

	return incident, nil
}

// CreateSlackChannelForIncident creates a Slack channel for an incident and posts the initial message.
// This method is called asynchronously after incident creation to avoid blocking the webhook response.
func (s *incidentService) CreateSlackChannelForIncident(incident *models.Incident, alerts []models.Alert) error {
	if s.chatService == nil {
		return fmt.Errorf("slack service not configured")
	}

	// Format channel name using incident number and slug
	channelName := formatIncidentChannelName(incident.IncidentNumber, incident.Slug)
	description := fmt.Sprintf("Incident #%d: %s", incident.IncidentNumber, incident.Title)

	slog.Info("creating slack channel for incident",
		"incident_id", incident.ID,
		"incident_number", incident.IncidentNumber,
		"channel_name", channelName)

	// Create channel
	channel, err := s.chatService.CreateChannel(channelName, description)
	if err != nil {
		// Create timeline entry for failure
		s.createTimelineEntry(incident.ID, "slack_channel_creation_failed", models.JSONB{
			"error":        err.Error(),
			"channel_name": channelName,
		})
		return fmt.Errorf("failed to create slack channel: %w", err)
	}

	// Update incident with Slack channel details
	err = s.incidentRepo.UpdateSlackChannel(incident.ID, channel.ID, channel.Name)
	if err != nil {
		slog.Error("failed to update incident with slack channel",
			"incident_id", incident.ID,
			"channel_id", channel.ID,
			"error", err)
		// Continue - channel was created successfully, this is just a DB update issue
	}

	// Post initial message
	message := s.messageBuilder.BuildIncidentCreatedMessage(incident, alerts)
	_, err = s.chatService.PostMessage(channel.ID, message)
	if err != nil {
		slog.Error("failed to post initial message to slack channel",
			"incident_id", incident.ID,
			"channel_id", channel.ID,
			"error", err)
		// Continue - channel was created, message posting is non-critical
	}

	// Create timeline entry for success
	s.createTimelineEntry(incident.ID, "slack_channel_created", models.JSONB{
		"channel_id":   channel.ID,
		"channel_name": channel.Name,
		"channel_url":  channel.URL,
	})

	slog.Info("slack channel created for incident",
		"incident_id", incident.ID,
		"incident_number", incident.IncidentNumber,
		"channel_id", channel.ID,
		"channel_name", channel.Name,
		"channel_url", channel.URL)

	return nil
}

// createTimelineEntry creates a timeline entry without failing if it errors.
// Timeline entries are important but not critical enough to fail the entire operation.
func (s *incidentService) createTimelineEntry(incidentID uuid.UUID, entryType string, content models.JSONB) {
	entry := &models.TimelineEntry{
		ID:         uuid.New(),
		IncidentID: incidentID,
		Timestamp:  time.Now(),
		Type:       entryType,
		ActorType:  "system",
		ActorID:    "slack_service",
		Content:    content,
	}

	if err := s.timelineRepo.Create(entry); err != nil {
		slog.Error("failed to create timeline entry",
			"incident_id", incidentID,
			"type", entryType,
			"error", err)
	}
}

// mapAlertSeverityToIncident maps alert severity to incident severity
func mapAlertSeverityToIncident(alertSeverity models.AlertSeverity) models.IncidentSeverity {
	switch alertSeverity {
	case models.AlertSeverityCritical:
		return models.IncidentSeverityCritical
	case models.AlertSeverityWarning:
		return models.IncidentSeverityHigh
	case models.AlertSeverityInfo:
		return models.IncidentSeverityMedium
	default:
		return models.IncidentSeverityMedium
	}
}

// generateSlug creates a URL-safe slug from a title
// - Converts to lowercase
// - Replaces spaces and special characters with hyphens
// - Removes consecutive hyphens
// - Truncates to 50 characters max
func generateSlug(title string) string {
	// Convert to lowercase
	slug := strings.ToLower(title)

	// Replace spaces and underscores with hyphens
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")

	// Remove all non-alphanumeric characters except hyphens
	reg := regexp.MustCompile(`[^a-z0-9-]+`)
	slug = reg.ReplaceAllString(slug, "")

	// Remove consecutive hyphens
	reg = regexp.MustCompile(`-+`)
	slug = reg.ReplaceAllString(slug, "-")

	// Trim hyphens from start and end
	slug = strings.Trim(slug, "-")

	// Truncate to 50 characters
	if len(slug) > 50 {
		slug = slug[:50]
		// Remove trailing hyphen if truncation created one
		slug = strings.TrimRight(slug, "-")
	}

	// If slug is empty after sanitization, use a default
	if slug == "" {
		slug = "incident"
	}

	return slug
}

// ListIncidents retrieves incidents with filtering and pagination
func (s *incidentService) ListIncidents(filters repository.IncidentFilters, pagination repository.Pagination) ([]models.Incident, int64, error) {
	return s.incidentRepo.List(filters, pagination)
}

// GetIncident retrieves an incident by UUID or incident number
func (s *incidentService) GetIncident(id uuid.UUID, number int) (*models.Incident, error) {
	if id != uuid.Nil {
		return s.incidentRepo.GetByID(id)
	}
	return s.incidentRepo.GetByNumber(number)
}

// CreateIncident creates a new incident manually (not from an alert)
func (s *incidentService) CreateIncident(params *CreateIncidentParams) (*models.Incident, error) {
	// Generate slug
	slug := generateSlug(params.Title)

	// Create incident
	incident := &models.Incident{
		ID:            uuid.New(),
		Title:         params.Title,
		Slug:          slug,
		Status:        models.IncidentStatusTriggered,
		Severity:      params.Severity,
		Summary:       params.Description,
		CreatedByType: "user",
		CreatedByID:   params.CreatedBy,
		TriggeredAt:   time.Now(),
		Labels:        make(models.JSONB),
		CustomFields:  make(models.JSONB),
	}

	// Execute in transaction
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Create incident
		if err := s.incidentRepo.Create(incident); err != nil {
			return fmt.Errorf("failed to create incident: %w", err)
		}

		// Create timeline entry
		timelineEntry := &models.TimelineEntry{
			ID:         uuid.New(),
			IncidentID: incident.ID,
			Timestamp:  time.Now(),
			Type:       models.TimelineTypeIncidentCreated,
			ActorType:  "user",
			ActorID:    params.CreatedBy,
			Content: models.JSONB{
				"trigger": "manual",
			},
		}

		if err := s.timelineRepo.Create(timelineEntry); err != nil {
			return fmt.Errorf("failed to create timeline entry: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Reload incident to get trigger-assigned fields (e.g., incident_number)
	reloadedIncident, err := s.incidentRepo.GetByID(incident.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to reload incident after creation: %w", err)
	}

	// Create Slack channel asynchronously
	if s.chatService != nil {
		go func() {
			if err := s.CreateSlackChannelForIncident(reloadedIncident, []models.Alert{}); err != nil {
				slog.Error("failed to create slack channel",
					"incident_id", reloadedIncident.ID,
					"error", err)
			}
		}()
	}

	return reloadedIncident, nil
}

// UpdateIncident updates an incident and creates timeline entries for changes
func (s *incidentService) UpdateIncident(id uuid.UUID, params *UpdateIncidentParams) (*models.Incident, error) {
	// Fetch current incident
	incident, err := s.incidentRepo.GetByID(id)
	if err != nil {
		return nil, err
	}

	previousStatus := incident.Status
	previousSeverity := incident.Severity

	// Execute update in transaction
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// Update status if provided
		if params.Status != "" && params.Status != incident.Status {
			if err := s.incidentRepo.UpdateStatus(id, params.Status); err != nil {
				return err
			}

			// Audit log: Record status change with actor and IP
			slog.Info("incident status changed",
				"incident_id", id,
				"incident_number", incident.IncidentNumber,
				"previous_status", string(previousStatus),
				"new_status", string(params.Status),
				"actor", params.UpdatedBy,
				"client_ip", params.ClientIP,
				"audit", true, // Tag for audit log filtering
			)

			// Create timeline entry for status change
			timelineEntry := &models.TimelineEntry{
				ID:         uuid.New(),
				IncidentID: id,
				Timestamp:  time.Now(),
				Type:       models.TimelineTypeStatusChanged,
				ActorType:  "user",
				ActorID:    params.UpdatedBy,
				Content: models.JSONB{
					"previous_status": string(previousStatus),
					"new_status":      string(params.Status),
					"client_ip":       params.ClientIP,
				},
			}
			if err := s.timelineRepo.Create(timelineEntry); err != nil {
				return err
			}
		}

		// Update severity if provided
		if params.Severity != "" && params.Severity != incident.Severity {
			incident.Severity = params.Severity

			// Audit log: Record severity change with actor and IP
			slog.Info("incident severity changed",
				"incident_id", id,
				"incident_number", incident.IncidentNumber,
				"previous_severity", string(previousSeverity),
				"new_severity", string(params.Severity),
				"actor", params.UpdatedBy,
				"client_ip", params.ClientIP,
				"audit", true, // Tag for audit log filtering
			)

			// Create timeline entry for severity change
			timelineEntry := &models.TimelineEntry{
				ID:         uuid.New(),
				IncidentID: id,
				Timestamp:  time.Now(),
				Type:       models.TimelineTypeSeverityChanged,
				ActorType:  "user",
				ActorID:    params.UpdatedBy,
				Content: models.JSONB{
					"previous_severity": string(previousSeverity),
					"new_severity":      string(params.Severity),
					"client_ip":         params.ClientIP,
				},
			}
			if err := s.timelineRepo.Create(timelineEntry); err != nil {
				return err
			}
		}

		// Update summary if provided
		if params.Summary != "" {
			incident.Summary = params.Summary
		}

		// Update incident
		if err := s.incidentRepo.Update(incident); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Post Slack notification asynchronously
	if params.Status != "" && params.Status != previousStatus && s.chatService != nil {
		go func() {
			if err := s.PostStatusUpdateToSlack(incident, previousStatus, params.Status); err != nil {
				slog.Error("failed to post slack notification", "error", err)
			}
		}()
	}

	// Fetch updated incident
	return s.incidentRepo.GetByID(id)
}

// GetIncidentAlerts retrieves all alerts linked to an incident
func (s *incidentService) GetIncidentAlerts(incidentID uuid.UUID) ([]models.Alert, error) {
	return s.incidentRepo.GetAlerts(incidentID)
}

// GetIncidentTimeline retrieves timeline entries for an incident with pagination
func (s *incidentService) GetIncidentTimeline(incidentID uuid.UUID, pagination repository.Pagination) ([]models.TimelineEntry, int64, error) {
	return s.timelineRepo.GetByIncidentID(incidentID, pagination)
}

// CreateTimelineEntry creates a new timeline entry (e.g., user note)
func (s *incidentService) CreateTimelineEntry(params *CreateTimelineEntryParams) (*models.TimelineEntry, error) {
	entry := &models.TimelineEntry{
		ID:         uuid.New(),
		IncidentID: params.IncidentID,
		Timestamp:  time.Now(),
		Type:       params.Type,
		ActorType:  params.ActorType,
		ActorID:    params.ActorID,
		Content:    params.Content,
	}

	if err := s.timelineRepo.Create(entry); err != nil {
		return nil, err
	}

	return entry, nil
}

// PostStatusUpdateToSlack posts a status update message to the incident's Slack channel
func (s *incidentService) PostStatusUpdateToSlack(
	incident *models.Incident,
	previousStatus models.IncidentStatus,
	newStatus models.IncidentStatus,
) error {
	if s.chatService == nil {
		return fmt.Errorf("slack service not configured")
	}

	if incident.SlackChannelID == "" {
		return fmt.Errorf("incident has no slack channel")
	}

	message := s.messageBuilder.BuildStatusUpdateMessage(incident, previousStatus, newStatus)

	_, err := s.chatService.PostMessage(incident.SlackChannelID, message)
	if err != nil {
		return fmt.Errorf("failed to post status update to slack: %w", err)
	}

	slog.Info("posted status update to slack",
		"incident_id", incident.ID,
		"incident_number", incident.IncidentNumber,
		"previous_status", previousStatus,
		"new_status", newStatus,
		"channel_id", incident.SlackChannelID,
	)

	return nil
}
