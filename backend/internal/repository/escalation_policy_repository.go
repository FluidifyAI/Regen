package repository

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/FluidifyAI/Regen/backend/internal/models"
	"gorm.io/gorm"
)

// EscalationPolicyRepository defines operations for managing escalation policies,
// tiers, and per-alert escalation state.
type EscalationPolicyRepository interface {
	// --- Policy CRUD ---

	// CreatePolicy persists a new escalation policy (without tiers).
	CreatePolicy(policy *models.EscalationPolicy) error

	// GetPolicyByID retrieves a policy by ID (without tiers).
	GetPolicyByID(id uuid.UUID) (*models.EscalationPolicy, error)

	// GetPolicyWithTiers retrieves a policy and eagerly loads its tiers in
	// ascending tier_index order.
	GetPolicyWithTiers(id uuid.UUID) (*models.EscalationPolicy, error)

	// GetAllPolicies retrieves all policies ordered by name (without tiers).
	GetAllPolicies() ([]models.EscalationPolicy, error)

	// GetAllPoliciesWithTiers retrieves all policies with their tiers eagerly
	// loaded using a single IN query. Use this for list endpoints to avoid N+1.
	GetAllPoliciesWithTiers() ([]models.EscalationPolicy, error)

	// GetEnabledPolicies retrieves only enabled policies.
	// This is the hot-path query used by the routing engine.
	GetEnabledPolicies() ([]models.EscalationPolicy, error)

	// UpdatePolicy updates mutable fields on an existing policy.
	UpdatePolicy(policy *models.EscalationPolicy) error

	// DeletePolicy deletes a policy and cascades to its tiers and states.
	DeletePolicy(id uuid.UUID) error

	// --- Tier CRUD ---

	// CreateTier adds a new tier to an existing policy.
	CreateTier(tier *models.EscalationTier) error

	// GetTiersByPolicy retrieves all tiers for a policy in ascending tier_index order.
	GetTiersByPolicy(policyID uuid.UUID) ([]models.EscalationTier, error)

	// UpdateTier persists changes to an existing escalation tier.
	UpdateTier(tier *models.EscalationTier) error

	// DeleteTier removes a single tier by ID.
	DeleteTier(id uuid.UUID) error

	// --- Escalation State ---

	// CreateState creates a new escalation state for an alert.
	// Returns ErrAlreadyExists if a state for this alert already exists.
	CreateState(state *models.EscalationState) error

	// GetStateByAlert retrieves the escalation state for a specific alert.
	GetStateByAlert(alertID uuid.UUID) (*models.EscalationState, error)

	// GetStateByIncident retrieves the active escalation state for a manually escalated incident.
	GetStateByIncident(incidentID uuid.UUID) (*models.EscalationState, error)

	// GetActiveStates retrieves all escalation states that are still in progress
	// (not acknowledged and not completed) for worker polling.
	GetActiveStates() ([]models.EscalationState, error)

	// UpdateState persists changes to an escalation state row (tier advancement,
	// last_notified_at, status transitions).
	UpdateState(state *models.EscalationState) error

	// RecordAcknowledgment marks an escalation state as acknowledged and updates
	// the alert's acknowledgment_status in a single transaction.
	RecordAcknowledgment(alertID uuid.UUID, by string, via models.AcknowledgmentVia) error

	// ListSeverityRules returns all severity → policy mappings.
	ListSeverityRules() ([]models.EscalationSeverityRule, error)

	// GetSeverityRule returns the rule for a specific severity, or nil if unset.
	GetSeverityRule(severity string) (*models.EscalationSeverityRule, error)

	// UpsertSeverityRule creates or updates the escalation policy for a severity level.
	UpsertSeverityRule(severity string, policyID uuid.UUID) (*models.EscalationSeverityRule, error)

	// DeleteSeverityRule removes the escalation rule for a severity level.
	DeleteSeverityRule(severity string) error
}

// escalationPolicyRepository implements EscalationPolicyRepository.
type escalationPolicyRepository struct {
	db *gorm.DB
}

// NewEscalationPolicyRepository creates a new escalation policy repository.
func NewEscalationPolicyRepository(db *gorm.DB) EscalationPolicyRepository {
	return &escalationPolicyRepository{db: db}
}

// ── Policy CRUD ──────────────────────────────────────────────────────────────

