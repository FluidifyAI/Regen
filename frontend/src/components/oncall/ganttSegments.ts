import type { TimelineSegment } from '../../api/types'

/** Name the backend gives a segment when nobody is on call. */
export const NOBODY = '(nobody)'

export function getSegmentForDay(segments: TimelineSegment[], day: Date): TimelineSegment | null {
  const dayStart = new Date(day)
  dayStart.setHours(0, 0, 0, 0)
  const dayEnd = new Date(day)
  dayEnd.setHours(23, 59, 59, 999)
  const overlapMs = (s: TimelineSegment) =>
    Math.min(new Date(s.end).getTime(), dayEnd.getTime()) -
    Math.max(new Date(s.start).getTime(), dayStart.getTime())
  const matches = segments.filter((s) => {
    const segStart = new Date(s.start)
    const segEnd = new Date(s.end)
    return segStart < dayEnd && segEnd > dayStart
  })
  // Prefer an override, then a real person over the (nobody) gap before a layer
  // starts, then whoever covers most of the day. Without this the first segment
  // of the day wins, so a layer starting mid-day shows up a day late.
  const rank = (s: TimelineSegment) => (s.is_override ? 2 : 0) + (s.user_name !== NOBODY ? 1 : 0)
  return matches.reduce<TimelineSegment | null>((best, s) => {
    if (!best) return s
    return rank(s) > rank(best) || (rank(s) === rank(best) && overlapMs(s) > overlapMs(best)) ? s : best
  }, null)
}
