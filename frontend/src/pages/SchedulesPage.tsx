import { useState, useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus, Calendar, Trash2, ChevronRight } from 'lucide-react'
import { Button } from '../components/ui/Button'
import { SkeletonTable } from '../components/ui/Skeleton'
import { EmptyState } from '../components/ui/EmptyState'
import { GeneralError } from '../components/ui/ErrorState'
import { useSchedules } from '../hooks/useSchedules'
import { createSchedule, deleteSchedule, getOnCall, COMMON_TIMEZONES } from '../api/schedules'
import type { OnCallResponse, CreateScheduleRequest } from '../api/types'

// ─── On-call badge (per-row async fetch) ─────────────────────────────────────

function OnCallBadge({ scheduleId }: { scheduleId: string }) {
  const [onCall, setOnCall] = useState<OnCallResponse | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    setLoading(true)
    getOnCall(scheduleId)
      .then(setOnCall)
      .catch(() => setOnCall(null))
      .finally(() => setLoading(false))
  }, [scheduleId])

  if (loading) {
    return <span className="text-xs text-text-tertiary">…</span>
  }

  if (!onCall || !onCall.user_name) {
    return <span className="text-xs text-text-tertiary italic">No one configured</span>
  }

  return (
    <span
      className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium ${
        onCall.is_override
          ? 'bg-orange-50 text-orange-700'
          : 'bg-green-50 text-green-700'
      }`}
    >
      <span className="w-1.5 h-1.5 rounded-full bg-current" />
      {onCall.user_name}
      {onCall.is_override && <span className="opacity-75">(override)</span>}
    </span>
  )
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
 * Shows all schedules with current on-call user badge per row.
 * Routes: GET /on-call
 */
export function SchedulesPage() {
  const navigate = useNavigate()
  const { schedules, loading, error, refetch } = useSchedules()
  const [modalOpen, setModalOpen] = useState(false)
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const [deleteError, setDeleteError] = useState<string | null>(null)

  const handleCreated = (id: string) => {
    navigate(`/on-call/${id}`)
  }

  const handleDelete = async (id: string, name: string, hasLayers: boolean) => {
    const warning = hasLayers
      ? `"${name}" has active layers. Deleting it will remove all rotation data. Continue?`
      : `Delete schedule "${name}"? This cannot be undone.`
    if (!confirm(warning)) return
    setDeletingId(id)
    setDeleteError(null)
    try {
      await deleteSchedule(id)
      await refetch()
    } catch (err) {
      setDeleteError(err instanceof Error ? err.message : 'Failed to delete schedule')
    } finally {
      setDeletingId(null)
    }
  }

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
        {deleteError && (
          <div className="mb-4 px-4 py-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-700">
            {deleteError}
          </div>
        )}

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
          <div className="bg-white border border-border rounded-lg overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border bg-gray-50">
                  <th className="text-left px-4 py-3 text-xs font-medium text-text-tertiary uppercase tracking-wider">Name</th>
                  <th className="text-left px-4 py-3 text-xs font-medium text-text-tertiary uppercase tracking-wider">Timezone</th>
                  <th className="text-left px-4 py-3 text-xs font-medium text-text-tertiary uppercase tracking-wider">Layers</th>
                  <th className="text-left px-4 py-3 text-xs font-medium text-text-tertiary uppercase tracking-wider">Currently on call</th>
                  <th className="w-24 px-4 py-3" />
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {schedules.map((s) => (
                  <tr
                    key={s.id}
                    className="hover:bg-gray-50 transition-colors cursor-pointer"
                    onClick={() => navigate(`/on-call/${s.id}`)}
                  >
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <Calendar className="w-4 h-4 text-text-tertiary flex-shrink-0" />
                        <span className="font-medium text-text-primary">{s.name}</span>
                      </div>
                      {s.description && (
                        <div className="text-xs text-text-tertiary mt-0.5 ml-6">{s.description}</div>
                      )}
                    </td>
                    <td className="px-4 py-3 text-text-secondary font-mono text-xs">{s.timezone}</td>
                    <td className="px-4 py-3 text-text-secondary">
                      {s.layers ? s.layers.length : '—'}
                    </td>
                    <td className="px-4 py-3" onClick={(e) => e.stopPropagation()}>
                      <OnCallBadge scheduleId={s.id} />
                    </td>
                    <td className="px-4 py-3" onClick={(e) => e.stopPropagation()}>
                      <div className="flex items-center justify-end gap-1">
                        <button
                          onClick={() => navigate(`/on-call/${s.id}`)}
                          className="p-1.5 text-text-tertiary hover:text-text-primary hover:bg-gray-100 rounded transition-colors"
                          title="View schedule"
                        >
                          <ChevronRight className="w-4 h-4" />
                        </button>
                        <button
                          onClick={() => handleDelete(s.id, s.name, (s.layers?.length ?? 0) > 0)}
                          disabled={deletingId === s.id}
                          className="p-1.5 text-text-tertiary hover:text-red-600 hover:bg-red-50 rounded transition-colors disabled:opacity-50"
                          title="Delete schedule"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}
