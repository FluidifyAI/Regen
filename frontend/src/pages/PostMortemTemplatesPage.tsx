import { useState, useEffect, useRef } from 'react'
import { Plus, Pencil, Trash2, FileText, Lock, GripVertical, X } from 'lucide-react'
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

// ─── Section Editor ────────────────────────────────────────────────────────────

function SectionEditor({
  sections,
  onChange,
}: {
  sections: string[]
  onChange: (sections: string[]) => void
}) {
  function update(i: number, value: string) {
    const next = [...sections]
    next[i] = value
    onChange(next)
  }

  function remove(i: number) {
    onChange(sections.filter((_, idx) => idx !== i))
  }

  function addSection() {
    onChange([...sections, ''])
    setTimeout(() => {
      const inputs = document.querySelectorAll<HTMLInputElement>('[data-section-input]')
      inputs[inputs.length - 1]?.focus()
    }, 30)
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>, i: number) {
    if (e.key === 'Enter') {
      e.preventDefault()
      addSection()
    } else if (e.key === 'Backspace' && sections[i] === '' && sections.length > 1) {
      e.preventDefault()
      remove(i)
      setTimeout(() => {
        const inputs = document.querySelectorAll<HTMLInputElement>('[data-section-input]')
        inputs[Math.max(0, i - 1)]?.focus()
      }, 30)
    }
  }

  return (
    <div className="space-y-1.5">
      {sections.map((section, i) => (
        <div key={i} className="flex items-center gap-2 group">
          <GripVertical className="w-4 h-4 text-text-tertiary flex-shrink-0 opacity-40 group-hover:opacity-70" />
          <span className="text-xs text-text-tertiary w-5 text-right flex-shrink-0">{i + 1}.</span>
          <input
            data-section-input
            type="text"
            value={section}
            onChange={(e) => update(i, e.target.value)}
            onKeyDown={(e) => handleKeyDown(e, i)}
            placeholder={`Section ${i + 1} heading`}
            className="flex-1 px-2.5 py-1.5 text-sm border border-border rounded focus:outline-none focus:ring-1 focus:ring-brand-primary"
          />
          <button
            type="button"
            onClick={() => remove(i)}
            disabled={sections.length === 1}
            className="p-1 rounded text-text-tertiary hover:text-red-500 hover:bg-red-50 transition-colors disabled:opacity-20 disabled:cursor-not-allowed"
          >
            <X className="w-3.5 h-3.5" />
          </button>
        </div>
      ))}
      <button
        type="button"
        onClick={addSection}
        className="flex items-center gap-1.5 mt-1 text-sm text-brand-primary hover:text-brand-primary-hover font-medium"
      >
        <Plus className="w-3.5 h-3.5" />
        Add section
      </button>
    </div>
  )
}

// ─── Template Modal ────────────────────────────────────────────────────────────

interface TemplateModalProps {
  isOpen: boolean
  template: PostMortemTemplate | null
  onClose: () => void
  onSaved: () => void
}

const DEFAULT_SECTIONS = ['Summary', 'Impact', 'Timeline', 'Root Cause', 'Action Items']

