package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/fluidify/regen/internal/models"
	"github.com/fluidify/regen/internal/repository"
)

// TeamsEventHandler processes inbound Bot Framework activities from Teams.
// It is wired to POST /api/v1/webhooks/teams by the Gin router.
// JWT authentication is handled by middleware/teams_auth.go before this handler is called.
type TeamsEventHandler struct {
	incidentSvc  IncidentService
	incidentRepo repository.IncidentRepository
	timelineRepo repository.TimelineRepository
	teamsSvc     *TeamsService
	appID        string // Bot's Azure App ID — used to filter out self-echoed messages
}

// NewTeamsEventHandler creates a handler for inbound Teams Bot Framework activities.
func NewTeamsEventHandler(appID string, incidentSvc IncidentService, incidentRepo repository.IncidentRepository, timelineRepo repository.TimelineRepository, teamsSvc *TeamsService) *TeamsEventHandler {
	return &TeamsEventHandler{
		appID:        appID,
		incidentSvc:  incidentSvc,
		incidentRepo: incidentRepo,
		timelineRepo: timelineRepo,
		teamsSvc:     teamsSvc,
	}
}

// BotActivity is a Bot Framework Activity JSON object (simplified subset we use).
type BotActivity struct {
	Type         string          `json:"type"`
	ID           string          `json:"id"`
	Text         string          `json:"text"`
	ChannelID    string          `json:"channelId"`  // "msteams"
	Conversation BotConversation `json:"conversation"`
	From         BotAccount      `json:"from"`
	Recipient    BotAccount      `json:"recipient"`
	ServiceURL   string          `json:"serviceUrl"`
	ChannelData  json.RawMessage `json:"channelData"`
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
func (h *TeamsEventHandler) Handle(ctx context.Context, activity BotActivity) {
	switch activity.Type {
	case "message":
		h.handleMessage(ctx, activity)
	case "conversationUpdate":
		// Bot was added to a team or channel. Log for diagnostics; the serviceUrl
		// here confirms the correct TEAMS_SERVICE_URL for the tenant/region.
		slog.Info("teams bot: conversationUpdate received",
			"service_url", activity.ServiceURL,
			"conversation_id", activity.Conversation.ID)
	default:
		// installationUpdate, typing, etc. — safely ignored.
	}
}

func (h *TeamsEventHandler) handleMessage(ctx context.Context, activity BotActivity) {
	// Ignore messages sent by the bot itself (echo prevention)
	if strings.EqualFold(activity.From.ID, h.appID) {
		return
	}

	text := strings.TrimSpace(activity.Text)
	text = stripAtMention(text)

	parts := strings.Fields(text)
	if len(parts) == 0 {
		h.sendHelp(activity)
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
		// Not a bot command — sync the message to the incident timeline (inbound parity with Slack)
		h.syncMessageToTimeline(ctx, activity, text)
	}
}

// handleNew creates a new incident from a Teams message.
func (h *TeamsEventHandler) handleNew(_ context.Context, activity BotActivity, title string) {
	if title == "" {
		h.reply(activity, "Usage: `new <incident title>`\nExample: `new Production database is down`")
		return
	}

	req := &CreateIncidentParams{
		Title:     title,
		Severity:  "medium",
		CreatedBy: activity.From.ID,
	}
	incident, err := h.incidentSvc.CreateIncident(req)
	if err != nil {
		slog.Error("teams bot: failed to create incident",
			"title", title,
			"requested_by", activity.From.ID,
			"error", err)
		h.reply(activity, "❌ Failed to create incident. The error has been recorded in system logs.")
		return
	}

	h.reply(activity, fmt.Sprintf("✅ Created INC-%d: **%s**\n\nA dedicated Teams channel has been created for this incident.",
		incident.IncidentNumber, incident.Title))
}

// lookupIncidentByConversation fetches the incident linked to the Bot Framework conversation.
// Bot Framework activity.Conversation.ID contains the conversation ID (a:xxx) which maps to
// teams_conversation_id — distinct from the Teams channel ID in teams_channel_id.
// Returns (nil, nil) when no incident is linked (normal for unlinked channels).
func (h *TeamsEventHandler) lookupIncidentByConversation(activity BotActivity) (*models.Incident, error) {
	incident, err := h.incidentRepo.GetByTeamsConversationID(activity.Conversation.ID)
	if err != nil {
		slog.Error("teams bot: database error looking up incident",
			"conversation_id", activity.Conversation.ID,
			"error", err)
		return nil, err
	}
	return incident, nil
}

// handleAck acknowledges the incident linked to the current Teams channel.
func (h *TeamsEventHandler) handleAck(_ context.Context, activity BotActivity) {
	incident, err := h.lookupIncidentByConversation(activity)
	if err != nil {
		h.reply(activity, "Internal error. Please try again or check system logs.")
		return
	}
	if incident == nil {
		h.reply(activity, "⚠️ No incident is linked to this channel.")
		return
	}
	if incident.Status != models.IncidentStatusTriggered {
		h.reply(activity, fmt.Sprintf("⚠️ Incident is already **%s**.", incident.Status))
		return
	}
	if err := h.incidentSvc.AcknowledgeIncident(incident.ID, "teams_bot", activity.From.ID); err != nil {
		slog.Error("teams bot: failed to acknowledge incident", "incident_id", incident.ID, "error", err)
		h.reply(activity, "❌ Failed to acknowledge incident. The error has been recorded in system logs.")
		return
	}
	h.reply(activity, fmt.Sprintf("🟡 INC-%d acknowledged by %s", incident.IncidentNumber, activity.From.Name))
}

// handleResolve resolves the incident linked to the current Teams channel.
func (h *TeamsEventHandler) handleResolve(_ context.Context, activity BotActivity) {
	incident, err := h.lookupIncidentByConversation(activity)
	if err != nil {
		h.reply(activity, "Internal error. Please try again or check system logs.")
		return
	}
	if incident == nil {
		h.reply(activity, "⚠️ No incident is linked to this channel.")
		return
	}
	if incident.Status == models.IncidentStatusResolved {
		h.reply(activity, "⚠️ Incident is already resolved.")
		return
	}
	if err := h.incidentSvc.ResolveIncident(incident.ID, "teams_bot", activity.From.ID); err != nil {
		slog.Error("teams bot: failed to resolve incident", "incident_id", incident.ID, "error", err)
		h.reply(activity, "❌ Failed to resolve incident. The error has been recorded in system logs.")
		return
	}
	h.reply(activity, fmt.Sprintf("✅ INC-%d resolved by %s", incident.IncidentNumber, activity.From.Name))
}

// handleStatus posts the current incident status card in the channel.
func (h *TeamsEventHandler) handleStatus(_ context.Context, activity BotActivity) {
	incident, err := h.lookupIncidentByConversation(activity)
	if err != nil {
		h.reply(activity, "Internal error. Please try again or check system logs.")
		return
	}
	if incident == nil {
		h.reply(activity, "⚠️ No incident is linked to this channel.")
		return
	}
	card := teamsIncidentCard(incident)
	msg := Message{Blocks: []interface{}{card}}
	if _, err := h.teamsSvc.PostToConversation(activity.Conversation.ID, msg); err != nil {
		slog.Error("teams bot: failed to post status card",
			"incident_id", incident.ID,
			"conversation_id", activity.Conversation.ID,
			"error", err)
		h.reply(activity, "Failed to retrieve incident status. Please try again.")
	}
}

// syncMessageToTimeline saves a non-command Teams message as a timeline entry
// for the incident associated with the channel. This gives inbound parity with
// Slack Socket Mode: messages posted in the Teams channel appear in the UI timeline.
func (h *TeamsEventHandler) syncMessageToTimeline(_ context.Context, activity BotActivity, text string) {
	if text == "" {
		return
	}
	incident, err := h.incidentRepo.GetByTeamsConversationID(activity.Conversation.ID)
	if err != nil {
		slog.Warn("teams bot: database error syncing message to timeline",
			"conversation_id", activity.Conversation.ID, "error", err)
		return
	}
	if incident == nil {
		// Conversation not linked to an incident — normal for channels created outside Fluidify Regen
		return
	}

	entry := &models.TimelineEntry{
		ID:         uuid.New(),
		IncidentID: incident.ID,
		Type:       "message",
		ActorType:  "teams_user",
		ActorID:    activity.From.ID,
		Content: models.JSONB{
			"message": text,
			"source":  "teams",
			"user":    activity.From.Name,
		},
	}
	if err := h.timelineRepo.Create(entry); err != nil {
		slog.Warn("teams bot: failed to sync message to timeline",
			"incident_id", incident.ID,
			"error", err)
	}
}

func (h *TeamsEventHandler) sendHelp(activity BotActivity) {
	help := "**Fluidify Regen Bot Commands:**\n\n" +
		"• `new <title>` — Create a new incident\n" +
		"• `ack` — Acknowledge this channel's incident\n" +
		"• `resolve` — Resolve this channel's incident\n" +
		"• `status` — Show current incident status card"
	h.reply(activity, help)
}

// reply sends a message back to the conversation where the activity arrived,
// using Bot Framework Proactive Messaging via the conversation ID from the inbound
// activity. This is equivalent to Slack's chat.postMessage with thread_ts.
func (h *TeamsEventHandler) reply(activity BotActivity, text string) {
	msg := Message{Text: text}
	// Use the conversation ID from the inbound activity directly — no need to
	// create a new conversation since we're replying to an existing one.
	if _, err := h.teamsSvc.PostToConversation(activity.Conversation.ID, msg); err != nil {
		slog.Error("teams bot: failed to send reply",
			"conversation_id", activity.Conversation.ID,
			"text_length", len(text),
			"error", err)
	}
}

// stripAtMention removes the leading @-mention that Teams injects into bot messages.
func stripAtMention(text string) string {
	text = strings.ReplaceAll(text, "<at>", "")
	text = strings.ReplaceAll(text, "</at>", "")
	text = strings.TrimSpace(text)

	commands := map[string]bool{"new": true, "create": true, "ack": true, "acknowledge": true, "resolve": true, "status": true, "help": true}
	parts := strings.Fields(text)
	if len(parts) >= 2 && !commands[strings.ToLower(parts[0])] {
		return strings.TrimSpace(strings.TrimPrefix(text, parts[0]))
	}
	return text
}
