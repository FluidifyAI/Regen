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
  getLayerTimelines,
  COMMON_TIMEZONES,
} from '../api/schedules'
import { listUsers } from '../api/users'
import type { UserSummary } from '../api/users'
import { listEscalationPolicies } from '../api/escalation'
import type {
  Schedule,
  ScheduleLayer,
  ScheduleOverride,
  UpdateScheduleRequest,
  CreateLayerRequest,
  UpdateLayerRequest,
  LayerTimelinesResponse,
  CreateOverrideRequest,
  EscalationPolicy,
} from '../api/types'

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
        className="relative z-10 w-full max-w-md bg-surface-primary rounded-xl shadow-xl mx-4"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="px-6 py-4 border-b border-border">
          <h2 className="text-lg font-semibold text-text-primary">Edit schedule</h2>
        </div>
        <form onSubmit={handleSubmit}>
          <div className="px-6 py-4 space-y-4">
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
  onClose: () => void
  onSaved: () => void
}

function AddLayerModal({ isOpen, scheduleId, nextOrderIndex, onClose, onSaved }: AddLayerModalProps) {
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
      // Default rotation_start: midnight UTC today
      const now = new Date()
      const midnight = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()))
      setRotationStart(midnight.toISOString().slice(0, 16))
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
              <label className={labelClass} htmlFor="layer-start">Rotation start (UTC)</label>
              <input id="layer-start" type="datetime-local" value={rotationStart} onChange={(e) => setRotationStart(e.target.value)} className={inputClass} disabled={isSubmitting} />
              <p className="mt-1 text-xs text-text-tertiary">The point in time when the rotation begins counting from slot 0.</p>
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
  onClose: () => void
  onSaved: () => void
}

