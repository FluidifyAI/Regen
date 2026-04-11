package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/FluidifyAI/Regen/backend/internal/models"
)

// GroupingRuleResponse is the response body for grouping rule endpoints
type GroupingRuleResponse struct {
	ID                uuid.UUID  `json:"id"`
	Name              string     `json:"name"`
	Description       string     `json:"description"`
	Enabled           bool       `json:"enabled"`
	Priority          int        `json:"priority"`
	MatchLabels       JSONB      `json:"match_labels"`
	CrossSourceLabels []string   `json:"cross_source_labels"`
	TimeWindowSeconds int        `json:"time_window_seconds"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// ToGroupingRuleResponse converts models.GroupingRule to GroupingRuleResponse
func ToGroupingRuleResponse(rule *models.GroupingRule) GroupingRuleResponse {
	// Convert models.JSONB to dto.JSONB
	matchLabels := make(JSONB)
	for k, v := range rule.MatchLabels {
		matchLabels[k] = v
	}

	// Convert models.JSONBArray to []string
	crossSourceLabels := []string(rule.CrossSourceLabels)
	if crossSourceLabels == nil {
		crossSourceLabels = []string{} // Return empty array instead of null
	}

	return GroupingRuleResponse{
		ID:                rule.ID,
		Name:              rule.Name,
		Description:       rule.Description,
		Enabled:           rule.Enabled,
		Priority:          rule.Priority,
		MatchLabels:       matchLabels,
		CrossSourceLabels: crossSourceLabels,
		TimeWindowSeconds: rule.TimeWindowSeconds,
		CreatedAt:         rule.CreatedAt,
		UpdatedAt:         rule.UpdatedAt,
	}
}
