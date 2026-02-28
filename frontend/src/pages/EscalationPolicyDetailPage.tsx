import { useState } from 'react'
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
  ArrowDown,
  ToggleLeft,
  ToggleRight,
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
import type {
  EscalationTier,
  EscalationTargetType,
  CreateEscalationTierRequest,
  UpdateEscalationTierRequest,
} from '../api/types'

// ─── Tier flowchart ───────────────────────────────────────────────────────────

function targetTypeLabel(t: EscalationTargetType): string {
  if (t === 'schedule') return 'On-call schedule'
  if (t === 'users') return 'Specific users'
  return 'Schedule + users'
}

function targetTypeIcon(t: EscalationTargetType) {
  if (t === 'schedule') return <Calendar className="w-4 h-4 text-blue-500" />
  if (t === 'users') return <Users className="w-4 h-4 text-violet-500" />
  return <Users className="w-4 h-4 text-orange-500" />
}

function formatTimeout(seconds: number): string {
  if (seconds < 60) return `${seconds}s`
  if (seconds < 3600) return `${Math.round(seconds / 60)}m`
  return `${(seconds / 3600).toFixed(1)}h`
}

interface TierCardProps {
  tier: EscalationTier
  index: number
  onEdit: (tier: EscalationTier) => void
  onDelete: (id: string) => void
  deletingId: string | null
}

