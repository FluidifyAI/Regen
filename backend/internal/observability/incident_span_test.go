package observability

import (
	"context"
	"testing"

	"github.com/google/uuid"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestSetIncidentID_TagsCurrentSpan(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	defer tp.Shutdown(context.Background())

	ctx, span := tp.Tracer("test").Start(context.Background(), "op")
	id := uuid.New()
	SetIncidentID(ctx, id)
	span.End()

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	got, ok := findStringAttr(spans[0], "incident.id")
	if !ok || got != id.String() {
		t.Errorf("incident.id = %q, ok=%v, want %q", got, ok, id.String())
	}
}

func TestMarkIncidentCreated_TagsCurrentSpan(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	defer tp.Shutdown(context.Background())

	ctx, span := tp.Tracer("test").Start(context.Background(), "op")
	MarkIncidentCreated(ctx)
	span.End()

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	found := false
	for _, a := range spans[0].Attributes() {
		if string(a.Key) == "incident.created" && a.Value.AsBool() {
			found = true
		}
	}
	if !found {
		t.Error("expected incident.created=true attribute, not found")
	}
}

func findStringAttr(span sdktrace.ReadOnlySpan, key string) (string, bool) {
	for _, a := range span.Attributes() {
		if string(a.Key) == key {
			return a.Value.AsString(), true
		}
	}
	return "", false
}
