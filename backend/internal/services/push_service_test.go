package services

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"firebase.google.com/go/v4/messaging"
	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/google/uuid"
)

// ─── mockFCMSender ────────────────────────────────────────────────────────────

type mockFCMSender struct {
	mu      sync.Mutex
	calls   int
	sendErr error // returned by every Send call; nil = success
}

func (m *mockFCMSender) Send(_ context.Context, _ *messaging.Message) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return "msg-id", m.sendErr
}

// ─── fakeDeviceTokenRepo ──────────────────────────────────────────────────────

type fakeDeviceTokenRepo struct {
	mu sync.Mutex

	tokens           []models.DeviceToken
	deletedByToken   []string
	updatedLastSeen  []string
	deleteByTokenErr error
	updateLastSeenFn func(token string) error
}

func (f *fakeDeviceTokenRepo) Upsert(_ uuid.UUID, _, _, _ string) error { return nil }

func (f *fakeDeviceTokenRepo) GetByUserID(_ uuid.UUID) ([]models.DeviceToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]models.DeviceToken{}, f.tokens...), nil
}

func (f *fakeDeviceTokenRepo) DeleteByUserAndToken(_ uuid.UUID, _ string) (bool, error) {
	return true, nil
}

func (f *fakeDeviceTokenRepo) DeleteByToken(token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletedByToken = append(f.deletedByToken, token)
	return f.deleteByTokenErr
}

func (f *fakeDeviceTokenRepo) UpdateLastSeen(token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updatedLastSeen = append(f.updatedLastSeen, token)
	if f.updateLastSeenFn != nil {
		return f.updateLastSeenFn(token)
	}
	return nil
}

func (f *fakeDeviceTokenRepo) DeleteStale(_ time.Time) (int64, error) { return 0, nil }

func (f *fakeDeviceTokenRepo) CountByUserID(_ uuid.UUID) (int64, error) { return 0, nil }

// Compile-time check that fakeDeviceTokenRepo satisfies the interface.
var _ repository.DeviceTokenRepository = (*fakeDeviceTokenRepo)(nil)

// ─── Helper: build a PushService with injected dependencies ───────────────────

