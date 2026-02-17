import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus, GitBranch, Trash2, ChevronRight, ToggleLeft, ToggleRight } from 'lucide-react'
import { Button } from '../components/ui/Button'
import { SkeletonTable } from '../components/ui/Skeleton'
import { GeneralError } from '../components/ui/ErrorState'
import { useEscalationPolicies } from '../hooks/useEscalationPolicies'
import {
  createEscalationPolicy,
  deleteEscalationPolicy,
  updateEscalationPolicy,
} from '../api/escalation'
import type { CreateEscalationPolicyRequest } from '../api/types'

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
                          <ToggleRight className="w-4 h-4 text-green-600" />
                          <span className="text-green-700">Enabled</span>
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