// CreatePolicy persists a new escalation policy.
func (r *escalationPolicyRepository) CreatePolicy(policy *models.EscalationPolicy) error {
	if err := validatePolicy(policy); err != nil {
		return err
	}
	if err := r.db.Create(policy).Error; err != nil {
		return fmt.Errorf("failed to create escalation policy: %w", err)
	}
	return nil
}

// GetPolicyByID retrieves a policy by ID without loading tiers.
func (r *escalationPolicyRepository) GetPolicyByID(id uuid.UUID) (*models.EscalationPolicy, error) {
	var policy models.EscalationPolicy
	err := r.db.Where("id = ?", id).First(&policy).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, &NotFoundError{Resource: "escalation_policy", ID: id.String()}
		}
		return nil, fmt.Errorf("failed to get escalation policy: %w", err)
	}
	return &policy, nil
}

// GetPolicyWithTiers retrieves a policy and eagerly loads its tiers.
func (r *escalationPolicyRepository) GetPolicyWithTiers(id uuid.UUID) (*models.EscalationPolicy, error) {
	policy, err := r.GetPolicyByID(id)
	if err != nil {
		return nil, err
	}
	tiers, err := r.GetTiersByPolicy(id)
	if err != nil {
		return nil, err
	}
	policy.Tiers = tiers
	return policy, nil
}

// GetAllPolicies retrieves all policies ordered by name (without tiers).
func (r *escalationPolicyRepository) GetAllPolicies() ([]models.EscalationPolicy, error) {
	var policies []models.EscalationPolicy
	if err := r.db.Order("name ASC").Find(&policies).Error; err != nil {
		return nil, fmt.Errorf("failed to get all escalation policies: %w", err)
	}
	return policies, nil
}

// GetAllPoliciesWithTiers retrieves all policies with their tiers in two queries
// (one for policies, one IN query for all tiers) to avoid N+1 database calls.
func (r *escalationPolicyRepository) GetAllPoliciesWithTiers() ([]models.EscalationPolicy, error) {
	policies, err := r.GetAllPolicies()
	if err != nil {
		return nil, err
	}
	if len(policies) == 0 {
		return policies, nil
	}

	// Collect all policy IDs for a single batch tier query.
	ids := make([]uuid.UUID, len(policies))
	for i, p := range policies {
		ids[i] = p.ID
	}

	var tiers []models.EscalationTier
	if err := r.db.
		Where("policy_id IN ?", ids).
		Order("policy_id ASC, tier_index ASC").
		Find(&tiers).Error; err != nil {
		return nil, fmt.Errorf("failed to batch-load escalation tiers: %w", err)
	}

	// Distribute tiers into the correct policy by index.
	tiersByPolicy := make(map[uuid.UUID][]models.EscalationTier, len(policies))
	for _, t := range tiers {
		tiersByPolicy[t.PolicyID] = append(tiersByPolicy[t.PolicyID], t)
	}
	for i := range policies {
		policies[i].Tiers = tiersByPolicy[policies[i].ID]
	}
	return policies, nil
}

// GetEnabledPolicies retrieves only enabled policies.
// Uses the partial index idx_escalation_policies_enabled for performance.
func (r *escalationPolicyRepository) GetEnabledPolicies() ([]models.EscalationPolicy, error) {
	var policies []models.EscalationPolicy
	err := r.db.Where("enabled = ?", true).Order("name ASC").Find(&policies).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get enabled escalation policies: %w", err)
	}
	return policies, nil
}

