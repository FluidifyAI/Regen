import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import {
  ArrowLeft,
  Plus,
  Trash2,
  Edit2,
  Clock,
  Users,
  Calendar,
  ChevronDown,
  ToggleLeft,
  ToggleRight,
  Bell,
} from 'lucide-react'
import { Button } from '../components/ui/Button'
import { SkeletonTable } from '../components/ui/Skeleton'
import { GeneralError } from '../components/ui/ErrorState'
import { useEscalationPolicy } from '../hooks/useEscalationPolicy'
import {
  createEscalationTier,
  updateEscalationTier,
  deleteEscalationTier,
  updateEscalationPolicy,
} from '../api/escalation'
import { listSchedules } from '../api/schedules'
import { listUsers } from '../api/users'
import type { UserSummary } from '../api/users'
import type {
  EscalationTier,
  EscalationTargetType,
  CreateEscalationTierRequest,
  UpdateEscalationTierRequest,
  Schedule,
} from '../api/types'

function formatTimeout(seconds: number): string {
  if (seconds < 60) return `${seconds}s`
  if (seconds < 3600) return `${Math.round(seconds / 60)}m`
  return `${(seconds / 3600).toFixed(1)}h`
}

// ─── Tier card ────────────────────────────────────────────────────────────────

interface TierCardProps {
  tier: EscalationTier
  index: number
  schedules: Schedule[]
  onEdit: (tier: EscalationTier) => void
  onDelete: (id: string) => void
  deletingId: string | null
}

function TierCard({ tier, index, schedules, onEdit, onDelete, deletingId }: TierCardProps) {
  const scheduleName = schedules.find(s => s.id === tier.schedule_id)?.name

  return (
    <div className="w-full border border-border rounded-xl bg-surface-primary shadow-sm overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-2.5 border-b border-border">
        <div className="flex items-center gap-2">
          <div className="w-6 h-6 rounded-full bg-brand-primary-light flex items-center justify-center">
            <Bell className="w-3.5 h-3.5 text-brand-primary" />
          </div>
          <span className="text-xs font-semibold text-text-tertiary uppercase tracking-wide">
            Level {index + 1}
          </span>
          <span className="text-text-tertiary text-xs">·</span>
          <span className="text-sm font-medium text-text-primary">Notify</span>
        </div>
        <div className="flex items-center gap-1">
          <button
            onClick={() => onEdit(tier)}
            className="p-2 rounded hover:bg-surface-secondary text-text-tertiary hover:text-text-primary transition-colors"
            aria-label="Edit tier"
          >
            <Edit2 className="w-3.5 h-3.5" />
          </button>
          <button
            onClick={() => onDelete(tier.id)}
            disabled={deletingId === tier.id}
            className="p-2 rounded hover:bg-red-50 text-text-tertiary hover:text-red-600 transition-colors disabled:opacity-50"
            aria-label="Delete tier"
          >
            <Trash2 className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {/* Targets */}
      <div className="px-4 py-3 space-y-2">
        {(tier.target_type === 'schedule' || tier.target_type === 'both') && (
          <div className="flex items-center gap-2">
            <Calendar className="w-3.5 h-3.5 text-blue-500 flex-shrink-0" />
            <span className="text-sm text-text-primary">
              {scheduleName ?? tier.schedule_id?.slice(0, 8) + '…'}
            </span>
          </div>
        )}
        {(tier.target_type === 'users' || tier.target_type === 'both') &&
          tier.user_names.length > 0 && (
            <div className="flex items-start gap-2">
              <Users className="w-3.5 h-3.5 text-violet-500 flex-shrink-0 mt-0.5" />
              <div className="flex flex-wrap gap-1">
                {tier.user_names.map(u => (
                  <span
                    key={u}
                    className="px-2 py-0.5 rounded-full bg-violet-50 border border-violet-100 text-xs text-violet-700 font-medium"
                  >
                    {u}
                  </span>
                ))}
              </div>
            </div>
          )}
      </div>

      {/* Footer: timeout chip */}
      <div className="px-4 py-2 bg-surface-secondary/40 border-t border-border flex items-center gap-1.5">
        <Clock className="w-3.5 h-3.5 text-text-tertiary" />
        <span className="text-xs text-text-tertiary">
          Wait{' '}
          <span className="font-semibold text-text-secondary">
            {formatTimeout(tier.timeout_seconds)}
          </span>{' '}
          before escalating
        </span>
      </div>
    </div>
  )
}

// ─── Flow connector with + button ────────────────────────────────────────────

