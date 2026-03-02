package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/repository"
	appredis "github.com/openincident/openincident/internal/redis"
	"gorm.io/gorm"
)

// CreateIncidentParams holds parameters for creating a manual incident
type CreateIncidentParams struct {
	Title       string
	Severity    models.IncidentSeverity
	Description string
	CreatedBy   string // "user", "system", "api"
	AIEnabled   bool   // Controls whether AI agents process this incident. Defaults to true.
}

// UpdateIncidentParams holds parameters for updating an incident
type UpdateIncidentParams struct {
	Status      models.IncidentStatus
	Severity    models.IncidentSeverity
	Summary     string
	UpdatedBy   string
	ClientIP    string     // For audit logging
	AIEnabled   *bool      // Controls whether AI agents process this incident. nil = no change.
	CommanderID *uuid.UUID // nil = no change
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
	CreateIncidentFromAlert(alert *models.Alert, aiEnabled bool) (*models.Incident, error)
	CreateIncidentFromAlertWithGrouping(alert *models.Alert, groupKey string, aiEnabled bool) (*models.Incident, error)
	LinkAlertToExistingIncident(alert *models.Alert, incidentID uuid.UUID) error
	ShouldCreateIncident(severity models.AlertSeverity) bool
	CreateSlackChannelForIncident(incident *models.Incident, alerts []models.Alert) error

	// API operations
	ListIncidents(filters repository.IncidentFilters, pagination repository.Pagination) ([]models.Incident, int64, error)
	GetIncident(id uuid.UUID, number int) (*models.Incident, error)
	GetIncidentBySlackChannelID(channelID string) (*models.Incident, error)
	CreateIncident(params *CreateIncidentParams) (*models.Incident, error)
	UpdateIncident(id uuid.UUID, params *UpdateIncidentParams) (*models.Incident, error)
	AcknowledgeIncident(id uuid.UUID, actorType, actorID string) error
	ResolveIncident(id uuid.UUID, actorType, actorID string) error
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
	teamsSvc          *TeamsService        // Optional - can be nil if Teams not configured (v0.8+)
	db                *gorm.DB             // For transaction management
	autoInviteUserIDs []string             // User IDs to auto-invite to incident channels
	aiService         AIService            // Optional - can be nil if OpenAI not configured
	userRepo          repository.UserRepository     // Optional — for commander name resolution
	scheduleRepo      repository.ScheduleRepository // Optional — for on-call auto-assign
	evaluator         ScheduleEvaluator             // Optional — for on-call auto-assign
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

// SetTeamsService wires the optional Teams service into the incident service (v0.8+).
// Called by routes.go after construction when TEAMS_APP_ID is configured.
func SetTeamsService(svc IncidentService, teams *TeamsService) {
	if is, ok := svc.(*incidentService); ok {
		is.teamsSvc = teams
	}
}

// SetCommanderDeps wires the optional user/schedule dependencies for commander auto-assignment.
// Called by routes.go after construction. Safe to skip in tests that don't test this feature.
func SetCommanderDeps(svc IncidentService, userRepo repository.UserRepository, scheduleRepo repository.ScheduleRepository, evaluator ScheduleEvaluator) {
	if s, ok := svc.(*incidentService); ok {
		s.userRepo = userRepo
		s.scheduleRepo = scheduleRepo
		s.evaluator = evaluator
	}
}

// resolveCommanderName looks up a user's display name by UUID.
// Returns "" if userRepo is not wired or the user is not found.
func (s *incidentService) resolveCommanderName(id *uuid.UUID) string {
	if id == nil || s.userRepo == nil {
		return ""
	}
	user, err := s.userRepo.GetByID(*id)
	if err != nil {
		return ""
	}
	if user.Name != "" {
		return user.Name
	}
	return user.Email
}

// findOnCallUserID returns the UUID of the first on-call user across all schedules.
// Returns nil if no schedule is configured, no one is on-call, or the user cannot be resolved.
func (s *incidentService) findOnCallUserID() *uuid.UUID {
	if s.scheduleRepo == nil || s.evaluator == nil || s.userRepo == nil {
		return nil
	}
	schedules, err := s.scheduleRepo.GetAll()
	if err != nil || len(schedules) == 0 {
		return nil
	}
	now := time.Now()
	for _, sch := range schedules {
		username, err := s.evaluator.WhoIsOnCall(sch.ID, now)
		if err != nil || username == "" {
			continue
		}
		// Try email match first (most reliable — new UI stores email as user_name)
		if user, err := s.userRepo.GetByEmail(username); err == nil {
			return &user.ID
		}
		// Fall back: scan all users for name or email-prefix match (legacy free-form entries)
		if users, err := s.userRepo.ListAll(); err == nil {
			for _, u := range users {
				if strings.EqualFold(u.Name, username) {
					id := u.ID
					return &id
				}
				// email prefix match: "alice" matches "alice@company.com"
				if idx := strings.Index(u.Email, "@"); idx > 0 {
					if strings.EqualFold(u.Email[:idx], username) {
						id := u.ID
						return &id
					}
				}
			}
		}
	}
	return nil
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
func (s *incidentService) CreateIncidentFromAlert(alert *models.Alert, aiEnabled bool) (*models.Incident, error) {
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
		AIEnabled:     aiEnabled,
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

	// Reload to pick up DB-assigned fields (e.g. incident_number) and get a fresh pointer
	// that is not shared with the caller, avoiding data races in the channel-creation goroutine.
	reloadedIncident, err := s.incidentRepo.GetByID(incident.ID)
	if err != nil {
		slog.Error("failed to reload incident after creation; proceeding with partial struct",
			"incident_id", incident.ID, "error", err)
		s.launchChannelCreation(incident, []models.Alert{*alert})
		return incident, nil
	}

	// Auto-assign on-call engineer as commander (best-effort)
	if reloadedIncident.CommanderID == nil {
		if onCallID := s.findOnCallUserID(); onCallID != nil {
			reloadedIncident.CommanderID = onCallID
			if err := s.incidentRepo.Update(reloadedIncident); err != nil {
				slog.Warn("failed to persist auto-assigned commander", "incident_id", reloadedIncident.ID, "error", err)
			}
		}
	}
	reloadedIncident.CommanderName = s.resolveCommanderName(reloadedIncident.CommanderID)

	// Create Slack + Teams channels asynchronously (non-blocking)
	s.launchChannelCreation(reloadedIncident, []models.Alert{*alert})

	return reloadedIncident, nil
}

// CreateIncidentFromAlertWithGrouping creates an incident from an alert with grouping support (v0.3+)
//
// This method is called when a grouping rule matches and no existing incident is found.
// It uses PostgreSQL advisory locks to prevent race conditions when concurrent webhooks
// with the same group_key try to create incidents.
func (s *incidentService) CreateIncidentFromAlertWithGrouping(alert *models.Alert, groupKey string, aiEnabled bool) (*models.Incident, error) {
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
		AIEnabled:     aiEnabled,
	}

	// Track whether a new incident was created vs. an existing one reused (race-condition path).
	// Channel creation must only run for genuinely new incidents.
	var newIncidentCreated bool

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
		newIncidentCreated = true

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

	if newIncidentCreated {
		// Reload to get DB-assigned fields and a fresh pointer for the goroutine.
		reloadedIncident, err := s.incidentRepo.GetByID(incident.ID)
		if err != nil {
			slog.Error("failed to reload incident after creation; proceeding with partial struct",
				"incident_id", incident.ID, "error", err)
			s.launchChannelCreation(incident, []models.Alert{*alert})
			return incident, nil
		}

		// Auto-assign on-call engineer as commander (best-effort)
		if reloadedIncident.CommanderID == nil {
			if onCallID := s.findOnCallUserID(); onCallID != nil {
				reloadedIncident.CommanderID = onCallID
				if err := s.incidentRepo.Update(reloadedIncident); err != nil {
					slog.Warn("failed to persist auto-assigned commander", "incident_id", reloadedIncident.ID, "error", err)
				}
			}
		}
		reloadedIncident.CommanderName = s.resolveCommanderName(reloadedIncident.CommanderID)

		s.launchChannelCreation(reloadedIncident, []models.Alert{*alert})
		return reloadedIncident, nil
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
	incidents, total, err := s.incidentRepo.List(filters, pagination)
	if err != nil {
		return nil, 0, err
	}
	// Resolve commander names in a single batch (collect unique IDs first)
	commanderNames := make(map[uuid.UUID]string)
	for _, inc := range incidents {
		if inc.CommanderID != nil {
			commanderNames[*inc.CommanderID] = "" // seed the map
		}
	}
	if s.userRepo != nil {
		for id := range commanderNames {
			user, err := s.userRepo.GetByID(id)
			if err == nil {
				if user.Name != "" {
					commanderNames[id] = user.Name
				} else {
					commanderNames[id] = user.Email
				}
			}
		}
	}
	for i := range incidents {
		if incidents[i].CommanderID != nil {
			incidents[i].CommanderName = commanderNames[*incidents[i].CommanderID]
		}
	}
	return incidents, total, nil
}

// GetIncident retrieves an incident by UUID or incident number
func (s *incidentService) GetIncident(id uuid.UUID, number int) (*models.Incident, error) {
	var (
		incident *models.Incident
		err      error
	)
	if id != uuid.Nil {
		incident, err = s.incidentRepo.GetByID(id)
	} else {
		incident, err = s.incidentRepo.GetByNumber(number)
	}
	if err != nil {
		return nil, err
	}
	incident.CommanderName = s.resolveCommanderName(incident.CommanderID)
	return incident, nil
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
		AIEnabled:     params.AIEnabled,
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

	// Auto-assign on-call engineer as commander (best-effort)
	if reloadedIncident.CommanderID == nil {
		if onCallID := s.findOnCallUserID(); onCallID != nil {
			reloadedIncident.CommanderID = onCallID
			if err := s.incidentRepo.Update(reloadedIncident); err != nil {
				slog.Warn("failed to persist auto-assigned commander", "incident_id", reloadedIncident.ID, "error", err)
			}
		}
	}
	reloadedIncident.CommanderName = s.resolveCommanderName(reloadedIncident.CommanderID)

	// Create Slack + Teams channels asynchronously
	s.launchChannelCreation(reloadedIncident, []models.Alert{})

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

		// Update ai_enabled if explicitly provided
		if params.AIEnabled != nil {
			incident.AIEnabled = *params.AIEnabled
		}

		// Update commander if explicitly provided
		if params.CommanderID != nil {
			if *params.CommanderID == uuid.Nil {
				// zero UUID = explicit clear
				incident.CommanderID = nil
			} else if s.userRepo != nil {
				if _, err := s.userRepo.GetByID(*params.CommanderID); err != nil {
					return fmt.Errorf("commander user not found: %w", err)
				}
				incident.CommanderID = params.CommanderID
			} else {
				incident.CommanderID = params.CommanderID
			}
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

	statusChanged := params.Status != "" && params.Status != previousStatus

	// Post Slack notification and optionally archive channel, asynchronously
	if statusChanged && s.chatService != nil {
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

	// Post Teams notification and optionally archive Teams channel, asynchronously (v0.8+)
	if statusChanged && s.teamsSvc != nil {
		go func() {
			defer recoverAsyncPanic("postStatusUpdateToTeams", "incident_id", incident.ID)
			s.postStatusUpdateToTeams(incident, previousStatus, params.Status, params.UpdatedBy)
		}()
	}

	// Publish event for AICoordinator (Post-Mortem Agent et al.)
	if statusChanged && (params.Status == models.IncidentStatusResolved || params.Status == models.IncidentStatusCanceled) {
		go publishResolved(incident.ID, incident.AIEnabled)
	}

	// Fetch updated incident
	updatedIncident, err := s.incidentRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	updatedIncident.CommanderName = s.resolveCommanderName(updatedIncident.CommanderID)
	return updatedIncident, nil
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

	// Post user notes to chat channels for bidirectional sync
	if params.Type == "message" {
		if s.chatService != nil {
			go func() {
				defer recoverAsyncPanic("postTimelineNoteToSlack", "incident_id", params.IncidentID)
				s.postTimelineNoteToSlack(params.IncidentID, params.Content)
			}()
		}
		if s.teamsSvc != nil {
			go func() {
				defer recoverAsyncPanic("postTimelineNoteToTeams", "incident_id", params.IncidentID)
				s.postTimelineNoteToTeams(params.IncidentID, params.Content)
			}()
		}
	}

	return entry, nil
}

// postTimelineNoteToSlack posts a user-created note to the incident's Slack channel.
// Runs asynchronously so it doesn't block the API response.
func (s *incidentService) postTimelineNoteToSlack(incidentID uuid.UUID, content models.JSONB) {
	incident, err := s.incidentRepo.GetByID(incidentID)
	if err != nil {
		slog.Error("postTimelineNoteToSlack: failed to load incident",
			"incident_id", incidentID, "error", err)
		return
	}
	if incident.SlackChannelID == "" {
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

// postTimelineNoteToTeams posts a user-created note to the incident's Teams channel.
// Mirrors postTimelineNoteToSlack for bidirectional Teams parity (v0.9+).
// Runs asynchronously so it doesn't block the API response.
func (s *incidentService) postTimelineNoteToTeams(incidentID uuid.UUID, content models.JSONB) {
	incident, err := s.incidentRepo.GetByID(incidentID)
	if err != nil {
		slog.Error("postTimelineNoteToTeams: failed to load incident",
			"incident_id", incidentID, "error", err)
		return
	}
	if incident.TeamsConversationID == nil {
		return
	}

	messageText := ""
	if msg, ok := content["message"].(string); ok {
		messageText = msg
	} else if text, ok := content["text"].(string); ok {
		messageText = text
	}
	if messageText == "" {
		return
	}

	teamsMessage := Message{
		Text: fmt.Sprintf("📝 **Note from web UI:**\n%s", messageText),
	}
	if _, err := s.teamsSvc.PostToConversation(*incident.TeamsConversationID, teamsMessage); err != nil {
		slog.Warn("failed to post timeline note to teams",
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

// ─── Bot command helpers (v0.8+) ──────────────────────────────────────────────

// AcknowledgeIncident transitions an incident to "acknowledged".
// Used by the Teams and Slack bots so they can acknowledge via simple commands.
func (s *incidentService) AcknowledgeIncident(id uuid.UUID, actorType, actorID string) error {
	incident, err := s.incidentRepo.GetByID(id)
	if err != nil {
		return err
	}
	if err := s.incidentRepo.UpdateStatus(id, models.IncidentStatusAcknowledged); err != nil {
		return err
	}
	s.createTimelineEntry(id, models.TimelineTypeStatusChanged, models.JSONB{
		"previous_status": string(incident.Status),
		"new_status":      string(models.IncidentStatusAcknowledged),
		"actor":           actorID,
	})
	return nil
}

// ResolveIncident transitions an incident to "resolved".
// Used by the Teams and Slack bots so they can resolve via simple commands.
func (s *incidentService) ResolveIncident(id uuid.UUID, actorType, actorID string) error {
	incident, err := s.incidentRepo.GetByID(id)
	if err != nil {
		return err
	}
	if err := s.incidentRepo.UpdateStatus(id, models.IncidentStatusResolved); err != nil {
		return err
	}
	s.createTimelineEntry(id, models.TimelineTypeStatusChanged, models.JSONB{
		"previous_status": string(incident.Status),
		"new_status":      string(models.IncidentStatusResolved),
		"actor":           actorID,
	})
	// Publish event for AICoordinator (Post-Mortem Agent et al.)
	go publishResolved(id, incident.AIEnabled)
	// Re-fetch with updated status so postStatusUpdateToTeams sees the correct state.
	// This also ensures the card update shows "resolved" rather than the previous status.
	if updatedIncident, err := s.incidentRepo.GetByID(id); err == nil && s.teamsSvc != nil {
		go func() {
			defer recoverAsyncPanic("postStatusUpdateToTeams(resolve)", "incident_id", id)
			s.postStatusUpdateToTeams(updatedIncident, incident.Status, models.IncidentStatusResolved, actorID)
		}()
	}
	return nil
}

// ─── Teams channel management (v0.8+) ────────────────────────────────────────

// createTeamsChannelForIncident creates a Teams channel for an incident, posts the initial
// Adaptive Card, and stores the channel ID in the database. Runs asynchronously.
func (s *incidentService) createTeamsChannelForIncident(incident *models.Incident, alerts []models.Alert) error {
	channelName := formatIncidentChannelName(incident.IncidentNumber, incident.Slug)
	// Teams channel display names are capped at 50 characters by the Graph API.
	if len(channelName) > 50 {
		channelName = channelName[:50]
	}
	description := fmt.Sprintf("Incident #%d: %s", incident.IncidentNumber, incident.Title)

	slog.Info("creating teams channel for incident",
		"incident_id", incident.ID,
		"incident_number", incident.IncidentNumber,
		"channel_name", channelName)

	channel, err := s.teamsSvc.CreateChannel(channelName, description)
	if err != nil {
		s.createTimelineEntry(incident.ID, "teams_channel_creation_failed", models.JSONB{
			"error":        err.Error(),
			"channel_name": channelName,
		})
		return fmt.Errorf("failed to create teams channel: %w", err)
	}

	// Link the channel to the incident in the database. If this fails the channel is
	// orphaned in Teams — bot commands will never be able to find the incident — so
	// we must abort here rather than continue. The timeline entry documents the orphaned ID
	// so an operator can manually investigate.
	if err := s.incidentRepo.UpdateTeamsChannel(incident.ID, channel.ID, channel.Name); err != nil {
		slog.Error("failed to update incident with teams channel — channel is orphaned",
			"incident_id", incident.ID,
			"teams_channel_id", channel.ID,
			"error", err)
		s.createTimelineEntry(incident.ID, "teams_channel_orphaned", models.JSONB{
			"teams_channel_id":   channel.ID,
			"teams_channel_name": channel.Name,
			"error":              err.Error(),
		})
		return fmt.Errorf("teams channel created but could not be linked to incident: %w", err)
	}

	// Post initial Adaptive Card via Bot Framework Proactive Messaging.
	// PostToChannel creates a Bot Framework conversation in the channel and returns
	// both the conversationID (needed for future PostToConversation calls) and the
	// activityID (needed to update the root card on status changes).
	// Both IDs are stored atomically to prevent a partial-write state.
	card := teamsIncidentCard(incident)
	msg := Message{Blocks: []interface{}{card}}
	conversationID, activityID, err := s.teamsSvc.PostToChannel(channel.ID, msg)
	if err != nil {
		slog.Error("failed to post initial teams card",
			"incident_id", incident.ID, "channel_id", channel.ID, "error", err)
	} else if conversationID != "" && activityID != "" {
		if storeErr := s.incidentRepo.UpdateTeamsPostingIDs(incident.ID, conversationID, activityID); storeErr != nil {
			slog.Warn("failed to store teams posting ids", "incident_id", incident.ID, "error", storeErr)
		}
	}

	s.createTimelineEntry(incident.ID, "teams_channel_created", models.JSONB{
		"channel_id":   channel.ID,
		"channel_name": channel.Name,
		"channel_url":  channel.URL,
	})

	slog.Info("teams channel created for incident",
		"incident_id", incident.ID,
		"incident_number", incident.IncidentNumber,
		"channel_id", channel.ID,
		"channel_name", channel.Name)

	return nil
}

// recoverAsyncPanic logs a recovered panic from an async goroutine.
// Gin's recovery middleware only covers the HTTP handler goroutine; any goroutine
// spawned with `go` must recover its own panics to prevent crashing the server.
func recoverAsyncPanic(op string, extra ...any) {
	if r := recover(); r != nil {
		args := append([]any{"panic", fmt.Sprintf("%v", r)}, extra...)
		slog.Error("panic in async goroutine — recovered", append([]any{"op", op}, args...)...)
	}
}

// publishResolved publishes an "incident.resolved" event to Redis so the
// AICoordinator can trigger the Post-Mortem Agent. Best-effort: log and continue on error.
func publishResolved(incidentID uuid.UUID, aiEnabled bool) {
	payload, err := json.Marshal(map[string]interface{}{
		"incident_id": incidentID.String(),
		"ai_enabled":  aiEnabled,
	})
	if err != nil {
		slog.Error("publishResolved: marshal failed", "error", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := appredis.Client.Publish(ctx, "events:incident.resolved", payload).Err(); err != nil {
		slog.Warn("publishResolved: redis publish failed (agent will not run)", "error", err)
	}
}

// launchChannelCreation spawns goroutines to create Slack and/or Teams channels for a new incident.
// Extracted to avoid repeating the same two-goroutine block in every CreateIncident* variant.
func (s *incidentService) launchChannelCreation(incident *models.Incident, alerts []models.Alert) {
	if s.chatService != nil {
		go func() {
			defer recoverAsyncPanic("CreateSlackChannelForIncident", "incident_id", incident.ID)
			if err := s.CreateSlackChannelForIncident(incident, alerts); err != nil {
				slog.Error("failed to create slack channel",
					"incident_id", incident.ID,
					"incident_number", incident.IncidentNumber,
					"error", err)
			}
		}()
	}
	if s.teamsSvc != nil {
		go func() {
			defer recoverAsyncPanic("createTeamsChannelForIncident", "incident_id", incident.ID)
			if err := s.createTeamsChannelForIncident(incident, alerts); err != nil {
				slog.Error("failed to create teams channel",
					"incident_id", incident.ID,
					"incident_number", incident.IncidentNumber,
					"error", err)
			}
		}()
	}
}

// postStatusUpdateToTeams posts a status change notification to the Teams channel
// and updates the root Adaptive Card. Runs asynchronously.
// Uses Bot Framework Proactive Messaging via the stored conversationID (v0.9+):
//   - UpdateConversationMessage refreshes the root incident card in-place
//   - PostToConversation posts a new status notification card
func (s *incidentService) postStatusUpdateToTeams(incident *models.Incident, previousStatus, newStatus models.IncidentStatus, updatedBy string) {
	if incident.TeamsConversationID == nil {
		// Pre-v0.9 incident without a stored conversationID — skip silently.
		// Channel was created but Bot Framework posting was not yet implemented.
		return
	}
	conversationID := *incident.TeamsConversationID

	// Update the root card in-place if we have its activity ID
	if incident.TeamsActivityID != nil {
		updatedCard := teamsIncidentCard(incident)
		msg := Message{Blocks: []interface{}{updatedCard}}
		if err := s.teamsSvc.UpdateConversationMessage(conversationID, *incident.TeamsActivityID, msg); err != nil {
			slog.Warn("teams: failed to update root card on status change",
				"incident_id", incident.ID, "error", err)
		}
	}

	// Post a visible notification to the conversation
	statusCard := teamsStatusUpdateCard(incident, updatedBy)
	notifMsg := Message{Blocks: []interface{}{statusCard}}
	if _, err := s.teamsSvc.PostToConversation(conversationID, notifMsg); err != nil {
		slog.Error("teams: failed to post status update notification",
			"incident_id", incident.ID, "error", err)
		return
	}

	// Archive on terminal status (best-effort rename via Graph API — known limitation)
	isTerminal := newStatus == models.IncidentStatusResolved || newStatus == models.IncidentStatusCanceled
	if isTerminal && incident.TeamsChannelID != nil {
		s.teamsSvc.ArchiveChannel(*incident.TeamsChannelID)
	}
}