// UpdatePolicy updates mutable fields on an existing policy.
func (r *escalationPolicyRepository) UpdatePolicy(policy *models.EscalationPolicy) error {
	if err := validatePolicy(policy); err != nil {
		return err
	}
	result := r.db.Model(policy).
		Select("name", "description", "enabled", "updated_at").
		Updates(policy)
	if result.Error != nil {
		return fmt.Errorf("failed to update escalation policy: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return &NotFoundError{Resource: "escalation_policy", ID: policy.ID.String()}
	}
	return nil
}

// DeletePolicy deletes a policy; cascades to tiers and states via DB constraints.
func (r *escalationPolicyRepository) DeletePolicy(id uuid.UUID) error {
	result := r.db.Delete(&models.EscalationPolicy{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete escalation policy: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return &NotFoundError{Resource: "escalation_policy", ID: id.String()}
	}
	return nil
}

// ── Tier CRUD ─────────────────────────────────────────────────────────────────

// CreateTier adds a new tier to an existing policy.
func (r *escalationPolicyRepository) CreateTier(tier *models.EscalationTier) error {
	if err := validateTier(tier); err != nil {
		return err
	}
	if err := r.db.Create(tier).Error; err != nil {
		return fmt.Errorf("failed to create escalation tier: %w", err)
	}
	return nil
}

// GetTiersByPolicy retrieves all tiers for a policy in tier_index order.
func (r *escalationPolicyRepository) GetTiersByPolicy(policyID uuid.UUID) ([]models.EscalationTier, error) {
	var tiers []models.EscalationTier
	err := r.db.
		Where("policy_id = ?", policyID).
		Order("tier_index ASC").
		Find(&tiers).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get tiers for policy %s: %w", policyID, err)
	}
	return tiers, nil
}

// DeleteTier removes a single tier by ID.
func (r *escalationPolicyRepository) DeleteTier(id uuid.UUID) error {
	result := r.db.Delete(&models.EscalationTier{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete escalation tier: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return &NotFoundError{Resource: "escalation_tier", ID: id.String()}
	}
	return nil
}

// UpdateTier persists changes to an existing escalation tier.
func (r *escalationPolicyRepository) UpdateTier(tier *models.EscalationTier) error {
	if err := validateTier(tier); err != nil {
		return err
	}
	if err := r.db.Save(tier).Error; err != nil {
		return fmt.Errorf("failed to update escalation tier: %w", err)
	}
	return nil
}

// ── Escalation State ──────────────────────────────────────────────────────────

// CreateState creates a new escalation state for an alert.
func (r *escalationPolicyRepository) CreateState(state *models.EscalationState) error {
	if err := r.db.Create(state).Error; err != nil {
		if isDuplicateKeyError(err) {
			val := "nil"
			if state.AlertID != nil {
				val = state.AlertID.String()
			}
			return &AlreadyExistsError{
				Resource: "escalation_state",
				Field:    "alert_id",
				Value:    val,
			}
		}
		return fmt.Errorf("failed to create escalation state: %w", err)
	}
	return nil
}

// GetStateByAlert retrieves the escalation state for a specific alert.
func (r *escalationPolicyRepository) GetStateByAlert(alertID uuid.UUID) (*models.EscalationState, error) {
	var state models.EscalationState
	err := r.db.Where("alert_id = ?", alertID).First(&state).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, &NotFoundError{Resource: "escalation_state", ID: alertID.String()}
		}
		return nil, fmt.Errorf("failed to get escalation state for alert %s: %w", alertID, err)
	}
	return &state, nil
}

// GetStateByIncident retrieves the active escalation state for a manually escalated incident.
func (r *escalationPolicyRepository) GetStateByIncident(incidentID uuid.UUID) (*models.EscalationState, error) {
	var state models.EscalationState
	err := r.db.Where("incident_id = ? AND source_type = 'incident'", incidentID).First(&state).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, &NotFoundError{Resource: "escalation_state", ID: incidentID.String()}
		}
		return nil, fmt.Errorf("failed to get escalation state for incident %s: %w", incidentID, err)
	}
	return &state, nil
}

// GetActiveStates retrieves all escalation states that are not yet resolved.
// The worker calls this every 30 s to find tiers that have timed out.
func (r *escalationPolicyRepository) GetActiveStates() ([]models.EscalationState, error) {
	var states []models.EscalationState
	err := r.db.
		Where("acknowledged_at IS NULL AND status IN ?",
			[]string{
				string(models.EscalationStatePending),
				string(models.EscalationStateNotified),
			}).
		Find(&states).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get active escalation states: %w", err)
	}
	return states, nil
}

// UpdateState persists changes to an escalation state (tier index, timestamps,
// status).
func (r *escalationPolicyRepository) UpdateState(state *models.EscalationState) error {
	err := r.db.Model(state).
		Select(
			"current_tier_index",
			"status",
			"last_notified_at",
			"acknowledged_at",
			"acknowledged_by",
			"acknowledged_via",
		).
		Updates(state).Error
	if err != nil {
		return fmt.Errorf("failed to update escalation state: %w", err)
	}
	return nil
}

