package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/fluidify/regen/internal/models"
	"github.com/fluidify/regen/internal/repository"
)

// ── Mock repository ───────────────────────────────────────────────────────────

type mockEscalationRepo struct {
	policies     map[uuid.UUID]*models.EscalationPolicy
	states       map[uuid.UUID]*models.EscalationState // keyed by alert_id
	createErr    error
	getStateErr  error
	updateErr    error
	ackErr       error
	activeStates []models.EscalationState
}

func newMockEscalationRepo() *mockEscalationRepo {
	return &mockEscalationRepo{
		policies: make(map[uuid.UUID]*models.EscalationPolicy),
		states:   make(map[uuid.UUID]*models.EscalationState),
	}
}

func (m *mockEscalationRepo) CreatePolicy(p *models.EscalationPolicy) error {
	m.policies[p.ID] = p
	return nil
}
func (m *mockEscalationRepo) GetPolicyByID(id uuid.UUID) (*models.EscalationPolicy, error) {
	p, ok := m.policies[id]
	if !ok {
		return nil, &repository.NotFoundError{Resource: "escalation_policy", ID: id.String()}
	}
	return p, nil
}
func (m *mockEscalationRepo) GetPolicyWithTiers(id uuid.UUID) (*models.EscalationPolicy, error) {
	return m.GetPolicyByID(id)
}
func (m *mockEscalationRepo) GetAllPolicies() ([]models.EscalationPolicy, error) { return nil, nil }
func (m *mockEscalationRepo) GetAllPoliciesWithTiers() ([]models.EscalationPolicy, error) {
	out := make([]models.EscalationPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		out = append(out, *p)
	}
	return out, nil
}
func (m *mockEscalationRepo) GetEnabledPolicies() ([]models.EscalationPolicy, error) {
	return nil, nil
}
func (m *mockEscalationRepo) UpdatePolicy(p *models.EscalationPolicy) error  { return nil }
func (m *mockEscalationRepo) DeletePolicy(id uuid.UUID) error                 { return nil }
func (m *mockEscalationRepo) CreateTier(t *models.EscalationTier) error       { return nil }
func (m *mockEscalationRepo) GetTiersByPolicy(id uuid.UUID) ([]models.EscalationTier, error) {
	p, ok := m.policies[id]
	if !ok {
		return nil, nil
	}
	return p.Tiers, nil
}
func (m *mockEscalationRepo) UpdateTier(t *models.EscalationTier) error { return nil }
func (m *mockEscalationRepo) DeleteTier(id uuid.UUID) error              { return nil }

func (m *mockEscalationRepo) CreateState(s *models.EscalationState) error {
	if m.createErr != nil {
		return m.createErr
	}
	if s.AlertID != nil {
		m.states[*s.AlertID] = s
	}
	return nil
}
func (m *mockEscalationRepo) GetStateByAlert(alertID uuid.UUID) (*models.EscalationState, error) {
	if m.getStateErr != nil {
		return nil, m.getStateErr
	}
	s, ok := m.states[alertID]
	if !ok {
		return nil, &repository.NotFoundError{Resource: "escalation_state", ID: alertID.String()}
	}
	return s, nil
}
func (m *mockEscalationRepo) GetStateByIncident(id uuid.UUID) (*models.EscalationState, error) {
	return nil, &repository.NotFoundError{Resource: "escalation_state", ID: id.String()}
}
func (m *mockEscalationRepo) GetActiveStates() ([]models.EscalationState, error) {
	return m.activeStates, nil
}
func (m *mockEscalationRepo) UpdateState(s *models.EscalationState) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if s.AlertID != nil {
		m.states[*s.AlertID] = s
	}
	return nil
}
func (m *mockEscalationRepo) RecordAcknowledgment(alertID uuid.UUID, by string, via models.AcknowledgmentVia) error {
	if m.ackErr != nil {
		return m.ackErr
	}
	s, ok := m.states[alertID]
	if !ok {
		return nil // idempotent
	}
	now := time.Now()
	s.AcknowledgedAt = &now
	s.AcknowledgedBy = &by
	s.AcknowledgedVia = &via
	s.Status = models.EscalationStateAcknowledged
	return nil
}

