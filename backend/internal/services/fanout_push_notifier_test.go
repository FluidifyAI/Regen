package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/services"
	"github.com/google/uuid"
)

// mockPushNotifier is a simple spy implementing PushNotifier.
type mockPushNotifier struct {
	enabled   bool
	callCount int
	returnErr error
}

func (m *mockPushNotifier) IsEnabled() bool { return m.enabled }
func (m *mockPushNotifier) SendToUser(_ context.Context, _ uuid.UUID, _ services.PushNotification) error {
	m.callCount++
	return m.returnErr
}

func TestNewFanoutPushNotifier_AllNil_ReturnsNil(t *testing.T) {
	got := services.NewFanoutPushNotifier(nil, nil)
	if got != nil {
		t.Error("expected nil when all notifiers are nil")
	}
}

func TestNewFanoutPushNotifier_AllDisabled_ReturnsNil(t *testing.T) {
	a := &mockPushNotifier{enabled: false}
	b := &mockPushNotifier{enabled: false}
	got := services.NewFanoutPushNotifier(a, b)
	if got != nil {
		t.Error("expected nil when all notifiers are disabled")
	}
}

func TestNewFanoutPushNotifier_OneEnabled_IsEnabled(t *testing.T) {
	a := &mockPushNotifier{enabled: false}
	b := &mockPushNotifier{enabled: true}
	got := services.NewFanoutPushNotifier(a, b)
	if got == nil {
		t.Fatal("expected non-nil fanout when one notifier is enabled")
	}
	if !got.IsEnabled() {
		t.Error("expected IsEnabled()=true")
	}
}

func TestFanoutPushNotifier_SendToUser_CallsBothNotifiers(t *testing.T) {
	a := &mockPushNotifier{enabled: true}
	b := &mockPushNotifier{enabled: true}
	fanout := services.NewFanoutPushNotifier(a, b)

	err := fanout.SendToUser(context.Background(), uuid.New(), services.PushNotification{Title: "T", Body: "B"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.callCount != 1 {
		t.Errorf("expected notifier A called once, got %d", a.callCount)
	}
	if b.callCount != 1 {
		t.Errorf("expected notifier B called once, got %d", b.callCount)
	}
}

func TestFanoutPushNotifier_SendToUser_FirstErrorDoesNotBlockSecond(t *testing.T) {
	a := &mockPushNotifier{enabled: true, returnErr: errors.New("a failed")}
	b := &mockPushNotifier{enabled: true}
	fanout := services.NewFanoutPushNotifier(a, b)

	err := fanout.SendToUser(context.Background(), uuid.New(), services.PushNotification{Title: "T", Body: "B"})
	if err != nil {
		t.Fatalf("fanout should not propagate errors, got: %v", err)
	}
	if a.callCount != 1 {
		t.Errorf("expected A called once, got %d", a.callCount)
	}
	if b.callCount != 1 {
		t.Errorf("expected B called once even after A errored, got %d", b.callCount)
	}
}

func TestFanoutPushNotifier_SendToUser_AlwaysReturnsNil(t *testing.T) {
	a := &mockPushNotifier{enabled: true, returnErr: errors.New("fail")}
	b := &mockPushNotifier{enabled: true, returnErr: errors.New("also fail")}
	fanout := services.NewFanoutPushNotifier(a, b)

	if err := fanout.SendToUser(context.Background(), uuid.New(), services.PushNotification{}); err != nil {
		t.Errorf("fanout must always return nil, got: %v", err)
	}
}

func TestNewFanoutPushNotifier_NilAndEnabled_FiltersNil(t *testing.T) {
	b := &mockPushNotifier{enabled: true}
	fanout := services.NewFanoutPushNotifier(nil, b)
	if fanout == nil {
		t.Fatal("expected non-nil fanout")
	}
	fanout.SendToUser(context.Background(), uuid.New(), services.PushNotification{})
	if b.callCount != 1 {
		t.Errorf("enabled notifier should be called, got %d", b.callCount)
	}
}
