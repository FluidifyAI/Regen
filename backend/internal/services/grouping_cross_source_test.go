package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/fluidify/regen/internal/models"
	"github.com/stretchr/testify/assert"
)

// Test cross-source alert correlation by verifying group key generation
func TestCrossSourceGroupKeyDerivation(t *testing.T) {
	// Create a grouping engine instance (without using real DB)
	engine := &groupingEngine{}

	// Test scenario: Rule with cross_source_labels should use those labels for grouping
	rule := &models.GroupingRule{
		ID:       uuid.New(),
		Name:     "Cross-source: Group by service and env",
		Priority: 50,
		// Match labels determines which alerts this rule applies to
		MatchLabels: models.JSONB{
			"severity": "critical",
		},
		// Cross-source labels determine the group key
		CrossSourceLabels: models.JSONBArray{"service", "env"},
		TimeWindowSeconds: 300,
	}

	// Alert 1: Prometheus with HighCPU
	prometheusLabels := map[string]string{
		"alertname": "HighCPU",
		"service":   "api",
		"env":       "production",
		"severity":  "critical",
		"instance":  "web-01",
	}

	// Alert 2: Grafana with HighLatency (different alertname)
	grafanaLabels := map[string]string{
		"alertname": "HighLatency",
		"service":   "api",
		"env":       "production",
		"severity":  "critical",
		"dashboard": "api-metrics",
	}

	// Alert 3: CloudWatch with HighErrorRate (different alertname)
	cloudwatchLabels := map[string]string{
		"alertname": "HighErrorRate",
		"service":   "api",
		"env":       "production",
		"severity":  "critical",
		"region":    "us-east-1",
	}

	// Derive group keys
	key1 := engine.deriveGroupKey(prometheusLabels, rule)
	key2 := engine.deriveGroupKey(grafanaLabels, rule)
	key3 := engine.deriveGroupKey(cloudwatchLabels, rule)

	// All three alerts should have THE SAME group key
	// because they have the same service=api, env=production
	// (even though alertnames are different)
	assert.Equal(t, key1, key2, "Prometheus and Grafana alerts should have same group key")
	assert.Equal(t, key1, key3, "Prometheus and CloudWatch alerts should have same group key")
	assert.NotEmpty(t, key1, "Group key should not be empty")

	t.Logf("✅ Cross-source correlation: All 3 alerts from different sources have same group key: %s", key1)
}

func TestCrossSourceGroupKeyDerivation_DifferentEnv(t *testing.T) {
	engine := &groupingEngine{}

	rule := &models.GroupingRule{
		ID:                uuid.New(),
		Name:              "Cross-source: Group by service and env",
		Priority:          50,
		MatchLabels:       models.JSONB{"severity": "critical"},
		CrossSourceLabels: models.JSONBArray{"service", "env"},
		TimeWindowSeconds: 300,
	}

	// Alert 1: production environment
	prodLabels := map[string]string{
		"alertname": "HighCPU",
		"service":   "api",
		"env":       "production",
		"severity":  "critical",
	}

	// Alert 2: staging environment (different env)
	stagingLabels := map[string]string{
		"alertname": "HighCPU",
		"service":   "api",
		"env":       "staging",
		"severity":  "critical",
	}

	key1 := engine.deriveGroupKey(prodLabels, rule)
	key2 := engine.deriveGroupKey(stagingLabels, rule)

	// Different env should create different group keys
	assert.NotEqual(t, key1, key2, "Different env should create different group keys")

	t.Logf("✅ Alerts with different env values have different group keys")
}

func TestFallbackToMatchLabels_WhenNoCrossSourceLabels(t *testing.T) {
	engine := &groupingEngine{}

	// Rule WITHOUT cross_source_labels (should use match_labels for grouping)
	rule := &models.GroupingRule{
		ID:                uuid.New(),
		Name:              "Default: Group by alertname",
		Priority:          100,
		MatchLabels:       models.JSONB{"alertname": "*"},
		CrossSourceLabels: nil, // No cross-source labels
		TimeWindowSeconds: 300,
	}

	// Two alerts from different sources with SAME alertname
	alert1 := map[string]string{
		"alertname": "HighCPU",
		"service":   "api",
		"instance":  "web-01",
	}

	alert2 := map[string]string{
		"alertname": "HighCPU",
		"service":   "web", // Different service
		"instance":  "app-01",
	}

	key1 := engine.deriveGroupKey(alert1, rule)
	key2 := engine.deriveGroupKey(alert2, rule)

	// Group keys should be THE SAME (same alertname, cross_source_labels not used)
	assert.Equal(t, key1, key2, "Same alertname should create same group key when cross_source_labels is empty")

	t.Logf("✅ Fallback to match_labels works when cross_source_labels is not specified")
}

func TestEmptyCrossSourceLabels_UsesMatchLabels(t *testing.T) {
	engine := &groupingEngine{}

	// Rule with EMPTY cross_source_labels array (should use match_labels)
	rule := &models.GroupingRule{
		ID:                uuid.New(),
		Name:              "Group by service",
		Priority:          50,
		MatchLabels:       models.JSONB{"service": "*"},
		CrossSourceLabels: models.JSONBArray{}, // Empty array
		TimeWindowSeconds: 300,
	}

	alert1 := map[string]string{
		"alertname": "HighCPU",
		"service":   "api",
	}

	alert2 := map[string]string{
		"alertname": "HighMemory",
		"service":   "api",
	}

	key1 := engine.deriveGroupKey(alert1, rule)
	key2 := engine.deriveGroupKey(alert2, rule)

	// Should group by service (from match_labels)
	assert.Equal(t, key1, key2, "Empty cross_source_labels should fall back to match_labels")

	t.Logf("✅ Empty cross_source_labels falls back to match_labels")
}

func TestCrossSourceMultipleLabels(t *testing.T) {
	engine := &groupingEngine{}

	// Rule with multiple cross_source_labels
	rule := &models.GroupingRule{
		ID:                uuid.New(),
		Name:              "Cross-source: Group by service, env, and region",
		Priority:          50,
		MatchLabels:       models.JSONB{"severity": "*"},
		CrossSourceLabels: models.JSONBArray{"service", "env", "region"},
		TimeWindowSeconds: 300,
	}

	alert1 := map[string]string{
		"alertname": "Alert1",
		"service":   "api",
		"env":       "prod",
		"region":    "us-east-1",
		"severity":  "critical",
	}

	alert2 := map[string]string{
		"alertname": "Alert2",
		"service":   "api",
		"env":       "prod",
		"region":    "us-east-1",
		"severity":  "warning",
	}

	alert3 := map[string]string{
		"alertname": "Alert3",
		"service":   "api",
		"env":       "prod",
		"region":    "us-west-2", // Different region
		"severity":  "critical",
	}

	key1 := engine.deriveGroupKey(alert1, rule)
	key2 := engine.deriveGroupKey(alert2, rule)
	key3 := engine.deriveGroupKey(alert3, rule)

	// alert1 and alert2 should have same key (same service+env+region)
	assert.Equal(t, key1, key2, "Alerts with same service+env+region should have same key")

	// alert3 should have different key (different region)
	assert.NotEqual(t, key1, key3, "Alerts with different region should have different key")

	t.Logf("✅ Multiple cross_source_labels work correctly")
}
