package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/repository"
)

// TeamsEventHandler processes inbound Bot Framework activities from Teams.
// It is wired to POST /api/v1/webhooks/teams by the Gin router.
// JWT authentication is handled by middleware/teams_auth.go before this handler is called.
type TeamsEventHandler struct {
	incidentSvc IncidentService
	incidentRepo repository.IncidentRepository
	teamsSvc     *TeamsService
	appID        string // Bot's Azure App ID — used to filter out self-echoed messages
}

// NewTeamsEventHandler creates a handler for inbound Teams Bot Framework activities.
func NewTeamsEventHandler(appID string, incidentSvc IncidentService, incidentRepo repository.IncidentRepository, teamsSvc *TeamsService) *TeamsEventHandler {
	return &TeamsEventHandler{
		appID:        appID,
		incidentSvc:  incidentSvc,
		incidentRepo: incidentRepo,
		teamsSvc:     teamsSvc,
	}
}

// BotActivity is a Bot Framework Activity JSON object (simplified subset we use).
type BotActivity struct {
	Type           string          `json:"type"`
	ID             string          `json:"id"`
	Text           string          `json:"text"`
	ChannelID      string          `json:"channelId"`  // "msteams"
	Conversation   BotConversation `json:"conversation"`
	From           BotAccount      `json:"from"`
	Recipient      BotAccount      `json:"recipient"`
	ServiceURL     string          `json:"serviceUrl"`
	ChannelData    json.RawMessage `json:"channelData"`
}

type BotConversation struct {
	ID      string `json:"id"`
	IsGroup bool   `json:"isGroup"`
}

type BotAccount struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Handle processes an inbound Bot Framework activity payload.
// The caller (Gin handler) decodes the JSON body and passes it here.
func (h *TeamsEventHandler) Handle(ctx context.Context, activity BotActivity) {
	// Only handle message activities
	if activity.Type != "message" {
		return
	}

	// Ignore messages sent by the bot itself (echo prevention)
	if strings.EqualFold(activity.From.ID, h.appID) {
		return
	}

	text := strings.TrimSpace(activity.Text)

	// Strip @-mention HTML that Teams injects: "<at>BotName</at> new title"
	// The text after stripping typically looks like "@BotName new title" — remove leading @word
	text = stripAtMention(text)

	parts := strings.Fields(text)
	if len(parts) == 0 {
		h.sendHelp(ctx, activity)
		return
	}

	command := strings.ToLower(parts[0])
	args := strings.TrimSpace(strings.TrimPrefix(text, parts[0]))

	switch command {
	case "new", "create":
		h.handleNew(ctx, activity, args)
	case "ack", "acknowledge":
		h.handleAck(ctx, activity)
	case "resolve":
		h.handleResolve(ctx, activity)
	case "status":
		h.handleStatus(ctx, activity)
	default:
		h.sendHelp(ctx, activity)
	}
}

// handleNew creates a new incident from a Teams message.
// Usage: @BotName new <title>
func (h *TeamsEventHandler) handleNew(ctx context.Context, activity BotActivity, title string) {
	if title == "" {
		h.reply(ctx, activity, "Usage: `new <incident title>`\nExample: `new Production database is down`")
		return
	}

	req := &CreateIncidentParams{
		Title:     title,
		Severity:  "medium",
		CreatedBy: activity.From.ID,
	}
	incident, err := h.incidentSvc.CreateIncident(req)
	if err != nil {
		slog.Error("teams bot: failed to create incident", "error", err)
		h.reply(ctx, activity, fmt.Sprintf("❌ Failed to create incident: %s", err))
		return
	}

	h.reply(ctx, activity, fmt.Sprintf("✅ Created INC-%d: **%s**\n\nA dedicated Teams channel has been created for this incident.",
		incident.IncidentNumber, incident.Title))
}

