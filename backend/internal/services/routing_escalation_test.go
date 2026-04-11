package services

// Tests for OI-127: escalation policy integration into the routing engine
// and alert processing pipeline.

import (
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/models/webhooks"
	"github.com/google/uuid"
)

// ── Routing engine: EscalationPolicyID in RoutingDecision ────────────────────

func TestRoutingEngine_EscalationPolicyID_PopulatedFromActions(t *testing.T) {
	policyID := uuid.New()
	rules := []models.RoutingRule{
		makeRoutingRule(10,
			models.JSONB{"severity": []interface{}{"critical"}},
			models.JSONB{"escalation_policy_id": policyID.String()},
		),
	}
	engine := NewRoutingEngine(&mockRoutingRuleRepo{rules: rules})
	alert := makeAlert("prometheus", "critical", nil)

	decision, err := engine.EvaluateAlert(alert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.EscalationPolicyID == nil {
		t.Fatal("expected EscalationPolicyID to be set, got nil")
	}
	if *decision.EscalationPolicyID != policyID {
		t.Errorf("EscalationPolicyID = %v, want %v", *decision.EscalationPolicyID, policyID)
	}
}

func TestRoutingEngine_EscalationPolicyID_NilWhenAbsent(t *testing.T) {
	rules := []models.RoutingRule{
		makeRoutingRule(10, models.JSONB{}, models.JSONB{"suppress": false}),
	}
	engine := NewRoutingEngine(&mockRoutingRuleRepo{rules: rules})
	alert := makeAlert("prometheus", "warning", nil)

	decision, err := engine.EvaluateAlert(alert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.EscalationPolicyID != nil {
		t.Errorf("EscalationPolicyID should be nil when not in actions, got %v", decision.EscalationPolicyID)
	}
}

func TestRoutingEngine_EscalationPolicyID_NilWhenInvalidUUID(t *testing.T) {
	rules := []models.RoutingRule{
		makeRoutingRule(10, models.JSONB{},
			models.JSONB{"escalation_policy_id": "not-a-valid-uuid"},
		),
	}
	engine := NewRoutingEngine(&mockRoutingRuleRepo{rules: rules})
	alert := makeAlert("prometheus", "critical", nil)

	decision, err := engine.EvaluateAlert(alert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Invalid UUID → treated as absent; should not panic or set bad value
	if decision.EscalationPolicyID != nil {
		t.Errorf("expected nil EscalationPolicyID for invalid UUID string, got %v", decision.EscalationPolicyID)
	}
}

func TestRoutingEngine_DefaultDecision_NoEscalationPolicyID(t *testing.T) {
	// No rules → default decision → EscalationPolicyID is nil
	engine := NewRoutingEngine(&mockRoutingRuleRepo{rules: nil})
	alert := makeAlert("prometheus", "critical", nil)

	decision, err := engine.EvaluateAlert(alert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.EscalationPolicyID != nil {
		t.Errorf("default decision should have nil EscalationPolicyID, got %v", decision.EscalationPolicyID)
	}
}

// ── Alert service pipeline: escalation triggered after routing ────────────────

// mockEscalationEngine records TriggerEscalation calls for assertion.
type mockEscalationEngine struct {
	triggered  []uuid.UUID // alert IDs that had TriggerEscalation called
	triggerErr error
}

func (m *mockEscalationEngine) TriggerIncidentEscalation(_ uuid.UUID, _ uuid.UUID) error { return nil }
func (m *mockEscalationEngine) TriggerEscalation(alert *models.Alert) error {
	if m.triggerErr != nil {
		return m.triggerErr
	}
	m.triggered = append(m.triggered, alert.ID)
	return nil
}
func (m *mockEscalationEngine) EvaluateEscalations() error { return nil }
func (m *mockEscalationEngine) AcknowledgeAlert(id uuid.UUID, by string, via models.AcknowledgmentVia) error {
	return nil
}
func (m *mockEscalationEngine) MarkAlertCompleted(id uuid.UUID) error { return nil }

var _ EscalationEngine = &mockEscalationEngine{}

// buildAlertServiceWithEscalation wires an AlertService with mock routing and escalation.
func buildAlertServiceWithEscalation(
	routingRules []models.RoutingRule,
	escalationEngine *mockEscalationEngine,
) AlertService {
	alertRepo := &mockAlertRepository{}
	incidentSvc := &mockIncidentService{shouldCreate: true}
	svc := NewAlertService(alertRepo, incidentSvc)
	svc.SetRoutingEngine(NewRoutingEngine(&mockRoutingRuleRepo{rules: routingRules}))
	svc.SetEscalationEngine(escalationEngine)
	return svc
}

func TestAlertService_EscalationTriggered_WhenPolicyInRoutingDecision(t *testing.T) {
	policyID := uuid.New()
	rules := []models.RoutingRule{
		makeRoutingRule(10,
			models.JSONB{"severity": []interface{}{"critical"}},
			models.JSONB{"escalation_policy_id": policyID.String()},
		),
	}

	escalation := &mockEscalationEngine{}
	svc := buildAlertServiceWithEscalation(rules, escalation)

	normalized := makeNormalizedAlert("prometheus", "critical")
	_, err := svc.ProcessNormalizedAlerts([]webhooks.NormalizedAlert{normalized})
	if err != nil {
		t.Fatalf("ProcessNormalizedAlerts failed: %v", err)
	}

	if len(escalation.triggered) != 1 {
		t.Fatalf("expected 1 TriggerEscalation call, got %d", len(escalation.triggered))
	}
}

func TestAlertService_EscalationNotTriggered_WhenSuppressed(t *testing.T) {
	rules := []models.RoutingRule{
		makeRoutingRule(10,
			models.JSONB{},
			models.JSONB{"suppress": true, "escalation_policy_id": uuid.New().String()},
		),
	}

	escalation := &mockEscalationEngine{}
	svc := buildAlertServiceWithEscalation(rules, escalation)

	normalized := makeNormalizedAlert("prometheus", "critical")
	_, err := svc.ProcessNormalizedAlerts([]webhooks.NormalizedAlert{normalized})
	if err != nil {
		t.Fatalf("ProcessNormalizedAlerts failed: %v", err)
	}

	if len(escalation.triggered) != 0 {
		t.Errorf("escalation must not trigger when alert is suppressed, got %d calls", len(escalation.triggered))
	}
}

func TestAlertService_EscalationNotTriggered_WhenNoPolicyInDecision(t *testing.T) {
	rules := []models.RoutingRule{
		makeRoutingRule(10, models.JSONB{}, models.JSONB{"create_incident": true}),
	}

	escalation := &mockEscalationEngine{}
	svc := buildAlertServiceWithEscalation(rules, escalation)

	normalized := makeNormalizedAlert("prometheus", "critical")
	_, err := svc.ProcessNormalizedAlerts([]webhooks.NormalizedAlert{normalized})
	if err != nil {
		t.Fatalf("ProcessNormalizedAlerts failed: %v", err)
	}

	if len(escalation.triggered) != 0 {
		t.Errorf("escalation must not trigger when no policy in routing decision, got %d calls", len(escalation.triggered))
	}
}

func TestAlertService_EscalationNotTriggered_WhenAlertUpdated(t *testing.T) {
	// Duplicate alert (update path) — escalation should only fire for new alerts
	policyID := uuid.New()
	rules := []models.RoutingRule{
		makeRoutingRule(10, models.JSONB{},
			models.JSONB{"escalation_policy_id": policyID.String()},
		),
	}

	alertRepo := &mockAlertRepository{existingAlert: &models.Alert{
		ID:         uuid.New(),
		ExternalID: "ext-001",
		Source:     "prometheus",
	}}
	incidentSvc := &mockIncidentService{shouldCreate: true}
	escalation := &mockEscalationEngine{}

	svc := NewAlertService(alertRepo, incidentSvc)
	svc.SetRoutingEngine(NewRoutingEngine(&mockRoutingRuleRepo{rules: rules}))
	svc.SetEscalationEngine(escalation)

	normalized := makeNormalizedAlert("prometheus", "critical")
	normalized.ExternalID = "ext-001" // matches existing alert → update path
	_, err := svc.ProcessNormalizedAlerts([]webhooks.NormalizedAlert{normalized})
	if err != nil {
		t.Fatalf("ProcessNormalizedAlerts failed: %v", err)
	}

	if len(escalation.triggered) != 0 {
		t.Errorf("escalation must not trigger on alert update, got %d calls", len(escalation.triggered))
	}
}

func TestAlertService_EscalationEngineNil_DoesNotPanic(t *testing.T) {
	// When no escalation engine is set, pipeline should proceed without panicking
	policyID := uuid.New()
	rules := []models.RoutingRule{
		makeRoutingRule(10, models.JSONB{},
			models.JSONB{"escalation_policy_id": policyID.String()},
		),
	}

	alertRepo := &mockAlertRepository{}
	incidentSvc := &mockIncidentService{shouldCreate: true}
	svc := NewAlertService(alertRepo, incidentSvc)
	svc.SetRoutingEngine(NewRoutingEngine(&mockRoutingRuleRepo{rules: rules}))
	// Intentionally do NOT set escalation engine

	normalized := makeNormalizedAlert("prometheus", "critical")
	if _, err := svc.ProcessNormalizedAlerts([]webhooks.NormalizedAlert{normalized}); err != nil {
		t.Fatalf("should not error when escalation engine is nil: %v", err)
	}
}
