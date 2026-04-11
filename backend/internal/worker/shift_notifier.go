// Package worker contains background workers that run alongside the HTTP server.
package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/metrics"
	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/FluidifyAI/Regen/backend/internal/services"
	"github.com/google/uuid"
)

const (
	// pollInterval is how often the worker checks for upcoming shift handoffs.
	pollInterval = time.Minute

	// notifyWindow is the look-ahead window: fire notification if the handoff
	// is within the next pollInterval + a small buffer to handle clock skew.
	notifyWindow = pollInterval + 10*time.Second
)

// ShiftNotifier watches all on-call schedules and sends Slack DMs when a shift
// handoff occurs. It uses an in-memory dedup map so each (layer, boundary) pair
// is notified at most once per process lifetime.
type ShiftNotifier struct {
	repo        repository.ScheduleRepository
	evaluator   services.ScheduleEvaluator
	chatService services.ChatService
	msgBuilder  *services.SlackMessageBuilder

	// notified tracks the last shift boundary we fired for each layer.
	// Key: layer UUID, Value: shift boundary time.
	notified   map[uuid.UUID]time.Time
	notifiedMu sync.Mutex
}

// NewShiftNotifier creates a new ShiftNotifier.
// If chatService is nil the worker runs but all send operations are no-ops.
func NewShiftNotifier(
	repo repository.ScheduleRepository,
	evaluator services.ScheduleEvaluator,
	chatService services.ChatService,
) *ShiftNotifier {
	return &ShiftNotifier{
		repo:        repo,
		evaluator:   evaluator,
		chatService: chatService,
		msgBuilder:  services.NewSlackMessageBuilder(),
		notified:    make(map[uuid.UUID]time.Time),
	}
}

// Run starts the notification loop and blocks until ctx is cancelled.
// Designed to be launched as a goroutine from main.
func (n *ShiftNotifier) Run(ctx context.Context) {
	slog.Info("shift notifier started", "poll_interval", pollInterval)

	// Run once immediately on startup, then every pollInterval.
	n.tick()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("shift notifier stopped")
			return
		case <-ticker.C:
			n.tick()
		}
	}
}

// tick fetches all schedules with their layers and fires notifications for any
// layers whose shift boundary falls within the next notifyWindow.
func (n *ShiftNotifier) tick() {
	now := time.Now().UTC()

	// Clean up stale entries in the dedup map (older than 24 hours) to prevent unbounded growth
	n.cleanupDedupMap(now)

	schedules, err := n.repo.GetAll()
	if err != nil {
		slog.Error("shift notifier: failed to list schedules", "error", err)
		metrics.WorkerJobsFailedTotal.WithLabelValues("shift_notify").Inc()
		return
	}
	metrics.WorkerJobsProcessedTotal.WithLabelValues("shift_notify").Inc()

	for i := range schedules {
		schedule, err := n.repo.GetWithLayers(schedules[i].ID)
		if err != nil {
			slog.Warn("shift notifier: failed to load schedule layers",
				"schedule_id", schedules[i].ID, "error", err)
			continue
		}
		for j := range schedule.Layers {
			n.checkLayer(now, schedule, &schedule.Layers[j])
		}
	}
}

// cleanupDedupMap removes entries older than 24 hours to prevent memory leak.
// Called once per tick (every pollInterval).
func (n *ShiftNotifier) cleanupDedupMap(now time.Time) {
	n.notifiedMu.Lock()
	defer n.notifiedMu.Unlock()

	cutoff := now.Add(-24 * time.Hour)
	for layerID, boundary := range n.notified {
		if boundary.Before(cutoff) {
			delete(n.notified, layerID)
		}
	}
}

