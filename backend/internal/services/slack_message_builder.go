package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/openincident/openincident/internal/models"
	"github.com/slack-go/slack"
)

// SlackMessageBuilder constructs rich Slack messages using Block Kit.
type SlackMessageBuilder struct{}

// NewSlackMessageBuilder creates a new SlackMessageBuilder instance.
func NewSlackMessageBuilder() *SlackMessageBuilder {
	return &SlackMessageBuilder{}
}

// BuildIncidentCreatedMessage creates a rich Block Kit message for a new incident.
// The message includes:
// - Header with severity emoji and incident title
// - Incident details (severity, status, created time)
// - Linked alerts (if any)
// - Action buttons (Acknowledge, Resolve)
func (b *SlackMessageBuilder) BuildIncidentCreatedMessage(
	incident *models.Incident,
	alerts []models.Alert,
) Message {
	blocks := []slack.Block{
		// Header block with emoji and title
		slack.NewHeaderBlock(
			slack.NewTextBlockObject(
				slack.PlainTextType,
				fmt.Sprintf("%s INC-%d: %s",
					getSeverityEmoji(incident.Severity),
					incident.IncidentNumber,
					incident.Title),
				false,
				false,
			),
		),

		// Divider
		slack.NewDividerBlock(),

		// Details section with fields
		slack.NewSectionBlock(
			nil,
			[]*slack.TextBlockObject{
				slack.NewTextBlockObject(
					slack.MarkdownType,
					fmt.Sprintf("*Severity:* %s %s",
						getSeverityEmoji(incident.Severity),
						incident.Severity),
					false,
					false,
				),
				slack.NewTextBlockObject(
					slack.MarkdownType,
					fmt.Sprintf("*Status:* %s", incident.Status),
					false,
					false,
				),
				slack.NewTextBlockObject(
					slack.MarkdownType,
					fmt.Sprintf("*Created:* <!date^%d^{date_short_pretty} at {time}|%s>",
						incident.TriggeredAt.Unix(),
						incident.TriggeredAt.Format("2006-01-02 15:04:05")),
					false,
					false,
				),
			},
			nil,
		),

		// Divider
		slack.NewDividerBlock(),
	}

	// Add linked alerts section if any
	if len(alerts) > 0 {
		alertsText := "*Linked Alerts:*\n"
		for _, alert := range alerts {
			alertSeverityEmoji := getAlertSeverityEmoji(alert.Severity)
			alertsText += fmt.Sprintf("• %s *%s* (%s): %s\n",
				alertSeverityEmoji,
				alert.Source,
				alert.Severity,
				alert.Title)
		}

		blocks = append(blocks, slack.NewSectionBlock(
			slack.NewTextBlockObject(
				slack.MarkdownType,
				alertsText,
				false,
				false,
			),
			nil,
			nil,
		))

		blocks = append(blocks, slack.NewDividerBlock())
	}

	// Quick-action buttons
	blocks = append(blocks, slack.NewActionBlock(
		"incident_quick_actions",
		slack.NewButtonBlockElement("make_me_lead", incident.ID.String(),
			slack.NewTextBlockObject(slack.PlainTextType, "🧑‍🚒 Make me Lead", false, false),
		),
		slack.NewButtonBlockElement("open_note_modal", incident.ID.String(),
			slack.NewTextBlockObject(slack.PlainTextType, "📝 Add Note", false, false),
		),
		slack.NewButtonBlockElement("view_overview", incident.ID.String(),
			slack.NewTextBlockObject(slack.PlainTextType, "👀 Overview", false, false),
		),
		slack.NewButtonBlockElement("view_commands", incident.ID.String(),
			slack.NewTextBlockObject(slack.PlainTextType, "🔍 View all commands", false, false),
		),
	))

	return Message{
		Text:   fmt.Sprintf("INC-%d: %s", incident.IncidentNumber, incident.Title),
		Blocks: blocksToInterfaces(blocks),
	}
}

// getSeverityEmoji returns an emoji representing the incident severity.
func getSeverityEmoji(severity models.IncidentSeverity) string {
	switch severity {
	case models.IncidentSeverityCritical:
		return "🔴"
	case models.IncidentSeverityHigh:
		return "🟠"
	case models.IncidentSeverityMedium:
		return "🟡"
	case models.IncidentSeverityLow:
		return "🟢"
	default:
		return "⚪"
	}
}

// getAlertSeverityEmoji returns an emoji representing the alert severity.
func getAlertSeverityEmoji(severity models.AlertSeverity) string {
	switch severity {
	case models.AlertSeverityCritical:
		return "🔴"
	case models.AlertSeverityWarning:
		return "🟠"
	case models.AlertSeverityInfo:
		return "🔵"
	default:
		return "⚪"
	}
}

