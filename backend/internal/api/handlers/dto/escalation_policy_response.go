package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/FluidifyAI/Regen/backend/internal/models"
)

// EscalationTierResponse is the JSON representation of a single escalation tier.
type EscalationTierResponse struct {
	ID             uuid.UUID                    `json:"id"`
	PolicyID       uuid.UUID                    `json:"policy_id"`
	TierIndex      int                          `json:"tier_index"`
	TimeoutSeconds int                          `json:"timeout_seconds"`
	TargetType     models.EscalationTargetType  `json:"target_type"`
	ScheduleID     *uuid.UUID                   `json:"schedule_id,omitempty"`
	UserNames      []string                     `json:"user_names"`
	CreatedAt      time.Time                    `json:"created_at"`
}

// EscalationPolicyResponse is the JSON representation of an escalation policy.
type EscalationPolicyResponse struct {
	ID          uuid.UUID                `json:"id"`
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Enabled     bool                     `json:"enabled"`
	Tiers       []EscalationTierResponse `json:"tiers"`
	CreatedAt   time.Time                `json:"created_at"`
	UpdatedAt   time.Time                `json:"updated_at"`
}

// ToEscalationTierResponse converts a model tier to a DTO.
func ToEscalationTierResponse(t *models.EscalationTier) EscalationTierResponse {
	userNames := []string(t.UserNames)
	if userNames == nil {
		userNames = []string{}
	}
	return EscalationTierResponse{
		ID:             t.ID,
		PolicyID:       t.PolicyID,
		TierIndex:      t.TierIndex,
		TimeoutSeconds: t.TimeoutSeconds,
		TargetType:     t.TargetType,
		ScheduleID:     t.ScheduleID,
		UserNames:      userNames,
		CreatedAt:      t.CreatedAt,
	}
}

// ToEscalationPolicyResponse converts a model policy (with tiers loaded) to a DTO.
func ToEscalationPolicyResponse(p *models.EscalationPolicy) EscalationPolicyResponse {
	tiers := make([]EscalationTierResponse, len(p.Tiers))
	for i := range p.Tiers {
		tiers[i] = ToEscalationTierResponse(&p.Tiers[i])
	}
	return EscalationPolicyResponse{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		Enabled:     p.Enabled,
		Tiers:       tiers,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}