// handleAck acknowledges the incident linked to the current Teams channel.
func (h *TeamsEventHandler) handleAck(ctx context.Context, activity BotActivity) {
	incident, err := h.incidentRepo.GetByTeamsChannelID(activity.Conversation.ID)
	if err != nil || incident == nil {
		h.reply(ctx, activity, "⚠️ No incident found for this channel.")
		return
	}
	if incident.Status != models.IncidentStatusTriggered {
		h.reply(ctx, activity, fmt.Sprintf("⚠️ Incident is already **%s**.", incident.Status))
		return
	}

	if err := h.incidentSvc.AcknowledgeIncident(incident.ID, "teams_bot", activity.From.ID); err != nil {
		slog.Error("teams bot: failed to acknowledge incident", "incident_id", incident.ID, "error", err)
		h.reply(ctx, activity, fmt.Sprintf("❌ Failed to acknowledge: %s", err))
		return
	}
	h.reply(ctx, activity, fmt.Sprintf("🟡 INC-%d acknowledged by <@%s>", incident.IncidentNumber, activity.From.Name))
}

// handleResolve resolves the incident linked to the current Teams channel.
func (h *TeamsEventHandler) handleResolve(ctx context.Context, activity BotActivity) {
	incident, err := h.incidentRepo.GetByTeamsChannelID(activity.Conversation.ID)
	if err != nil || incident == nil {
		h.reply(ctx, activity, "⚠️ No incident found for this channel.")
		return
	}
	if incident.Status == models.IncidentStatusResolved {
		h.reply(ctx, activity, "⚠️ Incident is already resolved.")
		return
	}

	if err := h.incidentSvc.ResolveIncident(incident.ID, "teams_bot", activity.From.ID); err != nil {
		slog.Error("teams bot: failed to resolve incident", "incident_id", incident.ID, "error", err)
		h.reply(ctx, activity, fmt.Sprintf("❌ Failed to resolve: %s", err))
		return
	}
	h.reply(ctx, activity, fmt.Sprintf("✅ INC-%d resolved by %s", incident.IncidentNumber, activity.From.Name))
}

// handleStatus posts the current incident status card in the channel.
func (h *TeamsEventHandler) handleStatus(ctx context.Context, activity BotActivity) {
	incident, err := h.incidentRepo.GetByTeamsChannelID(activity.Conversation.ID)
	if err != nil || incident == nil {
		h.reply(ctx, activity, "⚠️ No incident found for this channel.")
		return
	}
	card := teamsIncidentCard(incident)
	msg := Message{Blocks: []interface{}{card}}
	if _, err := h.teamsSvc.PostMessage(activity.Conversation.ID, msg); err != nil {
		slog.Error("teams bot: failed to post status card", "error", err)
	}
}

func (h *TeamsEventHandler) sendHelp(ctx context.Context, activity BotActivity) {
	help := "**OpenIncident Bot Commands:**\n\n" +
		"• `new <title>` — Create a new incident\n" +
		"• `ack` — Acknowledge this channel's incident\n" +
		"• `resolve` — Resolve this channel's incident\n" +
		"• `status` — Show current incident status card"
	h.reply(ctx, activity, help)
}

// reply sends a plain-text message back to the channel where the activity arrived.
func (h *TeamsEventHandler) reply(ctx context.Context, activity BotActivity, text string) {
	msg := Message{Text: text}
	if _, err := h.teamsSvc.PostMessage(activity.Conversation.ID, msg); err != nil {
		slog.Error("teams bot: failed to send reply", "error", err)
	}
}

// stripAtMention removes the leading @-mention that Teams injects into bot messages.
// Teams sends text like "<at>BotName</at> new title" which after HTML stripping becomes
// "BotName new title". We strip the leading word if it looks like a display name.
func stripAtMention(text string) string {
	// Remove HTML <at> tags that may be present in raw text
	text = strings.ReplaceAll(text, "<at>", "")
	text = strings.ReplaceAll(text, "</at>", "")
	text = strings.TrimSpace(text)

	// If the text starts with a known bot name pattern, strip it.
	// We strip the first word only if it doesn't look like a command itself.
	commands := map[string]bool{"new": true, "create": true, "ack": true, "acknowledge": true, "resolve": true, "status": true, "help": true}
	parts := strings.Fields(text)
	if len(parts) >= 2 && !commands[strings.ToLower(parts[0])] {
		// First word is likely a bot name — drop it
		return strings.TrimSpace(strings.TrimPrefix(text, parts[0]))
	}
	return text
}
