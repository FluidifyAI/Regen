package services

// White-box (internal) tests proving REG-12's context-based CallMeta wiring:
// aiService sets llm.CallMeta on the ctx it hands to client.Complete, for
// every method that has an incident or a named operation available. Kept
// separate from ai_service_test.go (package services_test) because it needs
// direct access to aiService's unexported fields to inject a capturing fake
// client — there is no other seam to observe what ctx a real llm.Client
// receives.

import (
	"context"
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/integrations/llm"
	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/google/uuid"
)

// capturingClient records the ctx it was called with, so tests can assert
// what llm.CallMetaFromContext(ctx) returns. response defaults to a plain
// string reply; set it explicitly for callers (EnhanceIncidentDraft) that
// parse the reply as JSON.
type capturingClient struct {
	observedCtx context.Context
	response    string
}

func (c *capturingClient) Complete(ctx context.Context, _ []llm.Message) (string, llm.Usage, error) {
	c.observedCtx = ctx
	if c.response != "" {
		return c.response, llm.Usage{}, nil
	}
	return "reply", llm.Usage{}, nil
}

func TestAIService_GenerateSummary_SetsCallMetaWithIncidentID(t *testing.T) {
	fake := &capturingClient{}
	incident := &models.Incident{ID: uuid.New()}
	svc := &aiService{client: fake}

	_, _, err := svc.GenerateSummary(context.Background(), incident, nil, nil, nil)
	if err != nil {
		t.Fatalf("GenerateSummary: unexpected error: %v", err)
	}

	meta := llm.CallMetaFromContext(fake.observedCtx)
	if meta.AgentName == "" {
		t.Error("expected a non-empty AgentName on the ctx passed to Complete")
	}
	if meta.IncidentID != incident.ID.String() {
		t.Errorf("meta.IncidentID = %q, want %q", meta.IncidentID, incident.ID.String())
	}
}

func TestAIService_GeneratePostMortem_SetsCallMetaWithIncidentID(t *testing.T) {
	fake := &capturingClient{}
	incident := &models.Incident{ID: uuid.New()}
	svc := &aiService{pmClient: fake}

	_, _, err := svc.GeneratePostMortem(context.Background(), incident, nil, nil, nil)
	if err != nil {
		t.Fatalf("GeneratePostMortem: unexpected error: %v", err)
	}

	meta := llm.CallMetaFromContext(fake.observedCtx)
	if meta.IncidentID != incident.ID.String() {
		t.Errorf("meta.IncidentID = %q, want %q", meta.IncidentID, incident.ID.String())
	}
}

func TestAIService_AnswerQuestion_SetsIncidentID_WhenCurrentGiven(t *testing.T) {
	fake := &capturingClient{}
	incident := &models.Incident{ID: uuid.New()}
	svc := &aiService{client: fake}

	_, _, err := svc.AnswerQuestion(context.Background(), "why?", incident, nil, nil)
	if err != nil {
		t.Fatalf("AnswerQuestion: unexpected error: %v", err)
	}

	meta := llm.CallMetaFromContext(fake.observedCtx)
	if meta.IncidentID != incident.ID.String() {
		t.Errorf("meta.IncidentID = %q, want %q", meta.IncidentID, incident.ID.String())
	}
}

// Note: AnswerQuestion(ctx, question, current, ...) accepts current == nil
// per its signature, but buildAnswerQuestionPrompt dereferences current
// unconditionally and panics if it is — a pre-existing bug, not something
// REG-12 introduces or is scoped to fix. The single production caller
// (SlackEventHandler, slack_event_handler.go) already guards incident == nil
// before calling AnswerQuestion, so this never fires today; flagged
// separately rather than exercised here, since doing so would just crash
// this test on an unrelated bug instead of proving anything about CallMeta.

func TestAIService_EnhanceIncidentDraft_SetsAgentName_NoIncidentYet(t *testing.T) {
	fake := &capturingClient{response: `{"title": "DB down", "summary": "database is down"}`}
	svc := &aiService{client: fake}

	_, _, _, err := svc.EnhanceIncidentDraft(context.Background(), "db is down")
	if err != nil {
		t.Fatalf("EnhanceIncidentDraft: unexpected error: %v", err)
	}

	meta := llm.CallMetaFromContext(fake.observedCtx)
	if meta.AgentName == "" {
		t.Error("expected a non-empty AgentName")
	}
	if meta.IncidentID != "" {
		t.Errorf("meta.IncidentID = %q, want empty — no incident exists yet at draft time", meta.IncidentID)
	}
}
