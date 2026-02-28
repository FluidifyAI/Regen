import { useState, useEffect, useRef, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus } from 'lucide-react'
import { Button } from '../components/ui/Button'
import { SkeletonTable } from '../components/ui/Skeleton'
import { EmptyState } from '../components/ui/EmptyState'
import { GeneralError } from '../components/ui/ErrorState'
import { useSchedules } from '../hooks/useSchedules'
import { createSchedule, createLayer, getTimeline, COMMON_TIMEZONES } from '../api/schedules'
import type { CreateScheduleRequest, TimelineSegment } from '../api/types'
import { GanttCalendar, GanttRow, getMondayOf, getMonthStart, daysInMonth } from '../components/oncall/GanttCalendar'

// ─── useScheduleTimelines hook ────────────────────────────────────────────────

function useScheduleTimelines(
  scheduleIds: string[],
  windowStart: Date,
  days: number,
): Record<string, TimelineSegment[]> {
  const [data, setData] = useState<Record<string, TimelineSegment[]>>({})

  useEffect(() => {
    if (scheduleIds.length === 0) {
      setData({})
      return
    }
    let cancelled = false
    const from = windowStart.toISOString()
    const toDate = new Date(windowStart)
    toDate.setDate(toDate.getDate() + days)
    const to = toDate.toISOString()

    Promise.all(
      scheduleIds.map((id) =>
        getTimeline(id, from, to)
          .then((res) => ({ id, segments: res.segments }))
          .catch(() => ({ id, segments: [] as TimelineSegment[] })),
      ),
    ).then((results) => {
      if (cancelled) return
      const map: Record<string, TimelineSegment[]> = {}
      results.forEach(({ id, segments }) => {
        map[id] = segments
      })
      setData(map)
    })
    return () => { cancelled = true }
  }, [scheduleIds, windowStart, days])

  return data
}

// ─── computeLayerSegments (local preview, mirrors ScheduleDetailPage) ─────────

/**
 * Computes on-call segments for a single layer over [windowStart, windowStart+days).
 * Used for live preview in the create modal; does not apply schedule.timezone.
 */
function computeLayerSegments(
  layer: {
    shift_duration_seconds: number
    rotation_start: string
    participants?: Array<{ user_name: string; order_index: number }>
  },
  windowStart: Date,
  days: number,
): TimelineSegment[] {
  const participants = layer.participants ?? []
  if (participants.length === 0) return []

  const shiftMs = (layer.shift_duration_seconds || 86400) * 1000
  const rotationStart = new Date(layer.rotation_start).getTime()
  const sorted = [...participants].sort((a, b) => a.order_index - b.order_index)
  const segments: TimelineSegment[] = []

  for (let i = 0; i < days; i++) {
    const dayStart = new Date(windowStart)
    dayStart.setDate(dayStart.getDate() + i)
    dayStart.setHours(0, 0, 0, 0)
    const dayEnd = new Date(dayStart)
    dayEnd.setHours(23, 59, 59, 999)

    const elapsed = dayStart.getTime() - rotationStart
    const slotIndex = Math.floor(elapsed / shiftMs)
    const normalizedIndex =
      ((slotIndex % sorted.length) + sorted.length) % sorted.length

    segments.push({
      start: dayStart.toISOString(),
      end: dayEnd.toISOString(),
      user_name: sorted[normalizedIndex]?.user_name ?? '?',
      is_override: false,
    })
  }

  return segments
}

// ─── Layer form state ─────────────────────────────────────────────────────────

interface LayerFormState {
  name: string
  rotation_type: 'daily' | 'weekly' | 'custom'
  rotation_start: string
  shift_duration_seconds: number
  participants: string[]
}

const DEFAULT_LAYER_FORM: LayerFormState = {
  name: '',
  rotation_type: 'weekly',
  rotation_start: new Date().toISOString().split('T')[0] as string,
  shift_duration_seconds: 604800,
  participants: [],
}

const SHIFT_DURATION_MAP: Record<'daily' | 'weekly' | 'custom', number> = {
  daily: 86400,
  weekly: 604800,
  custom: 604800,
}

