import { useState, useRef, useEffect, useId } from 'react'

// Full IANA timezone list from the browser. Intl.supportedValuesOf is available
// in all modern browsers (Chrome 99+, Firefox 110+, Safari 15.4+).
// Falls back to a minimal set if the API is absent (very old engines only).
function getBrowserTimezones(): string[] {
  try {
    return (Intl as { supportedValuesOf?: (key: string) => string[] }).supportedValuesOf?.('timeZone') ?? FALLBACK_TIMEZONES
  } catch {
    return FALLBACK_TIMEZONES
  }
}

const FALLBACK_TIMEZONES = [
  'UTC',
  'America/New_York', 'America/Chicago', 'America/Denver', 'America/Los_Angeles',
  'America/Sao_Paulo', 'Europe/London', 'Europe/Paris', 'Europe/Berlin',
  'Asia/Kolkata', 'Asia/Dubai', 'Asia/Singapore', 'Asia/Tokyo', 'Asia/Seoul',
  'Asia/Shanghai', 'Australia/Sydney', 'Australia/Melbourne', 'Australia/Brisbane',
  'Australia/Perth', 'Australia/Adelaide', 'Australia/Darwin', 'Pacific/Auckland',
]

const ALL_TIMEZONES = getBrowserTimezones()

interface TimezoneSelectProps {
  value: string
  onChange: (tz: string) => void
  id?: string
  className?: string
  disabled?: boolean
}

export function TimezoneSelect({ value, onChange, id, className, disabled }: TimezoneSelectProps) {
  const inputId = useId()
  const resolvedId = id ?? inputId

  const [query, setQuery] = useState('')
  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)
  const listRef = useRef<HTMLUListElement>(null)

  const filtered = query.trim() === ''
    ? ALL_TIMEZONES
    : ALL_TIMEZONES.filter((tz) =>
        tz.toLowerCase().includes(query.toLowerCase().replace(/\s+/g, '_'))
      )

  // Close on outside click
  useEffect(() => {
    function handleMouseDown(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false)
        setQuery('')
      }
    }
    document.addEventListener('mousedown', handleMouseDown)
    return () => document.removeEventListener('mousedown', handleMouseDown)
  }, [])

  // Scroll selected item into view when opening
  useEffect(() => {
    if (open && listRef.current) {
      const selected = listRef.current.querySelector('[aria-selected="true"]')
      selected?.scrollIntoView({ block: 'nearest' })
    }
  }, [open])

  function handleSelect(tz: string) {
    onChange(tz)
    setOpen(false)
    setQuery('')
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Escape') { setOpen(false); setQuery('') }
    if (e.key === 'Enter' && filtered.length > 0 && filtered[0]) { handleSelect(filtered[0]); e.preventDefault() }
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      if (!open) { setOpen(true); return }
      const items = listRef.current?.querySelectorAll('[role="option"]')
      if (items && items.length > 0) (items[0] as HTMLElement).focus()
    }
  }

  function handleOptionKeyDown(e: React.KeyboardEvent<HTMLLIElement>, tz: string, idx: number) {
    if (e.key === 'Enter' || e.key === ' ') { handleSelect(tz); e.preventDefault() }
    if (e.key === 'Escape') { setOpen(false); setQuery('') }
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      const items = listRef.current?.querySelectorAll('[role="option"]')
      if (items) (items[Math.min(idx + 1, items.length - 1)] as HTMLElement).focus()
    }
    if (e.key === 'ArrowUp') {
      e.preventDefault()
      if (idx === 0) { (containerRef.current?.querySelector('input') as HTMLElement)?.focus(); return }
      const items = listRef.current?.querySelectorAll('[role="option"]')
      if (items) (items[Math.max(idx - 1, 0)] as HTMLElement).focus()
    }
  }

  const baseClass = 'w-full px-3 py-2 border border-border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary focus:border-transparent disabled:opacity-50'

  return (
    <div ref={containerRef} className="relative" role="combobox" aria-expanded={open} aria-haspopup="listbox">
      <input
        id={resolvedId}
        type="text"
        autoComplete="off"
        disabled={disabled}
        value={open ? query : value}
        placeholder={open ? 'Search timezones…' : undefined}
        className={className ?? baseClass}
        onFocus={() => setOpen(true)}
        onChange={(e) => { setQuery(e.target.value); setOpen(true) }}
        onKeyDown={handleKeyDown}
        aria-autocomplete="list"
        aria-controls={`${resolvedId}-list`}
      />

      {open && (
        <ul
          ref={listRef}
          id={`${resolvedId}-list`}
          role="listbox"
          className="absolute z-50 mt-1 w-full max-h-60 overflow-y-auto rounded-lg border border-border bg-surface-primary shadow-lg text-sm"
        >
          {filtered.length === 0 ? (
            <li className="px-3 py-2 text-text-secondary">No timezones match "{query}"</li>
          ) : (
            filtered.map((tz, idx) => (
              <li
                key={tz}
                role="option"
                aria-selected={tz === value}
                tabIndex={-1}
                className={`px-3 py-2 cursor-pointer outline-none focus:bg-surface-secondary hover:bg-surface-secondary ${tz === value ? 'font-medium text-brand-primary' : 'text-text-primary'}`}
                onMouseDown={(e) => { e.preventDefault(); handleSelect(tz) }}
                onKeyDown={(e) => handleOptionKeyDown(e, tz, idx)}
              >
                {tz}
              </li>
            ))
          )}
        </ul>
      )}
    </div>
  )
}
