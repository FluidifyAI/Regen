/** API instants must include their timezone; never guess UTC or parse prose. */
export function parseAPITimestamp(value: unknown): Date | null {
  if (typeof value !== 'string') return null
  const match =
    /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/i.exec(value)
  if (!match) return null
  const y = Number(match[1])
  const m = Number(match[2])
  const d = Number(match[3])
  const h = Number(match[4])
  const min = Number(match[5])
  const sec = Number(match[6])
  // Date.parse normalizes some impossible calendar dates instead of rejecting them.
  const days = new Date(Date.UTC(y, m, 0)).getUTCDate()
  if (y < 100 || m < 1 || m > 12 || d < 1 || d > days || h > 23 || min > 59 || sec > 59) return null
  const date = new Date(value)
  return Number.isFinite(date.getTime()) ? date : null
}

export function formatAlertDate(
  value: unknown,
  fallback?: unknown,
  options?: Intl.DateTimeFormatOptions,
  locale?: string
): string {
  const date = parseAPITimestamp(value) ?? parseAPITimestamp(fallback)
  return date ? date.toLocaleString(locale, options) : '—'
}

/** Preserve the incident view's locale and browser timezone conventions. */
export function formatIncidentAlertDate(
  value: unknown,
  fallback?: unknown,
  timeZone?: string
): string {
  return formatAlertDate(
    value,
    fallback,
    {
      month: 'short',
      day: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
      hour12: true,
      timeZone,
    },
    'en-US'
  )
}
