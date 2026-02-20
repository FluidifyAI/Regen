package dto

import (
	"time"

	"github.com/google/uuid"
)

// AISummaryResponse is the response body for POST /api/v1/incidents/:id/summarize.
type AISummaryResponse struct {
	// IncidentID is the UUID of the incident that was summarized.
	IncidentID uuid.UUID `json:"incident_id"`
	// Summary is the AI-generated summary text.
	Summary string `json:"summary"`
	// GeneratedAt is the timestamp when the summary was generated.
	GeneratedAt time.Time `json:"generated_at"`
	// Model identifies the AI model/provider used.
	Model string `json:"model"`
	// ContextSources lists the data sources included in the context (e.g. "timeline", "alerts", "slack_thread").
	ContextSources []string `json:"context_sources"`
}

// HandoffDigestResponse is the response body for POST /api/v1/incidents/:id/handoff-digest.
type HandoffDigestResponse struct {
	// IncidentID is the UUID of the incident.
	IncidentID uuid.UUID `json:"incident_id"`
	// IncidentTitle is the incident title for display convenience.
	IncidentTitle string `json:"incident_title"`
	// Status is the current incident status.
	Status string `json:"status"`
	// Severity is the current incident severity.
	Severity string `json:"severity"`
	// Digest is the AI-generated handoff document.
	Digest string `json:"digest"`
	// GeneratedAt is the timestamp when the digest was generated.
	GeneratedAt time.Time `json:"generated_at"`
}