function EditLayerModal({ isOpen, scheduleId, layer, onClose, onSaved }: EditLayerModalProps) {
  const rotationStartStr = (() => {
    const d = new Date(layer.rotation_start)
    return isNaN(d.getTime()) ? '' : d.toISOString().slice(0, 16)
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
      setRotationStart(isNaN(d.getTime()) ? '' : d.toISOString().slice(0, 16))
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
              <label className={labelClass} htmlFor="edit-layer-start">Rotation start (UTC)</label>
              <input id="edit-layer-start" type="datetime-local" value={rotationStart} onChange={(e) => setRotationStart(e.target.value)} className={inputClass} disabled={isSubmitting} />
              <p className="mt-1 text-xs text-text-tertiary">The point in time when the rotation begins counting from slot 0.</p>
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
  onClose: () => void
  onSaved: () => void
}

function OverrideModal({ isOpen, scheduleId, onClose, onSaved }: OverrideModalProps) {
  const [overrideUser, setOverrideUser] = useState('')
  const [users, setUsers] = useState<UserSummary[]>([])
  const [startTime, setStartTime] = useState('')
  const [endTime, setEndTime] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)

  useEffect(() => {
    if (isOpen) {
      setOverrideUser('')
      // Default start = now (rounded to nearest UTC hour), end = +1 day (UTC)
      const n = new Date()
      const startUTC = new Date(Date.UTC(n.getUTCFullYear(), n.getUTCMonth(), n.getUTCDate(), n.getUTCHours()))
      const endUTC = new Date(startUTC.getTime() + 86_400_000)
      setStartTime(startUTC.toISOString().slice(0, 16))
      setEndTime(endUTC.toISOString().slice(0, 16))
      setError(null)
      listUsers().then(setUsers).catch(() => {})
    }
  }, [isOpen])

  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    if (isOpen) document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [isOpen, onClose])

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
                <label className={labelClass} htmlFor="ov-start">Start (UTC)</label>
                <input id="ov-start" type="datetime-local" value={startTime} onChange={(e) => setStartTime(e.target.value)} className={inputClass} disabled={isSubmitting} required />
              </div>
              <div className="flex-1">
                <label className={labelClass} htmlFor="ov-end">End (UTC)</label>
                <input id="ov-end" type="datetime-local" value={endTime} onChange={(e) => setEndTime(e.target.value)} className={inputClass} disabled={isSubmitting} required />
              </div>
            </div>
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

  const handleDelete = async (ov: ScheduleOverride) => {
    if (!confirm(`Delete override for "${ov.override_user}"?`)) return
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
        <p className="text-sm text-text-tertiary italic">No upcoming overrides.</p>
      ) : (
        <div className="bg-white border border-border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border bg-gray-50">
                <th className="text-left px-4 py-2 text-xs font-medium text-text-tertiary uppercase tracking-wider">On-call user</th>
                <th className="text-left px-4 py-2 text-xs font-medium text-text-tertiary uppercase tracking-wider">Start</th>
                <th className="text-left px-4 py-2 text-xs font-medium text-text-tertiary uppercase tracking-wider">End</th>
                <th className="text-left px-4 py-2 text-xs font-medium text-text-tertiary uppercase tracking-wider">Created by</th>
                <th className="w-12 px-4 py-2" />
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {overrides.map((ov) => (
                <tr key={ov.id} className="hover:bg-gray-50">
                  <td className="px-4 py-2.5 font-medium text-text-primary">{ov.override_user}</td>
                  <td className="px-4 py-2.5 text-text-secondary text-xs">{fmt(ov.start_time)}</td>
                  <td className="px-4 py-2.5 text-text-secondary text-xs">{fmt(ov.end_time)}</td>
                  <td className="px-4 py-2.5 text-text-tertiary text-xs">{ov.created_by || '—'}</td>
                  <td className="px-4 py-2.5">
                    <button
                      onClick={() => handleDelete(ov)}
                      disabled={deletingId === ov.id}
                      className="p-1.5 text-text-tertiary hover:text-red-600 hover:bg-red-50 rounded transition-colors disabled:opacity-50"
                      title="Delete override"
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </td>
                </tr>
              ))}
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

  const { schedule, onCall, overrides, loading, error, refetch } = useSchedule(id!)

  const [editModalOpen, setEditModalOpen] = useState(false)
  const [addLayerOpen, setAddLayerOpen] = useState(false)
  const [overrideModalOpen, setOverrideModalOpen] = useState(false)
  const [editLayerOpen, setEditLayerOpen] = useState(false)
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

  const ganttRows: GanttRow[] = useMemo(
    () =>
      [...(schedule?.layers ?? [])]
        .sort((a, b) => a.order_index - b.order_index)
        .map((layer) => ({
          id: layer.id,
          label: layer.name,
          segments: layerTimelines?.layers[layer.id] ?? [],
        })),
    [schedule?.layers, layerTimelines],
  )

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
          onClose={() => setAddLayerOpen(false)}
          onSaved={() => { toast.success('Layer added'); invalidateAll() }}
        />
      )}

      {editLayerOpen && editingLayer && (
        <EditLayerModal
          isOpen={editLayerOpen}
          scheduleId={schedule.id}
          layer={editingLayer}
          onClose={() => { setEditLayerOpen(false); setEditingLayer(null) }}
          onSaved={() => { toast.success('Layer updated'); invalidateAll() }}
        />
      )}

      {overrideModalOpen && (
        <OverrideModal
          isOpen={overrideModalOpen}
          scheduleId={schedule.id}
          onClose={() => setOverrideModalOpen(false)}
          onSaved={handleOverrideSaved}
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
                {onCall?.is_override && (
                  <div className="flex items-center gap-1 mt-1">
                    <AlertCircle className="w-3.5 h-3.5 text-orange-600" />
                    <span className="text-xs text-orange-700 font-medium">Active override</span>
                  </div>
                )}
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
        />

        {/* Overrides */}
        <OverridesTable
          scheduleId={schedule.id}
          overrides={overrides}
          onDeleted={handleOverrideDeleted}
          onAdd={() => setOverrideModalOpen(true)}
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
