package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
	CreateIncidentFromAlertWithGrouping(alert *models.Alert, groupKey string) (*models.Incident, error)
	LinkAlertToExistingIncident(alert *models.Alert, incidentID uuid.UUID) error
	ShouldCreateIncident(severity models.AlertSeverity) bool
	CreateSlackChannelForIncident(incident *models.Incident, alerts []models.Alert) error

	// API operations
	ListIncidents(filters repository.IncidentFilters, pagination repository.Pagination) ([]models.Incident, int64, error)
	GetIncident(id uuid.UUID, number int) (*models.Incident, error)
	GetIncidentBySlackChannelID(channelID string) (*models.Incident, error)
	CreateIncident(params *CreateIncidentParams) (*models.Incident, error)
	UpdateIncident(id uuid.UUID, params *UpdateIncidentParams) (*models.Incident, error)
	GetIncidentAlerts(incidentID uuid.UUID) ([]models.Alert, error)
	GetIncidentTimeline(incidentID uuid.UUID, pagination repository.Pagination) ([]models.TimelineEntry, int64, error)
	CreateTimelineEntry(params *CreateTimelineEntryParams) (*models.TimelineEntry, error)

	// Slack notifications
	PostStatusUpdateToSlack(incident *models.Incident, previousStatus, newStatus models.IncidentStatus) error

	// AI features (v0.6+)
	// GenerateAISummary generates an AI summary for an incident and persists it.
	GenerateAISummary(incident *models.Incident) (string, error)
	// GenerateHandoffDigest generates a shift handoff digest for an incident (not persisted).
	GenerateHandoffDigest(incident *models.Incident) (string, error)
}

// incidentService implements IncidentService
type incidentService struct {
	incidentRepo      repository.IncidentRepository
	timelineRepo      repository.TimelineRepository
	alertRepo         repository.AlertRepository
	chatService       ChatService          // Optional - can be nil if Slack not configured
	messageBuilder    *SlackMessageBuilder // Optional - can be nil if Slack not configured
	db                *gorm.DB             // For transaction management
	autoInviteUserIDs []string             // User IDs to auto-invite to incident channels
	aiService         AIService            // Optional - can be nil if OpenAI not configured
}

// NewIncidentService creates a new incident service
func NewIncidentService(
	incidentRepo repository.IncidentRepository,
	timelineRepo repository.TimelineRepository,
	alertRepo repository.AlertRepository,
	chatService ChatService, // Optional - pass nil if Slack not configured
	db *gorm.DB,
	autoInviteUserIDs []string,
) IncidentService {
	var messageBuilder *SlackMessageBuilder
	if chatService != nil {
		messageBuilder = NewSlackMessageBuilder()
	}

	return &incidentService{
		incidentRepo:      incidentRepo,
		timelineRepo:      timelineRepo,
		alertRepo:         alertRepo,
		chatService:       chatService,
		messageBuilder:    messageBuilder,
		db:                db,
		autoInviteUserIDs: autoInviteUserIDs,
	}
}