// blocksToInterfaces converts []slack.Block to []interface{} for the Message struct.
func blocksToInterfaces(blocks []slack.Block) []interface{} {
	result := make([]interface{}, len(blocks))
	for i, block := range blocks {
		result[i] = block
	}
	return result
}

// BuildIncidentUpdatedMessage rebuilds the incident message with status-aware buttons.
// Used to update the original pinned message when status changes from any source.
//
// BuildAlertLinkedMessage creates a message for when an alert is linked to an existing incident.
// This is used for grouped alerts (v0.3+) to notify that a new alert has been added to the incident.
func (b *SlackMessageBuilder) BuildAlertLinkedMessage(alert *models.Alert, incident *models.Incident) Message {
	alertSeverityEmoji := getAlertSeverityEmoji(alert.Severity)

	blocks := []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject(
				slack.MarkdownType,
				fmt.Sprintf(":link: *New alert linked to this incident*\n\n%s *%s* (%s) · `%s`\n_%s_",
					alertSeverityEmoji,
					alert.Title,
					alert.Severity,
					alert.Source,
					alert.Description),
				false,
				false,
			),
			nil,
			nil,
		),
	}

	// Build fallback text for notifications
	text := fmt.Sprintf("New alert linked: %s (%s) - %s",
		alert.Title,
		alert.Severity,
		alert.Description)

	// Convert []slack.Block to []interface{}
	interfaceBlocks := make([]interface{}, len(blocks))
	for i, block := range blocks {
		interfaceBlocks[i] = block
	}

	return Message{
		Text:   text,
		Blocks: interfaceBlocks,
	}
}

// Button rules:
//   - triggered: Acknowledge + Resolve
//   - acknowledged: Resolve only
//   - resolved/canceled: no buttons
func (b *SlackMessageBuilder) BuildIncidentUpdatedMessage(incident *models.Incident) Message {
	blocks := []slack.Block{
		slack.NewHeaderBlock(
			slack.NewTextBlockObject(
				slack.PlainTextType,
				fmt.Sprintf("%s INC-%d: %s",
					getSeverityEmoji(incident.Severity),
					incident.IncidentNumber,
					incident.Title),
				false, false,
			),
		),
		slack.NewDividerBlock(),
		slack.NewSectionBlock(
			nil,
			[]*slack.TextBlockObject{
				slack.NewTextBlockObject(slack.MarkdownType,
					fmt.Sprintf("*Severity:* %s %s", getSeverityEmoji(incident.Severity), incident.Severity),
					false, false),
				slack.NewTextBlockObject(slack.MarkdownType,
					fmt.Sprintf("*Status:* %s %s", getStatusEmoji(incident.Status), incident.Status),
					false, false),
				slack.NewTextBlockObject(slack.MarkdownType,
					fmt.Sprintf("*Triggered:* <!date^%d^{date_short_pretty} at {time}|%s>",
						incident.TriggeredAt.Unix(),
						incident.TriggeredAt.Format("2006-01-02 15:04:05")),
					false, false),
			},
			nil,
		),
		slack.NewDividerBlock(),
	}

	if incident.AcknowledgedAt != nil {
		blocks = append(blocks,
			slack.NewContextBlock("",
				slack.NewTextBlockObject(slack.MarkdownType,
					fmt.Sprintf("👀 Acknowledged <!date^%d^{date_short_pretty} at {time}|%s>",
						incident.AcknowledgedAt.Unix(),
						incident.AcknowledgedAt.Format("2006-01-02 15:04:05")),
					false, false),
			),
		)
	}
	if incident.ResolvedAt != nil {
		blocks = append(blocks,
			slack.NewContextBlock("",
				slack.NewTextBlockObject(slack.MarkdownType,
					fmt.Sprintf("✅ Resolved <!date^%d^{date_short_pretty} at {time}|%s>",
						incident.ResolvedAt.Unix(),
						incident.ResolvedAt.Format("2006-01-02 15:04:05")),
					false, false),
			),
		)
	}

	// Show action buttons for active incidents only
	if incident.Status == models.IncidentStatusTriggered || incident.Status == models.IncidentStatusAcknowledged {
		blocks = append(blocks, slack.NewActionBlock(
			"incident_quick_actions",
			slack.NewButtonBlockElement("make_me_lead", incident.ID.String(),
				slack.NewTextBlockObject(slack.PlainTextType, "🧑‍🚒 Make me Lead", false, false),
			),
			slack.NewButtonBlockElement("open_note_modal", incident.ID.String(),
				slack.NewTextBlockObject(slack.PlainTextType, "📝 Add Note", false, false),
			),
			slack.NewButtonBlockElement("view_overview", incident.ID.String(),
				slack.NewTextBlockObject(slack.PlainTextType, "👀 Overview", false, false),
			),
			slack.NewButtonBlockElement("view_commands", incident.ID.String(),
				slack.NewTextBlockObject(slack.PlainTextType, "🔍 View all commands", false, false),
			),
		))
	}

	return Message{
		Text:   fmt.Sprintf("INC-%d: %s [%s]", incident.IncidentNumber, incident.Title, incident.Status),
		Blocks: blocksToInterfaces(blocks),
	}
}

