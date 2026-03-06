import { useState, useEffect, useRef } from 'react'
import { Plus, Pencil, Trash2, ChevronDown, Zap, Filter, Bell, GripVertical } from 'lucide-react'
import { Button } from '../components/ui/Button'
import { SkeletonTable } from '../components/ui/Skeleton'
import { EmptyState } from '../components/ui/EmptyState'
import { GeneralError } from '../components/ui/ErrorState'
import { useRoutingRules } from '../hooks/useRoutingRules'
import { createRoutingRule, updateRoutingRule, deleteRoutingRule, reorderRoutingRules } from '../api/routing_rules'
import { listEscalationPolicies } from '../api/escalation'
import type { RoutingRule, CreateRoutingRuleRequest, EscalationPolicy } from '../api/types'

// ─── Constants ────────────────────────────────────────────────────────────────

const SOURCES = [
  { id: 'prometheus', label: 'Prometheus' },
  { id: 'grafana',    label: 'Grafana'    },
  { id: 'cloudwatch', label: 'CloudWatch' },
  { id: 'generic',    label: 'Generic'    },
] as const

const SEVERITIES = ['critical', 'high', 'warning', 'info', 'low'] as const

const SEVERITY_COLORS: Record<string, string> = {
  critical: 'bg-red-100 text-red-700 border-red-200',
  high:     'bg-orange-100 text-orange-700 border-orange-200',
  warning:  'bg-yellow-100 text-yellow-700 border-yellow-200',
  info:     'bg-blue-100 text-blue-700 border-blue-200',
  low:      'bg-gray-100 text-gray-600 border-gray-200',
}

// ─── Yes/No toggle ────────────────────────────────────────────────────────────

function YesNo({
  value, onChange, disabled,
}: { value: boolean; onChange: (v: boolean) => void; disabled?: boolean }) {
  return (
    <div className="flex rounded-lg border border-border overflow-hidden text-sm">
      <button
        type="button"
        onClick={() => onChange(true)}
        disabled={disabled}
        className={`px-3 py-1.5 font-medium transition-colors ${
          value
            ? 'bg-brand-primary text-white'
            : 'bg-surface-primary text-text-secondary hover:bg-surface-secondary'
        }`}
      >
        Yes
      </button>
      <button
        type="button"
        onClick={() => onChange(false)}
        disabled={disabled}
        className={`px-3 py-1.5 font-medium transition-colors border-l border-border ${
          !value
            ? 'bg-brand-primary text-white'
            : 'bg-surface-primary text-text-secondary hover:bg-surface-secondary'
        }`}
      >
        No
      </button>
    </div>
  )
}

// ─── Route builder modal ──────────────────────────────────────────────────────

interface RouteBuilderModalProps {
  isOpen: boolean
  rule: RoutingRule | null
  onClose: () => void
  onSaved: () => void
}

