package worker

// Tests for REG-15: notification.paged span tagging on SendEscalationDM —
// the attribute the collector's tail-sampling policy uses to keep 100% of
// traces that send a page. SendEscalationDM has no ctx parameter (it
// implements services.EscalationNotifier, called deep inside
// EscalationEngine, which has no ctx on its public interface — see REG-13's
// documented reasoning), so it starts its own root span here rather than
// threading one through, the same pattern already used by
// observability.StartWorkerTick.
//
// EscalationWorker.tracer is set directly (same package, unexported field —
// same idiom already used for pushSvc/userRepo in escalation_worker_test.go)
// to an isolated TracerProvider's tracer, rather than mutating the global
// TracerProvider: see REG-7's finding that the global is a shared,
// order-dependent singleton unsafe to touch from tests.

import (
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/google/uuid"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func lastSpanAttr(sr *tracetest.SpanRecorder, key string) (string, bool) {
	spans := sr.Ended()
	for i := len(spans) - 1; i >= 0; i-- {
		for _, a := range spans[i].Attributes() {
			if string(a.Key) == key {
				return a.Value.Emit(), true
			}
		}
	}
	return "", false
}

func TestSendEscalationDM_ChatSuccess_TagsSpanNotificationPaged(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))

	chat := &mockChatForWorker{}
	worker := newWorkerWithEngine(&mockEscalationEngineForWorker{}, chat)
	worker.tracer = tp.Tracer("test")

	alert := &models.Alert{ID: uuid.New(), Title: "DB down", Severity: models.AlertSeverityCritical}
	if err := worker.SendEscalationDM("alice", alert, 0); err != nil {
		t.Fatalf("SendEscalationDM: %v", err)
	}

	got, ok := lastSpanAttr(sr, "notification.paged")
	if !ok || got != "true" {
		t.Errorf("notification.paged = %q, ok=%v, want true", got, ok)
	}
}

func TestSendEscalationDM_ChatSkipped_TagsSpanNotificationNotPaged(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))

	worker := NewEscalationWorker(nil) // no chat configured -> skipped
	worker.SetEngine(&mockEscalationEngineForWorker{})
	worker.tracer = tp.Tracer("test")

	alert := &models.Alert{ID: uuid.New(), Title: "DB down", Severity: models.AlertSeverityCritical}
	if err := worker.SendEscalationDM("alice", alert, 0); err != nil {
		t.Fatalf("SendEscalationDM: %v", err)
	}

	got, ok := lastSpanAttr(sr, "notification.paged")
	if !ok || got != "false" {
		t.Errorf("notification.paged = %q, ok=%v, want false", got, ok)
	}
}
