package services

// Tests for REG-13: RED metrics on the "escalation dispatch" critical path
// (notifyTier). Kept separate from escalation_engine_test.go's behavioral
// tests since these assert on package-global Prometheus counters/histograms
// via before/after deltas.

import (
	"errors"
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/metrics"
	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
)

func dispatchSampleCount(t *testing.T, obs prometheus.Histogram) uint64 {
	t.Helper()
	var m dto.Metric
	if err := obs.Write(&m); err != nil {
		t.Fatalf("Write: %v", err)
	}
	return m.GetHistogram().GetSampleCount()
}

func TestNotifyTier_Success_RecordsDispatchDuration(t *testing.T) {
	schedID := uuid.New()
	policyID := uuid.New()
	tier0 := makeTier(policyID, 0, 300, models.EscalationTargetSchedule, &schedID, nil)
	policy := &models.EscalationPolicy{ID: policyID, Enabled: true, Tiers: []models.EscalationTier{tier0}}

	repo := newMockEscalationRepo()
	repo.policies[policyID] = policy
	alertID := uuid.New()
	state := models.EscalationState{
		ID: uuid.New(), AlertID: &alertID, PolicyID: policyID,
		CurrentTierIndex: 0, Status: models.EscalationStatePending,
	}
	repo.states[alertID] = &state
	repo.activeStates = []models.EscalationState{state}

	chat := &mockChatForEscalation{}
	evaluator := &mockScheduleEvaluator{onCallUser: "alice"}
	alert := &models.Alert{ID: alertID, EscalationPolicyID: &policyID}
	engine := NewEscalationEngine(repo, evaluator, chat)
	engine.(*escalationEngine).alertLookup = func(id uuid.UUID) (*models.Alert, error) { return alert, nil }

	before := dispatchSampleCount(t, metrics.EscalationDispatchDurationSeconds)
	failedBefore := testutil.ToFloat64(metrics.EscalationDispatchFailedTotal)

	if err := engine.EvaluateEscalations(); err != nil {
		t.Fatalf("EvaluateEscalations: %v", err)
	}

	after := dispatchSampleCount(t, metrics.EscalationDispatchDurationSeconds)
	if after != before+1 {
		t.Errorf("regen_escalation_dispatch_duration_seconds sample count = %d, want %d", after, before+1)
	}
	failedAfter := testutil.ToFloat64(metrics.EscalationDispatchFailedTotal)
	if failedAfter != failedBefore {
		t.Errorf("regen_escalation_dispatch_failed_total changed on a successful dispatch: %v -> %v", failedBefore, failedAfter)
	}
}

func TestNotifyTier_UpdateStateFails_RecordsDispatchFailure(t *testing.T) {
	schedID := uuid.New()
	policyID := uuid.New()
	tier0 := makeTier(policyID, 0, 300, models.EscalationTargetSchedule, &schedID, nil)
	policy := &models.EscalationPolicy{ID: policyID, Enabled: true, Tiers: []models.EscalationTier{tier0}}

	repo := newMockEscalationRepo()
	repo.policies[policyID] = policy
	alertID := uuid.New()
	state := models.EscalationState{
		ID: uuid.New(), AlertID: &alertID, PolicyID: policyID,
		CurrentTierIndex: 0, Status: models.EscalationStatePending,
	}
	repo.states[alertID] = &state
	repo.activeStates = []models.EscalationState{state}
	repo.updateErr = errors.New("update state boom")

	chat := &mockChatForEscalation{}
	evaluator := &mockScheduleEvaluator{onCallUser: "alice"}
	alert := &models.Alert{ID: alertID, EscalationPolicyID: &policyID}
	engine := NewEscalationEngine(repo, evaluator, chat)
	engine.(*escalationEngine).alertLookup = func(id uuid.UUID) (*models.Alert, error) { return alert, nil }

	before := testutil.ToFloat64(metrics.EscalationDispatchFailedTotal)

	// EvaluateEscalations itself never returns processState's error (logged
	// and swallowed per-state so one bad state doesn't block others) — the
	// dispatch-level metric is what makes this failure visible at all.
	if err := engine.EvaluateEscalations(); err != nil {
		t.Fatalf("EvaluateEscalations: %v", err)
	}

	after := testutil.ToFloat64(metrics.EscalationDispatchFailedTotal)
	if after != before+1 {
		t.Errorf("regen_escalation_dispatch_failed_total = %v, want %v", after, before+1)
	}
}
