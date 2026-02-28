import { useState, useEffect, useRef } from 'react'
import { Plus, Pencil, Trash2 } from 'lucide-react'
import { Button } from '../components/ui/Button'
import { SkeletonTable } from '../components/ui/Skeleton'
import { EmptyState } from '../components/ui/EmptyState'
import { GeneralError } from '../components/ui/ErrorState'
import { useRoutingRules } from '../hooks/useRoutingRules'
import { createRoutingRule, updateRoutingRule, deleteRoutingRule } from '../api/routing_rules'
import type { RoutingRule, CreateRoutingRuleRequest } from '../api/types'

// ─── Modal ────────────────────────────────────────────────────────────────────

interface RuleModalProps {
  isOpen: boolean
  rule: RoutingRule | null // null = create mode, non-null = edit mode
  onClose: () => void
  onSaved: () => void
}

function RuleModal({ isOpen, rule, onClose, onSaved }: RuleModalProps) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [priority, setPriority] = useState(100)
  const [enabled, setEnabled] = useState(true)
  const [matchCriteriaText, setMatchCriteriaText] = useState('{}')
  const [actionsText, setActionsText] = useState('{"create_incident": true}')
  const [error, setError] = useState<string | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const nameRef = useRef<HTMLInputElement>(null)

  // Populate form when editing an existing rule
  useEffect(() => {
    if (rule) {
      setName(rule.name)
      setDescription(rule.description)
      setPriority(rule.priority)
      setEnabled(rule.enabled)
      setMatchCriteriaText(JSON.stringify(rule.match_criteria, null, 2))
      setActionsText(JSON.stringify(rule.actions, null, 2))
    } else {
      setName('')
      setDescription('')
      setPriority(100)
      setEnabled(true)
      setMatchCriteriaText('{}')
      setActionsText('{"create_incident": true}')
    }
    setError(null)
  }, [rule, isOpen])

  useEffect(() => {
    if (isOpen) nameRef.current?.focus()
  }, [isOpen])

  // Close on Escape
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    if (isOpen) document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [isOpen, onClose])

  if (!isOpen) return null

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    // Validate JSON fields
    let matchCriteria: Record<string, unknown>
    let actions: Record<string, unknown>
    try {
      matchCriteria = JSON.parse(matchCriteriaText)
    } catch {
      setError('match_criteria must be valid JSON')
      return
    }
    try {
      actions = JSON.parse(actionsText)
    } catch {
      setError('actions must be valid JSON')
      return
    }

    setIsSubmitting(true)
    try {
      if (rule) {
        await updateRoutingRule(rule.id, { name, description, priority, enabled, match_criteria: matchCriteria, actions })
      } else {
        const body: CreateRoutingRuleRequest = { name, description, priority, enabled, match_criteria: matchCriteria, actions }
        await createRoutingRule(body)
      }
      onSaved()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save routing rule')
    } finally {
      setIsSubmitting(false)
    }
  }

  const inputClass = 'w-full px-3 py-2 border border-border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary focus:border-transparent disabled:opacity-50'
  const labelClass = 'block text-sm font-medium text-text-primary mb-1'

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />

      {/* Modal */}
      <div
        className="relative z-10 w-full max-w-lg bg-white rounded-xl shadow-xl mx-4 flex flex-col max-h-[90vh]"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="px-6 py-4 border-b border-border">
          <h2 className="text-lg font-semibold text-text-primary">
            {rule ? 'Edit routing rule' : 'New routing rule'}
          </h2>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} className="flex flex-col flex-1 overflow-hidden">
          <div className="px-6 py-4 overflow-y-auto flex-1 space-y-4">
            {error && (
              <div className="px-4 py-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-700">
                {error}
              </div>
            )}

            <div>
              <label className={labelClass} htmlFor="rule-name">Name</label>
              <input
                ref={nameRef}
                id="rule-name"
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. Suppress info alerts"
                className={inputClass}
                disabled={isSubmitting}
                required
              />
            </div>

            <div>
              <label className={labelClass} htmlFor="rule-description">Description</label>
              <input
                id="rule-description"
                type="text"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="What does this rule do?"
                className={inputClass}
                disabled={isSubmitting}
              />
            </div>

            <div className="flex gap-4">
              <div className="flex-1">
                <label className={labelClass} htmlFor="rule-priority">Priority</label>
                <input
                  id="rule-priority"
                  type="number"
                  value={priority}
                  onChange={(e) => setPriority(Number(e.target.value))}
                  min={1}
                  max={10000}
                  className={inputClass}
                  disabled={isSubmitting}
                  required
                />
                <p className="mt-1 text-xs text-text-tertiary">Lower number = higher priority</p>
              </div>

              <div className="flex items-center gap-2 pt-6">
                <input
                  id="rule-enabled"
                  type="checkbox"
                  checked={enabled}
                  onChange={(e) => setEnabled(e.target.checked)}
                  className="h-4 w-4 rounded border-border text-brand-primary focus:ring-brand-primary"
                  disabled={isSubmitting}
                />
                <label htmlFor="rule-enabled" className="text-sm font-medium text-text-primary">
                  Enabled
                </label>
              </div>
            </div>

            <div>
              <label className={labelClass} htmlFor="rule-match">Match criteria (JSON)</label>
              <textarea
                id="rule-match"
                value={matchCriteriaText}
                onChange={(e) => setMatchCriteriaText(e.target.value)}
                rows={4}
                className={`${inputClass} font-mono text-xs`}
                disabled={isSubmitting}
                placeholder={'{\n  "severity": ["critical"],\n  "source": ["prometheus"]\n}'}
              />
              <p className="mt-1 text-xs text-text-tertiary">
                Keys: <code>source</code> (list), <code>severity</code> (list), <code>labels</code> (map, * wildcard). Empty = match all.
              </p>
            </div>

            <div>
              <label className={labelClass} htmlFor="rule-actions">Actions (JSON)</label>
              <textarea
                id="rule-actions"
                value={actionsText}
                onChange={(e) => setActionsText(e.target.value)}
                rows={4}
                className={`${inputClass} font-mono text-xs`}
                disabled={isSubmitting}
                placeholder={'{\n  "create_incident": true\n}'}
              />
              <p className="mt-1 text-xs text-text-tertiary">
                Keys: <code>create_incident</code>, <code>suppress</code>, <code>severity_override</code>, <code>channel_override</code>, <code>escalation_policy_id</code>
              </p>
            </div>
          </div>

          {/* Footer */}
          <div className="px-6 py-4 border-t border-border flex justify-end gap-3">
            <Button type="button" variant="secondary" onClick={onClose} disabled={isSubmitting}>
              Cancel
            </Button>
            <Button type="submit" variant="primary" disabled={isSubmitting}>
              {isSubmitting ? 'Saving…' : rule ? 'Save changes' : 'Create rule'}
            </Button>
          </div>
        </form>
      </div>
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

