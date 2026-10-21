import type { TimelineSegment } from '../api/types'
import { NOBODY } from '../components/oncall/ganttSegments'
import { zonedMidnightToISO } from './timezone'

function localDateStr(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

/**
 * Per-day segments for the create-schedule preview.
 *
 * Each calendar day shows who is on call when that day begins in the schedule's timezone, so
 * the preview agrees with the timezone the user picked rather than the browser's. Days before
 * the layer starts show nobody, as the backend does. Segments are laid inside the local day so
 * the calendar, which buckets by the browser's days, draws them on the right date.
 */
export function computeLayerSegments(
  layer: {
    shift_duration_seconds: number
    rotation_start: string
    participants?: Array<{ user_name: string; order_index: number }>
  },
  windowStart: Date,
  days: number,
  timezone: string,
): TimelineSegment[] {
  const participants = layer.participants ?? []
  if (participants.length === 0) return []

  const shiftMs = (layer.shift_duration_seconds || 86400) * 1000
  const rotationStart = new Date(layer.rotation_start).getTime()
  const sorted = [...participants].sort((a, b) => a.order_index - b.order_index)
  const segments: TimelineSegment[] = []

  for (let i = 0; i < days; i++) {
    const dayStart = new Date(windowStart)
    dayStart.setDate(dayStart.getDate() + i)
    dayStart.setHours(0, 0, 0, 0)
    const dayEnd = new Date(dayStart)
    dayEnd.setHours(23, 59, 59, 999)

    const dayBeginsAt = new Date(zonedMidnightToISO(localDateStr(dayStart), timezone)).getTime()
    const elapsed = dayBeginsAt - rotationStart
    const user =
      elapsed < 0 ? NOBODY : (sorted[Math.floor(elapsed / shiftMs) % sorted.length]?.user_name ?? NOBODY)

    segments.push({
      start: dayStart.toISOString(),
      end: dayEnd.toISOString(),
      user_name: user,
      is_override: false,
    })
  }

  return segments
}
