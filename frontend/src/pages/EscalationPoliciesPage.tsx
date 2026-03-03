import { useState, useEffect, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus, Siren, Trash2, ChevronRight, ToggleLeft, ToggleRight, ChevronDown } from 'lucide-react'
import { Button } from '../components/ui/Button'
import { SkeletonTable } from '../components/ui/Skeleton'
import { GeneralError } from '../components/ui/ErrorState'
import { useEscalationPolicies } from '../hooks/useEscalationPolicies'
import {
  createEscalationPolicy,
  deleteEscalationPolicy,
  updateEscalationPolicy,
  listSeverityRules,
  upsertSeverityRule,
  deleteSeverityRule,
  getEscalationSettings,
  updateEscalationSettings,
  type EscalationSeverityRule,
} from '../api/escalation'
import type { CreateEscalationPolicyRequest, EscalationPolicy } from '../api/types'

// ─── Create policy modal ──────────────────────────────────────────────────────

interface CreatePolicyModalProps {
  isOpen: boolean
  onClose: () => void
  onSaved: (id: string) => void
}

function CreatePolicyModal({ isOpen, onClose, onSaved }: CreatePolicyModalProps) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  if (!isOpen) return null

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!name.trim()) return
    setSaving(true)
    setError(null)
    try {
      const req: CreateEscalationPolicyRequest = { name: name.trim(), description: description.trim() || undefined }
      const policy = await createEscalationPolicy(req)
      setName('')
      setDescription('')
      onSaved(policy.id)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create policy')
    } finally {
      setSaving(false)
    }
  }

  function handleClose() {
    setName('')
    setDescription('')
    setError(null)
    onClose()
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="bg-surface-primary border border-border rounded-xl shadow-xl w-full max-w-md p-6">
        <h2 className="text-lg font-semibold text-text-primary mb-4">New Escalation Path</h2>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-text-secondary mb-1">
              Name <span className="text-red-500">*</span>
            </label>
            <input
              className="w-full px-3 py-2 rounded-lg border border-border bg-surface-secondary text-text-primary text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary"
              value={name}
              onChange={e => setName(e.target.value)}
              placeholder="e.g. Default On-call Path"
              autoFocus
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-text-secondary mb-1">
              Description
            </label>
            <textarea
              className="w-full px-3 py-2 rounded-lg border border-border bg-surface-secondary text-text-primary text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary resize-none"
              rows={3}
              value={description}
              onChange={e => setDescription(e.target.value)}
              placeholder="Optional description"
            />
          </div>
          {error && <p className="text-sm text-red-600">{error}</p>}
          <div className="flex justify-end gap-2 pt-2">
            <Button type="button" variant="ghost" onClick={handleClose} disabled={saving}>
              Cancel
            </Button>
            <Button type="submit" disabled={!name.trim() || saving}>
              {saving ? 'Creating…' : 'Create Path'}
            </Button>
          </div>
        </form>
      </div>
    </div>
  )
}

// ─── Global fallback banner (top of page) ────────────────────────────────────

interface GlobalFallbackBannerProps {
  policies: EscalationPolicy[]
}

function GlobalFallbackBanner({ policies }: GlobalFallbackBannerProps) {
  const [fallbackId, setFallbackId] = useState<string>('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    getEscalationSettings()
      .then(r => setFallbackId(r.global_fallback_policy_id ?? ''))
      .catch(() => {})
  }, [])

  async function handleChange(policyId: string) {
    setFallbackId(policyId)
    setSaving(true)
    try {
      await updateEscalationSettings(policyId || null)
    } catch {
      // ignore
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="flex items-center gap-4 px-4 py-3 border border-border rounded-xl bg-surface-secondary mb-6">
      <div className="flex-1 min-w-0">
        <p className="text-sm font-semibold text-text-primary">Global Fallback Path</p>
        <p className="text-xs text-text-tertiary mt-0.5">
          Used when no routing rule or severity rule matches an incoming alert.
        </p>
      </div>
      <div className="relative w-56 flex-shrink-0">
        <select
          className="w-full px-3 py-2 pr-8 border border-border rounded-lg text-sm bg-surface-primary text-text-primary appearance-none focus:outline-none focus:ring-2 focus:ring-brand-primary disabled:opacity-50 cursor-pointer"
          value={fallbackId}
          onChange={e => handleChange(e.target.value)}
          disabled={saving}
          aria-label="Global fallback escalation path"
        >
          <option value="">— No fallback —</option>
          {policies.map(p => (
            <option key={p.id} value={p.id}>{p.name}</option>
          ))}
        </select>
        <ChevronDown className="absolute right-2.5 top-2.5 w-4 h-4 text-text-tertiary pointer-events-none" />
      </div>
    </div>
  )
}

