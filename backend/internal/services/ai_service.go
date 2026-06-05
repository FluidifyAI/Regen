package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/integrations/llm"
	"github.com/FluidifyAI/Regen/backend/internal/models"
)

// AIService provides AI-powered features for incidents.
// When no provider is configured, the service is disabled; callers must check IsEnabled().
type AIService interface {
	// IsEnabled returns true if the AI service is properly configured.
	IsEnabled() bool

	// Model returns the name of the currently configured LLM model (e.g. "gpt-4o").
	// Returns an empty string when the service is disabled.
	Model() string

	// Reload reinitialises the LLM clients with the given provider and credentials.
	// Passing an empty apiKey (for cloud providers) or empty ollamaBaseURL disables AI features.
	Reload(provider, apiKey, model, ollamaBaseURL string)

	GenerateSummary(ctx context.Context, incident *models.Incident, timeline []models.TimelineEntry, alerts []models.Alert, slackMessages []string) (string, llm.Usage, error)
	GenerateHandoffDigest(ctx context.Context, incident *models.Incident, timeline []models.TimelineEntry, alerts []models.Alert) (string, llm.Usage, error)
	GeneratePostMortem(ctx context.Context, incident *models.Incident, timeline []models.TimelineEntry, alerts []models.Alert, sections []string) (string, llm.Usage, error)
	EnhancePostMortem(ctx context.Context, content string) (string, llm.Usage, error)
	EnhanceIncidentDraft(ctx context.Context, brief string) (title, summary string, usage llm.Usage, err error)
	AnswerQuestion(ctx context.Context, question string, current *models.Incident, similar []models.Incident, postMortems map[string]string) (string, llm.Usage, error)
}

// NewAIService creates a reloadable AIService backed by the given provider.
// If the provider cannot be initialised (e.g. empty key, missing Ollama URL) the
// service starts disabled; call Reload after credentials are saved in Settings.
func NewAIService(provider, apiKey, model string, maxTokens, pmMaxTokens int, ollamaBaseURL string) AIService {
	svc := &aiService{maxTokens: maxTokens, pmMaxTokens: pmMaxTokens, model: model}
	svc.client, _ = llm.New(llm.Config{Provider: provider, APIKey: apiKey, Model: model, MaxTokens: maxTokens, BaseURL: ollamaBaseURL})
	svc.pmClient, _ = llm.New(llm.Config{Provider: provider, APIKey: apiKey, Model: model, MaxTokens: pmMaxTokens, BaseURL: ollamaBaseURL})
	// Treat a client with no usable credential as disabled.
	if !svc.hasCredential(provider, apiKey, ollamaBaseURL) {
		svc.client = nil
		svc.pmClient = nil
	}
	return svc
}

// ─── Implementation ───────────────────────────────────────────────────────────

type aiService struct {
	mu          sync.RWMutex
	client      llm.Client
	pmClient    llm.Client
	maxTokens   int
	pmMaxTokens int
	model       string
}

func (s *aiService) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.client != nil
}

func (s *aiService) Model() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.model
}

// hasCredential returns true when the provider has enough configuration to attempt a call.
func (s *aiService) hasCredential(provider, apiKey, ollamaBaseURL string) bool {
	switch provider {
	case "ollama":
		return ollamaBaseURL != ""
	default: // openai, anthropic
		return apiKey != ""
	}
}

func (s *aiService) Reload(provider, apiKey, model, ollamaBaseURL string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.model = model
	if !s.hasCredential(provider, apiKey, ollamaBaseURL) {
		s.client = nil
		s.pmClient = nil
		return
	}
	s.client, _ = llm.New(llm.Config{Provider: provider, APIKey: apiKey, Model: model, MaxTokens: s.maxTokens, BaseURL: ollamaBaseURL})
	s.pmClient, _ = llm.New(llm.Config{Provider: provider, APIKey: apiKey, Model: model, MaxTokens: s.pmMaxTokens, BaseURL: ollamaBaseURL})
}

func (s *aiService) GenerateSummary(ctx context.Context, incident *models.Incident, timeline []models.TimelineEntry, alerts []models.Alert, slackMessages []string) (string, llm.Usage, error) {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()
	if client == nil {
		return "", llm.Usage{}, fmt.Errorf("AI features are not configured")
	}
	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: buildSummaryPrompt(incident, timeline, alerts, slackMessages)},
	}
	text, usage, err := client.Complete(ctx, messages)
	return text, usage, err
}

func (s *aiService) GenerateHandoffDigest(ctx context.Context, incident *models.Incident, timeline []models.TimelineEntry, alerts []models.Alert) (string, llm.Usage, error) {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()
	if client == nil {
		return "", llm.Usage{}, fmt.Errorf("AI features are not configured")
	}
	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: buildHandoffPrompt(incident, timeline, alerts)},
	}
	text, usage, err := client.Complete(ctx, messages)
	return text, usage, err
}

func (s *aiService) GeneratePostMortem(ctx context.Context, incident *models.Incident, timeline []models.TimelineEntry, alerts []models.Alert, sections []string) (string, llm.Usage, error) {
	s.mu.RLock()
	client := s.pmClient
	s.mu.RUnlock()
	if client == nil {
		return "", llm.Usage{}, fmt.Errorf("AI features are not configured")
	}
	messages := []llm.Message{
		{Role: "system", Content: postMortemSystemPrompt},
		{Role: "user", Content: buildPostMortemPrompt(incident, timeline, alerts, sections)},
	}
	text, usage, err := client.Complete(ctx, messages)
	return text, usage, err
}

