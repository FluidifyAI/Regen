package services

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/repository"
	"gorm.io/gorm"
)

// GroupingDecision represents the result of evaluating an alert against grouping rules
type GroupingDecision struct {
	// Action specifies what to do with this alert
	Action GroupingAction

	// IncidentID is the existing incident to link to (only set if Action = GroupActionLinkToExisting)
	IncidentID *uuid.UUID

	// RuleName is the name of the rule that matched (empty for default action)
	RuleName string

	// GroupKey is the derived key for this alert group
	GroupKey string
}

// GroupingAction defines the possible grouping actions
type GroupingAction string

const (
	// GroupActionCreateNew means create a new incident for this alert
	GroupActionCreateNew GroupingAction = "create_new"

	// GroupActionLinkToExisting means link this alert to an existing incident
	GroupActionLinkToExisting GroupingAction = "link_to_existing"

	// GroupActionDefault means no rule matched, use default behavior (create new)
	GroupActionDefault GroupingAction = "default"
)

// GroupingEngine evaluates alerts against grouping rules to determine incident creation/linking
type GroupingEngine interface {
	// EvaluateAlert determines if an alert should create a new incident or link to existing
	EvaluateAlert(alert *models.Alert) (*GroupingDecision, error)

	// RefreshRules reloads grouping rules from the database (called on cache expiry)
	RefreshRules() error
}

// groupingEngine implements GroupingEngine
type groupingEngine struct {
	ruleRepo     repository.GroupingRuleRepository
	incidentRepo repository.IncidentRepository
	db           *gorm.DB

	// Rule cache to avoid database hits on every alert
	rulesCache      []models.GroupingRule
	rulesCacheMutex sync.RWMutex
	rulesCacheExpiry time.Time
	ruleCacheTTL    time.Duration
}

// NewGroupingEngine creates a new grouping engine
func NewGroupingEngine(
	ruleRepo repository.GroupingRuleRepository,
	incidentRepo repository.IncidentRepository,
	db *gorm.DB,
) GroupingEngine {
	engine := &groupingEngine{
		ruleRepo:     ruleRepo,
		incidentRepo: incidentRepo,
		db:           db,
		ruleCacheTTL: 5 * time.Second, // Refresh rules every 5 seconds
	}

	// Load rules on initialization
	_ = engine.RefreshRules()

	return engine
}

// EvaluateAlert determines grouping action for an alert
func (g *groupingEngine) EvaluateAlert(alert *models.Alert) (*GroupingDecision, error) {
	// Ensure rules cache is fresh
	if err := g.ensureRulesCached(); err != nil {
		return nil, fmt.Errorf("failed to load grouping rules: %w", err)
	}

	// Get cached rules (read lock)
	g.rulesCacheMutex.RLock()
	rules := g.rulesCache
	g.rulesCacheMutex.RUnlock()

	// Convert JSONB labels to map[string]string for matching
	alertLabels := make(map[string]string)
	for k, v := range alert.Labels {
		if strVal, ok := v.(string); ok {
			alertLabels[k] = strVal
		}
	}

	slog.Info("grouping engine evaluating alert",
		"alert_id", alert.ID,
		"alert_title", alert.Title,
		"alert_source", alert.Source,
		"alert_labels", alertLabels,
		"rules_count", len(rules),
	)

	// Evaluate rules in priority order (first match wins)
	for _, rule := range rules {
		if g.matchesRule(alertLabels, &rule) {
			// Rule matched - derive group key and check for existing incident
			groupKey := g.deriveGroupKey(alertLabels, &rule)

			slog.Info("grouping rule matched",
				"alert_id", alert.ID,
				"rule_name", rule.Name,
				"rule_id", rule.ID,
				"group_key", groupKey,
			)

			// Find existing open incident within time window
			existingIncident, err := g.findOpenIncidentForGroup(groupKey, rule.TimeWindowSeconds)
			if err != nil {
				return nil, fmt.Errorf("failed to find existing incident: %w", err)
			}

			if existingIncident != nil {
				slog.Info("grouping: linking to existing incident",
					"alert_id", alert.ID,
					"incident_id", existingIncident.ID,
					"group_key", groupKey,
				)
				return &GroupingDecision{
					Action:     GroupActionLinkToExisting,
					IncidentID: &existingIncident.ID,
					RuleName:   rule.Name,
					GroupKey:   groupKey,
				}, nil
			}

			slog.Info("grouping: creating new incident with group key",
				"alert_id", alert.ID,
				"group_key", groupKey,
			)
			return &GroupingDecision{
				Action:   GroupActionCreateNew,
				RuleName: rule.Name,
				GroupKey: groupKey,
			}, nil
		} else {
			slog.Debug("grouping rule did not match",
				"alert_id", alert.ID,
				"rule_name", rule.Name,
			)
		}
	}

	slog.Info("grouping: no rule matched, using default behavior",
		"alert_id", alert.ID,
	)
	return &GroupingDecision{
		Action: GroupActionDefault,
	}, nil
}