// RecordAcknowledgment marks the escalation state acknowledged and updates
// the alert's acknowledgment_status in a single transaction to keep them
// consistent.
func (r *escalationPolicyRepository) RecordAcknowledgment(
	alertID uuid.UUID,
	by string,
	via models.AcknowledgmentVia,
) error {
	now := time.Now()

	return r.db.Transaction(func(tx *gorm.DB) error {
		// Update escalation state.
		result := tx.Model(&models.EscalationState{}).
			Where("alert_id = ? AND acknowledged_at IS NULL", alertID).
			Updates(map[string]interface{}{
				"acknowledged_at":  now,
				"acknowledged_by":  by,
				"acknowledged_via": string(via),
				"status":           string(models.EscalationStateAcknowledged),
			})
		if result.Error != nil {
			return fmt.Errorf("failed to acknowledge escalation state: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			// Either already acknowledged or no state exists — both are acceptable
			// (idempotent).
			return nil
		}

		// Denormalize to alerts table for fast querying.
		if err := tx.Model(&models.Alert{}).
			Where("id = ?", alertID).
			Update("acknowledgment_status", string(models.AcknowledgmentStatusAcknowledged)).
			Error; err != nil {
			return fmt.Errorf("failed to update alert acknowledgment_status: %w", err)
		}

		return nil
	})
}

// ── Severity rule methods ─────────────────────────────────────────────────────

// ListSeverityRules returns all severity → policy mappings ordered by severity.
func (r *escalationPolicyRepository) ListSeverityRules() ([]models.EscalationSeverityRule, error) {
	var rules []models.EscalationSeverityRule
	if err := r.db.Order("severity").Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

// GetSeverityRule returns the rule for a specific severity, or nil if none exists.
func (r *escalationPolicyRepository) GetSeverityRule(severity string) (*models.EscalationSeverityRule, error) {
	var rule models.EscalationSeverityRule
	err := r.db.Where("severity = ?", severity).First(&rule).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// UpsertSeverityRule creates or replaces the escalation policy for a severity level.
func (r *escalationPolicyRepository) UpsertSeverityRule(severity string, policyID uuid.UUID) (*models.EscalationSeverityRule, error) {
	var rule models.EscalationSeverityRule
	result := r.db.Where(models.EscalationSeverityRule{Severity: severity}).
		Assign(models.EscalationSeverityRule{EscalationPolicyID: policyID}).
		FirstOrCreate(&rule)
	if result.Error != nil {
		return nil, result.Error
	}
	// If row already existed (RowsAffected == 0 from FirstOrCreate), update the policy.
	if rule.EscalationPolicyID != policyID {
		if err := r.db.Model(&rule).Update("escalation_policy_id", policyID).Error; err != nil {
			return nil, err
		}
		rule.EscalationPolicyID = policyID
	}
	return &rule, nil
}

// DeleteSeverityRule removes the escalation rule for a severity level (no-op if missing).
func (r *escalationPolicyRepository) DeleteSeverityRule(severity string) error {
	return r.db.Where("severity = ?", severity).Delete(&models.EscalationSeverityRule{}).Error
}

// ── Validation helpers ────────────────────────────────────────────────────────

func validatePolicy(policy *models.EscalationPolicy) error {
	if policy.Name == "" {
		return &ValidationError{Field: "name", Message: "escalation policy name cannot be empty"}
	}
	return nil
}

func validateTier(tier *models.EscalationTier) error {
	if tier.PolicyID == uuid.Nil {
		return &ValidationError{Field: "policy_id", Message: "tier must belong to a policy"}
	}
	if tier.TierIndex < 0 {
		return &ValidationError{Field: "tier_index", Message: "tier_index must be >= 0"}
	}
	if tier.TimeoutSeconds <= 0 {
		return &ValidationError{Field: "timeout_seconds", Message: "timeout_seconds must be > 0"}
	}
	switch tier.TargetType {
	case models.EscalationTargetSchedule, models.EscalationTargetUsers, models.EscalationTargetBoth:
		// valid
	default:
		return &ValidationError{
			Field:   "target_type",
			Message: fmt.Sprintf("invalid target_type %q; must be schedule, users, or both", tier.TargetType),
		}
	}
	return nil
}

// isDuplicateKeyError checks whether a GORM error is a unique-constraint violation.
// Uses both the GORM sentinel (works with pgx/v5) and a PostgreSQL error-code
// string check (works with lib/pq and other drivers that don't wrap via errors.Is).
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	// Sentinel works when GORM wraps errors properly (pgx/v5).
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	// Fallback: PostgreSQL SQLSTATE 23505 (unique_violation) in error string.
	// lib/pq and some other drivers surface this as a plain error message.
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "unique constraint") || strings.Contains(msg, "UNIQUE constraint failed")
}
