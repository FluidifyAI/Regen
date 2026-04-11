package services

import (
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
)

// TimelineSegment represents a contiguous window during which a single user is on call.
type TimelineSegment struct {
	// Start is the beginning of this segment (inclusive).
	Start time.Time `json:"start"`
	// End is the end of this segment (exclusive).
	End time.Time `json:"end"`
	// UserName is the on-call user for this segment.
	UserName string `json:"user_name"`
	// IsOverride is true when this segment is covered by an explicit override.
	IsOverride bool `json:"is_override"`
	// LayerID is the layer this segment belongs to. nil for effective/merged timeline.
	LayerID *uuid.UUID `json:"layer_id,omitempty"`
}

// ScheduleEvaluator computes who is on call at any given time.
type ScheduleEvaluator interface {
	// WhoIsOnCall returns the username on call for the given schedule at the given time.
	// Returns ("", nil) if the schedule has no layers/participants configured.
	WhoIsOnCall(scheduleID uuid.UUID, at time.Time) (string, error)

	// GetTimeline returns the on-call schedule broken into contiguous segments
	// for the window [from, to). Defaults to next 7 days if from/to are zero.
	GetTimeline(scheduleID uuid.UUID, from, to time.Time) ([]TimelineSegment, error)

	// GetLayerTimelines returns per-layer timelines and the effective merged timeline.
	// layerTimelines is keyed by layer UUID. Each layer is computed independently
	// (no override application). effective is the fully-merged timeline (same as GetTimeline).
	GetLayerTimelines(scheduleID uuid.UUID, from, to time.Time) (layerTimelines map[uuid.UUID][]TimelineSegment, effective []TimelineSegment, err error)
}

// scheduleEvaluator implements ScheduleEvaluator.
type scheduleEvaluator struct {
	repo repository.ScheduleRepository
}

// NewScheduleEvaluator creates a new schedule evaluator.
func NewScheduleEvaluator(repo repository.ScheduleRepository) ScheduleEvaluator {
	return &scheduleEvaluator{repo: repo}
}

func (e *scheduleEvaluator) WhoIsOnCall(scheduleID uuid.UUID, at time.Time) (string, error) {
	schedule, err := e.repo.GetWithLayers(scheduleID)
	if err != nil {
		return "", fmt.Errorf("failed to load schedule: %w", err)
	}

	overrides, err := e.repo.GetActiveOverrides(scheduleID, at)
	if err != nil {
		return "", fmt.Errorf("failed to load overrides: %w", err)
	}

	// Overrides take priority. Return the first one (ordered by start_time DESC
	// from the repo, so the most recently started override wins on ties).
	if len(overrides) > 0 {
		return overrides[0].OverrideUser, nil
	}

	// Walk layers by order_index (already sorted by repo). First layer with
	// a non-empty participant list for this time wins.
	return computeOnCallFromLayers(schedule.Layers, at), nil
}

func (e *scheduleEvaluator) GetTimeline(scheduleID uuid.UUID, from, to time.Time) ([]TimelineSegment, error) {
	// Apply default window: next 7 days.
	now := time.Now().UTC()
	if from.IsZero() {
		from = now
	}
	if to.IsZero() {
		to = now.Add(7 * 24 * time.Hour)
	}
	if !to.After(from) {
		return nil, fmt.Errorf("to must be after from")
	}

	schedule, err := e.repo.GetWithLayers(scheduleID)
	if err != nil {
		return nil, fmt.Errorf("failed to load schedule: %w", err)
	}

	overrides, err := e.repo.GetOverridesInWindow(scheduleID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to load overrides: %w", err)
	}

	return buildTimeline(schedule, overrides, from, to), nil
}

func (e *scheduleEvaluator) GetLayerTimelines(scheduleID uuid.UUID, from, to time.Time) (map[uuid.UUID][]TimelineSegment, []TimelineSegment, error) {
	// Apply default window: next 7 days.
	now := time.Now().UTC()
	if from.IsZero() {
		from = now
	}
	if to.IsZero() {
		to = now.Add(7 * 24 * time.Hour)
	}
	if !to.After(from) {
		return nil, nil, fmt.Errorf("to must be after from")
	}

	schedule, err := e.repo.GetWithLayers(scheduleID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load schedule: %w", err)
	}

	overrides, err := e.repo.GetOverridesInWindow(scheduleID, from, to)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load overrides: %w", err)
	}

	// Per-layer timelines: each layer computed independently, no overrides applied.
	layerTimelines := make(map[uuid.UUID][]TimelineSegment, len(schedule.Layers))
	for _, layer := range schedule.Layers {
		singleLayer := *schedule // shallow copy preserves ID, Timezone, etc.
		singleLayer.Layers = []models.ScheduleLayer{layer}
		segs := buildTimeline(&singleLayer, nil, from, to)
		// Tag each segment with the layer ID.
		for i := range segs {
			layerID := layer.ID
			segs[i].LayerID = &layerID
		}
		layerTimelines[layer.ID] = segs
	}

	// Effective timeline: all layers + overrides (same as GetTimeline).
	effective := buildTimeline(schedule, overrides, from, to)

	return layerTimelines, effective, nil
}

