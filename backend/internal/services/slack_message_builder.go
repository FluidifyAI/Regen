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

	// Action buttons
	blocks = append(blocks, slack.NewActionBlock(
		"incident_actions",
		slack.NewButtonBlockElement(
			"acknowledge",
			incident.ID.String(),
			slack.NewTextBlockObject(
				slack.PlainTextType,
				"👀 Acknowledge",
				false,
				false,
			),
		).WithStyle(slack.StylePrimary),
		slack.NewButtonBlockElement(
			"resolve",
			incident.ID.String(),
			slack.NewTextBlockObject(
				slack.PlainTextType,
				"✅ Resolve",
				false,
				false,
			),
		).WithStyle(slack.StyleDanger),
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