// SetAIService wires the optional AI service into the incident service.
// Called by routes.go after construction to avoid changing the constructor signature.
func SetAIService(svc IncidentService, ai AIService) {
	if is, ok := svc.(*incidentService); ok {
		is.aiService = ai
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

// CreateIncidentFromAlertWithGrouping creates an incident from an alert with grouping support (v0.3+)
//
// This method is called when a grouping rule matches and no existing incident is found.
// It uses PostgreSQL advisory locks to prevent race conditions when concurrent webhooks
// with the same group_key try to create incidents.
func (s *incidentService) CreateIncidentFromAlertWithGrouping(alert *models.Alert, groupKey string) (*models.Incident, error) {
	// Map alert severity to incident severity
	incidentSeverity := mapAlertSeverityToIncident(alert.Severity)

	// Generate slug from title
	slug := generateSlug(alert.Title)

	// Create incident object with group_key
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
		GroupKey:      &groupKey, // Set group_key for alert grouping
	}

	// Execute all operations in a transaction with advisory lock
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Step 0: Acquire advisory lock for this group_key to prevent race conditions
		// The lock is automatically released at the end of the transaction
		lockID := hashGroupKeyForLock(groupKey)
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", lockID).Error; err != nil {
			return fmt.Errorf("failed to acquire advisory lock: %w", err)
		}

		// Double-check: Another concurrent request might have created the incident while we waited for the lock
		var existingIncident models.Incident
		err := tx.Where("group_key = ?", groupKey).
			Where("status IN (?)", []string{"triggered", "acknowledged"}).
			First(&existingIncident).Error

		if err == nil {
			// Found existing incident - link alert to it instead of creating new
			slog.Info("incident already exists for group_key (race condition avoided)",
				"group_key", groupKey,
				"incident_id", existingIncident.ID)
			return s.linkAlertToIncidentInTx(tx, alert, &existingIncident)
		} else if err != gorm.ErrRecordNotFound {
			return fmt.Errorf("failed to check for existing incident: %w", err)
		}

		// No existing incident found - proceed with creation

		// Step 1: Create the incident
		if err := tx.Create(incident).Error; err != nil {
			return fmt.Errorf("failed to create incident: %w", err)
		}

		// Step 2: Link the alert to the incident (within transaction)
		link := map[string]interface{}{
			"incident_id":    incident.ID,
			"alert_id":       alert.ID,
			"linked_by_type": "system",
			"linked_by_id":   "alertmanager",
		}
		if err := tx.Table("incident_alerts").Create(link).Error; err != nil {
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
				"trigger":   "alert",
				"alert_id":  alert.ID.String(),
				"source":    alert.Source,
				"group_key": groupKey,
			},
		}

		// Create timeline entry within transaction
		if err := tx.Create(timelineEntry).Error; err != nil {
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

// LinkAlertToExistingIncident links an alert to an existing incident (for grouped alerts)
//
// This is called when the grouping engine finds an existing incident with the same group_key.
// It creates a timeline entry for the alert linkage and posts a Slack notification.
func (s *incidentService) LinkAlertToExistingIncident(alert *models.Alert, incidentID uuid.UUID) error {
	// Execute in transaction
	err := s.db.Transaction(func(tx *gorm.DB) error {
		return s.linkAlertToIncidentInTx(tx, alert, &models.Incident{ID: incidentID})
	})

	if err != nil {
		return fmt.Errorf("failed to link alert to incident: %w", err)
	}

	// Post Slack notification asynchronously (non-blocking)
	if s.chatService != nil {
		go func() {
			incident, err := s.incidentRepo.GetByID(incidentID)
			if err != nil {
				slog.Error("failed to get incident for slack notification",
					"incident_id", incidentID,
					"error", err)
				return
			}

			if incident.SlackChannelID == "" {
				// Channel not created yet (might be in progress)
				return
			}

			// Post alert notification to existing channel
			message := s.messageBuilder.BuildAlertLinkedMessage(alert, incident)
			_, err = s.chatService.PostMessage(incident.SlackChannelID, message)
			if err != nil {
				slog.Error("failed to post alert linked message to slack",
					"incident_id", incidentID,
					"alert_id", alert.ID,
					"error", err)
			}
		}()
	}

	return nil
}

// linkAlertToIncidentInTx links an alert to an incident within a transaction
func (s *incidentService) linkAlertToIncidentInTx(tx *gorm.DB, alert *models.Alert, incident *models.Incident) error {
	// Link the alert (within transaction)
	link := map[string]interface{}{
		"incident_id":    incident.ID,
		"alert_id":       alert.ID,
		"linked_by_type": "system",
		"linked_by_id":   "alertmanager",
	}
	if err := tx.Table("incident_alerts").Create(link).Error; err != nil {
		return fmt.Errorf("failed to link alert: %w", err)
	}

	// Create timeline entry (within transaction)
	timelineEntry := &models.TimelineEntry{
		ID:         uuid.New(),
		IncidentID: incident.ID,
		Timestamp:  time.Now(),
		Type:       "alert_linked",
		ActorType:  "system",
		ActorID:    "alertmanager",
		Content: models.JSONB{
			"alert_id": alert.ID.String(),
			"source":   alert.Source,
			"title":    alert.Title,
			"severity": string(alert.Severity),
		},
	}

	if err := tx.Create(timelineEntry).Error; err != nil {
		return fmt.Errorf("failed to create timeline entry: %w", err)
	}

	return nil
}

// hashGroupKeyForLock generates a numeric hash for PostgreSQL advisory locks
func hashGroupKeyForLock(groupKey string) int64 {
	// Simple hash function for advisory lock ID
	// Same as in grouping_engine.go
	var hash int64
	for i, c := range groupKey {
		hash = hash*31 + int64(c)
		if i >= 8 { // Use first 8 characters for hash
			break
		}
	}
	return hash
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
	slog.Info("posting initial message to slack channel",
		"incident_id", incident.ID,
		"channel_id", channel.ID,
		"has_blocks", len(message.Blocks) > 0,
		"block_count", len(message.Blocks),
		"text", message.Text)

	messageTS, err := s.chatService.PostMessage(channel.ID, message)
	if err != nil {
		slog.Error("failed to post initial message to slack channel",
			"incident_id", incident.ID,
			"channel_id", channel.ID,
			"error", err)
		// Continue - channel was created, message posting is non-critical
	} else {
		slog.Info("initial message posted to slack channel",
			"incident_id", incident.ID,
			"channel_id", channel.ID,
			"message_ts", messageTS)
		// Store message_ts so status changes can update the pinned card
		if storeErr := s.incidentRepo.UpdateSlackMessageTS(incident.ID, messageTS); storeErr != nil {
			slog.Warn("failed to store slack message ts", "incident_id", incident.ID, "error", storeErr)
		}
	}

	// Auto-invite users if configured
	if len(s.autoInviteUserIDs) > 0 {
		slog.Info("auto-inviting users to incident channel",
			"incident_id", incident.ID,
			"channel_id", channel.ID,
			"user_count", len(s.autoInviteUserIDs))

		err = s.chatService.InviteUsers(channel.ID, s.autoInviteUserIDs)
		if err != nil {
			slog.Error("failed to invite users",
				"incident_id", incident.ID,
				"error", err)

			// Timeline entry for failure
			s.createTimelineEntry(incident.ID, "user_invitation_failed", models.JSONB{
				"channel_id": channel.ID,
				"user_ids":   s.autoInviteUserIDs,
				"error":      err.Error(),
			})
			// Continue - non-fatal
		} else {
			slog.Info("users invited successfully",
				"incident_id", incident.ID,
				"user_count", len(s.autoInviteUserIDs))

			// Timeline entry for success
			s.createTimelineEntry(incident.ID, "users_invited", models.JSONB{
				"channel_id": channel.ID,
				"user_ids":   s.autoInviteUserIDs,
				"user_count": len(s.autoInviteUserIDs),
			})
		}
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

// generateSlug creates a URL-safe slug from a title with a random suffix for uniqueness.
// - Converts to lowercase
// - Replaces spaces and special characters with hyphens
// - Removes consecutive hyphens
// - Appends a 4-char random hex suffix
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

	// If slug is empty after sanitization, use a default
	if slug == "" {
		slug = "incident"
	}

	// Append random suffix to ensure uniqueness across same-title incidents
	suffix := randomHex(4)
	slug = slug + "-" + suffix

	// Truncate to 50 characters (after adding suffix)
	if len(slug) > 50 {
		slug = slug[:50]
		slug = strings.TrimRight(slug, "-")
	}

	return slug
}

// randomHex returns n random bytes encoded as a hex string (2n chars).
func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
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

// GetIncidentBySlackChannelID looks up an incident by its Slack channel ID.
// Returns nil, nil if no incident is associated with that channel.
func (s *incidentService) GetIncidentBySlackChannelID(channelID string) (*models.Incident, error) {
	return s.incidentRepo.GetBySlackChannelID(channelID)
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
			// Sync in-memory object so the subsequent Update() call doesn't overwrite
			incident.Status = params.Status
			now := time.Now()
			switch params.Status {
			case models.IncidentStatusAcknowledged:
				incident.AcknowledgedAt = &now
			case models.IncidentStatusResolved, models.IncidentStatusCanceled:
				incident.ResolvedAt = &now
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

	// Post Slack notification and optionally archive channel, asynchronously
	if params.Status != "" && params.Status != previousStatus && s.chatService != nil {
		go func() {
			if err := s.PostStatusUpdateToSlack(incident, previousStatus, params.Status); err != nil {
				slog.Error("failed to post slack notification", "error", err)
			}

			// Archive channel on terminal status (resolved or canceled)
			isTerminal := params.Status == models.IncidentStatusResolved ||
				params.Status == models.IncidentStatusCanceled
			if isTerminal && incident.SlackChannelID != "" {
				if err := s.chatService.ArchiveChannel(incident.SlackChannelID); err != nil {
					slog.Error("failed to archive slack channel",
						"incident_id", incident.ID,
						"channel_id", incident.SlackChannelID,
						"error", err)
					s.createTimelineEntry(incident.ID, "slack_channel_archive_failed", models.JSONB{
						"channel_id": incident.SlackChannelID,
						"error":      err.Error(),
					})
				} else {
					slog.Info("slack channel archived",
						"incident_id", incident.ID,
						"channel_id", incident.SlackChannelID)
					s.createTimelineEntry(incident.ID, "slack_channel_archived", models.JSONB{
						"channel_id": incident.SlackChannelID,
					})
				}
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
// For user-created notes, also posts the message to the incident's Slack channel
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

	// Post user notes to Slack channel for bidirectional sync
	if params.Type == "message" && s.chatService != nil {
		go s.postTimelineNoteToSlack(params.IncidentID, params.Content)
	}

	return entry, nil
}

// postTimelineNoteToSlack posts a user-created note to the incident's Slack channel.
// Runs asynchronously so it doesn't block the API response.
func (s *incidentService) postTimelineNoteToSlack(incidentID uuid.UUID, content models.JSONB) {
	incident, err := s.incidentRepo.GetByID(incidentID)
	if err != nil || incident.SlackChannelID == "" {
		return
	}

	// Extract message text from content
	messageText := ""
	if msg, ok := content["message"].(string); ok {
		messageText = msg
	} else if text, ok := content["text"].(string); ok {
		messageText = text
	}
	if messageText == "" {
		return
	}

	slackMessage := Message{
		Text: fmt.Sprintf(":memo: *Note from web UI:*\n%s", messageText),
	}

	if _, err := s.chatService.PostMessage(incident.SlackChannelID, slackMessage); err != nil {
		slog.Warn("failed to post timeline note to slack",
			"incident_id", incidentID,
			"error", err)
	}
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

	// Update the pinned incident card (if we have its message_ts)
	if incident.SlackMessageTS != "" {
		updatedCard := s.messageBuilder.BuildIncidentUpdatedMessage(incident)
		if err := s.chatService.UpdateMessage(incident.SlackChannelID, incident.SlackMessageTS, updatedCard); err != nil {
			slog.Warn("failed to update pinned slack message",
				"incident_id", incident.ID,
				"message_ts", incident.SlackMessageTS,
				"error", err)
		}
	}

	// Also post a new status-change notification for visibility
	notification := s.messageBuilder.BuildStatusUpdateMessage(incident, previousStatus, newStatus)
	_, err := s.chatService.PostMessage(incident.SlackChannelID, notification)
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

// ─── AI features (v0.6+) ──────────────────────────────────────────────────────

// GenerateAISummary generates an AI-powered summary for the incident, including
// timeline, alerts, and Slack thread context. Persists the result to the database.
func (s *incidentService) GenerateAISummary(incident *models.Incident) (string, error) {
	if !s.aiService.IsEnabled() {
		return "", fmt.Errorf("AI features are not configured: set OPENAI_API_KEY to enable")
	}

	// Gather context: timeline
	timeline, _, err := s.GetIncidentTimeline(incident.ID, repository.Pagination{Page: 1, PageSize: 100})
	if err != nil {
		slog.Warn("failed to fetch timeline for AI summary, continuing without it",
			"incident_id", incident.ID, "error", err)
		timeline = []models.TimelineEntry{}
	}

	// Gather context: alerts
	alerts, err := s.GetIncidentAlerts(incident.ID)
	if err != nil {
		slog.Warn("failed to fetch alerts for AI summary, continuing without them",
			"incident_id", incident.ID, "error", err)
		alerts = []models.Alert{}
	}

	// Gather context: Slack thread (best-effort, non-blocking)
	var slackMessages []string
	if s.chatService != nil && incident.SlackChannelID != "" && incident.SlackMessageTS != "" {
		msgs, err := s.chatService.GetThreadMessages(incident.SlackChannelID, incident.SlackMessageTS)
		if err != nil {
			slog.Warn("failed to fetch slack thread for AI summary, continuing without it",
				"incident_id", incident.ID, "error", err)
		} else {
			slackMessages = msgs
		}
	}

	summary, err := s.aiService.GenerateSummary(context.Background(), incident, timeline, alerts, slackMessages)
	if err != nil {
		return "", fmt.Errorf("generate AI summary: %w", err)
	}

	// Persist the summary
	generatedAt := time.Now()
	if err := s.incidentRepo.UpdateAISummary(incident.ID, summary, generatedAt); err != nil {
		slog.Error("failed to persist AI summary", "incident_id", incident.ID, "error", err)
		// Return summary anyway — don't fail the request just because persistence failed
	}

	return summary, nil
}

// GenerateHandoffDigest generates a shift handoff digest for the incident.
// The digest is not persisted — it is returned for display/posting.
func (s *incidentService) GenerateHandoffDigest(incident *models.Incident) (string, error) {
	if !s.aiService.IsEnabled() {
		return "", fmt.Errorf("AI features are not configured: set OPENAI_API_KEY to enable")
	}

	timeline, _, err := s.GetIncidentTimeline(incident.ID, repository.Pagination{Page: 1, PageSize: 100})
	if err != nil {
		slog.Warn("failed to fetch timeline for handoff digest, continuing without it",
			"incident_id", incident.ID, "error", err)
		timeline = []models.TimelineEntry{}
	}

	alerts, err := s.GetIncidentAlerts(incident.ID)
	if err != nil {
		slog.Warn("failed to fetch alerts for handoff digest, continuing without them",
			"incident_id", incident.ID, "error", err)
		alerts = []models.Alert{}
	}

	digest, err := s.aiService.GenerateHandoffDigest(context.Background(), incident, timeline, alerts)
	if err != nil {
		return "", fmt.Errorf("generate handoff digest: %w", err)
	}
	return digest, nil
}
