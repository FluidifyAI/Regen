package models

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// DateOnly is a date-only value stored and transmitted as "YYYY-MM-DD".
// Using this instead of time.Time prevents accidental timezone shifts when a
// time.Time (midnight UTC) is converted to local time in the browser or on a
// server not running UTC.
type DateOnly string

// Scan implements sql.Scanner so GORM can populate DateOnly from a DATE column.
// pgx delivers DATE values as time.Time (midnight UTC); we extract the UTC date string.
func (d *DateOnly) Scan(value interface{}) error {
	switch v := value.(type) {
	case time.Time:
		*d = DateOnly(v.UTC().Format("2006-01-02"))
	case string:
		if _, err := time.Parse("2006-01-02", v); err != nil {
			return fmt.Errorf("DateOnly: invalid date string %q", v)
		}
		*d = DateOnly(v)
	case []byte:
		s := string(v)
		if _, err := time.Parse("2006-01-02", s); err != nil {
			return fmt.Errorf("DateOnly: invalid date bytes %q", s)
		}
		*d = DateOnly(s)
	case nil:
		*d = ""
	default:
		return fmt.Errorf("DateOnly: cannot scan %T", value)
	}
	return nil
}

// Value implements driver.Valuer so GORM can write DateOnly back to a DATE column.
func (d DateOnly) Value() (driver.Value, error) {
	if d == "" {
		return nil, nil
	}
	return string(d), nil
}

// String returns the date as "YYYY-MM-DD".
func (d DateOnly) String() string { return string(d) }

// ParseDateOnly parses a "YYYY-MM-DD" string into a DateOnly.
func ParseDateOnly(s string) (DateOnly, error) {
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return "", fmt.Errorf("DateOnly: %w", err)
	}
	return DateOnly(s), nil
}

// DateOnlyFromTime converts a time.Time to DateOnly using its UTC date.
func DateOnlyFromTime(t time.Time) DateOnly {
	return DateOnly(t.UTC().Format("2006-01-02"))
}
