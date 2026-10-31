package services

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log/slog"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/models/webhooks"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ProcessNormalizedAlerts commits each alert, its links and timeline atomically.
func (s *alertService) ProcessNormalizedAlerts(ctx context.Context, alerts []webhooks.NormalizedAlert) (*ProcessingResult, error) {
	original, ok := s.incidentSvc.(*incidentService)
	if !ok {
		return s.processNormalizedAlerts(ctx, alerts)
	}
	result := &ProcessingResult{Received: len(alerts)}
	for _, alert := range alerts {
		var effects []func()
		var partial *ProcessingResult
		var boundIncident *incidentService
		err := original.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			// The stable alert identity protects first inserts as well as retries.
			// Independent alerts never share a lifecycle-wide lock.
			if tx.Name() == "postgres" {
				if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", alertLifecycleLockKey(alert.Source, alert.ExternalID)).Error; err != nil {
					return fmt.Errorf("lock alert lifecycle: %w", err)
				}
			}
			incident := *original
			boundIncident = &incident
			incident.db = tx
			incident.incidentRepo = repository.NewIncidentRepository(tx)
			incident.alertRepo = repository.NewAlertRepository(tx)
			incident.timelineRepo = repository.NewTimelineRepository(tx)
			incident.deferEffect = func(f func()) { effects = append(effects, f) }
			service := *s
			service.alertRepo = incident.alertRepo
			service.incidentSvc = &incident
			service.deferEffect = incident.deferEffect
			if grouping, ok := s.groupingEngine.(*groupingEngine); ok {
				boundGrouping := NewGroupingEngine(grouping.ruleRepo, incident.incidentRepo, tx).(*groupingEngine)
				// Always take locks in alert -> group -> incident order. Resolve
				// deliveries use the same group lock as first firing deliveries.
				if err := boundGrouping.lockAlertGroup(alert.Labels); err != nil {
					return err
				}
				service.groupingEngine = boundGrouping
			}
			var err error
			partial, err = service.processNormalizedAlerts(ctx, []webhooks.NormalizedAlert{alert})
			return err
		})
		if err != nil {
			return nil, fmt.Errorf("process alert lifecycle: %w", err)
		}
		result.Created += partial.Created
		result.Updated += partial.Updated
		result.IncidentsCreated += partial.IncidentsCreated
		// Effects must use committed connections, never the completed transaction.
		*boundIncident = *original
		for _, effect := range effects {
			effect()
		}
	}
	return result, nil
}

// reconcileAlert considers active links only. Closed links remain as history.
// Caller holds the alert identity lock (and matching group lock when configured)
// and uses transaction-bound repositories. Incident rows serialize shared links.
func (s *incidentService) reconcileAlert(ctx context.Context, alert *models.Alert) (bool, error) {
	var incidents []models.Incident
	err := s.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "incidents"}}).
		Joins("JOIN incident_alerts ON incident_alerts.incident_id = incidents.id").
		Where("incident_alerts.alert_id = ? AND incidents.status IN ?", alert.ID, []string{"triggered", "acknowledged"}).
		Order("incidents.id").Find(&incidents).Error
	if err != nil {
		return false, fmt.Errorf("find active alert incidents: %w", err)
	}
	if alert.Status == models.AlertStatusFiring {
		return len(incidents) == 0, nil
	}
	for _, incident := range incidents {
		if incident.CreatedByType != "system" || incident.CreatedByID != "alertmanager" {
			continue
		}
		var firing int64
		if err := s.db.Model(&models.Alert{}).Joins("JOIN incident_alerts ON incident_alerts.alert_id = alerts.id").
			Where("incident_alerts.incident_id = ? AND alerts.status <> ?", incident.ID, models.AlertStatusResolved).Count(&firing).Error; err != nil {
			return false, fmt.Errorf("reconcile incident %s: %w", incident.ID, err)
		}
		if firing != 0 {
			continue
		}
		if err := s.incidentRepo.UpdateStatus(incident.ID, models.IncidentStatusResolved); err != nil {
			return false, fmt.Errorf("reconcile incident %s: %w", incident.ID, err)
		}
		entry := &models.TimelineEntry{
			ID: uuid.New(), IncidentID: incident.ID, Timestamp: time.Now(),
			Type: models.TimelineTypeStatusChanged, ActorType: "system", ActorID: "alertmanager",
			Content: models.JSONB{
				"previous_status": string(incident.Status), "new_status": "resolved",
				"reason": "all linked alerts resolved",
			},
		}
		if err := s.timelineRepo.Create(ctx, entry); err != nil {
			return false, fmt.Errorf("reconcile incident %s: %w", incident.ID, err)
		}
		s.afterCommit(func() {
			bgCtx := context.WithoutCancel(ctx)
			go func() {
				defer recoverAsyncPanic(bgCtx, "notifyAutomaticResolution", "incident_id", incident.ID)
				publishResolved(incident.ID, incident.AIEnabled)
				updated, err := s.incidentRepo.GetByID(bgCtx, incident.ID)
				if err != nil {
					slog.ErrorContext(bgCtx, "reload automatically resolved incident", "incident_id", incident.ID, "error", err)
					return
				}
				if s.chatService != nil && updated.SlackChannelID != "" {
					if err := s.PostStatusUpdateToSlack(updated, incident.Status, models.IncidentStatusResolved); err != nil {
						slog.WarnContext(bgCtx, "notify automatic resolution", "incident_id", incident.ID, "error", err)
					}
				}
				if s.teamsSvc != nil {
					s.postStatusUpdateToTeams(updated, incident.Status, models.IncidentStatusResolved, "alertmanager")
				}
			}()
		})
	}
	return false, nil
}

// Length-prefixing the source makes identities such as ("a:b", "c") and
// ("a", "b:c") distinct. The domain prefix separates alert and group locks.
func alertLifecycleLockKey(source, externalID string) int64 {
	hash := sha256.Sum256([]byte(fmt.Sprintf("regen:alert:%d:%s:%s", len(source), source, externalID)))
	return int64(binary.BigEndian.Uint64(hash[:8]))
}
