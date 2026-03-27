package services

import (
	"fmt"

	"github.com/fluidify/regen/internal/models"
)

// teamsIncidentCard returns an Adaptive Card payload (as a Go map, to be JSON-serialised
// when posting via Graph API) representing the current incident state.
// The card is designed to show the same key information as the Slack incident card.
func teamsIncidentCard(incident *models.Incident) map[string]interface{} {
	statusEmoji := teamsStatusEmoji(string(incident.Status))
	severityColor := teamsSeverityColor(string(incident.Severity))

	body := []map[string]interface{}{
		{
			"type":   "TextBlock",
			"size":   "Large",
			"weight": "Bolder",
			"text":   fmt.Sprintf("%s INC-%d: %s", statusEmoji, incident.IncidentNumber, incident.Title),
			"wrap":   true,
			"color":  severityColor,
		},
		{
			"type": "FactSet",
			"facts": []map[string]interface{}{
				{"title": "Status", "value": string(incident.Status)},
				{"title": "Severity", "value": string(incident.Severity)},
				{"title": "Triggered", "value": incident.TriggeredAt.Format("2006-01-02 15:04 UTC")},
			},
		},
	}

	if incident.Summary != "" {
		body = append(body, map[string]interface{}{
			"type":      "TextBlock",
			"text":      incident.Summary,
			"wrap":      true,
			"isSubtle":  true,
			"separator": true,
		})
	}

	return map[string]interface{}{
		"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
		"type":    "AdaptiveCard",
		"version": "1.4",
		"body":    body,
	}
}

// teamsStatusUpdateCard builds a compact card for status change notifications.
func teamsStatusUpdateCard(incident *models.Incident, changedBy string) map[string]interface{} {
	emoji := teamsStatusEmoji(string(incident.Status))
	return map[string]interface{}{
		"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
		"type":    "AdaptiveCard",
		"version": "1.4",
		"body": []map[string]interface{}{
			{
				"type":   "TextBlock",
				"text":   fmt.Sprintf("%s INC-%d status changed to **%s**", emoji, incident.IncidentNumber, incident.Status),
				"wrap":   true,
				"weight": "Bolder",
			},
			{
				"type":     "TextBlock",
				"text":     fmt.Sprintf("Updated by: %s", changedBy),
				"isSubtle": true,
			},
		},
	}
}


// teamsDMCard returns an Adaptive Card for a proactive on-call DM.
// Uses OpenUrl actions so responders can act even outside the Teams channel context.
func teamsDMCard(incident *models.Incident, appURL string) map[string]interface{} {
	statusEmoji := teamsStatusEmoji(string(incident.Status))
	severityColor := teamsSeverityColor(string(incident.Severity))

	incidentURL := fmt.Sprintf("%s/incidents/%s", appURL, incident.ID)

	return map[string]interface{}{
		"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
		"type":    "AdaptiveCard",
		"version": "1.4",
		"body": []map[string]interface{}{
			{
				"type":   "TextBlock",
				"size":   "Medium",
				"weight": "Bolder",
				"text":   "🚨 You're on-call — new incident",
				"wrap":   true,
			},
			{
				"type":   "TextBlock",
				"size":   "Large",
				"weight": "Bolder",
				"text":   fmt.Sprintf("%s INC-%d: %s", statusEmoji, incident.IncidentNumber, incident.Title),
				"wrap":   true,
				"color":  severityColor,
			},
			{
				"type": "FactSet",
				"facts": []map[string]interface{}{
					{"title": "Severity", "value": string(incident.Severity)},
					{"title": "Status", "value": string(incident.Status)},
					{"title": "Triggered", "value": incident.TriggeredAt.Format("2006-01-02 15:04 UTC")},
				},
			},
		},
		"actions": []map[string]interface{}{
			{
				"type":  "Action.OpenUrl",
				"title": "View Incident",
				"url":   incidentURL,
			},
		},
	}
}

func teamsStatusEmoji(status string) string {
	switch status {
	case "triggered":
		return "🔴"
	case "acknowledged":
		return "🟡"
	case "resolved":
		return "✅"
	case "canceled":
		return "⛔"
	default:
		return "⚪"
	}
}

func teamsSeverityColor(severity string) string {
	switch severity {
	case "critical":
		return "Attention"
	case "high":
		return "Warning"
	case "medium":
		return "Default"
	case "low":
		return "Good"
	default:
		return "Default"
	}
}
