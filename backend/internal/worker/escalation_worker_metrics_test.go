package worker

// Tests for REG-13: RED metrics on the "notification send" critical path
// (SendEscalationDM's chat + push fan-out) and the worker-job duration
// histogram (tick()). Kept in a separate file from
// escalation_worker_test.go's behavioral tests since these assert on
// package-global Prometheus counters/histograms via before/after deltas
// rather than on worker/mock behavior directly.

import (
	"errors"
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/metrics"
	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
)

func sampleCount(t *testing.T, obs prometheus.Observer) uint64 {
	t.Helper()
	h, ok := obs.(interface{ Write(*dto.Metric) error })
	if !ok {
		t.Fatal("Observer does not support Write — not a *histogram")
	}
	var m dto.Metric
	if err := h.Write(&m); err != nil {
		t.Fatalf("Write: %v", err)
	}
	return m.GetHistogram().GetSampleCount()
}

func TestSendEscalationDM_ChatSuccess_RecordsNotificationSentMetrics(t *testing.T) {
	chat := &mockChatForWorker{}
	worker := newWorkerWithEngine(&mockEscalationEngineForWorker{}, chat)

	before := testutil.ToFloat64(metrics.NotificationsSentTotal.WithLabelValues("chat", "success"))
	durBefore := sampleCount(t, metrics.NotificationSendDurationSeconds.WithLabelValues("chat"))

	alert := &models.Alert{ID: uuid.New(), Title: "DB down", Severity: models.AlertSeverityCritical}
	if err := worker.SendEscalationDM("alice", alert, 0); err != nil {
		t.Fatalf("SendEscalationDM: %v", err)
	}

	after := testutil.ToFloat64(metrics.NotificationsSentTotal.WithLabelValues("chat", "success"))
	if after != before+1 {
		t.Errorf("regen_notifications_sent_total{channel=chat,status=success} = %v, want %v", after, before+1)
	}
	durAfter := sampleCount(t, metrics.NotificationSendDurationSeconds.WithLabelValues("chat"))
	if durAfter != durBefore+1 {
		t.Errorf("regen_notification_send_duration_seconds{channel=chat} sample count = %d, want %d", durAfter, durBefore+1)
	}
}

func TestSendEscalationDM_ChatError_RecordsErrorStatus(t *testing.T) {
	sendErr := errors.New("slack: rate limited")
	chat := &mockChatForWorker{sendErr: sendErr}
	worker := newWorkerWithEngine(&mockEscalationEngineForWorker{}, chat)

	before := testutil.ToFloat64(metrics.NotificationsSentTotal.WithLabelValues("chat", "error"))

	alert := &models.Alert{ID: uuid.New(), Title: "DB down", Severity: models.AlertSeverityCritical}
	if err := worker.SendEscalationDM("alice", alert, 0); err == nil {
		t.Fatal("expected SendEscalationDM to propagate the chat error")
	}

	after := testutil.ToFloat64(metrics.NotificationsSentTotal.WithLabelValues("chat", "error"))
	if after != before+1 {
		t.Errorf("regen_notifications_sent_total{channel=chat,status=error} = %v, want %v", after, before+1)
	}
}

func TestSendEscalationDM_ChatNil_RecordsSkippedStatus(t *testing.T) {
	worker := NewEscalationWorker(nil) // no chat service configured
	worker.SetEngine(&mockEscalationEngineForWorker{})

	before := testutil.ToFloat64(metrics.NotificationsSentTotal.WithLabelValues("chat", "skipped"))

	alert := &models.Alert{ID: uuid.New(), Title: "DB down", Severity: models.AlertSeverityCritical}
	if err := worker.SendEscalationDM("alice", alert, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	after := testutil.ToFloat64(metrics.NotificationsSentTotal.WithLabelValues("chat", "skipped"))
	if after != before+1 {
		t.Errorf("regen_notifications_sent_total{channel=chat,status=skipped} = %v, want %v", after, before+1)
	}
}

func TestSendEscalationDM_PushSuccess_RecordsNotificationSentMetrics(t *testing.T) {
	chat := &mockChatForWorker{}
	worker := newWorkerWithEngine(&mockEscalationEngineForWorker{}, chat)

	userID := uuid.New()
	push := &mockPushService{}
	userRepo := &fakeUserRepo{user: &models.User{ID: userID, Email: "alice@example.com"}}
	worker.SetPushService(push, userRepo)

	before := testutil.ToFloat64(metrics.NotificationsSentTotal.WithLabelValues("push", "success"))

	alert := &models.Alert{ID: uuid.New(), Title: "High CPU", Severity: models.AlertSeverityCritical}
	if err := worker.SendEscalationDM("alice@example.com", alert, 0); err != nil {
		t.Fatalf("SendEscalationDM: %v", err)
	}

	after := testutil.ToFloat64(metrics.NotificationsSentTotal.WithLabelValues("push", "success"))
	if after != before+1 {
		t.Errorf("regen_notifications_sent_total{channel=push,status=success} = %v, want %v", after, before+1)
	}
}

func TestEscalationWorkerTick_RecordsWorkerJobDuration(t *testing.T) {
	engine := &mockEscalationEngineForWorker{}
	worker := newWorkerWithEngine(engine, &mockChatForWorker{})

	before := sampleCount(t, metrics.WorkerJobDurationSeconds.WithLabelValues("escalation_evaluate"))

	worker.tick()

	after := sampleCount(t, metrics.WorkerJobDurationSeconds.WithLabelValues("escalation_evaluate"))
	if after != before+1 {
		t.Errorf("regen_worker_job_duration_seconds{job_type=escalation_evaluate} sample count = %d, want %d", after, before+1)
	}
}
