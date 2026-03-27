package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/fluidify/regen/internal/models"
	"github.com/fluidify/regen/internal/models/webhooks"
	"github.com/fluidify/regen/internal/repository"
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
	// ProcessAlertmanagerPayload processes Prometheus Alertmanager webhooks (legacy method for v0.1 compatibility)
	ProcessAlertmanagerPayload(payload *webhooks.AlertmanagerPayload) (*ProcessingResult, error)

	// ProcessNormalizedAlerts processes alerts from any source after normalization (v0.3+)
	ProcessNormalizedAlerts(alerts []webhooks.NormalizedAlert) (*ProcessingResult, error)

	// SetGroupingEngine sets the grouping engine for alert deduplication and grouping (v0.3+)
	SetGroupingEngine(engine GroupingEngine)

	// SetRoutingEngine sets the routing engine for alert routing decisions (v0.3+)
	SetRoutingEngine(engine RoutingEngine)

	// SetEscalationEngine sets the escalation engine for alert escalation (v0.5+)
	SetEscalationEngine(engine EscalationEngine)

	// SetEscalationRepos provides repos for the severity-rule and global-fallback
	// steps in the escalation policy resolution chain.
	SetEscalationRepos(escalationRepo repository.EscalationPolicyRepository, systemSettingsRepo repository.SystemSettingsRepository)
}

// alertService implements AlertService
type alertService struct {
	alertRepo          repository.AlertRepository
	incidentSvc        IncidentService
	groupingEngine     GroupingEngine             // Optional - can be nil if grouping disabled
	routingEngine      RoutingEngine              // Optional - can be nil if routing disabled
	escalationEngine   EscalationEngine           // Optional - can be nil if escalation disabled
	escalationRepo     repository.EscalationPolicyRepository  // For severity-rule fallback
	systemSettingsRepo repository.SystemSettingsRepository    // For global-fallback policy
}

// NewAlertService creates a new alert service
func NewAlertService(alertRepo repository.AlertRepository, incidentSvc IncidentService) AlertService {
	return &alertService{
		alertRepo:      alertRepo,
		incidentSvc:    incidentSvc,
		groupingEngine: nil, // Will be set via SetGroupingEngine if grouping enabled
	}
}

// SetGroupingEngine sets the grouping engine (for v0.3+ grouping support)
//
// This is called after AlertService construction because GroupingEngine needs IncidentRepository,
// which creates a circular dependency if passed to NewAlertService.
func (s *alertService) SetGroupingEngine(engine GroupingEngine) {
	s.groupingEngine = engine
}

// SetRoutingEngine sets the routing engine (for v0.3+ routing support)
func (s *alertService) SetRoutingEngine(engine RoutingEngine) {
	s.routingEngine = engine
}

// SetEscalationEngine sets the escalation engine (for v0.5+ escalation support)
func (s *alertService) SetEscalationEngine(engine EscalationEngine) {
	s.escalationEngine = engine
}

// SetEscalationRepos provides the repositories used for the severity-rule and
// global-fallback steps of the escalation policy resolution chain.
func (s *alertService) SetEscalationRepos(
	escalationRepo repository.EscalationPolicyRepository,
	systemSettingsRepo repository.SystemSettingsRepository,
) {
	s.escalationRepo = escalationRepo
	s.systemSettingsRepo = systemSettingsRepo
}

// ProcessAlertmanagerPayload processes all alerts from an Alertmanager webhook (v0.1 legacy method).
//
// This method maintains backwards compatibility with existing Prometheus webhook handlers.
// Internally, it delegates to ProcessNormalizedAlerts() after using PrometheusProvider to parse.
//
// The old flow was:
//   Handler → ProcessAlertmanagerPayload() → normalizeAlert() → createOrUpdateAlert()
//
// The new flow (v0.3+) is:
//   Handler → PrometheusProvider.ParsePayload() → ProcessNormalizedAlerts() → createOrUpdateAlert()
//
// This refactor uses the new flow while keeping the same public API for existing handlers.
func (s *alertService) ProcessAlertmanagerPayload(payload *webhooks.AlertmanagerPayload) (*ProcessingResult, error) {
	// Marshal payload back to JSON for PrometheusProvider
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal alertmanager payload: %w", err)
	}

	// Use PrometheusProvider to normalize the alerts
	provider := &webhooks.PrometheusProvider{}
	normalized, err := provider.ParsePayload(payloadBytes)
	if err != nil {
		return nil, err
	}

	// Delegate to the generic processing method
	return s.ProcessNormalizedAlerts(normalized)
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

