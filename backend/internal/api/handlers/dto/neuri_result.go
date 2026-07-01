package dto

import "encoding/json"

type NeuriResultRequest struct {
	IncidentID         string          `json:"incident_id"          binding:"required"`
	InvestigationRunID string          `json:"investigation_run_id" binding:"required"`
	TopHypothesis      string          `json:"top_hypothesis"       binding:"required"`
	Confidence         float64         `json:"confidence"           binding:"required,min=0,max=1"`
	Summary            string          `json:"summary"              binding:"required"`
	RankedHypotheses   json.RawMessage `json:"ranked_hypotheses"`
}

type NeuriResultResponse struct {
	ID         string `json:"id"`
	IncidentID string `json:"incident_id"`
}

type NeuriSettingsResponse struct {
	WebhookURL         string `json:"webhook_url"`
	RegenBaseURL       string `json:"regen_base_url"`
	WebhookSecretSet   bool   `json:"webhook_secret_set"`
	WebhookSecretHint  string `json:"webhook_secret_hint,omitempty"` // last 4 chars only
}

type NeuriSettingsRequest struct {
	WebhookURL    string `json:"webhook_url"`
	RegenBaseURL  string `json:"regen_base_url"`
	WebhookSecret string `json:"webhook_secret"`
}

type NeuriTriggerRequest struct {
	IncidentID string `json:"incident_id" binding:"required"`
}

type NeuriTriggerResponse struct {
	Status string `json:"status"` // "accepted"
}
