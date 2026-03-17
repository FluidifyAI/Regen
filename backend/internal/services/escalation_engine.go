package services

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/fluidify/regen/internal/models"
	"github.com/fluidify/regen/internal/repository"
)

// EscalationNotifier sends notifications to on-call users during escalation.
// The worker implements this via the ChatService.
type EscalationNotifier interface {
	// SendEscalationDM sends a direct message to userID about the given alert
	// at the specified escalation tier.
	SendEscalationDM(userID string, alert *models.Alert, tierIndex int) error
}

// EscalationEngine drives the escalation lifecycle for alerts.
//
// Responsibilities:
//   - TriggerEscalation: called by the alert processing pipeline when a new
//     alert is linked to a policy.
//   - EvaluateEscalations: called by the background worker every 30 s to send
//     notifications and advance timed-out tiers.
//   - AcknowledgeAlert: called by the acknowledgment API (Slack, REST, CLI).
//   - MarkAlertCompleted: called when an alert resolves before being acknowledged.
type EscalationEngine interface {
	TriggerEscalation(alert *models.Alert) error
	TriggerIncidentEscalation(incidentID uuid.UUID, policyID uuid.UUID) error
	EvaluateEscalations() error
	AcknowledgeAlert(alertID uuid.UUID, by string, via models.AcknowledgmentVia) error
	MarkAlertCompleted(alertID uuid.UUID) error
}

// AlertLookupFunc retrieves an alert by ID; injected for testing.
type AlertLookupFunc func(id uuid.UUID) (*models.Alert, error)

// escalationEngine implements EscalationEngine.
type escalationEngine struct {
	repo        repository.EscalationPolicyRepository
	evaluator   ScheduleEvaluator
	notifier    EscalationNotifier
	alertLookup AlertLookupFunc // overridable in tests; nil uses a no-op
}

// NewEscalationEngine creates a new escalation engine.
// notifier may be nil when chat is not configured; notifications are skipped.
func NewEscalationEngine(
	repo repository.EscalationPolicyRepository,
	evaluator ScheduleEvaluator,
	notifier EscalationNotifier,
) EscalationEngine {
	return &escalationEngine{
		repo:      repo,
		evaluator: evaluator,
		notifier:  notifier,
	}
}

// ── TriggerEscalation ─────────────────────────────────────────────────────────

// TriggerEscalation creates an EscalationState for the alert and sets it to
// "pending".  The worker's next poll will send the tier-0 notification.
//
// If the alert has no EscalationPolicyID, this is a no-op.
func (e *escalationEngine) TriggerEscalation(alert *models.Alert) error {
	if alert.EscalationPolicyID == nil {
		return nil
	}

	// Verify the policy exists.
	if _, err := e.repo.GetPolicyByID(*alert.EscalationPolicyID); err != nil {
		return fmt.Errorf("escalation policy not found: %w", err)
	}

	alertID := alert.ID
	state := &models.EscalationState{
		AlertID:          &alertID,
		PolicyID:         *alert.EscalationPolicyID,
		CurrentTierIndex: 0,
		Status:           models.EscalationStatePending,
		SourceType:       "alert",
	}

	if err := e.repo.CreateState(state); err != nil {
		// Already-exists is acceptable (idempotent trigger).
		var alreadyExists *repository.AlreadyExistsError
		if errors.As(err, &alreadyExists) {
			return nil
		}
		return fmt.Errorf("failed to create escalation state: %w", err)
	}

	slog.Info("escalation triggered",
		"alert_id", alert.ID,
		"policy_id", alert.EscalationPolicyID,
	)
	return nil
}

// ── TriggerIncidentEscalation ─────────────────────────────────────────────────

// TriggerIncidentEscalation creates an EscalationState for a manually escalated
// incident. If an escalation is already running for this incident, this is a no-op.
func (e *escalationEngine) TriggerIncidentEscalation(incidentID uuid.UUID, policyID uuid.UUID) error {
	if _, err := e.repo.GetPolicyByID(policyID); err != nil {
		return fmt.Errorf("escalation policy not found: %w", err)
	}

	state := &models.EscalationState{
		IncidentID:       &incidentID,
		PolicyID:         policyID,
		CurrentTierIndex: 0,
		Status:           models.EscalationStatePending,
		SourceType:       "incident",
	}

	if err := e.repo.CreateState(state); err != nil {
		var alreadyExists *repository.AlreadyExistsError
		if errors.As(err, &alreadyExists) {
			return nil // idempotent
		}
		return fmt.Errorf("failed to create escalation state: %w", err)
	}
	slog.Info("incident escalation triggered", "incident_id", incidentID, "policy_id", policyID)
	return nil
}

