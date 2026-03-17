package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/fluidify/regen/internal/models"
)

// RoutingRuleResponse is the response body for routing rule endpoints
type RoutingRuleResponse struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Enabled       bool      `json:"enabled"`
	Priority      int       `json:"priority"`
	MatchCriteria JSONB     `json:"match_criteria"`
	Actions       JSONB     `json:"actions"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ToRoutingRuleResponse converts models.RoutingRule to RoutingRuleResponse
func ToRoutingRuleResponse(rule *models.RoutingRule) RoutingRuleResponse {
	matchCriteria := make(JSONB)
	for k, v := range rule.MatchCriteria {
		matchCriteria[k] = v
	}

	actions := make(JSONB)
	for k, v := range rule.Actions {
		actions[k] = v
	}

	return RoutingRuleResponse{
		ID:            rule.ID,
		Name:          rule.Name,
		Description:   rule.Description,
		Enabled:       rule.Enabled,
		Priority:      rule.Priority,
		MatchCriteria: matchCriteria,
		Actions:       actions,
		CreatedAt:     rule.CreatedAt,
		UpdatedAt:     rule.UpdatedAt,
	}
}
