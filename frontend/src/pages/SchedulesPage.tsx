import { useState, useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus } from 'lucide-react'
import { Button } from '../components/ui/Button'
import { SkeletonTable } from '../components/ui/Skeleton'
import { EmptyState } from '../components/ui/EmptyState'
import { GeneralError } from '../components/ui/ErrorState'
import { useSchedules } from '../hooks/useSchedules'
import { createSchedule, getTimeline, COMMON_TIMEZONES } from '../api/schedules'
import type { CreateScheduleRequest, TimelineSegment } from '../api/types'
import { GanttCalendar, GanttRow, getMondayOf } from '../components/oncall/GanttCalendar'

// ─── Constants ────────────────────────────────────────────────────────────────

const GANTT_DAYS = 7

// ─── useScheduleTimelines hook ────────────────────────────────────────────────

function useScheduleTimelines(
  scheduleIds: string[],
  windowStart: Date,
): Record<string, TimelineSegment[]> {
  const [data, setData] = useState<Record<string, TimelineSegment[]>>({})

  useEffect(() => {
    if (scheduleIds.length === 0) {
      setData({})
      return
    }
    const from = windowStart.toISOString()
    const toDate = new Date(windowStart)
    toDate.setDate(toDate.getDate() + GANTT_DAYS)
    const to = toDate.toISOString()

    Promise.all(
      scheduleIds.map((id) =>
        getTimeline(id, from, to)
          .then((res) => ({ id, segments: res.segments }))
          .catch(() => ({ id, segments: [] as TimelineSegment[] })),
      ),
    ).then((results) => {
      const map: Record<string, TimelineSegment[]> = {}
      results.forEach(({ id, segments }) => {
        map[id] = segments
      })
      setData(map)
    })
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [scheduleIds.join(','), windowStart.toISOString()])

  return data
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
  const [error, setError] = useState<string | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const nameRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (isOpen) {
      setName('')
      setDescription('')
      setTimezone('UTC')
      setError(null)
      setTimeout(() => nameRef.current?.focus(), 50)
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
    setIsSubmitting(true)
    try {
      const body: CreateScheduleRequest = { name, timezone }
      if (description) body.description = description
      const created = await createSchedule(body)
      onSaved(created.id)
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create schedule')
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
        className="relative z-10 w-full max-w-md bg-white rounded-xl shadow-xl mx-4"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="px-6 py-4 border-b border-border">
          <h2 className="text-lg font-semibold text-text-primary">New on-call schedule</h2>
        </div>
        <form onSubmit={handleSubmit}>
          <div className="px-6 py-4 space-y-4">
            {error && (
              <div className="px-4 py-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-700">
                {error}
              </div>
            )}
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
                placeholder="Optional description"
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
          <div className="px-6 py-4 border-t border-border flex justify-end gap-3">
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
  const [windowStart, setWindowStart] = useState<Date>(() => getMondayOf(new Date()))

  const handleCreated = (id: string) => {
    navigate(`/on-call/${id}`)
  }

  const scheduleIds = schedules.map((s) => s.id)
  const timelines = useScheduleTimelines(scheduleIds, windowStart)

  const ganttRows: GanttRow[] = schedules.map((s) => ({
    id: s.id,
    label: s.name,
    segments: timelines[s.id] ?? [],
  }))

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