// checkLayer determines whether the given layer is at or past a shift boundary
// and sends notifications if so. Uses the same shift math as computeSlot in
// schedule_evaluator.go: boundary = rotation_start + (slotIndex+1) * shiftDur.
// Note: RotationStart is a TIMESTAMPTZ (absolute epoch), so timezone is implicit.
// The Schedule.Timezone field is for display purposes only, not shift calculation.
func (n *ShiftNotifier) checkLayer(now time.Time, schedule *models.Schedule, layer *models.ScheduleLayer) {
	if len(layer.Participants) == 0 {
		return
	}

	shiftDur := time.Duration(layer.ShiftDurationSeconds) * time.Second
	if shiftDur <= 0 {
		return
	}

	elapsed := now.Sub(layer.RotationStart)
	if elapsed < 0 {
		return // rotation hasn't started yet
	}

	slotIndex := int(elapsed / shiftDur)
	nextBoundary := layer.RotationStart.Add(time.Duration(slotIndex+1) * shiftDur)

	// Fire if we are within [nextBoundary - notifyWindow, nextBoundary)
	// i.e. the boundary is imminent but hasn't passed yet.
	untilBoundary := nextBoundary.Sub(now)
	if untilBoundary > notifyWindow || untilBoundary < 0 {
		return
	}

	// Dedup: skip if we already notified for this exact boundary.
	n.notifiedMu.Lock()
	if last, ok := n.notified[layer.ID]; ok && last.Equal(nextBoundary) {
		n.notifiedMu.Unlock()
		return
	}
	n.notified[layer.ID] = nextBoundary
	n.notifiedMu.Unlock()

	// Compute outgoing and incoming users.
	outgoing := layer.Participants[slotIndex%len(layer.Participants)].UserName
	incoming := layer.Participants[(slotIndex+1)%len(layer.Participants)].UserName

	// Compute end of the incoming shift (one full shift after nextBoundary).
	incomingShiftEnd := nextBoundary.Add(shiftDur)

	slog.Info("shift handoff detected",
		"schedule_id", schedule.ID,
		"schedule_name", schedule.Name,
		"layer_id", layer.ID,
		"layer_name", layer.Name,
		"outgoing", outgoing,
		"incoming", incoming,
		"boundary", nextBoundary)

	n.sendHandoffNotifications(schedule, layer, outgoing, incoming, incomingShiftEnd)
}

// sendHandoffNotifications sends DMs to the outgoing and incoming on-call users,
// and posts to the schedule's notification channel if one is configured.
func (n *ShiftNotifier) sendHandoffNotifications(
	schedule *models.Schedule,
	layer *models.ScheduleLayer,
	outgoing, incoming string,
	incomingShiftEnd time.Time,
) {
	if n.chatService == nil {
		slog.Warn("shift notifier: no chat service configured, skipping shift handoff notifications",
			"schedule", schedule.Name, "layer", layer.Name)
		return
	}

	// DM to outgoing user
	outMsg := n.msgBuilder.BuildShiftHandoffOutgoingMessage(schedule.Name, layer.Name, incoming)
	if err := n.chatService.SendDirectMessage(outgoing, outMsg); err != nil {
		slog.Warn("shift notifier: failed to DM outgoing user",
			"user", outgoing, "schedule", schedule.Name, "error", err)
		// Non-fatal: log and continue
	}

	// DM to incoming user
	inMsg := n.msgBuilder.BuildShiftHandoffIncomingMessage(schedule.Name, layer.Name, incomingShiftEnd)
	if err := n.chatService.SendDirectMessage(incoming, inMsg); err != nil {
		slog.Warn("shift notifier: failed to DM incoming user",
			"user", incoming, "schedule", schedule.Name, "error", err)
	}

	// Channel notification (if configured)
	// NOTE: NotificationChannel must be a Slack channel ID (e.g., "C01234567"),
	// not a channel name (e.g., "#oncall-alerts"). PostMessage requires IDs.
	if schedule.NotificationChannel != "" {
		chanMsg := n.msgBuilder.BuildShiftChannelNotification(
			schedule.Name, layer.Name, outgoing, incoming, incomingShiftEnd,
		)
		if _, err := n.chatService.PostMessage(schedule.NotificationChannel, chanMsg); err != nil {
			slog.Warn("shift notifier: failed to post to notification channel",
				"channel", schedule.NotificationChannel, "schedule", schedule.Name, "error", err)
		}
	}
}
