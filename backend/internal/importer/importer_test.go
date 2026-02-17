package importer

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/integrations/pagerduty"
	"github.com/openincident/openincident/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Shared mock repos ───────────────────────────────────────────────────────

type mockScheduleRepo struct {
	schedules    []models.Schedule
	layers       []models.ScheduleLayer
	participants []models.ScheduleParticipant
}

func (m *mockScheduleRepo) Create(s *models.Schedule) error {
	m.schedules = append(m.schedules, *s)
	return nil
}
func (m *mockScheduleRepo) GetAll() ([]models.Schedule, error) {
	return m.schedules, nil
}
func (m *mockScheduleRepo) CreateLayer(l *models.ScheduleLayer) error {
	m.layers = append(m.layers, *l)
	return nil
}
func (m *mockScheduleRepo) CreateParticipantsBulk(p []models.ScheduleParticipant) error {
	m.participants = append(m.participants, p...)
	return nil
}

type mockPolicyRepo struct {
	policies []models.EscalationPolicy
	tiers    []models.EscalationTier
}

func (m *mockPolicyRepo) CreatePolicy(p *models.EscalationPolicy) error {
	m.policies = append(m.policies, *p)
	return nil
}
func (m *mockPolicyRepo) GetAllPolicies() ([]models.EscalationPolicy, error) {
	return m.policies, nil
}
func (m *mockPolicyRepo) CreateTier(t *models.EscalationTier) error {
	m.tiers = append(m.tiers, *t)
	return nil
}

// ─── Validator tests ──────────────────────────────────────────────────────────

func TestValidateScheduleLayer_Custom_Skipped(t *testing.T) {
	layer := pagerduty.PDScheduleLayer{
		Name:                      "Weekend Layer",
		RotationTurnLengthSeconds: 0,
	}
	result := validateScheduleLayer("Platform On-Call", 0, layer)
	assert.False(t, result.ok, "custom rotation should be skipped")
	assert.Contains(t, result.warning, "rotation_turn_length_seconds=0")
}

func TestValidateScheduleLayer_Weekly_OK(t *testing.T) {
	layer := pagerduty.PDScheduleLayer{
		Name:                      "Weekly Layer",
		RotationTurnLengthSeconds: 604800,
		Users:                     []pagerduty.PDLayerUser{{User: pagerduty.PDUser{Name: "alice"}}},
	}
	result := validateScheduleLayer("My Schedule", 0, layer)
	assert.True(t, result.ok)
	assert.Empty(t, result.warning)
}

func TestValidateScheduleLayer_Daily_OK(t *testing.T) {
	layer := pagerduty.PDScheduleLayer{
		Name:                      "Daily Layer",
		RotationTurnLengthSeconds: 86400,
		Users:                     []pagerduty.PDLayerUser{{User: pagerduty.PDUser{Name: "bob"}}},
	}
	result := validateScheduleLayer("My Schedule", 0, layer)
	assert.True(t, result.ok)
	assert.Empty(t, result.warning)
}

func TestValidateScheduleLayer_NoUsers_Warning(t *testing.T) {
	layer := pagerduty.PDScheduleLayer{
		Name:                      "Empty Layer",
		RotationTurnLengthSeconds: 604800,
	}
	result := validateScheduleLayer("My Schedule", 0, layer)
	assert.True(t, result.ok, "empty layer is still importable")
	assert.Contains(t, result.warning, "no users")
}

func TestValidateEscalationRule_TeamTarget(t *testing.T) {
	rule := pagerduty.PDEscalationRule{
		EscalationDelayInMinutes: 5,
		Targets:                  []pagerduty.PDTarget{{Type: "team_reference", Name: "Infra Team"}},
	}
	warnings := validateEscalationRule("Infra Default", 0, rule)
	assert.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "team target")
}

func TestValidateEscalationRule_Mixed_PartialWarn(t *testing.T) {
	rule := pagerduty.PDEscalationRule{
		EscalationDelayInMinutes: 10,
		Targets: []pagerduty.PDTarget{
			{Type: "schedule_reference", Name: "Primary"},
			{Type: "team_reference", Name: "Backend Team"},
		},
	}
	warnings := validateEscalationRule("My Policy", 0, rule)
	assert.Len(t, warnings, 1, "only team target should warn")
	assert.Contains(t, warnings[0], "team target")
}

func TestValidateEscalationRule_NoTeam_NoWarning(t *testing.T) {
	rule := pagerduty.PDEscalationRule{
		Targets: []pagerduty.PDTarget{
			{Type: "schedule_reference", Name: "Primary"},
			{Type: "user_reference", Name: "alice"},
		},
	}
	warnings := validateEscalationRule("My Policy", 0, rule)
	assert.Empty(t, warnings)
}

