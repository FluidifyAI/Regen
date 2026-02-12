package dto

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/models"
)

// IncidentResponse is the standard incident response format
type IncidentResponse struct {
	ID             uuid.UUID         `json:"id"`
	IncidentNumber int               `json:"incident_number"`
	Title          string            `json:"title"`
	Slug           string            `json:"slug"`
	Status         string            `json:"status"`
	Severity       string            `json:"severity"`
	Summary        string            `json:"summary,omitempty"`
	GroupKey       *string           `json:"group_key,omitempty"`
	SlackChannel   *SlackChannelInfo `json:"slack_channel,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	TriggeredAt    time.Time         `json:"triggered_at"`
	AcknowledgedAt *time.Time        `json:"acknowledged_at,omitempty"`
	ResolvedAt     *time.Time        `json:"resolved_at,omitempty"`
	CreatedByType  string            `json:"created_by_type"`
	CreatedByID    string            `json:"created_by_id,omitempty"`
	CommanderID    *uuid.UUID        `json:"commander_id,omitempty"`
}

// SlackChannelInfo contains Slack channel details
type SlackChannelInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// IncidentDetailResponse extends IncidentResponse with related data
type IncidentDetailResponse struct {
	IncidentResponse
	Alerts   []AlertSummary         `json:"alerts"`
	Timeline []TimelineEntrySummary `json:"timeline"`
}

// AlertSummary is a minimal alert representation for incident details
type AlertSummary struct {
	ID         uuid.UUID              `json:"id"`
	Title      string                 `json:"title"`
	Source     string                 `json:"source"`
	Severity   string                 `json:"severity"`
	Status     string                 `json:"status"`
	Labels     map[string]interface{} `json:"labels,omitempty"`
	ReceivedAt time.Time              `json:"received_at"`
}

// TimelineEntrySummary is a minimal timeline entry for incident details
type TimelineEntrySummary struct {
	ID        uuid.UUID              `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Type      string                 `json:"type"`
	ActorType string                 `json:"actor_type"`
	ActorID   string                 `json:"actor_id,omitempty"`
	Content   map[string]interface{} `json:"content"`
}

// ToIncidentResponse converts a models.Incident to IncidentResponse
func ToIncidentResponse(incident *models.Incident) IncidentResponse {
	resp := IncidentResponse{
		ID:             incident.ID,
		IncidentNumber: incident.IncidentNumber,
		Title:          incident.Title,
		Slug:           incident.Slug,
		Status:         string(incident.Status),
		Severity:       string(incident.Severity),
		Summary:        incident.Summary,
		GroupKey:       incident.GroupKey,
		CreatedAt:      incident.CreatedAt,
		TriggeredAt:    incident.TriggeredAt,
		AcknowledgedAt: incident.AcknowledgedAt,
		ResolvedAt:     incident.ResolvedAt,
		CreatedByType:  incident.CreatedByType,
		CreatedByID:    incident.CreatedByID,
		CommanderID:    incident.CommanderID,
	}

	// Add Slack channel info if available
	if incident.SlackChannelID != "" {
		resp.SlackChannel = &SlackChannelInfo{
			ID:   incident.SlackChannelID,
			Name: incident.SlackChannelName,
			URL:  fmt.Sprintf("https://slack.com/app_redirect?channel=%s", incident.SlackChannelID),
		}
	}

	return resp
}
