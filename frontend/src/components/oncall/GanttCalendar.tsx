import { getMonthStart } from './ganttCalendarHelpers'
import { ChevronLeft, ChevronRight, Globe, Trash2 } from 'lucide-react'
import type { TimelineSegment } from '../../api/types'
import { NOBODY, getSegmentForDay } from './ganttSegments'

// ─── Public types ─────────────────────────────────────────────────────────────

export interface GanttRow {
  id: string
  label: string
  /** Segments from getTimeline() API or computeLayerSegments() */
  segments: TimelineSegment[]
}

export interface HolidayMarker {
  date: string       // YYYY-MM-DD
  name: string
  countryCode: string
  /** Hidden by an admin; shown dimmed to admins only (the server omits it for everyone else). */
  hidden?: boolean
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
  /** Clicking a day header pre-fills override creation for that date */
  onDayClick?: (date: Date) => void
  /** Public holidays to mark on day headers */
  holidays?: HolidayMarker[]
  /** IANA timezone of the schedule — shown in the calendar footer */
  scheduleTimezone?: string
}

// ─── Private helpers ──────────────────────────────────────────────────────────

/** Strip the @domain part of an email for compact display in the bar. */
function displayName(name: string): string {
  if (!name || name === NOBODY) return ''
  const at = name.indexOf('@')
  return at > 0 ? name.slice(0, at) : name
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

function countryFlagEmoji(code: string): string {
  return code.toUpperCase().replace(/./g, c => String.fromCodePoint(c.charCodeAt(0) + 127397))
}

// ─── GanttCalendar ────────────────────────────────────────────────────────────

export function GanttCalendar({
  rows,
  windowStart,
  days = 7,
  onNavigate,
  onRowClick,
  onRowDelete,
  onDayClick,
  holidays = [],
  scheduleTimezone,
}: GanttCalendarProps) {
  const holidayMap = new Map<string, HolidayMarker>()
  for (const h of holidays) {
    holidayMap.set(h.date, h)
  }
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
                          }${onDayClick ? ' cursor-pointer hover:bg-brand-primary/10 transition-colors select-none' : ''}`}
                          onClick={onDayClick ? () => onDayClick(day) : undefined}
                          title={(() => {
                            const dateKey = `${day.getFullYear()}-${String(day.getMonth()+1).padStart(2,'0')}-${String(day.getDate()).padStart(2,'0')}`
                            const holiday = holidayMap.get(dateKey)
                            if (holiday) return `🎉 ${holiday.name}${holiday.hidden ? ' (hidden)' : ''}${onDayClick ? ' · Click to add override' : ''}`
                            return onDayClick ? `Add override for ${SHORT_DAYS[day.getDay()]} ${day.getDate()}` : undefined
                          })()}
                        >
                          {(() => {
                            const dateKey = `${day.getFullYear()}-${String(day.getMonth()+1).padStart(2,'0')}-${String(day.getDate()).padStart(2,'0')}`
                            const holiday = holidayMap.get(dateKey)
                            return (
                              <div className="py-1.5 flex flex-col items-center gap-0.5">
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
                                {holiday && (
                                  <div className="relative group/hday flex justify-center mt-0.5">
                                    <div className={`inline-flex items-center px-1.5 py-0.5 bg-amber-50 border border-amber-300 rounded-full cursor-default ${holiday.hidden ? 'opacity-40 border-dashed' : ''}`}>
                                      <span className="text-[11px] font-semibold text-amber-700 leading-none">{countryFlagEmoji(holiday.countryCode)}</span>
                                    </div>
                                    <div className="pointer-events-none absolute bottom-full left-1/2 -translate-x-1/2 mb-1.5 px-2.5 py-1.5 bg-gray-900 text-white text-xs rounded-lg whitespace-nowrap shadow-lg opacity-0 group-hover/hday:opacity-100 transition-opacity duration-150 z-50">
                                      {holiday.name}{holiday.hidden ? ' (hidden)' : ''}
                                      <div className="absolute top-full left-1/2 -translate-x-1/2 border-4 border-transparent border-t-gray-900" />
                                    </div>
                                  </div>
                                )}
                              </div>
                            )
                          })()}
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
                                  style={{
                                    backgroundColor: seg.is_override ? '#7c3aed' : '#475569',
                                    ...(seg.is_override && { borderTop: '3px solid #f97316' }),
                                  }}
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

      {/* ── Timezone footer ── */}
      {scheduleTimezone && (
        <div className="mt-2 flex items-center gap-1.5 text-xs text-text-tertiary">
          <Globe className="w-3 h-3 flex-shrink-0" />
          <span>
            Schedule timezone: <span className="font-medium text-text-secondary">{scheduleTimezone}</span>
            {' · '}
            {new Date().toLocaleString(undefined, {
              timeZone: scheduleTimezone,
              weekday: 'short',
              month: 'short',
              day: 'numeric',
              hour: '2-digit',
              minute: '2-digit',
              timeZoneName: 'short',
            })}
          </span>
        </div>
      )}
    </div>
  )
}
