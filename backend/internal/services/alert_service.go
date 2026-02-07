package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/models/webhooks"
	"github.com/openincident/openincident/internal/repository"
)

// ProcessingResult holds statistics about webhook processing
type ProcessingResult struct {
	Received         int // Total alerts received in payload
	Created          int // New alerts created
	Updated          int // Existing alerts updated
	IncidentsCreated int // New incidents created from alerts
}

// AlertService defines the interface for alert processing operations
type AlertService interface {
	ProcessAlertmanagerPayload(payload *webhooks.AlertmanagerPayload) (*ProcessingResult, error)
}

// alertService implements AlertService
type alertService struct {
	alertRepo   repository.AlertRepository
	incidentSvc IncidentService
}

// NewAlertService creates a new alert service
func NewAlertService(alertRepo repository.AlertRepository, incidentSvc IncidentService) AlertService {
	return &alertService{
		alertRepo:   alertRepo,
		incidentSvc: incidentSvc,
	}
}

// ProcessAlertmanagerPayload processes all alerts from an Alertmanager webhook
func (s *alertService) ProcessAlertmanagerPayload(payload *webhooks.AlertmanagerPayload) (*ProcessingResult, error) {
	result := &ProcessingResult{
		Received: len(payload.Alerts),
	}

	for _, amAlert := range payload.Alerts {
		// Normalize the Alertmanager alert to our internal model
		alert := s.normalizeAlert(&amAlert)

		// Create or update the alert with deduplication
		created, err := s.createOrUpdateAlert(alert)
		if err != nil {
			return nil, fmt.Errorf("failed to process alert %s: %w", amAlert.Fingerprint, err)
		}

		if created {
			result.Created++

			// Check if this alert should trigger an incident
			if s.incidentSvc.ShouldCreateIncident(alert.Severity) {
				_, err := s.incidentSvc.CreateIncidentFromAlert(alert)
				if err != nil {
					return nil, fmt.Errorf("failed to create incident for alert %s: %w", alert.ID, err)
				}
				result.IncidentsCreated++
			}
		} else {
			result.Updated++
		}
	}

	return result, nil
}

// normalizeAlert converts an Alertmanager alert to the internal Alert model
func (s *alertService) normalizeAlert(amAlert *webhooks.AlertmanagerAlert) *models.Alert {
	// Extract title from alertname label
	title := amAlert.Labels["alertname"]
	if title == "" {
		title = "Unknown Alert"
	}

	// Extract description from annotations (prefer summary, fall back to description)
	description := amAlert.Annotations["summary"]
	if description == "" {
		description = amAlert.Annotations["description"]
	}

	// Parse severity from labels (default to warning if not specified or invalid)
	severity := parseSeverity(amAlert.Labels["severity"])

	// Determine status
	status := models.AlertStatusFiring
	if amAlert.Status == "resolved" {
		status = models.AlertStatusResolved
	}

	// Handle ended_at (only set if alert is resolved and has valid timestamp)
	var endedAt *time.Time
	if status == models.AlertStatusResolved && !amAlert.EndsAt.IsZero() {
		// Alertmanager sends 0001-01-01 for firing alerts
		if amAlert.EndsAt.Year() > 1900 {
			endedAt = &amAlert.EndsAt
		}
	}

	// Convert labels and annotations to JSONB
	labels := make(models.JSONB)
	for k, v := range amAlert.Labels {
		labels[k] = v
	}

	annotations := make(models.JSONB)
	for k, v := range amAlert.Annotations {
		annotations[k] = v
	}

	// Store raw payload for debugging and future processing
	rawPayload := make(models.JSONB)
	if bytes, err := json.Marshal(amAlert); err == nil {
		json.Unmarshal(bytes, &rawPayload)
	}

	return &models.Alert{
		ID:          uuid.New(),
		ExternalID:  amAlert.Fingerprint,
		Source:      "prometheus",
		Fingerprint: amAlert.Fingerprint,
		Status:      status,
		Severity:    severity,
		Title:       title,
		Description: description,
		Labels:      labels,
		Annotations: annotations,
		RawPayload:  rawPayload,
		StartedAt:   amAlert.StartsAt,
		EndedAt:     endedAt,
	}
}

// createOrUpdateAlert handles deduplication logic
// Returns true if alert was created, false if updated
func (s *alertService) createOrUpdateAlert(alert *models.Alert) (bool, error) {
	// Try to find existing alert by source and external_id
	existing, err := s.alertRepo.GetByExternalID(alert.Source, alert.ExternalID)

	if err != nil {
		// Check if it's a NotFoundError
		var notFoundErr *repository.NotFoundError
		if errors.As(err, &notFoundErr) {
			// Alert doesn't exist, create new one
			if err := s.alertRepo.Create(alert); err != nil {
				return false, fmt.Errorf("failed to create alert: %w", err)
			}
			return true, nil
		}
		// Other error occurred
		return false, fmt.Errorf("failed to check for existing alert: %w", err)
	}

	// Alert exists, update mutable fields
	existing.Status = alert.Status
	existing.Title = alert.Title
	existing.Description = alert.Description
	existing.EndedAt = alert.EndedAt

	if err := s.alertRepo.Update(existing); err != nil {
		return false, fmt.Errorf("failed to update alert: %w", err)
	}

	// Update the alert pointer to reference the existing alert (for incident creation)
	*alert = *existing

	return false, nil
}

// parseSeverity converts a string severity to AlertSeverity enum
// Returns warning as default for invalid or missing values
func parseSeverity(severity string) models.AlertSeverity {
	switch strings.ToLower(severity) {
	case "critical":
		return models.AlertSeverityCritical
	case "warning":
		return models.AlertSeverityWarning
	case "info":
		return models.AlertSeverityInfo
	default:
		return models.AlertSeverityWarning // Default to warning for safety
	}
}
