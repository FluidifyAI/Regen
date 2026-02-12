package dto

import (
	"github.com/openincident/openincident/internal/models"
)

// CreateGroupingRuleRequest is the request body for POST /api/v1/grouping-rules
type CreateGroupingRuleRequest struct {
	Name              string   `json:"name" binding:"required,min=1,max=255"`
	Description       string   `json:"description" binding:"max=1000"`
	Enabled           *bool    `json:"enabled"` // Pointer to distinguish between false and not set
	Priority          int      `json:"priority" binding:"required,min=1,max=1000"`
	MatchLabels       JSONB    `json:"match_labels" binding:"required"`
	CrossSourceLabels []string `json:"cross_source_labels"`
	TimeWindowSeconds int      `json:"time_window_seconds" binding:"required,min=1,max=86400"` // 1 second to 24 hours
}

// UpdateGroupingRuleRequest is the request body for PUT /api/v1/grouping-rules/:id
type UpdateGroupingRuleRequest struct {
	Name              *string  `json:"name" binding:"omitempty,min=1,max=255"`
	Description       *string  `json:"description" binding:"omitempty,max=1000"`
	Enabled           *bool    `json:"enabled"`
	Priority          *int     `json:"priority" binding:"omitempty,min=1,max=1000"`
	MatchLabels       JSONB    `json:"match_labels"`
	CrossSourceLabels []string `json:"cross_source_labels"`
	TimeWindowSeconds *int     `json:"time_window_seconds" binding:"omitempty,min=1,max=86400"`
}

// JSONB is a helper type for JSON objects in request/response
type JSONB map[string]interface{}

// ToModel converts CreateGroupingRuleRequest to models.GroupingRule
func (r *CreateGroupingRuleRequest) ToModel() *models.GroupingRule {
	enabled := true // Default to enabled
	if r.Enabled != nil {
		enabled = *r.Enabled
	}

	// Convert JSONB to models.JSONB
	matchLabels := make(models.JSONB)
	for k, v := range r.MatchLabels {
		matchLabels[k] = v
	}

	// Convert []string to models.JSONBArray
	crossSourceLabels := models.JSONBArray(r.CrossSourceLabels)

	return &models.GroupingRule{
		Name:              r.Name,
		Description:       r.Description,
		Enabled:           enabled,
		Priority:          r.Priority,
		MatchLabels:       matchLabels,
		CrossSourceLabels: crossSourceLabels,
		TimeWindowSeconds: r.TimeWindowSeconds,
	}
}

// ApplyTo applies UpdateGroupingRuleRequest to an existing GroupingRule
func (r *UpdateGroupingRuleRequest) ApplyTo(rule *models.GroupingRule) {
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
	if r.MatchLabels != nil {
		matchLabels := make(models.JSONB)
		for k, v := range r.MatchLabels {
			matchLabels[k] = v
		}
		rule.MatchLabels = matchLabels
	}
	if r.CrossSourceLabels != nil {
		rule.CrossSourceLabels = models.JSONBArray(r.CrossSourceLabels)
	}
	if r.TimeWindowSeconds != nil {
		rule.TimeWindowSeconds = *r.TimeWindowSeconds
	}
}