// newTestPushService builds a PushService with injected sender, repo, and
// a custom checkNotRegistered function (so tests don't depend on Firebase internals).
func newTestPushService(sender FCMSender, repo repository.DeviceTokenRepository, checkFn func(error) bool) *PushService {
	return &PushService{
		sender:              sender,
		repo:                repo,
		checkNotRegistered:  checkFn,
	}
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestPushService_IsEnabled_NilReturnsFalse(t *testing.T) {
	var svc *PushService
	if svc.IsEnabled() {
		t.Fatal("nil *PushService.IsEnabled() should return false")
	}
}

func TestNewPushService_EmptyCredPath_ReturnsNilNil(t *testing.T) {
	svc, err := NewPushService("", &fakeDeviceTokenRepo{})
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if svc != nil {
		t.Fatalf("expected nil service for empty cred path, got non-nil")
	}
}

func TestNewPushService_InvalidCredFile_ReturnsError(t *testing.T) {
	f, err := os.CreateTemp("", "fake-creds-*.json")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	// Write invalid JSON
	if _, err := f.WriteString("{ this is not valid json !!! }"); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	f.Close()

	_, initErr := NewPushService(f.Name(), &fakeDeviceTokenRepo{})
	if initErr == nil {
		t.Fatal("expected error for invalid credential JSON, got nil")
	}
}

func TestPushService_SendToUser_NoTokens_NoFCMCall(t *testing.T) {
	sender := &mockFCMSender{}
	repo := &fakeDeviceTokenRepo{tokens: nil}
	svc := newTestPushService(sender, repo, nil)

	err := svc.SendToUser(context.Background(), uuid.New(), PushNotification{
		Title: "test",
		Body:  "body",
	})
	if err != nil {
		t.Fatalf("SendToUser: unexpected error: %v", err)
	}
	if sender.calls != 0 {
		t.Fatalf("FCM Send called %d times, expected 0", sender.calls)
	}
}

func TestPushService_SendToUser_TwoValidTokens_SendsAndBumpsLastSeen(t *testing.T) {
	sender := &mockFCMSender{sendErr: nil}
	repo := &fakeDeviceTokenRepo{
		tokens: []models.DeviceToken{
			{Token: "tok-1", Platform: "ios"},
			{Token: "tok-2", Platform: "android"},
		},
	}
	svc := newTestPushService(sender, repo, nil)

	err := svc.SendToUser(context.Background(), uuid.New(), PushNotification{Title: "INC-1", Body: "test"})
	if err != nil {
		t.Fatalf("SendToUser: unexpected error: %v", err)
	}

	sender.mu.Lock()
	calls := sender.calls
	sender.mu.Unlock()

	if calls != 2 {
		t.Fatalf("FCM Send called %d times, expected 2", calls)
	}

	repo.mu.Lock()
	bumpCount := len(repo.updatedLastSeen)
	repo.mu.Unlock()

	if bumpCount != 2 {
		t.Fatalf("UpdateLastSeen called %d times, expected 2", bumpCount)
	}
}

func TestPushService_SendToUser_StaleToken_DeletedAndNilReturned(t *testing.T) {
	staleErr := errors.New("stale-token-sentinel")
	sender := &mockFCMSender{sendErr: staleErr}
	repo := &fakeDeviceTokenRepo{
		tokens: []models.DeviceToken{
			{Token: "tok-stale", Platform: "ios"},
		},
	}
	// checkNotRegistered returns true for our sentinel error
	svc := newTestPushService(sender, repo, func(err error) bool {
		return errors.Is(err, staleErr)
	})

	err := svc.SendToUser(context.Background(), uuid.New(), PushNotification{Title: "test"})
	if err != nil {
		t.Fatalf("SendToUser: expected nil, got: %v", err)
	}

	repo.mu.Lock()
	deleted := repo.deletedByToken
	repo.mu.Unlock()

	if len(deleted) != 1 || deleted[0] != "tok-stale" {
		t.Fatalf("DeleteByToken not called correctly; deleted=%v", deleted)
	}
}

func TestPushService_SendToUser_TransientError_LoggedAndContinues(t *testing.T) {
	transientErr := errors.New("transient-network-error")
	callIdx := 0
	sender := &mockFCMSender{}
	// First send fails, second succeeds
	sender.sendErr = nil // We'll use a custom sender below

	type callCountSender struct {
		mu   sync.Mutex
		n    int
		errs []error
	}
	ccs := &callCountSender{errs: []error{transientErr, nil}}
	customSender := &funcFCMSender{
		fn: func(_ context.Context, _ *messaging.Message) (string, error) {
			ccs.mu.Lock()
			defer ccs.mu.Unlock()
			idx := callIdx
			callIdx++
			if idx < len(ccs.errs) {
				return "", ccs.errs[idx]
			}
			return "id", nil
		},
	}

	repo := &fakeDeviceTokenRepo{
		tokens: []models.DeviceToken{
			{Token: "tok-fail", Platform: "ios"},
			{Token: "tok-ok", Platform: "android"},
		},
	}
	// checkNotRegistered always returns false (it's a transient error, not stale)
	svc := newTestPushService(customSender, repo, func(err error) bool { return false })

	err := svc.SendToUser(context.Background(), uuid.New(), PushNotification{Title: "test"})
	if err != nil {
		t.Fatalf("SendToUser: expected nil, got: %v", err)
	}

	// No DeleteByToken should have been called
	repo.mu.Lock()
	deletedCount := len(repo.deletedByToken)
	repo.mu.Unlock()

	if deletedCount != 0 {
		t.Fatalf("DeleteByToken called %d times, expected 0 for transient error", deletedCount)
	}

	// Second token should have triggered UpdateLastSeen (success)
	repo.mu.Lock()
	bumpCount := len(repo.updatedLastSeen)
	repo.mu.Unlock()

	if bumpCount != 1 {
		t.Fatalf("UpdateLastSeen called %d times, expected 1 (only for the succeeding token)", bumpCount)
	}
}

// funcFCMSender is a FCMSender backed by a function (used in transient error test).
type funcFCMSender struct {
	fn func(ctx context.Context, msg *messaging.Message) (string, error)
}

func (f *funcFCMSender) Send(ctx context.Context, msg *messaging.Message) (string, error) {
	return f.fn(ctx, msg)
}
