package services

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/observability"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// withTestAsyncSpanTracer swaps the package-level asyncSpanTracer for the
// duration of the test, backed by an isolated span recorder. A plain
// package-level var (not otel's global TracerProvider) is reliably
// save/restorable — unlike the process-global TracerProvider, which REG-7
// found to be a shared, order-dependent singleton unsafe to save/restore
// per-test.
func withTestAsyncSpanTracer(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	prev := asyncSpanTracer
	asyncSpanTracer = tp.Tracer("test")
	t.Cleanup(func() { asyncSpanTracer = prev })

	return sr
}

func TestRecoverAsyncPanic_LogsViaErrorContextWhenPanicRecovered(t *testing.T) {
	withTestAsyncSpanTracer(t)

	var buf bytes.Buffer
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(observability.NewContextHandler(slog.NewJSONHandler(&buf, nil))))
	defer slog.SetDefault(prevLogger)

	func() {
		defer recoverAsyncPanic(context.Background(), "testOp", "incident_id", "abc-123")
		panic("boom")
	}()

	out := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("testOp")) {
		t.Errorf("expected the log line to name the op, got: %q", out)
	}
	if !bytes.Contains(buf.Bytes(), []byte("boom")) {
		t.Errorf("expected the log line to include the panic value, got: %q", out)
	}
	if !bytes.Contains(buf.Bytes(), []byte("abc-123")) {
		t.Errorf("expected the log line to include the extra fields, got: %q", out)
	}
}

func TestRecoverAsyncPanic_DoesNotReturnAPanicToTheCaller(t *testing.T) {
	withTestAsyncSpanTracer(t)

	// The whole point of this helper: a panic inside the deferred block must
	// not propagate and crash the goroutine/test process.
	func() {
		defer recoverAsyncPanic(context.Background(), "testOp")
		panic("should be recovered")
	}()
}

func TestRecoverAsyncPanic_NoOpWhenNoPanicOccurs(t *testing.T) {
	sr := withTestAsyncSpanTracer(t)

	var buf bytes.Buffer
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(prevLogger)

	func() {
		defer recoverAsyncPanic(context.Background(), "testOp")
		// no panic
	}()

	if buf.Len() != 0 {
		t.Errorf("expected no log output when nothing panicked, got: %q", buf.String())
	}
	if len(sr.Ended()) != 0 {
		t.Errorf("expected no span when nothing panicked, got %d", len(sr.Ended()))
	}
}

func TestRecoverAsyncPanic_RecordsErrorOnASpanLinkedToTheOriginatingSpan(t *testing.T) {
	sr := withTestAsyncSpanTracer(t)

	// A real originating span, standing in for the eventual request-scoped
	// span REG-157 will thread through — proves the link, once fed a real
	// context, actually connects to the right trace.
	originTP := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	defer originTP.Shutdown(context.Background())
	originCtx, originSpan := originTP.Tracer("test").Start(context.Background(), "originating-request")
	originSpan.End()

	func() {
		defer recoverAsyncPanic(originCtx, "sendTelegramIncidentCreated", "incident_id", "abc-123")
		panic("telegram send failed")
	}()

	spans := sr.Ended()
	var panicSpan sdktrace.ReadOnlySpan
	for _, s := range spans {
		if s.Name() == "sendTelegramIncidentCreated" {
			panicSpan = s
		}
	}
	if panicSpan == nil {
		t.Fatalf("expected a span named %q, got: %v", "sendTelegramIncidentCreated", spanNamesFor(spans))
	}

	links := panicSpan.Links()
	if len(links) != 1 {
		t.Fatalf("expected 1 link to the originating span, got %d", len(links))
	}
	if links[0].SpanContext.TraceID() != originSpan.SpanContext().TraceID() {
		t.Error("the panic span's link points at a different trace than the originating span")
	}

	// Linked, not parented: the panic span must NOT be a child of the
	// originating span (which may well have already ended by the time an
	// async goroutine panics) — it's causally related, not nested.
	if panicSpan.Parent().IsValid() {
		t.Error("expected the panic span to have no parent (linked, not nested) — the async goroutine may outlive the originating span")
	}
}

func TestRecoverAsyncPanic_HandlesBackgroundContextGracefully(t *testing.T) {
	withTestAsyncSpanTracer(t)

	// This is the real, current state of every one of the 16 call sites in
	// incident_service.go today: no real request context available yet
	// (see REG-157). Must not panic or error just because the link source
	// has no valid span.
	func() {
		defer recoverAsyncPanic(context.Background(), "testOp")
		panic("boom")
	}()
}

func spanNamesFor(spans []sdktrace.ReadOnlySpan) []string {
	names := make([]string, len(spans))
	for i, s := range spans {
		names[i] = s.Name()
	}
	return names
}