func (m *mockEscalationRepo) ListSeverityRules() ([]models.EscalationSeverityRule, error) {
	return nil, nil
}
func (m *mockEscalationRepo) GetSeverityRule(_ string) (*models.EscalationSeverityRule, error) {
	return nil, nil
}
func (m *mockEscalationRepo) UpsertSeverityRule(_ string, _ uuid.UUID) (*models.EscalationSeverityRule, error) {
	return nil, nil
}
func (m *mockEscalationRepo) DeleteSeverityRule(_ string) error { return nil }

var _ repository.EscalationPolicyRepository = &mockEscalationRepo{}

// ── Mock schedule evaluator ───────────────────────────────────────────────────

type mockScheduleEvaluator struct {
	onCallUser string
	err        error
}

func (m *mockScheduleEvaluator) WhoIsOnCall(scheduleID uuid.UUID, at time.Time) (string, error) {
	return m.onCallUser, m.err
}
func (m *mockScheduleEvaluator) GetTimeline(scheduleID uuid.UUID, from, to time.Time) ([]TimelineSegment, error) {
	return nil, nil
}
func (m *mockScheduleEvaluator) GetLayerTimelines(scheduleID uuid.UUID, from, to time.Time) (map[uuid.UUID][]TimelineSegment, []TimelineSegment, error) {
	return nil, nil, nil
}

var _ ScheduleEvaluator = &mockScheduleEvaluator{}

// ── Mock chat service ─────────────────────────────────────────────────────────

type mockChatForEscalation struct {
	sentDMs []escalationDM
}

type escalationDM struct {
	userID  string
	alertID uuid.UUID
	tier    int
}