function FlowConnector({ onAdd }: { onAdd: () => void }) {
  return (
    <div className="flex flex-col items-center py-1">
      <div className="w-0.5 h-5 bg-border-strong" />
      <button
        onClick={onAdd}
        className="flex items-center gap-1.5 px-3 py-1.5 rounded-full border border-dashed border-border hover:border-brand-primary hover:bg-brand-primary-light/40 text-text-tertiary hover:text-brand-primary transition-all text-xs font-medium min-h-[36px]"
        aria-label="Add escalation tier"
      >
        <Plus className="w-3.5 h-3.5 flex-shrink-0" />
        <span>Add tier</span>
      </button>
      <div className="w-0.5 h-5 bg-border-strong" />
    </div>
  )
}

// ─── Tier modal (create + edit) ───────────────────────────────────────────────

interface TierModalProps {
  policyId: string
  existing?: EscalationTier
  isOpen: boolean
  onClose: () => void
  onSaved: () => void
  users: UserSummary[]
  schedules: Schedule[]
}

function TierModal({
  policyId, existing, isOpen, onClose, onSaved, users, schedules,
}: TierModalProps) {
  const [timeoutSeconds, setTimeoutSeconds] = useState(
    existing ? String(existing.timeout_seconds) : '300'
  )
  const [scheduleId, setScheduleId] = useState(existing?.schedule_id ?? '')
  // Each row holds one user email; empty string = unset slot
  const [userRows, setUserRows] = useState<string[]>(
    existing?.user_names?.length ? existing.user_names : []
  )
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Reset whenever modal opens/switches between create and edit
  useEffect(() => {
    if (isOpen) {
      setTimeoutSeconds(existing ? String(existing.timeout_seconds) : '300')
      setScheduleId(existing?.schedule_id ?? '')
      setUserRows(existing?.user_names?.length ? existing.user_names : [])
      setError(null)
    }
  }, [isOpen, existing])

  if (!isOpen) return null

  const isEdit = !!existing

  // Derive target_type from what the user has selected — no explicit dropdown needed
  const validUsers = userRows.filter(Boolean)
  const hasSchedule = Boolean(scheduleId)
  const hasUsers = validUsers.length > 0
  const derivedTargetType: EscalationTargetType =
    hasSchedule && hasUsers ? 'both' : hasSchedule ? 'schedule' : 'users'

  function addUserRow() {
    setUserRows(rows => [...rows, ''])
  }
  function removeUserRow(i: number) {
    setUserRows(rows => rows.filter((_, idx) => idx !== i))
  }
  function updateUserRow(i: number, val: string) {
    setUserRows(rows => rows.map((r, idx) => (idx === i ? val : r)))
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const secs = parseInt(timeoutSeconds, 10)
    if (!secs || secs < 1) {
      setError('Timeout must be at least 1 second')
      return
    }
    if (!hasSchedule && !hasUsers) {
      setError('Select an on-call schedule or add at least one user to notify')
      return
    }
    setSaving(true)
    setError(null)
    try {
      const base = {
        timeout_seconds: secs,
        target_type: derivedTargetType,
        user_names: validUsers,
        schedule_id: hasSchedule ? scheduleId : undefined,
      }
      if (isEdit) {
        await updateEscalationTier(policyId, existing.id, base as UpdateEscalationTierRequest)
      } else {
        await createEscalationTier(policyId, base as CreateEscalationTierRequest)
      }
      onSaved()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save tier')
    } finally {
      setSaving(false)
    }
  }

  const inputCls =
    'w-full px-3 py-2 border border-border rounded-lg text-sm bg-surface-secondary text-text-primary focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent disabled:opacity-50'
  const labelCls = 'flex items-center gap-1.5 text-sm font-medium text-text-secondary mb-1.5'

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div
        className="bg-surface-primary border border-border rounded-xl shadow-xl w-full max-w-md mx-4"
        onClick={e => e.stopPropagation()}
      >
        {/* Modal header */}
        <div className="px-6 py-4 border-b border-border">
          <h2 className="text-lg font-semibold text-text-primary">
            {isEdit ? 'Edit Escalation Tier' : 'Add Escalation Tier'}
          </h2>
          <p className="text-xs text-text-tertiary mt-0.5">
            Notify a schedule, specific users, or both when this tier is reached.
          </p>
        </div>

        <form onSubmit={handleSubmit}>
          <div className="px-6 py-5 space-y-5 max-h-[65vh] overflow-y-auto">

            {/* On-call schedule */}
            <div>
              <label className={labelCls}>
                <Calendar className="w-3.5 h-3.5 text-blue-500" />
                On-call schedule
                <span className="ml-1 text-text-tertiary font-normal text-xs">(optional)</span>
              </label>
              <div className="relative">
                <select
                  className={`${inputCls} pr-8 appearance-none`}
                  value={scheduleId}
                  onChange={e => setScheduleId(e.target.value)}
                  disabled={saving}
                >
                  <option value="">— No schedule —</option>
                  {schedules.map(s => (
                    <option key={s.id} value={s.id}>{s.name}</option>
                  ))}
                </select>
                <ChevronDown className="absolute right-2.5 top-2.5 w-4 h-4 text-text-tertiary pointer-events-none" />
              </div>
            </div>

            {/* Specific users */}
            <div>
              <label className={labelCls}>
                <Users className="w-3.5 h-3.5 text-violet-500" />
                Specific users
                <span className="ml-1 text-text-tertiary font-normal text-xs">(optional)</span>
              </label>
              <div className="space-y-2">
                {userRows.map((row, i) => (
                  <div key={i} className="flex items-center gap-2">
                    {users.length > 0 ? (
                      <select
                        value={row}
                        onChange={e => updateUserRow(i, e.target.value)}
                        className={`${inputCls} flex-1`}
                        disabled={saving}
                      >
                        <option value="">— Select user —</option>
                        {users.map(u => (
                          <option key={u.id} value={u.email}>
                            {u.name} ({u.email})
                          </option>
                        ))}
                      </select>
                    ) : (
                      <input
                        type="text"
                        value={row}
                        onChange={e => updateUserRow(i, e.target.value)}
                        placeholder="User email"
                        className={`${inputCls} flex-1`}
                        disabled={saving}
                      />
                    )}
                    <button
                      type="button"
                      onClick={() => removeUserRow(i)}
                      className="p-1.5 text-text-tertiary hover:text-red-600 hover:bg-red-50 rounded transition-colors"
                      disabled={saving}
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>
                ))}
              </div>
              <button
                type="button"
                onClick={addUserRow}
                className="mt-2 text-sm text-brand-primary hover:text-brand-primary-hover font-medium transition-colors"
                disabled={saving}
              >
                + Add user
              </button>
            </div>

            {/* Timeout */}
            <div>
              <label className={labelCls}>
                <Clock className="w-3.5 h-3.5 text-text-tertiary" />
                Timeout <span className="text-red-500 ml-0.5">*</span>
              </label>
              <input
                type="number"
                min={1}
                className={inputCls}
                value={timeoutSeconds}
                onChange={e => setTimeoutSeconds(e.target.value)}
                placeholder="300"
                disabled={saving}
              />
              {timeoutSeconds && parseInt(timeoutSeconds) > 0 && (
                <p className="text-xs text-text-tertiary mt-1">
                  = {formatTimeout(parseInt(timeoutSeconds))} before escalating to next tier
                </p>
              )}
            </div>

            {error && (
              <div className="px-3 py-2 bg-red-50 border border-red-200 rounded-lg text-sm text-red-700">
                {error}
              </div>
            )}
          </div>

          <div className="px-6 py-4 border-t border-border flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={onClose} disabled={saving}>
              Cancel
            </Button>
            <Button type="submit" disabled={saving}>
              {saving ? 'Saving…' : isEdit ? 'Save Changes' : 'Add Tier'}
            </Button>
          </div>
        </form>
      </div>
    </div>
  )
}