// ─── Schedule importer tests ──────────────────────────────────────────────────

func TestImportSchedule_Basic(t *testing.T) {
	repo := &mockScheduleRepo{}
	emailToName := map[string]string{"alice@example.com": "Alice"}
	report := &ImportReport{}

	pdSchedule := pagerduty.PDScheduleDetail{
		ID:       "S1",
		Name:     "Primary On-Call",
		TimeZone: "UTC",
		ScheduleLayers: []pagerduty.PDScheduleLayer{
			{
				Name:                      "Layer 1",
				RotationTurnLengthSeconds: 604800,
				RotationVirtualStart:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				Users:                     []pagerduty.PDLayerUser{{User: pagerduty.PDUser{Email: "alice@example.com", Name: "Alice"}}},
			},
		},
	}

	err := importSchedule(repo, pdSchedule, emailToName, false, report)
	require.NoError(t, err)

	assert.Equal(t, 1, report.Summary.SchedulesImported)
	assert.Len(t, repo.schedules, 1)
	assert.Equal(t, "Primary On-Call", repo.schedules[0].Name)
	assert.Equal(t, "UTC", repo.schedules[0].Timezone)
	assert.Len(t, repo.layers, 1)
	assert.Equal(t, 604800, repo.layers[0].ShiftDurationSeconds)
	assert.Equal(t, models.RotationTypeWeekly, repo.layers[0].RotationType)
	assert.Len(t, repo.participants, 1)
	assert.Equal(t, "Alice", repo.participants[0].UserName)
}

func TestImportSchedule_DailyRotation(t *testing.T) {
	repo := &mockScheduleRepo{}
	report := &ImportReport{}

	pdSchedule := pagerduty.PDScheduleDetail{
		ID:   "S2",
		Name: "Daily Rota",
		ScheduleLayers: []pagerduty.PDScheduleLayer{
			{RotationTurnLengthSeconds: 86400, Users: []pagerduty.PDLayerUser{{User: pagerduty.PDUser{Name: "bob"}}}},
		},
	}

	err := importSchedule(repo, pdSchedule, nil, false, report)
	require.NoError(t, err)
	assert.Equal(t, models.RotationTypeDaily, repo.layers[0].RotationType)
}

func TestImportSchedule_EmailFallback(t *testing.T) {
	repo := &mockScheduleRepo{}
	report := &ImportReport{}

	pdSchedule := pagerduty.PDScheduleDetail{
		ID:   "S3",
		Name: "Backup Rota",
		ScheduleLayers: []pagerduty.PDScheduleLayer{
			{
				RotationTurnLengthSeconds: 86400,
				Users:                     []pagerduty.PDLayerUser{{User: pagerduty.PDUser{Email: "bob@example.com", Name: "Bob"}}},
			},
		},
	}

	err := importSchedule(repo, pdSchedule, map[string]string{}, false, report)
	require.NoError(t, err)
	// No email→name mapping → falls back to email
	assert.Equal(t, "bob@example.com", repo.participants[0].UserName)
}

func TestImportSchedule_CustomLayerSkipped(t *testing.T) {
	repo := &mockScheduleRepo{}
	report := &ImportReport{}

	pdSchedule := pagerduty.PDScheduleDetail{
		ID:   "S4",
		Name: "Complex Rota",
		ScheduleLayers: []pagerduty.PDScheduleLayer{
			{Name: "Custom", RotationTurnLengthSeconds: 0},
		},
	}

	err := importSchedule(repo, pdSchedule, nil, false, report)
	require.NoError(t, err)
	assert.Equal(t, 1, report.Summary.SchedulesImported)
	assert.Equal(t, 1, report.Summary.LayersSkipped)
	assert.Equal(t, 0, report.Summary.LayersImported)
	assert.Len(t, report.Warnings, 1)
}

func TestImportSchedule_ConflictSkip(t *testing.T) {
	repo := &mockScheduleRepo{schedules: []models.Schedule{{Name: "Existing Rota"}}}
	report := &ImportReport{}

	pdSchedule := pagerduty.PDScheduleDetail{ID: "S5", Name: "Existing Rota"}

	err := importSchedule(repo, pdSchedule, nil, false, report)
	require.NoError(t, err)
	assert.Equal(t, 0, report.Summary.SchedulesImported)
	assert.Equal(t, 1, report.Summary.SchedulesSkipped)
	assert.Len(t, report.Warnings, 1)
	assert.Contains(t, report.Warnings[0], "name conflict")
}

