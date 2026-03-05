package services

import (
	"encoding/json"
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
	userRepo        repository.UserRepository
	botUserID       string
}

// NewSlackEventHandler creates a Socket Mode event handler.
// Requires both SLACK_APP_TOKEN (xapp-...) and SLACK_BOT_TOKEN (xoxb-...).
func NewSlackEventHandler(
	appToken string,
	botToken string,
	incidentService IncidentService,
	chatService ChatService,
	userRepo repository.UserRepository,
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
		userRepo:        userRepo,
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
			case "make_me_lead":
				h.handleMakeMeLead(callback, action)
			case "open_note_modal":
				h.handleOpenNoteModal(callback, action)
			case "view_overview":
				h.handleViewOverview(callback, action)
			case "view_commands":
				h.handleViewCommands(callback)
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
		UpdatedBy: h.resolveActorID(callback.User.ID),
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
	case "ack", "acknowledge":
		h.ackIncidentInChannel(cmd)
	case "resolve":
		h.resolveIncidentInChannel(cmd)
	case "note":
		noteText := strings.TrimSpace(strings.TrimPrefix(cmd.Text, parts[0]))
		h.addNoteToIncident(cmd, noteText)
	case "lead":
		targetArg := ""
		if len(parts) > 1 {
			targetArg = strings.Join(parts[1:], " ")
		}
		h.assignLeadFromSlash(cmd, targetArg)
	case "status":
		h.showIncidentStatus(cmd)
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

// handleModalSubmission processes modal form submissions.
func (h *SlackEventHandler) handleModalSubmission(callback slack.InteractionCallback) {
	switch callback.View.CallbackID {
	case "create_incident":
		h.handleCreateIncidentSubmission(callback)
	case "add_note_modal":
		h.handleAddNoteSubmission(callback)
	}
}

// handleCreateIncidentSubmission processes the create_incident modal.
func (h *SlackEventHandler) handleCreateIncidentSubmission(callback slack.InteractionCallback) {

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
		"*OpenIncident Slash Commands:*\n\n"+
			"*Declare & Browse*\n"+
			"• `/incident new [title]` — Declare a new incident (opens form)\n"+
			"• `/incident list` — List open incidents\n\n"+
			"*In an Incident Channel*\n"+
			"• `/incident ack` — Acknowledge this incident\n"+
			"• `/incident resolve` — Resolve this incident\n"+
			"• `/incident status` — Show incident status\n"+
			"• `/incident note <text>` — Add a timeline note (opens form if no text)\n"+
			"• `/incident lead [me|@user]` — Assign incident commander\n\n"+
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

	slog.Info("slack events api event received", "type", eventsAPIEvent.InnerEvent.Type)
	switch ev := eventsAPIEvent.InnerEvent.Data.(type) {
	case *slackevents.MessageEvent:
		h.handleChannelMessage(ev)
	default:
		slog.Warn("slack events api: unhandled inner event type", "type", eventsAPIEvent.InnerEvent.Type)
	}
}

// handleChannelMessage syncs a Slack channel message to the incident timeline.
// Skips bot messages and messages in non-incident channels.
func (h *SlackEventHandler) handleChannelMessage(ev *slackevents.MessageEvent) {
	slog.Info("slack channel message received",
		"channel", ev.Channel,
		"user", ev.User,
		"subtype", ev.SubType,
	)

	// Echo prevention: skip bot messages and our own bot's messages
	if ev.SubType == "bot_message" || ev.User == h.botUserID || ev.User == "" {
		slog.Info("slack channel message skipped (bot/empty)", "subtype", ev.SubType, "user", ev.User)
		return
	}

	// Only process messages in channels linked to incidents
	incident, err := h.incidentService.GetIncidentBySlackChannelID(ev.Channel)
	if err != nil {
		slog.Warn("slack channel message: error looking up incident by channel",
			"channel", ev.Channel, "error", err)
		return
	}
	if incident == nil {
		slog.Warn("slack channel message: no incident linked to channel", "channel", ev.Channel)
		return
	}
	slog.Info("slack channel message: syncing to timeline",
		"channel", ev.Channel,
		"incident_id", incident.ID,
		"user", ev.User,
	)

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

// ── Channel-context slash command helpers ──────────────────────────────────

// getIncidentFromChannel looks up the incident linked to a Slack channel.
func (h *SlackEventHandler) getIncidentFromChannel(channelID string) (*models.Incident, error) {
	incident, err := h.incidentService.GetIncidentBySlackChannelID(channelID)
	if err != nil || incident == nil {
		return nil, fmt.Errorf("no incident is linked to this channel")
	}
	return incident, nil
}

// ackIncidentInChannel acknowledges the incident linked to the slash command channel.
func (h *SlackEventHandler) ackIncidentInChannel(cmd slack.SlashCommand) {
	incident, err := h.getIncidentFromChannel(cmd.ChannelID)
	if err != nil {
		h.postEphemeral(cmd.ChannelID, cmd.UserID, "No incident is linked to this channel.")
		return
	}
	if !isValidSlackTransition(incident.Status, models.IncidentStatusAcknowledged) {
		h.postEphemeral(cmd.ChannelID, cmd.UserID,
			fmt.Sprintf("Cannot acknowledge: incident is already *%s*.", incident.Status))
		return
	}
	updated, err := h.incidentService.UpdateIncident(incident.ID, &UpdateIncidentParams{
		Status:    models.IncidentStatusAcknowledged,
		UpdatedBy: h.resolveActorID(cmd.UserID),
	})
	if err != nil {
		h.postEphemeral(cmd.ChannelID, cmd.UserID, "Failed to acknowledge: "+err.Error())
		return
	}
	h.refreshIncidentCard(updated)
	_, _ = h.chatService.PostMessage(cmd.ChannelID, Message{
		Text: fmt.Sprintf("<@%s> acknowledged INC-%d", cmd.UserID, updated.IncidentNumber),
	})
}

// resolveIncidentInChannel resolves the incident linked to the slash command channel.
func (h *SlackEventHandler) resolveIncidentInChannel(cmd slack.SlashCommand) {
	incident, err := h.getIncidentFromChannel(cmd.ChannelID)
	if err != nil {
		h.postEphemeral(cmd.ChannelID, cmd.UserID, "No incident is linked to this channel.")
		return
	}
	if !isValidSlackTransition(incident.Status, models.IncidentStatusResolved) {
		h.postEphemeral(cmd.ChannelID, cmd.UserID,
			fmt.Sprintf("Cannot resolve: incident is already *%s*.", incident.Status))
		return
	}
	updated, err := h.incidentService.UpdateIncident(incident.ID, &UpdateIncidentParams{
		Status:    models.IncidentStatusResolved,
		UpdatedBy: h.resolveActorID(cmd.UserID),
	})
	if err != nil {
		h.postEphemeral(cmd.ChannelID, cmd.UserID, "Failed to resolve: "+err.Error())
		return
	}
	h.refreshIncidentCard(updated)
	_, _ = h.chatService.PostMessage(cmd.ChannelID, Message{
		Text: fmt.Sprintf("<@%s> resolved INC-%d", cmd.UserID, updated.IncidentNumber),
	})
}

// addNoteToIncident creates a timeline note for the incident in the current channel.
// If noteText is empty, opens the add-note modal instead.
func (h *SlackEventHandler) addNoteToIncident(cmd slack.SlashCommand, noteText string) {
	incident, err := h.getIncidentFromChannel(cmd.ChannelID)
	if err != nil {
		h.postEphemeral(cmd.ChannelID, cmd.UserID, "No incident is linked to this channel.")
		return
	}
	if noteText == "" {
		h.openNoteModalForIncident(cmd.TriggerID, cmd.UserID, incident.ID.String())
		return
	}
	if _, err := h.incidentService.CreateTimelineEntry(&CreateTimelineEntryParams{
		IncidentID: incident.ID,
		Type:       models.TimelineTypeMessage,
		ActorType:  "user",
		ActorID:    h.resolveActorID(cmd.UserID),
		Content:    models.JSONB{"text": noteText, "source": "slack_slash_command"},
	}); err != nil {
		h.postEphemeral(cmd.ChannelID, cmd.UserID, "Failed to add note: "+err.Error())
		return
	}
	_, _ = h.chatService.PostMessage(cmd.ChannelID, Message{
		Text: fmt.Sprintf("📝 <@%s> added a note: %s", cmd.UserID, noteText),
	})
}

// showIncidentStatus posts an ephemeral status summary for the current channel's incident.
func (h *SlackEventHandler) showIncidentStatus(cmd slack.SlashCommand) {
	incident, err := h.getIncidentFromChannel(cmd.ChannelID)
	if err != nil {
		h.postEphemeral(cmd.ChannelID, cmd.UserID, "No incident is linked to this channel.")
		return
	}
	commander := "_unassigned_"
	if incident.CommanderID != nil {
		if user, err := h.userRepo.GetByID(*incident.CommanderID); err == nil {
			commander = user.Name
		}
	}
	summary := "_none_"
	if incident.Summary != "" {
		summary = incident.Summary
	}
	msg := fmt.Sprintf(
		"*INC-%d: %s*\n*Status:* %s %s\n*Severity:* %s %s\n*Commander:* %s\n*Summary:* %s",
		incident.IncidentNumber, incident.Title,
		getStatusEmoji(incident.Status), incident.Status,
		getSeverityEmoji(incident.Severity), incident.Severity,
		commander, summary,
	)
	h.postEphemeral(cmd.ChannelID, cmd.UserID, msg)
}

// assignLeadFromSlash assigns an incident commander via /incident lead [me|@user].
// targetArg="" or "me" → self; "<@UXXXXXX>" or "<@UXXXXXX|name>" → that user.
func (h *SlackEventHandler) assignLeadFromSlash(cmd slack.SlashCommand, targetArg string) {
	incident, err := h.getIncidentFromChannel(cmd.ChannelID)
	if err != nil {
		h.postEphemeral(cmd.ChannelID, cmd.UserID, "No incident is linked to this channel.")
		return
	}

	slackUserID := cmd.UserID // default: self
	if targetArg != "" && targetArg != "me" {
		// Parse "<@UXXXXX>" or "<@UXXXXX|displayname>"
		slackUserID = parseSlackMention(targetArg)
		if slackUserID == "" {
			h.postEphemeral(cmd.ChannelID, cmd.UserID,
				"Could not parse user. Usage: `/incident lead` or `/incident lead @username`")
			return
		}
	}

	internalUser, err := h.resolveSlackUserToInternal(slackUserID)
	if err != nil {
		h.postEphemeral(cmd.ChannelID, cmd.UserID,
			fmt.Sprintf("Could not find user <@%s> in OpenIncident. Make sure they have a profile with a Slack ID set.", slackUserID))
		return
	}

	if _, err := h.incidentService.UpdateIncident(incident.ID, &UpdateIncidentParams{
		CommanderID: &internalUser.ID,
		UpdatedBy:   h.resolveActorID(cmd.UserID),
	}); err != nil {
		h.postEphemeral(cmd.ChannelID, cmd.UserID, "Failed to assign commander: "+err.Error())
		return
	}
	_, _ = h.chatService.PostMessage(cmd.ChannelID, Message{
		Text: fmt.Sprintf("🎖️ <@%s> is now the Incident Commander for INC-%d", slackUserID, incident.IncidentNumber),
	})
}

// ── Button interaction handlers ─────────────────────────────────────────────

// handleMakeMeLead handles the "🎖️ Take Lead" button.
func (h *SlackEventHandler) handleMakeMeLead(callback slack.InteractionCallback, action *slack.BlockAction) {
	incidentID, err := uuid.Parse(action.Value)
	if err != nil {
		h.postEphemeral(callback.Channel.ID, callback.User.ID, "Failed to process action: invalid incident ID")
		return
	}

	if h.userRepo == nil {
		h.postEphemeral(callback.Channel.ID, callback.User.ID, "User repository not available")
		return
	}

	internalUser, err := h.resolveSlackUserToInternal(callback.User.ID)
	if err != nil {
		h.postEphemeral(callback.Channel.ID, callback.User.ID,
			"Could not find your OpenIncident profile. Ask an admin to link your Slack ID.")
		return
	}

	updated, err := h.incidentService.UpdateIncident(incidentID, &UpdateIncidentParams{
		CommanderID: &internalUser.ID,
		UpdatedBy:   h.resolveActorID(callback.User.ID),
	})
	if err != nil {
		h.postEphemeral(callback.Channel.ID, callback.User.ID, "Failed to assign command: "+err.Error())
		return
	}

	h.refreshIncidentCard(updated)
	_, _ = h.chatService.PostMessage(callback.Channel.ID, Message{
		Text: fmt.Sprintf("🎖️ <@%s> is now the Incident Commander for INC-%d", callback.User.ID, updated.IncidentNumber),
	})
}

// handleOpenNoteModal handles the "📝 Add Note" button — opens the note input modal.
func (h *SlackEventHandler) handleOpenNoteModal(callback slack.InteractionCallback, action *slack.BlockAction) {
	h.openNoteModalForIncident(callback.TriggerID, callback.User.ID, action.Value)
}

// openNoteModalForIncident opens the Add Note modal, storing the incident ID in private_metadata.
func (h *SlackEventHandler) openNoteModalForIncident(triggerID, userID, incidentID string) {
	meta, _ := json.Marshal(map[string]string{"incident_id": incidentID})

	modalView := slack.ModalViewRequest{
		Type:            slack.VTModal,
		Title:           slack.NewTextBlockObject(slack.PlainTextType, "Add Timeline Note", false, false),
		Submit:          slack.NewTextBlockObject(slack.PlainTextType, "Add Note", false, false),
		Close:           slack.NewTextBlockObject(slack.PlainTextType, "Cancel", false, false),
		CallbackID:      "add_note_modal",
		PrivateMetadata: string(meta),
		Blocks: slack.Blocks{
			BlockSet: []slack.Block{
				slack.NewInputBlock("note",
					slack.NewTextBlockObject(slack.PlainTextType, "Note", false, false),
					nil,
					slack.NewPlainTextInputBlockElement(
						slack.NewTextBlockObject(slack.PlainTextType, "Describe what's happening…", false, false),
						"note_input",
					),
				),
			},
		},
	}

	if _, err := h.client.OpenView(triggerID, modalView); err != nil {
		slog.Error("failed to open add note modal", "error", err, "user", userID)
	}
}

// handleAddNoteSubmission processes the add_note_modal submission.
// Creates the timeline entry AND posts the note text to the incident channel
// so it also flows back via handleChannelMessage (Slack → UI sync).
func (h *SlackEventHandler) handleAddNoteSubmission(callback slack.InteractionCallback) {
	var meta map[string]string
	if err := json.Unmarshal([]byte(callback.View.PrivateMetadata), &meta); err != nil {
		slog.Error("failed to parse add_note_modal metadata", "error", err)
		return
	}
	incidentID, err := uuid.Parse(meta["incident_id"])
	if err != nil {
		slog.Error("invalid incident_id in add_note_modal metadata", "error", err)
		return
	}

	noteText := strings.TrimSpace(callback.View.State.Values["note"]["note_input"].Value)
	if noteText == "" {
		return
	}

	if _, err := h.incidentService.CreateTimelineEntry(&CreateTimelineEntryParams{
		IncidentID: incidentID,
		Type:       models.TimelineTypeMessage,
		ActorType:  "user",
		ActorID:    h.resolveActorID(callback.User.ID),
		Content:    models.JSONB{"text": noteText, "source": "slack_note_modal"},
	}); err != nil {
		slog.Error("failed to create timeline note from slack modal",
			"incident_id", incidentID, "error", err)
		return
	}

	// Post the note to the Slack channel so the team sees it.
	// We look up the incident to get the channel ID.
	if incident, err := h.incidentService.GetIncident(incidentID, 0); err == nil && incident.SlackChannelID != "" {
		_, _ = h.chatService.PostMessage(incident.SlackChannelID, Message{
			Text: fmt.Sprintf("📝 *Note from <@%s>:*\n%s", callback.User.ID, noteText),
		})
	}
}

// handleViewOverview posts an ephemeral incident status summary for the button clicker.
func (h *SlackEventHandler) handleViewOverview(callback slack.InteractionCallback, action *slack.BlockAction) {
	incidentID, err := uuid.Parse(action.Value)
	if err != nil {
		h.postEphemeral(callback.Channel.ID, callback.User.ID, "Invalid incident ID")
		return
	}
	incident, err := h.incidentService.GetIncident(incidentID, 0)
	if err != nil {
		h.postEphemeral(callback.Channel.ID, callback.User.ID, "Could not find incident")
		return
	}
	commander := "_unassigned_"
	if incident.CommanderID != nil {
		if user, err := h.userRepo.GetByID(*incident.CommanderID); err == nil {
			commander = user.Name
		}
	}
	summary := "_none_"
	if incident.Summary != "" {
		summary = incident.Summary
	}
	msg := fmt.Sprintf(
		"*INC-%d: %s*\n*Status:* %s %s\n*Severity:* %s %s\n*Commander:* %s\n*Summary:* %s",
		incident.IncidentNumber, incident.Title,
		getStatusEmoji(incident.Status), incident.Status,
		getSeverityEmoji(incident.Severity), incident.Severity,
		commander, summary,
	)
	h.postEphemeral(callback.Channel.ID, callback.User.ID, msg)
}

// handleViewCommands posts an ephemeral list of available slash commands.
func (h *SlackEventHandler) handleViewCommands(callback slack.InteractionCallback) {
	h.postEphemeral(callback.Channel.ID, callback.User.ID,
		"*OpenIncident Slash Commands:*\n\n"+
			"*Declare & Browse*\n"+
			"• `/incident new [title]` — Declare a new incident (opens form)\n"+
			"• `/incident list` — List open incidents\n\n"+
			"*In an Incident Channel*\n"+
			"• `/incident ack` — Acknowledge this incident\n"+
			"• `/incident resolve` — Resolve this incident\n"+
			"• `/incident status` — Show incident status\n"+
			"• `/incident note <text>` — Add a timeline note (opens form if no text)\n"+
			"• `/incident lead [me|@user]` — Assign incident commander\n\n"+
			"• `/incident help` — Show this message")
}

// ── Shared utilities ─────────────────────────────────────────────────────────

// refreshIncidentCard updates the pinned incident card in Slack using the stored message TS.
func (h *SlackEventHandler) refreshIncidentCard(incident *models.Incident) {
	if incident.SlackChannelID == "" || incident.SlackMessageTS == "" {
		return
	}
	msg := NewSlackMessageBuilder().BuildIncidentUpdatedMessage(incident)
	if err := h.chatService.UpdateMessage(incident.SlackChannelID, incident.SlackMessageTS, msg); err != nil {
		slog.Warn("failed to refresh incident card after slash command",
			"incident_id", incident.ID, "error", err)
	}
}

// resolveSlackUserToInternal maps a Slack user ID to an internal models.User.
func (h *SlackEventHandler) resolveSlackUserToInternal(slackUserID string) (*models.User, error) {
	if h.userRepo == nil {
		return nil, fmt.Errorf("user repository not configured")
	}
	user, err := h.userRepo.GetBySlackUserID(slackUserID)
	if err != nil {
		return nil, fmt.Errorf("no OpenIncident user found for Slack ID %s: %w", slackUserID, err)
	}
	return user, nil
}

// resolveActorID converts a Slack user ID to the best available actor identifier
// for timeline entries, in priority order:
//  1. Internal user UUID (if the user's Slack ID is linked in OpenIncident)
//  2. Slack display name (so the timeline shows a real name instead of a raw ID)
//  3. Original Slack user ID (last resort)
func (h *SlackEventHandler) resolveActorID(slackUserID string) string {
	// 1. Prefer internal user UUID — frontend can resolve to name+avatar
	if h.userRepo != nil {
		if user, err := h.userRepo.GetBySlackUserID(slackUserID); err == nil {
			return user.ID.String()
		}
	}
	// 2. Fall back to Slack display name so at least a human-readable name shows
	if userInfo, err := h.client.GetUserInfo(slackUserID); err == nil {
		if userInfo.Profile.DisplayName != "" {
			return userInfo.Profile.DisplayName
		}
		if userInfo.RealName != "" {
			return userInfo.RealName
		}
	}
	// 3. Last resort — still better than nothing
	return slackUserID
}

// parseSlackMention extracts the Slack user ID from a mention string like "<@U012345>" or "<@U012345|name>".
// Returns "" if the string is not a valid mention.
func parseSlackMention(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "<@") || !strings.HasSuffix(s, ">") {
		return ""
	}
	inner := s[2 : len(s)-1] // strip <@ and >
	if idx := strings.Index(inner, "|"); idx != -1 {
		inner = inner[:idx]
	}
	if strings.HasPrefix(inner, "U") && len(inner) >= 9 {
		return inner
	}
	return ""
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
