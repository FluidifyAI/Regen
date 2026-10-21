import { describe, it, expect } from 'vitest'
import { computeLayerSegments } from './schedulePreview'
import { zonedMidnightToISO } from './timezone'

const WEEK = 604800
const DAY = 86400
const people = [
  { user_name: 'alice', order_index: 0 },
  { user_name: 'bob', order_index: 1 },
]

// Calendar dates are local Date objects, as the calendar component uses them.
const monday = new Date(2026, 9, 19) // Mon 19 Oct 2026

function names(segs: { user_name: string }[]): string[] {
  return segs.map((s) => s.user_name)
}

describe('computeLayerSegments', () => {
  it('starts on the chosen date when the layer starts at midnight in the schedule timezone', () => {
    // The #184 case: a Perth (UTC+8) schedule that starts on Monday used to show the
    // previous person on Monday because the start was taken as midnight UTC.
    const layer = {
      shift_duration_seconds: WEEK,
      rotation_start: zonedMidnightToISO('2026-10-19', 'Australia/Perth'),
      participants: people,
    }
    const segs = computeLayerSegments(layer, monday, 14, 'Australia/Perth')
    expect(names(segs)).toEqual([...Array(7).fill('alice'), ...Array(7).fill('bob')])
  })

  it('gives the same answer for a schedule far from UTC on the other side', () => {
    const layer = {
      shift_duration_seconds: WEEK,
      rotation_start: zonedMidnightToISO('2026-10-19', 'America/Los_Angeles'),
      participants: people,
    }
    expect(names(computeLayerSegments(layer, monday, 8, 'America/Los_Angeles')))
      .toEqual([...Array(7).fill('alice'), 'bob'])
  })

  it('shows nobody before the layer starts, like the backend does', () => {
    const layer = {
      shift_duration_seconds: DAY,
      rotation_start: zonedMidnightToISO('2026-10-21', 'UTC'),
      participants: people,
    }
    expect(names(computeLayerSegments(layer, monday, 4, 'UTC')))
      .toEqual(['(nobody)', '(nobody)', 'alice', 'bob'])
  })

  it('rotates daily shifts through the participants', () => {
    const layer = {
      shift_duration_seconds: DAY,
      rotation_start: zonedMidnightToISO('2026-10-19', 'Asia/Kolkata'),
      participants: people,
    }
    expect(names(computeLayerSegments(layer, monday, 4, 'Asia/Kolkata')))
      .toEqual(['alice', 'bob', 'alice', 'bob'])
  })

  it('returns nothing without participants', () => {
    expect(computeLayerSegments(
      { shift_duration_seconds: WEEK, rotation_start: '2026-10-19T00:00:00.000Z', participants: [] },
      monday, 7, 'UTC',
    )).toEqual([])
  })

  it('emits one segment per calendar day, each inside its own local day', () => {
    const layer = { shift_duration_seconds: WEEK, rotation_start: zonedMidnightToISO('2026-10-19', 'UTC'), participants: people }
    const segs = computeLayerSegments(layer, monday, 3, 'UTC')
    expect(segs).toHaveLength(3)
    segs.forEach((s, i) => {
      const start = new Date(s.start)
      expect(start.getDate()).toBe(19 + i)
      expect(start.getHours()).toBe(0)
      expect(new Date(s.end).getDate()).toBe(19 + i)
    })
  })
})
