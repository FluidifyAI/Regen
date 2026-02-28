import { ChevronLeft, ChevronRight } from 'lucide-react'
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
  /** Monday of the visible week */
  windowStart: Date
  /** Number of days to show (default 7) */
  days?: number
  /** Called when user clicks prev/next */
  onNavigate: (newStart: Date) => void
  /** Optional: clicking a row label navigates somewhere */
  onRowClick?: (id: string) => void
}

// ─── Exported helper ──────────────────────────────────────────────────────────

/** Returns the Monday of the week containing `date`. Uses local calendar date, not UTC. */
export function getMondayOf(date: Date): Date {
  const d = new Date(date)
  const day = d.getDay() // 0=Sun…6=Sat
  const diff = day === 0 ? -6 : 1 - day
  d.setDate(d.getDate() + diff)
  d.setHours(0, 0, 0, 0)
  return d
}

// ─── Private colour helpers ───────────────────────────────────────────────────

function userHue(name: string): number {
  let h = 5381
  for (let i = 0; i < name.length; i++) {
    h = ((h << 5) + h) ^ name.charCodeAt(i)
    h = h | 0
  }
  return Math.abs(h) % 360
}

function segmentBg(name: string): string {
  return `hsl(${userHue(name)}, 55%, 88%)`
}

function segmentText(name: string): string {
  return `hsl(${userHue(name)}, 45%, 30%)`
}

// ─── Private calendar helpers ─────────────────────────────────────────────────

function getSegmentForDay(segments: TimelineSegment[], day: Date): TimelineSegment | null {
  const dayStart = new Date(day)
  dayStart.setHours(0, 0, 0, 0)
  const dayEnd = new Date(day)
  dayEnd.setHours(23, 59, 59, 999)
  return (
    segments.find((s) => {
      const segStart = new Date(s.start)
      const segEnd = new Date(s.end)
      return segStart <= dayEnd && segEnd >= dayStart
    }) ?? null
  )
}

function isStreakStart(
  segments: TimelineSegment[],
  dayDates: Date[],
  dayIdx: number,
): boolean {
  if (dayIdx === 0) return true
  const curDay = dayDates[dayIdx]
  const prevDay = dayDates[dayIdx - 1]
  if (!curDay || !prevDay) return true
  const cur = getSegmentForDay(segments, curDay)
  const prev = getSegmentForDay(segments, prevDay)
  if (!cur) return false
  if (!prev) return true
  return cur.user_name !== prev.user_name
}

// ─── Static lookup tables ─────────────────────────────────────────────────────

const SHORT_DAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']

const MONTHS = [
  'January', 'February', 'March', 'April', 'May', 'June',
  'July', 'August', 'September', 'October', 'November', 'December',
]

// ─── GanttCalendar ────────────────────────────────────────────────────────────