function RouteBuilderModal({ isOpen, rule, onClose, onSaved }: RouteBuilderModalProps) {
  const [name, setName]               = useState('')
  const [enabled, setEnabled]         = useState(true)
  const [sources, setSources]         = useState<string[]>([])
  const [severities, setSeverities]   = useState<string[]>([])
  const [createIncident, setCreate]   = useState(true)
  const [suppress, setSuppress]       = useState(false)
  const [escalate, setEscalate]       = useState(false)
  const [escalationPolicyId, setEpId] = useState('')
  const [severityOverride, setSevOvr] = useState('')

  const [policies, setPolicies] = useState<EscalationPolicy[]>([])
  const [error, setError]       = useState<string | null>(null)
  const [saving, setSaving]     = useState(false)
  const nameRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (isOpen) listEscalationPolicies().then((r) => setPolicies(r.data)).catch(() => {})
  }, [isOpen])

  useEffect(() => {
    if (!isOpen) return
    if (rule) {
      setName(rule.name)
      setEnabled(rule.enabled)
      const mc = rule.match_criteria as Record<string, unknown>
      setSources(Array.isArray(mc.source) ? (mc.source as string[]) : [])
      setSeverities(Array.isArray(mc.severity) ? (mc.severity as string[]) : [])
      const ac = rule.actions as Record<string, unknown>
      setCreate(ac.create_incident !== false)
      setSuppress(Boolean(ac.suppress))
      setEscalate(Boolean(ac.escalation_policy_id))
      setEpId(typeof ac.escalation_policy_id === 'string' ? ac.escalation_policy_id : '')
      setSevOvr(typeof ac.severity_override === 'string' ? ac.severity_override : '')
    } else {
      setName(''); setEnabled(true); setSources([]); setSeverities([])
      setCreate(true); setSuppress(false); setEscalate(false); setEpId(''); setSevOvr('')
    }
    setError(null)
  }, [rule, isOpen])

  useEffect(() => {
    if (isOpen) setTimeout(() => nameRef.current?.focus(), 50)
  }, [isOpen])

  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    if (isOpen) document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [isOpen, onClose])

  if (!isOpen) return null

  const toggleSource   = (id: string) => setSources((p) => p.includes(id) ? p.filter((s) => s !== id) : [...p, id])
  const toggleSeverity = (s: string)  => setSeverities((p) => p.includes(s) ? p.filter((x) => x !== s) : [...p, s])

  const handleCreateChange   = (v: boolean) => { setCreate(v); if (v) setSuppress(false) }
  const handleSuppressChange = (v: boolean) => { setSuppress(v); if (v) setCreate(false) }
  const handleEscalateChange = (v: boolean) => { setEscalate(v); if (!v) setEpId('') }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return

    const match_criteria: Record<string, unknown> = {}
    if (sources.length > 0)    match_criteria.source   = sources
    if (severities.length > 0) match_criteria.severity = severities

    const actions: Record<string, unknown> = {}
    if (createIncident)                     actions.create_incident    = true
    if (suppress)                           actions.suppress           = true
    if (escalate && escalationPolicyId)     actions.escalation_policy_id = escalationPolicyId
    if (severityOverride)                   actions.severity_override  = severityOverride

    setSaving(true); setError(null)
    try {
      if (rule) {
        await updateRoutingRule(rule.id, { name: name.trim(), enabled, match_criteria, actions })
      } else {
        const body: CreateRoutingRuleRequest = { name: name.trim(), description: '', priority: 0, enabled, match_criteria, actions }
        await createRoutingRule(body)
      }
      onSaved(); onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save route')
    } finally {
      setSaving(false)
    }
  }

  const sectionClass = 'bg-surface-primary border border-border rounded-xl p-5 space-y-4'
  const labelClass   = 'text-xs font-semibold uppercase tracking-wide text-text-tertiary'

  return (
    <div className="fixed inset-0 z-50 flex items-end sm:items-center justify-center">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />
      <div
        className="relative z-10 w-full max-w-lg bg-surface-secondary rounded-t-2xl sm:rounded-2xl shadow-2xl mx-0 sm:mx-4 flex flex-col max-h-[92vh]"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="px-6 py-4 border-b border-border bg-surface-primary rounded-t-2xl flex-shrink-0">
          <div className="flex items-center justify-between">
            <h2 className="text-base font-semibold text-text-primary">
              {rule ? 'Edit alert route' : 'New alert route'}
            </h2>
            <label className="flex items-center gap-2 text-sm text-text-secondary cursor-pointer select-none">
              <input
                type="checkbox"
                checked={enabled}
                onChange={(e) => setEnabled(e.target.checked)}
                className="h-4 w-4 rounded border-border text-brand-primary focus:ring-brand-primary"
              />
              Enabled
            </label>
          </div>
        </div>

        <form onSubmit={handleSubmit} className="flex flex-col flex-1 overflow-hidden">
          <div className="flex-1 overflow-y-auto px-5 py-4 space-y-3">
            {error && (
              <div className="px-4 py-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-700">{error}</div>
            )}

            {/* Name */}
            <div className={sectionClass}>
              <div>
                <label className={`${labelClass} block mb-1.5`}>Route name</label>
                <input
                  ref={nameRef}
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="e.g. Critical SRE alerts"
                  required
                  className="w-full px-3 py-2 border border-border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary focus:border-transparent"
                />
              </div>
            </div>

            {/* Sources */}
            <div className={sectionClass}>
              <div className="flex items-center gap-2">
                <div className="w-6 h-6 rounded-full bg-brand-primary/10 flex items-center justify-center flex-shrink-0">
                  <Zap className="w-3.5 h-3.5 text-brand-primary" />
                </div>
                <span className="text-sm font-semibold text-text-primary">Alert sources</span>
                <span className="text-xs text-text-tertiary ml-auto">empty = all sources</span>
              </div>
              <div className="grid grid-cols-2 gap-2">
                {SOURCES.map((src) => (
                  <label
                    key={src.id}
                    className={`flex items-center gap-2.5 px-3 py-2.5 rounded-lg border cursor-pointer transition-colors text-sm ${
                      sources.includes(src.id)
                        ? 'bg-brand-primary/5 border-brand-primary text-brand-primary font-medium'
                        : 'bg-surface-primary border-border text-text-secondary hover:border-brand-primary/40'
                    }`}
                  >
                    <input type="checkbox" checked={sources.includes(src.id)} onChange={() => toggleSource(src.id)} className="sr-only" />
                    <span className={`w-4 h-4 rounded border flex items-center justify-center flex-shrink-0 ${
                      sources.includes(src.id) ? 'bg-brand-primary border-brand-primary' : 'border-border bg-white'
                    }`}>
                      {sources.includes(src.id) && (
                        <svg className="w-2.5 h-2.5 text-white" fill="none" viewBox="0 0 10 10">
                          <path d="M1.5 5l2.5 2.5 4.5-4.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                        </svg>
                      )}
                    </span>
                    {src.label}
                  </label>
                ))}
              </div>
            </div>

            {/* Severity filter */}
            <div className={sectionClass}>
              <div className="flex items-center gap-2">
                <div className="w-6 h-6 rounded-full bg-amber-50 flex items-center justify-center flex-shrink-0">
                  <Filter className="w-3.5 h-3.5 text-amber-600" />
                </div>
                <span className="text-sm font-semibold text-text-primary">Severity filter</span>
                <span className="text-xs text-text-tertiary ml-auto">empty = all severities</span>
              </div>
              <div className="flex flex-wrap gap-2">
                {SEVERITIES.map((sev) => (
                  <button
                    key={sev} type="button" onClick={() => toggleSeverity(sev)}
                    className={`px-3 py-1 rounded-full border text-xs font-semibold uppercase tracking-wide transition-colors ${
                      severities.includes(sev)
                        ? SEVERITY_COLORS[sev]
                        : 'bg-surface-primary border-border text-text-tertiary hover:border-brand-primary/40'
                    }`}
                  >
                    {sev}
                  </button>
                ))}
              </div>
            </div>

            {/* Actions */}
            <div className={sectionClass}>
              <div className="flex items-center gap-2">
                <div className="w-6 h-6 rounded-full bg-red-50 flex items-center justify-center flex-shrink-0">
                  <Bell className="w-3.5 h-3.5 text-red-500" />
                </div>
                <span className="text-sm font-semibold text-text-primary">Actions</span>
              </div>
              <div className="space-y-3">
                <div className="flex items-center justify-between gap-4">
                  <div>
                    <p className="text-sm font-medium text-text-primary">Create incident</p>
                    <p className="text-xs text-text-tertiary">Automatically open an incident for matching alerts</p>
                  </div>
                  <YesNo value={createIncident} onChange={handleCreateChange} disabled={suppress} />
                </div>
                <div className="space-y-2">
                  <div className="flex items-center justify-between gap-4">
                    <div>
                      <p className="text-sm font-medium text-text-primary">Escalate</p>
                      <p className="text-xs text-text-tertiary">Page the on-call team via an escalation path</p>
                    </div>
                    <YesNo value={escalate} onChange={handleEscalateChange} />
                  </div>
                  {escalate && (
                    <div className="ml-4 pl-3 border-l-2 border-brand-primary/20">
                      <label className="block text-xs font-medium text-text-secondary mb-1">Escalation path</label>
                      <div className="relative">
                        <select
                          value={escalationPolicyId}
                          onChange={(e) => setEpId(e.target.value)}
                          className="w-full appearance-none px-3 py-2 pr-8 border border-border rounded-lg text-sm bg-surface-primary focus:outline-none focus:ring-2 focus:ring-brand-primary"
                        >
                          <option value="">— Select a path —</option>
                          {policies.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
                        </select>
                        <ChevronDown className="absolute right-2.5 top-2.5 w-4 h-4 text-text-tertiary pointer-events-none" />
                      </div>
                    </div>
                  )}
                </div>
                <div className="flex items-center justify-between gap-4">
                  <div>
                    <p className="text-sm font-medium text-text-primary">Suppress alert</p>
                    <p className="text-xs text-text-tertiary">Drop matching alerts silently — no incident, no page</p>
                  </div>
                  <YesNo value={suppress} onChange={handleSuppressChange} disabled={createIncident} />
                </div>
                <div className="flex items-center justify-between gap-4">
                  <div>
                    <p className="text-sm font-medium text-text-primary">Severity override</p>
                    <p className="text-xs text-text-tertiary">Override the alert's severity before incident creation</p>
                  </div>
                  <div className="relative">
                    <select
                      value={severityOverride}
                      onChange={(e) => setSevOvr(e.target.value)}
                      className="appearance-none pl-3 pr-8 py-1.5 border border-border rounded-lg text-sm bg-surface-primary focus:outline-none focus:ring-2 focus:ring-brand-primary"
                    >
                      <option value="">None</option>
                      {SEVERITIES.map((s) => <option key={s} value={s}>{s}</option>)}
                    </select>
                    <ChevronDown className="absolute right-2 top-2 w-4 h-4 text-text-tertiary pointer-events-none" />
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div className="px-6 py-4 border-t border-border bg-surface-primary rounded-b-2xl flex justify-end gap-3 flex-shrink-0">
            <Button type="button" variant="secondary" onClick={onClose} disabled={saving}>Cancel</Button>
            <Button type="submit" variant="primary" disabled={saving || !name.trim()}>
              {saving ? 'Saving…' : rule ? 'Save changes' : 'Create route'}
            </Button>
          </div>
        </form>
      </div>
    </div>
  )
}

