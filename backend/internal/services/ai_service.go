package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openincident/openincident/internal/integrations/openai"
	"github.com/openincident/openincident/internal/models"
)

// AIService provides AI-powered features for incidents.
// When OpenAI is not configured, NewAIService returns a noopAIService that
// satisfies the interface but returns clear errors — callers must check IsEnabled().
type AIService interface {
	// IsEnabled returns true if the AI service is properly configured.
	IsEnabled() bool

	// GenerateSummary generates a concise incident summary using all available context.
	// slackMessages should be the plain-text messages from the incident Slack thread.
	// Pass an empty slice when Slack is not configured or the channel has no messages.
	GenerateSummary(ctx context.Context, incident *models.Incident, timeline []models.TimelineEntry, alerts []models.Alert, slackMessages []string) (string, error)

	// GenerateHandoffDigest generates a structured shift handoff document.
	// Suitable for posting to Slack or displaying in the UI at shift change.
	GenerateHandoffDigest(ctx context.Context, incident *models.Incident, timeline []models.TimelineEntry, alerts []models.Alert) (string, error)
}

// NewAIService creates an AIService backed by OpenAI, or a noop if apiKey is empty.
func NewAIService(apiKey, model string, maxTokens int) AIService {
	if apiKey == "" {
		return &noopAIService{}
	}
	return &aiService{
		client: openai.New(apiKey, model, maxTokens),
	}
}

// ─── Real implementation ──────────────────────────────────────────────────────

type aiService struct {
	client *openai.Client
}

func (s *aiService) IsEnabled() bool { return true }

func (s *aiService) GenerateSummary(ctx context.Context, incident *models.Incident, timeline []models.TimelineEntry, alerts []models.Alert, slackMessages []string) (string, error) {
	messages := []openai.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: buildSummaryPrompt(incident, timeline, alerts, slackMessages)},
	}
	return s.client.Complete(ctx, messages)
}

func (s *aiService) GenerateHandoffDigest(ctx context.Context, incident *models.Incident, timeline []models.TimelineEntry, alerts []models.Alert) (string, error) {
	messages := []openai.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: buildHandoffPrompt(incident, timeline, alerts)},
	}
	return s.client.Complete(ctx, messages)
}

// ─── Noop implementation (AI not configured) ─────────────────────────────────

type noopAIService struct{}

func (n *noopAIService) IsEnabled() bool { return false }
func (n *noopAIService) GenerateSummary(_ context.Context, _ *models.Incident, _ []models.TimelineEntry, _ []models.Alert, _ []string) (string, error) {
	return "", fmt.Errorf("AI features are not configured: set OPENAI_API_KEY to enable")
}
func (n *noopAIService) GenerateHandoffDigest(_ context.Context, _ *models.Incident, _ []models.TimelineEntry, _ []models.Alert) (string, error) {
	return "", fmt.Errorf("AI features are not configured: set OPENAI_API_KEY to enable")
}

// ─── Prompts ──────────────────────────────────────────────────────────────────

const systemPrompt = `You are an expert incident management assistant helping on-call engineers. Be concise, technical, and actionable. Write in plain text (no markdown formatting). Focus on what happened, why, and what actions were taken.`

