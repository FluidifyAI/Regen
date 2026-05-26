package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/FluidifyAI/Regen/backend/internal/api/middleware"
	"github.com/FluidifyAI/Regen/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

// SlackEvents handles POST /api/v1/slack/events — Slack Events API payloads.
// Signature verification is done by SlackSignatureVerification middleware upstream.
func SlackEvents(handler *services.SlackEventHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := middleware.SlackBodyFromContext(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
			return
		}

		// url_verification is Slack's one-time challenge when saving the Events URL.
		var challenge struct {
			Type      string `json:"type"`
			Challenge string `json:"challenge"`
		}
		if err := json.Unmarshal(body, &challenge); err == nil && challenge.Type == "url_verification" {
			c.JSON(http.StatusOK, gin.H{"challenge": challenge.Challenge})
			return
		}

		eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
		if err != nil {
			slog.Warn("slack events: failed to parse payload", "error", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid event payload"})
			return
		}

		// ACK immediately; dispatch is async inside HandleEventsAPI.
		c.Status(http.StatusOK)
		handler.HandleEventsAPI(eventsAPIEvent)
	}
}

// SlackInteractions handles POST /api/v1/slack/interactions — button clicks and modal submissions.
func SlackInteractions(handler *services.SlackEventHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := middleware.SlackBodyFromContext(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
			return
		}

		// Interactions arrive as application/x-www-form-urlencoded with a "payload" key.
		values, err := url.ParseQuery(string(body))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid form body"})
			return
		}
		payloadJSON := values.Get("payload")
		if payloadJSON == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing payload field"})
			return
		}

		var callback slack.InteractionCallback
		if err := json.Unmarshal([]byte(payloadJSON), &callback); err != nil {
			slog.Warn("slack interactions: failed to parse payload", "error", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid interaction payload"})
			return
		}

		// ACK with 200 immediately — Slack requires a response within 3 seconds.
		c.Status(http.StatusOK)
		go handler.HandleInteraction(callback)
	}
}

// SlackCommands handles POST /api/v1/slack/commands — slash command payloads.
func SlackCommands(handler *services.SlackEventHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := middleware.SlackBodyFromContext(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
			return
		}

		values, err := url.ParseQuery(string(body))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid form body"})
			return
		}

		cmd := slack.SlashCommand{
			Token:          values.Get("token"),
			TeamID:         values.Get("team_id"),
			TeamDomain:     values.Get("team_domain"),
			EnterpriseID:   values.Get("enterprise_id"),
			EnterpriseName: values.Get("enterprise_name"),
			ChannelID:      values.Get("channel_id"),
			ChannelName:    values.Get("channel_name"),
			UserID:         values.Get("user_id"),
			UserName:       values.Get("user_name"),
			Command:        values.Get("command"),
			Text:           values.Get("text"),
			ResponseURL:    values.Get("response_url"),
			TriggerID:      values.Get("trigger_id"),
		}

		// ACK with 200 immediately — Slack requires a response within 3 seconds.
		c.Status(http.StatusOK)
		go handler.HandleCommand(cmd)
	}
}
