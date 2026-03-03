import { ChevronLeft, ChevronRight, Trash2 } from 'lucide-react'
import type { TimelineSegment } from '../../api/types'

// ─── Public types ─────────────────────────────────────────────────────────────

export interface GanttRow {
  id: string
  label: string
  /** Segments from getTimeline() API or computeLayerSegments() */
  segments: TimelineSegment[]
}

interface GanttCalendarProps {
  rows: GanttRow[]
  /** First day of the visible window (1st of a month) */
  windowStart: Date
  /** Number of days to show (default 7) */
  days?: number
  /** Called when user clicks prev/next */
  onNavigate: (newStart: Date) => void
  /** Optional: clicking a row label navigates somewhere */
  onRowClick?: (id: string) => void
  /** Optional: delete button shown on hover in the row label */
  onRowDelete?: (id: string) => void
}

// ─── Exported helpers ─────────────────────────────────────────────────────────

/** Returns the Monday of the week containing `date`. Uses local calendar date, not UTC. */
export function getMondayOf(date: Date): Date {
  const d = new Date(date)
  const day = d.getDay()
  const diff = day === 0 ? -6 : 1 - day
  d.setDate(d.getDate() + diff)
  d.setHours(0, 0, 0, 0)
  return d
}

/** Returns midnight on the 1st of the month containing `date`. */
export function getMonthStart(date: Date): Date {
  const d = new Date(date)
  d.setDate(1)
  d.setHours(0, 0, 0, 0)
  return d
}

/** Returns the number of days in the month containing `date`. */
export function daysInMonth(date: Date): number {
  return new Date(date.getFullYear(), date.getMonth() + 1, 0).getDate()
}

// ─── Colour helpers (exported for use in detail page participant lists) ───────

// Curated hues that complement the navy/blue app UI.
// Covers blue, sky, teal, emerald, violet, indigo, amber, slate-blue —
// intentionally avoids pink, red, and tan/beige regions.
const PALETTE_HUES = [217, 196, 172, 145, 258, 230, 36, 186]

export function userHue(name: string): number {
  let h = 5381
  for (let i = 0; i < name.length; i++) {
    h = ((h << 5) + h) ^ name.charCodeAt(i)
    h = h | 0
  }
  const idx = Math.abs(h) % PALETTE_HUES.length
  return PALETTE_HUES[idx] as number
}

const NOBODY = '(nobody)'

export function segmentBg(name: string): string {
  if (!name || name === NOBODY) return '#e5e7eb' // gray-200 — neutral empty slot
  return `hsl(${userHue(name)}, 65%, 63%)`
}

export function segmentText(name: string): string {
  if (!name || name === NOBODY) return '#9ca3af' // gray-400
  return `hsl(${userHue(name)}, 75%, 18%)`
}

// ─── Private helpers ──────────────────────────────────────────────────────────

/** Strip the @domain part of an email for compact display in the bar. */
function displayName(name: string): string {
  if (!name || name === NOBODY) return ''
  const at = name.indexOf('@')
  return at > 0 ? name.slice(0, at) : name
}

function getSegmentForDay(segments: TimelineSegment[], day: Date): TimelineSegment | null {
  const dayStart = new Date(day)
  dayStart.setHours(0, 0, 0, 0)
  const dayEnd = new Date(day)
  dayEnd.setHours(23, 59, 59, 999)
  const matches = segments.filter((s) => {
    const segStart = new Date(s.start)
    const segEnd = new Date(s.end)
    return segStart <= dayEnd && segEnd >= dayStart
  })
  // Prefer override segments over regular rotation segments
  return matches.find((s) => s.is_override) ?? matches[0] ?? null
}

interface DayGroup {
  seg: TimelineSegment | null
  /** Number of consecutive days in this group */
  span: number
  /** Index of the first day of this group within the week (0-based) */
  weekOffset: number
}

/** Group consecutive same-user days in a week into spans for colSpan rendering. */
function groupWeekDays(segments: TimelineSegment[], weekDays: Date[]): DayGroup[] {
  const groups: DayGroup[] = []
  let i = 0
  while (i < weekDays.length) {
    const day = weekDays[i]
    if (!day) { i++; continue }
    const seg = getSegmentForDay(segments, day)
    if (!seg) {
      groups.push({ seg: null, span: 1, weekOffset: i })
      i++
    } else {
      let j = i + 1
      while (j < weekDays.length) {
        const nextDay = weekDays[j]
        if (!nextDay) break
        const nextSeg = getSegmentForDay(segments, nextDay)
        if (!nextSeg || nextSeg.user_name !== seg.user_name) break
        j++
      }
      groups.push({ seg, span: j - i, weekOffset: i })
      i = j
    }
  }
  return groups
}

