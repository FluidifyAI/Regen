import { useState } from 'react'
import { createHoliday, updateHoliday, deleteHoliday } from '../../api/schedules'
import type { ScheduleHoliday } from '../../api/types'
import { Button } from '../ui/Button'

/** country_code the server gives holidays an admin added by hand. */
const CUSTOM = 'custom'

interface HolidaysPanelProps {
  scheduleId: string
  /** Holidays in the visible month, including hidden ones (the server sends those to admins only). */
  holidays: ScheduleHoliday[]
  /** Called after any change so the page can refetch. */
  onChanged: () => void
  onError: (message: string) => void
  onSuccess: (message: string) => void
}

const inputClass =
  'px-3 py-1.5 border border-border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary focus:border-transparent'

function countryFlag(code: string): string {
  if (code === CUSTOM) return '📌'
  return code.toUpperCase().replace(/./g, (c) => String.fromCodePoint(c.charCodeAt(0) + 127397))
}

/**
 * Admin controls for a schedule's holidays: hide or restore imported holidays, rename them,
 * and add your own. The daily country-feed sync never undoes these changes.
 */
export function HolidaysPanel({ scheduleId, holidays, onChanged, onError, onSuccess }: HolidaysPanelProps) {
  const [newDate, setNewDate] = useState('')
  const [newName, setNewName] = useState('')
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editName, setEditName] = useState('')
  const [busy, setBusy] = useState(false)

  async function run(action: () => Promise<unknown>, success: string) {
    setBusy(true)
    try {
      await action()
      onSuccess(success)
      onChanged()
    } catch (err) {
      onError(err instanceof Error ? err.message : 'Holiday update failed')
    } finally {
      setBusy(false)
    }
  }

  async function handleAdd() {
    if (!newDate || !newName.trim()) return
    await run(() => createHoliday(scheduleId, { date: newDate, name: newName.trim() }), 'Holiday added')
    setNewDate('')
    setNewName('')
  }

  async function handleRename(h: ScheduleHoliday) {
    const name = editName.trim()
    if (!name || name === h.name) {
      setEditingId(null)
      return
    }
    await run(() => updateHoliday(scheduleId, h.id, { name }), 'Holiday renamed')
    setEditingId(null)
  }

  return (
    <div className="border border-border rounded-lg bg-white p-4">
      <h3 className="text-sm font-semibold text-text-primary">Holidays this month</h3>
      <p className="text-xs text-text-tertiary mb-3">
        Hide holidays that do not apply to this team, rename them, or add your own. Country feeds keep
        updating, but your changes are kept.
      </p>

      {holidays.length === 0 ? (
        <p className="text-xs text-text-tertiary mb-3">No holidays in this month.</p>
      ) : (
        <ul className="divide-y divide-border mb-3">
          {holidays.map((h) => (
            <li key={h.id} className={`flex items-center gap-2 py-2 ${h.hidden ? 'opacity-50' : ''}`}>
              <span className="text-base leading-none" title={h.country_code === CUSTOM ? 'Added by an admin' : h.country_code}>
                {countryFlag(h.country_code)}
              </span>
              <span className="text-xs text-text-tertiary w-24 shrink-0">{h.date}</span>

              {editingId === h.id ? (
                <input
                  autoFocus
                  value={editName}
                  maxLength={255}
                  onChange={(e) => setEditName(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') void handleRename(h)
                    if (e.key === 'Escape') setEditingId(null)
                  }}
                  className={`${inputClass} flex-1 min-w-0`}
                  aria-label={`New name for ${h.name}`}
                />
              ) : (
                <span className={`text-sm text-text-primary flex-1 min-w-0 truncate ${h.hidden ? 'line-through' : ''}`}>
                  {h.name}
                  {h.source === 'manual' && h.country_code !== CUSTOM && (
                    <span className="ml-2 text-[10px] uppercase text-text-tertiary">edited</span>
                  )}
                  {h.hidden && <span className="ml-2 text-[10px] uppercase text-text-tertiary">hidden</span>}
                </span>
              )}

              <div className="flex items-center gap-1 shrink-0">
                {editingId === h.id ? (
                  <>
                    <Button size="sm" variant="secondary" disabled={busy} onClick={() => void handleRename(h)}>Save</Button>
                    <Button size="sm" variant="ghost" disabled={busy} onClick={() => setEditingId(null)}>Cancel</Button>
                  </>
                ) : (
                  <>
                    {!h.hidden && (
                      <Button size="sm" variant="ghost" disabled={busy} onClick={() => { setEditingId(h.id); setEditName(h.name) }}>
                        Rename
                      </Button>
                    )}
                    {h.hidden ? (
                      <Button size="sm" variant="ghost" disabled={busy}
                        onClick={() => void run(() => updateHoliday(scheduleId, h.id, { hidden: false }), 'Holiday restored')}>
                        Restore
                      </Button>
                    ) : (
                      <Button size="sm" variant="ghost" disabled={busy}
                        onClick={() => void run(() => deleteHoliday(scheduleId, h.id),
                          h.country_code === CUSTOM ? 'Holiday removed' : 'Holiday hidden')}>
                        {h.country_code === CUSTOM ? 'Remove' : 'Hide'}
                      </Button>
                    )}
                  </>
                )}
              </div>
            </li>
          ))}
        </ul>
      )}

      <form
        className="flex flex-wrap items-center gap-2"
        onSubmit={(e) => { e.preventDefault(); void handleAdd() }}
      >
        <input type="date" value={newDate} onChange={(e) => setNewDate(e.target.value)} className={inputClass} aria-label="Holiday date" />
        <input
          value={newName}
          maxLength={255}
          onChange={(e) => setNewName(e.target.value)}
          placeholder="Holiday name"
          className={`${inputClass} flex-1 min-w-[10rem]`}
          aria-label="Holiday name"
        />
        <Button size="sm" type="submit" disabled={busy || !newDate || !newName.trim()}>Add holiday</Button>
      </form>
    </div>
  )
}
