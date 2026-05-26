package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── unfoldLines ─────────────────────────────────────────────────────────────

func TestUnfoldLines_NoFolding(t *testing.T) {
	input := "BEGIN:VCALENDAR\nSUMMARY:New Year\nEND:VCALENDAR\n"
	result := unfoldLines(strings.NewReader(input))
	assert.Contains(t, result, "BEGIN:VCALENDAR")
	assert.Contains(t, result, "SUMMARY:New Year")
}

func TestUnfoldLines_SpaceContinuation(t *testing.T) {
	// RFC 5545: the leading space on the continuation line is the fold indicator
	// and is stripped — not part of the content.
	input := "SUMMARY:Happy New\n Year's Day\n"
	result := unfoldLines(strings.NewReader(input))
	assert.Contains(t, result, "SUMMARY:Happy NewYear's Day")
}

func TestUnfoldLines_TabContinuation(t *testing.T) {
	input := "SUMMARY:Long\n\tHoliday Name\n"
	result := unfoldLines(strings.NewReader(input))
	assert.Contains(t, result, "SUMMARY:LongHoliday Name")
}

// ─── parseICS ────────────────────────────────────────────────────────────────

func TestParseICS_BasicEvent(t *testing.T) {
	ics := "BEGIN:VCALENDAR\nBEGIN:VEVENT\nDTSTART;VALUE=DATE:20260101\nSUMMARY:New Year's Day\nEND:VEVENT\nEND:VCALENDAR\n"
	entries, err := parseICS(strings.NewReader(ics))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "New Year's Day", entries[0].name)
	assert.Equal(t, "2026-01-01", entries[0].date.String())
}

func TestParseICS_DtStartNoValueType(t *testing.T) {
	// Some feeds use DTSTART:20260815 without the ;VALUE=DATE qualifier
	ics := "BEGIN:VEVENT\nDTSTART:20260815\nSUMMARY:Independence Day\nEND:VEVENT\n"
	entries, err := parseICS(strings.NewReader(ics))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "Independence Day", entries[0].name)
	assert.Equal(t, "2026-08-15", entries[0].date.String())
}

func TestParseICS_MultipleEvents(t *testing.T) {
	ics := strings.Join([]string{
		"BEGIN:VCALENDAR",
		"BEGIN:VEVENT",
		"DTSTART;VALUE=DATE:20260101",
		"SUMMARY:New Year's Day",
		"END:VEVENT",
		"BEGIN:VEVENT",
		"DTSTART;VALUE=DATE:20260815",
		"SUMMARY:Independence Day",
		"END:VEVENT",
		"BEGIN:VEVENT",
		"DTSTART;VALUE=DATE:20261025",
		"SUMMARY:Dussehra",
		"END:VEVENT",
		"END:VCALENDAR",
	}, "\n")

	entries, err := parseICS(strings.NewReader(ics))
	require.NoError(t, err)
	assert.Len(t, entries, 3)
}

func TestParseICS_SkipsEventMissingSummary(t *testing.T) {
	ics := "BEGIN:VEVENT\nDTSTART;VALUE=DATE:20260101\nEND:VEVENT\n"
	entries, err := parseICS(strings.NewReader(ics))
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestParseICS_SkipsEventMissingDate(t *testing.T) {
	ics := "BEGIN:VEVENT\nSUMMARY:Some Holiday\nEND:VEVENT\n"
	entries, err := parseICS(strings.NewReader(ics))
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestParseICS_SkipsMalformedDate(t *testing.T) {
	ics := "BEGIN:VEVENT\nDTSTART;VALUE=DATE:not-a-date\nSUMMARY:Bad Event\nEND:VEVENT\n"
	entries, err := parseICS(strings.NewReader(ics))
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestParseICS_SkipsDatetimeEvents(t *testing.T) {
	// Datetime events (not all-day) have a time component — should not be parsed
	// as holidays since our format expects YYYYMMDD only
	ics := "BEGIN:VEVENT\nDTSTART:20260101T090000Z\nSUMMARY:Meeting\nEND:VEVENT\n"
	entries, err := parseICS(strings.NewReader(ics))
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestParseICS_DuplicateDates(t *testing.T) {
	// Indian ICS feed has multiple VEVENTs for the same date (regional variants).
	// parseICS returns them all — deduplication happens in UpsertHolidays.
	ics := strings.Join([]string{
		"BEGIN:VEVENT",
		"DTSTART;VALUE=DATE:20260527",
		"SUMMARY:Bakrid (tentative)",
		"END:VEVENT",
		"BEGIN:VEVENT",
		"DTSTART;VALUE=DATE:20260527",
		"SUMMARY:Bakrid",
		"END:VEVENT",
	}, "\n")

	entries, err := parseICS(strings.NewReader(ics))
	require.NoError(t, err)
	assert.Len(t, entries, 2, "parseICS returns both; caller deduplicates")
}

func TestParseICS_EmptyInput(t *testing.T) {
	entries, err := parseICS(strings.NewReader(""))
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestParseICS_FoldedSummary(t *testing.T) {
	// Long summary line folded at 75 chars per RFC 5545
	ics := "BEGIN:VEVENT\nDTSTART;VALUE=DATE:20260115\nSUMMARY:Makar Sankranti /\n  Pongal / Uttarayan\nEND:VEVENT\n"
	entries, err := parseICS(strings.NewReader(ics))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "Makar Sankranti / Pongal / Uttarayan", entries[0].name)
}
