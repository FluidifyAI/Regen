package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/FluidifyAI/Regen/backend/enterprise"
	"github.com/FluidifyAI/Regen/backend/internal/api/handlers/dto"
	"github.com/FluidifyAI/Regen/backend/internal/api/middleware"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/FluidifyAI/Regen/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SummarizeIncident handles POST /api/v1/incidents/:id/summarize
func SummarizeIncident(incidentSvc services.IncidentService, aiSvc services.AIService, hooks enterprise.Hooks) gin.HandlerFunc {
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

		summary, usage, err := incidentSvc.GenerateAISummary(incident)
		if err != nil {
			slog.Error("failed to generate AI summary",
				"incident_id", incident.ID,
				"error", err,
				"request_id", c.GetString("request_id"),
			)
			dto.InternalError(c, err)
			return
		}

		var userID *uuid.UUID
		if u := middleware.GetLocalUser(c); u != nil {
			uid := u.ID
			userID = &uid
		}
		costUSD, _ := hooks.CostTracker.RecordUsage(c.Request.Context(), enterprise.UsageEvent{
			Operation:        "summarize",
			Model:            aiSvc.Model(),
			PromptTokens:     usage.PromptTokens,
			CompletionTokens: usage.CompletionTokens,
			OccurredAt:       time.Now().UTC(),
			UserID:           userID,
		})

		c.JSON(http.StatusOK, dto.AISummaryResponse{
			IncidentID:     incident.ID,
			Summary:        summary,
			GeneratedAt:    time.Now().UTC(),
			Model:          aiSvc.Model(),
			ContextSources: buildContextSources(incident.SlackChannelID != ""),
			CostUSD:        costUSD,
		})
	}
}

// GenerateHandoffDigest handles POST /api/v1/incidents/:id/handoff-digest
func GenerateHandoffDigest(incidentSvc services.IncidentService, aiSvc services.AIService, hooks enterprise.Hooks) gin.HandlerFunc {
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

		digest, usage, err := incidentSvc.GenerateHandoffDigest(incident)
		if err != nil {
			slog.Error("failed to generate handoff digest",
				"incident_id", incident.ID,
				"error", err,
				"request_id", c.GetString("request_id"),
			)
			dto.InternalError(c, err)
			return
		}

		var userID *uuid.UUID
		if u := middleware.GetLocalUser(c); u != nil {
			uid := u.ID
			userID = &uid
		}
		costUSD, _ := hooks.CostTracker.RecordUsage(c.Request.Context(), enterprise.UsageEvent{
			Operation:        "handoff",
			Model:            aiSvc.Model(),
			PromptTokens:     usage.PromptTokens,
			CompletionTokens: usage.CompletionTokens,
			OccurredAt:       time.Now().UTC(),
			UserID:           userID,
		})

		c.JSON(http.StatusOK, dto.HandoffDigestResponse{
			IncidentID:    incident.ID,
			Digest:        digest,
			IncidentTitle: incident.Title,
			Status:        string(incident.Status),
			Severity:      string(incident.Severity),
			GeneratedAt:   time.Now().UTC(),
			CostUSD:       costUSD,
		})
	}
}

// GetAISettings handles GET /api/v1/settings/ai
func GetAISettings(aiSvc services.AIService) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"enabled": aiSvc.IsEnabled(),
		})
	}
}

// EnhanceIncidentDraft handles POST /api/v1/ai/enhance-draft
func EnhanceIncidentDraft(aiSvc services.AIService, hooks enterprise.Hooks) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !aiSvc.IsEnabled() {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": gin.H{
					"code":    "ai_not_configured",
					"message": "AI features are not configured. Set your OpenAI key in Settings → System.",
				},
			})
			return
		}
		var req struct {
			Brief string `json:"brief" binding:"required,min=5,max=1000"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			dto.BadRequest(c, "brief is required (5-1000 characters)", nil)
			return
		}
		title, summary, usage, err := aiSvc.EnhanceIncidentDraft(c.Request.Context(), req.Brief)
		if err != nil {
			slog.Error("failed to enhance incident draft", "error", err, "request_id", c.GetString("request_id"))
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{"code": "ai_error", "message": err.Error()},
			})
			return
		}

		var userID *uuid.UUID
		if u := middleware.GetLocalUser(c); u != nil {
			uid := u.ID
			userID = &uid
		}
		costUSD, _ := hooks.CostTracker.RecordUsage(c.Request.Context(), enterprise.UsageEvent{
			Operation:        "enhance_draft",
			Model:            aiSvc.Model(),
			PromptTokens:     usage.PromptTokens,
			CompletionTokens: usage.CompletionTokens,
			OccurredAt:       time.Now().UTC(),
			UserID:           userID,
		})

		c.JSON(http.StatusOK, gin.H{"title": title, "summary": summary, "cost_usd": costUSD})
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
