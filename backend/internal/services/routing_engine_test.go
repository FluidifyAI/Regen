package services

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/repository"
)

// mockRoutingRuleRepo is a test double for RoutingRuleRepository
type mockRoutingRuleRepo struct {
	rules []models.RoutingRule
	err   error
}

func (m *mockRoutingRuleRepo) Create(rule *models.RoutingRule) error          { return nil }
func (m *mockRoutingRuleRepo) GetByID(id uuid.UUID) (*models.RoutingRule, error) { return nil, nil }
func (m *mockRoutingRuleRepo) GetAll() ([]models.RoutingRule, error)           { return m.rules, m.err }
func (m *mockRoutingRuleRepo) Update(rule *models.RoutingRule) error           { return nil }
func (m *mockRoutingRuleRepo) Delete(id uuid.UUID) error                       { return nil }
func (m *mockRoutingRuleRepo) CheckPriorityConflict(priority int, excludeID uuid.UUID) (*models.RoutingRule, error) {
	return nil, nil
}
func (m *mockRoutingRuleRepo) GetEnabled() ([]models.RoutingRule, error) {
	return m.rules, m.err
}

var _ repository.RoutingRuleRepository = &mockRoutingRuleRepo{}

// helpers

func makeRoutingRule(priority int, criteria, actions models.JSONB) models.RoutingRule {
	return models.RoutingRule{
		ID:            uuid.New(),
		Name:          "test-rule",
		Enabled:       true,
		Priority:      priority,
		MatchCriteria: criteria,
		Actions:       actions,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

func makeAlert(source, severity string, labels models.JSONB) *models.Alert {
	return &models.Alert{
		ID:       uuid.New(),
		Source:   source,
		Severity: models.AlertSeverity(severity),
		Labels:   labels,
	}
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestRoutingEngine_NoRules_DefaultDecision(t *testing.T) {
	engine := NewRoutingEngine(&mockRoutingRuleRepo{rules: nil})
	alert := makeAlert("prometheus", "critical", nil)

	decision, err := engine.EvaluateAlert(alert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decision.Suppress {
		t.Error("expected no suppress with no rules")
	}
	if decision.RuleName != "" {
		t.Errorf("expected empty RuleName, got %q", decision.RuleName)
	}
}

func TestRoutingEngine_SuppressRule_Matches(t *testing.T) {
	rules := []models.RoutingRule{
		makeRoutingRule(10,
			models.JSONB{"severity": []interface{}{"info"}},
			models.JSONB{"suppress": true},
		),
	}
	engine := NewRoutingEngine(&mockRoutingRuleRepo{rules: rules})
	alert := makeAlert("prometheus", "info", nil)

	decision, err := engine.EvaluateAlert(alert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !decision.Suppress {
		t.Error("expected suppress=true for info alert with suppress rule")
	}
	if decision.RuleName != "test-rule" {
		t.Errorf("expected RuleName=test-rule, got %q", decision.RuleName)
	}
}

func TestRoutingEngine_SuppressRule_DoesNotMatchOtherSeverity(t *testing.T) {
	rules := []models.RoutingRule{
		makeRoutingRule(10,
			models.JSONB{"severity": []interface{}{"info"}},
			models.JSONB{"suppress": true},
		),
	}
	engine := NewRoutingEngine(&mockRoutingRuleRepo{rules: rules})
	alert := makeAlert("prometheus", "critical", nil)

	decision, err := engine.EvaluateAlert(alert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decision.Suppress {
		t.Error("critical alert should not be suppressed by info-only rule")
	}
}

func TestRoutingEngine_SeverityOverride(t *testing.T) {
	rules := []models.RoutingRule{
		makeRoutingRule(10,
			models.JSONB{"source": []interface{}{"cloudwatch"}},
			models.JSONB{"severity_override": "critical"},
		),
	}
	engine := NewRoutingEngine(&mockRoutingRuleRepo{rules: rules})
	alert := makeAlert("cloudwatch", "warning", nil)

	decision, err := engine.EvaluateAlert(alert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decision.SeverityOverride != "critical" {
		t.Errorf("expected SeverityOverride=critical, got %q", decision.SeverityOverride)
	}
}

func TestRoutingEngine_ChannelOverride(t *testing.T) {
	rules := []models.RoutingRule{
		makeRoutingRule(10,
			models.JSONB{"labels": map[string]interface{}{"team": "db"}},
			models.JSONB{"channel_override": "db-oncall"},
		),
	}
	engine := NewRoutingEngine(&mockRoutingRuleRepo{rules: rules})
	alert := makeAlert("prometheus", "critical", models.JSONB{"team": "db"})

	decision, err := engine.EvaluateAlert(alert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decision.ChannelOverride != "db-oncall" {
		t.Errorf("expected ChannelOverride=db-oncall, got %q", decision.ChannelOverride)
	}
}

func TestRoutingEngine_LabelWildcard(t *testing.T) {
	rules := []models.RoutingRule{
		makeRoutingRule(10,
			models.JSONB{"labels": map[string]interface{}{"env": "*"}},
			models.JSONB{"suppress": true},
		),
	}
	engine := NewRoutingEngine(&mockRoutingRuleRepo{rules: rules})

	// Alert with env label — should match wildcard
	alertWithEnv := makeAlert("prometheus", "info", models.JSONB{"env": "prod"})
	decision, err := engine.EvaluateAlert(alertWithEnv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.Suppress {
		t.Error("alert with env label should match wildcard and be suppressed")
	}

	// Alert without env label — should not match
	alertWithoutEnv := makeAlert("prometheus", "info", models.JSONB{})
	decision2, err := engine.EvaluateAlert(alertWithoutEnv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision2.Suppress {
		t.Error("alert without env label should not match env=* rule")
	}
}

func TestRoutingEngine_LabelExactMatch(t *testing.T) {
	rules := []models.RoutingRule{
		makeRoutingRule(10,
			models.JSONB{"labels": map[string]interface{}{"env": "prod"}},
			models.JSONB{"suppress": true},
		),
	}
	engine := NewRoutingEngine(&mockRoutingRuleRepo{rules: rules})

	alertProd := makeAlert("prometheus", "info", models.JSONB{"env": "prod"})
	d1, _ := engine.EvaluateAlert(alertProd)
	if !d1.Suppress {
		t.Error("env=prod alert should match exact rule")
	}

	alertStaging := makeAlert("prometheus", "info", models.JSONB{"env": "staging"})
	d2, _ := engine.EvaluateAlert(alertStaging)
	if d2.Suppress {
		t.Error("env=staging alert should not match env=prod rule")
	}
}

func TestRoutingEngine_PriorityOrder_FirstMatchWins(t *testing.T) {
	rules := []models.RoutingRule{
		// Priority 1: suppress all
		makeRoutingRule(1, models.JSONB{}, models.JSONB{"suppress": true}),
		// Priority 100: create incident
		makeRoutingRule(100, models.JSONB{}, models.JSONB{"create_incident": true}),
	}
	rules[0].Name = "suppress-all"
	rules[1].Name = "create-incident"

	engine := NewRoutingEngine(&mockRoutingRuleRepo{rules: rules})
	alert := makeAlert("prometheus", "critical", nil)

	decision, err := engine.EvaluateAlert(alert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !decision.Suppress {
		t.Error("priority 1 suppress rule should win over priority 100 rule")
	}
	if decision.RuleName != "suppress-all" {
		t.Errorf("expected RuleName=suppress-all, got %q", decision.RuleName)
	}
}

func TestRoutingEngine_EmptyCriteria_MatchesAll(t *testing.T) {
	rules := []models.RoutingRule{
		makeRoutingRule(10, models.JSONB{}, models.JSONB{"suppress": true}),
	}
	engine := NewRoutingEngine(&mockRoutingRuleRepo{rules: rules})

	// Empty criteria should match any alert regardless of source/severity/labels
	for _, src := range []string{"prometheus", "grafana", "cloudwatch"} {
		alert := makeAlert(src, "critical", models.JSONB{"some": "label"})
		decision, err := engine.EvaluateAlert(alert)
		if err != nil {
			t.Fatalf("unexpected error for source %s: %v", src, err)
		}
		if !decision.Suppress {
			t.Errorf("empty criteria rule should match source=%s", src)
		}
	}
}

func TestRoutingEngine_MultiCriteria_AllMustMatch(t *testing.T) {
	rules := []models.RoutingRule{
		makeRoutingRule(10,
			models.JSONB{
				"source":   []interface{}{"prometheus"},
				"severity": []interface{}{"critical"},
			},
			models.JSONB{"suppress": true},
		),
	}
	engine := NewRoutingEngine(&mockRoutingRuleRepo{rules: rules})

	// Both criteria match
	alertBoth := makeAlert("prometheus", "critical", nil)
	d1, _ := engine.EvaluateAlert(alertBoth)
	if !d1.Suppress {
		t.Error("alert matching both source and severity should be suppressed")
	}

	// Only source matches
	alertSourceOnly := makeAlert("prometheus", "warning", nil)
	d2, _ := engine.EvaluateAlert(alertSourceOnly)
	if d2.Suppress {
		t.Error("alert matching only source should not be suppressed (AND semantics)")
	}

	// Only severity matches
	alertSeverityOnly := makeAlert("grafana", "critical", nil)
	d3, _ := engine.EvaluateAlert(alertSeverityOnly)
	if d3.Suppress {
		t.Error("alert matching only severity should not be suppressed (AND semantics)")
	}
}

func TestRoutingEngine_RepoError_ReturnsError(t *testing.T) {
	repo := &mockRoutingRuleRepo{err: errors.New("db connection lost")}
	engine := NewRoutingEngine(repo)

	// Force a cache miss by using a fresh engine with zero TTL
	e := engine.(*routingEngine)
	e.rulesCacheExpiry = time.Time{} // expired immediately

	alert := makeAlert("prometheus", "critical", nil)
	_, err := engine.EvaluateAlert(alert)
	if err == nil {
		t.Error("expected error when repo fails, got nil")
	}
}

func TestRoutingEngine_RefreshRules_UpdatesCache(t *testing.T) {
	repo := &mockRoutingRuleRepo{rules: nil}
	engine := NewRoutingEngine(repo)

	// Initially no rules — no suppress
	alert := makeAlert("prometheus", "info", nil)
	d1, _ := engine.EvaluateAlert(alert)
	if d1.Suppress {
		t.Error("should not suppress with no rules")
	}

	// Add a suppress rule and force refresh
	repo.rules = []models.RoutingRule{
		makeRoutingRule(10, models.JSONB{}, models.JSONB{"suppress": true}),
	}
	if err := engine.RefreshRules(); err != nil {
		t.Fatalf("RefreshRules failed: %v", err)
	}

	d2, _ := engine.EvaluateAlert(alert)
	if !d2.Suppress {
		t.Error("should suppress after rules refreshed")
	}
}

func TestExtractAIEnabled_DefaultsTrueWhenAbsent(t *testing.T) {
	// Rule with no ai_enabled key in actions — should default to true
	rules := []models.RoutingRule{
		makeRoutingRule(10,
			models.JSONB{"source": []interface{}{"prometheus"}},
			models.JSONB{"severity_override": "critical"},
		),
	}
	engine := NewRoutingEngine(&mockRoutingRuleRepo{rules: rules})
	alert := makeAlert("prometheus", "warning", nil)

	decision, err := engine.EvaluateAlert(alert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !decision.AIEnabled {
		t.Error("expected AIEnabled=true when ai_enabled is absent from rule actions")
	}
}

func TestExtractAIEnabled_FalseWhenExplicitlySet(t *testing.T) {
	// Rule with ai_enabled: false — should propagate false
	rules := []models.RoutingRule{
		makeRoutingRule(10,
			models.JSONB{"source": []interface{}{"prometheus"}},
			models.JSONB{"ai_enabled": false},
		),
	}
	engine := NewRoutingEngine(&mockRoutingRuleRepo{rules: rules})
	alert := makeAlert("prometheus", "critical", nil)

	decision, err := engine.EvaluateAlert(alert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decision.AIEnabled {
		t.Error("expected AIEnabled=false when ai_enabled is explicitly set to false in rule actions")
	}
}

func TestRoutingDecision_NoMatch_AIEnabledTrue(t *testing.T) {
	// No rules at all — no match means default decision with AIEnabled=true
	engine := NewRoutingEngine(&mockRoutingRuleRepo{rules: nil})
	alert := makeAlert("prometheus", "critical", nil)

	decision, err := engine.EvaluateAlert(alert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !decision.AIEnabled {
		t.Error("expected AIEnabled=true when no rules match")
	}
}