// ProcessNormalizedAlerts processes alerts from any monitoring source (v0.3+).
//
// This is the generic alert processing pipeline that works with alerts from Prometheus,
// Grafana, CloudWatch, Generic webhooks, or any future source.
//
// Pipeline (v0.3+ with grouping):
//  1. Convert NormalizedAlert → models.Alert
//  2. Deduplicate: Check (source, external_id) - create new or update existing
//  3. If new alert and should create incident:
//     a. Evaluate grouping rules (if grouping engine configured)
//     b. Based on grouping decision:
//        - Link to existing incident (if group match found)
//        - Create new incident with group_key (if rule matched)
//        - Create new incident without group_key (default behavior)
//
// The existing ProcessAlertmanagerPayload() delegates to this method after Prometheus-specific
// parsing, ensuring backwards compatibility while enabling multi-source support.
func (s *alertService) ProcessNormalizedAlerts(alerts []webhooks.NormalizedAlert) (*ProcessingResult, error) {
	result := &ProcessingResult{
		Received: len(alerts),
	}

	for _, normalized := range alerts {
		// Convert NormalizedAlert to internal Alert model
		alert := s.normalizedAlertToModel(&normalized)

		// Create or update the alert with deduplication
		created, err := s.createOrUpdateAlert(alert)
		if err != nil {
			return nil, fmt.Errorf("failed to process alert %s from source %s: %w",
				normalized.ExternalID, normalized.Source, err)
		}

		if created {
			result.Created++

			// v0.3+: Evaluate routing rules to determine incident behavior
			routingDecision := &RoutingDecision{AIEnabled: true}
			if s.routingEngine != nil {
				rd, err := s.routingEngine.EvaluateAlert(alert)
				if err != nil {
					return nil, fmt.Errorf("failed to evaluate routing for alert %s: %w", alert.ID, err)
				}
				routingDecision = rd
			}

			// If routing says suppress, skip incident creation entirely.
			// Escalation must also not trigger for suppressed alerts.
			if routingDecision.Suppress {
				continue
			}

			// v0.5+: Resolve escalation policy via fallback chain, then trigger.
			// Chain: routing rule → severity rule → global fallback.
			if s.escalationEngine != nil {
				policyID := routingDecision.EscalationPolicyID
				if policyID == nil {
					policyID = s.resolveEscalationPolicy(alert)
				}
				if policyID != nil {
					alert.EscalationPolicyID = policyID
					if err := s.alertRepo.Update(alert); err != nil {
						slog.Error("failed to persist escalation_policy_id on alert", "alert_id", alert.ID, "err", err)
					}
					if err := s.escalationEngine.TriggerEscalation(alert); err != nil {
						// Log and continue — escalation failure must not block incident creation.
						slog.Error("failed to trigger escalation for alert", "alert_id", alert.ID, "err", err)
					}
				}
			}

			// Apply severity override before incident creation.
			// NOTE: The alert is already persisted with the original monitoring-tool
			// severity (intentional — preserves the raw audit record). The override
			// only affects which severity the resulting incident is created at.
			if routingDecision.SeverityOverride != "" {
				alert.Severity = parseSeverity(routingDecision.SeverityOverride)
			}

			// Check if this alert should trigger an incident (uses possibly-overridden severity)
			if s.incidentSvc.ShouldCreateIncident(alert.Severity) {
				// v0.3+: Use grouping engine if configured
				if s.groupingEngine != nil {
					decision, err := s.groupingEngine.EvaluateAlert(alert)
					if err != nil {
						return nil, fmt.Errorf("failed to evaluate grouping for alert %s: %w", alert.ID, err)
					}

					switch decision.Action {
					case GroupActionLinkToExisting:
						// Link alert to existing incident
						if err := s.incidentSvc.LinkAlertToExistingIncident(alert, *decision.IncidentID); err != nil {
							return nil, fmt.Errorf("failed to link alert to incident: %w", err)
						}
						// Note: Incident was already created, don't increment IncidentsCreated

					case GroupActionCreateNew:
						// Create new incident with group_key
						_, err := s.incidentSvc.CreateIncidentFromAlertWithGrouping(alert, decision.GroupKey, routingDecision.AIEnabled)
						if err != nil {
							return nil, fmt.Errorf("failed to create incident with grouping: %w", err)
						}
						result.IncidentsCreated++

					case GroupActionDefault:
						// No grouping rule matched — use default behavior
						_, err := s.incidentSvc.CreateIncidentFromAlert(alert, routingDecision.AIEnabled)
						if err != nil {
							return nil, fmt.Errorf("failed to create incident: %w", err)
						}
						result.IncidentsCreated++
					}
				} else {
					// Grouping disabled - use v0.2 behavior (create without group_key)
					_, err := s.incidentSvc.CreateIncidentFromAlert(alert, routingDecision.AIEnabled)
					if err != nil {
						return nil, fmt.Errorf("failed to create incident for alert %s: %w", alert.ID, err)
					}
					result.IncidentsCreated++
				}
			}
		} else {
			result.Updated++
		}
	}

	return result, nil
}