/**
 * Routing rules management page
 * Shows all routing rules in priority order with create/edit/delete actions.
 * Rules determine whether alerts create incidents, suppress them, or override channel/severity.
 */
export function RoutingRulesPage() {
  const { rules, loading, error, refetch } = useRoutingRules()
  const [modalOpen, setModalOpen] = useState(false)
  const [editingRule, setEditingRule] = useState<RoutingRule | null>(null)
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const [deleteError, setDeleteError] = useState<string | null>(null)

  const handleCreate = () => {
    setEditingRule(null)
    setModalOpen(true)
  }

  const handleEdit = (rule: RoutingRule) => {
    setEditingRule(rule)
    setModalOpen(true)
  }

  const handleDelete = async (rule: RoutingRule) => {
    if (!confirm(`Delete routing rule "${rule.name}"? This cannot be undone.`)) return
    setDeletingId(rule.id)
    setDeleteError(null)
    try {
      await deleteRoutingRule(rule.id)
      await refetch()
    } catch (err) {
      setDeleteError(err instanceof Error ? err.message : 'Failed to delete rule')
    } finally {
      setDeletingId(null)
    }
  }

  const summarizeActions = (actions: Record<string, unknown>): string => {
    const parts: string[] = []
    if (actions.suppress) parts.push('Suppress')
    if (actions.create_incident) parts.push('Create incident')
    if (actions.severity_override) parts.push(`Severity → ${actions.severity_override}`)
    if (actions.channel_override) parts.push(`Channel → ${actions.channel_override}`)
    if (actions.escalation_policy_id) parts.push(`Escalation → ${String(actions.escalation_policy_id).slice(0, 8)}…`)
    return parts.length > 0 ? parts.join(', ') : 'No action'
  }

  const summarizeCriteria = (criteria: Record<string, unknown>): string => {
    if (Object.keys(criteria).length === 0) return 'All alerts'
    const parts: string[] = []
    if (criteria.source) parts.push(`source: ${(criteria.source as string[]).join(', ')}`)
    if (criteria.severity) parts.push(`severity: ${(criteria.severity as string[]).join(', ')}`)
    if (criteria.labels) parts.push('labels: ...')
    return parts.join(' · ')
  }

  return (
    <div className="flex flex-col h-full">
      <RuleModal
        isOpen={modalOpen}
        rule={editingRule}
        onClose={() => setModalOpen(false)}
        onSaved={refetch}
      />

      {/* Page Header */}
      <div className="border-b border-border bg-white px-6 py-4">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-text-primary">Routing Rules</h1>
            <p className="mt-1 text-sm text-text-secondary">
              Route alerts to the right channels and control which alerts create incidents.
              Rules are evaluated in priority order — first match wins.
            </p>
          </div>
          <Button variant="primary" onClick={handleCreate}>
            <Plus className="w-4 h-4" />
            Add rule
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
            title="No routing rules"
            description="Create a routing rule to control how alerts are routed to incidents and channels."
            actionLabel="Add rule"
            onAction={handleCreate}
          />
        ) : (
          <div className="bg-white border border-border rounded-lg overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border bg-gray-50">
                  <th className="text-left px-4 py-3 text-xs font-medium text-text-tertiary uppercase tracking-wider w-20">Priority</th>
                  <th className="text-left px-4 py-3 text-xs font-medium text-text-tertiary uppercase tracking-wider">Name</th>
                  <th className="text-left px-4 py-3 text-xs font-medium text-text-tertiary uppercase tracking-wider">Match</th>
                  <th className="text-left px-4 py-3 text-xs font-medium text-text-tertiary uppercase tracking-wider">Action</th>
                  <th className="text-left px-4 py-3 text-xs font-medium text-text-tertiary uppercase tracking-wider w-20">Status</th>
                  <th className="w-24 px-4 py-3" />
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {rules.map((rule) => (
                  <tr key={rule.id} className="hover:bg-gray-50 transition-colors group">
                    <td className="px-4 py-3 font-mono text-text-secondary">{rule.priority}</td>
                    <td className="px-4 py-3">
                      <div className="font-medium text-text-primary">{rule.name}</div>
                      {rule.description && (
                        <div className="text-xs text-text-tertiary mt-0.5">{rule.description}</div>
                      )}
                    </td>
                    <td className="px-4 py-3 text-text-secondary">
                      {summarizeCriteria(rule.match_criteria)}
                    </td>
                    <td className="px-4 py-3 text-text-secondary">
                      {summarizeActions(rule.actions)}
                    </td>
                    <td className="px-4 py-3">
                      <span
                        className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${
                          rule.enabled
                            ? 'bg-brand-primary/10 text-brand-primary'
                            : 'bg-gray-100 text-gray-500'
                        }`}
                      >
                        {rule.enabled ? 'Active' : 'Disabled'}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center justify-end gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                        <button
                          onClick={() => handleEdit(rule)}
                          className="p-1.5 text-text-tertiary hover:text-text-primary hover:bg-gray-100 rounded transition-colors"
                          title="Edit rule"
                        >
                          <Pencil className="w-4 h-4" />
                        </button>
                        <button
                          onClick={() => handleDelete(rule)}
                          disabled={deletingId === rule.id}
                          className="p-1.5 text-text-tertiary hover:text-red-600 hover:bg-red-50 rounded transition-colors disabled:opacity-50"
                          title="Delete rule"
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