func (m *mockChatForEscalation) SendEscalationDM(userID string, alert *models.Alert, tierIndex int) error {
	m.sentDMs = append(m.sentDMs, escalationDM{userID: userID, alertID: alert.ID, tier: tierIndex})
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func makePolicy(tiers []models.EscalationTier) *models.EscalationPolicy {
	id := uuid.New()
	p := &models.EscalationPolicy{
		ID:      id,
		Name:    "test-policy",
		Enabled: true,
		Tiers:   tiers,
	}
	return p
}

func makeTier(policyID uuid.UUID, index, timeoutSecs int, targetType models.EscalationTargetType, schedID *uuid.UUID, users []string) models.EscalationTier {
	return models.EscalationTier{
		ID:             uuid.New(),
		PolicyID:       policyID,
		TierIndex:      index,
		TimeoutSeconds: timeoutSecs,
		TargetType:     targetType,
		ScheduleID:     schedID,
		UserNames:      models.JSONBArray(users),
	}
}

func makeTestAlert(policyID uuid.UUID) *models.Alert {
	return &models.Alert{
		ID:                 uuid.New(),
		Title:              "CPU High",
		Severity:           models.AlertSeverityCritical,
		EscalationPolicyID: &policyID,
	}
}

// ── Tests: TriggerEscalation ──────────────────────────────────────────────────

func TestEscalationEngine_TriggerEscalation_CreatesState(t *testing.T) {
	schedID := uuid.New()
	policy := makePolicy([]models.EscalationTier{
		makeTier(uuid.Nil, 0, 300, models.EscalationTargetSchedule, &schedID, nil),
	})
	policy.Tiers[0].PolicyID = policy.ID

	repo := newMockEscalationRepo()
	repo.policies[policy.ID] = policy

	engine := NewEscalationEngine(repo, &mockScheduleEvaluator{onCallUser: "alice"}, nil)
	alert := makeTestAlert(policy.ID)

	if err := engine.TriggerEscalation(alert); err != nil {
		t.Fatalf("TriggerEscalation failed: %v", err)
	}

	state, err := repo.GetStateByAlert(alert.ID)
	if err != nil {
		t.Fatalf("expected state to be created, got error: %v", err)
	}
	if state.PolicyID != policy.ID {
		t.Errorf("state.PolicyID = %v, want %v", state.PolicyID, policy.ID)
	}
	if state.CurrentTierIndex != 0 {
		t.Errorf("state.CurrentTierIndex = %d, want 0", state.CurrentTierIndex)
	}
	if state.Status != models.EscalationStatePending {
		t.Errorf("state.Status = %q, want pending", state.Status)
	}
}

func TestEscalationEngine_TriggerEscalation_NoPolicyID_IsNoop(t *testing.T) {
	repo := newMockEscalationRepo()
	engine := NewEscalationEngine(repo, &mockScheduleEvaluator{}, nil)

	alert := &models.Alert{ID: uuid.New(), EscalationPolicyID: nil}
	if err := engine.TriggerEscalation(alert); err != nil {
		t.Fatalf("expected no error for alert with no policy, got: %v", err)
	}

	_, err := repo.GetStateByAlert(alert.ID)
	if err == nil {
		t.Error("expected no state to be created for alert without policy")
	}
}

func TestEscalationEngine_TriggerEscalation_PolicyNotFound_ReturnsError(t *testing.T) {
	repo := newMockEscalationRepo()
	engine := NewEscalationEngine(repo, &mockScheduleEvaluator{}, nil)

	policyID := uuid.New()
	alert := makeTestAlert(policyID)

	err := engine.TriggerEscalation(alert)
	if err == nil {
		t.Error("expected error when policy not found, got nil")
	}
}

// ── Tests: EvaluateEscalations ────────────────────────────────────────────────

func TestEscalationEngine_EvaluateEscalations_NotifiesTier0WhenPending(t *testing.T) {
	schedID := uuid.New()
	policyID := uuid.New()
	tier0 := makeTier(policyID, 0, 300, models.EscalationTargetSchedule, &schedID, nil)

	policy := &models.EscalationPolicy{
		ID:      policyID,
		Name:    "test",
		Enabled: true,
		Tiers:   []models.EscalationTier{tier0},
	}

	repo := newMockEscalationRepo()
	repo.policies[policyID] = policy

	alertID := uuid.New()
	state := models.EscalationState{
		ID:               uuid.New(),
		AlertID:          &alertID,
		PolicyID:         policyID,
		CurrentTierIndex: 0,
		Status:           models.EscalationStatePending,
		LastNotifiedAt:   nil, // never notified
	}
	repo.states[alertID] = &state
	repo.activeStates = []models.EscalationState{state}

	chat := &mockChatForEscalation{}
	evaluator := &mockScheduleEvaluator{onCallUser: "alice"}

	alert := &models.Alert{ID: alertID, EscalationPolicyID: &policyID}
	engine := NewEscalationEngine(repo, evaluator, chat)
	// Give engine an alert lookup function
	engine.(*escalationEngine).alertLookup = func(id uuid.UUID) (*models.Alert, error) {
		return alert, nil
	}

	if err := engine.EvaluateEscalations(); err != nil {
		t.Fatalf("EvaluateEscalations failed: %v", err)
	}

	if len(chat.sentDMs) != 1 {
		t.Fatalf("expected 1 DM sent, got %d", len(chat.sentDMs))
	}
	if chat.sentDMs[0].userID != "alice" {
		t.Errorf("DM sent to %q, want alice", chat.sentDMs[0].userID)
	}
	if chat.sentDMs[0].tier != 0 {
		t.Errorf("DM tier = %d, want 0", chat.sentDMs[0].tier)
	}

	// State should now be 'notified'
	updated := repo.states[alertID]
	if updated.Status != models.EscalationStateNotified {
		t.Errorf("state.Status = %q after notify, want notified", updated.Status)
	}
	if updated.LastNotifiedAt == nil {
		t.Error("LastNotifiedAt should be set after notification")
	}
}

func TestEscalationEngine_EvaluateEscalations_AdvancesToNextTierOnTimeout(t *testing.T) {
	schedID := uuid.New()
	policyID := uuid.New()

	tier0 := makeTier(policyID, 0, 300, models.EscalationTargetSchedule, &schedID, nil)
	tier1 := makeTier(policyID, 1, 600, models.EscalationTargetUsers, nil, []string{"backup-bob"})

	policy := &models.EscalationPolicy{
		ID:      policyID,
		Enabled: true,
		Tiers:   []models.EscalationTier{tier0, tier1},
	}

	repo := newMockEscalationRepo()
	repo.policies[policyID] = policy

	alertID := uuid.New()
	// Tier 0 was notified 10 minutes ago; timeout is 5 minutes → should advance
	notifiedAt := time.Now().Add(-10 * time.Minute)
	state := models.EscalationState{
		ID:               uuid.New(),
		AlertID:          &alertID,
		PolicyID:         policyID,
		CurrentTierIndex: 0,
		Status:           models.EscalationStateNotified,
		LastNotifiedAt:   &notifiedAt,
	}
	repo.states[alertID] = &state
	repo.activeStates = []models.EscalationState{state}

	chat := &mockChatForEscalation{}
	alert := &models.Alert{ID: alertID, EscalationPolicyID: &policyID}

	engine := NewEscalationEngine(repo, &mockScheduleEvaluator{onCallUser: "alice"}, chat)
	engine.(*escalationEngine).alertLookup = func(id uuid.UUID) (*models.Alert, error) {
		return alert, nil
	}

	if err := engine.EvaluateEscalations(); err != nil {
		t.Fatalf("EvaluateEscalations failed: %v", err)
	}

	// Should have sent DM to backup-bob (tier 1)
	if len(chat.sentDMs) != 1 {
		t.Fatalf("expected 1 DM, got %d", len(chat.sentDMs))
	}
	if chat.sentDMs[0].userID != "backup-bob" {
		t.Errorf("DM to %q, want backup-bob", chat.sentDMs[0].userID)
	}
	if chat.sentDMs[0].tier != 1 {
		t.Errorf("DM tier = %d, want 1", chat.sentDMs[0].tier)
	}

	updated := repo.states[alertID]
	if updated.CurrentTierIndex != 1 {
		t.Errorf("CurrentTierIndex = %d, want 1", updated.CurrentTierIndex)
	}
}

func TestEscalationEngine_EvaluateEscalations_DoesNotAdvanceBeforeTimeout(t *testing.T) {
	schedID := uuid.New()
	policyID := uuid.New()

	tier0 := makeTier(policyID, 0, 300, models.EscalationTargetSchedule, &schedID, nil)
	policy := &models.EscalationPolicy{ID: policyID, Enabled: true, Tiers: []models.EscalationTier{tier0}}

	repo := newMockEscalationRepo()
	repo.policies[policyID] = policy

	alertID := uuid.New()
	// Notified only 1 minute ago; timeout is 5 minutes → must NOT advance
	recentlyNotified := time.Now().Add(-1 * time.Minute)
	state := models.EscalationState{
		ID:               uuid.New(),
		AlertID:          &alertID,
		PolicyID:         policyID,
		CurrentTierIndex: 0,
		Status:           models.EscalationStateNotified,
		LastNotifiedAt:   &recentlyNotified,
	}
	repo.states[alertID] = &state
	repo.activeStates = []models.EscalationState{state}

	chat := &mockChatForEscalation{}
	alert := &models.Alert{ID: alertID, EscalationPolicyID: &policyID}
	engine := NewEscalationEngine(repo, &mockScheduleEvaluator{onCallUser: "alice"}, chat)
	engine.(*escalationEngine).alertLookup = func(id uuid.UUID) (*models.Alert, error) {
		return alert, nil
	}

	if err := engine.EvaluateEscalations(); err != nil {
		t.Fatalf("EvaluateEscalations failed: %v", err)
	}

	if len(chat.sentDMs) != 0 {
		t.Errorf("expected 0 DMs before timeout, got %d", len(chat.sentDMs))
	}
	updated := repo.states[alertID]
	if updated.CurrentTierIndex != 0 {
		t.Errorf("CurrentTierIndex should remain 0, got %d", updated.CurrentTierIndex)
	}
}

func TestEscalationEngine_EvaluateEscalations_LastTierExhausted_MarksCompleted(t *testing.T) {
	policyID := uuid.New()
	tier0 := makeTier(policyID, 0, 300, models.EscalationTargetUsers, nil, []string{"alice"})
	policy := &models.EscalationPolicy{ID: policyID, Enabled: true, Tiers: []models.EscalationTier{tier0}}

	repo := newMockEscalationRepo()
	repo.policies[policyID] = policy

	alertID := uuid.New()
	// Already on tier 0 (the only tier), notified 10 minutes ago → timed out, no more tiers
	notifiedAt := time.Now().Add(-10 * time.Minute)
	state := models.EscalationState{
		ID:               uuid.New(),
		AlertID:          &alertID,
		PolicyID:         policyID,
		CurrentTierIndex: 0,
		Status:           models.EscalationStateNotified,
		LastNotifiedAt:   &notifiedAt,
	}
	repo.states[alertID] = &state
	repo.activeStates = []models.EscalationState{state}

	alert := &models.Alert{ID: alertID, EscalationPolicyID: &policyID}
	engine := NewEscalationEngine(repo, &mockScheduleEvaluator{}, &mockChatForEscalation{})
	engine.(*escalationEngine).alertLookup = func(id uuid.UUID) (*models.Alert, error) {
		return alert, nil
	}

	if err := engine.EvaluateEscalations(); err != nil {
		t.Fatalf("EvaluateEscalations failed: %v", err)
	}

	updated := repo.states[alertID]
	if updated.Status != models.EscalationStateCompleted {
		t.Errorf("state.Status = %q, want completed when last tier exhausted", updated.Status)
	}
}

func TestEscalationEngine_EvaluateEscalations_ScheduleNoOnCall_SkipsTierAdvances(t *testing.T) {
	schedID := uuid.New()
	policyID := uuid.New()

	tier0 := makeTier(policyID, 0, 300, models.EscalationTargetSchedule, &schedID, nil)
	tier1 := makeTier(policyID, 1, 300, models.EscalationTargetUsers, nil, []string{"bob"})
	policy := &models.EscalationPolicy{
		ID:      policyID,
		Enabled: true,
		Tiers:   []models.EscalationTier{tier0, tier1},
	}

	repo := newMockEscalationRepo()
	repo.policies[policyID] = policy

	alertID := uuid.New()
	state := models.EscalationState{
		ID:               uuid.New(),
		AlertID:          &alertID,
		PolicyID:         policyID,
		CurrentTierIndex: 0,
		Status:           models.EscalationStatePending,
		LastNotifiedAt:   nil,
	}
	repo.states[alertID] = &state
	repo.activeStates = []models.EscalationState{state}

	chat := &mockChatForEscalation{}
	// Schedule has nobody on call
	evaluator := &mockScheduleEvaluator{onCallUser: ""}
	alert := &models.Alert{ID: alertID, EscalationPolicyID: &policyID}
	engine := NewEscalationEngine(repo, evaluator, chat)
	engine.(*escalationEngine).alertLookup = func(id uuid.UUID) (*models.Alert, error) {
		return alert, nil
	}

	if err := engine.EvaluateEscalations(); err != nil {
		t.Fatalf("EvaluateEscalations failed: %v", err)
	}

	// Tier 0 has no on-call user → should advance immediately to tier 1 and DM bob
	if len(chat.sentDMs) != 1 {
		t.Fatalf("expected 1 DM (to bob after skipping empty tier 0), got %d", len(chat.sentDMs))
	}
	if chat.sentDMs[0].userID != "bob" {
		t.Errorf("DM to %q, want bob", chat.sentDMs[0].userID)
	}
}

// ── Tests: AcknowledgeAlert ───────────────────────────────────────────────────

func TestEscalationEngine_AcknowledgeAlert_StopsEscalation(t *testing.T) {
	policyID := uuid.New()
	alertID := uuid.New()

	repo := newMockEscalationRepo()
	state := &models.EscalationState{
		ID:       uuid.New(),
		AlertID:  &alertID,
		PolicyID: policyID,
		Status:   models.EscalationStateNotified,
	}
	repo.states[alertID] = state

	engine := NewEscalationEngine(repo, &mockScheduleEvaluator{}, nil)

	if err := engine.AcknowledgeAlert(alertID, "alice", models.AcknowledgmentViaSlack); err != nil {
		t.Fatalf("AcknowledgeAlert failed: %v", err)
	}

	updated := repo.states[alertID]
	if updated.AcknowledgedAt == nil {
		t.Error("AcknowledgedAt should be set after acknowledgment")
	}
	if updated.AcknowledgedBy == nil || *updated.AcknowledgedBy != "alice" {
		t.Errorf("AcknowledgedBy = %v, want alice", updated.AcknowledgedBy)
	}
	if updated.Status != models.EscalationStateAcknowledged {
		t.Errorf("Status = %q, want acknowledged", updated.Status)
	}
}

func TestEscalationEngine_AcknowledgeAlert_Idempotent(t *testing.T) {
	policyID := uuid.New()
	alertID := uuid.New()
	now := time.Now()

	repo := newMockEscalationRepo()
	state := &models.EscalationState{
		ID:             uuid.New(),
		AlertID:        &alertID,
		PolicyID:       policyID,
		Status:         models.EscalationStateAcknowledged,
		AcknowledgedAt: &now,
	}
	repo.states[alertID] = state

	engine := NewEscalationEngine(repo, &mockScheduleEvaluator{}, nil)

	// Second acknowledgment — should not error
	if err := engine.AcknowledgeAlert(alertID, "bob", models.AcknowledgmentViaAPI); err != nil {
		t.Fatalf("second AcknowledgeAlert failed: %v", err)
	}
}

// ── Tests: MarkAlertCompleted ─────────────────────────────────────────────────

func TestEscalationEngine_MarkAlertCompleted_SetsCompletedStatus(t *testing.T) {
	policyID := uuid.New()
	alertID := uuid.New()

	repo := newMockEscalationRepo()
	state := &models.EscalationState{
		ID:       uuid.New(),
		AlertID:  &alertID,
		PolicyID: policyID,
		Status:   models.EscalationStateNotified,
	}
	repo.states[alertID] = state

	engine := NewEscalationEngine(repo, &mockScheduleEvaluator{}, nil)

	if err := engine.MarkAlertCompleted(alertID); err != nil {
		t.Fatalf("MarkAlertCompleted failed: %v", err)
	}

	updated := repo.states[alertID]
	if updated.Status != models.EscalationStateCompleted {
		t.Errorf("Status = %q, want completed", updated.Status)
	}
}

func TestEscalationEngine_MarkAlertCompleted_NoState_IsNoop(t *testing.T) {
	repo := newMockEscalationRepo()
	engine := NewEscalationEngine(repo, &mockScheduleEvaluator{}, nil)

	// Alert with no escalation state — should not error
	if err := engine.MarkAlertCompleted(uuid.New()); err != nil {
		t.Fatalf("MarkAlertCompleted with no state should be a no-op, got: %v", err)
	}
}

// ── Tests: BothTargetType ─────────────────────────────────────────────────────

func TestEscalationEngine_BothTargetType_NotifiesScheduleAndUsers(t *testing.T) {
	schedID := uuid.New()
	policyID := uuid.New()

	tier0 := makeTier(policyID, 0, 300, models.EscalationTargetBoth, &schedID, []string{"extra-carol"})
	policy := &models.EscalationPolicy{ID: policyID, Enabled: true, Tiers: []models.EscalationTier{tier0}}

	repo := newMockEscalationRepo()
	repo.policies[policyID] = policy

	alertID := uuid.New()
	state := models.EscalationState{
		ID: uuid.New(), AlertID: &alertID, PolicyID: policyID,
		Status: models.EscalationStatePending,
	}
	repo.states[alertID] = &state
	repo.activeStates = []models.EscalationState{state}

	chat := &mockChatForEscalation{}
	evaluator := &mockScheduleEvaluator{onCallUser: "alice"}
	alert := &models.Alert{ID: alertID, EscalationPolicyID: &policyID}

	engine := NewEscalationEngine(repo, evaluator, chat)
	engine.(*escalationEngine).alertLookup = func(id uuid.UUID) (*models.Alert, error) {
		return alert, nil
	}

	if err := engine.EvaluateEscalations(); err != nil {
		t.Fatalf("EvaluateEscalations failed: %v", err)
	}

	// Should notify both alice (from schedule) and extra-carol (from user list)
	if len(chat.sentDMs) != 2 {
		t.Fatalf("expected 2 DMs (alice + extra-carol), got %d", len(chat.sentDMs))
	}
	users := map[string]bool{}
	for _, dm := range chat.sentDMs {
		users[dm.userID] = true
	}
	if !users["alice"] {
		t.Error("expected DM to alice (from schedule)")
	}
	if !users["extra-carol"] {
		t.Error("expected DM to extra-carol (from user list)")
	}
}