// getStatusEmoji returns an emoji for the given incident status.
func getStatusEmoji(status models.IncidentStatus) string {
	switch status {
	case models.IncidentStatusTriggered:
		return "🔴"
	case models.IncidentStatusAcknowledged:
		return "🟡"
	case models.IncidentStatusResolved:
		return "🟢"
	case models.IncidentStatusCanceled:
		return "⚫"
	default:
		return "⚪"
	}
}

// BuildStatusUpdateMessage creates a message for incident status changes
func (b *SlackMessageBuilder) BuildStatusUpdateMessage(
	incident *models.Incident,
	previousStatus models.IncidentStatus,
	newStatus models.IncidentStatus,
) Message {
	// Status emoji mapping
	statusEmoji := map[models.IncidentStatus]string{
		models.IncidentStatusTriggered:    "🔴",
		models.IncidentStatusAcknowledged: "🟡",
		models.IncidentStatusResolved:     "🟢",
		models.IncidentStatusCanceled:     "⚫",
	}

	emoji := statusEmoji[newStatus]
	title := fmt.Sprintf("%s Incident #%d: %s → %s",
		emoji,
		incident.IncidentNumber,
		strings.ToUpper(string(previousStatus)),
		strings.ToUpper(string(newStatus)),
	)

	blocks := []slack.Block{
		slack.NewHeaderBlock(slack.NewTextBlockObject(slack.PlainTextType, title, true, false)),
		slack.NewDividerBlock(),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType,
				fmt.Sprintf("*Incident:* %s\n*Previous Status:* %s\n*New Status:* %s\n*Changed At:* <!date^%d^{date_short_pretty} at {time}|%s>",
					incident.Title,
					strings.Title(string(previousStatus)),
					strings.Title(string(newStatus)),
					time.Now().Unix(),
					time.Now().Format("2006-01-02 15:04:05 MST"),
				),
				false,
				false,
			),
			nil,
			nil,
		),
	}

	// Add specific messaging for terminal states
	if newStatus == models.IncidentStatusResolved {
		blocks = append(blocks,
			slack.NewContextBlock("", slack.NewTextBlockObject(slack.MarkdownType, "✅ This incident has been resolved. Great work team!", false, false)),
		)
	} else if newStatus == models.IncidentStatusCanceled {
		blocks = append(blocks,
			slack.NewContextBlock("", slack.NewTextBlockObject(slack.MarkdownType, "⚠️ This incident has been canceled.", false, false)),
		)
	}

	return Message{
		Text:   title,
		Blocks: blocksToInterfaces(blocks),
	}
}

// BuildShiftHandoffIncomingMessage creates a DM for the user who is starting their on-call shift.
func (b *SlackMessageBuilder) BuildShiftHandoffIncomingMessage(
	scheduleName, layerName string,
	shiftEnd time.Time,
) Message {
	text := fmt.Sprintf("You are now on call for %s until %s. Good luck!",
		scheduleName, shiftEnd.Format("Mon Jan 2 15:04 MST"))

	blocks := []slack.Block{
		slack.NewHeaderBlock(
			slack.NewTextBlockObject(slack.PlainTextType, "📟 You're now on call", false, false),
		),
		slack.NewDividerBlock(),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType,
				fmt.Sprintf("*Schedule:* %s\n*Layer:* %s\n*Shift ends:* <!date^%d^{date_long_pretty} at {time}|%s>",
					scheduleName, layerName,
					shiftEnd.Unix(), shiftEnd.Format("Mon Jan 2 15:04 MST")),
				false, false),
			nil, nil,
		),
		slack.NewContextBlock("",
			slack.NewTextBlockObject(slack.MarkdownType,
				"Powered by <https://github.com/openincident/openincident|Fluidify Alert>",
				false, false),
		),
	}
	return Message{Text: text, Blocks: blocksToInterfaces(blocks)}
}