// normalizedAlertToModel converts a NormalizedAlert (from any source) to the internal Alert model.
//
// This conversion is source-agnostic - all field mapping was done by the WebhookProvider.
// We simply copy fields from the normalized representation into the database model.
func (s *alertService) normalizedAlertToModel(normalized *webhooks.NormalizedAlert) *models.Alert {
	// Parse severity enum
	severity := parseSeverity(normalized.Severity)

	// Parse status
	status := models.AlertStatusFiring
	if normalized.Status == "resolved" {
		status = models.AlertStatusResolved
	}

	// Convert string maps to JSONB
	labels := make(models.JSONB)
	for k, v := range normalized.Labels {
		labels[k] = v
	}

	annotations := make(models.JSONB)
	for k, v := range normalized.Annotations {
		annotations[k] = v
	}

	// Convert RawPayload json.RawMessage to JSONB
	rawPayload := make(models.JSONB)
	if len(normalized.RawPayload) > 0 {
		if err := json.Unmarshal(normalized.RawPayload, &rawPayload); err != nil { slog.Warn("failed to unmarshal raw payload", "error", err) }
	}

	return &models.Alert{
		ID:          uuid.New(),
		ExternalID:  normalized.ExternalID,
		Source:      normalized.Source,
		Fingerprint: normalized.ExternalID, // Use external_id as fingerprint (for Prometheus, this IS the fingerprint)
		Status:      status,
		Severity:    severity,
		Title:       normalized.Title,
		Description: normalized.Description,
		Labels:      labels,
		Annotations: annotations,
		RawPayload:  rawPayload,
		StartedAt:   normalized.StartedAt,
		EndedAt:     normalized.EndedAt,
	}
}

// resolveEscalationPolicy walks the fallback chain to find an escalation policy
// for the given alert when the routing rule provides no explicit policy ID.
//
// Fallback order:
//  1. Severity rule (escalation_severity_rules table)
//  2. Global fallback (system_settings "escalation.global_fallback_policy_id")
func (s *alertService) resolveEscalationPolicy(alert *models.Alert) *uuid.UUID {
	// 1. Severity rule
	if s.escalationRepo != nil {
		if rule, err := s.escalationRepo.GetSeverityRule(string(alert.Severity)); err == nil && rule != nil {
			return &rule.EscalationPolicyID
		}
	}
	// 2. Global fallback
	if s.systemSettingsRepo != nil {
		if id, err := s.systemSettingsRepo.GetGlobalFallbackPolicyID(); err == nil && id != nil {
			return id
		}
	}
	return nil
}