// ─── Static lookup tables ─────────────────────────────────────────────────────

const SHORT_DAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']

const MONTHS = [
  'January', 'February', 'March', 'April', 'May', 'June',
  'July', 'August', 'September', 'October', 'November', 'December',
]

const WEEK_SIZE = 7

// ─── GanttCalendar ────────────────────────────────────────────────────────────

export function GanttCalendar({
  rows,
  windowStart,
  days = 7,
  onNavigate,
  onRowClick,
  onRowDelete,
}: GanttCalendarProps) {
  // Full day array, then split into 7-day week chunks
  const dayDates: Date[] = Array.from({ length: days }, (_, i) => {
    const d = new Date(windowStart)
    d.setDate(d.getDate() + i)
    return d
  })

  const weeks: Date[][] = []
  for (let i = 0; i < dayDates.length; i += WEEK_SIZE) {
    weeks.push(dayDates.slice(i, i + WEEK_SIZE))
  }

  // Today reference
  const now = new Date()
  const todayMidnight = new Date(now)
  todayMidnight.setHours(0, 0, 0, 0)
  const nowFraction = (now.getHours() * 3600 + now.getMinutes() * 60 + now.getSeconds()) / 86400

  const monthLabel = `${MONTHS[windowStart.getMonth()]} ${windowStart.getFullYear()}`

  function isTodayDate(d: Date): boolean {
    const dm = new Date(d)
    dm.setHours(0, 0, 0, 0)
    return dm.getTime() === todayMidnight.getTime()
  }

  function prevWindow() {
    const d = new Date(windowStart)
    d.setDate(1)
    d.setMonth(d.getMonth() - 1)
    onNavigate(d)
  }

  function nextWindow() {
    const d = new Date(windowStart)
    d.setDate(1)
    d.setMonth(d.getMonth() + 1)
    onNavigate(d)
  }

  function goToday() {
    onNavigate(getMonthStart(new Date()))
  }

  const labelCellBase = `h-14 border-b border-r border-border px-4 text-sm font-semibold text-text-primary align-middle overflow-hidden group`
  const labelCellHover = onRowClick ? ' cursor-pointer hover:text-brand-primary transition-colors' : ''

  return (
    <div>
      {/* ── Navigation bar ── */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-1">
          <button
            type="button"
            onClick={prevWindow}
            className="p-1.5 rounded hover:bg-gray-100 transition-colors"
            title="Previous month"
          >
            <ChevronLeft className="w-4 h-4 text-text-secondary" />
          </button>
          <button
            type="button"
            onClick={nextWindow}
            className="p-1.5 rounded hover:bg-gray-100 transition-colors"
            title="Next month"
          >
            <ChevronRight className="w-4 h-4 text-text-secondary" />
          </button>
          <span className="ml-2 text-sm font-semibold text-text-primary">{monthLabel}</span>
        </div>
        <button
          type="button"
          onClick={goToday}
          className="text-xs font-medium px-2.5 py-1 rounded border border-border text-text-secondary hover:bg-gray-50 transition-colors"
        >
          Today
        </button>
      </div>

      {/* ── Empty state ── */}
      {rows.length === 0 && (
        <div className="px-4 py-8 text-center text-sm text-text-tertiary italic border border-border rounded-lg">
          No layers configured
        </div>
      )}

      {/* ── Week blocks stacked vertically ── */}
      {rows.length > 0 && (
        <div className="space-y-3">
          {weeks.map((weekDays, weekIdx) => {
            // How many filler columns are needed to pad to WEEK_SIZE
            const fillerCount = WEEK_SIZE - weekDays.length

            return (
              <table
                key={weekIdx}
                className="w-full border-collapse table-fixed rounded-lg overflow-hidden border border-border shadow-sm"
              >
                {/* Always declare WEEK_SIZE + 1 cols so widths are uniform across all weeks */}
                <colgroup>
                  <col style={{ width: 192 }} />
                  {Array.from({ length: WEEK_SIZE }, (_, i) => <col key={i} />)}
                </colgroup>

                {/* Day header */}
                <thead>
                  <tr>
                    <th className="p-0 border-b-2 border-r border-border bg-gray-50" />
                    {weekDays.map((day, i) => {
                      const today = isTodayDate(day)
                      return (
                        <th
                          key={i}
                          className={`p-0 border-b-2 border-r border-border bg-gray-50 ${
                            today ? 'text-brand-primary' : 'text-text-tertiary'
                          }`}
                        >
                          <div className="py-2 flex flex-col items-center gap-0.5">
                            <span className="text-xs font-medium">
                              {SHORT_DAYS[day.getDay()]}
                            </span>
                            <span
                              className={`text-xs w-5 h-5 flex items-center justify-center rounded-full leading-none ${
                                today ? 'bg-brand-primary text-white font-semibold' : ''
                              }`}
                            >
                              {day.getDate()}
                            </span>
                          </div>
                        </th>
                      )
                    })}
                    {/* Filler header cells for partial weeks */}
                    {Array.from({ length: fillerCount }, (_, i) => (
                      <th key={`fh-${i}`} className="p-0 border-b-2 border-border bg-gray-50" />
                    ))}
                  </tr>
                </thead>

                {/* Data rows */}
                <tbody>
                  {rows.map((row, rowIdx) => {
                    const groups = groupWeekDays(row.segments, weekDays)
                    const rowBg = rowIdx % 2 === 0 ? 'bg-white' : 'bg-gray-50/60'

                    return (
                      <tr key={row.id}>
                        {/* Row label */}
                        <td
                          className={`${labelCellBase} ${rowBg}${labelCellHover}`}
                          style={{ maxWidth: 192 }}
                          onClick={onRowClick ? () => onRowClick(row.id) : undefined}
                          title={row.label}
                        >
                          <div className="flex items-center justify-between gap-1">
                            <span className="block truncate">{row.label}</span>
                            {onRowDelete && (
                              <button
                                type="button"
                                onClick={(e) => { e.stopPropagation(); onRowDelete(row.id) }}
                                className="flex-shrink-0 opacity-0 group-hover:opacity-100 p-1 rounded hover:bg-red-50 hover:text-red-500 text-text-tertiary transition-all"
                                title={`Delete ${row.label}`}
                              >
                                <Trash2 className="w-3.5 h-3.5" />
                              </button>
                            )}
                          </div>
                        </td>

                        {/* Continuous streak cells via colSpan */}
                        {groups.map((group, gi) => {
                          if (!group.seg) {
                            return (
                              <td
                                key={gi}
                                colSpan={group.span}
                                className={`h-14 border-b border-r border-border p-0 ${rowBg}`}
                              />
                            )
                          }

                          const { seg, span, weekOffset } = group

                          // Determine if today falls within this group and where
                          let todayOffsetInGroup = -1
                          for (let k = 0; k < span; k++) {
                            const d = weekDays[weekOffset + k]
                            if (d && isTodayDate(d)) { todayOffsetInGroup = k; break }
                          }
                          const containsToday = todayOffsetInGroup >= 0

                          // Position the now-line within the colSpanned cell
                          const nowLineLeft = containsToday
                            ? `${((todayOffsetInGroup + nowFraction) / span) * 100}%`
                            : null

                          return (
                            <td
                              key={gi}
                              colSpan={span}
                              className={`h-14 border-b border-r border-border relative p-0 ${rowBg}`}
                            >
                              {/* Coloured bar — full width, slight vertical inset */}
                              {seg.user_name && seg.user_name !== NOBODY ? (
                                <div
                                  className="absolute inset-y-1.5 left-0 right-0 flex items-center px-2.5 overflow-hidden rounded"
                                  style={{ backgroundColor: segmentBg(seg.user_name) }}
                                  title={seg.is_override ? `${seg.user_name} (override)` : seg.user_name}
                                >
                                  <span className="text-xs font-bold truncate leading-none text-white drop-shadow-sm">
                                    {displayName(seg.user_name)}
                                    {seg.is_override && (
                                      <span className="opacity-80 ml-1 text-[10px] font-medium">(override)</span>
                                    )}
                                  </span>
                                </div>
                              ) : (
                                /* Nobody slot — dashed outline, no text */
                                <div
                                  className="absolute inset-y-1.5 left-0 right-0 rounded border border-dashed border-gray-300"
                                  title="Nobody on call"
                                />
                              )}

                              {/* "Now" line, fractionally positioned within the colSpanned cell */}
                              {nowLineLeft && (
                                <div
                                  className="absolute top-0 bottom-0 w-0.5 bg-red-400 z-10 pointer-events-none"
                                  style={{ left: nowLineLeft }}
                                />
                              )}
                            </td>
                          )
                        })}

                        {/* Filler cells for partial weeks */}
                        {Array.from({ length: fillerCount }, (_, i) => (
                          <td key={`fd-${i}`} className="h-14 border-b bg-gray-50/40" />
                        ))}
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            )
          })}
        </div>
      )}
    </div>
  )
}