export function GanttCalendar({
  rows,
  windowStart,
  days = 7,
  onNavigate,
  onRowClick,
}: GanttCalendarProps) {
  // Build array of day Date objects for the visible window (local time)
  const dayDates: Date[] = Array.from({ length: days }, (_, i) => {
    const d = new Date(windowStart)
    d.setDate(d.getDate() + i)
    return d
  })

  // Today reference (local time, midnight) for column highlighting
  const now = new Date()
  const todayMidnight = new Date(now)
  todayMidnight.setHours(0, 0, 0, 0)

  // Fraction through the current day for the "now" line position
  const nowFraction =
    (now.getHours() * 3600 + now.getMinutes() * 60 + now.getSeconds()) / 86400

  // Month label derived from the middle day of the window (days >= 1, always defined).
  const midDay = dayDates[Math.floor(days / 2)] ?? windowStart
  const monthLabel = `${MONTHS[midDay.getMonth()]} ${midDay.getFullYear()}`

  // Helpers to determine if a given Date object is today
  function isTodayDate(d: Date): boolean {
    const dm = new Date(d)
    dm.setHours(0, 0, 0, 0)
    return dm.getTime() === todayMidnight.getTime()
  }

  // Navigation
  function prevWindow() {
    const d = new Date(windowStart)
    d.setDate(d.getDate() - days)
    onNavigate(d)
  }

  function nextWindow() {
    const d = new Date(windowStart)
    d.setDate(d.getDate() + days)
    onNavigate(d)
  }

  function goToday() {
    onNavigate(getMondayOf(new Date()))
  }

  return (
    <div>
      {/* ── Navigation bar ── */}
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-1">
          <button
            type="button"
            onClick={prevWindow}
            className="p-1.5 rounded hover:bg-gray-100 transition-colors"
            title="Previous period"
          >
            <ChevronLeft className="w-4 h-4 text-text-secondary" />
          </button>
          <button
            type="button"
            onClick={nextWindow}
            className="p-1.5 rounded hover:bg-gray-100 transition-colors"
            title="Next period"
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

      {/* ── Scrollable table wrapper ── */}
      <div className="overflow-x-auto">
        <table
          className="w-full border-collapse table-fixed"
          style={{ minWidth: 600 }}
        >
          <colgroup>
            {/* Fixed 144px label column */}
            <col style={{ width: 144 }} />
            {/* Equal-width day columns for the rest */}
            {dayDates.map((_, i) => (
              <col key={i} />
            ))}
          </colgroup>

          {/* ── Day header row ── */}
          <thead>
            <tr>
              {/* Empty corner above label column */}
              <th className="p-0 border-b border-border bg-gray-50" />

              {dayDates.map((day, i) => {
                const today = isTodayDate(day)
                return (
                  <th
                    key={i}
                    className={`p-0 border-b border-r border-border bg-gray-50 ${
                      today ? 'text-brand-primary' : 'text-text-tertiary'
                    }`}
                  >
                    <div className="py-2 flex flex-col items-center gap-0.5">
                      <span className="text-xs font-medium">
                        {SHORT_DAYS[day.getDay()]}
                      </span>
                      <span
                        className={`text-xs w-5 h-5 flex items-center justify-center rounded-full leading-none ${
                          today
                            ? 'bg-brand-primary text-white font-semibold'
                            : ''
                        }`}
                      >
                        {day.getDate()}
                      </span>
                    </div>
                  </th>
                )
              })}
            </tr>
          </thead>

          {/* ── Data rows ── */}
          <tbody>
            {rows.length === 0 ? (
              <tr>
                <td
                  colSpan={days + 1}
                  className="px-4 py-8 text-center text-sm text-text-tertiary italic"
                >
                  No layers configured
                </td>
              </tr>
            ) : rows.map((row) => {
              const hasSegments = row.segments.length > 0

              return (
                <tr key={row.id}>
                  {/* Row label cell */}
                  <td
                    className={`h-12 border-b border-r border-border bg-white px-3 text-sm font-medium text-text-primary align-middle overflow-hidden ${
                      onRowClick
                        ? 'cursor-pointer hover:text-brand-primary hover:bg-gray-50 transition-colors'
                        : ''
                    }`}
                    style={{ maxWidth: 144 }}
                    onClick={onRowClick ? () => onRowClick(row.id) : undefined}
                    title={row.label}
                  >
                    <span className="block truncate">{row.label}</span>
                  </td>

                  {/* Empty state: single cell spanning all day columns */}
                  {!hasSegments && (
                    <td
                      colSpan={days}
                      className="h-12 border-b border-r border-border bg-white px-3 align-middle"
                    >
                      <span className="text-xs text-text-tertiary">—</span>
                    </td>
                  )}

                  {/* Day cells — only rendered when there are segments */}
                  {hasSegments &&
                    dayDates.map((day, dayIdx) => {
                      const today = isTodayDate(day)
                      const seg = getSegmentForDay(row.segments, day)
                      const showLabel = seg
                        ? isStreakStart(row.segments, dayDates, dayIdx)
                        : false

                      return (
                        <td
                          key={dayIdx}
                          className={`h-12 border-b border-r border-border relative p-0 ${
                            today ? 'bg-blue-50/30' : 'bg-white'
                          }`}
                        >
                          {/* Coloured segment bar */}
                          {seg && (
                            <div
                              className="absolute inset-1 rounded flex items-center px-1.5 overflow-hidden"
                              style={{
                                backgroundColor: segmentBg(seg.user_name),
                                color: segmentText(seg.user_name),
                              }}
                              title={
                                seg.is_override
                                  ? `${seg.user_name} (override)`
                                  : seg.user_name
                              }
                            >
                              {showLabel && (
                                <span className="text-xs font-medium truncate leading-none">
                                  {seg.user_name}
                                  {seg.is_override && (
                                    <span className="opacity-60 ml-1 text-[10px]">(override)</span>
                                  )}
                                </span>
                              )}
                            </div>
                          )}

                          {/* "Now" line — only rendered in today's column */}
                          {today && (
                            <div
                              className="absolute top-0 bottom-0 w-0.5 bg-red-400 z-10 pointer-events-none"
                              style={{ left: `${nowFraction * 100}%` }}
                            />
                          )}
                        </td>
                      )
                    })}
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
  )
}
