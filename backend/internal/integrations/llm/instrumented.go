package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/FluidifyAI/Regen/backend/internal/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/trace"
)

// instrumentedClient wraps a provider Client with a span per completion,
// using OTel's GenAI semantic conventions (REG-12). It changes nothing about
// inner's behavior — every return value is passed through unchanged — it
// only observes the call.
//
// semconv v1.34.0 specifically: it is the last version in this module's
// vendored set that still carries gen_ai.system as a stable-named attribute
// (later spec revisions replace it with gen_ai.provider.name) — GenAI
// semconv is marked "Development" stability upstream and churns between
// versions; v1.34.0 is the one that matches the attribute names REG-12 asks
// for exactly.
type instrumentedClient struct {
	inner  Client
	system string // "openai" | "anthropic" | "ollama" -> gen_ai.system
	model  string // -> gen_ai.request.model
	tracer trace.Tracer
}

// newInstrumentedClient wraps inner for production use, reading the shared
// "regen" tracer (see observability.Tracer) rather than taking one as a
// parameter here — New's callers (NewAIService) have no test-injection need
// of their own; tests in this package construct instrumentedClient directly
// with an isolated tracer instead of going through this constructor (see
// REG-7: never touch the global TracerProvider from a test).
func newInstrumentedClient(inner Client, system, model string) *instrumentedClient {
	return &instrumentedClient{inner: inner, system: system, model: model, tracer: observability.Tracer()}
}

func (c *instrumentedClient) Complete(ctx context.Context, messages []Message) (string, Usage, error) {
	meta := CallMetaFromContext(ctx)

	ctx, span := c.tracer.Start(ctx, fmt.Sprintf("chat %s", c.model), trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	attrs := []attribute.KeyValue{
		semconv.GenAISystemKey.String(c.system),
		semconv.GenAIRequestModel(c.model),
		semconv.GenAIOperationNameChat,
		attribute.String("regen.llm.prompt_hash", promptHash(messages)),
	}
	if meta.AgentName != "" {
		attrs = append(attrs, semconv.GenAIAgentName(meta.AgentName))
	}
	if meta.IncidentID != "" {
		attrs = append(attrs, attribute.String("incident.id", meta.IncidentID))
	}
	span.SetAttributes(attrs...)

	// Prompt/completion text is never recorded unless explicitly opted in —
	// incident data is customer data (REG-12). This only ever adds a span
	// event, never an attribute, so the default-off PII test above can
	// assert "zero events" as a single, simple invariant.
	if observability.GenAIPromptRecordingEnabled() {
		span.AddEvent("gen_ai.content.prompt", trace.WithAttributes(
			attribute.String("gen_ai.prompt", renderMessages(messages)),
		))
	}

	text, usage, err := c.inner.Complete(ctx, messages)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return text, usage, err
	}

	span.SetAttributes(
		semconv.GenAIUsageInputTokens(usage.PromptTokens),
		semconv.GenAIUsageOutputTokens(usage.CompletionTokens),
		attribute.Float64("regen.llm.cost_usd", estimateCostUSD(c.model, usage.PromptTokens, usage.CompletionTokens)),
	)
	if observability.GenAIPromptRecordingEnabled() {
		span.AddEvent("gen_ai.content.completion", trace.WithAttributes(
			attribute.String("gen_ai.completion", text),
		))
	}
	span.SetStatus(codes.Ok, "")

	return text, usage, nil
}

// renderMessages concatenates messages into one string ("role: content" per
// line) — the input to both promptHash and the opt-in prompt-content event,
// so the two always agree on exactly what "the prompt" means.
func renderMessages(messages []Message) string {
	var b strings.Builder
	for i, m := range messages {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(m.Role)
		b.WriteString(": ")
		b.WriteString(m.Content)
	}
	return b.String()
}

// promptHash returns the sha256 hex digest of the rendered messages, so a
// bad answer traces back to the exact prompt without the prompt text itself
// ever touching a span.
func promptHash(messages []Message) string {
	sum := sha256.Sum256([]byte(renderMessages(messages)))
	return hex.EncodeToString(sum[:])
}