function TemplateModal({ isOpen, template, onClose, onSaved }: TemplateModalProps) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [sections, setSections] = useState<string[]>(DEFAULT_SECTIONS)
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const nameRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (template) {
      setName(template.name)
      setDescription(template.description)
      setSections(template.sections.length > 0 ? template.sections : DEFAULT_SECTIONS)
    } else {
      setName('')
      setDescription('')
      setSections([...DEFAULT_SECTIONS])
    }
    setError(null)
  }, [template, isOpen])

  useEffect(() => {
    if (isOpen) nameRef.current?.focus()
  }, [isOpen])

  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    if (isOpen) document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [isOpen, onClose])

  async function handleSubmit() {
    const cleaned = sections.map((s) => s.trim()).filter(Boolean)
    if (!name.trim()) { setError('Name is required'); return }
    if (cleaned.length === 0) { setError('At least one section is required'); return }

    setSubmitting(true)
    setError(null)
    try {
      if (template) {
        await updatePostMortemTemplate(template.id, { name: name.trim(), description, sections: cleaned })
      } else {
        await createPostMortemTemplate({ name: name.trim(), description, sections: cleaned })
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
      <div className="absolute inset-0 bg-black bg-opacity-50" onClick={onClose} />
      <div className="relative bg-white rounded-xl shadow-xl w-full max-w-lg mx-4 flex flex-col max-h-[90vh]">
        {/* Header */}
        <div className="px-6 py-4 border-b border-border">
          <h2 className="text-base font-semibold text-text-primary">
            {template ? 'Edit Template' : 'New Template'}
          </h2>
          <p className="text-xs text-text-tertiary mt-0.5">
            Each section becomes a heading in the generated post-mortem document.
          </p>
        </div>

        {/* Body */}
        <div className="px-6 py-4 space-y-4 overflow-y-auto">
          {error && (
            <p className="text-sm text-red-600 bg-red-50 border border-red-200 rounded px-3 py-2">
              {error}
            </p>
          )}

          <div>
            <label className="block text-sm font-medium text-text-primary mb-1">
              Template name <span className="text-red-500">*</span>
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
              Description <span className="text-xs text-text-tertiary font-normal">(optional)</span>
            </label>
            <input
              type="text"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="When to use this template"
              className="w-full px-3 py-2 text-sm border border-border rounded focus:outline-none focus:ring-1 focus:ring-brand-primary"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-text-primary mb-2">
              Sections <span className="text-red-500">*</span>
            </label>
            <SectionEditor sections={sections} onChange={setSections} />
          </div>
        </div>

        {/* Footer */}
        <div className="px-6 py-4 border-t border-border flex justify-end gap-3">
          <Button variant="ghost" size="sm" onClick={onClose}>Cancel</Button>
          <Button variant="primary" size="sm" onClick={handleSubmit} loading={submitting} disabled={submitting}>
            {template ? 'Save changes' : 'Create template'}
          </Button>
        </div>
      </div>
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export function PostMortemTemplatesPage() {
  const [templates, setTemplates] = useState<PostMortemTemplate[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [modalOpen, setModalOpen] = useState(false)
  const [editingTemplate, setEditingTemplate] = useState<PostMortemTemplate | null>(null)
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const [deleteError, setDeleteError] = useState<string | null>(null)

  async function fetchTemplates() {
    setLoading(true)
    setError(null)
    try {
      setTemplates(await listPostMortemTemplates())
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load templates')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { fetchTemplates() }, [])

  function openCreate() { setEditingTemplate(null); setModalOpen(true) }
  function openEdit(t: PostMortemTemplate) { setEditingTemplate(t); setModalOpen(true) }

  async function handleDelete(id: string) {
    if (!confirm('Delete this template? This cannot be undone.')) return
    setDeletingId(id)
    setDeleteError(null)
    try {
      await deletePostMortemTemplate(id)
      setTemplates((prev) => prev.filter((t) => t.id !== id))
    } catch (err) {
      setDeleteError(err instanceof Error ? err.message : 'Failed to delete template')
    } finally {
      setDeletingId(null)
    }
  }

  if (loading) {
    return (
      <div className="max-w-3xl mx-auto px-6 py-8 space-y-3">
        {[1, 2, 3].map((i) => <div key={i} className="h-20 bg-surface-tertiary rounded-xl animate-pulse" />)}
      </div>
    )
  }

  if (error) {
    return <div className="flex h-full items-center justify-center"><GeneralError message={error} onRetry={fetchTemplates} /></div>
  }

  return (
    <div className="max-w-3xl mx-auto px-6 py-8">
      {/* Page header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-semibold text-text-primary">Post-Mortem Templates</h1>
          <p className="text-sm text-text-secondary mt-1">
            Define the sections included when AI generates a post-mortem after an incident.
          </p>
        </div>
        <Button variant="primary" size="sm" onClick={openCreate}>
          <Plus className="w-4 h-4" />
          New template
        </Button>
      </div>

      {deleteError && (
        <div className="mb-4 px-4 py-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-700">
          {deleteError}
        </div>
      )}

      {/* Template list — flat, no Built-in/Custom split */}
      {templates.length === 0 ? (
        <EmptyState
          title="No templates yet"
          description="Create a template to define the structure of your AI-generated post-mortems."
          actionLabel="Create template"
          onAction={openCreate}
        />
      ) : (
        <div className="space-y-3">
          {templates.map((t) => (
            <TemplateCard
              key={t.id}
              template={t}
              deleting={deletingId === t.id}
              onEdit={openEdit}
              onDelete={handleDelete}
            />
          ))}
        </div>
      )}

      <TemplateModal
        isOpen={modalOpen}
        template={editingTemplate}
        onClose={() => setModalOpen(false)}
        onSaved={fetchTemplates}
      />
    </div>
  )
}

function TemplateCard({
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
    <div className={`bg-white border border-border rounded-xl px-5 py-4 transition-opacity ${deleting ? 'opacity-50' : ''}`}>
      <div className="flex items-start gap-3">
        <FileText className="w-4 h-4 text-text-tertiary flex-shrink-0 mt-0.5" />
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="text-sm font-medium text-text-primary">{template.name}</span>
            {template.is_built_in && (
              <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-xs bg-surface-secondary text-text-tertiary border border-border">
                <Lock className="w-2.5 h-2.5" />
                Built-in
              </span>
            )}
          </div>
          {template.description && (
            <p className="text-xs text-text-tertiary mt-0.5">{template.description}</p>
          )}
          {/* Sections as numbered pills */}
          <div className="flex flex-wrap gap-1.5 mt-2">
            {template.sections.map((s, i) => (
              <span
                key={i}
                className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-surface-secondary border border-border text-xs text-text-secondary"
              >
                <span className="text-text-tertiary">{i + 1}.</span>
                {s}
              </span>
            ))}
          </div>
        </div>
        {/* Actions — edit always shown; delete hidden for built-in */}
        <div className="flex items-center gap-1 flex-shrink-0">
          <button
            onClick={() => onEdit(template)}
            className="p-1.5 rounded text-text-tertiary hover:text-text-primary hover:bg-surface-secondary transition-colors"
            title="Edit template"
          >
            <Pencil className="w-3.5 h-3.5" />
          </button>
          {!template.is_built_in && (
            <button
              onClick={() => onDelete(template.id)}
              disabled={deleting}
              className="p-1.5 rounded text-text-tertiary hover:text-red-500 hover:bg-red-50 transition-colors"
              title="Delete template"
            >
              <Trash2 className="w-3.5 h-3.5" />
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
