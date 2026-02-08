package services

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/repository"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// validSlackTransitions mirrors the state machine in the HTTP handler.
// Duplicated here to avoid coupling the service layer to the HTTP handler package.
var validSlackTransitions = map[models.IncidentStatus][]models.IncidentStatus{
	models.IncidentStatusTriggered: {
		models.IncidentStatusAcknowledged,
		models.IncidentStatusResolved,
	},
	models.IncidentStatusAcknowledged: {
		models.IncidentStatusResolved,
	},
	models.IncidentStatusResolved: {},
	models.IncidentStatusCanceled: {},
}

// SlackEventHandler listens for Slack events via Socket Mode (WebSocket) and
// dispatches them to the appropriate handlers. Socket Mode avoids needing a
// public URL or SSL certificate — it uses an outbound WebSocket connection.
type SlackEventHandler struct {
	client          *socketmode.Client
	incidentService IncidentService
	chatService     ChatService
	botUserID       string
}

// NewSlackEventHandler creates a Socket Mode event handler.
// Requires both SLACK_APP_TOKEN (xapp-...) and SLACK_BOT_TOKEN (xoxb-...).
func NewSlackEventHandler(
	appToken string,
	botToken string,
	incidentService IncidentService,
	chatService ChatService,
) (*SlackEventHandler, error) {
	if appToken == "" {
		return nil, fmt.Errorf("SLACK_APP_TOKEN is required for Socket Mode")
	}
	if botToken == "" {
		return nil, fmt.Errorf("SLACK_BOT_TOKEN is required for Socket Mode")
	}

	api := slack.New(
		botToken,
		slack.OptionAppLevelToken(appToken),
	)

	client := socketmode.New(api)

	// Identify the bot's own user ID so we can filter out its messages
	// (prevents echo loops when the bot posts status updates)
	auth, err := api.AuthTest()
	if err != nil {
		return nil, fmt.Errorf("slack auth failed: %w", err)
	}

	slog.Info("slack socket mode initialized",
		"bot_id", auth.BotID,
		"bot_user_id", auth.UserID,
		"team", auth.Team,
	)

	return &SlackEventHandler{
		client:          client,
		incidentService: incidentService,
		chatService:     chatService,
		botUserID:       auth.UserID,
	}, nil
}

// Start begins the Socket Mode connection and event listener in background goroutines.
// It returns immediately; events are processed asynchronously.
func (h *SlackEventHandler) Start() {
	go h.listen()
	go func() {
		if err := h.client.Run(); err != nil {
			slog.Error("slack socket mode client stopped", "error", err)
		}
	}()
}

// handleInteraction handles block action button clicks and modal submissions.
func (h *SlackEventHandler) handleInteraction(evt socketmode.Event) {
	callback, ok := evt.Data.(slack.InteractionCallback)
	if !ok {
		h.client.Ack(*evt.Request)
		return
	}
	h.client.Ack(*evt.Request)

	switch callback.Type {
	case slack.InteractionTypeBlockActions:
		for _, action := range callback.ActionCallback.BlockActions {
			switch action.ActionID {
			case "acknowledge":
				h.handleStatusButton(callback, action, models.IncidentStatusAcknowledged)
			case "resolve":
				h.handleStatusButton(callback, action, models.IncidentStatusResolved)
			}
		}
	case slack.InteractionTypeViewSubmission:
		h.handleModalSubmission(callback)
	}
}

// handleStatusButton processes Acknowledge or Resolve button clicks.
func (h *SlackEventHandler) handleStatusButton(
	callback slack.InteractionCallback,
	action *slack.BlockAction,
	targetStatus models.IncidentStatus,
) {
	incidentID, err := uuid.Parse(action.Value)
	if err != nil {
		h.postEphemeral(callback.Channel.ID, callback.User.ID, "Failed to process action: invalid incident ID")
		return
	}

	// Fetch current incident to validate transition
	incident, err := h.incidentService.GetIncident(incidentID, 0)
	if err != nil {
		h.postEphemeral(callback.Channel.ID, callback.User.ID, "Could not find incident")
		return
	}

	if !isValidSlackTransition(incident.Status, targetStatus) {
		h.postEphemeral(callback.Channel.ID, callback.User.ID,
			fmt.Sprintf("Cannot %s: incident is already %s", targetStatus, incident.Status))
		return
	}

	// Update incident via the same service used by the HTTP API
	updated, err := h.incidentService.UpdateIncident(incidentID, &UpdateIncidentParams{
		Status:    targetStatus,
		UpdatedBy: callback.User.ID,
	})
	if err != nil {
		slog.Error("failed to update incident from slack button",
			"incident_id", incidentID,
			"target_status", targetStatus,
			"user", callback.User.ID,
			"error", err)
		h.postEphemeral(callback.Channel.ID, callback.User.ID,
			fmt.Sprintf("Failed to update incident: %s", err.Error()))
		return
	}

	// Update the original incident message to reflect new status/buttons
	msg := NewSlackMessageBuilder().BuildIncidentUpdatedMessage(updated)
	if err := h.chatService.UpdateMessage(callback.Channel.ID, callback.Message.Timestamp, msg); err != nil {
		slog.Warn("failed to update slack message after status change", "error", err)
	}

	// Post public confirmation so the whole team sees who acted
	_, err = h.chatService.PostMessage(callback.Channel.ID, Message{
		Text: fmt.Sprintf("<@%s> %s INC-%d", callback.User.ID, targetStatus, updated.IncidentNumber),
	})
	if err != nil {
		slog.Warn("failed to post confirmation message", "error", err)
	}
}

