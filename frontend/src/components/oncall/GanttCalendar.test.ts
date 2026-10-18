import { describe, it, expect } from 'vitest'
import { getSegmentForDay } from './ganttSegments'
import type { TimelineSegment } from '../../api/types'

// Local-time dates keep these tests independent of the machine's timezone,
// matching how the calendar buckets days.
function at(day: number, hour = 0): Date {
  return new Date(2026, 9, day, hour, 0, 0, 0)
}

function seg(user: string, start: Date, end: Date, isOverride = false): TimelineSegment {
  return { start: start.toISOString(), end: end.toISOString(), user_name: user, is_override: isOverride }
}

describe('getSegmentForDay', () => {
  it('shows the rotation start day as the real user, not the (nobody) gap before it', () => {
    // Layer starts at 09:00 on the 5th; the backend emits (nobody) for 00:00-09:00.
    const segments = [
      seg('(nobody)', at(1), at(5, 9)),
      seg('alice', at(5, 9), at(12, 9)),
    ]
    expect(getSegmentForDay(segments, at(5))?.user_name).toBe('alice')
  })

  it('still shows (nobody) for days before the rotation starts', () => {
    const segments = [
      seg('(nobody)', at(1), at(5, 9)),
      seg('alice', at(5, 9), at(12, 9)),
    ]
    expect(getSegmentForDay(segments, at(4))?.user_name).toBe('(nobody)')
  })

  it('attributes a mid-day handoff to the person on call for most of the day', () => {
    const segments = [
      seg('alice', at(1), at(5, 18)),
      seg('bob', at(5, 18), at(12)),
    ]
    expect(getSegmentForDay(segments, at(5))?.user_name).toBe('alice')
    expect(getSegmentForDay(segments, at(6))?.user_name).toBe('bob')
  })

  it('prefers an override over the rotation on the same day', () => {
    const segments = [
      seg('alice', at(1), at(12)),
      seg('carol', at(5, 9), at(5, 11), true),
    ]
    expect(getSegmentForDay(segments, at(5))?.user_name).toBe('carol')
  })

  it('returns null when no segment overlaps the day', () => {
    expect(getSegmentForDay([seg('alice', at(1), at(3))], at(10))).toBeNull()
  })
})