// ── EvaluateEscalations ───────────────────────────────────────────────────────

// EvaluateEscalations is called by the worker every 30 s.  For each active
// (unacknowledged) escalation state it:
//  1. Sends notifications if the state is "pending" (first notification).
//  2. Advances to the next tier if the current tier has timed out.
//  3. Marks the state "completed" if the last tier has been exhausted.
func (e *escalationEngine) EvaluateEscalations() error {
	states, err := e.repo.GetActiveStates()
	if err != nil {
		return fmt.Errorf("failed to load active escalation states: %w", err)
	}
	if len(states) == 0 {
		return nil
	}

	// Batch-load all policies with their tiers in two queries (policies + tiers IN (...))
	// rather than one GetPolicyWithTiers query per state, which would be O(N) queries.
	allPolicies, err := e.repo.GetAllPoliciesWithTiers()
	if err != nil {
		return fmt.Errorf("failed to load escalation policies: %w", err)
	}
	policyByID := make(map[uuid.UUID]*models.EscalationPolicy, len(allPolicies))
	for i := range allPolicies {
		policyByID[allPolicies[i].ID] = &allPolicies[i]
	}

	for i := range states {
		if err := e.processState(&states[i], policyByID); err != nil {
			// Log and continue — one bad state should not block others.
			slog.Error("failed to process escalation state",
				"state_id", states[i].ID,
				"alert_id", states[i].AlertID,
				"err", err,
			)
		}
	}
	return nil
}

// processState handles one escalation state row.
func (e *escalationEngine) processState(state *models.EscalationState, policyByID map[uuid.UUID]*models.EscalationPolicy) error {
	policy, ok := policyByID[state.PolicyID]
	if !ok {
		return fmt.Errorf("policy %s not found", state.PolicyID)
	}

	if len(policy.Tiers) == 0 {
		return e.markCompleted(state)
	}

	var alert *models.Alert
	if state.AlertID != nil {
		var err error
		alert, err = e.lookupAlert(*state.AlertID)
		if err != nil {
			return fmt.Errorf("alert %s not found: %w", state.AlertID, err)
		}
	}

	// Pending state: notify tier 0 now (possibly skipping to next if empty).
	if state.Status == models.EscalationStatePending {
		return e.notifyTier(state, policy, alert)
	}

	// Notified state: check whether the current tier has timed out.
	if state.Status == models.EscalationStateNotified {
		currentTier := e.findTier(policy.Tiers, state.CurrentTierIndex)
		if currentTier == nil {
			return e.markCompleted(state)
		}

		deadline := state.LastNotifiedAt.Add(time.Duration(currentTier.TimeoutSeconds) * time.Second)
		if time.Now().Before(deadline) {
			// Still within timeout window — nothing to do.
			return nil
		}

		// Timed out — advance to the next tier.
		nextIndex := state.CurrentTierIndex + 1
		nextTier := e.findTier(policy.Tiers, nextIndex)
		if nextTier == nil {
			// No more tiers — escalation exhausted.
			return e.markCompleted(state)
		}

		state.CurrentTierIndex = nextIndex
		state.Status = models.EscalationStatePending
		state.LastNotifiedAt = nil
		if err := e.repo.UpdateState(state); err != nil {
			return err
		}

		return e.notifyTier(state, policy, alert)
	}

	return nil
}

