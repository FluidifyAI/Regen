import { NOBODY } from './ganttSegments'

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

export function segmentBg(name: string): string {
  if (!name || name === NOBODY) return '#e5e7eb' // gray-200 — neutral empty slot
  return `hsl(${userHue(name)}, 65%, 63%)`
}

export function segmentText(name: string): string {
  if (!name || name === NOBODY) return '#9ca3af' // gray-400
  return `hsl(${userHue(name)}, 75%, 18%)`
}
