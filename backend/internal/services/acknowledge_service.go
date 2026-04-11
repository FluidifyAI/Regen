package services

import (
	"errors"
	"log/slog"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/google/uuid"
)

// AcknowledgeAlertWithTimeline acknowledges an alert via the escalation engine and,
// if the alert is linked to an incident, appends a timeline entry recording who
// acknowledged it, when, and via which channel.
//
// This is the canonical acknowledgment call used by the REST handler and Slack
// button callback — both paths converge here so timeline entries are always written.
func AcknowledgeAlertWithTimeline(
	alertID uuid.UUID,
	userName string,
	via models.AcknowledgmentVia,
	engine EscalationEngine,
	incidentRepo repository.IncidentRepository,
	timelineRepo repository.TimelineRepository,
) error {
	if err := engine.AcknowledgeAlert(alertID, userName, via); err != nil {
		return err
	}

	// Timeline integration requires both repos — skip if not configured.
	if incidentRepo == nil || timelineRepo == nil {
		return nil
	}

	// Best-effort timeline entry — if alert is not linked to an incident, skip silently.
	incident, err := incidentRepo.GetIncidentByAlertID(alertID)
	if err != nil {
		var nfe *repository.NotFoundError
		if errors.As(err, &nfe) {
			return nil // alert not linked to an incident — that's fine
		}
		slog.Warn("failed to look up incident for acknowledgment timeline entry",
			"alert_id", alertID,
			"error", err,
		)
		return nil // non-fatal
	}

	entry := &models.TimelineEntry{
		ID:         uuid.New(),
		IncidentID: incident.ID,
		Timestamp:  time.Now().UTC(),
		Type:       "alert_acknowledged",
		ActorType:  "user",
		ActorID:    userName,
		Content: models.JSONB{
			"alert_id": alertID.String(),
			"via":      string(via),
		},
	}

	if err := timelineRepo.Create(entry); err != nil {
		slog.Warn("failed to create acknowledgment timeline entry",
			"alert_id", alertID,
			"incident_id", incident.ID,
			"error", err,
		)
		// Non-fatal: acknowledgment already recorded in escalation state
	}

	return nil
}
