import { useState, useEffect, useRef } from 'react'
import { Plus, Pencil, Trash2, FileText, Lock } from 'lucide-react'
import { Button } from '../components/ui/Button'
import { EmptyState } from '../components/ui/EmptyState'
import { GeneralError } from '../components/ui/ErrorState'
import {
  listPostMortemTemplates,
  createPostMortemTemplate,
  updatePostMortemTemplate,
  deletePostMortemTemplate,
} from '../api/postmortems'
import type { PostMortemTemplate } from '../api/types'

// ─── Template Modal ────────────────────────────────────────────────────────────

interface TemplateModalProps {
  isOpen: boolean
  template: PostMortemTemplate | null
  onClose: () => void
  onSaved: () => void
}

function TemplateModal({ isOpen, template, onClose, onSaved }: TemplateModalProps) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [sectionsText, setSectionsText] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const nameRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (template) {
      setName(template.name)
      setDescription(template.description)
      setSectionsText(template.sections.join('\n'))
    } else {
      setName('')
      setDescription('')
      setSectionsText('Summary\nImpact\nTimeline\nRoot Cause\nAction Items')
    }
    setError(null)
  }, [template, isOpen])

  useEffect(() => {
    if (isOpen) nameRef.current?.focus()
  }, [isOpen])

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    if (isOpen) document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [isOpen, onClose])

  async function handleSubmit() {
    const sections = sectionsText
      .split('\n')
      .map((s) => s.trim())
      .filter(Boolean)

    if (!name.trim()) {
      setError('Name is required')
      return
    }
    if (sections.length === 0) {
      setError('At least one section is required')
      return
    }

    setSubmitting(true)
    setError(null)
    try {
      if (template) {
        await updatePostMortemTemplate(template.id, { name: name.trim(), description, sections })
      } else {
        await createPostMortemTemplate({ name: name.trim(), description, sections })
      }
      onSaved()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save template')
    } finally {
      setSubmitting(false)
    }
  }

  if (!isOpen) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-black bg-opacity-50" onClick={onClose} />

      {/* Dialog */}
      <div className="relative bg-white rounded-lg shadow-xl w-full max-w-lg mx-4">
        <div className="px-6 py-4 border-b border-border">
          <h2 className="text-base font-semibold text-text-primary">
            {template ? 'Edit Template' : 'New Template'}
          </h2>
        </div>

        <div className="px-6 py-4 space-y-4">
          {error && (
            <p className="text-sm text-red-600 bg-red-50 border border-red-200 rounded px-3 py-2">
              {error}
            </p>
          )}

          <div>
            <label className="block text-sm font-medium text-text-primary mb-1">
              Name <span className="text-red-500">*</span>
            </label>
            <input
              ref={nameRef}
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Standard Incident Post-Mortem"
              className="w-full px-3 py-2 text-sm border border-border rounded focus:outline-none focus:ring-1 focus:ring-brand-primary"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-text-primary mb-1">
              Description
            </label>
            <input
              type="text"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Optional short description"
              className="w-full px-3 py-2 text-sm border border-border rounded focus:outline-none focus:ring-1 focus:ring-brand-primary"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-text-primary mb-1">
              Sections <span className="text-red-500">*</span>
            </label>
            <p className="text-xs text-text-tertiary mb-1">One section per line</p>
            <textarea
              value={sectionsText}
              onChange={(e) => setSectionsText(e.target.value)}
              rows={6}
              placeholder="Summary&#10;Impact&#10;Timeline&#10;Root Cause&#10;Action Items"
              className="w-full px-3 py-2 text-sm border border-border rounded focus:outline-none focus:ring-1 focus:ring-brand-primary font-mono resize-y"
            />
          </div>
        </div>

        <div className="px-6 py-4 border-t border-border flex justify-end gap-3">
          <Button variant="ghost" size="sm" onClick={onClose}>
            Cancel
          </Button>
          <Button
            variant="primary"
            size="sm"
            onClick={handleSubmit}
            loading={submitting}
            disabled={submitting}
          >
            {template ? 'Save Changes' : 'Create Template'}
          </Button>
        </div>
      </div>
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

/**
 * PostMortemTemplatesPage manages post-mortem templates.
 * Displays built-in templates (read-only) and user-created templates (full CRUD).
 */
