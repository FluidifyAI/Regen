package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/services"
)

// HolidayWorker refreshes public holiday data daily for all schedules that have
// holiday country codes configured.
type HolidayWorker struct {
	svc *services.HolidayService
}

// NewHolidayWorker creates a new HolidayWorker.
func NewHolidayWorker(svc *services.HolidayService) *HolidayWorker {
	return &HolidayWorker{svc: svc}
}

// Run starts the daily holiday sync loop. It runs an initial sync on startup,
// then repeats every 24 hours at approximately midnight UTC.
func (w *HolidayWorker) Run(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("holiday worker panicked", "panic", r)
		}
	}()

	slog.Info("holiday worker started")
	w.svc.SyncAll()

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.svc.SyncAll()
		}
	}
}
