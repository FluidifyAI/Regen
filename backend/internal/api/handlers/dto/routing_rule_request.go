package dto

import (
	"github.com/FluidifyAI/Regen/backend/internal/models"
)

// CreateRoutingRuleRequest is the request body for POST /api/v1/routing-rules
type CreateRoutingRuleRequest struct {
	Name          string `json:"name" binding:"required,min=1,max=255"`
	Description   string `json:"description" binding:"max=1000"`
	Enabled       *bool  `json:"enabled"` // Pointer to distinguish between false and not set
	Priority      int    `json:"priority" binding:"required,min=1,max=10000"`
	MatchCriteria JSONB  `json:"match_criteria" binding:"required"`
	Actions       JSONB  `json:"actions" binding:"required"`
}

// UpdateRoutingRuleRequest is the request body for PATCH /api/v1/routing-rules/:id
type UpdateRoutingRuleRequest struct {
	Name          *string `json:"name" binding:"omitempty,min=1,max=255"`
	Description   *string `json:"description" binding:"omitempty,max=1000"`
	Enabled       *bool   `json:"enabled"`
	Priority      *int    `json:"priority" binding:"omitempty,min=1,max=10000"`
	MatchCriteria JSONB   `json:"match_criteria"`
	Actions       JSONB   `json:"actions"`
}

// ToModel converts CreateRoutingRuleRequest to models.RoutingRule
func (r *CreateRoutingRuleRequest) ToModel() *models.RoutingRule {
	enabled := true
	if r.Enabled != nil {
		enabled = *r.Enabled
	}

	matchCriteria := make(models.JSONB)
	for k, v := range r.MatchCriteria {
		matchCriteria[k] = v
	}

	actions := make(models.JSONB)
	for k, v := range r.Actions {
		actions[k] = v
	}

	return &models.RoutingRule{
		Name:          r.Name,
		Description:   r.Description,
		Enabled:       enabled,
		Priority:      r.Priority,
		MatchCriteria: matchCriteria,
		Actions:       actions,
	}
}

// ApplyTo applies UpdateRoutingRuleRequest to an existing RoutingRule
func (r *UpdateRoutingRuleRequest) ApplyTo(rule *models.RoutingRule) {
	if r.Name != nil {
		rule.Name = *r.Name
	}
	if r.Description != nil {
		rule.Description = *r.Description
	}
	if r.Enabled != nil {
		rule.Enabled = *r.Enabled
	}
	if r.Priority != nil {
		rule.Priority = *r.Priority
	}
	if r.MatchCriteria != nil {
		matchCriteria := make(models.JSONB)
		for k, v := range r.MatchCriteria {
			matchCriteria[k] = v
		}
		rule.MatchCriteria = matchCriteria
	}
	if r.Actions != nil {
		actions := make(models.JSONB)
		for k, v := range r.Actions {
			actions[k] = v
		}
		rule.Actions = actions
	}
}
