package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/fluidify/regen/internal/database"
	"github.com/fluidify/regen/internal/models"
	"github.com/fluidify/regen/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ----------------------------------------------------------------------------
// Integration test DB: all tables needed for escalation flow
// ----------------------------------------------------------------------------

func setupEscalationIntegrationDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()
	db, err := database.NewTestDB()
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)

	tables := []string{
		"timeline_entries", "incident_alerts", "incidents",
		"escalation_severity_rules", "escalation_states", "escalation_tiers", "escalation_policies", "alerts",
	}
	for _, tbl := range tables {
		sqlDB.Exec("DROP TABLE IF EXISTS " + tbl)
	}
	sqlDB.Exec("DROP TRIGGER IF EXISTS assign_incident_number")

	_, err = sqlDB.Exec(`CREATE TABLE alerts (
		id TEXT PRIMARY KEY,
		external_id TEXT NOT NULL,
		source TEXT NOT NULL,
		fingerprint TEXT,
		status TEXT NOT NULL DEFAULT 'firing',
		severity TEXT NOT NULL DEFAULT 'info',
		title TEXT NOT NULL,
		description TEXT,
		labels TEXT DEFAULT '{}',
		annotations TEXT DEFAULT '{}',
		raw_payload TEXT,
		started_at DATETIME NOT NULL,
		ended_at DATETIME,
		received_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		escalation_policy_id TEXT,
		acknowledgment_status TEXT NOT NULL DEFAULT 'pending'
	)`)
	require.NoError(t, err)

	_, err = sqlDB.Exec(`CREATE TABLE incidents (
		id TEXT PRIMARY KEY,
		incident_number INTEGER UNIQUE,
		title TEXT NOT NULL,
		slug TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'triggered',
		severity TEXT NOT NULL DEFAULT 'medium',
		summary TEXT,
		group_key TEXT,
		slack_channel_id TEXT,
		slack_channel_name TEXT,
		slack_message_ts TEXT,
		teams_channel_id TEXT,
		teams_channel_name TEXT,
		teams_conversation_id TEXT,
		teams_activity_id TEXT,
		ai_enabled INTEGER NOT NULL DEFAULT 1,
		ai_summary TEXT,
		ai_summary_generated_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		triggered_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		acknowledged_at DATETIME,
		resolved_at DATETIME,
		canceled_at DATETIME,
		created_by_type TEXT NOT NULL DEFAULT 'system',
		created_by_id TEXT NOT NULL DEFAULT 'system',
		commander_id TEXT,
		labels TEXT DEFAULT '{}',
		custom_fields TEXT DEFAULT '{}'
	)`)
	require.NoError(t, err)

	_, err = sqlDB.Exec(`CREATE TRIGGER assign_incident_number
		AFTER INSERT ON incidents
		BEGIN
			UPDATE incidents SET incident_number = (
				SELECT COALESCE(MAX(incident_number), 0) + 1 FROM incidents WHERE id != NEW.id
			) WHERE id = NEW.id;
		END`)
	require.NoError(t, err)

	_, err = sqlDB.Exec(`CREATE TABLE incident_alerts (
		incident_id TEXT NOT NULL,
		alert_id TEXT NOT NULL,
		linked_by_type TEXT,
		linked_by_id TEXT,
		linked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (incident_id, alert_id)
	)`)
	require.NoError(t, err)

	_, err = sqlDB.Exec(`CREATE TABLE timeline_entries (
		id TEXT PRIMARY KEY,
		incident_id TEXT NOT NULL,
		timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		type TEXT NOT NULL,
		actor_type TEXT NOT NULL DEFAULT 'system',
		actor_id TEXT NOT NULL DEFAULT 'system',
		content TEXT DEFAULT '{}',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)

	_, err = sqlDB.Exec(`CREATE TABLE escalation_policies (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT,
		enabled INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)

	_, err = sqlDB.Exec(`CREATE TABLE escalation_tiers (
		id TEXT PRIMARY KEY,
		policy_id TEXT NOT NULL REFERENCES escalation_policies(id),
		tier_index INTEGER NOT NULL,
		timeout_seconds INTEGER NOT NULL DEFAULT 300,
		target_type TEXT NOT NULL DEFAULT 'users',
		schedule_id TEXT,
		user_names TEXT NOT NULL DEFAULT '[]',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE (policy_id, tier_index)
	)`)
	require.NoError(t, err)

	_, err = sqlDB.Exec(`CREATE TABLE escalation_states (
		id TEXT PRIMARY KEY,
		alert_id TEXT UNIQUE,
		incident_id TEXT,
		source_type TEXT NOT NULL DEFAULT 'alert',
		policy_id TEXT NOT NULL,
		current_tier_index INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'pending',
		last_notified_at DATETIME,
		acknowledged_at DATETIME,
		acknowledged_by TEXT,
		acknowledged_via TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)

	_, err = sqlDB.Exec(`CREATE TABLE escalation_severity_rules (
		id TEXT PRIMARY KEY,
		severity TEXT NOT NULL UNIQUE,
		escalation_policy_id TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)

	cleanup := func() {
		for _, tbl := range tables {
			sqlDB.Exec("DROP TABLE IF EXISTS " + tbl)
		}
		sqlDB.Exec("DROP TRIGGER IF EXISTS assign_incident_number")
	}
	return db, cleanup
}

// ----------------------------------------------------------------------------
// Integration-specific helpers (prefixed intg to avoid collision with unit tests)
// ----------------------------------------------------------------------------

// intgMockNotifier is an EscalationNotifier that records calls.
type intgMockNotifier struct {
	SendFn func(userID string, alert *models.Alert, tierIndex int) error
}

func (m *intgMockNotifier) SendEscalationDM(userID string, alert *models.Alert, tierIndex int) error {
	return m.SendFn(userID, alert, tierIndex)
}

type intgNoScheduleEvaluator struct{}

func (n *intgNoScheduleEvaluator) WhoIsOnCall(scheduleID uuid.UUID, at time.Time) (string, error) {
	return "", nil // no on-call user
}
func (n *intgNoScheduleEvaluator) GetTimeline(scheduleID uuid.UUID, from, to time.Time) ([]TimelineSegment, error) {
	return nil, nil
}
func (n *intgNoScheduleEvaluator) GetLayerTimelines(scheduleID uuid.UUID, from, to time.Time) (map[uuid.UUID][]TimelineSegment, []TimelineSegment, error) {
	return nil, nil, nil
}

func intgMakePolicy(t *testing.T, repo repository.EscalationPolicyRepository, name string) *models.EscalationPolicy {
	t.Helper()
	policy := &models.EscalationPolicy{ID: uuid.New(), Name: name, Enabled: true}
	require.NoError(t, repo.CreatePolicy(policy))
	return policy
}

func intgMakeTier(t *testing.T, repo repository.EscalationPolicyRepository, policyID uuid.UUID, idx, timeoutSecs int, users []string) {
	t.Helper()
	tier := &models.EscalationTier{
		ID:             uuid.New(),
		PolicyID:       policyID,
		TierIndex:      idx,
		TimeoutSeconds: timeoutSecs,
		TargetType:     models.EscalationTargetUsers,
		UserNames:      models.JSONBArray(users),
	}
	require.NoError(t, repo.CreateTier(tier))
}

func intgMakeAlert(t *testing.T, repo repository.AlertRepository, policyID *uuid.UUID) *models.Alert {
	t.Helper()
	alert := &models.Alert{
		ID:                 uuid.New(),
		ExternalID:         uuid.New().String(),
		Source:             "prometheus",
		Fingerprint:        uuid.New().String(),
		Status:             models.AlertStatusFiring,
		Severity:           models.AlertSeverityCritical,
		Title:              "High CPU Usage",
		StartedAt:          time.Now(),
		ReceivedAt:         time.Now(),
		EscalationPolicyID: policyID,
	}
	require.NoError(t, repo.Create(alert))
	return alert
}

// ----------------------------------------------------------------------------
// Integration tests
// ----------------------------------------------------------------------------

// TestIntegration_TriggerAndAcknowledge: full happy-path with real repos.
func TestIntegration_TriggerAndAcknowledge(t *testing.T) {
	db, cleanup := setupEscalationIntegrationDB(t)
	defer cleanup()

	escalationRepo := repository.NewEscalationPolicyRepository(db)
	alertRepo := repository.NewAlertRepository(db)

	policy := intgMakePolicy(t, escalationRepo, "pager-team")
	intgMakeTier(t, escalationRepo, policy.ID, 0, 300, []string{"alice", "bob"})

	alert := intgMakeAlert(t, alertRepo, &policy.ID)

	dmsSent := []string{}
	engine := NewEscalationEngine(escalationRepo, &intgNoScheduleEvaluator{}, &intgMockNotifier{
		SendFn: func(userID string, _ *models.Alert, _ int) error {
			dmsSent = append(dmsSent, userID)
			return nil
		},
	})

	// Trigger creates state (pending, no DMs yet)
	require.NoError(t, engine.TriggerEscalation(alert))
	state, err := escalationRepo.GetStateByAlert(alert.ID)
	require.NoError(t, err)
	assert.Equal(t, models.EscalationStatePending, state.Status)

	// First evaluate notifies tier-0
	require.NoError(t, engine.EvaluateEscalations())
	assert.ElementsMatch(t, []string{"alice", "bob"}, dmsSent)

	state, err = escalationRepo.GetStateByAlert(alert.ID)
	require.NoError(t, err)
	assert.Equal(t, models.EscalationStateNotified, state.Status)

	// Acknowledge stops escalation
	require.NoError(t, engine.AcknowledgeAlert(alert.ID, "alice", models.AcknowledgmentViaAPI))
	state, err = escalationRepo.GetStateByAlert(alert.ID)
	require.NoError(t, err)
	assert.Equal(t, models.EscalationStateAcknowledged, state.Status)
	require.NotNil(t, state.AcknowledgedBy)
	assert.Equal(t, "alice", *state.AcknowledgedBy)

	// Further evaluate sends no more DMs
	dmsSent = nil
	require.NoError(t, engine.EvaluateEscalations())
	assert.Empty(t, dmsSent)
}

// TestIntegration_NoPolicy_NoStateCreated: alert without policy → no escalation state.
func TestIntegration_NoPolicy_NoStateCreated(t *testing.T) {
	db, cleanup := setupEscalationIntegrationDB(t)
	defer cleanup()

	escalationRepo := repository.NewEscalationPolicyRepository(db)
	alertRepo := repository.NewAlertRepository(db)
	alert := intgMakeAlert(t, alertRepo, nil)

	engine := NewEscalationEngine(escalationRepo, &intgNoScheduleEvaluator{}, &intgMockNotifier{
		SendFn: func(string, *models.Alert, int) error { return nil },
	})
	require.NoError(t, engine.TriggerEscalation(alert))

	_, err := escalationRepo.GetStateByAlert(alert.ID)
	assert.Error(t, err, "no state should be created when alert has no policy")
}

// TestIntegration_AcknowledgeAtTier0_Tier1NeverTriggered: early ack prevents tier-1 notify.
func TestIntegration_AcknowledgeAtTier0_Tier1NeverTriggered(t *testing.T) {
	db, cleanup := setupEscalationIntegrationDB(t)
	defer cleanup()

	escalationRepo := repository.NewEscalationPolicyRepository(db)
	alertRepo := repository.NewAlertRepository(db)

	policy := intgMakePolicy(t, escalationRepo, "two-tier")
	intgMakeTier(t, escalationRepo, policy.ID, 0, 1, []string{"tier0user"}) // 1-second timeout
	intgMakeTier(t, escalationRepo, policy.ID, 1, 300, []string{"tier1user"})

	alert := intgMakeAlert(t, alertRepo, &policy.ID)

	tier1Notified := false
	engine := NewEscalationEngine(escalationRepo, &intgNoScheduleEvaluator{}, &intgMockNotifier{
		SendFn: func(_ string, _ *models.Alert, tierIndex int) error {
			if tierIndex == 1 {
				tier1Notified = true
			}
			return nil
		},
	})

	require.NoError(t, engine.TriggerEscalation(alert))
	require.NoError(t, engine.EvaluateEscalations()) // notify tier-0

	// Acknowledge before tier-0 times out
	require.NoError(t, engine.AcknowledgeAlert(alert.ID, "tier0user", models.AcknowledgmentViaSlack))

	// Wait for tier-0 timeout, then evaluate — tier-1 must NOT fire
	time.Sleep(1100 * time.Millisecond)
	require.NoError(t, engine.EvaluateEscalations())
	assert.False(t, tier1Notified)
}

// TestIntegration_AlertResolves_EscalationCompleted: MarkAlertCompleted stops polling.
func TestIntegration_AlertResolves_EscalationCompleted(t *testing.T) {
	db, cleanup := setupEscalationIntegrationDB(t)
	defer cleanup()

	escalationRepo := repository.NewEscalationPolicyRepository(db)
	alertRepo := repository.NewAlertRepository(db)

	policy := intgMakePolicy(t, escalationRepo, "resolver-policy")
	intgMakeTier(t, escalationRepo, policy.ID, 0, 300, []string{"on-call"})

	alert := intgMakeAlert(t, alertRepo, &policy.ID)

	dmCount := 0
	engine := NewEscalationEngine(escalationRepo, &intgNoScheduleEvaluator{}, &intgMockNotifier{
		SendFn: func(string, *models.Alert, int) error { dmCount++; return nil },
	})

	require.NoError(t, engine.TriggerEscalation(alert))
	require.NoError(t, engine.EvaluateEscalations())
	assert.Equal(t, 1, dmCount)

	require.NoError(t, engine.MarkAlertCompleted(alert.ID))
	state, err := escalationRepo.GetStateByAlert(alert.ID)
	require.NoError(t, err)
	assert.Equal(t, models.EscalationStateCompleted, state.Status)

	dmCount = 0
	require.NoError(t, engine.EvaluateEscalations())
	assert.Equal(t, 0, dmCount)
}

// TestIntegration_MultiTier_AdvancesOnTimeout: backdating last_notified_at simulates timeout.
func TestIntegration_MultiTier_AdvancesOnTimeout(t *testing.T) {
	db, cleanup := setupEscalationIntegrationDB(t)
	defer cleanup()

	escalationRepo := repository.NewEscalationPolicyRepository(db)
	alertRepo := repository.NewAlertRepository(db)

	policy := intgMakePolicy(t, escalationRepo, "multi-tier")
	intgMakeTier(t, escalationRepo, policy.ID, 0, 60, []string{"tier0"})
	intgMakeTier(t, escalationRepo, policy.ID, 1, 60, []string{"tier1"})

	alert := intgMakeAlert(t, alertRepo, &policy.ID)

	tier1Notified := false
	engine := NewEscalationEngine(escalationRepo, &intgNoScheduleEvaluator{}, &intgMockNotifier{
		SendFn: func(_ string, _ *models.Alert, tierIndex int) error {
			if tierIndex == 1 {
				tier1Notified = true
			}
			return nil
		},
	})

	require.NoError(t, engine.TriggerEscalation(alert))
	require.NoError(t, engine.EvaluateEscalations()) // notify tier-0

	// Backdate last_notified_at by 2 minutes to simulate tier-0 timeout elapsed
	sqlDB, _ := db.DB()
	past := time.Now().Add(-2 * time.Minute).UTC().Format("2006-01-02 15:04:05")
	sqlDB.Exec(`UPDATE escalation_states SET last_notified_at = ? WHERE alert_id = ?`, past, alert.ID.String())

	require.NoError(t, engine.EvaluateEscalations()) // should advance to tier-1
	assert.True(t, tier1Notified)
}

// TestIntegration_AcknowledgeAlert_Idempotent: double-ack does not error.
func TestIntegration_AcknowledgeAlert_Idempotent(t *testing.T) {
	db, cleanup := setupEscalationIntegrationDB(t)
	defer cleanup()

	escalationRepo := repository.NewEscalationPolicyRepository(db)
	alertRepo := repository.NewAlertRepository(db)

	policy := intgMakePolicy(t, escalationRepo, "idem-policy")
	intgMakeTier(t, escalationRepo, policy.ID, 0, 300, []string{"on-call"})
	alert := intgMakeAlert(t, alertRepo, &policy.ID)

	engine := NewEscalationEngine(escalationRepo, &intgNoScheduleEvaluator{}, &intgMockNotifier{
		SendFn: func(string, *models.Alert, int) error { return nil },
	})
	require.NoError(t, engine.TriggerEscalation(alert))

	require.NoError(t, engine.AcknowledgeAlert(alert.ID, "alice", models.AcknowledgmentViaAPI))
	require.NoError(t, engine.AcknowledgeAlert(alert.ID, "alice", models.AcknowledgmentViaAPI)) // idempotent

	state, err := escalationRepo.GetStateByAlert(alert.ID)
	require.NoError(t, err)
	assert.Equal(t, models.EscalationStateAcknowledged, state.Status)
}

// TestIntegration_AcknowledgeHandler_CreatesTimelineEntry: calling AcknowledgeAlertWithTimeline
// creates a timeline entry on the linked incident.
func TestIntegration_AcknowledgeHandler_CreatesTimelineEntry(t *testing.T) {
	db, cleanup := setupEscalationIntegrationDB(t)
	defer cleanup()

	escalationRepo := repository.NewEscalationPolicyRepository(db)
	alertRepo := repository.NewAlertRepository(db)
	incidentRepo := repository.NewIncidentRepository(db)
	timelineRepo := repository.NewTimelineRepository(db)

	policy := intgMakePolicy(t, escalationRepo, "timeline-policy")
	intgMakeTier(t, escalationRepo, policy.ID, 0, 300, []string{"on-call"})
	alert := intgMakeAlert(t, alertRepo, &policy.ID)

	// Create incident and link the alert to it
	incident := &models.Incident{
		ID:            uuid.New(),
		Title:         "CPU incident",
		Slug:          "cpu-incident",
		Status:        models.IncidentStatusTriggered,
		Severity:      models.IncidentSeverityCritical,
		CreatedByType: "system",
		CreatedByID:   "test",
		TriggeredAt:   time.Now(),
	}
	require.NoError(t, incidentRepo.Create(incident))
	require.NoError(t, incidentRepo.LinkAlert(incident.ID, alert.ID, "system", "test"))

	engine := NewEscalationEngine(escalationRepo, &intgNoScheduleEvaluator{}, &intgMockNotifier{
		SendFn: func(string, *models.Alert, int) error { return nil },
	})
	require.NoError(t, engine.TriggerEscalation(alert))

	// AcknowledgeAlertWithTimeline is the service-layer function we are about to create
	err := AcknowledgeAlertWithTimeline(alert.ID, "alice", models.AcknowledgmentViaAPI, engine, incidentRepo, timelineRepo)
	require.NoError(t, err)

	entries, total, err := timelineRepo.GetByIncidentID(incident.ID, repository.Pagination{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "alert_acknowledged", entries[0].Type)
	assert.Equal(t, "alice", entries[0].ActorID)
}
