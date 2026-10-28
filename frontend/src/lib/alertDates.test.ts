import { describe, expect, it } from 'vitest'
import { formatAlertDate, formatIncidentAlertDate, parseAPITimestamp } from './alertDates'

describe('alert API timestamps', () => {
  it.each(['2026-10-28T05:26:41Z', '2026-10-28T07:26:41+02:00', '2026-10-28T00:26:41-05:00'])(
    'preserves the instant in %s',
    (value) => {
      expect(parseAPITimestamp(value)?.toISOString()).toBe('2026-10-28T05:26:41.000Z')
    }
  )
  it('accepts fractional seconds with UTC and offsets', () => {
    for (const value of ['2026-10-28T05:26:41.123456Z', '2026-10-28T06:26:41.123456+01:00']) {
      expect(parseAPITimestamp(value)?.toISOString()).toBe('2026-10-28T05:26:41.123Z')
    }
  })
  it.each([
    undefined,
    null,
    '',
    'invalid',
    0,
    '2026-10-28T05:26:41',
    '2026-02-30T00:00:00Z',
    '0001-01-01T00:00:00Z',
  ])('uses fallback and placeholder for %s', (value) => {
    expect(parseAPITimestamp(value)).toBeNull()
    expect(formatIncidentAlertDate(value, '2026-10-28T05:26:41Z', 'Europe/Amsterdam')).toBe(
      'Oct 28, 6:26 AM'
    )
    expect(formatIncidentAlertDate(value, value)).toBe('—')
    expect(formatAlertDate(value)).toBe('—')
  })
  it('uses timezone rules on both sides of the summer and winter transitions', () => {
    const format = (value: string) => formatIncidentAlertDate(value, undefined, 'Europe/Amsterdam')
    expect(format('2026-03-29T00:30:00Z')).toBe('Mar 29, 1:30 AM')
    expect(format('2026-03-29T01:30:00Z')).toBe('Mar 29, 3:30 AM')
    expect(format('2026-10-25T00:30:00Z')).toBe('Oct 25, 2:30 AM')
    expect(format('2026-10-25T01:30:00Z')).toBe('Oct 25, 2:30 AM')
  })
})
