package worker

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/metrics"
	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/observability"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/FluidifyAI/Regen/backend/internal/services"
)

const (
	// escalationPollInterval is how often the worker evaluates escalation states.
	// Tier timeouts are typically 5-15 minutes, so 30s latency is negligible.
	escalationPollInterval = 30 * time.Second
)

// EscalationWorker polls active escalation states every 30 seconds and:
//   - Sends Slack DMs to on-call users at the appropriate tier.
//   - Advances to the next tier when the current tier's timeout expires.
//   - Marks escalations completed when the last tier is exhausted.
//
// It also implements services.EscalationNotifier so it can be passed directly
// into NewEscalationEngine as the notification sink.
type EscalationWorker struct {
	engine      services.EscalationEngine
	chatService services.ChatService // nil → DM sends are graceful no-ops
	msgBuilder  *services.SlackMessageBuilder
	pushSvc     services.PushNotifier     // nil → push disabled
	userRepo    repository.UserRepository // nil → push disabled (can't resolve UUID)
}

// NewEscalationWorker creates a new EscalationWorker.
// chatService may be nil; in that case SendEscalationDM is a no-op.
// Call SetEngine before Run to wire the escalation engine.
func NewEscalationWorker(chatService services.ChatService) *EscalationWorker {
	return &EscalationWorker{
		chatService: chatService,
		msgBuilder:  services.NewSlackMessageBuilder(),
	}
}

// SetEngine wires the escalation engine into the worker.
// Must be called before Run. This two-step construction breaks the circular
// dependency: EscalationEngine needs an EscalationNotifier (the worker), and
// the worker needs an EscalationEngine.
func (w *EscalationWorker) SetEngine(engine services.EscalationEngine) {
	w.engine = engine
}

// SetPushService wires an optional push notification service into the worker.
// When set, SendEscalationDM will also send a push notification to the paged user.
func (w *EscalationWorker) SetPushService(pushSvc services.PushNotifier, userRepo repository.UserRepository) {
	w.pushSvc = pushSvc
	w.userRepo = userRepo
}

// Run starts the evaluation loop and blocks until ctx is cancelled.
// Designed to be launched as a goroutine from worker.StartAll.
func (w *EscalationWorker) Run(ctx context.Context) {
	slog.Info("escalation worker started", "poll_interval", escalationPollInterval)

	// Evaluate immediately on startup, then every 30 s.
	w.tick()

	ticker := time.NewTicker(escalationPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("escalation worker stopped")
			return
		case <-ticker.C:
			w.tick()
		}
	}
}

// tick calls EvaluateEscalations once and logs any error.
//
// Opens its own root span per REG-10: this worker has no originating HTTP
// request, so each tick starts a fresh trace rather than inheriting Run's
// long-lived lifecycle context.
func (w *EscalationWorker) tick() {
	ctx, span := observability.StartWorkerTick(observability.Tracer(), "escalation_worker.tick")
	start := time.Now()
	var err error
	defer func() {
		observability.EndWorkerTick(span, err)
		observability.ObserveWithTraceExemplar(ctx,
			metrics.WorkerJobDurationSeconds.WithLabelValues("escalation_evaluate"),
			time.Since(start).Seconds())
	}()

	if err = w.engine.EvaluateEscalations(); err != nil {
		slog.Error("escalation worker: EvaluateEscalations failed", "err", err)
		metrics.WorkerJobsFailedTotal.WithLabelValues("escalation_evaluate").Inc()
		return
	}
	metrics.WorkerJobsProcessedTotal.WithLabelValues("escalation_evaluate").Inc()
}

// SendEscalationDM implements services.EscalationNotifier.
// Sends a Slack DM to userID with alert details and an Acknowledge button.
// If chatService is nil, the call is a no-op (Slack not configured).
// alert may be nil for incident-sourced escalations.
func (w *EscalationWorker) SendEscalationDM(userID string, alert *models.Alert, tierIndex int) error {
	var alertIDStr string
	if alert != nil {
		alertIDStr = alert.ID.String()
	}
	if w.chatService == nil {
		slog.Warn("escalation worker: no chat service; skipping DM",
			"user_id", userID, "alert_id", alertIDStr, "tier", tierIndex)
		metrics.NotificationsSentTotal.WithLabelValues("chat", "skipped").Inc()
		return nil
	}

	chatStart := time.Now()
	msg := w.msgBuilder.BuildEscalationDMMessage(alert, tierIndex)
	err := w.chatService.SendDirectMessage(userID, msg)
	metrics.NotificationSendDurationSeconds.WithLabelValues("chat").Observe(time.Since(chatStart).Seconds())
	if err != nil {
		slog.Error("escalation worker: failed to send DM",
			"user_id", userID, "alert_id", alertIDStr, "tier", tierIndex, "err", err)
		metrics.NotificationsSentTotal.WithLabelValues("chat", "error").Inc()
		return err
	}
	metrics.NotificationsSentTotal.WithLabelValues("chat", "success").Inc()

	slog.Info("escalation DM sent",
		"user_id", userID, "alert_id", alertIDStr, "tier", tierIndex)

	// Push notification — resolve the on-call identifier to a UUID via user repo
	if w.pushSvc != nil && w.pushSvc.IsEnabled() && w.userRepo != nil {
		userRecord, lookupErr := w.userRepo.GetByEmail(userID)
		if lookupErr != nil {
			slog.Debug("push: could not resolve escalation target by email, skipping push",
				"err", lookupErr)
		} else {
			var alertTitle string
			if alert != nil {
				alertTitle = alert.Title
			}
			n := services.PushNotification{
				Title: fmt.Sprintf("You are being paged (tier %d)", tierIndex+1),
				Body:  alertTitle,
				Data: map[string]string{
					"event":      "escalation_paged",
					"tier_index": strconv.Itoa(tierIndex),
				},
			}
			pushStart := time.Now()
			pushErr := w.pushSvc.SendToUser(context.Background(), userRecord.ID, n)
			metrics.NotificationSendDurationSeconds.WithLabelValues("push").Observe(time.Since(pushStart).Seconds())
			if pushErr != nil {
				slog.Warn("push: escalation push failed", "user_id", userRecord.ID, "err", pushErr)
				metrics.NotificationsSentTotal.WithLabelValues("push", "error").Inc()
			} else {
				metrics.NotificationsSentTotal.WithLabelValues("push", "success").Inc()
			}
		}
	}

	return nil
}
