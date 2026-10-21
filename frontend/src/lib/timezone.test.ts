import { describe, it, expect } from 'vitest'
import { zonedMidnightToISO, defaultScheduleTimezone, isValidTimezone, todayInTimezone } from './timezone'

describe('zonedMidnightToISO', () => {
  it('returns midnight UTC for UTC', () => {
    expect(zonedMidnightToISO('2026-10-19', 'UTC')).toBe('2026-10-19T00:00:00.000Z')
  })

  it('treats the date as midnight in the zone, not midnight UTC (Perth is UTC+8)', () => {
    expect(zonedMidnightToISO('2026-10-19', 'Australia/Perth')).toBe('2026-10-18T16:00:00.000Z')
  })

  it('handles zones behind UTC', () => {
    // New York is on EDT (UTC-4) on this date.
    expect(zonedMidnightToISO('2026-10-19', 'America/New_York')).toBe('2026-10-19T04:00:00.000Z')
  })

  it('handles half-hour offsets', () => {
    expect(zonedMidnightToISO('2026-10-19', 'Asia/Kolkata')).toBe('2026-10-18T18:30:00.000Z')
  })

  it('uses the offset in force on that date, across a DST change', () => {
    // Sydney moves to daylight time on 2026-10-04 at 02:00, so midnight that day is still AEST (+10).
    expect(zonedMidnightToISO('2026-10-04', 'Australia/Sydney')).toBe('2026-10-03T14:00:00.000Z')
    // The next day is AEDT (+11).
    expect(zonedMidnightToISO('2026-10-05', 'Australia/Sydney')).toBe('2026-10-04T13:00:00.000Z')
  })

  it('falls back to UTC for an unknown zone instead of throwing', () => {
    expect(zonedMidnightToISO('2026-10-19', 'Not/AZone')).toBe('2026-10-19T00:00:00.000Z')
  })
})

describe('isValidTimezone', () => {
  it('accepts IANA names and rejects junk', () => {
    expect(isValidTimezone('Australia/Perth')).toBe(true)
    expect(isValidTimezone('UTC')).toBe(true)
    expect(isValidTimezone('Not/AZone')).toBe(false)
    expect(isValidTimezone('')).toBe(false)
  })
})

describe('defaultScheduleTimezone', () => {
  it('prefers the instance timezone', () => {
    expect(defaultScheduleTimezone('Australia/Perth')).toBe('Australia/Perth')
  })

  it('falls back to the browser timezone when the instance has none', () => {
    const browser = Intl.DateTimeFormat().resolvedOptions().timeZone
    expect(defaultScheduleTimezone(undefined)).toBe(browser || 'UTC')
    expect(defaultScheduleTimezone('')).toBe(browser || 'UTC')
  })

  it('ignores an invalid instance timezone', () => {
    const browser = Intl.DateTimeFormat().resolvedOptions().timeZone
    expect(defaultScheduleTimezone('Not/AZone')).toBe(browser || 'UTC')
  })
})

describe('todayInTimezone', () => {
  it('is the date in that zone, which can differ from the UTC date', () => {
    const lateSundayUtc = new Date('2026-10-18T20:00:00Z')
    expect(todayInTimezone('UTC', lateSundayUtc)).toBe('2026-10-18')
    expect(todayInTimezone('Australia/Perth', lateSundayUtc)).toBe('2026-10-19')
    expect(todayInTimezone('America/Los_Angeles', lateSundayUtc)).toBe('2026-10-18')
  })

  it('falls back to UTC for an unknown zone', () => {
    expect(todayInTimezone('Not/AZone', new Date('2026-10-18T20:00:00Z'))).toBe('2026-10-18')
  })
})