function TierCard({ tier, index, onEdit, onDelete, deletingId }: TierCardProps) {
  return (
    <div className="flex flex-col items-center">
      <div className="w-full max-w-md border border-border-subtle rounded-xl bg-surface-primary shadow-sm">
        <div className="flex items-start justify-between p-4">
          <div className="flex items-center gap-3">
            <div className="w-8 h-8 rounded-full bg-blue-100 text-blue-700 font-bold text-sm flex items-center justify-center flex-shrink-0">
              {index + 1}
            </div>
            <div>
              <div className="flex items-center gap-2 mb-1">
                {targetTypeIcon(tier.target_type)}
                <span className="text-sm font-medium text-text-primary">
                  {targetTypeLabel(tier.target_type)}
                </span>
              </div>
              {tier.target_type !== 'schedule' && tier.user_names.length > 0 && (
                <div className="flex flex-wrap gap-1 mt-1">
                  {tier.user_names.map(u => (
                    <span
                      key={u}
                      className="px-2 py-0.5 rounded bg-surface-secondary text-xs text-text-secondary"
                    >
                      @{u}
                    </span>
                  ))}
                </div>
              )}
              {tier.target_type !== 'users' && tier.schedule_id && (
                <span className="text-xs text-text-tertiary mt-1 block">
                  Schedule: {tier.schedule_id.slice(0, 8)}…
                </span>
              )}
            </div>
          </div>
          <div className="flex items-center gap-1">
            <button
              onClick={() => onEdit(tier)}
              className="p-1.5 rounded hover:bg-surface-secondary text-text-tertiary hover:text-text-primary transition-colors"
              title="Edit tier"
            >
              <Edit2 className="w-4 h-4" />
            </button>
            <button
              onClick={() => onDelete(tier.id)}
              disabled={deletingId === tier.id}
              className="p-1.5 rounded hover:bg-red-50 text-text-tertiary hover:text-red-600 transition-colors disabled:opacity-50"
              title="Delete tier"
            >
              <Trash2 className="w-4 h-4" />
            </button>
          </div>
        </div>
        <div className="border-t border-border-subtle px-4 py-2 bg-surface-secondary/50 rounded-b-xl flex items-center gap-2">
          <Clock className="w-3.5 h-3.5 text-text-tertiary" />
          <span className="text-xs text-text-tertiary">
            Wait <span className="font-medium text-text-secondary">{formatTimeout(tier.timeout_seconds)}</span> before escalating
          </span>
        </div>
      </div>
      <div className="flex flex-col items-center my-1 text-text-tertiary">
        <ArrowDown className="w-5 h-5" />
      </div>
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
}

function TierModal({ policyId, existing, isOpen, onClose, onSaved }: TierModalProps) {
  const [timeoutSeconds, setTimeoutSeconds] = useState(
    existing ? String(existing.timeout_seconds) : '300'
  )
  const [targetType, setTargetType] = useState<EscalationTargetType>(
    existing?.target_type ?? 'users'
  )
  const [userNamesRaw, setUserNamesRaw] = useState(
    existing?.user_names.join(', ') ?? ''
  )
  const [scheduleId, setScheduleId] = useState(existing?.schedule_id ?? '')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  if (!isOpen) return null

  const isEdit = !!existing

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const secs = parseInt(timeoutSeconds, 10)
    if (!secs || secs < 1) {
      setError('Timeout must be at least 1 second')
      return
    }
    setSaving(true)
    setError(null)
    const userNames = userNamesRaw
      .split(',')
      .map(s => s.trim())
      .filter(Boolean)
    try {
      if (isEdit) {
        const req: UpdateEscalationTierRequest = {
          timeout_seconds: secs,
          target_type: targetType,
          user_names: userNames,
          schedule_id: (targetType !== 'users' && scheduleId) ? scheduleId : undefined,
        }
        await updateEscalationTier(policyId, existing.id, req)
      } else {
        const req: CreateEscalationTierRequest = {
          timeout_seconds: secs,
          target_type: targetType,
          user_names: userNames,
          schedule_id: (targetType !== 'users' && scheduleId) ? scheduleId : undefined,
        }
        await createEscalationTier(policyId, req)
      }
      onSaved()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save tier')
    } finally {
      setSaving(false)
    }
  }

  function handleClose() {
    setError(null)
    onClose()
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="bg-surface-primary border border-border-subtle rounded-xl shadow-xl w-full max-w-md p-6">
        <h2 className="text-lg font-semibold text-text-primary mb-4">
          {isEdit ? 'Edit Tier' : 'Add Escalation Tier'}
        </h2>
        <form onSubmit={handleSubmit} className="space-y-4">
          {/* Target type */}
          <div>
            <label className="block text-sm font-medium text-text-secondary mb-1">
              Notify
            </label>
            <div className="relative">
              <select
                className="w-full px-3 py-2 pr-8 rounded-lg border border-border-subtle bg-surface-secondary text-text-primary text-sm appearance-none focus:outline-none focus:ring-2 focus:ring-blue-500"
                value={targetType}
                onChange={e => setTargetType(e.target.value as EscalationTargetType)}
              >
                <option value="users">Specific users</option>
                <option value="schedule">On-call schedule</option>
                <option value="both">Schedule + specific users</option>
              </select>
              <ChevronDown className="absolute right-2 top-2.5 w-4 h-4 text-text-tertiary pointer-events-none" />
            </div>
          </div>

          {/* User names */}
          {(targetType === 'users' || targetType === 'both') && (
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">
                Users <span className="text-text-tertiary font-normal">(comma-separated)</span>
              </label>
              <input
                className="w-full px-3 py-2 rounded-lg border border-border-subtle bg-surface-secondary text-text-primary text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                value={userNamesRaw}
                onChange={e => setUserNamesRaw(e.target.value)}
                placeholder="alice, bob, charlie"
              />
            </div>
          )}

          {/* Schedule ID */}
          {(targetType === 'schedule' || targetType === 'both') && (
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">
                Schedule ID
              </label>
              <input
                className="w-full px-3 py-2 rounded-lg border border-border-subtle bg-surface-secondary text-text-primary text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono"
                value={scheduleId}
                onChange={e => setScheduleId(e.target.value)}
                placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
              />
            </div>
          )}

          {/* Timeout */}
          <div>
            <label className="block text-sm font-medium text-text-secondary mb-1">
              Timeout (seconds) <span className="text-red-500">*</span>
            </label>
            <input
              type="number"
              min={1}
              className="w-full px-3 py-2 rounded-lg border border-border-subtle bg-surface-secondary text-text-primary text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              value={timeoutSeconds}
              onChange={e => setTimeoutSeconds(e.target.value)}
              placeholder="300"
            />
            <p className="text-xs text-text-tertiary mt-1">
              {timeoutSeconds && parseInt(timeoutSeconds) > 0
                ? `= ${formatTimeout(parseInt(timeoutSeconds))}`
                : ''}
            </p>
          </div>

          {error && <p className="text-sm text-red-600">{error}</p>}

          <div className="flex justify-end gap-2 pt-2">
            <Button type="button" variant="ghost" onClick={handleClose} disabled={saving}>
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

// ─── Main detail page ─────────────────────────────────────────────────────────

export function EscalationPolicyDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { policy, loading, error, refetch } = useEscalationPolicy(id!)
  const [showTierModal, setShowTierModal] = useState(false)
  const [editingTier, setEditingTier] = useState<EscalationTier | null>(null)
  const [deletingTierId, setDeletingTierId] = useState<string | null>(null)
  const [togglingEnabled, setTogglingEnabled] = useState(false)

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

  if (loading) {
    return (
      <div className="p-6 max-w-3xl mx-auto">
        <div className="mb-6">
          <button onClick={() => navigate(-1)} className="flex items-center gap-1 text-sm text-text-tertiary hover:text-text-primary mb-4">
            <ArrowLeft className="w-4 h-4" /> Back
          </button>
        </div>
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
      {/* Header */}
      <button
        onClick={() => navigate('/escalation-policies')}
        className="flex items-center gap-1 text-sm text-text-tertiary hover:text-text-primary mb-4 transition-colors"
      >
        <ArrowLeft className="w-4 h-4" /> Escalation Policies
      </button>

      <div className="flex items-start justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-text-primary">{policy.name}</h1>
          {policy.description && (
            <p className="text-sm text-text-tertiary mt-1">{policy.description}</p>
          )}
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={handleToggleEnabled}
            disabled={togglingEnabled}
            className="flex items-center gap-1.5 text-sm px-3 py-1.5 rounded-lg border border-border-subtle hover:bg-surface-secondary transition-colors disabled:opacity-50"
          >
            {policy.enabled ? (
              <>
                <ToggleRight className="w-4 h-4 text-brand-primary" />
                <span className="text-brand-primary font-medium">Enabled</span>
              </>
            ) : (
              <>
                <ToggleLeft className="w-4 h-4 text-text-tertiary" />
                <span className="text-text-tertiary">Disabled</span>
              </>
            )}
          </button>
        </div>
      </div>

      {/* Escalation flowchart */}
      <div className="mb-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-base font-semibold text-text-primary">Escalation Tiers</h2>
          <Button
            onClick={() => { setEditingTier(null); setShowTierModal(true) }}
            variant="ghost"
            className="text-sm"
          >
            <Plus className="w-4 h-4 mr-1" />
            Add Tier
          </Button>
        </div>

        {sortedTiers.length === 0 ? (
          <div className="border border-dashed border-border-subtle rounded-xl p-8 text-center">
            <p className="text-text-tertiary text-sm mb-3">No escalation tiers yet.</p>
            <Button onClick={() => { setEditingTier(null); setShowTierModal(true) }}>
              <Plus className="w-4 h-4 mr-1" />
              Add First Tier
            </Button>
          </div>
        ) : (
          <div className="flex flex-col items-center">
            {/* Alert trigger node */}
            <div className="w-full max-w-md border border-orange-200 bg-orange-50 rounded-xl px-4 py-3 text-center mb-1">
              <span className="text-sm font-medium text-orange-700">Alert Triggered</span>
            </div>
            <div className="flex flex-col items-center my-1 text-text-tertiary">
              <ArrowDown className="w-5 h-5" />
            </div>

            {sortedTiers.map((tier, index) => (
              <TierCard
                key={tier.id}
                tier={tier}
                index={index}
                onEdit={t => { setEditingTier(t); setShowTierModal(true) }}
                onDelete={handleDeleteTier}
                deletingId={deletingTierId}
              />
            ))}

            {/* Final fallback node */}
            <div className="w-full max-w-md border border-red-200 bg-red-50 rounded-xl px-4 py-3 text-center">
              <span className="text-sm font-medium text-red-700">Unacknowledged — incident escalation exhausted</span>
            </div>
          </div>
        )}
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
      />
    </div>
  )
}
