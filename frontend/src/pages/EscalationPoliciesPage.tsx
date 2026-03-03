import { useState, useEffect, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus, GitBranch, Trash2, ChevronRight, ToggleLeft, ToggleRight, ChevronDown } from 'lucide-react'
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
      <div className="bg-surface-primary border border-border-subtle rounded-xl shadow-xl w-full max-w-md p-6">
        <h2 className="text-lg font-semibold text-text-primary mb-4">New Escalation Policy</h2>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-text-secondary mb-1">
              Name <span className="text-red-500">*</span>
            </label>
            <input
              className="w-full px-3 py-2 rounded-lg border border-border-subtle bg-surface-secondary text-text-primary text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              value={name}
              onChange={e => setName(e.target.value)}
              placeholder="e.g. Default On-call Policy"
              autoFocus
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-text-secondary mb-1">
              Description
            </label>
            <textarea
              className="w-full px-3 py-2 rounded-lg border border-border-subtle bg-surface-secondary text-text-primary text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none"
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
              {saving ? 'Creating…' : 'Create Policy'}
            </Button>
          </div>
        </form>
      </div>
    </div>
  )
}

// ─── Severity rules section ───────────────────────────────────────────────────

const SEVERITIES = ['critical', 'high', 'medium', 'low'] as const

const SEVERITY_COLORS: Record<string, string> = {
  critical: 'bg-red-100 text-red-700',
  high: 'bg-orange-100 text-orange-700',
  medium: 'bg-yellow-100 text-yellow-700',
  low: 'bg-blue-100 text-blue-700',
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

  const selectClass = 'flex-1 px-3 py-1.5 border border-border-subtle rounded-lg text-sm bg-surface-secondary text-text-primary appearance-none focus:outline-none focus:ring-2 focus:ring-brand-primary disabled:opacity-50'

  return (
    <div className="mt-8">
      <h2 className="text-base font-semibold text-text-primary mb-1">Auto-Escalation by Severity</h2>
      <p className="text-sm text-text-tertiary mb-4">
        When no routing rule specifies a policy, these rules fire based on alert severity.
      </p>
      <div className="border border-border-subtle rounded-xl overflow-hidden">
        {SEVERITIES.map(severity => {
          const rule = rules.find(r => r.severity === severity)
          return (
            <div key={severity} className="flex items-center gap-4 px-4 py-3 border-b border-border-subtle last:border-0">
              <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold uppercase tracking-wide w-20 justify-center ${SEVERITY_COLORS[severity]}`}>
                {severity}
              </span>
              <div className="relative flex-1">
                <select
                  className={selectClass}
                  value={rule?.escalation_policy_id ?? ''}
                  onChange={e => handleChange(severity, e.target.value)}
                  disabled={saving === severity}
                >
                  <option value="">— No auto-escalation —</option>
                  {policies.map(p => (
                    <option key={p.id} value={p.id}>{p.name}</option>
                  ))}
                </select>
                <ChevronDown className="absolute right-2.5 top-2 w-4 h-4 text-text-tertiary pointer-events-none" />
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}

// ─── Global fallback section ──────────────────────────────────────────────────

interface GlobalFallbackSectionProps {
  policies: EscalationPolicy[]
}

function GlobalFallbackSection({ policies }: GlobalFallbackSectionProps) {
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

  const selectClass = 'w-full px-3 py-2 pr-8 border border-border-subtle rounded-lg text-sm bg-surface-secondary text-text-primary appearance-none focus:outline-none focus:ring-2 focus:ring-brand-primary disabled:opacity-50'

  return (
    <div className="mt-8 border border-border-subtle rounded-xl p-4">
      <div className="flex items-center justify-between gap-6">
        <div className="flex-1">
          <h2 className="text-sm font-semibold text-text-primary">Global Fallback Policy</h2>
          <p className="text-xs text-text-tertiary mt-0.5">
            Used when no routing rule, severity rule, or schedule default applies.
          </p>
        </div>
        <div className="relative w-64 flex-shrink-0">
          <select
            className={selectClass}
            value={fallbackId}
            onChange={e => handleChange(e.target.value)}
            disabled={saving}
          >
            <option value="">— No fallback —</option>
            {policies.map(p => (
              <option key={p.id} value={p.id}>{p.name}</option>
            ))}
          </select>
          <ChevronDown className="absolute right-2.5 top-2.5 w-4 h-4 text-text-tertiary pointer-events-none" />
        </div>
      </div>
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
    if (!confirm('Delete this escalation policy? This cannot be undone.')) return
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
          <h1 className="text-2xl font-bold text-text-primary">Escalation Policies</h1>
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
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-text-primary">Escalation Policies</h1>
          <p className="text-sm text-text-tertiary mt-1">
            Define how alerts escalate when no one responds.
          </p>
        </div>
        <Button onClick={() => setShowCreate(true)}>
          <Plus className="w-4 h-4 mr-1" />
          New Policy
        </Button>
      </div>

      {policies.length === 0 ? (
        <div className="flex items-center justify-center min-h-[300px]">
          <div className="text-center max-w-md">
            <GitBranch className="w-12 h-12 mx-auto mb-4 text-text-tertiary" />
            <h3 className="text-lg font-semibold text-text-primary mb-2">No escalation policies</h3>
            <p className="text-sm text-text-secondary mb-6">
              Create a policy to define multi-step escalation chains for your on-call rotations.
            </p>
            <Button onClick={() => setShowCreate(true)}>
              <Plus className="w-4 h-4 mr-1" />
              New Policy
            </Button>
          </div>
        </div>
      ) : (
        <div className="border border-border-subtle rounded-xl overflow-hidden bg-surface-primary">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border-subtle bg-surface-secondary">
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
                  className="border-b border-border-subtle last:border-0 hover:bg-surface-secondary/50 cursor-pointer transition-colors"
                  onClick={() => navigate(`/escalation-policies/${policy.id}`)}
                >
                  <td className="px-4 py-3 font-medium text-text-primary flex items-center gap-2">
                    <GitBranch className="w-4 h-4 text-text-tertiary flex-shrink-0" />
                    {policy.name}
                  </td>
                  <td className="px-4 py-3 text-text-secondary truncate max-w-xs">
                    {policy.description || <span className="italic text-text-tertiary">—</span>}
                  </td>
                  <td className="px-4 py-3 text-text-secondary">
                    {policy.tiers.length} tier{policy.tiers.length !== 1 ? 's' : ''}
                  </td>
                  <td className="px-4 py-3">
                    <button
                      onClick={e => handleToggleEnabled(policy.id, policy.enabled, e)}
                      disabled={togglingId === policy.id}
                      className="flex items-center gap-1.5 text-sm disabled:opacity-50"
                      title={policy.enabled ? 'Click to disable' : 'Click to enable'}
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
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center justify-end gap-1">
                      <button
                        onClick={e => handleDelete(policy.id, e)}
                        disabled={deletingId === policy.id}
                        className="p-1.5 rounded hover:bg-red-50 text-text-tertiary hover:text-red-600 transition-colors disabled:opacity-50"
                        title="Delete policy"
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

      {/* Global fallback policy — always visible regardless of policy count */}
      <GlobalFallbackSection policies={policies} />

      {/* Severity-based auto-escalation rules */}
      <SeverityRulesSection policies={policies} />

      <CreatePolicyModal
        isOpen={showCreate}
        onClose={() => setShowCreate(false)}
        onSaved={id => {
          setShowCreate(false)
          navigate(`/escalation-policies/${id}`)
        }}
      />
    </div>
  )
}