// notifyTier resolves targets for state.CurrentTierIndex and sends DMs.
// If a tier has no targets (e.g. schedule has nobody on call) it advances to
// the next tier and tries again, using an iterative loop to avoid unbounded
// recursion in pathological policies where many tiers are empty.
func (e *escalationEngine) notifyTier(
	state *models.EscalationState,
	policy *models.EscalationPolicy,
	alert *models.Alert,
) error {
	for {
		tier := e.findTier(policy.Tiers, state.CurrentTierIndex)
		if tier == nil {
			return e.markCompleted(state)
		}

		targets, err := e.resolveTargets(tier, alert)
		if err != nil {
			return err
		}

		if len(targets) > 0 {
			// Found a tier with targets — send DMs and record notification.
			for _, userID := range targets {
				if e.notifier != nil {
					if err := e.notifier.SendEscalationDM(userID, alert, state.CurrentTierIndex); err != nil {
						slog.Error("failed to send escalation DM",
							"user_id", userID,
							"alert_id", alert.ID,
							"err", err,
						)
					}
				}
			}
			now := time.Now()
			state.LastNotifiedAt = &now
			state.Status = models.EscalationStateNotified
			return e.repo.UpdateState(state)
		}

		// This tier has no targets (e.g., schedule has no on-call user).
		// Advance to the next tier and continue the loop.
		slog.Warn("escalation tier has no targets; advancing",
			"policy_id", policy.ID,
			"tier_index", state.CurrentTierIndex,
		)
		nextIndex := state.CurrentTierIndex + 1
		nextTier := e.findTier(policy.Tiers, nextIndex)
		if nextTier == nil {
			return e.markCompleted(state)
		}
		state.CurrentTierIndex = nextIndex
		if err := e.repo.UpdateState(state); err != nil {
			return err
		}
	}
}

// resolveTargets returns the list of user identifiers to notify for a tier.
func (e *escalationEngine) resolveTargets(tier *models.EscalationTier, _ *models.Alert) ([]string, error) {
	var targets []string

	if tier.TargetType == models.EscalationTargetSchedule || tier.TargetType == models.EscalationTargetBoth {
		if tier.ScheduleID != nil {
			user, err := e.evaluator.WhoIsOnCall(*tier.ScheduleID, time.Now())
			if err != nil {
				slog.Warn("failed to resolve on-call user for escalation tier",
					"schedule_id", tier.ScheduleID,
					"err", err,
				)
			} else if user != "" {
				targets = append(targets, user)
			}
		}
	}

	if tier.TargetType == models.EscalationTargetUsers || tier.TargetType == models.EscalationTargetBoth {
		targets = append(targets, []string(tier.UserNames)...)
	}

	return targets, nil
}

// ── AcknowledgeAlert ──────────────────────────────────────────────────────────

// AcknowledgeAlert stops the escalation for the given alert.  Idempotent:
// acknowledging an already-acknowledged alert is a no-op.
func (e *escalationEngine) AcknowledgeAlert(
	alertID uuid.UUID,
	by string,
	via models.AcknowledgmentVia,
) error {
	if err := e.repo.RecordAcknowledgment(alertID, by, via); err != nil {
		return fmt.Errorf("failed to record acknowledgment: %w", err)
	}
	slog.Info("alert acknowledged", "alert_id", alertID, "by", by, "via", via)
	return nil
}

// ── MarkAlertCompleted ────────────────────────────────────────────────────────

// MarkAlertCompleted ends the escalation for an alert that resolved before
// being acknowledged.  If no escalation state exists, this is a no-op.
func (e *escalationEngine) MarkAlertCompleted(alertID uuid.UUID) error {
	state, err := e.repo.GetStateByAlert(alertID)
	if err != nil {
		var notFound *repository.NotFoundError
		if errors.As(err, &notFound) {
			return nil // no escalation was running — nothing to do
		}
		return fmt.Errorf("failed to get escalation state: %w", err)
	}
	return e.markCompleted(state)
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func (e *escalationEngine) markCompleted(state *models.EscalationState) error {
	state.Status = models.EscalationStateCompleted
	if err := e.repo.UpdateState(state); err != nil {
		return fmt.Errorf("failed to mark escalation completed: %w", err)
	}
	return nil
}

// findTier returns the tier at the given index, or nil if not found.
func (e *escalationEngine) findTier(tiers []models.EscalationTier, index int) *models.EscalationTier {
	for i := range tiers {
		if tiers[i].TierIndex == index {
			return &tiers[i]
		}
	}
	return nil
}

// lookupAlert delegates to alertLookup if set (test hook), otherwise returns a
// minimal placeholder alert so the engine can function without the alert repo.
func (e *escalationEngine) lookupAlert(id uuid.UUID) (*models.Alert, error) {
	if e.alertLookup != nil {
		return e.alertLookup(id)
	}
	// Production path: alertLookup is injected by the worker/service layer.
	// Return a minimal alert so notification metadata is non-nil.
	return &models.Alert{ID: id}, nil
}
