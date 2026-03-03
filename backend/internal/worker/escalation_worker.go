package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/services"
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
	chatService services.ChatService        // nil → DM sends are graceful no-ops
	msgBuilder  *services.SlackMessageBuilder
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
func (w *EscalationWorker) tick() {
	if err := w.engine.EvaluateEscalations(); err != nil {
		slog.Error("escalation worker: EvaluateEscalations failed", "err", err)
	}
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
		return nil
	}

	msg := w.msgBuilder.BuildEscalationDMMessage(alert, tierIndex)
	if err := w.chatService.SendDirectMessage(userID, msg); err != nil {
		slog.Error("escalation worker: failed to send DM",
			"user_id", userID, "alert_id", alertIDStr, "tier", tierIndex, "err", err)
		return err
	}

	slog.Info("escalation DM sent",
		"user_id", userID, "alert_id", alertIDStr, "tier", tierIndex)
	return nil
}