// ─── Main page ────────────────────────────────────────────────────────────────

export function EscalationPolicyDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { policy, loading, error, refetch } = useEscalationPolicy(id!)
  const [showTierModal, setShowTierModal] = useState(false)
  const [editingTier, setEditingTier] = useState<EscalationTier | null>(null)
  const [deletingTierId, setDeletingTierId] = useState<string | null>(null)
  const [togglingEnabled, setTogglingEnabled] = useState(false)
  const [users, setUsers] = useState<UserSummary[]>([])
  const [schedules, setSchedules] = useState<Schedule[]>([])

  // Pre-load users and schedules so selectors are ready when the modal opens
  useEffect(() => {
    listUsers().then(setUsers).catch(() => {})
    listSchedules().then(r => setSchedules(r.data)).catch(() => {})
  }, [])

  async function handleDeleteTier(tierId: string) {
    if (!confirm('Remove this escalation tier?')) return
    setDeletingTierId(tierId)
    try {
      await deleteEscalationTier(id!, tierId)
      await refetch()
    } catch {
      // ignore
    } finally {
      setDeletingTierId(null)
    }
  }

  async function handleToggleEnabled() {
    if (!policy) return
    setTogglingEnabled(true)
    try {
      await updateEscalationPolicy(id!, { enabled: !policy.enabled })
      await refetch()
    } catch {
      // ignore
    } finally {
      setTogglingEnabled(false)
    }
  }

  function openAddTier() {
    setEditingTier(null)
    setShowTierModal(true)
  }

  if (loading) {
    return (
      <div className="p-6 max-w-3xl mx-auto">
        <button
          onClick={() => navigate(-1)}
          className="flex items-center gap-1 text-sm text-text-tertiary hover:text-text-primary mb-6"
        >
          <ArrowLeft className="w-4 h-4" /> Back
        </button>
        <SkeletonTable rows={3} />
      </div>
    )
  }

  if (error || !policy) {
    return (
      <div className="p-6 max-w-3xl mx-auto">
        <GeneralError message={error ?? 'Policy not found'} onRetry={refetch} />
      </div>
    )
  }

  const sortedTiers = [...policy.tiers].sort((a, b) => a.tier_index - b.tier_index)

  return (
    <div className="p-6 max-w-3xl mx-auto">
      {/* Back */}
      <button
        onClick={() => navigate('/on-call/escalation-paths')}
        className="flex items-center gap-1 text-sm text-text-tertiary hover:text-text-primary mb-5 transition-colors"
      >
        <ArrowLeft className="w-4 h-4" /> Escalation Paths
      </button>

      {/* Header */}
      <div className="flex items-start justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-text-primary">{policy.name}</h1>
          {policy.description && (
            <p className="text-sm text-text-tertiary mt-1 max-w-lg">{policy.description}</p>
          )}
        </div>
        <button
          onClick={handleToggleEnabled}
          disabled={togglingEnabled}
          className="flex items-center gap-1.5 text-sm px-3 py-1.5 rounded-lg border border-border hover:bg-surface-secondary transition-colors disabled:opacity-50 flex-shrink-0 ml-4"
        >
          {policy.enabled ? (
            <>
              <ToggleRight className="w-4 h-4 text-blue-600" />
              <span className="text-blue-600 font-medium">Enabled</span>
            </>
          ) : (
            <>
              <ToggleLeft className="w-4 h-4 text-text-tertiary" />
              <span className="text-text-tertiary">Disabled</span>
            </>
          )}
        </button>
      </div>

      {/* Flow */}
      <div>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-sm font-semibold text-text-primary uppercase tracking-wide">
            Escalation Path
          </h2>
          <Button onClick={openAddTier} variant="ghost" className="text-sm">
            <Plus className="w-4 h-4 mr-1" /> Add Tier
          </Button>
        </div>

        <div className="flex flex-col items-center">
          {/* Start node */}
          <div className="w-full border border-amber-200 bg-amber-50 rounded-xl px-4 py-3 text-center">
            <span className="text-sm font-medium text-amber-700">Alert Triggered</span>
          </div>

          {sortedTiers.length === 0 ? (
            <>
              <div className="w-0.5 h-8 bg-border-strong my-1" />
              <button
                onClick={openAddTier}
                className="w-full border border-dashed border-border rounded-xl p-5 text-center hover:border-blue-400 hover:bg-blue-50/30 transition-colors group"
              >
                <Plus className="w-5 h-5 text-text-tertiary group-hover:text-blue-500 mx-auto mb-1 transition-colors" />
                <span className="text-sm text-text-tertiary group-hover:text-blue-600 transition-colors">
                  Add first escalation tier
                </span>
              </button>
              <div className="w-0.5 h-8 bg-border-strong my-1" />
            </>
          ) : (
            sortedTiers.map((tier, index) => (
              <div key={tier.id} className="w-full flex flex-col items-center">
                <FlowConnector onAdd={openAddTier} />
                <TierCard
                  tier={tier}
                  index={index}
                  schedules={schedules}
                  onEdit={t => { setEditingTier(t); setShowTierModal(true) }}
                  onDelete={handleDeleteTier}
                  deletingId={deletingTierId}
                />
              </div>
            ))
          )}

          {/* End connector + exhausted node */}
          {sortedTiers.length > 0 && (
            <FlowConnector onAdd={openAddTier} />
          )}
          <div className="w-full border border-red-200 bg-red-50 rounded-xl px-4 py-3 text-center">
            <span className="text-sm font-medium text-red-600">
              Unacknowledged — escalation exhausted
            </span>
          </div>
        </div>
      </div>

      <TierModal
        policyId={id!}
        existing={editingTier ?? undefined}
        isOpen={showTierModal}
        onClose={() => { setShowTierModal(false); setEditingTier(null) }}
        onSaved={async () => {
          setShowTierModal(false)
          setEditingTier(null)
          await refetch()
        }}
        users={users}
        schedules={schedules}
      />
    </div>
  )
}
