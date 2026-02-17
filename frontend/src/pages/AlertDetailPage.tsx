import { useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useEffect, useCallback } from 'react'
import {
  ArrowLeft,
  CheckCircle2,
  Clock,
  AlertTriangle,
  ExternalLink,
  GitBranch,
} from 'lucide-react'
import { Button } from '../components/ui/Button'
import { SkeletonTable } from '../components/ui/Skeleton'
import { GeneralError } from '../components/ui/ErrorState'
import { getAlert, acknowledgeAlert } from '../api/alerts'
import type { Alert } from '../api/types'

// ─── Acknowledgment status badge ──────────────────────────────────────────────

function AckStatusBadge({ status }: { status?: string }) {
  if (!status || status === 'pending') {
    return (
      <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-yellow-50 text-yellow-700 border border-yellow-200">
        <Clock className="w-3 h-3" />
        Pending
      </span>
    )
  }
  if (status === 'acknowledged') {
    return (
      <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-green-50 text-green-700 border border-green-200">
        <CheckCircle2 className="w-3 h-3" />
        Acknowledged
      </span>
    )
  }
  return (
    <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-blue-50 text-blue-700 border border-blue-200">
      <CheckCircle2 className="w-3 h-3" />
      Completed
    </span>
  )
}

// ─── Acknowledge modal ────────────────────────────────────────────────────────

interface AckModalProps {
  alertId: string
  isOpen: boolean
  onClose: () => void
  onAcknowledged: () => void
}

function AcknowledgeModal({ alertId, isOpen, onClose, onAcknowledged }: AckModalProps) {
  const [userName, setUserName] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  if (!isOpen) return null

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!userName.trim()) return
    setSaving(true)
    setError(null)
    try {
      await acknowledgeAlert(alertId, userName.trim(), 'api')
      onAcknowledged()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to acknowledge alert')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="bg-surface-primary border border-border-subtle rounded-xl shadow-xl w-full max-w-sm p-6">
        <h2 className="text-lg font-semibold text-text-primary mb-4">Acknowledge Alert</h2>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-text-secondary mb-1">
              Your name <span className="text-red-500">*</span>
            </label>
            <input
              className="w-full px-3 py-2 rounded-lg border border-border-subtle bg-surface-secondary text-text-primary text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              value={userName}
              onChange={e => setUserName(e.target.value)}
              placeholder="e.g. alice"
              autoFocus
            />
          </div>
          {error && <p className="text-sm text-red-600">{error}</p>}
          <div className="flex justify-end gap-2 pt-1">
            <Button type="button" variant="ghost" onClick={onClose} disabled={saving}>
              Cancel
            </Button>
            <Button type="submit" disabled={!userName.trim() || saving}>
              {saving ? 'Acknowledging…' : 'Acknowledge'}
            </Button>
          </div>
        </form>
      </div>
    </div>
  )
}

// ─── Main page ────────────────────────────────────────────────────────────────

