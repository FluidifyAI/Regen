package repository_test

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var holidayTestDBCounter uint64

func setupHolidayTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	n := atomic.AddUint64(&holidayTestDBCounter, 1)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:holidaytestdb%d?mode=memory&cache=shared", n)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	err = db.Exec(`CREATE TABLE schedule_holidays (
		id           TEXT PRIMARY KEY,
		schedule_id  TEXT NOT NULL,
		country_code TEXT NOT NULL,
		date         DATE NOT NULL,
		name         TEXT NOT NULL,
		source       TEXT NOT NULL DEFAULT 'feed',
		hidden       BOOLEAN NOT NULL DEFAULT 0,
		created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE (schedule_id, country_code, date)
	)`).Error
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

var (
	holidayFrom = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	holidayTo   = time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
)

func feedHoliday(scheduleID uuid.UUID, date, name string) models.ScheduleHoliday {
	return models.ScheduleHoliday{
		ScheduleID: scheduleID, CountryCode: "AU", Date: models.DateOnly(date), Name: name,
		Source: models.HolidaySourceFeed,
	}
}

func listAll(t *testing.T, repo repository.ScheduleRepository, id uuid.UUID, includeHidden bool) []models.ScheduleHoliday {
	t.Helper()
	got, err := repo.ListHolidays(id, holidayFrom, holidayTo, includeHidden)
	if err != nil {
		t.Fatalf("ListHolidays: %v", err)
	}
	return got
}

func TestUpsertHolidays_RefreshesUntouchedFeedRows(t *testing.T) {
	repo := repository.NewScheduleRepository(setupHolidayTestDB(t))
	id := uuid.New()
	if err := repo.UpsertHolidays([]models.ScheduleHoliday{feedHoliday(id, "2026-10-05", "Labour Day")}); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertHolidays([]models.ScheduleHoliday{feedHoliday(id, "2026-10-05", "Labour Day (renamed upstream)")}); err != nil {
		t.Fatal(err)
	}
	got := listAll(t, repo, id, false)
	if len(got) != 1 || got[0].Name != "Labour Day (renamed upstream)" {
		t.Fatalf("got %+v, want the refreshed name", got)
	}
}

func TestUpsertHolidays_DoesNotResurrectHiddenRows(t *testing.T) {
	repo := repository.NewScheduleRepository(setupHolidayTestDB(t))
	id := uuid.New()
	_ = repo.UpsertHolidays([]models.ScheduleHoliday{feedHoliday(id, "2026-10-05", "NSW only day")})
	h := listAll(t, repo, id, false)[0]
	h.Hidden = true
	if err := repo.UpdateHoliday(&h); err != nil {
		t.Fatal(err)
	}

	_ = repo.UpsertHolidays([]models.ScheduleHoliday{feedHoliday(id, "2026-10-05", "NSW only day")})

	if got := listAll(t, repo, id, false); len(got) != 0 {
		t.Fatalf("hidden holiday reappeared after sync: %+v", got)
	}
	all := listAll(t, repo, id, true)
	if len(all) != 1 || !all[0].Hidden {
		t.Fatalf("hidden row should still exist and stay hidden, got %+v", all)
	}
}

func TestUpsertHolidays_DoesNotOverwriteManualEdits(t *testing.T) {
	repo := repository.NewScheduleRepository(setupHolidayTestDB(t))
	id := uuid.New()
	_ = repo.UpsertHolidays([]models.ScheduleHoliday{feedHoliday(id, "2026-10-05", "Labour Day")})
	h := listAll(t, repo, id, false)[0]
	h.Name, h.Source = "King's Birthday (WA)", models.HolidaySourceManual
	if err := repo.UpdateHoliday(&h); err != nil {
		t.Fatal(err)
	}

	_ = repo.UpsertHolidays([]models.ScheduleHoliday{feedHoliday(id, "2026-10-05", "Labour Day")})

	got := listAll(t, repo, id, false)
	if len(got) != 1 || got[0].Name != "King's Birthday (WA)" || got[0].Source != models.HolidaySourceManual {
		t.Fatalf("manual edit was overwritten: %+v", got)
	}
}

func TestListHolidays_HiddenOnlyWhenRequested(t *testing.T) {
	repo := repository.NewScheduleRepository(setupHolidayTestDB(t))
	id := uuid.New()
	_ = repo.UpsertHolidays([]models.ScheduleHoliday{
		feedHoliday(id, "2026-01-26", "Australia Day"),
		feedHoliday(id, "2026-10-05", "NSW only day"),
	})
	for _, h := range listAll(t, repo, id, false) {
		if h.Name == "NSW only day" {
			h.Hidden = true
			_ = repo.UpdateHoliday(&h)
		}
	}
	if got := listAll(t, repo, id, false); len(got) != 1 || got[0].Name != "Australia Day" {
		t.Fatalf("default list should exclude hidden, got %+v", got)
	}
	if got := listAll(t, repo, id, true); len(got) != 2 {
		t.Fatalf("includeHidden should return both, got %+v", got)
	}
}

func TestCreateHoliday_ManualAndDuplicate(t *testing.T) {
	repo := repository.NewScheduleRepository(setupHolidayTestDB(t))
	id := uuid.New()
	h := &models.ScheduleHoliday{
		ScheduleID: id, CountryCode: models.HolidayCountryCustom, Date: "2026-11-03", Name: "Company day",
		Source: models.HolidaySourceManual,
	}
	if err := repo.CreateHoliday(h); err != nil {
		t.Fatalf("CreateHoliday: %v", err)
	}
	if h.ID == uuid.Nil {
		t.Fatal("CreateHoliday should assign an ID")
	}
	dup := *h
	dup.ID = uuid.Nil
	if err := repo.CreateHoliday(&dup); !errors.Is(err, repository.ErrAlreadyExists) {
		t.Fatalf("duplicate date: got %v, want ErrAlreadyExists", err)
	}
}

func TestGetAndDeleteHoliday_ScopedToSchedule(t *testing.T) {
	repo := repository.NewScheduleRepository(setupHolidayTestDB(t))
	id, other := uuid.New(), uuid.New()
	h := &models.ScheduleHoliday{
		ScheduleID: id, CountryCode: models.HolidayCountryCustom, Date: "2026-11-03", Name: "Company day",
		Source: models.HolidaySourceManual,
	}
	_ = repo.CreateHoliday(h)

	if _, err := repo.GetHoliday(other, h.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("other schedule's lookup: got %v, want ErrNotFound", err)
	}
	if err := repo.DeleteHoliday(other, h.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("other schedule's delete: got %v, want ErrNotFound", err)
	}
	if got, err := repo.GetHoliday(id, h.ID); err != nil || got.Name != "Company day" {
		t.Fatalf("GetHoliday: %v %+v", err, got)
	}
	if err := repo.DeleteHoliday(id, h.ID); err != nil {
		t.Fatalf("DeleteHoliday: %v", err)
	}
	if got := listAll(t, repo, id, true); len(got) != 0 {
		t.Fatalf("holiday should be gone, got %+v", got)
	}
}
