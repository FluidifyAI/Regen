package dto

import "github.com/google/uuid"

// CreateEscalationPolicyRequest is the request body for POST /api/v1/escalation-policies
type CreateEscalationPolicyRequest struct {
	Name        string `json:"name"        binding:"required"`
	Description string `json:"description"`
	Enabled     *bool  `json:"enabled"` // defaults to true if omitted
}

// UpdateEscalationPolicyRequest is the request body for PATCH /api/v1/escalation-policies/:id
type UpdateEscalationPolicyRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Enabled     *bool   `json:"enabled"`
}

// CreateEscalationTierRequest is the request body for POST /api/v1/escalation-policies/:id/tiers
type CreateEscalationTierRequest struct {
	TimeoutSeconds int        `json:"timeout_seconds" binding:"required,min=1"`
	TargetType     string     `json:"target_type"     binding:"required,oneof=schedule users both"`
	ScheduleID     *uuid.UUID `json:"schedule_id"`
	UserNames      []string   `json:"user_names"`
}

// UpdateEscalationTierRequest is the request body for PATCH /api/v1/escalation-policies/:id/tiers/:tier_id
type UpdateEscalationTierRequest struct {
	TimeoutSeconds *int       `json:"timeout_seconds" binding:"omitempty,min=1"`
	TargetType     *string    `json:"target_type"     binding:"omitempty,oneof=schedule users both"`
	ScheduleID     *uuid.UUID `json:"schedule_id"`
	UserNames      []string   `json:"user_names"`
}
