package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/fluidify/regen/internal/integrations/openai"
	"github.com/fluidify/regen/internal/models"
)

// AIService provides AI-powered features for incidents.
// When OpenAI is not configured, NewAIService returns a noopAIService that
// satisfies the interface but returns clear errors — callers must check IsEnabled().
type AIService interface {
	// IsEnabled returns true if the AI service is properly configured.
	IsEnabled() bool

	// Reload replaces the API key and re-initialises the OpenAI clients in place.
	// Calling with an empty string disables AI features until a key is set again.
	Reload(apiKey string)

	// GenerateSummary generates a concise incident summary using all available context.
	// slackMessages should be the plain-text messages from the incident Slack thread.
	// Pass an empty slice when Slack is not configured or the channel has no messages.
	GenerateSummary(ctx context.Context, incident *models.Incident, timeline []models.TimelineEntry, alerts []models.Alert, slackMessages []string) (string, error)

	// GenerateHandoffDigest generates a structured shift handoff document.
	// Suitable for posting to Slack or displaying in the UI at shift change.
	GenerateHandoffDigest(ctx context.Context, incident *models.Incident, timeline []models.TimelineEntry, alerts []models.Alert) (string, error)

	// GeneratePostMortem generates a full post-mortem document in Markdown.
	// sections is the ordered list of section names from the chosen template.
	// Uses a higher token budget than summary generation.
	GeneratePostMortem(ctx context.Context, incident *models.Incident, timeline []models.TimelineEntry, alerts []models.Alert, sections []string) (string, error)

	// EnhancePostMortem improves an existing post-mortem draft for clarity,
	// structure, and completeness while preserving all factual details.
	EnhancePostMortem(ctx context.Context, content string) (string, error)

	// EnhanceIncidentDraft converts a rough user brief into a polished incident
	// title and summary. Returns structured title + summary strings.
	EnhanceIncidentDraft(ctx context.Context, brief string) (title, summary string, err error)
}

// NewAIService creates a reloadable AIService. If apiKey is empty the service
// starts disabled; call Reload(key) after saving a key from the UI.
func NewAIService(apiKey, model string, maxTokens, postMortemMaxTokens int) AIService {
	svc := &aiService{
		model:       model,
		maxTokens:   maxTokens,
		pmMaxTokens: postMortemMaxTokens,
	}
	if apiKey != "" {
		svc.client = openai.New(apiKey, model, maxTokens)
		svc.postMortemClient = openai.New(apiKey, model, postMortemMaxTokens)
	}
	return svc
}

// ─── Implementation ───────────────────────────────────────────────────────────

type aiService struct {
	mu              sync.RWMutex
	client           *openai.Client
	postMortemClient *openai.Client
	model            string
	maxTokens        int
	pmMaxTokens      int
}

func (s *aiService) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.client != nil
}

func (s *aiService) Reload(apiKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if apiKey == "" {
		s.client = nil
		s.postMortemClient = nil
	} else {
		s.client = openai.New(apiKey, s.model, s.maxTokens)
		s.postMortemClient = openai.New(apiKey, s.model, s.pmMaxTokens)
	}
}

func (s *aiService) GenerateSummary(ctx context.Context, incident *models.Incident, timeline []models.TimelineEntry, alerts []models.Alert, slackMessages []string) (string, error) {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()
	if client == nil {
		return "", fmt.Errorf("AI features are not configured")
	}
	messages := []openai.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: buildSummaryPrompt(incident, timeline, alerts, slackMessages)},
	}
	return client.Complete(ctx, messages)
}

func (s *aiService) GenerateHandoffDigest(ctx context.Context, incident *models.Incident, timeline []models.TimelineEntry, alerts []models.Alert) (string, error) {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()
	if client == nil {
		return "", fmt.Errorf("AI features are not configured")
	}
	messages := []openai.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: buildHandoffPrompt(incident, timeline, alerts)},
	}
	return client.Complete(ctx, messages)
}

func (s *aiService) GeneratePostMortem(ctx context.Context, incident *models.Incident, timeline []models.TimelineEntry, alerts []models.Alert, sections []string) (string, error) {
	s.mu.RLock()
	client := s.postMortemClient
	s.mu.RUnlock()
	if client == nil {
		return "", fmt.Errorf("AI features are not configured")
	}
	messages := []openai.ChatMessage{
		{Role: "system", Content: postMortemSystemPrompt},
		{Role: "user", Content: buildPostMortemPrompt(incident, timeline, alerts, sections)},
	}
	return client.Complete(ctx, messages)
}

func (s *aiService) EnhancePostMortem(ctx context.Context, content string) (string, error) {
	s.mu.RLock()
	client := s.postMortemClient
	s.mu.RUnlock()
	if client == nil {
		return "", fmt.Errorf("AI features are not configured")
	}
	messages := []openai.ChatMessage{
		{Role: "system", Content: postMortemSystemPrompt},
		{Role: "user", Content: buildEnhancePrompt(content)},
	}
	return client.Complete(ctx, messages)
}

