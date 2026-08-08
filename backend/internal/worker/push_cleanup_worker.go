package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/repository"
)

const (
	// staleTokenAge is how long a device token can go unseen before it is deleted.
	staleTokenAge = 90 * 24 * time.Hour

	// pushCleanupInterval controls how often the cleanup sweep runs.
	pushCleanupInterval = 24 * time.Hour
)

// PushCleanupWorker periodically removes device tokens that have not been used
// for 90 days. It runs once immediately on startup, then every 24 hours.
type PushCleanupWorker struct {
	repo repository.DeviceTokenRepository
}

// NewPushCleanupWorker creates a PushCleanupWorker backed by the given repo.
func NewPushCleanupWorker(repo repository.DeviceTokenRepository) *PushCleanupWorker {
	return &PushCleanupWorker{repo: repo}
}

// Run starts the cleanup loop. Blocks until ctx is cancelled.
// Designed to be launched as a goroutine from worker.StartAll.
func (w *PushCleanupWorker) Run(ctx context.Context) {
	slog.Info("push cleanup worker started", "interval", pushCleanupInterval, "stale_age", staleTokenAge)

	w.sweep()

	ticker := time.NewTicker(pushCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("push cleanup worker stopped")
			return
		case <-ticker.C:
			w.sweep()
		}
	}
}

// sweep deletes device tokens that have not been seen in 90 days.
func (w *PushCleanupWorker) sweep() {
	cutoff := time.Now().Add(-staleTokenAge)
	deleted, err := w.repo.DeleteStale(cutoff)
	if err != nil {
		slog.Error("push cleanup: failed to delete stale tokens", "err", err)
		return
	}
	if deleted > 0 {
		slog.Info("push cleanup: removed stale tokens", "count", deleted)
	}
}
