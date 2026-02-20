package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openincident/openincident/internal/api/handlers/dto"
	"github.com/openincident/openincident/internal/repository"
	"github.com/openincident/openincident/internal/services"
)

// SummarizeIncident handles POST /api/v1/incidents/:id/summarize
//
// Generates an AI-powered summary of the incident using timeline, alert, and
// Slack thread context. The summary is persisted on the incident and returned.
// Returns 503 if OpenAI is not configured (OPENAI_API_KEY not set).
func SummarizeIncident(incidentSvc services.IncidentService, aiSvc services.AIService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !aiSvc.IsEnabled() {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": gin.H{
					"code":    "ai_not_configured",
					"message": "AI features are not configured. Set the OPENAI_API_KEY environment variable to enable them.",
				},
			})
			return
		}

		idParam := c.Param("id")
		uid, num, err := parseIncidentIdentifier(idParam)
		if err != nil {
			dto.BadRequest(c, "Invalid incident identifier", map[string]interface{}{
				"id": "must be a valid UUID or incident number",
			})
			return
		}

		incident, err := incidentSvc.GetIncident(uid, num)
		if err != nil {
			if _, ok := err.(*repository.NotFoundError); ok {
				dto.NotFound(c, "incident", idParam)
				return
			}
			dto.InternalError(c, err)
			return
		}

		summary, err := incidentSvc.GenerateAISummary(incident)
		if err != nil {
			slog.Error("failed to generate AI summary",
				"incident_id", incident.ID,
				"error", err,
				"request_id", c.GetString("request_id"),
			)
			dto.InternalError(c, err)
			return
		}

		c.JSON(http.StatusOK, dto.AISummaryResponse{
			IncidentID:     incident.ID,
			Summary:        summary,
			GeneratedAt:    time.Now().UTC(),
			Model:          "openai",
			ContextSources: buildContextSources(incident.SlackChannelID != ""),
		})
	}
}

// GenerateHandoffDigest handles POST /api/v1/incidents/:id/handoff-digest
//
// Generates a structured shift handoff document for the incoming on-call engineer.
// The digest is not persisted — callers can post it to Slack or display in the UI.
func GenerateHandoffDigest(incidentSvc services.IncidentService, aiSvc services.AIService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !aiSvc.IsEnabled() {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": gin.H{
					"code":    "ai_not_configured",
					"message": "AI features are not configured. Set the OPENAI_API_KEY environment variable to enable them.",
				},
			})
			return
		}

		idParam := c.Param("id")
		uid, num, err := parseIncidentIdentifier(idParam)
		if err != nil {
			dto.BadRequest(c, "Invalid incident identifier", map[string]interface{}{
				"id": "must be a valid UUID or incident number",
			})
			return
		}

		incident, err := incidentSvc.GetIncident(uid, num)
		if err != nil {
			if _, ok := err.(*repository.NotFoundError); ok {
				dto.NotFound(c, "incident", idParam)
				return
			}
			dto.InternalError(c, err)
			return
		}

		digest, err := incidentSvc.GenerateHandoffDigest(incident)
		if err != nil {
			slog.Error("failed to generate handoff digest",
				"incident_id", incident.ID,
				"error", err,
				"request_id", c.GetString("request_id"),
			)
			dto.InternalError(c, err)
			return
		}

		c.JSON(http.StatusOK, dto.HandoffDigestResponse{
			IncidentID:    incident.ID,
			Digest:        digest,
			IncidentTitle: incident.Title,
			Status:        string(incident.Status),
			Severity:      string(incident.Severity),
			GeneratedAt:   time.Now().UTC(),
		})
	}
}

// GetAISettings handles GET /api/v1/settings/ai
//
// Returns whether AI features are enabled. Frontend uses this to show/hide AI controls.
func GetAISettings(aiSvc services.AIService) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"enabled": aiSvc.IsEnabled(),
		})
	}
}

// buildContextSources returns which context sources are included in the summary.
func buildContextSources(hasSlack bool) []string {
	sources := []string{"timeline", "alerts"}
	if hasSlack {
		sources = append(sources, "slack_thread")
	}
	return sources
}