// --- Pure computation helpers (no DB access) ---

// computeOnCallFromLayers determines the on-call user by walking layers.
// The first layer (by order_index) with at least one participant wins.
// Returns "" if no layer can yield a user.
func computeOnCallFromLayers(layers []models.ScheduleLayer, at time.Time) string {
	for _, layer := range layers {
		if len(layer.Participants) == 0 {
			continue
		}
		user := computeSlot(layer, at)
		if user != "" {
			return user
		}
	}
	return ""
}

// computeSlot returns the participant username for the given layer at time `at`.
// Uses modulo arithmetic: slotIndex = floor((at - rotation_start) / shift_duration).
// Returns "" if the layer has no participants or at is before rotation_start.
func computeSlot(layer models.ScheduleLayer, at time.Time) string {
	if len(layer.Participants) == 0 {
		return ""
	}
	elapsed := at.Sub(layer.RotationStart)
	if elapsed < 0 {
		// Time is before this layer started; skip.
		return ""
	}
	shiftDur := time.Duration(layer.ShiftDurationSeconds) * time.Second
	if shiftDur <= 0 {
		return ""
	}
	slotIndex := int(elapsed / shiftDur)
	participant := layer.Participants[slotIndex%len(layer.Participants)]
	return participant.UserName
}

// buildTimeline constructs contiguous TimelineSegments for [from, to).
//
// Algorithm:
//  1. Collect all "boundary" time points within the window: from, to, every
//     shift boundary from each layer, and every override start/end.
//  2. Sort and deduplicate boundaries.
//  3. For each sub-interval [boundaries[i], boundaries[i+1]), evaluate
//     WhoIsOnCall at the midpoint using the in-memory schedule and overrides.
//  4. Merge adjacent segments with the same user.
//
// This approach is O(n*m) where n=boundaries, m=layers — acceptable for
// typical windows (7 days, weekly rotations yields ~14 boundaries per layer).
func buildTimeline(
	schedule *models.Schedule,
	overrides []models.ScheduleOverride,
	from, to time.Time,
) []TimelineSegment {
	boundaries := collectBoundaries(schedule.Layers, overrides, from, to)

	var segments []TimelineSegment
	for i := 0; i < len(boundaries)-1; i++ {
		start := boundaries[i]
		end := boundaries[i+1]
		mid := start.Add(end.Sub(start) / 2)

		isOverride, user := resolveAtTime(schedule.Layers, overrides, mid)
		if user == "" {
			user = "(nobody)"
		}

		// Merge with previous segment if same user and same override status.
		if len(segments) > 0 {
			last := &segments[len(segments)-1]
			if last.UserName == user && last.IsOverride == isOverride {
				last.End = end
				continue
			}
		}
		segments = append(segments, TimelineSegment{
			Start:      start,
			End:        end,
			UserName:   user,
			IsOverride: isOverride,
		})
	}
	return segments
}

// collectBoundaries gathers all time points that could change who is on call.
func collectBoundaries(
	layers []models.ScheduleLayer,
	overrides []models.ScheduleOverride,
	from, to time.Time,
) []time.Time {
	seen := map[time.Time]struct{}{
		from: {},
		to:   {},
	}

	// Shift boundaries from each layer
	for _, layer := range layers {
		if layer.ShiftDurationSeconds <= 0 || len(layer.Participants) == 0 {
			continue
		}
		shiftDur := time.Duration(layer.ShiftDurationSeconds) * time.Second

		// Find the first shift boundary at or after `from`
		elapsed := from.Sub(layer.RotationStart)
		var firstBoundary time.Time
		if elapsed <= 0 {
			firstBoundary = layer.RotationStart
		} else {
			// Compute the slot containing `from`, then find the next boundary
			slotIndex := int(elapsed / shiftDur)
			nextBoundary := layer.RotationStart.Add(time.Duration(slotIndex+1) * shiftDur)
			firstBoundary = nextBoundary
		}

		for t := firstBoundary; t.Before(to); t = t.Add(shiftDur) {
			seen[t] = struct{}{}
		}
	}

	// Override start/end times
	// Include boundaries at the window edges: start_time >= from, end_time < to
	for _, ov := range overrides {
		if !ov.StartTime.Before(from) && ov.StartTime.Before(to) {
			seen[ov.StartTime] = struct{}{}
		}
		if ov.EndTime.After(from) && ov.EndTime.Before(to) {
			seen[ov.EndTime] = struct{}{}
		}
	}

	// Sort into a slice
	times := make([]time.Time, 0, len(seen))
	for t := range seen {
		times = append(times, t)
	}
	sort.Slice(times, func(i, j int) bool {
		return times[i].Before(times[j])
	})
	return times
}

// resolveAtTime returns (isOverride, userName) for a given time using in-memory data.
func resolveAtTime(
	layers []models.ScheduleLayer,
	overrides []models.ScheduleOverride,
	at time.Time,
) (bool, string) {
	for _, ov := range overrides {
		if !at.Before(ov.StartTime) && at.Before(ov.EndTime) {
			return true, ov.OverrideUser
		}
	}
	return false, computeOnCallFromLayers(layers, at)
}
