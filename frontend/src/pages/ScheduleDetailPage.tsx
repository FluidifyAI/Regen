import { useState, useEffect, useCallback, useRef, useMemo } from 'react'
import { useNavigate, useParams, Link } from 'react-router-dom'
import {
  ChevronRight,
  Plus,
  Trash2,
  Pencil,
  User,
  AlertCircle,
  Layers,
  Clock,
  Globe,
  Sunrise,
} from 'lucide-react'
import { GanttCalendar, GanttRow, getMonthStart, daysInMonth, segmentBg, segmentText } from '../components/oncall/GanttCalendar'
import { Button } from '../components/ui/Button'
import { SkeletonDetail } from '../components/ui/Skeleton'
import { GeneralError } from '../components/ui/ErrorState'
import { ToastContainer, useToast } from '../components/ui/Toast'
import { useSchedule } from '../hooks/useSchedule'
import {
  updateSchedule,
  deleteSchedule,
  createLayer,
  updateLayer,
  deleteLayer,
  createOverride,
  deleteOverride,
  createUnavailability,
  deleteUnavailability,
  getLayerTimelines,
  getHolidays,
  COMMON_TIMEZONES,
  SUPPORTED_HOLIDAY_COUNTRIES,
} from '../api/schedules'
import { listUsers } from '../api/users'
import type { UserSummary } from '../api/users'
import { listEscalationPolicies } from '../api/escalation'
import type {
  Schedule,
  ScheduleLayer,
  ScheduleOverride,
  ScheduleUnavailability,
  ScheduleHoliday,
  TimelineSegment,
  UpdateScheduleRequest,
  CreateLayerRequest,
  UpdateLayerRequest,
  LayerTimelinesResponse,
  CreateOverrideRequest,
  EscalationPolicy,
} from '../api/types'

// ─── Shared helpers ───────────────────────────────────────────────────────────

function formatDuration(startIso: string, endIso: string): string {
  const ms = new Date(endIso).getTime() - new Date(startIso).getTime()
  const totalMinutes = Math.round(ms / 60000)
  if (totalMinutes < 60) return `${totalMinutes}m`
  const h = Math.floor(totalMinutes / 60)
  const m = totalMinutes % 60
  if (h < 24) return m > 0 ? `${h}h ${m}m` : `${h}h`
  const d = Math.floor(h / 24)
  const rh = h % 24
  return rh > 0 ? `${d}d ${rh}h` : `${d}d`
}

function overrideStatus(ov: ScheduleOverride): 'past' | 'active' | 'upcoming' {
  const now = Date.now()
  if (new Date(ov.end_time).getTime() < now) return 'past'
  if (new Date(ov.start_time).getTime() <= now) return 'active'
  return 'upcoming'
}

function timeUntil(iso: string): string {
  const ms = new Date(iso).getTime() - Date.now()
  if (ms <= 0) return 'now'
  const totalMinutes = Math.floor(ms / 60000)
  if (totalMinutes < 60) return `${totalMinutes}m`
  const h = Math.floor(totalMinutes / 60)
  if (h >= 24) return `${Math.floor(h / 24)}d`
  const m = totalMinutes % 60
  return m > 0 ? `${h}h ${m}m` : `${h}h`
}