// RefreshRules reloads grouping rules from the database
func (g *groupingEngine) RefreshRules() error {
	rules, err := g.ruleRepo.GetEnabled()
	if err != nil {
		return fmt.Errorf("failed to load enabled grouping rules: %w", err)
	}

	g.rulesCacheMutex.Lock()
	g.rulesCache = rules
	g.rulesCacheExpiry = time.Now().Add(g.ruleCacheTTL)
	g.rulesCacheMutex.Unlock()

	slog.Info("grouping engine rules refreshed",
		"rules_count", len(rules),
	)

	return nil
}

// ensureRulesCached refreshes the rules cache if expired
func (g *groupingEngine) ensureRulesCached() error {
	g.rulesCacheMutex.RLock()
	expired := time.Now().After(g.rulesCacheExpiry)
	g.rulesCacheMutex.RUnlock()

	if expired {
		return g.RefreshRules()
	}

	return nil
}

// matchesRule checks if an alert matches a grouping rule
func (g *groupingEngine) matchesRule(alertLabels map[string]string, rule *models.GroupingRule) bool {
	// Parse rule's match_labels from JSONB
	matchLabels := make(map[string]string)
	for k, v := range rule.MatchLabels {
		if strVal, ok := v.(string); ok {
			matchLabels[k] = strVal
		}
	}

	// Empty match_labels matches all alerts
	if len(matchLabels) == 0 {
		return true
	}

	// Check each match label
	for key, matchValue := range matchLabels {
		alertValue, exists := alertLabels[key]

		// Wildcard "*" matches any value (but label must exist)
		if matchValue == "*" {
			if !exists {
				return false
			}
			continue
		}

		// Exact value match required
		if !exists || alertValue != matchValue {
			return false
		}
	}

	return true
}

// deriveGroupKey generates a unique key for grouping alerts
func (g *groupingEngine) deriveGroupKey(alertLabels map[string]string, rule *models.GroupingRule) string {
	// Determine which labels to use for group key derivation:
	// - If cross_source_labels is specified, use ONLY those labels (enables cross-source correlation)
	// - Otherwise, use match_labels (default behavior for same-source grouping)
	var keysToInclude []string

	// Check if cross_source_labels is specified and non-empty
	if len(rule.CrossSourceLabels) > 0 {
		// Use cross_source_labels for grouping (enables cross-source correlation)
		keysToInclude = rule.CrossSourceLabels
	} else {
		// Fallback to match_labels keys (default behavior)
		matchLabels := make(map[string]string)
		for k, v := range rule.MatchLabels {
			if strVal, ok := v.(string); ok {
				matchLabels[k] = strVal
			}
		}

		// Get all keys from match_labels
		for k := range matchLabels {
			keysToInclude = append(keysToInclude, k)
		}
	}

	// Sort keys for deterministic ordering
	sort.Strings(keysToInclude)

	// Build key parts from alert labels
	var keyParts []string
	for _, key := range keysToInclude {
		if value, exists := alertLabels[key]; exists {
			keyParts = append(keyParts, fmt.Sprintf("%s=%s", key, value))
		}
	}

	// Join parts and hash for compact, consistent key
	groupKeyStr := strings.Join(keyParts, "|")
	hash := sha256.Sum256([]byte(groupKeyStr))
	return fmt.Sprintf("%x", hash)
}

// findOpenIncidentForGroup finds an existing open incident for the given group key
func (g *groupingEngine) findOpenIncidentForGroup(groupKey string, timeWindowSeconds int) (*models.Incident, error) {
	// Calculate time window cutoff
	cutoffTime := time.Now().Add(-time.Duration(timeWindowSeconds) * time.Second)

	// Query for open incidents within time window with matching group key
	// Uses the composite index: idx_incidents_group_key_status_created
	var incident models.Incident
	err := g.db.
		Where("group_key = ?", groupKey).
		Where("status IN (?)", []string{"triggered", "acknowledged"}).
		Where("created_at >= ?", cutoffTime).
		Order("created_at DESC").
		First(&incident).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// No matching incident found - this is not an error
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query open incidents: %w", err)
	}

	return &incident, nil
}

// hashGroupKey generates a numeric hash for PostgreSQL advisory locks
func (g *groupingEngine) hashGroupKey(groupKey string) int64 {
	hash := sha256.Sum256([]byte(groupKey))
	// Use first 8 bytes as int64 for advisory lock ID
	lockID := int64(hash[0]) | int64(hash[1])<<8 | int64(hash[2])<<16 | int64(hash[3])<<24 |
		int64(hash[4])<<32 | int64(hash[5])<<40 | int64(hash[6])<<48 | int64(hash[7])<<56
	return lockID
}

// AcquireGroupLock acquires a PostgreSQL advisory lock for the given group key
//
// This prevents race conditions when multiple concurrent webhooks try to create
// the same incident group simultaneously.
//
// The lock is automatically released at the end of the current transaction.
func (g *groupingEngine) AcquireGroupLock(groupKey string) error {
	lockID := g.hashGroupKey(groupKey)

	// pg_advisory_xact_lock is transaction-scoped (auto-releases on commit/rollback)
	err := g.db.Exec("SELECT pg_advisory_xact_lock(?)", lockID).Error
	if err != nil {
		return fmt.Errorf("failed to acquire advisory lock for group %s: %w", groupKey, err)
	}

	return nil
}