// ─── Draggable rule row ────────────────────────────────────────────────────────

function RuleRow({
  rule,
  index,
  policies,
  deleting,
  onEdit,
  onDelete,
  onDragStart,
  onDragOver,
  onDrop,
  isDragOver,
}: {
  rule: RoutingRule
  index: number
  policies: EscalationPolicy[]
  deleting: boolean
  onEdit: (r: RoutingRule) => void
  onDelete: (r: RoutingRule) => void
  onDragStart: (index: number) => void
  onDragOver: (e: React.DragEvent, index: number) => void
  onDrop: (index: number) => void
  isDragOver: boolean
}) {
  const mc = rule.match_criteria as Record<string, unknown>
  const ac = rule.actions as Record<string, unknown>

  const matchSummary = (): string => {
    if (Object.keys(mc).length === 0) return 'All alerts'
    const parts: string[] = []
    if (mc.source)   parts.push((mc.source as string[]).join(', '))
    if (mc.severity) parts.push((mc.severity as string[]).join(', '))
    return parts.join(' · ')
  }

  const actionSummary = (): string => {
    const parts: string[] = []
    if (ac.suppress)          parts.push('Suppress')
    if (ac.create_incident)   parts.push('Create incident')
    if (ac.severity_override) parts.push(`→ ${ac.severity_override}`)
    if (ac.escalation_policy_id) {
      const policy = policies.find((p) => p.id === ac.escalation_policy_id)
      parts.push(`Escalate → ${policy?.name ?? '…'}`)
    }
    return parts.length > 0 ? parts.join(', ') : 'No action'
  }

  return (
    <div
      draggable
      onDragStart={() => onDragStart(index)}
      onDragOver={(e) => { e.preventDefault(); onDragOver(e, index) }}
      onDrop={() => onDrop(index)}
      className={`flex items-center gap-3 px-4 py-3 bg-white border border-border rounded-xl transition-all group
        ${deleting ? 'opacity-40' : ''}
        ${isDragOver ? 'border-brand-primary shadow-sm ring-1 ring-brand-primary/30' : ''}
      `}
    >
      {/* Drag handle */}
      <div
        className="cursor-grab active:cursor-grabbing text-text-tertiary opacity-0 group-hover:opacity-100 transition-opacity flex-shrink-0"
        title="Drag to reorder"
      >
        <GripVertical className="w-4 h-4" />
      </div>

      {/* Position badge */}
      <span className="w-6 text-center text-xs font-mono text-text-tertiary flex-shrink-0">{index + 1}</span>

      {/* Content */}
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium text-text-primary">{rule.name}</span>
          {!rule.enabled && (
            <span className="px-1.5 py-0.5 rounded text-xs bg-gray-100 text-gray-500">Disabled</span>
          )}
        </div>
        <p className="text-xs text-text-tertiary mt-0.5">
          <span className="text-text-secondary">Match:</span> {matchSummary()}
          <span className="mx-1.5">·</span>
          <span className="text-text-secondary">Then:</span> {actionSummary()}
        </p>
      </div>

      {/* Actions */}
      <div className="flex items-center gap-1 flex-shrink-0 opacity-0 group-hover:opacity-100 transition-opacity">
        <button
          onClick={() => onEdit(rule)}
          className="p-1.5 text-text-tertiary hover:text-text-primary hover:bg-gray-100 rounded transition-colors"
          title="Edit route"
        >
          <Pencil className="w-3.5 h-3.5" />
        </button>
        <button
          onClick={() => onDelete(rule)}
          disabled={deleting}
          className="p-1.5 text-text-tertiary hover:text-red-600 hover:bg-red-50 rounded transition-colors disabled:opacity-50"
          title="Delete route"
        >
          <Trash2 className="w-3.5 h-3.5" />
        </button>
      </div>
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export function RoutingRulesPage() {
  const { rules: fetchedRules, loading, error, refetch } = useRoutingRules()
  const [rules, setRules]           = useState<RoutingRule[]>([])
  const [modalOpen, setModalOpen]   = useState(false)
  const [editingRule, setEditingRule] = useState<RoutingRule | null>(null)
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const [deleteError, setDeleteError] = useState<string | null>(null)
  const [policies, setPolicies]     = useState<EscalationPolicy[]>([])

  // drag state
  const dragIndex = useRef<number | null>(null)
  const [dragOverIndex, setDragOverIndex] = useState<number | null>(null)

  useEffect(() => { setRules(fetchedRules) }, [fetchedRules])
  useEffect(() => {
    listEscalationPolicies().then((r) => setPolicies(r.data)).catch(() => {})
  }, [])

  const handleCreate = () => { setEditingRule(null); setModalOpen(true) }
  const handleEdit   = (rule: RoutingRule) => { setEditingRule(rule); setModalOpen(true) }

  const handleDelete = async (rule: RoutingRule) => {
    if (!confirm(`Delete alert route "${rule.name}"? This cannot be undone.`)) return
    setDeletingId(rule.id)
    setDeleteError(null)
    try {
      await deleteRoutingRule(rule.id)
      await refetch()
    } catch (err) {
      setDeleteError(err instanceof Error ? err.message : 'Failed to delete route')
    } finally {
      setDeletingId(null)
    }
  }

  // ── Drag-to-reorder handlers ──
  const handleDragStart = (index: number) => { dragIndex.current = index }

  const handleDragOver = (_e: React.DragEvent, index: number) => { setDragOverIndex(index) }

  const handleDrop = async (dropIndex: number) => {
    const fromIndex = dragIndex.current
    if (fromIndex === null || fromIndex === dropIndex) {
      dragIndex.current = null
      setDragOverIndex(null)
      return
    }

    // Reorder locally (optimistic)
    const next = [...rules]
    const removed = next.splice(fromIndex, 1)
    if (!removed[0]) { dragIndex.current = null; setDragOverIndex(null); return }
    next.splice(dropIndex, 0, removed[0])
    setRules(next)

    dragIndex.current = null
    setDragOverIndex(null)

    // Persist
    try {
      await reorderRoutingRules(next.map((r) => r.id))
    } catch {
      // Revert on failure
      setRules(fetchedRules)
    }
  }

  const handleDragEnd = () => { dragIndex.current = null; setDragOverIndex(null) }

  return (
    <div className="flex flex-col h-full">
      <RouteBuilderModal
        isOpen={modalOpen}
        rule={editingRule}
        onClose={() => setModalOpen(false)}
        onSaved={refetch}
      />

      {/* Page Header */}
      <div className="border-b border-border bg-surface-primary px-6 py-4">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-text-primary">Alert Routes</h1>
            <p className="mt-1 text-sm text-text-secondary">
              Route alerts to the right escalation path. Drag to change evaluation order — top rule wins.
            </p>
          </div>
          <Button variant="primary" onClick={handleCreate}>
            <Plus className="w-4 h-4" />
            Add route
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
          <SkeletonTable />
        ) : error ? (
          <GeneralError message={error} onRetry={refetch} />
        ) : rules.length === 0 ? (
          <EmptyState
            icon="check"
            title="No alert routes"
            description="Create an alert route to control how alerts are routed to incidents and escalation paths."
            actionLabel="Add route"
            onAction={handleCreate}
          />
        ) : (
          <div
            className="space-y-2"
            onDragEnd={handleDragEnd}
          >
            {rules.map((rule, i) => (
              <RuleRow
                key={rule.id}
                rule={rule}
                index={i}
                policies={policies}
                deleting={deletingId === rule.id}
                onEdit={handleEdit}
                onDelete={handleDelete}
                onDragStart={handleDragStart}
                onDragOver={handleDragOver}
                onDrop={handleDrop}
                isDragOver={dragOverIndex === i}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
