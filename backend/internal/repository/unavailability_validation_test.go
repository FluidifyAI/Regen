package repository

// White-box tests for validateUnavailability — no DB required.
// Lives in package repository (not repository_test) to access the unexported function.

import (
	"testing"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/google/uuid"
)

func midnightUTCForTest(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func TestValidateUnavailability_Valid(t *testing.T) {
	u := &models.ScheduleUnavailability{
		ScheduleID: uuid.New(),
		UserName:   "alice",
		StartDate:  midnightUTCForTest(2026, 5, 10),
		EndDate:    midnightUTCForTest(2026, 5, 14),
	}
	if err := validateUnavailability(u); err != nil {
		t.Errorf("expected no error for valid unavailability, got: %v", err)
	}
}

func TestValidateUnavailability_SingleDay(t *testing.T) {
	u := &models.ScheduleUnavailability{
		ScheduleID: uuid.New(),
		UserName:   "bob",
		StartDate:  midnightUTCForTest(2026, 5, 10),
		EndDate:    midnightUTCForTest(2026, 5, 10), // same day — valid
	}
	if err := validateUnavailability(u); err != nil {
		t.Errorf("start == end (single day) should be valid, got: %v", err)
	}
}

func TestValidateUnavailability_MissingScheduleID(t *testing.T) {
	u := &models.ScheduleUnavailability{
		ScheduleID: uuid.Nil,
		UserName:   "alice",
		StartDate:  midnightUTCForTest(2026, 5, 10),
		EndDate:    midnightUTCForTest(2026, 5, 14),
	}
	if err := validateUnavailability(u); err == nil {
		t.Error("expected error for nil ScheduleID, got nil")
	}
}

func TestValidateUnavailability_EmptyUserName(t *testing.T) {
	u := &models.ScheduleUnavailability{
		ScheduleID: uuid.New(),
		UserName:   "",
		StartDate:  midnightUTCForTest(2026, 5, 10),
		EndDate:    midnightUTCForTest(2026, 5, 14),
	}
	if err := validateUnavailability(u); err == nil {
		t.Error("expected error for empty user_name, got nil")
	}
}

func TestValidateUnavailability_EndBeforeStart(t *testing.T) {
	u := &models.ScheduleUnavailability{
		ScheduleID: uuid.New(),
		UserName:   "alice",
		StartDate:  midnightUTCForTest(2026, 5, 14),
		EndDate:    midnightUTCForTest(2026, 5, 10), // before start
	}
	if err := validateUnavailability(u); err == nil {
		t.Error("expected error when end_date is before start_date, got nil")
	}
}

func TestValidateUnavailability_EndBeforeStart_ByOneDay(t *testing.T) {
	u := &models.ScheduleUnavailability{
		ScheduleID: uuid.New(),
		UserName:   "alice",
		StartDate:  midnightUTCForTest(2026, 5, 11),
		EndDate:    midnightUTCForTest(2026, 5, 10), // one day before start
	}
	if err := validateUnavailability(u); err == nil {
		t.Error("expected error when end_date is one day before start_date, got nil")
	}
}