// ─── Create schedule modal ────────────────────────────────────────────────────

interface CreateScheduleModalProps {
  isOpen: boolean
  onClose: () => void
  onSaved: (id: string) => void
}

function CreateScheduleModal({ isOpen, onClose, onSaved }: CreateScheduleModalProps) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [timezone, setTimezone] = useState('UTC')
  const [layerForm, setLayerForm] = useState<LayerFormState>(DEFAULT_LAYER_FORM)
  const [previewWindowStart, setPreviewWindowStart] = useState<Date>(() => getMondayOf(new Date()))
  const [error, setError] = useState<string | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const nameRef = useRef<HTMLInputElement>(null)

  // Reset all form state when the modal opens
  useEffect(() => {
    if (isOpen) {
      setName('')
      setDescription('')
      setTimezone('UTC')
      setLayerForm({
        ...DEFAULT_LAYER_FORM,
        rotation_start: new Date().toISOString().split('T')[0] as string,
      })
      setPreviewWindowStart(getMondayOf(new Date()))
      setError(null)
      setTimeout(() => nameRef.current?.focus(), 50)
    }
  }, [isOpen])

  // Escape key closes modal
  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    if (isOpen) document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [isOpen, onClose])

  // Live preview: compute Gantt rows from current layer form state
  const previewRows: GanttRow[] = useMemo(() => {
    const filled = layerForm.participants.filter((p) => p.trim() !== '')
    if (filled.length === 0) return []
    const layerDef = {
      shift_duration_seconds: layerForm.shift_duration_seconds,
      rotation_start: layerForm.rotation_start + 'T00:00:00',
      participants: filled.map((name, i) => ({
        user_name: name,
        order_index: i,
      })),
    }
    return [
      {
        id: 'preview',
        label: layerForm.name || 'Layer 1',
        segments: computeLayerSegments(layerDef, getMondayOf(new Date()), 7),
      },
    ]
  }, [layerForm])

  if (!isOpen) return null

  const handleRotationTypeChange = (type: 'daily' | 'weekly' | 'custom') => {
    setLayerForm((f) => ({
      ...f,
      rotation_type: type,
      // For daily/weekly auto-update the duration; for custom preserve existing
      shift_duration_seconds:
        type === 'custom' ? f.shift_duration_seconds : SHIFT_DURATION_MAP[type],
    }))
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setIsSubmitting(true)
    try {
      const body: CreateScheduleRequest = { name, timezone }
      if (description) body.description = description
      const created = await createSchedule(body)

      const filledParticipants = layerForm.participants.filter((p) => p.trim() !== '')
      if (filledParticipants.length > 0) {
        await createLayer(created.id, {
          name: layerForm.name || 'Primary',
          rotation_type: layerForm.rotation_type,
          rotation_start: layerForm.rotation_start + 'T00:00:00',
          shift_duration_seconds: layerForm.shift_duration_seconds,
          participants: filledParticipants.map((user_name, i) => ({
            user_name,
            order_index: i,
          })),
        })
      }

      onSaved(created.id)
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create schedule')
    } finally {
      setIsSubmitting(false)
    }
  }

  const inputClass =
    'w-full px-3 py-2 border border-border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary focus:border-transparent disabled:opacity-50'
  const labelClass = 'block text-sm font-medium text-text-primary mb-1'
  const sectionHeadingClass = 'text-xs font-semibold uppercase tracking-wide text-text-tertiary mb-3'

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />
      <div
        className="relative z-10 w-full max-w-3xl bg-white rounded-xl shadow-xl mx-4 flex flex-col"
        style={{ maxHeight: '90vh' }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="px-6 py-4 border-b border-border flex-shrink-0">
          <h2 className="text-lg font-semibold text-text-primary">New on-call schedule</h2>
        </div>

        {/* Body: two columns */}
        <form onSubmit={handleSubmit} className="flex flex-col flex-1 min-h-0">
          <div className="flex flex-1 min-h-0 overflow-hidden">

            {/* Left panel — form fields */}
            <div className="w-80 flex-shrink-0 border-r border-border px-6 py-4 overflow-y-auto space-y-5">
              {error && (
                <div className="px-4 py-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-700">
                  {error}
                </div>
              )}

              {/* Schedule section */}
              <div>
                <p className={sectionHeadingClass}>Schedule</p>
                <div className="space-y-3">
                  <div>
                    <label className={labelClass} htmlFor="sched-name">Name</label>
                    <input
                      ref={nameRef}
                      id="sched-name"
                      type="text"
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                      placeholder="e.g. Primary Engineering On-call"
                      className={inputClass}
                      disabled={isSubmitting}
                      required
                    />
                  </div>
                  <div>
                    <label className={labelClass} htmlFor="sched-desc">Description</label>
                    <input
                      id="sched-desc"
                      type="text"
                      value={description}
                      onChange={(e) => setDescription(e.target.value)}
                      placeholder="Optional"
                      className={inputClass}
                      disabled={isSubmitting}
                    />
                  </div>
                  <div>
                    <label className={labelClass} htmlFor="sched-tz">Timezone</label>
                    <select
                      id="sched-tz"
                      value={timezone}
                      onChange={(e) => setTimezone(e.target.value)}
                      className={inputClass}
                      disabled={isSubmitting}
                    >
                      {COMMON_TIMEZONES.map((tz) => (
                        <option key={tz} value={tz}>{tz}</option>
                      ))}
                    </select>
                  </div>
                </div>
              </div>

              {/* First rotation section */}
              <div>
                <p className={sectionHeadingClass}>First rotation</p>
                <div className="space-y-3">
                  <div>
                    <label className={labelClass} htmlFor="layer-name">Layer name</label>
                    <input
                      id="layer-name"
                      type="text"
                      value={layerForm.name}
                      onChange={(e) => setLayerForm((f) => ({ ...f, name: e.target.value }))}
                      placeholder="e.g. Primary"
                      className={inputClass}
                      disabled={isSubmitting}
                    />
                  </div>
                  <div>
                    <label className={labelClass} htmlFor="layer-type">Rotation type</label>
                    <select
                      id="layer-type"
                      value={layerForm.rotation_type}
                      onChange={(e) =>
                        handleRotationTypeChange(e.target.value as 'daily' | 'weekly' | 'custom')
                      }
                      className={inputClass}
                      disabled={isSubmitting}
                    >
                      <option value="daily">Daily</option>
                      <option value="weekly">Weekly</option>
                      <option value="custom">Custom</option>
                    </select>
                  </div>
                  {layerForm.rotation_type === 'custom' && (
                    <div>
                      <label className={labelClass} htmlFor="layer-duration">
                        Shift duration (seconds)
                      </label>
                      <input
                        id="layer-duration"
                        type="number"
                        min={3600}
                        step={3600}
                        value={layerForm.shift_duration_seconds}
                        onChange={(e) =>
                          setLayerForm((f) => ({
                            ...f,
                            shift_duration_seconds: parseInt(e.target.value, 10) || 86400,
                          }))
                        }
                        className={inputClass}
                        disabled={isSubmitting}
                      />
                    </div>
                  )}
                  <div>
                    <label className={labelClass} htmlFor="layer-start">Rotation start</label>
                    <input
                      id="layer-start"
                      type="date"
                      value={layerForm.rotation_start}
                      onChange={(e) =>
                        setLayerForm((f) => ({ ...f, rotation_start: e.target.value }))
                      }
                      className={inputClass}
                      disabled={isSubmitting}
                    />
                  </div>
                  <div>
                    <label className={labelClass}>Participants</label>
                    <div className="space-y-2">
                      {layerForm.participants.map((p, i) => (
                        <div key={i} className="flex gap-2">
                          <input
                            type="text"
                            value={p}
                            onChange={(e) =>
                              setLayerForm((f) => {
                                const ps = [...f.participants]
                                ps[i] = e.target.value
                                return { ...f, participants: ps }
                              })
                            }
                            placeholder="Name"
                            className={inputClass}
                            disabled={isSubmitting}
                          />
                          <button
                            type="button"
                            onClick={() =>
                              setLayerForm((f) => ({
                                ...f,
                                participants: f.participants.filter((_, j) => j !== i),
                              }))
                            }
                            className="p-2 text-text-tertiary hover:text-red-500 text-lg leading-none"
                            disabled={isSubmitting}
                          >
                            ×
                          </button>
                        </div>
                      ))}
                      <button
                        type="button"
                        onClick={() =>
                          setLayerForm((f) => ({
                            ...f,
                            participants: [...f.participants, ''],
                          }))
                        }
                        className="text-xs text-brand-primary hover:underline"
                        disabled={isSubmitting}
                      >
                        + Add person
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            {/* Right panel — live Gantt preview */}
            <div className="flex-1 flex flex-col min-w-0 px-6 py-4 overflow-hidden">
              <p className="text-xs font-semibold uppercase tracking-wide text-text-tertiary mb-3">
                Preview
              </p>
              <div className="flex-1 overflow-auto">
                {previewRows.length === 0 ? (
                  <div className="flex items-center justify-center h-full">
                    <p className="text-sm text-text-tertiary">
                      Add participants to see a preview.
                    </p>
                  </div>
                ) : (
                  <GanttCalendar
                    rows={previewRows}
                    windowStart={previewWindowStart}
                    days={7}
                    onNavigate={setPreviewWindowStart}
                  />
                )}
              </div>
            </div>
          </div>

          {/* Footer */}
          <div className="px-6 py-4 border-t border-border flex justify-end gap-3 flex-shrink-0">
            <Button type="button" variant="secondary" onClick={onClose} disabled={isSubmitting}>
              Cancel
            </Button>
            <Button type="submit" variant="primary" disabled={isSubmitting}>
              {isSubmitting ? 'Creating…' : 'Create schedule'}
            </Button>
          </div>
        </form>
      </div>
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

/**
 * On-call schedules list page.
 * Shows all schedules in a Gantt calendar view.
 * Routes: GET /on-call
 */
export function SchedulesPage() {
  const navigate = useNavigate()
  const { schedules, loading, error, refetch } = useSchedules()
  const [modalOpen, setModalOpen] = useState(false)
  const [windowStart, setWindowStart] = useState<Date>(() => getMonthStart(new Date()))
  const GANTT_DAYS = daysInMonth(windowStart)

  const handleCreated = (id: string) => {
    navigate(`/on-call/${id}`)
  }

  const scheduleIds = useMemo(() => schedules.map((s) => s.id), [schedules])
  const timelines = useScheduleTimelines(scheduleIds, windowStart, GANTT_DAYS)

  const ganttRows: GanttRow[] = useMemo(
    () => schedules.map((s) => ({ id: s.id, label: s.name, segments: timelines[s.id] ?? [] })),
    [schedules, timelines],
  )

  return (
    <div className="flex flex-col h-full">
      <CreateScheduleModal
        isOpen={modalOpen}
        onClose={() => setModalOpen(false)}
        onSaved={handleCreated}
      />

      {/* Page Header */}
      <div className="border-b border-border bg-white px-6 py-4">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-text-primary">On-call</h1>
            <p className="mt-1 text-sm text-text-secondary">
              Manage on-call schedules, rotation layers, and overrides.
            </p>
          </div>
          <Button variant="primary" onClick={() => setModalOpen(true)}>
            <Plus className="w-4 h-4" />
            Add schedule
          </Button>
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-6">
        {loading ? (
          <SkeletonTable rows={4} />
        ) : error ? (
          <GeneralError message={error} onRetry={refetch} />
        ) : schedules.length === 0 ? (
          <EmptyState
            icon="clock"
            title="No on-call schedules"
            description="Create your first schedule to start managing on-call rotations."
            actionLabel="Add schedule"
            onAction={() => setModalOpen(true)}
          />
        ) : (
          <GanttCalendar
            rows={ganttRows}
            windowStart={windowStart}
            days={GANTT_DAYS}
            onNavigate={setWindowStart}
            onRowClick={(id) => navigate(`/on-call/${id}`)}
          />
        )}
      </div>
    </div>
  )
}
