package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace/noop"
)

// newTestHandler builds a ContextHandler writing JSON to buf, so assertions
// can parse real log output rather than inspect internals.
func newTestHandler(buf *bytes.Buffer) *ContextHandler {
	return NewContextHandler(slog.NewJSONHandler(buf, nil))
}

func TestContextHandler_InjectsTraceAndSpanIDWhenRecording(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	defer tp.Shutdown(context.Background())

	ctx, span := tp.Tracer("test").Start(context.Background(), "op")
	defer span.End()
	sc := span.SpanContext()

	var buf bytes.Buffer
	logger := slog.New(newTestHandler(&buf))
	logger.InfoContext(ctx, "hello")

	var entry map[string]string
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log output as JSON: %v\noutput: %s", err, buf.String())
	}

	if got := entry["trace_id"]; got != sc.TraceID().String() {
		t.Errorf("trace_id = %q, want %q", got, sc.TraceID().String())
	}
	if got := entry["span_id"]; got != sc.SpanID().String() {
		t.Errorf("span_id = %q, want %q", got, sc.SpanID().String())
	}
}

func TestContextHandler_NoOpWhenNoSpanInContext(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(newTestHandler(&buf))
	logger.InfoContext(context.Background(), "hello")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log output as JSON: %v", err)
	}
	if _, ok := entry["trace_id"]; ok {
		t.Errorf("expected no trace_id attribute with no span in context, got: %s", buf.String())
	}
	if _, ok := entry["span_id"]; ok {
		t.Errorf("expected no span_id attribute with no span in context, got: %s", buf.String())
	}
}

func TestContextHandler_NoOpWhenTracingDisabled(t *testing.T) {
	// Mirrors InitTracer's disabled-by-default path: a no-op TracerProvider.
	// Its spans are never recording, and this must be a true passthrough —
	// zero behavior change for self-hosters, per the acceptance criteria.
	ctx, span := noop.NewTracerProvider().Tracer("test").Start(context.Background(), "op")
	defer span.End()

	if span.IsRecording() {
		t.Fatal("test setup bug: expected a no-op span to never be recording")
	}

	var buf bytes.Buffer
	logger := slog.New(newTestHandler(&buf))
	logger.InfoContext(ctx, "hello")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log output as JSON: %v", err)
	}
	if _, ok := entry["trace_id"]; ok {
		t.Errorf("expected no trace_id with tracing disabled (no-op span), got: %s", buf.String())
	}
}

func TestContextHandler_WithAttrsStillInjectsTraceContext(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	defer tp.Shutdown(context.Background())

	ctx, span := tp.Tracer("test").Start(context.Background(), "op")
	defer span.End()

	var buf bytes.Buffer
	// .With(...) calls WithAttrs on the handler — must not silently drop the
	// ContextHandler wrapper, or trace injection breaks for every logger built
	// via .With(), which is the common pattern (component-scoped loggers).
	logger := slog.New(newTestHandler(&buf)).With("component", "test")
	logger.InfoContext(ctx, "hello")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log output as JSON: %v", err)
	}
	if _, ok := entry["trace_id"]; !ok {
		t.Errorf("expected trace_id to survive .With(), got: %s", buf.String())
	}
	if entry["component"] != "test" {
		t.Errorf("expected the .With() attribute to still be present, got: %s", buf.String())
	}
}

func TestContextHandler_WithGroupStillInjectsTraceContext(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	defer tp.Shutdown(context.Background())

	ctx, span := tp.Tracer("test").Start(context.Background(), "op")
	defer span.End()

	var buf bytes.Buffer
	logger := slog.New(newTestHandler(&buf)).WithGroup("req")
	logger.InfoContext(ctx, "hello")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log output as JSON: %v", err)
	}
	// trace_id/span_id are added via r.AddAttrs before delegating, so they
	// land inside the active group too — same as any other attribute would.
	req, ok := entry["req"].(map[string]any)
	if !ok {
		t.Fatalf("expected a \"req\" group in output, got: %s", buf.String())
	}
	if _, ok := req["trace_id"]; !ok {
		t.Errorf("expected trace_id to survive .WithGroup(), got: %s", buf.String())
	}
}

// Confirms the handler type itself satisfies slog.Handler at compile time.
var _ slog.Handler = (*ContextHandler)(nil)