export function AlertDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [alert, setAlert] = useState<Alert | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showAckModal, setShowAckModal] = useState(false)

  const fetchAlert = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const data = await getAlert(id!)
      setAlert(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch alert')
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => { fetchAlert() }, [fetchAlert])

  if (loading) {
    return (
      <div className="p-6 max-w-3xl mx-auto">
        <button onClick={() => navigate(-1)} className="flex items-center gap-1 text-sm text-text-tertiary hover:text-text-primary mb-4">
          <ArrowLeft className="w-4 h-4" /> Back
        </button>
        <SkeletonTable rows={4} />
      </div>
    )
  }

  if (error || !alert) {
    return (
      <div className="p-6 max-w-3xl mx-auto">
        <GeneralError message={error ?? 'Alert not found'} onRetry={fetchAlert} />
      </div>
    )
  }

  const isAcknowledged = alert.acknowledgment_status === 'acknowledged' || alert.acknowledgment_status === 'completed'
  const canAcknowledge = !!alert.escalation_policy_id && !isAcknowledged

  return (
    <div className="p-6 max-w-3xl mx-auto">
      <button
        onClick={() => navigate(-1)}
        className="flex items-center gap-1 text-sm text-text-tertiary hover:text-text-primary mb-4 transition-colors"
      >
        <ArrowLeft className="w-4 h-4" /> Back
      </button>

      {/* Header */}
      <div className="flex items-start justify-between mb-6">
        <div className="flex items-start gap-3">
          <AlertTriangle className={`w-6 h-6 mt-0.5 flex-shrink-0 ${
            alert.severity === 'critical' ? 'text-red-500' :
            alert.severity === 'warning' ? 'text-yellow-500' : 'text-blue-400'
          }`} />
          <div>
            <h1 className="text-xl font-bold text-text-primary">{alert.title}</h1>
            <p className="text-sm text-text-tertiary mt-1">
              {alert.source} · {alert.severity}
            </p>
          </div>
        </div>
        {canAcknowledge && (
          <Button onClick={() => setShowAckModal(true)}>
            <CheckCircle2 className="w-4 h-4 mr-1" />
            Acknowledge
          </Button>
        )}
      </div>

      {/* Escalation status card */}
      {alert.escalation_policy_id && (
        <div className="border border-border-subtle rounded-xl bg-surface-primary p-4 mb-4">
          <div className="flex items-center gap-2 mb-3">
            <GitBranch className="w-4 h-4 text-text-tertiary" />
            <h2 className="text-sm font-semibold text-text-primary">Escalation Status</h2>
          </div>
          <div className="grid grid-cols-2 gap-3 text-sm">
            <div>
              <span className="text-text-tertiary text-xs">Status</span>
              <div className="mt-1">
                <AckStatusBadge status={alert.acknowledgment_status} />
              </div>
            </div>
            {alert.acknowledged_by && (
              <div>
                <span className="text-text-tertiary text-xs">Acknowledged by</span>
                <p className="mt-1 font-medium text-text-primary">@{alert.acknowledged_by}</p>
              </div>
            )}
            {alert.acknowledged_at && (
              <div>
                <span className="text-text-tertiary text-xs">Acknowledged at</span>
                <p className="mt-1 text-text-secondary">
                  {new Date(alert.acknowledged_at).toLocaleString()}
                </p>
              </div>
            )}
            {alert.acknowledged_via && (
              <div>
                <span className="text-text-tertiary text-xs">Via</span>
                <p className="mt-1 text-text-secondary capitalize">{alert.acknowledged_via}</p>
              </div>
            )}
            <div className="col-span-2">
              <span className="text-text-tertiary text-xs">Policy</span>
              <div className="mt-1">
                <Link
                  to={`/escalation-policies/${alert.escalation_policy_id}`}
                  className="text-blue-600 hover:underline text-sm inline-flex items-center gap-1"
                >
                  View policy
                  <ExternalLink className="w-3 h-3" />
                </Link>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Alert metadata */}
      <div className="border border-border-subtle rounded-xl bg-surface-primary p-4 mb-4">
        <h2 className="text-sm font-semibold text-text-primary mb-3">Details</h2>
        <dl className="grid grid-cols-2 gap-3 text-sm">
          <div>
            <dt className="text-text-tertiary text-xs">Status</dt>
            <dd className="mt-0.5 font-medium text-text-primary capitalize">{alert.status}</dd>
          </div>
          <div>
            <dt className="text-text-tertiary text-xs">Source</dt>
            <dd className="mt-0.5 text-text-secondary">{alert.source}</dd>
          </div>
          <div>
            <dt className="text-text-tertiary text-xs">Started</dt>
            <dd className="mt-0.5 text-text-secondary">{new Date(alert.started_at).toLocaleString()}</dd>
          </div>
          {alert.ended_at && (
            <div>
              <dt className="text-text-tertiary text-xs">Ended</dt>
              <dd className="mt-0.5 text-text-secondary">{new Date(alert.ended_at).toLocaleString()}</dd>
            </div>
          )}
          <div>
            <dt className="text-text-tertiary text-xs">Received</dt>
            <dd className="mt-0.5 text-text-secondary">{new Date(alert.received_at).toLocaleString()}</dd>
          </div>
          <div>
            <dt className="text-text-tertiary text-xs">External ID</dt>
            <dd className="mt-0.5 text-text-secondary font-mono text-xs truncate">{alert.external_id}</dd>
          </div>
        </dl>
        {alert.description && (
          <div className="mt-3 pt-3 border-t border-border-subtle">
            <dt className="text-text-tertiary text-xs mb-1">Description</dt>
            <dd className="text-sm text-text-secondary">{alert.description}</dd>
          </div>
        )}
      </div>

      {/* Labels */}
      {Object.keys(alert.labels ?? {}).length > 0 && (
        <div className="border border-border-subtle rounded-xl bg-surface-primary p-4">
          <h2 className="text-sm font-semibold text-text-primary mb-3">Labels</h2>
          <div className="flex flex-wrap gap-2">
            {Object.entries(alert.labels).map(([k, v]) => (
              <span
                key={k}
                className="px-2 py-0.5 rounded bg-surface-secondary text-xs text-text-secondary font-mono"
              >
                {k}={v}
              </span>
            ))}
          </div>
        </div>
      )}

      <AcknowledgeModal
        alertId={id!}
        isOpen={showAckModal}
        onClose={() => setShowAckModal(false)}
        onAcknowledged={async () => {
          setShowAckModal(false)
          await fetchAlert()
        }}
      />
    </div>
  )
}