// BuildShiftHandoffOutgoingMessage creates a DM for the user whose on-call shift has ended.
func (b *SlackMessageBuilder) BuildShiftHandoffOutgoingMessage(
	scheduleName, layerName, incomingUser string,
) Message {
	text := fmt.Sprintf("Your on-call shift for %s has ended. %s is now on call.",
		scheduleName, incomingUser)

	blocks := []slack.Block{
		slack.NewHeaderBlock(
			slack.NewTextBlockObject(slack.PlainTextType, "✅ Your on-call shift has ended", false, false),
		),
		slack.NewDividerBlock(),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType,
				fmt.Sprintf("*Schedule:* %s\n*Layer:* %s\n*Now on call:* %s",
					scheduleName, layerName, incomingUser),
				false, false),
			nil, nil,
		),
		slack.NewContextBlock("",
			slack.NewTextBlockObject(slack.MarkdownType,
				"Powered by <https://github.com/openincident/openincident|Fluidify Alert>",
				false, false),
		),
	}
	return Message{Text: text, Blocks: blocksToInterfaces(blocks)}
}

// BuildShiftChannelNotification creates a channel post announcing a shift handoff.
// Posted to schedule.NotificationChannel when configured.
func (b *SlackMessageBuilder) BuildShiftChannelNotification(
	scheduleName, layerName, outgoingUser, incomingUser string,
	shiftEnd time.Time,
) Message {
	text := fmt.Sprintf("🔄 Shift change: %s → %s on call for %s",
		outgoingUser, incomingUser, scheduleName)

	blocks := []slack.Block{
		slack.NewHeaderBlock(
			slack.NewTextBlockObject(slack.PlainTextType, "🔄 On-call shift change", false, false),
		),
		slack.NewDividerBlock(),
		slack.NewSectionBlock(
			nil,
			[]*slack.TextBlockObject{
				slack.NewTextBlockObject(slack.MarkdownType,
					fmt.Sprintf("*Schedule:* %s", scheduleName), false, false),
				slack.NewTextBlockObject(slack.MarkdownType,
					fmt.Sprintf("*Layer:* %s", layerName), false, false),
				slack.NewTextBlockObject(slack.MarkdownType,
					fmt.Sprintf("*Outgoing:* %s", outgoingUser), false, false),
				slack.NewTextBlockObject(slack.MarkdownType,
					fmt.Sprintf("*Incoming:* %s", incomingUser), false, false),
			},
			nil,
		),
		slack.NewContextBlock("",
			slack.NewTextBlockObject(slack.MarkdownType,
				fmt.Sprintf("Next handoff <!date^%d^{date_short_pretty} at {time}|%s> · Powered by <https://github.com/openincident/openincident|Fluidify Alert>",
					shiftEnd.Unix(), shiftEnd.Format("Mon Jan 2 15:04 MST")),
				false, false),
		),
	}
	return Message{Text: text, Blocks: blocksToInterfaces(blocks)}
}

// BuildEscalationDMMessage creates the Slack DM sent to an on-call user when an
// alert escalates to their tier. Includes an "Acknowledge" interactive button
// whose action_id encodes the alert ID so the Slack event handler can route it
// to POST /api/v1/alerts/:id/acknowledge.
//
// tierIndex is 0-based; the display shows "Tier N" where N = tierIndex+1.
func (b *SlackMessageBuilder) BuildEscalationDMMessage(alert *models.Alert, tierIndex int) Message {
	severityEmoji := map[models.AlertSeverity]string{
		models.AlertSeverityCritical: "🔴",
		models.AlertSeverityWarning:  "🟡",
		models.AlertSeverityInfo:     "🔵",
	}
	emoji := severityEmoji[alert.Severity]
	if emoji == "" {
		emoji = "⚠️"
	}

	text := fmt.Sprintf("%s [Tier %d] You're being paged: %s", emoji, tierIndex+1, alert.Title)

	blocks := []slack.Block{
		slack.NewHeaderBlock(
			slack.NewTextBlockObject(slack.PlainTextType,
				fmt.Sprintf("%s You're being paged (Tier %d)", emoji, tierIndex+1),
				false, false),
		),
		slack.NewDividerBlock(),
		slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType,
				fmt.Sprintf("*Alert:* %s\n*Severity:* %s\n*Source:* %s\n*Tier:* %d of escalation policy",
					alert.Title, string(alert.Severity), alert.Source, tierIndex+1),
				false, false),
			nil, nil,
		),
		slack.NewActionBlock("",
			slack.NewButtonBlockElement(
				fmt.Sprintf("acknowledge_%s", alert.ID.String()),
				alert.ID.String(),
				slack.NewTextBlockObject(slack.PlainTextType, "✅ Acknowledge", false, false),
			),
		),
		slack.NewContextBlock("",
			slack.NewTextBlockObject(slack.MarkdownType,
				"Powered by <https://github.com/openincident/openincident|Fluidify Alert>",
				false, false),
		),
	}

	return Message{Text: text, Blocks: blocksToInterfaces(blocks)}
}
