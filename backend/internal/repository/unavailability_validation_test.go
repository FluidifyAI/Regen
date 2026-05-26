package repository

// White-box tests for validateUnavailability — no DB required.
// Lives in package repository (not repository_test) to access the unexported function.

import (
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/google/uuid"
)

func TestValidateUnavailability_Valid(t *testing.T) {
	u := &models.ScheduleUnavailability{
		ScheduleID: uuid.New(),
		UserName:   "alice",
		StartDate:  "2026-05-10",
		EndDate:    "2026-05-14",
	}
	if err := validateUnavailability(u); err != nil {
		t.Errorf("expected no error for valid unavailability, got: %v", err)
	}
}

func TestValidateUnavailability_SingleDay(t *testing.T) {
	u := &models.ScheduleUnavailability{
		ScheduleID: uuid.New(),
		UserName:   "bob",
		StartDate:  "2026-05-10",
		EndDate:    "2026-05-10",
	}
	if err := validateUnavailability(u); err != nil {
		t.Errorf("start == end (single day) should be valid, got: %v", err)
	}
}

func TestValidateUnavailability_MissingScheduleID(t *testing.T) {
	u := &models.ScheduleUnavailability{
		ScheduleID: uuid.Nil,
		UserName:   "alice",
		StartDate:  "2026-05-10",
		EndDate:    "2026-05-14",
	}
	if err := validateUnavailability(u); err == nil {
		t.Error("expected error for nil ScheduleID, got nil")
	}
}

func TestValidateUnavailability_EmptyUserName(t *testing.T) {
	u := &models.ScheduleUnavailability{
		ScheduleID: uuid.New(),
		UserName:   "",
		StartDate:  "2026-05-10",
		EndDate:    "2026-05-14",
	}
	if err := validateUnavailability(u); err == nil {
		t.Error("expected error for empty user_name, got nil")
	}
}

func TestValidateUnavailability_EndBeforeStart(t *testing.T) {
	u := &models.ScheduleUnavailability{
		ScheduleID: uuid.New(),
		UserName:   "alice",
		StartDate:  "2026-05-14",
		EndDate:    "2026-05-10",
	}
	if err := validateUnavailability(u); err == nil {
		t.Error("expected error when end_date is before start_date, got nil")
	}
}

func TestValidateUnavailability_EndBeforeStart_ByOneDay(t *testing.T) {
	u := &models.ScheduleUnavailability{
		ScheduleID: uuid.New(),
		UserName:   "alice",
		StartDate:  "2026-05-11",
		EndDate:    "2026-05-10",
	}
	if err := validateUnavailability(u); err == nil {
		t.Error("expected error when end_date is one day before start_date, got nil")
	}
}
