package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// fakeClient is a Client test double whose behavior (and observed ctx) is
// fully controlled by the test.
type fakeClient struct {
	text        string
	usage       Usage
	err         error
	observedCtx context.Context
}

func (f *fakeClient) Complete(ctx context.Context, messages []Message) (string, Usage, error) {
	f.observedCtx = ctx
	return f.text, f.usage, f.err
}

func newRecordedTracer(t *testing.T) (*sdktrace.TracerProvider, *tracetest.SpanRecorder) {
	t.Helper()
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() { tp.Shutdown(context.Background()) })
	return tp, sr
}

func testMessages() []Message {
	return []Message{
		{Role: "system", Content: "You are a helpful incident assistant."},
		{Role: "user", Content: "Summarize this incident: database is down."},
	}
}

func findAttr(span sdktrace.ReadOnlySpan, key string) (string, bool) {
	for _, a := range span.Attributes() {
		if string(a.Key) == key {
			return a.Value.Emit(), true
		}
	}
	return "", false
}

func TestInstrumentedClient_SetsGenAIAttributes(t *testing.T) {
	tp, sr := newRecordedTracer(t)
	inner := &fakeClient{text: "summary text", usage: Usage{PromptTokens: 10, CompletionTokens: 5}}
	c := &instrumentedClient{inner: inner, system: "openai", model: "gpt-4o", tracer: tp.Tracer("test")}

	_, _, err := c.Complete(context.Background(), testMessages())
	if err != nil {
		t.Fatalf("Complete: unexpected error: %v", err)
	}

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]

	if got, ok := findAttr(span, "gen_ai.system"); !ok || got != "openai" {
		t.Errorf("gen_ai.system = %q, ok=%v, want %q", got, ok, "openai")
	}
	if got, ok := findAttr(span, "gen_ai.request.model"); !ok || got != "gpt-4o" {
		t.Errorf("gen_ai.request.model = %q, ok=%v, want %q", got, ok, "gpt-4o")
	}
	if got, ok := findAttr(span, "gen_ai.operation.name"); !ok || got != "chat" {
		t.Errorf("gen_ai.operation.name = %q, ok=%v, want %q", got, ok, "chat")
	}
}

func TestInstrumentedClient_SetsPromptHash_MatchingExpectedDigest(t *testing.T) {
	tp, sr := newRecordedTracer(t)
	messages := testMessages()
	inner := &fakeClient{text: "x", usage: Usage{}}
	c := &instrumentedClient{inner: inner, system: "openai", model: "gpt-4o", tracer: tp.Tracer("test")}

	_, _, _ = c.Complete(context.Background(), messages)

	span := sr.Ended()[0]
	got, ok := findAttr(span, "regen.llm.prompt_hash")
	if !ok {
		t.Fatal("regen.llm.prompt_hash attribute not set")
	}

	want := sha256.Sum256([]byte(renderMessages(messages)))
	wantHex := hex.EncodeToString(want[:])
	if got != wantHex {
		t.Errorf("regen.llm.prompt_hash = %q, want %q (sha256 of rendered messages)", got, wantHex)
	}
}

func TestInstrumentedClient_RecordsUsageTokensAndEstimatedCost(t *testing.T) {
	tp, sr := newRecordedTracer(t)
	inner := &fakeClient{text: "x", usage: Usage{PromptTokens: 1_000_000, CompletionTokens: 1_000_000}}
	c := &instrumentedClient{inner: inner, system: "openai", model: "gpt-4o", tracer: tp.Tracer("test")}

	_, _, _ = c.Complete(context.Background(), testMessages())

	span := sr.Ended()[0]
	if got, ok := findAttr(span, "gen_ai.usage.input_tokens"); !ok || got != "1000000" {
		t.Errorf("gen_ai.usage.input_tokens = %q, ok=%v, want 1000000", got, ok)
	}
	if got, ok := findAttr(span, "gen_ai.usage.output_tokens"); !ok || got != "1000000" {
		t.Errorf("gen_ai.usage.output_tokens = %q, ok=%v, want 1000000", got, ok)
	}
	// gpt-4o: $2.50/1M input + $10.00/1M output = 12.5 for 1M+1M tokens.
	if got, ok := findAttr(span, "regen.llm.cost_usd"); !ok || got != "12.5" {
		t.Errorf("regen.llm.cost_usd = %q, ok=%v, want 12.5", got, ok)
	}
}

func TestInstrumentedClient_UnknownModel_CostZero(t *testing.T) {
	tp, sr := newRecordedTracer(t)
	inner := &fakeClient{text: "x", usage: Usage{PromptTokens: 100, CompletionTokens: 100}}
	c := &instrumentedClient{inner: inner, system: "ollama", model: "llama3.2", tracer: tp.Tracer("test")}

	_, _, _ = c.Complete(context.Background(), testMessages())

	span := sr.Ended()[0]
	if got, ok := findAttr(span, "regen.llm.cost_usd"); !ok || got != "0" {
		t.Errorf("regen.llm.cost_usd = %q, ok=%v, want 0 for an unpriced model", got, ok)
	}
}

func TestInstrumentedClient_SetsAgentNameAndIncidentID_WhenCallMetaPresent(t *testing.T) {
	tp, sr := newRecordedTracer(t)
	inner := &fakeClient{text: "x", usage: Usage{}}
	c := &instrumentedClient{inner: inner, system: "openai", model: "gpt-4o", tracer: tp.Tracer("test")}

	ctx := WithCallMeta(context.Background(), CallMeta{AgentName: "Incident Summarizer", IncidentID: "inc-42"})
	_, _, _ = c.Complete(ctx, testMessages())

	span := sr.Ended()[0]
	if got, ok := findAttr(span, "gen_ai.agent.name"); !ok || got != "Incident Summarizer" {
		t.Errorf("gen_ai.agent.name = %q, ok=%v, want %q", got, ok, "Incident Summarizer")
	}
	if got, ok := findAttr(span, "incident.id"); !ok || got != "inc-42" {
		t.Errorf("incident.id = %q, ok=%v, want %q", got, ok, "inc-42")
	}
}