// postEphemeral sends an error message visible only to the user who triggered the action.
func (h *SlackEventHandler) postEphemeral(channelID, userID, text string) {
	_, err := h.client.PostEphemeral(channelID, userID, slack.MsgOptionText(text, false))
	if err != nil {
		slog.Warn("failed to post ephemeral message", "channel", channelID, "user", userID, "error", err)
	}
}

// isValidSlackTransition checks whether the status transition is allowed.
func isValidSlackTransition(current, target models.IncidentStatus) bool {
	if current == target {
		return false // Already in this state — treat as invalid to show user a useful error
	}
	for _, allowed := range validSlackTransitions[current] {
		if allowed == target {
			return true
		}
	}
	return false
}

// handleSlashCommand handles /incident slash commands.
// Supported: /incident new [title], /incident list, /incident help
func (h *SlackEventHandler) handleSlashCommand(evt socketmode.Event) {
	cmd, ok := evt.Data.(slack.SlashCommand)
	if !ok {
		h.client.Ack(*evt.Request)
		return
	}
	h.client.Ack(*evt.Request)

	parts := strings.Fields(cmd.Text)
	if len(parts) == 0 {
		h.sendHelpResponse(cmd)
		return
	}

	switch parts[0] {
	case "new":
		h.openCreateIncidentModal(cmd)
	case "list":
		h.listOpenIncidents(cmd)
	case "help":
		h.sendHelpResponse(cmd)
	default:
		h.sendHelpResponse(cmd)
	}
}

// openCreateIncidentModal opens a Block Kit modal for declaring a new incident.
// Pre-fills the title from the text after "new" (e.g. /incident new High CPU).
func (h *SlackEventHandler) openCreateIncidentModal(cmd slack.SlashCommand) {
	prefillTitle := strings.TrimSpace(strings.TrimPrefix(cmd.Text, "new"))

	titleInput := slack.NewPlainTextInputBlockElement(
		slack.NewTextBlockObject(slack.PlainTextType, "e.g., API Gateway 5xx errors", false, false),
		"title_input",
	)
	if prefillTitle != "" {
		titleInput.InitialValue = prefillTitle
	}

	modalView := slack.ModalViewRequest{
		Type:       slack.VTModal,
		Title:      slack.NewTextBlockObject(slack.PlainTextType, "Declare Incident", false, false),
		Submit:     slack.NewTextBlockObject(slack.PlainTextType, "Create", false, false),
		Close:      slack.NewTextBlockObject(slack.PlainTextType, "Cancel", false, false),
		CallbackID: "create_incident",
		Blocks: slack.Blocks{
			BlockSet: []slack.Block{
				slack.NewInputBlock("title",
					slack.NewTextBlockObject(slack.PlainTextType, "Title", false, false),
					nil,
					titleInput,
				),
				slack.NewInputBlock("severity",
					slack.NewTextBlockObject(slack.PlainTextType, "Severity", false, false),
					nil,
					slack.NewOptionsSelectBlockElement(
						slack.OptTypeStatic,
						slack.NewTextBlockObject(slack.PlainTextType, "Select severity", false, false),
						"severity_input",
						slack.NewOptionBlockObject("critical", slack.NewTextBlockObject(slack.PlainTextType, "🔴 Critical", false, false), nil),
						slack.NewOptionBlockObject("high", slack.NewTextBlockObject(slack.PlainTextType, "🟠 High", false, false), nil),
						slack.NewOptionBlockObject("medium", slack.NewTextBlockObject(slack.PlainTextType, "🟡 Medium", false, false), nil),
						slack.NewOptionBlockObject("low", slack.NewTextBlockObject(slack.PlainTextType, "🟢 Low", false, false), nil),
					),
				),
				slack.NewInputBlock("summary",
					slack.NewTextBlockObject(slack.PlainTextType, "Summary (optional)", false, false),
					nil,
					slack.NewPlainTextInputBlockElement(
						slack.NewTextBlockObject(slack.PlainTextType, "Brief description", false, false),
						"summary_input",
					),
				).WithOptional(true),
			},
		},
	}

	if _, err := h.client.OpenView(cmd.TriggerID, modalView); err != nil {
		slog.Error("failed to open create incident modal", "error", err, "user", cmd.UserID)
	}
}

