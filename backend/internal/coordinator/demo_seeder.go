package coordinator

import (
	"errors"
	"log/slog"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
)

const demoSentinel = "Sample: Redis memory usage high"

// DemoDataExists returns true if the demo dataset has already been seeded.
func DemoDataExists(incidentRepo repository.IncidentRepository) (bool, error) {
	_, err := incidentRepo.GetByNumber(1)
	if err == nil {
		return true, nil
	}
	var notFound *repository.NotFoundError
	if errors.As(err, &notFound) {
		return false, nil
	}
	return false, err
}

// SeedDemoData creates a representative dataset so new installs feel populated.
// Safe to call only when DemoDataExists() returns false.
func SeedDemoData(
	scheduleRepo repository.ScheduleRepository,
	escalationRepo repository.EscalationPolicyRepository,
	routingRepo repository.RoutingRuleRepository,
	incidentRepo repository.IncidentRepository,
	timelineRepo repository.TimelineRepository,
) error {
	// 1. Schedule -----------------------------------------------------------
	schedule := &models.Schedule{
		Name:        "Primary On-Call",
		Description: "Default on-call rotation — replace Alice and Bob with your team members.",
		Timezone:    "UTC",
	}
	if err := scheduleRepo.Create(schedule); err != nil {
		return err
	}
	slog.Info("demo: created schedule", "id", schedule.ID)

	monday := lastMonday()
	layer := &models.ScheduleLayer{
		ScheduleID:           schedule.ID,
		Name:                 "Primary Rotation",
		OrderIndex:           0,
		RotationType:         models.RotationTypeWeekly,
		RotationStart:        monday,
		ShiftDurationSeconds: 604800, // 7 days
	}
	if err := scheduleRepo.CreateLayer(layer); err != nil {
		return err
	}

	participants := []models.ScheduleParticipant{
		{LayerID: layer.ID, UserName: "Alice", OrderIndex: 0},
		{LayerID: layer.ID, UserName: "Bob", OrderIndex: 1},
	}
	if err := scheduleRepo.CreateParticipantsBulk(participants); err != nil {
		return err
	}

	// 2. Escalation policy --------------------------------------------------
	policy := &models.EscalationPolicy{
		Name:        "Default Escalation",
		Description: "Page the on-call engineer. If no response in 10 minutes, escalate to admin.",
		Enabled:     true,
	}
	if err := escalationRepo.CreatePolicy(policy); err != nil {
		return err
	}
	slog.Info("demo: created escalation policy", "id", policy.ID)

	tier0 := &models.EscalationTier{
		PolicyID:       policy.ID,
		TierIndex:      0,
		TimeoutSeconds: 600, // 10 min
		TargetType:     models.EscalationTargetSchedule,
		ScheduleID:     &schedule.ID,
		UserNames:      models.JSONBArray{},
	}
	if err := escalationRepo.CreateTier(tier0); err != nil {
		return err
	}

	tier1 := &models.EscalationTier{
		PolicyID:       policy.ID,
		TierIndex:      1,
		TimeoutSeconds: 600,
		TargetType:     models.EscalationTargetUsers,
		UserNames:      models.JSONBArray{"admin"},
	}
	if err := escalationRepo.CreateTier(tier1); err != nil {
		return err
	}

	// 3. Routing rule -------------------------------------------------------
	rule := &models.RoutingRule{
		Name:        "Critical & Warning → Auto-create Incident",
		Description: "Automatically opens an incident for any critical or warning alert.",
		Enabled:     true,
		Priority:    10,
		MatchCriteria: models.JSONB{
			"severity": []string{"critical", "warning"},
		},
		Actions: models.JSONB{
			"create_incident": true,
		},
	}
	if err := routingRepo.Create(rule); err != nil {
		return err
	}
	slog.Info("demo: created routing rule", "id", rule.ID)

	// 4. Demo incident with full lifecycle timeline -------------------------
	now := time.Now().UTC()
	triggeredAt := now.Add(-2 * time.Hour)
	acknowledgedAt := now.Add(-90 * time.Minute)
	resolvedAt := now.Add(-60 * time.Minute)

	incident := &models.Incident{
		Title:          demoSentinel,
		Slug:           "sample-redis-memory-usage-high",
		Status:         models.IncidentStatusResolved,
		Severity:       models.IncidentSeverityMedium,
		Summary:        "Redis memory usage exceeded 85% on prod-cache-1. Cache key without TTL consuming 2.3 GB flushed. Resolved in 60 minutes.",
		CreatedByType:  "system",
		CreatedByID:    "demo-seeder",
		TriggeredAt:    triggeredAt,
		AcknowledgedAt: &acknowledgedAt,
		ResolvedAt:     &resolvedAt,
	}
	if err := incidentRepo.Create(incident); err != nil {
		return err
	}
	slog.Info("demo: created incident", "id", incident.ID, "number", incident.IncidentNumber)

	entries := []models.TimelineEntry{
		{
			IncidentID: incident.ID,
			Timestamp:  triggeredAt,
			Type:       models.TimelineTypeIncidentCreated,
			ActorType:  "system",
			ActorID:    "routing-engine",
			Content: models.JSONB{
				"text": "Incident auto-created from Prometheus alert: Redis memory usage > 85% on prod-cache-1.",
			},
		},
		{
			IncidentID: incident.ID,
			Timestamp:  acknowledgedAt,
			Type:       models.TimelineTypeStatusChanged,
			ActorType:  "user",
			ActorID:    "Alice",
			Content: models.JSONB{
				"from": "triggered",
				"to":   "acknowledged",
				"text": "Alice acknowledged via Slack.",
			},
		},
		{
			IncidentID: incident.ID,
			Timestamp:  acknowledgedAt.Add(5 * time.Minute),
			Type:       models.TimelineTypeMessage,
			ActorType:  "user",
			ActorID:    "Alice",
			Content: models.JSONB{
				"text": "Root cause identified: a cache key without TTL was consuming 2.3 GB. Flushing now.",
			},
		},
		{
			IncidentID: incident.ID,
			Timestamp:  resolvedAt,
			Type:       models.TimelineTypeStatusChanged,
			ActorType:  "user",
			ActorID:    "Alice",
			Content: models.JSONB{
				"from": "acknowledged",
				"to":   "resolved",
				"text": "Cache key flushed. Memory back to 42%. Resolved.",
			},
		},
	}
	if err := timelineRepo.CreateBulk(entries); err != nil {
		return err
	}

	slog.Info("demo data seeded successfully")
	return nil
}

// lastMonday returns the most recent Monday at midnight UTC.
func lastMonday() time.Time {
	now := time.Now().UTC()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday → 7
	}
	daysBack := weekday - 1
	monday := now.AddDate(0, 0, -daysBack)
	return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.UTC)
}
