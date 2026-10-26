package services

import (
	"errors"
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/google/uuid"
)

type captureNotifier struct{ alerts []*models.Alert }

func (c *captureNotifier) SendEscalationDM(_ string, alert *models.Alert, _ int) error {
	c.alerts = append(c.alerts, alert)
	return nil
}

// pendingAlertEngine builds an engine with one pending, alert-sourced escalation.
func pendingAlertEngine(t *testing.T, alertID uuid.UUID, notifier EscalationNotifier, opts ...EscalationEngineOption) EscalationEngine {
	t.Helper()
	policyID := uuid.New()
	tier := makeTier(policyID, 0, 300, models.EscalationTargetUsers, nil, []string{"alice"})
	repo := newMockEscalationRepo()
	repo.policies[policyID] = &models.EscalationPolicy{ID: policyID, Enabled: true, Tiers: []models.EscalationTier{tier}}
	state := models.EscalationState{
		ID: uuid.New(), AlertID: &alertID, PolicyID: policyID,
		Status: models.EscalationStatePending,
	}
	repo.states[alertID] = &state
	repo.activeStates = []models.EscalationState{state}
	return NewEscalationEngine(repo, &mockScheduleEvaluator{}, notifier, opts...)
}

func TestEscalationEngine_PassesFullAlertToNotifier(t *testing.T) {
	alertID := uuid.New()
	n := &captureNotifier{}
	engine := pendingAlertEngine(t, alertID, n)
	engine.(*escalationEngine).alertLookup = func(id uuid.UUID) (*models.Alert, error) {
		return &models.Alert{ID: id, Title: "EscalationCheck", Severity: models.AlertSeverityCritical}, nil
	}

	if err := engine.EvaluateEscalations(); err != nil {
		t.Fatal(err)
	}
	if len(n.alerts) != 1 || n.alerts[0].Title != "EscalationCheck" || n.alerts[0].Severity != models.AlertSeverityCritical {
		t.Errorf("notifier got %+v, want the full alert", n.alerts)
	}
}

func TestEscalationEngine_StillPagesWhenAlertLookupFails(t *testing.T) {
	alertID := uuid.New()
	n := &captureNotifier{}
	engine := pendingAlertEngine(t, alertID, n)
	engine.(*escalationEngine).alertLookup = func(uuid.UUID) (*models.Alert, error) {
		return nil, errors.New("db down")
	}

	if err := engine.EvaluateEscalations(); err != nil {
		t.Fatal(err)
	}
	if len(n.alerts) != 1 || n.alerts[0].ID != alertID {
		t.Fatalf("expected one page with an ID-only alert, got %+v", n.alerts)
	}
}

type alertRepoStub struct {
	repository.AlertRepository
	alert *models.Alert
}

func (a alertRepoStub) GetByID(id uuid.UUID) (*models.Alert, error) { return a.alert, nil }

func TestWithAlertRepository_LoadsAlertFromRepository(t *testing.T) {
	alertID := uuid.New()
	n := &captureNotifier{}
	stub := alertRepoStub{alert: &models.Alert{ID: alertID, Title: "from repo"}}
	engine := pendingAlertEngine(t, alertID, n, WithAlertRepository(stub))

	if err := engine.EvaluateEscalations(); err != nil {
		t.Fatal(err)
	}
	if len(n.alerts) != 1 || n.alerts[0].Title != "from repo" {
		t.Errorf("notifier got %+v, want the alert from the repository", n.alerts)
	}
}
