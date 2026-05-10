package services

import (
	"testing"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/google/uuid"
)

// ── Helpers ───────────────────────────────────────────────────────────────────

// day returns midnight UTC for the given Y-M-D.
func day(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

// makeLayer builds a ScheduleLayer with daily rotation and the given participants (in order).
func makeLayer(rotationStart time.Time, shiftDuration time.Duration, participants ...string) models.ScheduleLayer {
	ps := make([]models.ScheduleParticipant, len(participants))
	for i, name := range participants {
		ps[i] = models.ScheduleParticipant{UserName: name, OrderIndex: i}
	}
	return models.ScheduleLayer{
		ID:                   uuid.New(),
		ScheduleID:           uuid.New(),
		RotationStart:        rotationStart,
		ShiftDurationSeconds: int(shiftDuration.Seconds()),
		Participants:         ps,
	}
}

// makeUnavailability builds a ScheduleUnavailability for the given user over [start, end] (inclusive dates).
func makeUnavailability(scheduleID uuid.UUID, user string, start, end time.Time) models.ScheduleUnavailability {
	return models.ScheduleUnavailability{
		ID:         uuid.New(),
		ScheduleID: scheduleID,
		UserName:   user,
		StartDate:  start,
		EndDate:    end,
	}
}

// ── buildUnavailableSet ───────────────────────────────────────────────────────

func TestBuildUnavailableSet_NoUnavailabilities(t *testing.T) {
	at := day(2026, 5, 10)
	result := buildUnavailableSet(nil, at)
	if len(result) != 0 {
		t.Errorf("expected empty set, got %v", result)
	}
}

func TestBuildUnavailableSet_ActiveOnExactDay(t *testing.T) {
	schedID := uuid.New()
	at := day(2026, 5, 10)
	u := makeUnavailability(schedID, "alice", day(2026, 5, 10), day(2026, 5, 10))
	result := buildUnavailableSet([]models.ScheduleUnavailability{u}, at)
	if _, ok := result["alice"]; !ok {
		t.Error("alice should be unavailable on her start=end date")
	}
}

func TestBuildUnavailableSet_ActiveMidRange(t *testing.T) {
	schedID := uuid.New()
	at := day(2026, 5, 12)
	u := makeUnavailability(schedID, "bob", day(2026, 5, 10), day(2026, 5, 14))
	result := buildUnavailableSet([]models.ScheduleUnavailability{u}, at)
	if _, ok := result["bob"]; !ok {
		t.Error("bob should be unavailable in the middle of his range")
	}
}

func TestBuildUnavailableSet_ExpiredYesterday(t *testing.T) {
	schedID := uuid.New()
	at := day(2026, 5, 10)
	u := makeUnavailability(schedID, "carol", day(2026, 5, 8), day(2026, 5, 9))
	result := buildUnavailableSet([]models.ScheduleUnavailability{u}, at)
	if _, ok := result["carol"]; ok {
		t.Error("carol's unavailability ended yesterday; she should be available today")
	}
}

func TestBuildUnavailableSet_StartsTimorrow(t *testing.T) {
	schedID := uuid.New()
	at := day(2026, 5, 10)
	u := makeUnavailability(schedID, "dave", day(2026, 5, 11), day(2026, 5, 15))
	result := buildUnavailableSet([]models.ScheduleUnavailability{u}, at)
	if _, ok := result["dave"]; ok {
		t.Error("dave's unavailability starts tomorrow; he should be available today")
	}
}

func TestBuildUnavailableSet_MultipleUsers(t *testing.T) {
	schedID := uuid.New()
	at := day(2026, 5, 10)
	unavails := []models.ScheduleUnavailability{
		makeUnavailability(schedID, "alice", day(2026, 5, 9), day(2026, 5, 11)),
		makeUnavailability(schedID, "bob", day(2026, 5, 10), day(2026, 5, 10)),
		makeUnavailability(schedID, "carol", day(2026, 5, 11), day(2026, 5, 13)), // tomorrow
	}
	result := buildUnavailableSet(unavails, at)
	if _, ok := result["alice"]; !ok {
		t.Error("alice should be unavailable")
	}
	if _, ok := result["bob"]; !ok {
		t.Error("bob should be unavailable")
	}
	if _, ok := result["carol"]; ok {
		t.Error("carol starts tomorrow; should be available today")
	}
}

func TestBuildUnavailableSet_MidnightBoundary_StartOfDay(t *testing.T) {
	schedID := uuid.New()
	// Check: 00:00:01 UTC on start day → still in range
	at := day(2026, 5, 10).Add(time.Second)
	u := makeUnavailability(schedID, "alice", day(2026, 5, 10), day(2026, 5, 10))
	result := buildUnavailableSet([]models.ScheduleUnavailability{u}, at)
	if _, ok := result["alice"]; !ok {
		t.Error("alice should be unavailable at 00:00:01 on her start date")
	}
}

func TestBuildUnavailableSet_MidnightBoundary_LastSecondOfEndDay(t *testing.T) {
	schedID := uuid.New()
	// Check: 23:59:59 UTC on end day → still in range
	at := day(2026, 5, 10).Add(24*time.Hour - time.Second)
	u := makeUnavailability(schedID, "alice", day(2026, 5, 10), day(2026, 5, 10))
	result := buildUnavailableSet([]models.ScheduleUnavailability{u}, at)
	if _, ok := result["alice"]; !ok {
		t.Error("alice should be unavailable at 23:59:59 on her end date")
	}
}

// ── computeSlotSkipping ───────────────────────────────────────────────────────

func TestComputeSlotSkipping_NilUnavailable_SameAsComputeSlot(t *testing.T) {
	epoch := day(2026, 1, 1)
	layer := makeLayer(epoch, 24*time.Hour, "alice", "bob", "carol")

	for i := 0; i < 6; i++ {
		at := epoch.Add(time.Duration(i) * 24 * time.Hour)
		want := computeSlot(layer, at)
		got := computeSlotSkipping(layer, at, nil)
		if got != want {
			t.Errorf("slot %d: computeSlotSkipping = %q, want %q", i, got, want)
		}
	}
}

func TestComputeSlotSkipping_SkipsUnavailableParticipant(t *testing.T) {
	epoch := day(2026, 1, 1)
	layer := makeLayer(epoch, 24*time.Hour, "alice", "bob", "carol")

	at := epoch // slot 0 → alice
	unavailable := map[string]struct{}{"alice": {}}

	got := computeSlotSkipping(layer, at, unavailable)
	if got != "bob" {
		t.Errorf("expected bob (next after unavailable alice), got %q", got)
	}
}

func TestComputeSlotSkipping_SkipsMultiple(t *testing.T) {
	epoch := day(2026, 1, 1)
	layer := makeLayer(epoch, 24*time.Hour, "alice", "bob", "carol")

	at := epoch // slot 0 → alice
	unavailable := map[string]struct{}{"alice": {}, "bob": {}}

	got := computeSlotSkipping(layer, at, unavailable)
	if got != "carol" {
		t.Errorf("expected carol (only available), got %q", got)
	}
}

func TestComputeSlotSkipping_AllUnavailable_ReturnsEmpty(t *testing.T) {
	epoch := day(2026, 1, 1)
	layer := makeLayer(epoch, 24*time.Hour, "alice", "bob")

	at := epoch
	unavailable := map[string]struct{}{"alice": {}, "bob": {}}

	got := computeSlotSkipping(layer, at, unavailable)
	if got != "" {
		t.Errorf("expected empty string when all unavailable, got %q", got)
	}
}

func TestComputeSlotSkipping_BeforeRotationStart_ReturnsEmpty(t *testing.T) {
	epoch := day(2026, 6, 1) // rotation starts in the future
	layer := makeLayer(epoch, 24*time.Hour, "alice", "bob")

	at := day(2026, 5, 10) // before epoch
	got := computeSlotSkipping(layer, at, nil)
	if got != "" {
		t.Errorf("expected empty string before rotation start, got %q", got)
	}
}

func TestComputeSlotSkipping_EmptyParticipants_ReturnsEmpty(t *testing.T) {
	epoch := day(2026, 1, 1)
	layer := makeLayer(epoch, 24*time.Hour)

	got := computeSlotSkipping(layer, epoch, nil)
	if got != "" {
		t.Errorf("expected empty string with no participants, got %q", got)
	}
}

func TestComputeSlotSkipping_WrapAround(t *testing.T) {
	epoch := day(2026, 1, 1)
	layer := makeLayer(epoch, 24*time.Hour, "alice", "bob", "carol")

	// Day 3 = slot 3, which wraps to carol (index 0) → alice is skipped → bob
	at := epoch.Add(3 * 24 * time.Hour) // slot index 3 mod 3 = 0 = alice
	unavailable := map[string]struct{}{"alice": {}}

	got := computeSlotSkipping(layer, at, unavailable)
	if got != "bob" {
		t.Errorf("expected bob after wrap-around skip of alice, got %q", got)
	}
}

// ── computeOnCallFromLayersSkipping ──────────────────────────────────────────

func TestComputeOnCallFromLayersSkipping_FallsThrough_WhenLayerFullyUnavailable(t *testing.T) {
	epoch := day(2026, 1, 1)

	// Layer 0 (primary): only alice
	layer0 := makeLayer(epoch, 24*time.Hour, "alice")
	// Layer 1 (secondary): only bob
	layer1 := makeLayer(epoch, 24*time.Hour, "bob")

	layers := []models.ScheduleLayer{layer0, layer1}

	at := epoch
	unavailable := map[string]struct{}{"alice": {}}

	got := computeOnCallFromLayersSkipping(layers, at, unavailable)
	if got != "bob" {
		t.Errorf("expected fallthrough to bob (layer 1), got %q", got)
	}
}

func TestComputeOnCallFromLayersSkipping_AllLayersUnavailable_ReturnsEmpty(t *testing.T) {
	epoch := day(2026, 1, 1)
	layer0 := makeLayer(epoch, 24*time.Hour, "alice")
	layer1 := makeLayer(epoch, 24*time.Hour, "bob")

	unavailable := map[string]struct{}{"alice": {}, "bob": {}}
	got := computeOnCallFromLayersSkipping([]models.ScheduleLayer{layer0, layer1}, epoch, unavailable)
	if got != "" {
		t.Errorf("expected empty when all layers unavailable, got %q", got)
	}
}

func TestComputeOnCallFromLayersSkipping_NilUnavailable_SameAsPrimary(t *testing.T) {
	epoch := day(2026, 1, 1)
	layer0 := makeLayer(epoch, 24*time.Hour, "alice", "bob")
	layer1 := makeLayer(epoch, 24*time.Hour, "carol")

	got := computeOnCallFromLayersSkipping([]models.ScheduleLayer{layer0, layer1}, epoch, nil)
	if got != "alice" {
		t.Errorf("expected alice (layer 0, slot 0), got %q", got)
	}
}

// ── collectBoundaries (unavailability boundaries) ─────────────────────────────

func TestCollectBoundaries_IncludesUnavailabilityDayBoundaries(t *testing.T) {
	schedID := uuid.New()
	epoch := day(2026, 5, 1)
	layer := makeLayer(epoch, 7*24*time.Hour, "alice", "bob") // weekly rotation
	from := day(2026, 5, 1)
	to := day(2026, 5, 15)

	unavail := makeUnavailability(schedID, "alice", day(2026, 5, 8), day(2026, 5, 10))
	bounds := collectBoundaries([]models.ScheduleLayer{layer}, nil, []models.ScheduleUnavailability{unavail}, from, to)

	boundSet := make(map[time.Time]struct{}, len(bounds))
	for _, b := range bounds {
		boundSet[b] = struct{}{}
	}

	// start_date boundary: midnight May 8
	if _, ok := boundSet[day(2026, 5, 8)]; !ok {
		t.Error("expected boundary at unavailability start (May 8)")
	}
	// resume boundary: midnight May 11 (day after end_date May 10)
	if _, ok := boundSet[day(2026, 5, 11)]; !ok {
		t.Error("expected boundary at day after unavailability end (May 11)")
	}
}

func TestCollectBoundaries_NoUnavailabilities_BoundariesUnchanged(t *testing.T) {
	epoch := day(2026, 5, 1)
	layer := makeLayer(epoch, 7*24*time.Hour, "alice", "bob")
	from := day(2026, 5, 1)
	to := day(2026, 5, 15)

	withNone := collectBoundaries([]models.ScheduleLayer{layer}, nil, nil, from, to)
	withEmpty := collectBoundaries([]models.ScheduleLayer{layer}, nil, []models.ScheduleUnavailability{}, from, to)

	if len(withNone) != len(withEmpty) {
		t.Errorf("nil and empty unavailabilities should produce same boundary count: %d vs %d", len(withNone), len(withEmpty))
	}
}

func TestCollectBoundaries_UnavailabilityOutsideWindow_NotAdded(t *testing.T) {
	schedID := uuid.New()
	epoch := day(2026, 5, 1)
	layer := makeLayer(epoch, 7*24*time.Hour, "alice")
	from := day(2026, 5, 1)
	to := day(2026, 5, 7)

	// Unavailability entirely after the window
	unavail := makeUnavailability(schedID, "alice", day(2026, 5, 20), day(2026, 5, 25))
	bounds := collectBoundaries([]models.ScheduleLayer{layer}, nil, []models.ScheduleUnavailability{unavail}, from, to)

	for _, b := range bounds {
		if b.Equal(day(2026, 5, 20)) || b.Equal(day(2026, 5, 26)) {
			t.Errorf("boundary %v outside window should not be added", b)
		}
	}
}

// ── buildTimeline with unavailabilities ──────────────────────────────────────

func TestBuildTimeline_UnavailabilitySplitsSegment(t *testing.T) {
	schedID := uuid.New()
	epoch := day(2026, 5, 1)
	// Daily rotation: alice (slot 0), bob (slot 1), alice (slot 2), bob (slot 3) ...
	layer := makeLayer(epoch, 24*time.Hour, "alice", "bob")
	schedule := &models.Schedule{
		ID:     schedID,
		Layers: []models.ScheduleLayer{layer},
	}

	// Alice is unavailable May 10-10 (one day). During that day bob should be on call.
	unavail := makeUnavailability(schedID, "alice", day(2026, 5, 10), day(2026, 5, 10))

	from := day(2026, 5, 9)
	to := day(2026, 5, 12)
	segments := buildTimeline(schedule, nil, []models.ScheduleUnavailability{unavail}, from, to)

	// Expect at least 3 segments: [May 9: alice], [May 10: bob], [May 11: alice]
	if len(segments) < 3 {
		t.Fatalf("expected at least 3 segments, got %d: %+v", len(segments), segments)
	}

	// Find the May 10 segment — it should be bob
	for _, s := range segments {
		if s.Start.Equal(day(2026, 5, 10)) {
			if s.UserName != "bob" {
				t.Errorf("May 10 segment: expected bob (alice is unavailable), got %q", s.UserName)
			}
			return
		}
	}
	t.Error("no segment starting at May 10 found")
}

func TestBuildTimeline_NoUnavailabilities_NormalRotation(t *testing.T) {
	epoch := day(2026, 5, 1)
	layer := makeLayer(epoch, 24*time.Hour, "alice", "bob")
	schedule := &models.Schedule{
		ID:     uuid.New(),
		Layers: []models.ScheduleLayer{layer},
	}

	from := day(2026, 5, 1)
	to := day(2026, 5, 5)
	segments := buildTimeline(schedule, nil, nil, from, to)

	if len(segments) != 4 {
		t.Fatalf("expected 4 daily segments, got %d", len(segments))
	}

	expected := []string{"alice", "bob", "alice", "bob"}
	for i, s := range segments {
		if s.UserName != expected[i] {
			t.Errorf("segment %d: expected %q, got %q", i, expected[i], s.UserName)
		}
	}
}

func TestBuildTimeline_OverrideWinsOverUnavailability(t *testing.T) {
	schedID := uuid.New()
	epoch := day(2026, 5, 1)
	layer := makeLayer(epoch, 24*time.Hour, "alice", "bob")
	schedule := &models.Schedule{
		ID:     schedID,
		Layers: []models.ScheduleLayer{layer},
	}

	// Alice is unavailable May 10
	unavail := makeUnavailability(schedID, "alice", day(2026, 5, 10), day(2026, 5, 10))

	// Override for May 10: carol explicitly takes over (should win over skip logic)
	override := models.ScheduleOverride{
		ID:           uuid.New(),
		ScheduleID:   schedID,
		OverrideUser: "carol",
		StartTime:    day(2026, 5, 10),
		EndTime:      day(2026, 5, 11),
	}

	from := day(2026, 5, 10)
	to := day(2026, 5, 11)
	segments := buildTimeline(schedule, []models.ScheduleOverride{override}, []models.ScheduleUnavailability{unavail}, from, to)

	if len(segments) == 0 {
		t.Fatal("expected at least one segment")
	}
	// The entire window is covered by the override
	if segments[0].UserName != "carol" {
		t.Errorf("override should win: expected carol, got %q", segments[0].UserName)
	}
	if !segments[0].IsOverride {
		t.Error("segment should be marked as override")
	}
}

func TestBuildTimeline_AllParticipantsUnavailable_ShowsNobody(t *testing.T) {
	schedID := uuid.New()
	epoch := day(2026, 5, 10)
	layer := makeLayer(epoch, 24*time.Hour, "alice")
	schedule := &models.Schedule{
		ID:     schedID,
		Layers: []models.ScheduleLayer{layer},
	}

	unavail := makeUnavailability(schedID, "alice", day(2026, 5, 10), day(2026, 5, 10))

	from := day(2026, 5, 10)
	to := day(2026, 5, 11)
	segments := buildTimeline(schedule, nil, []models.ScheduleUnavailability{unavail}, from, to)

	if len(segments) == 0 {
		t.Fatal("expected at least one segment")
	}
	if segments[0].UserName != "(nobody)" {
		t.Errorf("expected (nobody) when only participant is unavailable, got %q", segments[0].UserName)
	}
}

// ── resolveAtTime ─────────────────────────────────────────────────────────────

func TestResolveAtTime_NoOverrides_NoUnavailabilities(t *testing.T) {
	epoch := day(2026, 1, 1)
	layer := makeLayer(epoch, 24*time.Hour, "alice", "bob")

	isOverride, user := resolveAtTime([]models.ScheduleLayer{layer}, nil, nil, epoch)
	if isOverride {
		t.Error("should not be override")
	}
	if user != "alice" {
		t.Errorf("expected alice at slot 0, got %q", user)
	}
}

func TestResolveAtTime_UnavailableUser_AdvancesRotation(t *testing.T) {
	schedID := uuid.New()
	epoch := day(2026, 1, 1)
	layer := makeLayer(epoch, 24*time.Hour, "alice", "bob")

	unavail := makeUnavailability(schedID, "alice", day(2026, 1, 1), day(2026, 1, 1))
	isOverride, user := resolveAtTime([]models.ScheduleLayer{layer}, nil, []models.ScheduleUnavailability{unavail}, epoch)
	if isOverride {
		t.Error("should not be override")
	}
	if user != "bob" {
		t.Errorf("alice unavailable: expected bob, got %q", user)
	}
}

func TestResolveAtTime_OverrideWins_EvenWhenUnavailable(t *testing.T) {
	schedID := uuid.New()
	epoch := day(2026, 1, 1)
	layer := makeLayer(epoch, 24*time.Hour, "alice")

	// Alice is unavailable
	unavail := makeUnavailability(schedID, "alice", day(2026, 1, 1), day(2026, 1, 1))
	// Override puts carol on-call
	override := models.ScheduleOverride{
		ScheduleID:   schedID,
		OverrideUser: "carol",
		StartTime:    epoch,
		EndTime:      epoch.Add(24 * time.Hour),
	}

	isOverride, user := resolveAtTime(
		[]models.ScheduleLayer{layer},
		[]models.ScheduleOverride{override},
		[]models.ScheduleUnavailability{unavail},
		epoch.Add(time.Hour), // 01:00 on Jan 1 — inside override window
	)
	if !isOverride {
		t.Error("should be marked as override")
	}
	if user != "carol" {
		t.Errorf("override should win: expected carol, got %q", user)
	}
}
