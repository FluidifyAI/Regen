package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/fluidify/regen/internal/services"
)

// TeamsWebhook receives Bot Framework activity payloads from Microsoft Teams
// and delegates processing to the TeamsEventHandler.
//
// Route: POST /api/v1/webhooks/teams
// Auth:  Handled by middleware.TeamsAuth before this handler
func TeamsWebhook(handler *services.TeamsEventHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var activity services.BotActivity
		if err := c.ShouldBindJSON(&activity); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid activity payload"})
			return
		}

		// Process asynchronously so we return 200 to Microsoft quickly.
		// The Bot Framework requires a response within 5 seconds or it retries.
		// Use context.Background() — the request context is cancelled as soon as
		// this handler returns, which would abort all downstream DB and HTTP calls.
		go handler.Handle(context.Background(), activity)

		c.Status(http.StatusOK)
	}
}

// GetTeamsSettings returns the current Teams integration configuration status.
// Does not expose credentials — only whether Teams is configured.
//
// Route: GET /api/v1/settings/teams
func GetTeamsSettings(teamsSvc *services.TeamsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		enabled := teamsSvc != nil
		c.JSON(http.StatusOK, gin.H{
			"enabled": enabled,
		})
	}
}
