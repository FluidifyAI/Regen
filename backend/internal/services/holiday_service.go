package services

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/google/uuid"
)

// supportedCountries maps ISO country codes to their Google Calendar public
// holiday ICS feed URLs. These feeds are maintained by Google and are reliable.
var supportedCountries = map[string]string{
	"US": "https://calendar.google.com/calendar/ical/en.usa%23holiday%40group.v.calendar.google.com/public/basic.ics",
	"CA": "https://calendar.google.com/calendar/ical/en.canadian%23holiday%40group.v.calendar.google.com/public/basic.ics",
	"GB": "https://calendar.google.com/calendar/ical/en.uk%23holiday%40group.v.calendar.google.com/public/basic.ics",
	"AU": "https://calendar.google.com/calendar/ical/en.australian%23holiday%40group.v.calendar.google.com/public/basic.ics",
	"DE": "https://calendar.google.com/calendar/ical/de.german%23holiday%40group.v.calendar.google.com/public/basic.ics",
	"FR": "https://calendar.google.com/calendar/ical/fr.french%23holiday%40group.v.calendar.google.com/public/basic.ics",
	"IN": "https://calendar.google.com/calendar/ical/en.indian%23holiday%40group.v.calendar.google.com/public/basic.ics",
}

// SupportedCountryCodes returns the list of country codes supported for holiday sync.
func SupportedCountryCodes() []string {
	codes := make([]string, 0, len(supportedCountries))
	for code := range supportedCountries {
		codes = append(codes, code)
	}
	return codes
}

// HolidayService fetches public holidays from ICS feeds and syncs them to the DB.
type HolidayService struct {
	repo       repository.ScheduleRepository
	httpClient *http.Client
}

// NewHolidayService creates a new HolidayService.
func NewHolidayService(repo repository.ScheduleRepository) *HolidayService {
	return &HolidayService{
		repo:       repo,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// SyncSchedule fetches holidays for the given countries and upserts them for scheduleID.
// Countries not in supportedCountries are silently skipped.
func (s *HolidayService) SyncSchedule(scheduleID uuid.UUID, countries []string) error {
	for _, code := range countries {
		url, ok := supportedCountries[code]
		if !ok {
			slog.Warn("holiday sync: unsupported country code, skipping", "country", code, "schedule_id", scheduleID)
			continue
		}
		holidays, err := s.fetchHolidays(scheduleID, code, url)
		if err != nil {
			slog.Error("holiday sync: fetch failed", "country", code, "schedule_id", scheduleID, "error", err)
			continue
		}
		if err := s.repo.UpsertHolidays(holidays); err != nil {
			slog.Error("holiday sync: upsert failed", "country", code, "schedule_id", scheduleID, "error", err)
			continue
		}
		slog.Info("holiday sync: complete", "country", code, "schedule_id", scheduleID, "count", len(holidays))
	}
	return nil
}

// SyncAll refreshes holidays for every schedule that has holiday countries configured.
func (s *HolidayService) SyncAll() {
	schedules, err := s.repo.ListSchedulesWithHolidays()
	if err != nil {
		slog.Error("holiday sync: failed to list schedules", "error", err)
		return
	}
	for _, sc := range schedules {
		if len(sc.HolidayCountries) == 0 {
			continue
		}
		if err := s.SyncSchedule(sc.ID, sc.HolidayCountries); err != nil {
			slog.Error("holiday sync: schedule sync failed", "schedule_id", sc.ID, "error", err)
		}
	}
}

// fetchHolidays downloads and parses the ICS feed for one country.
func (s *HolidayService) fetchHolidays(scheduleID uuid.UUID, code, url string) ([]models.ScheduleHoliday, error) {
	resp, err := s.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch ICS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ICS feed returned %d", resp.StatusCode)
	}

	entries, err := parseICS(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("parse ICS: %w", err)
	}

	holidays := make([]models.ScheduleHoliday, 0, len(entries))
	for _, e := range entries {
		holidays = append(holidays, models.ScheduleHoliday{
			ScheduleID:  scheduleID,
			CountryCode: code,
			Date:        e.date,
			Name:        e.name,
		})
	}
	return holidays, nil
}

// holidayEntry is a parsed (date, name) pair from an ICS feed.
type holidayEntry struct {
	date models.DateOnly
	name string
}

// parseICS parses an iCalendar stream and extracts all-day VEVENT entries.
// Handles ICS line folding (continuation lines starting with space/tab).
// Only processes DTSTART;VALUE=DATE (date-only) entries — skips datetime events.
func parseICS(r io.Reader) ([]holidayEntry, error) {
	unfolded := unfoldLines(r)
	scanner := bufio.NewScanner(strings.NewReader(unfolded))

	var entries []holidayEntry
	var cur holidayEntry
	inEvent := false

	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "BEGIN:VEVENT":
			inEvent = true
			cur = holidayEntry{}
		case line == "END:VEVENT":
			if inEvent && cur.date != "" && cur.name != "" {
				entries = append(entries, cur)
			}
			inEvent = false
		case inEvent && strings.HasPrefix(line, "DTSTART"):
			// Matches: DTSTART;VALUE=DATE:20260101  or  DTSTART:20260101
			if idx := strings.LastIndex(line, ":"); idx >= 0 {
				val := strings.TrimSpace(line[idx+1:])
				if t, err := time.Parse("20060102", val); err == nil {
					cur.date = models.DateOnlyFromTime(t)
				}
			}
		case inEvent && strings.HasPrefix(line, "SUMMARY:"):
			cur.name = strings.TrimPrefix(line, "SUMMARY:")
		}
	}
	return entries, scanner.Err()
}

// unfoldLines joins ICS folded lines (continuation lines start with space or tab).
func unfoldLines(r io.Reader) string {
	scanner := bufio.NewScanner(r)
	var sb strings.Builder
	var prev string
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			prev += line[1:]
		} else {
			if prev != "" {
				sb.WriteString(prev)
				sb.WriteByte('\n')
			}
			prev = line
		}
	}
	if prev != "" {
		sb.WriteString(prev)
		sb.WriteByte('\n')
	}
	return sb.String()
}