export function PostMortemTemplatesPage() {
  const [templates, setTemplates] = useState<PostMortemTemplate[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [modalOpen, setModalOpen] = useState(false)
  const [editingTemplate, setEditingTemplate] = useState<PostMortemTemplate | null>(null)
  const [deletingId, setDeletingId] = useState<string | null>(null)

  async function fetchTemplates() {
    setLoading(true)
    setError(null)
    try {
      const data = await listPostMortemTemplates()
      setTemplates(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load templates')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchTemplates()
  }, [])

  function openCreate() {
    setEditingTemplate(null)
    setModalOpen(true)
  }

  function openEdit(t: PostMortemTemplate) {
    setEditingTemplate(t)
    setModalOpen(true)
  }

  async function handleDelete(id: string) {
    if (!confirm('Delete this template? This cannot be undone.')) return
    setDeletingId(id)
    try {
      await deletePostMortemTemplate(id)
      setTemplates((prev) => prev.filter((t) => t.id !== id))
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to delete template')
    } finally {
      setDeletingId(null)
    }
  }

  if (loading) {
    return (
      <div className="max-w-4xl mx-auto px-6 py-8">
        <div className="space-y-3">
          {[1, 2, 3].map((i) => (
            <div key={i} className="h-16 bg-surface-tertiary rounded animate-pulse" />
          ))}
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex h-full items-center justify-center">
        <GeneralError message={error} onRetry={fetchTemplates} />
      </div>
    )
  }

  const builtIn = templates.filter((t) => t.is_built_in)
  const custom = templates.filter((t) => !t.is_built_in)

  return (
    <div className="max-w-4xl mx-auto px-6 py-8">
      {/* Page header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-semibold text-text-primary">Post-Mortem Templates</h1>
          <p className="text-sm text-text-secondary mt-1">
            Templates define the sections used when generating post-mortems.
          </p>
        </div>
        <Button variant="primary" size="sm" onClick={openCreate}>
          <Plus className="w-4 h-4" />
          New Template
        </Button>
      </div>

      {/* Built-in templates */}
      {builtIn.length > 0 && (
        <div className="mb-6">
          <h2 className="text-xs font-semibold uppercase tracking-wider text-text-tertiary mb-2">
            Built-in
          </h2>
          <div className="border border-border rounded-lg divide-y divide-border bg-white">
            {builtIn.map((t) => (
              <TemplateRow
                key={t.id}
                template={t}
                deleting={deletingId === t.id}
                onEdit={openEdit}
                onDelete={handleDelete}
              />
            ))}
          </div>
        </div>
      )}

      {/* Custom templates */}
      <div>
        <h2 className="text-xs font-semibold uppercase tracking-wider text-text-tertiary mb-2">
          Custom
        </h2>
        {custom.length === 0 ? (
          <EmptyState
            title="No custom templates"
            description="Create your own template to define the structure of your post-mortems."
            actionLabel="Create template"
            onAction={openCreate}
          />
        ) : (
          <div className="border border-border rounded-lg divide-y divide-border bg-white">
            {custom.map((t) => (
              <TemplateRow
                key={t.id}
                template={t}
                deleting={deletingId === t.id}
                onEdit={openEdit}
                onDelete={handleDelete}
              />
            ))}
          </div>
        )}
      </div>

      <TemplateModal
        isOpen={modalOpen}
        template={editingTemplate}
        onClose={() => setModalOpen(false)}
        onSaved={fetchTemplates}
      />
    </div>
  )
}

function TemplateRow({
  template,
  deleting,
  onEdit,
  onDelete,
}: {
  template: PostMortemTemplate
  deleting: boolean
  onEdit: (t: PostMortemTemplate) => void
  onDelete: (id: string) => void
}) {
  return (
    <div className={`flex items-center gap-3 px-4 py-3 ${deleting ? 'opacity-50' : ''}`}>
      <FileText className="w-4 h-4 text-text-tertiary flex-shrink-0" />
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium text-text-primary">{template.name}</span>
          {template.is_built_in && (
            <span className="inline-flex items-center gap-1 text-xs text-text-tertiary">
              <Lock className="w-3 h-3" />
              Built-in
            </span>
          )}
        </div>
        {template.description && (
          <p className="text-xs text-text-tertiary truncate">{template.description}</p>
        )}
        <p className="text-xs text-text-tertiary mt-0.5">
          {template.sections.join(' · ')}
        </p>
      </div>
      <div className="flex items-center gap-1 flex-shrink-0">
        <button
          onClick={() => onEdit(template)}
          className="p-1.5 rounded text-text-tertiary hover:text-text-primary hover:bg-surface-secondary transition-colors"
          title="Edit template"
        >
          <Pencil className="w-3.5 h-3.5" />
        </button>
        <button
          onClick={() => onDelete(template.id)}
          disabled={deleting}
          className="p-1.5 rounded text-text-tertiary hover:text-red-500 hover:bg-red-50 transition-colors"
          title="Delete template"
        >
          <Trash2 className="w-3.5 h-3.5" />
        </button>
      </div>
    </div>
  )
}