// handleModalSubmission processes modal form submissions (create_incident).
func (h *SlackEventHandler) handleModalSubmission(callback slack.InteractionCallback) {
	if callback.View.CallbackID != "create_incident" {
		return
	}

	values := callback.View.State.Values
	title := values["title"]["title_input"].Value
	severity := models.IncidentSeverity(values["severity"]["severity_input"].SelectedOption.Value)
	summary := values["summary"]["summary_input"].Value

	incident, err := h.incidentService.CreateIncident(&CreateIncidentParams{
		Title:       title,
		Severity:    severity,
		Description: summary,
		CreatedBy:   callback.User.ID,
	})
	if err != nil {
		slog.Error("failed to create incident from modal",
			"error", err,
			"user", callback.User.ID,
			"title", title)
		return
	}

	slog.Info("incident created via slack modal",
		"incident_id", incident.ID,
		"incident_number", incident.IncidentNumber,
		"created_by", callback.User.ID)
}

// listOpenIncidents posts an ephemeral message listing recent open incidents.
func (h *SlackEventHandler) listOpenIncidents(cmd slack.SlashCommand) {
	incidents, _, err := h.incidentService.ListIncidents(
		repository.IncidentFilters{Status: models.IncidentStatusTriggered},
		repository.Pagination{Page: 1, PageSize: 10},
	)
	if err != nil {
		h.postEphemeral(cmd.ChannelID, cmd.UserID, "Failed to list incidents: "+err.Error())
		return
	}

	if len(incidents) == 0 {
		h.postEphemeral(cmd.ChannelID, cmd.UserID, "✅ No open incidents right now.")
		return
	}

	var sb strings.Builder
	sb.WriteString("*Open Incidents:*\n")
	for _, inc := range incidents {
		sb.WriteString(fmt.Sprintf("• %s *INC-%d:* %s (%s)\n",
			getSeverityEmoji(inc.Severity),
			inc.IncidentNumber,
			inc.Title,
			inc.Severity))
	}
	h.postEphemeral(cmd.ChannelID, cmd.UserID, sb.String())
}

// sendHelpResponse posts ephemeral usage instructions.
func (h *SlackEventHandler) sendHelpResponse(cmd slack.SlashCommand) {
	h.postEphemeral(cmd.ChannelID, cmd.UserID,
		"*OpenIncident Slash Commands:*\n"+
			"• `/incident new [title]` — Declare a new incident (opens form)\n"+
			"• `/incident list` — List open incidents\n"+
			"• `/incident help` — Show this message")
}

// handleEventsAPI handles Events API payloads (message events, etc.).
func (h *SlackEventHandler) handleEventsAPI(evt socketmode.Event) {
	eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		h.client.Ack(*evt.Request)
		return
	}
	h.client.Ack(*evt.Request)

	switch ev := eventsAPIEvent.InnerEvent.Data.(type) {
	case *slackevents.MessageEvent:
		h.handleChannelMessage(ev)
	}
}

// handleChannelMessage syncs a Slack channel message to the incident timeline.
// Skips bot messages and messages in non-incident channels.
func (h *SlackEventHandler) handleChannelMessage(ev *slackevents.MessageEvent) {
	// Echo prevention: skip bot messages and our own bot's messages
	if ev.SubType == "bot_message" || ev.User == h.botUserID || ev.User == "" {
		return
	}

	// Only process messages in channels linked to incidents
	incident, err := h.incidentService.GetIncidentBySlackChannelID(ev.Channel)
	if err != nil || incident == nil {
		return
	}

	// Resolve display name and avatar
	displayName := ev.User
	avatarURL := ""
	if userInfo, err := h.client.GetUserInfo(ev.User); err == nil {
		if userInfo.Profile.DisplayName != "" {
			displayName = userInfo.Profile.DisplayName
		} else {
			displayName = userInfo.RealName
		}
		avatarURL = userInfo.Profile.Image72
	}

	content := models.JSONB{
		"text":        ev.Text,
		"author_id":   ev.User,
		"author_name": displayName,
		"avatar_url":  avatarURL,
		"message_ts":  ev.TimeStamp,
		"is_thread":   ev.ThreadTimeStamp != "" && ev.ThreadTimeStamp != ev.TimeStamp,
	}
	if ev.ThreadTimeStamp != "" {
		content["thread_ts"] = ev.ThreadTimeStamp
	}

	if _, err := h.incidentService.CreateTimelineEntry(&CreateTimelineEntryParams{
		IncidentID: incident.ID,
		Type:       "slack_message",
		ActorType:  "slack_user",
		ActorID:    ev.User,
		Content:    content,
	}); err != nil {
		slog.Warn("failed to create timeline entry for slack message",
			"incident_id", incident.ID,
			"error", err)
	}
}

// listen processes events from the Socket Mode channel.
func (h *SlackEventHandler) listen() {
	for evt := range h.client.Events {
		switch evt.Type {
		case socketmode.EventTypeConnecting:
			slog.Info("slack socket mode connecting...")
		case socketmode.EventTypeConnectionError:
			slog.Warn("slack socket mode connection error, will retry")
		case socketmode.EventTypeConnected:
			slog.Info("slack socket mode connected - bidirectional sync active")
		case socketmode.EventTypeInteractive:
			h.handleInteraction(evt)
		case socketmode.EventTypeSlashCommand:
			h.handleSlashCommand(evt)
		case socketmode.EventTypeEventsAPI:
			h.handleEventsAPI(evt)
		}
	}
}
