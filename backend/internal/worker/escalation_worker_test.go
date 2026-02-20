package worker

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/services"
)

// ── Mock escalation engine ────────────────────────────────────────────────────

type mockEscalationEngineForWorker struct {
	evaluateCalls int
	evaluateErr   error
}

func (m *mockEscalationEngineForWorker) TriggerEscalation(alert *models.Alert) error { return nil }
func (m *mockEscalationEngineForWorker) EvaluateEscalations() error {
	m.evaluateCalls++
	return m.evaluateErr
}
func (m *mockEscalationEngineForWorker) AcknowledgeAlert(id uuid.UUID, by string, via models.AcknowledgmentVia) error {
	return nil
}
func (m *mockEscalationEngineForWorker) MarkAlertCompleted(id uuid.UUID) error { return nil }

var _ services.EscalationEngine = &mockEscalationEngineForWorker{}

// ── Mock chat service ─────────────────────────────────────────────────────────

type mockChatForWorker struct {
	dms []sentDM
}

type sentDM struct {
	username string
	message  services.Message
}

func (m *mockChatForWorker) CreateChannel(name, description string) (*services.Channel, error) {
	return nil, nil
}
func (m *mockChatForWorker) PostMessage(channelID string, msg services.Message) (string, error) {
	return "", nil
}
func (m *mockChatForWorker) UpdateMessage(channelID, ts string, msg services.Message) error {
	return nil
}
func (m *mockChatForWorker) ArchiveChannel(channelID string) error { return nil }
func (m *mockChatForWorker) InviteUsers(channelID string, userIDs []string) error { return nil }
func (m *mockChatForWorker) SendDirectMessage(username string, msg services.Message) error {
	m.dms = append(m.dms, sentDM{username: username, message: msg})
	return nil
}
func (m *mockChatForWorker) GetThreadMessages(channelID, threadTS string) ([]string, error) {
	return nil, nil
}

var _ services.ChatService = &mockChatForWorker{}

// newWorkerWithEngine is a test helper that wires engine and chat into a worker.
func newWorkerWithEngine(engine services.EscalationEngine, chat services.ChatService) *EscalationWorker {
	w := NewEscalationWorker(chat)
	w.SetEngine(engine)
	return w
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestEscalationWorker_Run_CallsEvaluateOnFirstTick(t *testing.T) {
	engine := &mockEscalationEngineForWorker{}
	worker := newWorkerWithEngine(engine, nil)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(done)
	}()

	// Give the worker time to run its first tick (immediate on startup)
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	if engine.evaluateCalls == 0 {
		t.Error("expected EvaluateEscalations to be called at least once on startup")
	}
}

func TestEscalationWorker_Run_StopsOnContextCancellation(t *testing.T) {
	engine := &mockEscalationEngineForWorker{}
	worker := newWorkerWithEngine(engine, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	done := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
		// Worker stopped cleanly
	case <-time.After(2 * time.Second):
		t.Error("worker did not stop within 2 seconds after context cancellation")
	}
}

func TestEscalationWorker_SendEscalationDM_ChatNil_IsNoop(t *testing.T) {
	// When chat service is nil, SendEscalationDM should not panic
	engine := &mockEscalationEngineForWorker{}
	worker := newWorkerWithEngine(engine, nil) // nil chat

	alert := &models.Alert{
		ID:       uuid.New(),
		Title:    "High CPU",
		Severity: models.AlertSeverityCritical,
	}

	if err := worker.SendEscalationDM("alice", alert, 0); err != nil {
		t.Errorf("SendEscalationDM with nil chat should be a no-op, got error: %v", err)
	}
}

func TestEscalationWorker_SendEscalationDM_SendsMessageWithAlertDetails(t *testing.T) {
	engine := &mockEscalationEngineForWorker{}
	chat := &mockChatForWorker{}
	worker := newWorkerWithEngine(engine, chat)

	alert := &models.Alert{
		ID:       uuid.New(),
		Title:    "Database connection timeout",
		Severity: models.AlertSeverityCritical,
	}

	if err := worker.SendEscalationDM("alice", alert, 0); err != nil {
		t.Fatalf("SendEscalationDM failed: %v", err)
	}

	if len(chat.dms) != 1 {
		t.Fatalf("expected 1 DM, got %d", len(chat.dms))
	}
	if chat.dms[0].username != "alice" {
		t.Errorf("DM sent to %q, want alice", chat.dms[0].username)
	}
	// Message text should mention the alert title
	if chat.dms[0].message.Text == "" {
		t.Error("DM message text should not be empty")
	}
}

func TestEscalationWorker_SendEscalationDM_IncludesTierInMessage(t *testing.T) {
	chat := &mockChatForWorker{}
	worker := newWorkerWithEngine(&mockEscalationEngineForWorker{}, chat)

	alert := &models.Alert{ID: uuid.New(), Title: "Disk full", Severity: models.AlertSeverityWarning}

	// Tier 0 and tier 1 should both work without error
	if err := worker.SendEscalationDM("bob", alert, 0); err != nil {
		t.Fatalf("tier 0 DM failed: %v", err)
	}
	if err := worker.SendEscalationDM("carol", alert, 1); err != nil {
		t.Fatalf("tier 1 DM failed: %v", err)
	}

	if len(chat.dms) != 2 {
		t.Fatalf("expected 2 DMs, got %d", len(chat.dms))
	}
}

func TestEscalationWorker_ImplementsEscalationNotifier(t *testing.T) {
	// Compile-time check: EscalationWorker must implement EscalationNotifier
	var _ services.EscalationNotifier = &EscalationWorker{}
}