function localDateStr(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

function localDateTimeStr(d: Date): string {
  return `${localDateStr(d)}T${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

/** Current time formatted in the schedule's timezone, e.g. "Thu, Jun 12, 2:30 PM IST" */
function nowInScheduleTz(scheduleTz: string): string {
  try {
    return new Intl.DateTimeFormat('en-US', {
      timeZone: scheduleTz,
      weekday: 'short', month: 'short', day: 'numeric',
      hour: 'numeric', minute: '2-digit', hour12: true,
      timeZoneName: 'short',
    }).format(new Date())
  } catch {
    return ''
  }
}

/**
 * Returns the next midnight in the schedule's timezone, expressed as a
 * local datetime string suitable for a datetime-local input.
 *
 * Computed by reading the current hour:minute in the schedule tz (via
 * formatToParts), subtracting that from now to find the last midnight in
 * that tz, then adding 24 h. Works across DST boundaries because we
 * derive the offset from the live formatter, not a fixed constant.
 */
function nextScheduleMidnightAsLocal(scheduleTz: string): string {
  const now = new Date()
  try {
    const parts = new Intl.DateTimeFormat('en-US', {
      timeZone: scheduleTz, hour: '2-digit', minute: '2-digit', hour12: false,
    }).formatToParts(now)
    const h = parseInt(parts.find(p => p.type === 'hour')?.value ?? '0', 10) % 24
    const m = parseInt(parts.find(p => p.type === 'minute')?.value ?? '0', 10)
    const msSinceSchedMidnight = (h * 60 + m) * 60_000
    const nextMidnight = new Date(now.getTime() - msSinceSchedMidnight + 86_400_000)
    return localDateTimeStr(nextMidnight)
  } catch {
    const d = new Date()
    d.setHours(0, 0, 0, 0)
    return localDateTimeStr(new Date(d.getTime() + 86_400_000))
  }
}

/** Pill shown at the top of every schedule form modal. */
function TzBadge({ timezone }: { timezone: string }) {
  if (!timezone) return null
  const now = nowInScheduleTz(timezone)
  const label = timezone.replace(/_/g, ' ')
  return (
    <div className="flex items-start gap-2 px-3 py-2.5 bg-indigo-50 border border-indigo-200 rounded-lg text-xs">
      <Globe className="w-3.5 h-3.5 text-indigo-500 mt-0.5 flex-shrink-0" />
      <div>
        <span className="font-semibold text-indigo-800">Schedule timezone: {label}</span>
        {now && (
          <span className="text-indigo-600 ml-2">· Now: {now}</span>
        )}
      </div>
    </div>
  )
}

function toScheduleTz(datetimeLocalValue: string, scheduleTz: string): string {
  const d = new Date(datetimeLocalValue)
  if (isNaN(d.getTime())) return ''
  try {
    return new Intl.DateTimeFormat('en-US', {
      timeZone: scheduleTz,
      weekday: 'short', month: 'short', day: 'numeric',
      hour: 'numeric', minute: '2-digit', timeZoneName: 'short',
    }).format(d)
  } catch {
    return ''
  }
}

function countryFlag(code: string): string {
  return code.toUpperCase().replace(/./g, c =>
    String.fromCodePoint(c.charCodeAt(0) + 127397)
  )
}

// ─── Edit schedule modal ──────────────────────────────────────────────────────

interface EditScheduleModalProps {
  isOpen: boolean
  schedule: Schedule
  onClose: () => void
  onSaved: () => void
}

function EditScheduleModal({ isOpen, schedule, onClose, onSaved }: EditScheduleModalProps) {
  const [name, setName] = useState(schedule.name)
  const [description, setDescription] = useState(schedule.description)
  const [timezone, setTimezone] = useState(schedule.timezone)
  const [defaultPolicyId, setDefaultPolicyId] = useState(schedule.default_escalation_policy_id ?? '')
  const [holidayCountries, setHolidayCountries] = useState<string[]>(schedule.holiday_countries ?? [])
  const [policies, setPolicies] = useState<EscalationPolicy[]>([])
  const [error, setError] = useState<string | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const nameRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    listEscalationPolicies().then(r => setPolicies(r.data)).catch(() => {})
  }, [])

  useEffect(() => {
    if (isOpen) {
      setName(schedule.name)
      setDescription(schedule.description)
      setTimezone(schedule.timezone)
      setDefaultPolicyId(schedule.default_escalation_policy_id ?? '')
      setHolidayCountries(schedule.holiday_countries ?? [])
      setError(null)
      setTimeout(() => nameRef.current?.focus(), 50)
    }
  }, [isOpen, schedule])

  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    if (isOpen) document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [isOpen, onClose])

  if (!isOpen) return null

  const toggleCountry = (code: string) => {
    setHolidayCountries(prev =>
      prev.includes(code) ? prev.filter(c => c !== code) : [...prev, code]
    )
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setIsSubmitting(true)
    try {
      const body: UpdateScheduleRequest = {
        name,
        timezone,
        description,
        default_escalation_policy_id: defaultPolicyId || null,
        holiday_countries: holidayCountries,
      }
      await updateSchedule(schedule.id, body)
      onSaved()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update schedule')
    } finally {
      setIsSubmitting(false)
    }
  }

  const inputClass = 'w-full px-3 py-2 border border-border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary focus:border-transparent disabled:opacity-50'
  const labelClass = 'block text-sm font-medium text-text-primary mb-1'

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />
      <div
        className="relative z-10 w-full max-w-md bg-surface-primary rounded-xl shadow-xl mx-4 flex flex-col max-h-[90vh]"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="px-6 py-4 border-b border-border">
          <h2 className="text-lg font-semibold text-text-primary">Edit schedule</h2>
        </div>
        <form onSubmit={handleSubmit} className="flex flex-col flex-1 overflow-hidden">
          <div className="px-6 py-4 space-y-4 overflow-y-auto flex-1">
            {error && (
              <div className="px-4 py-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-700">{error}</div>
            )}
            <div>
              <label className={labelClass} htmlFor="edit-name">Name</label>
              <input ref={nameRef} id="edit-name" type="text" value={name} onChange={(e) => setName(e.target.value)} className={inputClass} disabled={isSubmitting} required />
            </div>
            <div>
              <label className={labelClass} htmlFor="edit-desc">Description</label>
              <input id="edit-desc" type="text" value={description} onChange={(e) => setDescription(e.target.value)} className={inputClass} disabled={isSubmitting} />
            </div>
            <div>
              <label className={labelClass} htmlFor="edit-tz">Timezone</label>
              <select id="edit-tz" value={timezone} onChange={(e) => setTimezone(e.target.value)} className={inputClass} disabled={isSubmitting}>
                {COMMON_TIMEZONES.map((tz) => <option key={tz} value={tz}>{tz}</option>)}
              </select>
            </div>
            <div>
              <label className={labelClass} htmlFor="edit-default-policy">Default escalation policy</label>
              <select
                id="edit-default-policy"
                value={defaultPolicyId}
                onChange={(e) => setDefaultPolicyId(e.target.value)}
                className={inputClass}
                disabled={isSubmitting}
              >
                <option value="">— None —</option>
                {policies.map((p) => (
                  <option key={p.id} value={p.id}>{p.name}</option>
                ))}
              </select>
              <p className="mt-1 text-xs text-text-tertiary">Used as fallback when no routing rule specifies a policy.</p>
            </div>
            <div>
              <label className={labelClass}>Public holidays</label>
              <p className="text-xs text-text-tertiary mb-2">Select countries to show public holidays on the schedule calendar.</p>
              <div className="grid grid-cols-2 gap-1.5">
                {SUPPORTED_HOLIDAY_COUNTRIES.map(({ code, label }) => (
                  <label
                    key={code}
                    className={`flex items-center gap-2 px-3 py-2 rounded-lg border cursor-pointer text-sm transition-colors ${
                      holidayCountries.includes(code)
                        ? 'border-brand-primary bg-brand-primary-light text-brand-primary font-medium'
                        : 'border-border text-text-secondary hover:bg-gray-50'
                    } ${isSubmitting ? 'opacity-50 cursor-not-allowed' : ''}`}
                  >
                    <input
                      type="checkbox"
                      className="hidden"
                      checked={holidayCountries.includes(code)}
                      onChange={() => !isSubmitting && toggleCountry(code)}
                      disabled={isSubmitting}
                    />
                    <span className="text-base leading-none">{countryFlag(code)}</span>
                    <span>{label}</span>
                  </label>
                ))}
              </div>
              {holidayCountries.length > 0 && (
                <p className="mt-2 text-xs text-text-tertiary">Holidays sync in the background after saving.</p>
              )}
            </div>
          </div>
          <div className="px-6 py-4 border-t border-border flex justify-end gap-3">
            <Button type="button" variant="secondary" onClick={onClose} disabled={isSubmitting}>Cancel</Button>
            <Button type="submit" variant="primary" disabled={isSubmitting}>{isSubmitting ? 'Saving…' : 'Save changes'}</Button>
          </div>
        </form>
      </div>
    </div>
  )
}

// ─── Add layer modal ──────────────────────────────────────────────────────────

interface AddLayerModalProps {
  isOpen: boolean
  scheduleId: string
  nextOrderIndex: number
  scheduleTimezone: string
  onClose: () => void
  onSaved: () => void
}

function AddLayerModal({ isOpen, scheduleId, nextOrderIndex, scheduleTimezone, onClose, onSaved }: AddLayerModalProps) {
  const [name, setName] = useState('')
  const [rotationType, setRotationType] = useState<'daily' | 'weekly' | 'custom'>('weekly')
  const [rotationStart, setRotationStart] = useState('')
  const [customDuration, setCustomDuration] = useState(8)
  const [customUnit, setCustomUnit] = useState<'hours' | 'days'>('hours')
  const [participants, setParticipants] = useState<string[]>([''])
  const [users, setUsers] = useState<UserSummary[]>([])
  const [error, setError] = useState<string | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const nameRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (isOpen) {
      setName('')
      setRotationType('weekly')
      const now = new Date()
      const midnight = new Date(now.getFullYear(), now.getMonth(), now.getDate())
      setRotationStart(localDateTimeStr(midnight))
      setCustomDuration(8)
      setCustomUnit('hours')
      setParticipants([''])
      setError(null)
      setTimeout(() => nameRef.current?.focus(), 50)
      listUsers().then(setUsers).catch(() => {})
    }
  }, [isOpen])

  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    if (isOpen) document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [isOpen, onClose])

  if (!isOpen) return null

  const addParticipant = () => setParticipants((p) => [...p, ''])
  const removeParticipant = (i: number) => setParticipants((p) => p.filter((_, idx) => idx !== i))
  const updateParticipant = (i: number, val: string) =>
    setParticipants((p) => p.map((v, idx) => (idx === i ? val : v)))

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    const validParticipants = participants.map((p) => p.trim()).filter(Boolean)
    if (validParticipants.length === 0) {
      setError('Add at least one participant')
      return
    }

    let shiftDurationSeconds: number | undefined
    if (rotationType === 'custom') {
      shiftDurationSeconds = customDuration * (customUnit === 'hours' ? 3600 : 86400)
      if (shiftDurationSeconds <= 0) {
        setError('Shift duration must be greater than zero')
        return
      }
    }

    setIsSubmitting(true)
    try {
      const body: CreateLayerRequest = {
        name,
        order_index: nextOrderIndex,
        rotation_type: rotationType,
        rotation_start: (() => {
          if (!rotationStart) return undefined
          const d = new Date(rotationStart)
          if (isNaN(d.getTime())) { setError('Invalid rotation start date'); return undefined }
          return d.toISOString()
        })(),
        shift_duration_seconds: shiftDurationSeconds,
        participants: validParticipants.map((user_name, i) => ({ user_name, order_index: i })),
      }
      await createLayer(scheduleId, body)
      onSaved()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create layer')
    } finally {
      setIsSubmitting(false)
    }
  }

  const inputClass = 'w-full px-3 py-2 border border-border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary focus:border-transparent disabled:opacity-50'
  const labelClass = 'block text-sm font-medium text-text-primary mb-1'

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />
      <div
        className="relative z-10 w-full max-w-lg bg-surface-primary rounded-xl shadow-xl mx-4 flex flex-col max-h-[90vh]"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="px-6 py-4 border-b border-border">
          <h2 className="text-lg font-semibold text-text-primary">Add rotation layer</h2>
        </div>
        <form onSubmit={handleSubmit} className="flex flex-col flex-1 overflow-hidden">
          <div className="px-6 py-4 overflow-y-auto flex-1 space-y-4">
            <TzBadge timezone={scheduleTimezone} />
            {error && (
              <div className="px-4 py-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-700">{error}</div>
            )}
            <div>
              <label className={labelClass} htmlFor="layer-name">Layer name</label>
              <input ref={nameRef} id="layer-name" type="text" value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Primary rotation" className={inputClass} disabled={isSubmitting} required />
            </div>
            <div className="flex gap-4">
              <div className="flex-1">
                <label className={labelClass} htmlFor="layer-type">Rotation type</label>
                <select id="layer-type" value={rotationType} onChange={(e) => setRotationType(e.target.value as 'daily' | 'weekly' | 'custom')} className={inputClass} disabled={isSubmitting}>
                  <option value="daily">Daily</option>
                  <option value="weekly">Weekly</option>
                  <option value="custom">Custom</option>
                </select>
              </div>
              {rotationType === 'custom' && (
                <div className="flex-1">
                  <label className={labelClass}>Shift duration</label>
                  <div className="flex gap-2">
                    <input type="number" min={1} value={customDuration} onChange={(e) => setCustomDuration(Number(e.target.value))} className={`${inputClass} w-20`} disabled={isSubmitting} />
                    <select value={customUnit} onChange={(e) => setCustomUnit(e.target.value as 'hours' | 'days')} className={`${inputClass} flex-1`} disabled={isSubmitting}>
                      <option value="hours">Hours</option>
                      <option value="days">Days</option>
                    </select>
                  </div>
                </div>
              )}
            </div>
            <div>
              <div className="flex items-center justify-between mb-1">
                <label className={labelClass} htmlFor="layer-start">Rotation start</label>
                <button
                  type="button"
                  onClick={() => setRotationStart(nextScheduleMidnightAsLocal(scheduleTimezone))}
                  className="flex items-center gap-1 text-xs text-brand-primary hover:underline"
                >
                  <Sunrise className="w-3 h-3" />
                  Use schedule midnight
                </button>
              </div>
              <input id="layer-start" type="datetime-local" value={rotationStart} onChange={(e) => setRotationStart(e.target.value)} className={inputClass} disabled={isSubmitting} />
              {rotationStart && scheduleTimezone ? (
                <p className="mt-1 text-xs text-text-tertiary flex items-center gap-1">
                  <Clock className="w-3 h-3 flex-shrink-0" />
                  In {scheduleTimezone}: {toScheduleTz(rotationStart, scheduleTimezone)}
                </p>
              ) : (
                <p className="mt-1 text-xs text-text-tertiary">The point in time when the rotation begins counting from slot 0.</p>
              )}
            </div>
            <div>
              <label className={labelClass}>Participants (in rotation order)</label>
              <div className="space-y-2">
                {participants.map((p, i) => (
                  <div key={i} className="flex items-center gap-2">
                    <span className="text-xs text-text-tertiary w-5 text-right">{i + 1}.</span>
                    {users.length > 0 ? (
                      <select
                        value={p}
                        onChange={(e) => updateParticipant(i, e.target.value)}
                        className={`${inputClass} flex-1`}
                        disabled={isSubmitting}
                      >
                        <option value="">— Select user —</option>
                        {users.map((u) => (
                          <option key={u.id} value={u.email}>{u.name} ({u.email})</option>
                        ))}
                      </select>
                    ) : (
                      <input
                        type="text"
                        value={p}
                        onChange={(e) => updateParticipant(i, e.target.value)}
                        placeholder="User email"
                        className={`${inputClass} flex-1`}
                        disabled={isSubmitting}
                      />
                    )}
                    {participants.length > 1 && (
                      <button type="button" onClick={() => removeParticipant(i)} className="p-1.5 text-text-tertiary hover:text-red-600 hover:bg-red-50 rounded transition-colors" disabled={isSubmitting}>
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    )}
                  </div>
                ))}
              </div>
              <button type="button" onClick={addParticipant} className="mt-2 text-sm text-brand-primary hover:underline" disabled={isSubmitting}>
                + Add participant
              </button>
            </div>
          </div>
          <div className="px-6 py-4 border-t border-border flex justify-end gap-3">
            <Button type="button" variant="secondary" onClick={onClose} disabled={isSubmitting}>Cancel</Button>
            <Button type="submit" variant="primary" disabled={isSubmitting}>{isSubmitting ? 'Adding…' : 'Add layer'}</Button>
          </div>
        </form>
      </div>
    </div>
  )
}

// ─── Edit layer modal ─────────────────────────────────────────────────────────

interface EditLayerModalProps {
  isOpen: boolean
  scheduleId: string
  layer: ScheduleLayer
  scheduleTimezone: string
  onClose: () => void
  onSaved: () => void
}

function EditLayerModal({ isOpen, scheduleId, layer, scheduleTimezone, onClose, onSaved }: EditLayerModalProps) {
  const rotationStartStr = (() => {
    const d = new Date(layer.rotation_start)
    return isNaN(d.getTime()) ? '' : localDateTimeStr(d)
  })()

  const initialParticipants = (layer.participants ?? [])
    .sort((a, b) => a.order_index - b.order_index)
    .map((p) => p.user_name)

  const [name, setName] = useState(layer.name)
  const [rotationType, setRotationType] = useState<'daily' | 'weekly' | 'custom'>(layer.rotation_type)
  const [rotationStart, setRotationStart] = useState(rotationStartStr)
  const [customDuration, setCustomDuration] = useState(() => {
    const secs = layer.shift_duration_seconds || 3600
    if (secs % 86400 === 0) return secs / 86400
    return secs / 3600
  })
  const [customUnit, setCustomUnit] = useState<'hours' | 'days'>(() => {
    const secs = layer.shift_duration_seconds || 3600
    return secs % 86400 === 0 ? 'days' : 'hours'
  })
  const [participants, setParticipants] = useState<string[]>(
    initialParticipants.length > 0 ? initialParticipants : [''],
  )
  const [users, setUsers] = useState<UserSummary[]>([])
  const [error, setError] = useState<string | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const nameRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (isOpen) {
      setName(layer.name)
      setRotationType(layer.rotation_type)
      const d = new Date(layer.rotation_start)
      setRotationStart(isNaN(d.getTime()) ? '' : localDateTimeStr(d))
      const secs = layer.shift_duration_seconds || 3600
      if (secs % 86400 === 0) {
        setCustomDuration(secs / 86400)
        setCustomUnit('days')
      } else {
        setCustomDuration(secs / 3600)
        setCustomUnit('hours')
      }
      const sorted = (layer.participants ?? [])
        .sort((a, b) => a.order_index - b.order_index)
        .map((p) => p.user_name)
      setParticipants(sorted.length > 0 ? sorted : [''])
      setError(null)
      listUsers().then(setUsers).catch(() => {})
      setTimeout(() => nameRef.current?.focus(), 50)
    }
  }, [isOpen, layer])

  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    if (isOpen) document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [isOpen, onClose])

  if (!isOpen) return null

  const addParticipant = () => setParticipants((p) => [...p, ''])
  const removeParticipant = (i: number) => setParticipants((p) => p.filter((_, idx) => idx !== i))
  const updateParticipant = (i: number, val: string) =>
    setParticipants((p) => p.map((v, idx) => (idx === i ? val : v)))

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    const validParticipants = participants.map((p) => p.trim()).filter(Boolean)
    if (validParticipants.length === 0) {
      setError('Add at least one participant')
      return
    }

    let shiftDurationSeconds: number | undefined
    if (rotationType === 'custom') {
      shiftDurationSeconds = customDuration * (customUnit === 'hours' ? 3600 : 86400)
      if (shiftDurationSeconds <= 0) {
        setError('Shift duration must be greater than zero')
        return
      }
    }

    setIsSubmitting(true)
    try {
      const body: UpdateLayerRequest = {
        name,
        rotation_type: rotationType,
        rotation_start: (() => {
          if (!rotationStart) return undefined
          const d = new Date(rotationStart)
          if (isNaN(d.getTime())) return undefined
          return d.toISOString()
        })(),
        shift_duration_seconds: shiftDurationSeconds,
        participants: validParticipants.map((user_name, i) => ({ user_name, order_index: i })),
      }
      await updateLayer(scheduleId, layer.id, body)
      onSaved()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update layer')
    } finally {
      setIsSubmitting(false)
    }
  }

  const inputClass = 'w-full px-3 py-2 border border-border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary focus:border-transparent disabled:opacity-50'
  const labelClass = 'block text-sm font-medium text-text-primary mb-1'

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />
      <div
        className="relative z-10 w-full max-w-lg bg-surface-primary rounded-xl shadow-xl mx-4 flex flex-col max-h-[90vh]"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="px-6 py-4 border-b border-border">
          <h2 className="text-lg font-semibold text-text-primary">Edit layer</h2>
        </div>
        <form onSubmit={handleSubmit} className="flex flex-col flex-1 overflow-hidden">
          <div className="px-6 py-4 overflow-y-auto flex-1 space-y-4">
            {error && (
              <div className="px-4 py-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-700">{error}</div>
            )}
            <TzBadge timezone={scheduleTimezone} />
            <div>
              <label className={labelClass} htmlFor="edit-layer-name">Layer name</label>
              <input ref={nameRef} id="edit-layer-name" type="text" value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Primary rotation" className={inputClass} disabled={isSubmitting} required />
            </div>
            <div className="flex gap-4">
              <div className="flex-1">
                <label className={labelClass} htmlFor="edit-layer-type">Rotation type</label>
                <select id="edit-layer-type" value={rotationType} onChange={(e) => setRotationType(e.target.value as 'daily' | 'weekly' | 'custom')} className={inputClass} disabled={isSubmitting}>
                  <option value="daily">Daily</option>
                  <option value="weekly">Weekly</option>
                  <option value="custom">Custom</option>
                </select>
              </div>
              {rotationType === 'custom' && (
                <div className="flex-1">
                  <label className={labelClass}>Shift duration</label>
                  <div className="flex gap-2">
                    <input type="number" min={1} value={customDuration} onChange={(e) => setCustomDuration(Number(e.target.value))} className={`${inputClass} w-20`} disabled={isSubmitting} />
                    <select value={customUnit} onChange={(e) => setCustomUnit(e.target.value as 'hours' | 'days')} className={`${inputClass} flex-1`} disabled={isSubmitting}>
                      <option value="hours">Hours</option>
                      <option value="days">Days</option>
                    </select>
                  </div>
                </div>
              )}
            </div>
            <div>
              <div className="flex items-center justify-between mb-1">
                <label className={labelClass} htmlFor="edit-layer-start">Rotation start</label>
                <button
                  type="button"
                  onClick={() => setRotationStart(nextScheduleMidnightAsLocal(scheduleTimezone))}
                  className="flex items-center gap-1 text-xs text-brand-primary hover:underline"
                >
                  <Sunrise className="w-3 h-3" />
                  Use schedule midnight
                </button>
              </div>
              <input id="edit-layer-start" type="datetime-local" value={rotationStart} onChange={(e) => setRotationStart(e.target.value)} className={inputClass} disabled={isSubmitting} />
              {rotationStart && scheduleTimezone ? (
                <p className="mt-1 text-xs text-text-tertiary flex items-center gap-1">
                  <Clock className="w-3 h-3 flex-shrink-0" />
                  In {scheduleTimezone}: {toScheduleTz(rotationStart, scheduleTimezone)}
                </p>
              ) : (
                <p className="mt-1 text-xs text-text-tertiary">The point in time when the rotation begins counting from slot 0.</p>
              )}
            </div>
            <div>
              <label className={labelClass}>Participants (in rotation order)</label>
              <div className="space-y-2">
                {participants.map((p, i) => (
                  <div key={i} className="flex items-center gap-2">
                    <span className="text-xs text-text-tertiary w-5 text-right">{i + 1}.</span>
                    {users.length > 0 ? (
                      <select
                        value={p}
                        onChange={(e) => updateParticipant(i, e.target.value)}
                        className={`${inputClass} flex-1`}
                        disabled={isSubmitting}
                      >
                        <option value="">— Select user —</option>
                        {users.map((u) => (
                          <option key={u.id} value={u.email}>{u.name} ({u.email})</option>
                        ))}
                      </select>
                    ) : (
                      <input
                        type="text"
                        value={p}
                        onChange={(e) => updateParticipant(i, e.target.value)}
                        placeholder="User email"
                        className={`${inputClass} flex-1`}
                        disabled={isSubmitting}
                      />
                    )}
                    {participants.length > 1 && (
                      <button type="button" onClick={() => removeParticipant(i)} className="p-1.5 text-text-tertiary hover:text-red-600 hover:bg-red-50 rounded transition-colors" disabled={isSubmitting}>
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    )}
                  </div>
                ))}
              </div>
              <button type="button" onClick={addParticipant} className="mt-2 text-sm text-brand-primary hover:underline" disabled={isSubmitting}>
                + Add participant
              </button>
            </div>
          </div>
          <div className="px-6 py-4 border-t border-border flex justify-end gap-3">
            <Button type="button" variant="secondary" onClick={onClose} disabled={isSubmitting}>Cancel</Button>
            <Button type="submit" variant="primary" disabled={isSubmitting}>{isSubmitting ? 'Saving…' : 'Save changes'}</Button>
          </div>
        </form>
      </div>
    </div>
  )
}

// ─── Override modal ───────────────────────────────────────────────────────────

interface OverrideModalProps {
  isOpen: boolean
  scheduleId: string
  scheduleTimezone: string
  prefilledStart?: string
  onClose: () => void
  onSaved: () => void
}

function OverrideModal({ isOpen, scheduleId, scheduleTimezone, prefilledStart, onClose, onSaved }: OverrideModalProps) {
  const [overrideUser, setOverrideUser] = useState('')
  const [users, setUsers] = useState<UserSummary[]>([])
  const [startTime, setStartTime] = useState('')
  const [endTime, setEndTime] = useState('')
  const [replacesSegments, setReplacesSegments] = useState<TimelineSegment[]>([])
  const [error, setError] = useState<string | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)

  useEffect(() => {
    if (isOpen) {
      setOverrideUser('')
      if (prefilledStart) {
        const end = new Date(new Date(prefilledStart).getTime() + 86_400_000)
        setStartTime(prefilledStart)
        setEndTime(localDateTimeStr(end))
      } else {
        const n = new Date()
        const startLocal = new Date(n.getFullYear(), n.getMonth(), n.getDate(), n.getHours())
        const endLocal = new Date(startLocal.getTime() + 86_400_000)
        setStartTime(localDateTimeStr(startLocal))
        setEndTime(localDateTimeStr(endLocal))
      }
      setReplacesSegments([])
      setError(null)
      listUsers().then(setUsers).catch(() => {})
    }
  }, [isOpen, prefilledStart])

  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    if (isOpen) document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [isOpen, onClose])

  useEffect(() => {
    if (!startTime || !endTime) { setReplacesSegments([]); return }
    const start = new Date(startTime)
    const end = new Date(endTime)
    if (isNaN(start.getTime()) || isNaN(end.getTime()) || end <= start) {
      setReplacesSegments([])
      return
    }
    const timer = setTimeout(() => {
      getLayerTimelines(scheduleId, start.toISOString(), end.toISOString())
        .then(data => setReplacesSegments((data.effective ?? []).filter(s => !s.is_override)))
        .catch(() => setReplacesSegments([]))
    }, 400)
    return () => clearTimeout(timer)
  }, [scheduleId, startTime, endTime])

  if (!isOpen) return null

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    const start = new Date(startTime)
    const end = new Date(endTime)
    if (end <= start) {
      setError('End time must be after start time')
      return
    }

    setIsSubmitting(true)
    try {
      const body: CreateOverrideRequest = {
        override_user: overrideUser,
        start_time: start.toISOString(),
        end_time: end.toISOString(),
      }
      await createOverride(scheduleId, body)
      onSaved()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create override')
    } finally {
      setIsSubmitting(false)
    }
  }

  const inputClass = 'w-full px-3 py-2 border border-border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary focus:border-transparent disabled:opacity-50'
  const labelClass = 'block text-sm font-medium text-text-primary mb-1'
  const replacesUsers = [...new Set(replacesSegments.map(s => s.user_name).filter(Boolean))]

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />
      <div
        className="relative z-10 w-full max-w-md bg-surface-primary rounded-xl shadow-xl mx-4"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="px-6 py-4 border-b border-border">
          <h2 className="text-lg font-semibold text-text-primary">Create override</h2>
          <p className="text-sm text-text-secondary mt-0.5">Temporarily replace the on-call user for a specific time window.</p>
        </div>
        <form onSubmit={handleSubmit}>
          <div className="px-6 py-4 space-y-4">
            {error && (
              <div className="px-4 py-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-700">{error}</div>
            )}
            <TzBadge timezone={scheduleTimezone} />
            <div>
              <label className={labelClass} htmlFor="ov-user">On-call user during override</label>
              {users.length > 0 ? (
                <select
                  id="ov-user"
                  value={overrideUser}
                  onChange={(e) => setOverrideUser(e.target.value)}
                  className={inputClass}
                  disabled={isSubmitting}
                  required
                >
                  <option value="">— Select user —</option>
                  {users.map((u) => (
                    <option key={u.id} value={u.email}>{u.name} ({u.email})</option>
                  ))}
                </select>
              ) : (
                <input
                  id="ov-user"
                  type="text"
                  value={overrideUser}
                  onChange={(e) => setOverrideUser(e.target.value)}
                  placeholder="User email"
                  className={inputClass}
                  disabled={isSubmitting}
                  required
                />
              )}
            </div>
            <div className="flex gap-4">
              <div className="flex-1">
                <label className={labelClass} htmlFor="ov-start">Start</label>
                <input id="ov-start" type="datetime-local" value={startTime} onChange={(e) => setStartTime(e.target.value)} className={inputClass} disabled={isSubmitting} required />
                {startTime && scheduleTimezone && (
                  <p className="mt-1 text-xs text-text-tertiary">{toScheduleTz(startTime, scheduleTimezone)}</p>
                )}
              </div>
              <div className="flex-1">
                <label className={labelClass} htmlFor="ov-end">End</label>
                <input id="ov-end" type="datetime-local" value={endTime} onChange={(e) => setEndTime(e.target.value)} className={inputClass} disabled={isSubmitting} required />
                {endTime && scheduleTimezone && (
                  <p className="mt-1 text-xs text-text-tertiary">{toScheduleTz(endTime, scheduleTimezone)}</p>
                )}
              </div>
            </div>
            {startTime && endTime && new Date(endTime) > new Date(startTime) && (
              <p className="text-xs text-text-tertiary">Duration: <span className="font-medium text-text-secondary">{formatDuration(startTime, endTime)}</span></p>
            )}
            {replacesUsers.length > 0 && (
              <div className="flex items-start gap-2 px-3 py-2.5 bg-amber-50 border border-amber-200 rounded-lg">
                <AlertCircle className="w-3.5 h-3.5 text-amber-600 flex-shrink-0 mt-0.5" />
                <p className="text-xs text-amber-800">
                  <span className="font-semibold">Replaces: </span>
                  {replacesUsers.join(', ')}
                </p>
              </div>
            )}
          </div>
          <div className="px-6 py-4 border-t border-border flex justify-end gap-3">
            <Button type="button" variant="secondary" onClick={onClose} disabled={isSubmitting}>Cancel</Button>
            <Button type="submit" variant="primary" disabled={isSubmitting}>{isSubmitting ? 'Creating…' : 'Create override'}</Button>
          </div>
        </form>
      </div>
    </div>
  )
}

// ─── Overrides table ──────────────────────────────────────────────────────────

interface OverridesTableProps {
  scheduleId: string
  overrides: ScheduleOverride[]
  onDeleted: () => void
  onAdd: () => void
  toast: ReturnType<typeof useToast>
}

function OverridesTable({ scheduleId, overrides, onDeleted, onAdd, toast }: OverridesTableProps) {
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const [confirmId, setConfirmId] = useState<string | null>(null)

  const handleDelete = async (ov: ScheduleOverride) => {
    setConfirmId(null)
    setDeletingId(ov.id)
    try {
      await deleteOverride(scheduleId, ov.id)
      toast.success('Override deleted')
      onDeleted()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to delete override')
    } finally {
      setDeletingId(null)
    }
  }

  const fmt = (iso: string) =>
    new Date(iso).toLocaleString(undefined, {
      month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
    })

  const sorted = [...overrides].sort((a, b) => {
    const rank = { active: 0, upcoming: 1, past: 2 }
    const ra = rank[overrideStatus(a)]
    const rb = rank[overrideStatus(b)]
    if (ra !== rb) return ra - rb
    return new Date(a.start_time).getTime() - new Date(b.start_time).getTime()
  })

  const statusPill = (ov: ScheduleOverride) => {
    const s = overrideStatus(ov)
    if (s === 'active') return (
      <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase tracking-wide bg-green-100 text-green-700">Active</span>
    )
    if (s === 'upcoming') return (
      <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase tracking-wide bg-blue-100 text-blue-700">Upcoming</span>
    )
    return (
      <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase tracking-wide bg-gray-100 text-gray-500">Past</span>
    )
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-sm font-semibold text-text-primary">Overrides</h2>
        <Button size="sm" variant="secondary" onClick={onAdd}>
          <Plus className="w-3.5 h-3.5" />
          Add override
        </Button>
      </div>
      {overrides.length === 0 ? (
        <p className="text-sm text-text-tertiary italic">No overrides.</p>
      ) : (
        <div className="bg-white border border-border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border bg-gray-50">
                <th className="text-left px-4 py-2 text-xs font-medium text-text-tertiary uppercase tracking-wider">Status</th>
                <th className="text-left px-4 py-2 text-xs font-medium text-text-tertiary uppercase tracking-wider">On-call user</th>
                <th className="text-left px-4 py-2 text-xs font-medium text-text-tertiary uppercase tracking-wider">Start</th>
                <th className="text-left px-4 py-2 text-xs font-medium text-text-tertiary uppercase tracking-wider">End</th>
                <th className="text-left px-4 py-2 text-xs font-medium text-text-tertiary uppercase tracking-wider">Duration</th>
                <th className="text-left px-4 py-2 text-xs font-medium text-text-tertiary uppercase tracking-wider">Created by</th>
                <th className="w-24 px-4 py-2" />
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {sorted.map((ov) => {
                const isPast = overrideStatus(ov) === 'past'
                return (
                  <tr key={ov.id} className={`hover:bg-gray-50 ${isPast ? 'opacity-50' : ''}`}>
                    <td className="px-4 py-2.5">{statusPill(ov)}</td>
                    <td className="px-4 py-2.5 font-medium text-text-primary">{ov.override_user}</td>
                    <td className="px-4 py-2.5 text-text-secondary text-xs">{fmt(ov.start_time)}</td>
                    <td className="px-4 py-2.5 text-text-secondary text-xs">{fmt(ov.end_time)}</td>
                    <td className="px-4 py-2.5 text-text-tertiary text-xs font-mono">{formatDuration(ov.start_time, ov.end_time)}</td>
                    <td className="px-4 py-2.5 text-text-tertiary text-xs">{ov.created_by || '—'}</td>
                    <td className="px-4 py-2.5">
                      {!isPast && (
                        confirmId === ov.id ? (
                          <div className="flex items-center gap-1">
                            <button
                              onClick={() => handleDelete(ov)}
                              disabled={deletingId === ov.id}
                              className="text-xs px-2 py-1 bg-red-600 text-white rounded hover:bg-red-700 disabled:opacity-50 transition-colors"
                            >
                              Delete
                            </button>
                            <button
                              onClick={() => setConfirmId(null)}
                              className="text-xs px-2 py-1 bg-gray-100 text-text-secondary rounded hover:bg-gray-200 transition-colors"
                            >
                              Cancel
                            </button>
                          </div>
                        ) : (
                          <button
                            onClick={() => setConfirmId(ov.id)}
                            disabled={deletingId === ov.id}
                            className="p-1.5 text-text-tertiary hover:text-red-600 hover:bg-red-50 rounded transition-colors disabled:opacity-50"
                            title="Delete override"
                          >
                            <Trash2 className="w-3.5 h-3.5" />
                          </button>
                        )
                      )}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

// ─── Unavailability modal ─────────────────────────────────────────────────────

interface UnavailabilityModalProps {
  isOpen: boolean
  scheduleId: string
  users: string[]
  onClose: () => void
  onSaved: () => void
}

function UnavailabilityModal({ isOpen, scheduleId, users, onClose, onSaved }: UnavailabilityModalProps) {
  const [userName, setUserName] = useState(users[0] ?? '')
  const [startDate, setStartDate] = useState('')
  const [endDate, setEndDate] = useState('')
  const [reason, setReason] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)

  useEffect(() => {
    if (isOpen) {
      setUserName(users[0] ?? '')
      setStartDate('')
      setEndDate('')
      setReason('')
      setError(null)
    }
  }, [isOpen, users])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!startDate || !endDate) { setError('Start and end dates are required'); return }
    if (endDate < startDate) { setError('End date must be on or after start date'); return }
    setIsSubmitting(true)
    setError(null)
    try {
      await createUnavailability(scheduleId, {
        user_name: userName,
        start_date: startDate + 'T00:00:00Z',
        end_date: endDate + 'T00:00:00Z',
        reason: reason || undefined,
      })
      onSaved()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create unavailability')
    } finally {
      setIsSubmitting(false)
    }
  }

  if (!isOpen) return null

  const labelClass = 'block text-xs font-medium text-text-secondary mb-1'
  const inputClass = 'w-full border border-border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary/30 focus:border-brand-primary'

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="bg-white rounded-xl shadow-xl w-full max-w-md mx-4 p-6">
        <h2 className="text-lg font-semibold text-text-primary mb-4">Mark as unavailable</h2>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className={labelClass} htmlFor="unav-user">User</label>
            {users.length > 0 ? (
              <select
                id="unav-user"
                className={inputClass}
                value={userName}
                onChange={e => setUserName(e.target.value)}
                required
              >
                {users.map(u => <option key={u} value={u}>{u}</option>)}
              </select>
            ) : (
              <input
                id="unav-user"
                className={inputClass}
                value={userName}
                onChange={e => setUserName(e.target.value)}
                placeholder="user_name"
                required
              />
            )}
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className={labelClass} htmlFor="unav-start">Start date</label>
              <input
                id="unav-start"
                type="date"
                className={inputClass}
                value={startDate}
                onChange={e => setStartDate(e.target.value)}
                required
              />
            </div>
            <div>
              <label className={labelClass} htmlFor="unav-end">End date (inclusive)</label>
              <input
                id="unav-end"
                type="date"
                className={inputClass}
                value={endDate}
                min={startDate}
                onChange={e => setEndDate(e.target.value)}
                required
              />
            </div>
          </div>
          <div>
            <label className={labelClass} htmlFor="unav-reason">Reason (optional)</label>
            <input
              id="unav-reason"
              className={inputClass}
              value={reason}
              onChange={e => setReason(e.target.value)}
              placeholder="PTO, sick leave, etc."
              maxLength={500}
            />
          </div>
          {error && <p className="text-xs text-red-600">{error}</p>}
          <div className="flex justify-end gap-2 pt-2">
            <Button type="button" variant="secondary" onClick={onClose}>Cancel</Button>
            <Button type="submit" variant="primary" disabled={isSubmitting}>
              {isSubmitting ? 'Saving…' : 'Mark unavailable'}
            </Button>
          </div>
        </form>
      </div>
    </div>
  )
}

// ─── Unavailabilities table ───────────────────────────────────────────────────

interface UnavailabilitiesTableProps {
  scheduleId: string
  unavailabilities: ScheduleUnavailability[]
  onDeleted: () => void
  onAdd: () => void
  toast: ReturnType<typeof useToast>
}

function UnavailabilitiesTable({ scheduleId, unavailabilities, onDeleted, onAdd, toast }: UnavailabilitiesTableProps) {
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const [confirmId, setConfirmId] = useState<string | null>(null)

  const handleDelete = async (u: ScheduleUnavailability) => {
    setConfirmId(null)
    setDeletingId(u.id)
    try {
      await deleteUnavailability(scheduleId, u.id)
      toast.success('Unavailability removed')
      onDeleted()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to remove unavailability')
    } finally {
      setDeletingId(null)
    }
  }

  const today = localDateStr(new Date())

  const statusPill = (u: ScheduleUnavailability) => {
    if (u.end_date < today) return (
      <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase tracking-wide bg-gray-100 text-gray-500">Past</span>
    )
    if (u.start_date <= today) return (
      <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase tracking-wide bg-amber-100 text-amber-700">Active</span>
    )
    return (
      <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase tracking-wide bg-blue-100 text-blue-700">Upcoming</span>
    )
  }

  const dayCount = (u: ScheduleUnavailability) => {
    const start = new Date(u.start_date)
    const end = new Date(u.end_date)
    const days = Math.round((end.getTime() - start.getTime()) / 86400000) + 1
    return days === 1 ? '1 day' : `${days} days`
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-sm font-semibold text-text-primary">Leave / Unavailability</h2>
        <Button size="sm" variant="secondary" onClick={onAdd}>
          <Plus className="w-3.5 h-3.5" />
          Mark unavailable
        </Button>
      </div>
      {unavailabilities.length === 0 ? (
        <p className="text-sm text-text-tertiary italic">No unavailabilities.</p>
      ) : (
        <div className="bg-white border border-border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border bg-gray-50">
                <th className="text-left px-4 py-2 text-xs font-medium text-text-tertiary uppercase tracking-wider">Status</th>
                <th className="text-left px-4 py-2 text-xs font-medium text-text-tertiary uppercase tracking-wider">User</th>
                <th className="text-left px-4 py-2 text-xs font-medium text-text-tertiary uppercase tracking-wider">Start</th>
                <th className="text-left px-4 py-2 text-xs font-medium text-text-tertiary uppercase tracking-wider">End</th>
                <th className="text-left px-4 py-2 text-xs font-medium text-text-tertiary uppercase tracking-wider">Duration</th>
                <th className="text-left px-4 py-2 text-xs font-medium text-text-tertiary uppercase tracking-wider">Reason</th>
                <th className="w-24 px-4 py-2" />
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {unavailabilities.map((u) => {
                const isPast = u.end_date < today
                return (
                  <tr key={u.id} className={`hover:bg-gray-50 ${isPast ? 'opacity-50' : ''}`}>
                    <td className="px-4 py-2.5">{statusPill(u)}</td>
                    <td className="px-4 py-2.5 font-medium text-text-primary">{u.user_name}</td>
                    <td className="px-4 py-2.5 text-text-secondary text-xs font-mono">{u.start_date}</td>
                    <td className="px-4 py-2.5 text-text-secondary text-xs font-mono">{u.end_date}</td>
                    <td className="px-4 py-2.5 text-text-tertiary text-xs">{dayCount(u)}</td>
                    <td className="px-4 py-2.5 text-text-tertiary text-xs">{u.reason || '—'}</td>
                    <td className="px-4 py-2.5">
                      {confirmId === u.id ? (
                        <div className="flex items-center gap-1">
                          <button
                            onClick={() => handleDelete(u)}
                            disabled={deletingId === u.id}
                            className="text-xs px-2 py-1 bg-red-600 text-white rounded hover:bg-red-700 disabled:opacity-50 transition-colors"
                          >
                            Remove
                          </button>
                          <button
                            onClick={() => setConfirmId(null)}
                            className="text-xs px-2 py-1 bg-gray-100 text-text-secondary rounded hover:bg-gray-200 transition-colors"
                          >
                            Cancel
                          </button>
                        </div>
                      ) : (
                        <button
                          onClick={() => setConfirmId(u.id)}
                          disabled={deletingId === u.id}
                          className="p-1.5 text-text-tertiary hover:text-red-600 hover:bg-red-50 rounded transition-colors disabled:opacity-50"
                          title="Remove unavailability"
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                      )}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

// ─── Layer card ───────────────────────────────────────────────────────────────

interface LayerCardProps {
  scheduleId: string
  layer: ScheduleLayer
  currentOnCallUser: string | null
  onDeleted: () => void
  onEdit: (layer: ScheduleLayer) => void
  toast: ReturnType<typeof useToast>
}

function LayerCard({ scheduleId, layer, currentOnCallUser, onDeleted, onEdit, toast }: LayerCardProps) {
  const [deleting, setDeleting] = useState(false)

  const handleDelete = async () => {
    if (!confirm(`Delete layer "${layer.name}"? Participants will also be removed.`)) return
    setDeleting(true)
    try {
      await deleteLayer(scheduleId, layer.id)
      toast.success('Layer deleted')
      onDeleted()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to delete layer')
      setDeleting(false)
    }
  }

  const rotationLabel = {
    daily: 'Daily',
    weekly: 'Weekly',
    custom: `Every ${Math.round(layer.shift_duration_seconds / 3600)}h`,
  }[layer.rotation_type]

  const startDate = new Date(layer.rotation_start).toLocaleDateString(undefined, {
    month: 'short', day: 'numeric', year: 'numeric',
  })

  return (
    <div className="bg-white border border-border rounded-lg p-4">
      <div className="flex items-start justify-between mb-3">
        <div>
          <span className="font-medium text-text-primary text-sm">{layer.name}</span>
          <div className="flex items-center gap-2 mt-1">
            <span className="text-xs px-2 py-0.5 bg-gray-100 text-text-secondary rounded font-medium">
              {rotationLabel}
            </span>
            <span className="text-xs text-text-tertiary">starts {startDate}</span>
          </div>
        </div>
        <div className="flex items-center gap-1">
          <button
            onClick={() => onEdit(layer)}
            className="p-1.5 text-text-tertiary hover:text-brand-primary hover:bg-blue-50 rounded transition-colors"
            title="Edit layer"
          >
            <Pencil className="w-3.5 h-3.5" />
          </button>
          <button
            onClick={handleDelete}
            disabled={deleting}
            className="p-1.5 text-text-tertiary hover:text-red-600 hover:bg-red-50 rounded transition-colors disabled:opacity-50"
            title="Delete layer"
          >
            <Trash2 className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {/* Participants */}
      {layer.participants && layer.participants.length > 0 ? (
        <ol className="space-y-1">
          {layer.participants.map((p) => {
            const isCurrent = p.user_name === currentOnCallUser
            return (
              <li
                key={p.id}
                className={`flex items-center gap-2 px-2 py-1 rounded text-sm ${
                  isCurrent ? 'bg-green-50 font-semibold text-green-800' : 'text-text-secondary'
                }`}
              >
                <span className="text-xs text-text-tertiary w-4 text-right flex-shrink-0">
                  {p.order_index + 1}.
                </span>
                <span
                  className="w-2 h-2 rounded-full flex-shrink-0"
                  style={{ backgroundColor: segmentBg(p.user_name), border: `1px solid ${segmentText(p.user_name)}` }}
                />
                <span className="flex-1">{p.user_name}</span>
                {isCurrent && (
                  <span className="text-xs text-green-700 font-normal">← on call now</span>
                )}
              </li>
            )
          })}
        </ol>
      ) : (
        <p className="text-xs text-text-tertiary italic">No participants configured.</p>
      )}
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

/**
 * Schedule detail page.
 * Shows current on-call card, 2-week shift calendar, overrides table, and layer cards.
 * Routes: GET /on-call/:id
 */
export function ScheduleDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const toast = useToast()

  const { schedule, onCall, overrides, unavailabilities, loading, error, refetch } = useSchedule(id!)

  const [editModalOpen, setEditModalOpen] = useState(false)
  const [addLayerOpen, setAddLayerOpen] = useState(false)
  const [overrideModalOpen, setOverrideModalOpen] = useState(false)
  const [overridePrefilledStart, setOverridePrefilledStart] = useState<string | undefined>()
  const [unavailModalOpen, setUnavailModalOpen] = useState(false)
  const [editLayerOpen, setEditLayerOpen] = useState(false)
  const [holidays, setHolidays] = useState<ScheduleHoliday[]>([])
  const [editingLayer, setEditingLayer] = useState<ScheduleLayer | null>(null)
  const [windowStart, setWindowStart] = useState<Date>(() => getMonthStart(new Date()))
  const GANTT_DAYS = useMemo(() => daysInMonth(windowStart), [windowStart])

  const [layerTimelines, setLayerTimelines] = useState<LayerTimelinesResponse | null>(null)
  const [layerTimelineKey, setLayerTimelineKey] = useState(0)

  useEffect(() => {
    if (!id) return
    let cancelled = false
    const from = windowStart.toISOString()
    const toDate = new Date(windowStart)
    toDate.setDate(toDate.getDate() + GANTT_DAYS)
    const to = toDate.toISOString()
    getLayerTimelines(id, from, to)
      .then((data) => { if (!cancelled) setLayerTimelines(data) })
      .catch(() => {})
    return () => { cancelled = true }
  }, [id, windowStart, GANTT_DAYS, layerTimelineKey])

  // Fetch holidays for the visible window whenever the schedule or month changes.
  useEffect(() => {
    if (!id) return
    const from = localDateStr(windowStart)
    const toDate = new Date(windowStart)
    toDate.setDate(toDate.getDate() + GANTT_DAYS)
    const to = localDateStr(toDate)
    getHolidays(id, from, to)
      .then(r => setHolidays(r.data))
      .catch(() => {})
  }, [id, windowStart, GANTT_DAYS])

  const handleScheduleDeleted = async () => {
    if (!schedule) return
    const hasLayers = (schedule.layers?.length ?? 0) > 0
    const warning = hasLayers
      ? `"${schedule.name}" has active layers. Delete it and all rotation data?`
      : `Delete schedule "${schedule.name}"? This cannot be undone.`
    if (!confirm(warning)) return
    try {
      await deleteSchedule(schedule.id)
      toast.success('Schedule deleted')
      navigate('/on-call/schedules')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to delete schedule')
    }
  }

  const invalidateAll = useCallback(() => {
    refetch()
    setLayerTimelineKey((k) => k + 1)
  }, [refetch])

  const ganttRows: GanttRow[] = useMemo(() => {
    const overrideSegments = (layerTimelines?.effective ?? []).filter(s => s.is_override)
    return [...(schedule?.layers ?? [])]
      .sort((a, b) => a.order_index - b.order_index)
      .map((layer) => ({
        id: layer.id,
        label: layer.name,
        segments: [...(layerTimelines?.layers[layer.id] ?? []), ...overrideSegments],
      }))
  }, [schedule?.layers, layerTimelines])

  const allParticipants = useMemo(() => {
    const seen = new Set<string>()
    const names: string[] = []
    for (const layer of schedule?.layers ?? []) {
      for (const p of layer.participants ?? []) {
        if (!seen.has(p.user_name)) { seen.add(p.user_name); names.push(p.user_name) }
      }
    }
    return names
  }, [schedule?.layers])

  const currentSegment = useMemo(() => {
    const now = new Date().toISOString()
    return (layerTimelines?.effective ?? []).find(
      s => s.start <= now && s.end > now
    ) ?? null
  }, [layerTimelines])

  const handleEditLayer = (layer: ScheduleLayer) => {
    setEditingLayer(layer)
    setEditLayerOpen(true)
  }

  const handleOverrideSaved = () => {
    toast.success('Override created')
    invalidateAll()
  }

  const handleOverrideDeleted = () => {
    invalidateAll()
  }

  const handleDayClick = (date: Date) => {
    setOverridePrefilledStart(localDateStr(date) + 'T00:00')
    setOverrideModalOpen(true)
  }

  if (loading) return <SkeletonDetail />
  if (error) return <GeneralError message={error} onRetry={refetch} />
  if (!schedule) return null

  const currentUser = onCall?.user_name || null
  const nextOrderIndex = (schedule.layers?.length ?? 0)

  return (
    <div className="flex flex-col h-full">
      <ToastContainer toasts={toast.toasts} onDismiss={toast.dismissToast} />

      {editModalOpen && (
        <EditScheduleModal
          isOpen={editModalOpen}
          schedule={schedule}
          onClose={() => setEditModalOpen(false)}
          onSaved={() => { toast.success('Schedule updated'); refetch() }}
        />
      )}

      {addLayerOpen && (
        <AddLayerModal
          isOpen={addLayerOpen}
          scheduleId={schedule.id}
          nextOrderIndex={nextOrderIndex}
          scheduleTimezone={schedule.timezone}
          onClose={() => setAddLayerOpen(false)}
          onSaved={() => { toast.success('Layer added'); invalidateAll() }}
        />
      )}

      {editLayerOpen && editingLayer && (
        <EditLayerModal
          isOpen={editLayerOpen}
          scheduleId={schedule.id}
          layer={editingLayer}
          scheduleTimezone={schedule.timezone}
          onClose={() => { setEditLayerOpen(false); setEditingLayer(null) }}
          onSaved={() => { toast.success('Layer updated'); invalidateAll() }}
        />
      )}

      {overrideModalOpen && (
        <OverrideModal
          isOpen={overrideModalOpen}
          scheduleId={schedule.id}
          scheduleTimezone={schedule.timezone}
          prefilledStart={overridePrefilledStart}
          onClose={() => { setOverrideModalOpen(false); setOverridePrefilledStart(undefined) }}
          onSaved={handleOverrideSaved}
        />
      )}

      {unavailModalOpen && (
        <UnavailabilityModal
          isOpen={unavailModalOpen}
          scheduleId={schedule.id}
          users={allParticipants}
          onClose={() => setUnavailModalOpen(false)}
          onSaved={() => { toast.success('Unavailability recorded'); invalidateAll() }}
        />
      )}

      {/* Page Header */}
      <div className="border-b border-border bg-surface-primary px-6 py-4">
        <div className="flex items-center gap-2 text-sm text-text-secondary mb-2">
          <Link to="/on-call/schedules" className="hover:text-text-primary transition-colors">On-call</Link>
          <ChevronRight className="w-3.5 h-3.5" />
          <span className="text-text-primary font-medium">{schedule.name}</span>
        </div>
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-text-primary">{schedule.name}</h1>
            <div className="flex items-center gap-3 mt-1">
              <span className="text-sm text-text-secondary font-mono">{schedule.timezone}</span>
              {schedule.description && (
                <span className="text-sm text-text-secondary">· {schedule.description}</span>
              )}
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Button size="sm" variant="secondary" onClick={() => setEditModalOpen(true)}>
              <Pencil className="w-3.5 h-3.5" />
              Edit
            </Button>
            <Button size="sm" variant="danger" onClick={handleScheduleDeleted}>
              <Trash2 className="w-3.5 h-3.5" />
              Delete
            </Button>
          </div>
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-6 space-y-8">

        {/* Current on-call card */}
        <div className={`rounded-xl border-2 p-5 flex items-center gap-4 ${
          onCall?.is_override
            ? 'border-orange-200 bg-orange-50'
            : 'border-green-200 bg-green-50'
        }`}>
          <div className={`w-12 h-12 rounded-full flex items-center justify-center flex-shrink-0 ${
            currentUser ? '' : 'bg-gray-200'
          }`}
            style={currentUser ? { backgroundColor: segmentBg(currentUser) } : undefined}
          >
            <User className="w-6 h-6" style={currentUser ? { color: segmentText(currentUser) } : { color: '#9CA3AF' }} />
          </div>
          <div>
            <div className="text-xs font-semibold uppercase tracking-wider text-text-tertiary mb-0.5">
              Currently on call
            </div>
            {currentUser ? (
              <>
                <div className="text-xl font-bold text-text-primary">{currentUser}</div>
                {currentSegment && (
                  <div className="flex items-center gap-1 mt-1 text-xs text-text-secondary">
                    <Clock className="w-3 h-3 flex-shrink-0" />
                    <span>
                      Shift ends {timeUntil(currentSegment.end)}
                      {schedule.timezone !== Intl.DateTimeFormat().resolvedOptions().timeZone && (
                        <span className="text-text-tertiary ml-1">
                          ({new Date(currentSegment.end).toLocaleString(undefined, { timeZone: schedule.timezone, month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })} {schedule.timezone})
                        </span>
                      )}
                    </span>
                  </div>
                )}
                {onCall?.is_override && (
                  <div className="flex items-center gap-1 mt-1">
                    <AlertCircle className="w-3.5 h-3.5 text-orange-600" />
                    <span className="text-xs text-orange-700 font-medium">
                      {(() => {
                        const now = Date.now()
                        const activeOv = overrides.find(ov =>
                          new Date(ov.start_time).getTime() <= now && new Date(ov.end_time).getTime() >= now
                        )
                        return activeOv
                          ? `Active override · ends in ${timeUntil(activeOv.end_time)}`
                          : 'Active override'
                      })()}
                    </span>
                  </div>
                )}
                {(() => {
                  const todayStr = localDateStr(new Date())
                  const todayHoliday = holidays.find(h => h.date === todayStr)
                  if (!todayHoliday) return null
                  return (
                    <div className="mt-2">
                      <div className="inline-flex items-center gap-2 px-2 py-1 bg-amber-50 border border-amber-200 rounded-lg">
                        <span className="text-sm">{countryFlag(todayHoliday.country_code)}</span>
                        <span className="text-xs text-amber-800 font-medium">{todayHoliday.name}</span>
                        <button
                          onClick={() => { setOverridePrefilledStart(undefined); setOverrideModalOpen(true) }}
                          className="text-xs text-brand-primary hover:underline ml-1 font-medium"
                        >
                          Cover this shift?
                        </button>
                      </div>
                    </div>
                  )
                })()}
              </>
            ) : (
              <div className="text-base text-text-secondary italic">No one configured</div>
            )}
          </div>
        </div>

        {/* Shift calendar */}
        <GanttCalendar
          rows={ganttRows}
          windowStart={windowStart}
          days={GANTT_DAYS}
          onNavigate={setWindowStart}
          onDayClick={handleDayClick}
          holidays={holidays.map(h => ({ date: h.date, name: h.name, countryCode: h.country_code }))}
          scheduleTimezone={schedule.timezone}
        />

        {/* Overrides */}
        <OverridesTable
          scheduleId={schedule.id}
          overrides={overrides}
          onDeleted={handleOverrideDeleted}
          onAdd={() => { setOverridePrefilledStart(undefined); setOverrideModalOpen(true) }}
          toast={toast}
        />

        {/* Leave / Unavailability */}
        <UnavailabilitiesTable
          scheduleId={schedule.id}
          unavailabilities={unavailabilities}
          onDeleted={refetch}
          onAdd={() => setUnavailModalOpen(true)}
          toast={toast}
        />

        {/* Layers */}
        <div>
          <div className="flex items-center justify-between mb-3">
            <h2 className="text-sm font-semibold text-text-primary flex items-center gap-2">
              <Layers className="w-4 h-4 text-text-tertiary" />
              Rotation layers
            </h2>
            <Button size="sm" variant="secondary" onClick={() => setAddLayerOpen(true)}>
              <Plus className="w-3.5 h-3.5" />
              Add layer
            </Button>
          </div>
          {schedule.layers && schedule.layers.length > 0 ? (
            <div className="space-y-3">
              {schedule.layers.map((layer) => (
                <LayerCard
                  key={layer.id}
                  scheduleId={schedule.id}
                  layer={layer}
                  currentOnCallUser={currentUser}
                  onDeleted={invalidateAll}
                  onEdit={handleEditLayer}
                  toast={toast}
                />
              ))}
            </div>
          ) : (
            <div className="bg-gray-50 border border-border rounded-lg p-6 text-center">
              <Layers className="w-8 h-8 text-text-tertiary mx-auto mb-2" />
              <p className="text-sm text-text-secondary">No rotation layers yet.</p>
              <button
                onClick={() => setAddLayerOpen(true)}
                className="mt-2 text-sm text-brand-primary hover:underline"
              >
                Add your first layer
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
