package services

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
)

// RoutingDecision represents the result of evaluating an alert against routing rules
type RoutingDecision struct {
	// Suppress means: store the alert but do not create an incident
	Suppress bool

	// SeverityOverride replaces the alert's severity for incident creation (empty = no override)
	SeverityOverride string

	// ChannelOverride replaces the auto-generated Slack channel name suffix (empty = no override)
	ChannelOverride string

	// RuleName is the name of the rule that matched (empty if no rule matched)
	RuleName string

	// EscalationPolicyID is the escalation policy to trigger for this alert (nil = no escalation).
	// Set from the "escalation_policy_id" key in the matching rule's actions JSONB.
	EscalationPolicyID *uuid.UUID

	// AIEnabled controls whether AI agents process the resulting incident.
	// Defaults to true; can be set false via the "ai_enabled" key in the rule's actions JSONB.
	AIEnabled bool
}

// RoutingEngine evaluates alerts against routing rules to determine incident routing behavior
type RoutingEngine interface {
	// EvaluateAlert returns the routing decision for a given alert
	EvaluateAlert(alert *models.Alert) (*RoutingDecision, error)

	// RefreshRules reloads routing rules from the database
	RefreshRules() error
}

// routingEngine implements RoutingEngine
type routingEngine struct {
	ruleRepo repository.RoutingRuleRepository

	// Rule cache to avoid database hits on every alert
	rulesCache       []models.RoutingRule
	rulesCacheMutex  sync.RWMutex
	rulesCacheExpiry time.Time
	ruleCacheTTL     time.Duration
}

// NewRoutingEngine creates a new routing engine
func NewRoutingEngine(ruleRepo repository.RoutingRuleRepository) RoutingEngine {
	engine := &routingEngine{
		ruleRepo:     ruleRepo,
		ruleCacheTTL: 5 * time.Second,
	}

	// Load rules on initialization
	_ = engine.RefreshRules()

	return engine
}

// EvaluateAlert returns the routing decision for an alert
func (r *routingEngine) EvaluateAlert(alert *models.Alert) (*RoutingDecision, error) {
	if err := r.ensureRulesCached(); err != nil {
		return nil, fmt.Errorf("failed to load routing rules: %w", err)
	}

	r.rulesCacheMutex.RLock()
	rules := r.rulesCache // Copy slice header under lock; safe — GC keeps backing array alive
	r.rulesCacheMutex.RUnlock()

	// Flatten alert labels to map[string]string for matching
	alertLabels := make(map[string]string)
	for k, v := range alert.Labels {
		if strVal, ok := v.(string); ok {
			alertLabels[k] = strVal
		}
	}

	slog.Info("routing engine evaluating alert",
		"alert_id", alert.ID,
		"alert_source", alert.Source,
		"alert_severity", alert.Severity,
		"rules_count", len(rules),
	)

	// Evaluate rules in priority order — first match wins
	for _, rule := range rules {
		if r.matchesRule(alert, alertLabels, &rule) {
			decision := r.buildDecision(&rule)

			slog.Info("routing rule matched",
				"alert_id", alert.ID,
				"rule_name", rule.Name,
				"rule_id", rule.ID,
				"suppress", decision.Suppress,
				"severity_override", decision.SeverityOverride,
				"channel_override", decision.ChannelOverride,
			)

			return decision, nil
		}
	}

	// No rule matched — default behavior: create incident, no overrides
	slog.Info("routing: no rule matched, using default behavior (create incident)",
		"alert_id", alert.ID,
	)
	return &RoutingDecision{AIEnabled: true}, nil
}

// RefreshRules reloads routing rules from the database
func (r *routingEngine) RefreshRules() error {
	rules, err := r.ruleRepo.GetEnabled()
	if err != nil {
		return fmt.Errorf("failed to load enabled routing rules: %w", err)
	}

	r.rulesCacheMutex.Lock()
	r.rulesCache = rules
	r.rulesCacheExpiry = time.Now().Add(r.ruleCacheTTL)
	r.rulesCacheMutex.Unlock()

	slog.Info("routing engine rules refreshed", "rules_count", len(rules))

	return nil
}

// ensureRulesCached refreshes the rules cache if expired.
//
// Uses double-checked locking to avoid a thundering herd: multiple goroutines
// hitting an expired cache would all pass the RLock check and pile into
// RefreshRules simultaneously. The write-lock re-check ensures only the first
// goroutine to acquire the write lock actually refreshes; the rest see a
// fresh cache and return immediately.
func (r *routingEngine) ensureRulesCached() error {
	r.rulesCacheMutex.RLock()
	expired := time.Now().After(r.rulesCacheExpiry)
	r.rulesCacheMutex.RUnlock()

	if !expired {
		return nil
	}

	// Acquire write lock and re-check — another goroutine may have refreshed
	// between our RLock check above and this point.
	r.rulesCacheMutex.Lock()
	defer r.rulesCacheMutex.Unlock()

	if !time.Now().After(r.rulesCacheExpiry) {
		return nil // Another goroutine already refreshed
	}

	rules, err := r.ruleRepo.GetEnabled()
	if err != nil {
		return fmt.Errorf("failed to load enabled routing rules: %w", err)
	}

	r.rulesCache = rules
	r.rulesCacheExpiry = time.Now().Add(r.ruleCacheTTL)

	slog.Info("routing engine rules refreshed (cache expired)", "rules_count", len(rules))

	return nil
}