func (s *aiService) EnhanceIncidentDraft(ctx context.Context, brief string) (string, string, error) {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()
	if client == nil {
		return "", "", fmt.Errorf("AI features are not configured")
	}
	messages := []openai.ChatMessage{
		{Role: "system", Content: `You are an expert incident management assistant. Convert rough incident descriptions into professional incident titles and summaries. Always respond with valid JSON only, no markdown, no commentary.`},
		{Role: "user", Content: buildEnhanceDraftPrompt(brief)},
	}
	raw, err := client.Complete(ctx, messages)
	if err != nil {
		return "", "", fmt.Errorf("AI enhance draft: %w", err)
	}
	// Strip markdown code fences if the model wraps output
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		raw = strings.Trim(raw, "`")
		raw = strings.TrimPrefix(raw, "json")
		raw = strings.TrimSpace(raw)
	}
	var result struct {
		Title   string `json:"title"`
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return "", "", fmt.Errorf("AI returned unexpected format: %w", err)
	}
	if result.Title == "" {
		return "", "", fmt.Errorf("AI returned an empty title")
	}
	return result.Title, result.Summary, nil
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

// ─── Post-mortem prompt ───────────────────────────────────────────────────────

// postMortemSystemPrompt instructs the model to produce structured Markdown.
// Unlike the summary prompt, markdown formatting is explicitly encouraged here
// since post-mortems are documents intended for export and long-term reference.
const postMortemSystemPrompt = `You are an expert incident management assistant drafting post-mortem reports. Write in structured Markdown using the section headers provided. Adopt a blameless, fact-based tone. Be precise and technical. Avoid speculation — if information is not available, say so briefly. Use bullet points within sections where appropriate.`

func buildPostMortemPrompt(incident *models.Incident, timeline []models.TimelineEntry, alerts []models.Alert, sections []string) string {
	var b strings.Builder

	// Instruct the model to use exactly the requested sections
	fmt.Fprintf(&b, "Draft a post-mortem report. Use ONLY these Markdown sections in this order:\n")
	for _, s := range sections {
		fmt.Fprintf(&b, "## %s\n", s)
	}
	fmt.Fprintf(&b, "\n")

	// Incident metadata
	fmt.Fprintf(&b, "INCIDENT: INC-%d — %s\n", incident.IncidentNumber, incident.Title)
	fmt.Fprintf(&b, "Status: %s | Severity: %s\n", incident.Status, incident.Severity)
	fmt.Fprintf(&b, "Triggered: %s\n", incident.TriggeredAt.Format(time.RFC3339))
	if incident.AcknowledgedAt != nil {
		fmt.Fprintf(&b, "Acknowledged: %s\n", incident.AcknowledgedAt.Format(time.RFC3339))
	}
	if incident.ResolvedAt != nil {
		duration := incident.ResolvedAt.Sub(incident.TriggeredAt)
		fmt.Fprintf(&b, "Resolved: %s (duration: %s)\n",
			incident.ResolvedAt.Format(time.RFC3339),
			duration.Round(time.Minute),
		)
	}
	if incident.Summary != "" {
		fmt.Fprintf(&b, "Manual summary: %s\n", incident.Summary)
	}
	if incident.AISummary != nil {
		fmt.Fprintf(&b, "AI summary: %s\n", *incident.AISummary)
	}

	// Alerts (all of them — post-mortems benefit from the full picture)
	if len(alerts) > 0 {
		fmt.Fprintf(&b, "\nTRIGGERING ALERTS (%d):\n", len(alerts))
		for i, a := range alerts {
			if i >= 10 {
				fmt.Fprintf(&b, "  ... and %d more\n", len(alerts)-10)
				break
			}
			fmt.Fprintf(&b, "  - [%s/%s] %s: %s\n", a.Source, a.Severity, a.Title, a.Description)
		}
	}

	// Full timeline (post-mortems need the complete picture, not a summary cap)
	if len(timeline) > 0 {
		fmt.Fprintf(&b, "\nFULL TIMELINE (%d entries):\n", len(timeline))
		for _, e := range timeline {
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

func buildEnhancePrompt(content string) string {
	return fmt.Sprintf(`Enhance the following incident post-mortem for clarity, completeness, and professional structure. Preserve all facts, timeline details, and technical specifics exactly. Improve headings, fix grammar, ensure action items are clearly stated, and add any missing standard sections (summary, impact, root cause, timeline, action items). Return only the improved markdown with no commentary.

Current post-mortem:
%s`, content)
}

func buildEnhanceDraftPrompt(brief string) string {
	return fmt.Sprintf(`Convert this rough incident description into a professional incident title and summary.

Rules:
- Title: concise (max 10 words), action-oriented, names the affected system and symptom (e.g. "API Gateway 5xx errors causing checkout failures")
- Summary: 2-3 sentences describing what is happening, suspected impact, and any known context. Plain text, no markdown.
- Respond ONLY with valid JSON: {"title": "...", "summary": "..."}

User description:
%s`, brief)
}
