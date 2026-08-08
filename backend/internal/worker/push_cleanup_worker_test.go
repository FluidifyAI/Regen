package worker

import (
	"testing"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/google/uuid"
)

// ── Mock DeviceTokenRepository ─────────────────────────────────────────────────

type mockTokenRepo struct {
	deleteStaleCount int64
	deleteStaleErr   error
	deleteStaleCutoff time.Time // captured by DeleteStale call
	deleteStaleCallCount int
}

func (m *mockTokenRepo) Upsert(_ uuid.UUID, _, _, _ string) error { return nil }
func (m *mockTokenRepo) GetByUserID(_ uuid.UUID) ([]models.DeviceToken, error) {
	return nil, nil
}
func (m *mockTokenRepo) DeleteByUserAndToken(_ uuid.UUID, _ string) (bool, error) {
	return false, nil
}
func (m *mockTokenRepo) DeleteByToken(_ string) error          { return nil }
func (m *mockTokenRepo) UpdateLastSeen(_ string) error         { return nil }
func (m *mockTokenRepo) CountByUserID(_ uuid.UUID) (int64, error) { return 0, nil }
func (m *mockTokenRepo) DeleteStale(before time.Time) (int64, error) {
	m.deleteStaleCutoff = before
	m.deleteStaleCallCount++
	return m.deleteStaleCount, m.deleteStaleErr
}

var _ repository.DeviceTokenRepository = &mockTokenRepo{}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestPushCleanupWorker_Sweep_CallsDeleteStaleWithCorrectCutoff(t *testing.T) {
	repo := &mockTokenRepo{}
	w := NewPushCleanupWorker(repo) // RED: type does not exist yet

	before := time.Now()
	w.sweep()
	after := time.Now()

	if repo.deleteStaleCallCount != 1 {
		t.Fatalf("expected DeleteStale called once, got %d", repo.deleteStaleCallCount)
	}

	// Cutoff should be ~90 days before now — allow ±1 minute tolerance
	expectedCutoff := before.Add(-90 * 24 * time.Hour)
	latestCutoff := after.Add(-90 * 24 * time.Hour)

	if repo.deleteStaleCutoff.Before(expectedCutoff.Add(-time.Minute)) {
		t.Errorf("cutoff %v is more than 1 minute before expected %v", repo.deleteStaleCutoff, expectedCutoff)
	}
	if repo.deleteStaleCutoff.After(latestCutoff.Add(time.Minute)) {
		t.Errorf("cutoff %v is more than 1 minute after expected %v", repo.deleteStaleCutoff, latestCutoff)
	}
}

func TestPushCleanupWorker_Sweep_LogsWhenTokensDeleted(t *testing.T) {
	repo := &mockTokenRepo{deleteStaleCount: 3}
	w := NewPushCleanupWorker(repo)

	// Should not panic; slog output goes to discard in test env
	w.sweep()

	if repo.deleteStaleCallCount != 1 {
		t.Fatalf("expected DeleteStale called once, got %d", repo.deleteStaleCallCount)
	}
}

func TestPushCleanupWorker_Sweep_NoLogWhenZeroDeleted(t *testing.T) {
	repo := &mockTokenRepo{deleteStaleCount: 0}
	w := NewPushCleanupWorker(repo)

	w.sweep()

	if repo.deleteStaleCallCount != 1 {
		t.Fatalf("expected DeleteStale called once, got %d", repo.deleteStaleCallCount)
	}
}
