package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDateOnly_ScanTimeTime(t *testing.T) {
	// Midnight UTC — the format pgx delivers DATE columns
	ts := time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC)
	var d DateOnly
	require.NoError(t, d.Scan(ts))
	assert.Equal(t, DateOnly("2026-05-24"), d)
}

func TestDateOnly_ScanTimeTime_NonMidnight(t *testing.T) {
	// Any non-zero time on the same UTC day must still give the UTC date
	ts := time.Date(2026, 5, 24, 18, 30, 0, 0, time.UTC)
	var d DateOnly
	require.NoError(t, d.Scan(ts))
	assert.Equal(t, DateOnly("2026-05-24"), d)
}

func TestDateOnly_ScanTimeTime_UTC_Plus(t *testing.T) {
	// time.Time in UTC+5:30 at local midnight: 2026-05-24T00:00:00+05:30
	// UTC equivalent: 2026-05-23T18:30:00Z  — UTC date is May 23
	// Scan must use UTC to yield "2026-05-23", not "2026-05-24"
	ist := time.FixedZone("IST", 5*3600+30*60)
	ts := time.Date(2026, 5, 24, 0, 0, 0, 0, ist) // local midnight IST
	var d DateOnly
	require.NoError(t, d.Scan(ts))
	// UTC date for this instant is 2026-05-23
	assert.Equal(t, DateOnly("2026-05-23"), d)
}

func TestDateOnly_ScanString(t *testing.T) {
	var d DateOnly
	require.NoError(t, d.Scan("2026-05-24"))
	assert.Equal(t, DateOnly("2026-05-24"), d)
}

func TestDateOnly_ScanString_Invalid(t *testing.T) {
	var d DateOnly
	assert.Error(t, d.Scan("24-05-2026"))
}

func TestDateOnly_ScanNil(t *testing.T) {
	var d DateOnly
	require.NoError(t, d.Scan(nil))
	assert.Equal(t, DateOnly(""), d)
}

func TestDateOnly_Value(t *testing.T) {
	d := DateOnly("2026-05-24")
	v, err := d.Value()
	require.NoError(t, err)
	assert.Equal(t, "2026-05-24", v)
}

func TestDateOnly_Value_Empty(t *testing.T) {
	d := DateOnly("")
	v, err := d.Value()
	require.NoError(t, err)
	assert.Nil(t, v)
}

func TestDateOnly_JSONMarshal(t *testing.T) {
	d := DateOnly("2026-05-24")
	b, err := json.Marshal(d)
	require.NoError(t, err)
	assert.Equal(t, `"2026-05-24"`, string(b))
}

func TestDateOnly_JSONUnmarshal(t *testing.T) {
	var d DateOnly
	require.NoError(t, json.Unmarshal([]byte(`"2026-05-24"`), &d))
	assert.Equal(t, DateOnly("2026-05-24"), d)
}

func TestDateOnly_JSONRoundTrip_InStruct(t *testing.T) {
	type payload struct {
		Date DateOnly `json:"date"`
	}
	in := payload{Date: "2026-05-24"}
	b, err := json.Marshal(in)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"date":"2026-05-24"`)

	var out payload
	require.NoError(t, json.Unmarshal(b, &out))
	assert.Equal(t, in, out)
}

func TestDateOnlyFromTime(t *testing.T) {
	ts := time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, DateOnly("2026-05-24"), DateOnlyFromTime(ts))
}

func TestParseeDateOnly(t *testing.T) {
	d, err := ParseDateOnly("2026-05-24")
	require.NoError(t, err)
	assert.Equal(t, DateOnly("2026-05-24"), d)

	_, err = ParseDateOnly("not-a-date")
	assert.Error(t, err)
}