func buildSummaryPrompt(incident *models.Incident, timeline []models.TimelineEntry, alerts []models.Alert, slackMessages []string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Summarize this incident in 3-5 sentences covering: what happened, the impact, key actions taken, and current status.\n\n")
	fmt.Fprintf(&b, "INCIDENT: INC-%d - %s\n", incident.IncidentNumber, incident.Title)
	fmt.Fprintf(&b, "Status: %s | Severity: %s\n", incident.Status, incident.Severity)
	fmt.Fprintf(&b, "Triggered: %s\n", incident.TriggeredAt.Format(time.RFC3339))
	if incident.ResolvedAt != nil {
		fmt.Fprintf(&b, "Resolved: %s\n", incident.ResolvedAt.Format(time.RFC3339))
	}
	if incident.Summary != "" {
		fmt.Fprintf(&b, "Manual summary: %s\n", incident.Summary)
	}

	if len(alerts) > 0 {
		fmt.Fprintf(&b, "\nTRIGGERING ALERTS (%d):\n", len(alerts))
		for i, a := range alerts {
			if i >= 5 {
				fmt.Fprintf(&b, "  ... and %d more\n", len(alerts)-5)
				break
			}
			fmt.Fprintf(&b, "  - [%s/%s] %s: %s\n", a.Source, a.Severity, a.Title, a.Description)
		}
	}

	if len(timeline) > 0 {
		fmt.Fprintf(&b, "\nTIMELINE (%d entries):\n", len(timeline))
		for i, e := range timeline {
			if i >= 20 {
				fmt.Fprintf(&b, "  ... %d more entries\n", len(timeline)-20)
				break
			}
			content := extractTimelineText(e)
			fmt.Fprintf(&b, "  [%s] %s: %s\n", e.Timestamp.Format("15:04"), e.Type, content)
		}
	}

	if len(slackMessages) > 0 {
		fmt.Fprintf(&b, "\nSLACK THREAD (%d messages):\n", len(slackMessages))
		for i, msg := range slackMessages {
			if i >= 30 {
				fmt.Fprintf(&b, "  ... %d more messages\n", len(slackMessages)-30)
				break
			}
			fmt.Fprintf(&b, "  %s\n", msg)
		}
	}

	return b.String()
}

func buildHandoffPrompt(incident *models.Incident, timeline []models.TimelineEntry, alerts []models.Alert) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Generate a shift handoff digest for the incoming on-call engineer. Include:\n")
	fmt.Fprintf(&b, "1. Current situation (2-3 sentences)\n")
	fmt.Fprintf(&b, "2. Key events so far (bullet list)\n")
	fmt.Fprintf(&b, "3. Active concerns or open questions\n")
	fmt.Fprintf(&b, "4. Recommended next steps\n\n")

	fmt.Fprintf(&b, "INCIDENT: INC-%d - %s\n", incident.IncidentNumber, incident.Title)
	fmt.Fprintf(&b, "Status: %s | Severity: %s\n", incident.Status, incident.Severity)
	fmt.Fprintf(&b, "Triggered: %s\n", incident.TriggeredAt.Format(time.RFC3339))
	if incident.AcknowledgedAt != nil {
		fmt.Fprintf(&b, "Acknowledged: %s\n", incident.AcknowledgedAt.Format(time.RFC3339))
	}
	if incident.Summary != "" {
		fmt.Fprintf(&b, "Current summary: %s\n", incident.Summary)
	}

	if len(alerts) > 0 {
		fmt.Fprintf(&b, "\nACTIVE ALERTS:\n")
		for i, a := range alerts {
			if i >= 10 {
				fmt.Fprintf(&b, "  ... %d more\n", len(alerts)-10)
				break
			}
			fmt.Fprintf(&b, "  - [%s] %s (status: %s)\n", a.Severity, a.Title, a.Status)
		}
	}

	if len(timeline) > 0 {
		fmt.Fprintf(&b, "\nRECENT ACTIVITY:\n")
		// Show last 15 timeline entries for recency
		start := 0
		if len(timeline) > 15 {
			start = len(timeline) - 15
		}
		for _, e := range timeline[start:] {
			content := extractTimelineText(e)
			fmt.Fprintf(&b, "  [%s] %s: %s\n", e.Timestamp.Format("15:04"), e.Type, content)
		}
	}

	return b.String()
}

// extractTimelineText pulls a readable string from a timeline entry's Content JSONB.
func extractTimelineText(e models.TimelineEntry) string {
	if msg, ok := e.Content["message"].(string); ok && msg != "" {
		return msg
	}
	if text, ok := e.Content["text"].(string); ok && text != "" {
		return text
	}
	// Generic fallback: concatenate all string values
	var parts []string
	for k, v := range e.Content {
		if s, ok := v.(string); ok && s != "" {
			parts = append(parts, fmt.Sprintf("%s=%s", k, s))
		}
	}
	return strings.Join(parts, " ")
}
