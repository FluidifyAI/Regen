package dto

import (
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/google/uuid"
)

// AlertResponse is the standard alert response format
type AlertResponse struct {
	ID                   uuid.UUID              `json:"id"`
	ExternalID           string                 `json:"external_id"`
	Source               string                 `json:"source"`
	Fingerprint          string                 `json:"fingerprint,omitempty"`
	Status               string                 `json:"status"`
	Severity             string                 `json:"severity"`
	Title                string                 `json:"title"`
	Description          string                 `json:"description,omitempty"`
	Labels               map[string]interface{} `json:"labels,omitempty"`
	Annotations          map[string]interface{} `json:"annotations,omitempty"`
	EscalationPolicyID   *uuid.UUID             `json:"escalation_policy_id,omitempty"`
	AcknowledgmentStatus string                 `json:"acknowledgment_status"`
	StartedAt            time.Time              `json:"started_at"`
	EndedAt              *time.Time             `json:"ended_at,omitempty"`
	ReceivedAt           time.Time              `json:"received_at"`
}

// AlertFilters holds query parameters for filtering alerts
type AlertFilters struct {
	Source   string `form:"source"`
	Status   string `form:"status"   binding:"omitempty,oneof=firing resolved"`
	Severity string `form:"severity" binding:"omitempty,oneof=critical warning info"`
}

// ToRepository converts API alert filters to repository filters
func (f *AlertFilters) ToRepository() repository.AlertFilters {
	return repository.AlertFilters{
		Source:   f.Source,
		Status:   models.AlertStatus(f.Status),
		Severity: models.AlertSeverity(f.Severity),
	}
}

// ToAlertResponse converts a models.Alert to AlertResponse
func ToAlertResponse(alert *models.Alert) AlertResponse {
	return AlertResponse{
		ID:                   alert.ID,
		ExternalID:           alert.ExternalID,
		Source:               alert.Source,
		Fingerprint:          alert.Fingerprint,
		Status:               string(alert.Status),
		Severity:             string(alert.Severity),
		Title:                alert.Title,
		Description:          alert.Description,
		Labels:               alert.Labels,
		Annotations:          alert.Annotations,
		EscalationPolicyID:   alert.EscalationPolicyID,
		AcknowledgmentStatus: string(alert.AcknowledgmentStatus),
		StartedAt:            alert.StartedAt,
		EndedAt:              alert.EndedAt,
		ReceivedAt:           alert.ReceivedAt,
	}
}