func TestInstrumentedClient_OmitsAgentNameAndIncidentID_WhenCallMetaAbsent(t *testing.T) {
	tp, sr := newRecordedTracer(t)
	inner := &fakeClient{text: "x", usage: Usage{}}
	c := &instrumentedClient{inner: inner, system: "openai", model: "gpt-4o", tracer: tp.Tracer("test")}

	_, _, _ = c.Complete(context.Background(), testMessages())

	span := sr.Ended()[0]
	if _, ok := findAttr(span, "gen_ai.agent.name"); ok {
		t.Error("gen_ai.agent.name should be absent when no CallMeta was set")
	}
	if _, ok := findAttr(span, "incident.id"); ok {
		t.Error("incident.id should be absent when no CallMeta was set")
	}
}

func TestInstrumentedClient_RecordsErrorStatus_OnInnerError(t *testing.T) {
	tp, sr := newRecordedTracer(t)
	innerErr := errors.New("openai: rate limited")
	inner := &fakeClient{err: innerErr}
	c := &instrumentedClient{inner: inner, system: "openai", model: "gpt-4o", tracer: tp.Tracer("test")}

	_, _, err := c.Complete(context.Background(), testMessages())
	if !errors.Is(err, innerErr) {
		t.Fatalf("Complete returned %v, want the inner error propagated unchanged", err)
	}

	span := sr.Ended()[0]
	if span.Status().Code != codes.Error {
		t.Errorf("span status code = %v, want Error", span.Status().Code)
	}
	events := span.Events()
	foundException := false
	for _, e := range events {
		if e.Name == "exception" {
			foundException = true
		}
	}
	if !foundException {
		t.Error("expected span.RecordError to add an exception event")
	}
}

func TestInstrumentedClient_PropagatesReturnValuesUnchanged(t *testing.T) {
	tp, _ := newRecordedTracer(t)
	inner := &fakeClient{text: "the answer", usage: Usage{PromptTokens: 7, CompletionTokens: 3}}
	c := &instrumentedClient{inner: inner, system: "anthropic", model: "claude-3-5-sonnet-20241022", tracer: tp.Tracer("test")}

	text, usage, err := c.Complete(context.Background(), testMessages())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "the answer" {
		t.Errorf("text = %q, want %q", text, "the answer")
	}
	if usage != (Usage{PromptTokens: 7, CompletionTokens: 3}) {
		t.Errorf("usage = %+v, want %+v", usage, Usage{PromptTokens: 7, CompletionTokens: 3})
	}
}

func TestInstrumentedClient_NeverRecordsPromptOrCompletionText_ByDefault(t *testing.T) {
	clearGenAIContentEnvForLLM(t)

	tp, sr := newRecordedTracer(t)
	messages := testMessages()
	inner := &fakeClient{text: "the secret incident summary", usage: Usage{PromptTokens: 1, CompletionTokens: 1}}
	c := &instrumentedClient{inner: inner, system: "openai", model: "gpt-4o", tracer: tp.Tracer("test")}

	_, _, _ = c.Complete(context.Background(), messages)

	span := sr.Ended()[0]
	if len(span.Events()) != 0 {
		t.Errorf("expected zero span events by default, got %d: %+v", len(span.Events()), span.Events())
	}
	for _, a := range span.Attributes() {
		v := a.Value.Emit()
		if strings.Contains(v, "database is down") || strings.Contains(v, "helpful incident assistant") || strings.Contains(v, "the secret incident summary") {
			t.Errorf("span attribute %s leaked prompt/completion text: %q", a.Key, v)
		}
	}
}

func TestInstrumentedClient_RecordsPromptAndCompletionText_WhenEnvVarEnabled(t *testing.T) {
	clearGenAIContentEnvForLLM(t)
	os.Setenv("OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT", "true")

	tp, sr := newRecordedTracer(t)
	messages := testMessages()
	inner := &fakeClient{text: "the secret incident summary", usage: Usage{PromptTokens: 1, CompletionTokens: 1}}
	c := &instrumentedClient{inner: inner, system: "openai", model: "gpt-4o", tracer: tp.Tracer("test")}

	_, _, _ = c.Complete(context.Background(), messages)

	span := sr.Ended()[0]
	events := span.Events()
	if len(events) == 0 {
		t.Fatal("expected span events with OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT=true, got none")
	}

	var sawPrompt, sawCompletion bool
	for _, e := range events {
		for _, a := range e.Attributes {
			v := a.Value.Emit()
			if strings.Contains(v, "database is down") {
				sawPrompt = true
			}
			if strings.Contains(v, "the secret incident summary") {
				sawCompletion = true
			}
		}
	}
	if !sawPrompt {
		t.Error("expected an event carrying the prompt text when opted in")
	}
	if !sawCompletion {
		t.Error("expected an event carrying the completion text when opted in")
	}
}

func clearGenAIContentEnvForLLM(t *testing.T) {
	t.Helper()
	old, had := os.LookupEnv("OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT")
	os.Unsetenv("OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT")
	t.Cleanup(func() {
		if had {
			os.Setenv("OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT", old)
		} else {
			os.Unsetenv("OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT")
		}
	})
}