func TestImportSchedule_ForceOverwrite(t *testing.T) {
	repo := &mockScheduleRepo{schedules: []models.Schedule{{Name: "Existing Rota"}}}
	report := &ImportReport{}

	pdSchedule := pagerduty.PDScheduleDetail{
		ID:   "S6",
		Name: "Existing Rota",
		ScheduleLayers: []pagerduty.PDScheduleLayer{
			{RotationTurnLengthSeconds: 604800, Users: []pagerduty.PDLayerUser{{User: pagerduty.PDUser{Name: "alice"}}}},
		},
	}

	err := importSchedule(repo, pdSchedule, nil, true, report)
	require.NoError(t, err)
	assert.Equal(t, 1, report.Summary.SchedulesImported)
}

// ─── Policy importer tests ────────────────────────────────────────────────────

func TestImportPolicy_Basic(t *testing.T) {
	repo := &mockPolicyRepo{}
	scheduleNameToID := map[string]uuid.UUID{
		"Primary Rota": uuid.MustParse("00000000-0000-0000-0000-000000000001"),
	}
	report := &ImportReport{}

	pdPolicy := pagerduty.PDEscalationPolicyDetail{
		ID:   "P1",
		Name: "Default Escalation",
		EscalationRules: []pagerduty.PDEscalationRule{
			{
				EscalationDelayInMinutes: 5,
				Targets:                  []pagerduty.PDTarget{{Type: "schedule_reference", Name: "Primary Rota"}},
			},
			{
				EscalationDelayInMinutes: 10,
				Targets:                  []pagerduty.PDTarget{{Type: "user_reference", Name: "alice"}},
			},
		},
	}

	err := importPolicy(repo, pdPolicy, scheduleNameToID, nil, false, report)
	require.NoError(t, err)

	assert.Equal(t, 1, report.Summary.PoliciesImported)
	assert.Equal(t, 2, report.Summary.TiersImported)
	require.Len(t, repo.tiers, 2)

	tier0 := repo.tiers[0]
	assert.Equal(t, 0, tier0.TierIndex)
	assert.Equal(t, 300, tier0.TimeoutSeconds)
	assert.Equal(t, models.EscalationTargetSchedule, tier0.TargetType)
	assert.Equal(t, scheduleNameToID["Primary Rota"], *tier0.ScheduleID)

	tier1 := repo.tiers[1]
	assert.Equal(t, 1, tier1.TierIndex)
	assert.Equal(t, 600, tier1.TimeoutSeconds)
	assert.Equal(t, models.EscalationTargetUsers, tier1.TargetType)
	assert.Equal(t, models.JSONBArray{"alice"}, tier1.UserNames)
}

func TestImportPolicy_TeamTargetSkipped(t *testing.T) {
	repo := &mockPolicyRepo{}
	report := &ImportReport{}

	pdPolicy := pagerduty.PDEscalationPolicyDetail{
		ID:   "P2",
		Name: "Team Policy",
		EscalationRules: []pagerduty.PDEscalationRule{
			{
				EscalationDelayInMinutes: 5,
				Targets:                  []pagerduty.PDTarget{{Type: "team_reference", Name: "Backend Team"}},
			},
		},
	}

	err := importPolicy(repo, pdPolicy, nil, nil, false, report)
	require.NoError(t, err)
	assert.Equal(t, 1, report.Summary.PoliciesImported)
	assert.Len(t, report.Warnings, 1)
	assert.Contains(t, report.Warnings[0], "team target")
}

func TestImportPolicy_ConflictSkip(t *testing.T) {
	repo := &mockPolicyRepo{policies: []models.EscalationPolicy{{Name: "Existing Policy"}}}
	report := &ImportReport{}

	pdPolicy := pagerduty.PDEscalationPolicyDetail{ID: "P3", Name: "Existing Policy"}

	err := importPolicy(repo, pdPolicy, nil, nil, false, report)
	require.NoError(t, err)
	assert.Equal(t, 0, report.Summary.PoliciesImported)
	assert.Equal(t, 1, report.Summary.PoliciesSkipped)
}

func TestImportPolicy_BothTargetType(t *testing.T) {
	scheduleID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	repo := &mockPolicyRepo{}
	report := &ImportReport{}

	pdPolicy := pagerduty.PDEscalationPolicyDetail{
		ID:   "P4",
		Name: "Both Policy",
		EscalationRules: []pagerduty.PDEscalationRule{
			{
				EscalationDelayInMinutes: 5,
				Targets: []pagerduty.PDTarget{
					{Type: "schedule_reference", Name: "Primary"},
					{Type: "user_reference", Name: "alice"},
				},
			},
		},
	}

	err := importPolicy(repo, pdPolicy, map[string]uuid.UUID{"Primary": scheduleID}, nil, false, report)
	require.NoError(t, err)
	require.Len(t, repo.tiers, 1)
	assert.Equal(t, models.EscalationTargetBoth, repo.tiers[0].TargetType)
}
