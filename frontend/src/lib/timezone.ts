/** True when `tz` is an IANA timezone name the browser understands. */
export function isValidTimezone(tz: string | undefined | null): tz is string {
  if (!tz) return false
  try {
    new Intl.DateTimeFormat('en-US', { timeZone: tz })
    return true
  } catch {
    return false
  }
}

/**
 * Timezone a new schedule should start with: the instance setting, then the browser's
 * timezone, then UTC.
 */
export function defaultScheduleTimezone(instanceTz?: string | null): string {
  if (isValidTimezone(instanceTz)) return instanceTz
  const browser = Intl.DateTimeFormat().resolvedOptions().timeZone
  return isValidTimezone(browser) ? browser : 'UTC'
}

/** Offset (ms) of `tz` from UTC at the given instant: wall-clock time in tz minus the UTC time. */
function offsetMs(instantMs: number, tz: string): number {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: tz,
    hourCycle: 'h23',
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit',
  }).formatToParts(new Date(instantMs))
  const get = (type: string) => Number(parts.find((p) => p.type === type)?.value)
  const wallAsUtc = Date.UTC(get('year'), get('month') - 1, get('day'), get('hour'), get('minute'), get('second'))
  return wallAsUtc - Math.floor(instantMs / 1000) * 1000
}

/**
 * The instant (ISO string) at which the calendar date `dateStr` (YYYY-MM-DD) begins in `tz`.
 *
 * A date typed into a form has no timezone of its own. Appending `T00:00:00Z` makes it
 * midnight UTC, which is the wrong moment for every schedule outside UTC. Falls back to
 * midnight UTC when `tz` is not a valid zone.
 */
export function zonedMidnightToISO(dateStr: string, tz: string): string {
  const [y, m, d] = dateStr.split('-').map(Number) as [number, number, number]
  const utcMidnight = Date.UTC(y, m - 1, d)
  if (!isValidTimezone(tz)) return new Date(utcMidnight).toISOString()

  // The offset depends on the instant, so refine once: guess with the offset at UTC midnight,
  // then re-check at the guess. This lands on the correct side of a DST change.
  const first = utcMidnight - offsetMs(utcMidnight, tz)
  const second = utcMidnight - offsetMs(first, tz)
  return new Date(second).toISOString()
}

/** Today's calendar date (YYYY-MM-DD) in `tz`, which can differ from the UTC or browser date. */
export function todayInTimezone(tz: string, now: Date = new Date()): string {
  const zone = isValidTimezone(tz) ? tz : 'UTC'
  // The en-CA locale formats dates as YYYY-MM-DD.
  return new Intl.DateTimeFormat('en-CA', { timeZone: zone }).format(now)
}