// ─── Severity rules section ───────────────────────────────────────────────────

const SEVERITIES = ['critical', 'high', 'medium', 'low'] as const

const SEVERITY_COLORS: Record<string, string> = {
  critical: 'bg-red-100 text-red-700',
  high:     'bg-orange-100 text-orange-700',
  medium:   'bg-yellow-100 text-yellow-700',
  low:      'bg-blue-100 text-blue-700',
}

interface SeverityRulesSectionProps {
  policies: EscalationPolicy[]
}

function SeverityRulesSection({ policies }: SeverityRulesSectionProps) {
  const [rules, setRules] = useState<EscalationSeverityRule[]>([])
  const [saving, setSaving] = useState<string | null>(null)

  const loadRules = useCallback(() => {
    listSeverityRules().then(r => setRules(r.data)).catch(() => {})
  }, [])

  useEffect(() => { loadRules() }, [loadRules])

  async function handleChange(severity: string, policyId: string) {
    setSaving(severity)
    try {
      if (policyId) {
        await upsertSeverityRule(severity, policyId)
      } else {
        await deleteSeverityRule(severity)
      }
      loadRules()
    } catch {
      // ignore
    } finally {
      setSaving(null)
    }
  }

  const selectClass = 'flex-1 px-3 py-2 border border-border rounded-lg text-sm bg-surface-secondary text-text-primary appearance-none focus:outline-none focus:ring-2 focus:ring-brand-primary disabled:opacity-50 cursor-pointer'

  return (
    <div className="mt-6">
      <h2 className="text-sm font-semibold text-text-primary mb-1">Auto-Escalation by Severity</h2>
      <p className="text-xs text-text-tertiary mb-3">
        When no routing rule specifies a path, these rules fire based on alert severity.
      </p>
      <div className="border border-border rounded-xl overflow-hidden">
        {SEVERITIES.map(severity => {
          const rule = rules.find(r => r.severity === severity)
          return (
            <div key={severity} className="flex items-center gap-4 px-4 py-3 border-b border-border last:border-0 bg-surface-primary">
              <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold uppercase tracking-wide w-20 justify-center ${SEVERITY_COLORS[severity]}`}>
                {severity}
              </span>
              <div className="relative flex-1">
                <select
                  className={selectClass}
                  value={rule?.escalation_policy_id ?? ''}
                  onChange={e => handleChange(severity, e.target.value)}
                  disabled={saving === severity}
                  aria-label={`Escalation path for ${severity} severity alerts`}
                >
                  <option value="">— No auto-escalation —</option>
                  {policies.map(p => (
                    <option key={p.id} value={p.id}>{p.name}</option>
                  ))}
                </select>
                <ChevronDown className="absolute right-2.5 top-2.5 w-4 h-4 text-text-tertiary pointer-events-none" />
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}

// ─── Tier step dots ───────────────────────────────────────────────────────────

function TierDots({ count }: { count: number }) {
  if (count === 0) {
    return <span className="text-xs text-text-tertiary italic">No tiers</span>
  }
  return (
    <div className="flex items-center gap-1.5" title={`${count} escalation tier${count !== 1 ? 's' : ''}`}>
      {Array.from({ length: Math.min(count, 6) }).map((_, i) => (
        <div key={i} className="w-2 h-2 rounded-full bg-brand-primary opacity-70" />
      ))}
      {count > 6 && <span className="text-xs text-text-tertiary">+{count - 6}</span>}
    </div>
  )
}

// ─── Main page ────────────────────────────────────────────────────────────────

export function EscalationPoliciesPage() {
  const navigate = useNavigate()
  const { policies, loading, error, refetch } = useEscalationPolicies()
  const [showCreate, setShowCreate] = useState(false)
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const [togglingId, setTogglingId] = useState<string | null>(null)

  async function handleDelete(id: string, e: React.MouseEvent) {
    e.stopPropagation()
    if (!confirm('Delete this escalation path? This cannot be undone.')) return
    setDeletingId(id)
    try {
      await deleteEscalationPolicy(id)
      await refetch()
    } catch {
      // ignore
    } finally {
      setDeletingId(null)
    }
  }

  async function handleToggleEnabled(id: string, currentEnabled: boolean, e: React.MouseEvent) {
    e.stopPropagation()
    setTogglingId(id)
    try {
      await updateEscalationPolicy(id, { enabled: !currentEnabled })
      await refetch()
    } catch {
      // ignore
    } finally {
      setTogglingId(null)
    }
  }

  if (loading) {
    return (
      <div className="p-6 max-w-5xl mx-auto">
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-2xl font-bold text-text-primary">Escalation Paths</h1>
        </div>
        <SkeletonTable rows={4} />
      </div>
    )
  }

  if (error) {
    return (
      <div className="p-6 max-w-5xl mx-auto">
        <GeneralError message={error} onRetry={refetch} />
      </div>
    )
  }

  return (
    <div className="p-6 max-w-5xl mx-auto">
      {/* Page header */}
      <div className="flex items-center justify-between mb-4">
        <div>
          <h1 className="text-2xl font-bold text-text-primary">Escalation Paths</h1>
          <p className="text-sm text-text-tertiary mt-1">
            Define how alerts escalate when no one responds.
          </p>
        </div>
        <Button onClick={() => setShowCreate(true)}>
          <Plus className="w-4 h-4 mr-1" />
          New Path
        </Button>
      </div>

      {/* Global fallback — prominent banner at top */}
      <GlobalFallbackBanner policies={policies} />

      {policies.length === 0 ? (
        <div className="flex items-center justify-center min-h-[240px]">
          <div className="text-center max-w-md">
            <Siren className="w-12 h-12 mx-auto mb-4 text-text-tertiary" />
            <h3 className="text-lg font-semibold text-text-primary mb-2">No escalation paths yet</h3>
            <p className="text-sm text-text-secondary mb-6">
              Create an escalation path to define multi-step paging chains for your on-call rotations.
            </p>
            <Button onClick={() => setShowCreate(true)}>
              <Plus className="w-4 h-4 mr-1" />
              New Path
            </Button>
          </div>
        </div>
      ) : (
        <div className="border border-border rounded-xl overflow-hidden bg-surface-primary">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border bg-surface-secondary">
                <th className="px-4 py-3 text-left font-medium text-text-secondary">Name</th>
                <th className="px-4 py-3 text-left font-medium text-text-secondary">Description</th>
                <th className="px-4 py-3 text-left font-medium text-text-secondary">Tiers</th>
                <th className="px-4 py-3 text-left font-medium text-text-secondary">Status</th>
                <th className="px-4 py-3" />
              </tr>
            </thead>
            <tbody>
              {policies.map(policy => (
                <tr
                  key={policy.id}
                  className="border-b border-border last:border-0 hover:bg-surface-secondary/50 cursor-pointer transition-colors"
                  onClick={() => navigate(`/on-call/escalation-paths/${policy.id}`)}
                >
                  <td className="px-4 py-3 font-medium text-text-primary">
                    <div className="flex items-center gap-2">
                      <Siren className="w-4 h-4 text-text-tertiary flex-shrink-0" />
                      {policy.name}
                    </div>
                  </td>
                  <td className="px-4 py-3 text-text-secondary truncate max-w-xs">
                    {policy.description || <span className="italic text-text-tertiary">—</span>}
                  </td>
                  <td className="px-4 py-3">
                    <TierDots count={policy.tiers.length} />
                  </td>
                  <td className="px-4 py-3">
                    <button
                      onClick={e => handleToggleEnabled(policy.id, policy.enabled, e)}
                      disabled={togglingId === policy.id}
                      aria-label={policy.enabled ? 'Disable escalation path' : 'Enable escalation path'}
                      className={`flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium border transition-colors disabled:opacity-50 ${
                        policy.enabled
                          ? 'bg-green-50 border-green-200 text-green-700 hover:bg-green-100'
                          : 'bg-surface-tertiary border-border text-text-tertiary hover:bg-surface-secondary'
                      }`}
                    >
                      {policy.enabled ? (
                        <ToggleRight className="w-3.5 h-3.5" />
                      ) : (
                        <ToggleLeft className="w-3.5 h-3.5" />
                      )}
                      {policy.enabled ? 'Enabled' : 'Disabled'}
                    </button>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center justify-end gap-1">
                      <button
                        onClick={e => handleDelete(policy.id, e)}
                        disabled={deletingId === policy.id}
                        aria-label="Delete escalation path"
                        className="p-2 rounded hover:bg-red-50 text-text-tertiary hover:text-red-600 transition-colors disabled:opacity-50"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                      <ChevronRight className="w-4 h-4 text-text-tertiary" />
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Severity-based auto-escalation rules */}
      <SeverityRulesSection policies={policies} />

      <CreatePolicyModal
        isOpen={showCreate}
        onClose={() => setShowCreate(false)}
        onSaved={id => {
          setShowCreate(false)
          navigate(`/on-call/escalation-paths/${id}`)
        }}
      />
    </div>
  )
}
