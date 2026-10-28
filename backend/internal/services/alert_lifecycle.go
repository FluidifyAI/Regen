package services

import (
	"context"
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
			// Serialize lifecycle decisions across sources and grouping keys. A single
			// transaction-scoped lock also covers first inserts (no row to lock yet) and
			// multi-alert groups. Never hold it while calling external integrations.
			if tx.Name() == "postgres" {
				if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", int64(0x726567656e)).Error; err != nil {
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
				service.groupingEngine = NewGroupingEngine(grouping.ruleRepo, incident.incidentRepo, tx)
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
// Caller holds the lifecycle lock and uses transaction-bound repositories.
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
