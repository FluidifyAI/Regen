package dto

import (
	"time"

	"github.com/google/uuid"
)

// AISummaryResponse is the response body for POST /api/v1/incidents/:id/summarize.
type AISummaryResponse struct {
	IncidentID     uuid.UUID `json:"incident_id"`
	Summary        string    `json:"summary"`
	GeneratedAt    time.Time `json:"generated_at"`
	Model          string    `json:"model"`
	ContextSources []string  `json:"context_sources"`
	CostUSD        float64   `json:"cost_usd"`
}

// HandoffDigestResponse is the response body for POST /api/v1/incidents/:id/handoff-digest.
type HandoffDigestResponse struct {
	IncidentID    uuid.UUID `json:"incident_id"`
	IncidentTitle string    `json:"incident_title"`
	Status        string    `json:"status"`
	Severity      string    `json:"severity"`
	Digest        string    `json:"digest"`
	GeneratedAt   time.Time `json:"generated_at"`
	CostUSD       float64   `json:"cost_usd"`
}
