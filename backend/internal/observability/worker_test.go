package observability

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// newTestTracer builds an isolated tracer backed by a span recorder, so tests
// never touch the process-global TracerProvider (see REG-7: it's a shared,
// order-dependent singleton, unreliable to save/restore per-test).
func newTestTracer(t *testing.T) (trace.Tracer, *tracetest.SpanRecorder) {
	t.Helper()
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return tp.Tracer("test"), sr
}

func TestStartWorkerTick_ProducesAFreshRootSpanEachCall(t *testing.T) {
	tracer, sr := newTestTracer(t)

	_, span1 := StartWorkerTick(tracer, "escalation_worker.tick")
	span1.End()
	_, span2 := StartWorkerTick(tracer, "escalation_worker.tick")
	span2.End()

	spans := sr.Ended()
	if len(spans) != 2 {
		t.Fatalf("expected 2 independent spans, got %d", len(spans))
	}
	if spans[0].SpanContext().TraceID() == spans[1].SpanContext().TraceID() {
		t.Error("expected each tick to start its own trace (fresh root span), got the same trace ID for both — ticks must not share one unbounded trace for the worker's lifetime")
	}
	for _, s := range spans {
		if s.Name() != "escalation_worker.tick" {
			t.Errorf("span name = %q, want %q", s.Name(), "escalation_worker.tick")
		}
	}
}

func TestEndWorkerTick_RecordsErrorStatusWhenErrGiven(t *testing.T) {
	tracer, sr := newTestTracer(t)

	_, span := StartWorkerTick(tracer, "push_cleanup_worker.tick")
	EndWorkerTick(span, errors.New("boom"))

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Status().Code != codes.Error {
		t.Errorf("expected span status Error, got %v", spans[0].Status().Code)
	}
	if len(spans[0].Events()) == 0 {
		t.Fatal("expected the error to be recorded as a span event")
	}
}

func TestEndWorkerTick_NoErrorStatusWhenErrNil(t *testing.T) {
	tracer, sr := newTestTracer(t)

	_, span := StartWorkerTick(tracer, "holiday_worker.tick")
	EndWorkerTick(span, nil)

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Status().Code == codes.Error {
		t.Error("expected no Error status when err is nil")
	}
}