// matchesRule checks if an alert matches a routing rule's match_criteria.
//
// match_criteria schema:
//
//	{
//	  "source":   ["prometheus", "grafana"],  // alert.Source must be in list (empty = all)
//	  "severity": ["critical", "warning"],    // alert.Severity must be in list (empty = all)
//	  "labels":   {"env": "prod", "svc": "*"} // label matching; * = any value
//	}
//
// All specified criteria must match (AND semantics).
func (r *routingEngine) matchesRule(alert *models.Alert, alertLabels map[string]string, rule *models.RoutingRule) bool {
	criteria := rule.MatchCriteria

	// Match source list (if specified)
	if sources, ok := criteria["source"]; ok {
		if sourceList, ok := toStringSlice(sources); ok && len(sourceList) > 0 {
			if !containsString(sourceList, string(alert.Source)) {
				return false
			}
		}
	}

	// Match severity list (if specified)
	if severities, ok := criteria["severity"]; ok {
		if severityList, ok := toStringSlice(severities); ok && len(severityList) > 0 {
			if !containsString(severityList, strings.ToLower(string(alert.Severity))) {
				return false
			}
		}
	}

	// Match labels (if specified) — same wildcard logic as grouping engine
	if labelsVal, ok := criteria["labels"]; ok {
		if labelMap, ok := labelsVal.(map[string]interface{}); ok {
			for key, matchVal := range labelMap {
				matchStr, ok := matchVal.(string)
				if !ok {
					continue
				}

				alertVal, exists := alertLabels[key]

				// Wildcard: label must exist, any value accepted
				if matchStr == "*" {
					if !exists {
						return false
					}
					continue
				}

				// Exact match required
				if !exists || alertVal != matchStr {
					return false
				}
			}
		}
	}

	return true
}

// buildDecision extracts the routing decision from a rule's actions JSONB
func (r *routingEngine) buildDecision(rule *models.RoutingRule) *RoutingDecision {
	decision := &RoutingDecision{RuleName: rule.Name}
	actions := rule.Actions

	if suppress, ok := actions["suppress"]; ok {
		if b, ok := suppress.(bool); ok {
			decision.Suppress = b
		}
	}

	if override, ok := actions["severity_override"]; ok {
		if s, ok := override.(string); ok {
			decision.SeverityOverride = s
		}
	}

	if override, ok := actions["channel_override"]; ok {
		if s, ok := override.(string); ok {
			decision.ChannelOverride = s
		}
	}

	if policyIDVal, ok := actions["escalation_policy_id"]; ok {
		if s, ok := policyIDVal.(string); ok {
			if id, err := uuid.Parse(s); err == nil {
				decision.EscalationPolicyID = &id
			} else {
				slog.Warn("routing rule has invalid escalation_policy_id; ignoring",
					"value", s,
					"rule", rule.Name,
				)
			}
		}
	}

	decision.AIEnabled = extractAIEnabled(actions)

	return decision
}

// extractAIEnabled reads the ai_enabled key from routing rule actions.
// Returns true if the key is absent (opt-out requires explicit false).
func extractAIEnabled(actions models.JSONB) bool {
	if v, ok := actions["ai_enabled"]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return true
}

// toStringSlice converts an interface{} (from JSONB) to []string.
// Handles both []interface{} (from JSON arrays) and []string.
//
// Returns (nil, false) if the value is not a recognizable list type, or if any
// element is non-string. A partially non-string array is treated as invalid
// rather than silently dropping items, which could cause a malformed rule to
// behave as "match all" instead of "match nothing".
func toStringSlice(v interface{}) ([]string, bool) {
	switch val := v.(type) {
	case []string:
		return val, true
	case []interface{}:
		result := make([]string, 0, len(val))
		for _, item := range val {
			s, ok := item.(string)
			if !ok {
				// Non-string element — treat whole array as invalid to avoid
				// a malformed rule accidentally becoming "match all"
				return nil, false
			}
			result = append(result, s)
		}
		return result, true
	}
	return nil, false
}

// containsString checks if a string is in a slice (case-insensitive)
func containsString(slice []string, s string) bool {
	lower := strings.ToLower(s)
	for _, item := range slice {
		if strings.ToLower(item) == lower {
			return true
		}
	}
	return false
}