func (s *aiService) EnhancePostMortem(ctx context.Context, content string) (string, llm.Usage, error) {
	s.mu.RLock()
	client := s.pmClient
	s.mu.RUnlock()
	if client == nil {
		return "", llm.Usage{}, fmt.Errorf("AI features are not configured")
	}
	messages := []llm.Message{
		{Role: "system", Content: postMortemSystemPrompt},
		{Role: "user", Content: buildEnhancePrompt(content)},
	}
	text, usage, err := client.Complete(ctx, messages)
	return text, usage, err
}

func (s *aiService) EnhanceIncidentDraft(ctx context.Context, brief string) (string, string, llm.Usage, error) {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()
	if client == nil {
		return "", "", llm.Usage{}, fmt.Errorf("AI features are not configured")
	}
	messages := []llm.Message{
		{Role: "system", Content: `You are an expert incident management assistant. Convert rough incident descriptions into professional incident titles and summaries. Always respond with valid JSON only, no markdown, no commentary.`},
		{Role: "user", Content: buildEnhanceDraftPrompt(brief)},
	}
	raw, usage, err := client.Complete(ctx, messages)
	if err != nil {
		return "", "", usage, fmt.Errorf("AI enhance draft: %w", err)
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
		return "", "", usage, fmt.Errorf("AI returned unexpected format: %w", err)
	}
	if result.Title == "" {
		return "", "", usage, fmt.Errorf("AI returned an empty title")
	}
	return result.Title, result.Summary, usage, nil
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

func (s *aiService) AnswerQuestion(ctx context.Context, question string, current *models.Incident, similar []models.Incident, postMortems map[string]string) (string, llm.Usage, error) {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()
	if client == nil {
		return "AI features are not configured. Add an AI provider key in Settings → System.", llm.Usage{}, nil
	}
	messages := []llm.Message{
		{Role: "system", Content: buildAnswerQuestionSystemPrompt()},
		{Role: "user", Content: buildAnswerQuestionPrompt(question, current, similar, postMortems)},
	}
	reply, usage, err := client.Complete(ctx, messages)
	if err != nil {
		return "", usage, fmt.Errorf("AI answer question: %w", err)
	}
	return strings.TrimSpace(reply), usage, nil
}

func buildAnswerQuestionSystemPrompt() string {
	return `You are Fluidify Regen, an AI incident management assistant embedded in Slack. You help on-call engineers during live incidents by answering questions concisely and accurately.

STRICT FORMATTING RULES — Slack mrkdwn only:
- Bold: *text* (never ** or ***)
- Bullet: • or - (never ###, ####, or other markdown headers)
- Code: ` + "`" + `code` + "`" + ` or ` + "```" + `block` + "```" + `
- No markdown headers (#, ##, ###, ####) ever — use *bold* for section labels instead
- No horizontal rules (---, ***)
- No HTML

Keep responses under 200 words. Never fabricate details. If unsure, say so. When referencing post-mortem details, quote them accurately.`
}

func buildAnswerQuestionPrompt(question string, current *models.Incident, similar []models.Incident, postMortems map[string]string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Current incident: *INC-%d* \u2014 %s\nStatus: %s | Severity: %s\n",
		current.IncidentNumber, current.Title, current.Status, current.Severity))
	if current.Summary != "" {
		b.WriteString(fmt.Sprintf("Summary: %s\n", current.Summary))
	}
	b.WriteString("\n")
	if len(similar) > 0 {
		b.WriteString(fmt.Sprintf("Recent incidents (%d in past 90 days):\n", len(similar)))
		for _, inc := range similar {
			duration := "ongoing"
			if inc.ResolvedAt != nil {
				duration = fmt.Sprintf("MTTR %.0f min", inc.ResolvedAt.Sub(inc.TriggeredAt).Minutes())
			}
			hasPM := ""
			if _, ok := postMortems[fmt.Sprintf("INC-%d", inc.IncidentNumber)]; ok {
				hasPM = " [has post-mortem]"
			}
			b.WriteString(fmt.Sprintf("\u2022 INC-%d: %s (%s, %s)%s\n", inc.IncidentNumber, inc.Title, inc.Severity, duration, hasPM))
		}
		b.WriteString("\n")
	} else {
		b.WriteString("No similar recent incidents found.\n\n")
	}
	// Include full post-mortem content for any incidents that have one
	if len(postMortems) > 0 {
		b.WriteString("--- Post-Mortem Reports ---\n")
		for incRef, pmContent := range postMortems {
			// Truncate to ~2000 chars to stay within token budget
			truncated := pmContent
			if len(truncated) > 2000 {
				truncated = truncated[:2000] + "... [truncated]"
			}
			b.WriteString(fmt.Sprintf("\nPost-mortem for %s:\n%s\n", incRef, truncated))
		}
		b.WriteString("\n")
	}
	b.WriteString(fmt.Sprintf("Question from engineer: %s", question))
	return b.String()
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
