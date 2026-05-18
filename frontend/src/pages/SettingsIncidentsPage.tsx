import { useState, useEffect } from 'react'
import { Plus, Trash2, ChevronUp, ChevronDown, Pencil, X } from 'lucide-react'
import { Button } from '../components/ui/Button'
import {
  listCustomFields,
  createCustomField,
  updateCustomField,
  deleteCustomField,
  reorderCustomFields,
  CustomFieldDefinition,
  DropdownOption,
  FieldType,
} from '../api/customFields'

type FormState = {
  name: string
  key: string
  field_type: FieldType
  options: DropdownOption[]
  newOptionLabel: string
  newOptionValue: string
}

const emptyForm = (): FormState => ({
  name: '',
  key: '',
  field_type: 'string',
  options: [],
  newOptionLabel: '',
  newOptionValue: '',
})

function toSlug(name: string): string {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '')
    .replace(/^([^a-z])/, '_$1')
}

export function SettingsIncidentsPage() {
  const [fields, setFields] = useState<CustomFieldDefinition[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showForm, setShowForm] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [form, setForm] = useState<FormState>(emptyForm())
  const [saving, setSaving] = useState(false)
  const [formError, setFormError] = useState('')

  useEffect(() => {
    load()
  }, [])

  async function load() {
    setLoading(true)
    try {
      setFields(await listCustomFields())
      setError('')
    } catch {
      setError('Failed to load custom fields')
    } finally {
      setLoading(false)
    }
  }

  function openCreate() {
    setEditingId(null)
    setForm(emptyForm())
    setFormError('')
    setShowForm(true)
  }

  function openEdit(f: CustomFieldDefinition) {
    setEditingId(f.id)
    setForm({
      name: f.name,
      key: f.key,
      field_type: f.field_type,
      options: f.options ?? [],
      newOptionLabel: '',
      newOptionValue: '',
    })
    setFormError('')
    setShowForm(true)
  }

  function cancelForm() {
    setShowForm(false)
    setEditingId(null)
    setForm(emptyForm())
    setFormError('')
  }

  function handleNameChange(name: string) {
    setForm(prev => ({
      ...prev,
      name,
      key: editingId ? prev.key : toSlug(name),
    }))
  }

  function addOption() {
    const label = form.newOptionLabel.trim()
    const value = form.newOptionValue.trim() || toSlug(label)
    if (!label || !value) return
    setForm(prev => ({
      ...prev,
      options: [...prev.options, { label, value }],
      newOptionLabel: '',
      newOptionValue: '',
    }))
  }

  function removeOption(idx: number) {
    setForm(prev => ({ ...prev, options: prev.options.filter((_, i) => i !== idx) }))
  }

  async function handleSave() {
    setFormError('')
    if (!form.name.trim()) { setFormError('Name is required'); return }
    if (!form.key.trim()) { setFormError('Key is required'); return }
    if (form.field_type === 'dropdown' && form.options.length === 0) {
      setFormError('Dropdown fields require at least one option')
      return
    }

    setSaving(true)
    try {
      const payload = {
        name: form.name.trim(),
        key: form.key.trim(),
        field_type: form.field_type,
        options: form.options,
      }
      if (editingId) {
        await updateCustomField(editingId, payload)
      } else {
        await createCustomField(payload)
      }
      await load()
      cancelForm()
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : 'Failed to save'
      setFormError(msg.includes('409') || msg.includes('already exists') ? 'A field with that key already exists' : msg)
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(f: CustomFieldDefinition) {
    if (!confirm(`Delete field "${f.name}"? This cannot be undone.`)) return
    try {
      await deleteCustomField(f.id)
      await load()
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : ''
      if (msg.includes('409') || msg.includes('in use')) {
        alert(`Cannot delete "${f.name}" — it has values on existing incidents.`)
      } else {
        alert('Failed to delete field')
      }
    }
  }

  async function move(idx: number, dir: -1 | 1) {
    const next = [...fields]
    const target = idx + dir
    if (target < 0 || target >= next.length) return
    const a = next[idx]!
    const b = next[target]!
    next[idx] = b
    next[target] = a
    setFields(next)
    await reorderCustomFields(next.map((f, i) => ({ id: f.id, order: i })))
  }

  return (
    <div className="flex flex-col h-full">
      <div className="border-b border-border bg-surface-primary px-6 py-4">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-text-primary">Incident Settings</h1>
            <p className="text-sm text-text-secondary mt-0.5">Configure custom fields and incident schema</p>
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto p-6">
        {error && (
          <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-lg text-red-600 text-sm">{error}</div>
        )}

        {/* Custom Fields card */}
        <div className="bg-surface-primary border border-border rounded-lg">
          <div className="flex items-center justify-between px-6 py-4 border-b border-border">
            <div>
              <h2 className="text-base font-semibold text-text-primary">Custom Fields</h2>
              <p className="text-sm text-text-secondary mt-0.5">
                Define metadata fields that appear on every incident
              </p>
            </div>
            {!showForm && (
              <Button size="sm" onClick={openCreate}>
                <Plus className="w-4 h-4 mr-1" />
                Add field
              </Button>
            )}
          </div>

          {/* Inline form */}
          {showForm && (
            <div className="px-6 py-4 border-b border-border bg-surface-secondary">
              <h3 className="text-sm font-medium text-text-primary mb-3">
                {editingId ? 'Edit field' : 'New field'}
              </h3>
              {formError && (
                <div className="mb-3 p-2 bg-red-50 border border-red-200 rounded text-red-600 text-sm">{formError}</div>
              )}
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-medium text-text-secondary mb-1">Name</label>
                  <input
                    className="w-full px-3 py-1.5 text-sm border border-border rounded bg-surface-primary text-text-primary focus:outline-none focus:ring-1 focus:ring-brand"
                    placeholder="Affected Service"
                    value={form.name}
                    onChange={e => handleNameChange(e.target.value)}
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium text-text-secondary mb-1">Key (snake_case)</label>
                  <input
                    className="w-full px-3 py-1.5 text-sm border border-border rounded bg-surface-primary text-text-primary focus:outline-none focus:ring-1 focus:ring-brand font-mono"
                    placeholder="affected_service"
                    value={form.key}
                    onChange={e => setForm(prev => ({ ...prev, key: e.target.value }))}
                    readOnly={!!editingId}
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium text-text-secondary mb-1">Type</label>
                  <select
                    className="w-full px-3 py-1.5 text-sm border border-border rounded bg-surface-primary text-text-primary focus:outline-none focus:ring-1 focus:ring-brand"
                    value={form.field_type}
                    onChange={e => setForm(prev => ({ ...prev, field_type: e.target.value as FieldType, options: [] }))}
                  >
                    <option value="string">Text</option>
                    <option value="number">Number</option>
                    <option value="dropdown">Dropdown</option>
                  </select>
                </div>
              </div>

              {form.field_type === 'dropdown' && (
                <div className="mt-3">
                  <label className="block text-xs font-medium text-text-secondary mb-1">Options</label>
                  {form.options.map((opt, i) => (
                    <div key={i} className="flex items-center gap-2 mb-1">
                      <span className="text-sm text-text-primary flex-1">{opt.label}</span>
                      <span className="text-xs text-text-secondary font-mono">{opt.value}</span>
                      <button onClick={() => removeOption(i)} className="text-text-tertiary hover:text-red-500">
                        <X className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  ))}
                  <div className="flex gap-2 mt-2">
                    <input
                      className="flex-1 px-2 py-1 text-sm border border-border rounded bg-surface-primary text-text-primary focus:outline-none"
                      placeholder="Label"
                      value={form.newOptionLabel}
                      onChange={e => setForm(prev => ({ ...prev, newOptionLabel: e.target.value }))}
                      onKeyDown={e => e.key === 'Enter' && addOption()}
                    />
                    <input
                      className="w-32 px-2 py-1 text-sm border border-border rounded bg-surface-primary text-text-primary focus:outline-none font-mono"
                      placeholder="value"
                      value={form.newOptionValue}
                      onChange={e => setForm(prev => ({ ...prev, newOptionValue: e.target.value }))}
                      onKeyDown={e => e.key === 'Enter' && addOption()}
                    />
                    <Button size="sm" variant="secondary" onClick={addOption}>Add</Button>
                  </div>
                </div>
              )}

              <div className="flex justify-end gap-2 mt-4">
                <Button size="sm" variant="secondary" onClick={cancelForm}>Cancel</Button>
                <Button size="sm" onClick={handleSave} disabled={saving}>
                  {saving ? 'Saving…' : editingId ? 'Save changes' : 'Create field'}
                </Button>
              </div>
            </div>
          )}

          {/* Field list */}
          {loading ? (
            <div className="px-6 py-8 text-center text-sm text-text-secondary">Loading…</div>
          ) : fields.length === 0 ? (
            <div className="px-6 py-8 text-center text-sm text-text-secondary">
              No custom fields yet. Add one to start capturing extra metadata on incidents.
            </div>
          ) : (
            <table className="w-full">
              <thead>
                <tr className="border-b border-border">
                  <th className="px-6 py-2 text-left text-xs font-medium text-text-secondary w-8"></th>
                  <th className="px-4 py-2 text-left text-xs font-medium text-text-secondary">Name</th>
                  <th className="px-4 py-2 text-left text-xs font-medium text-text-secondary">Key</th>
                  <th className="px-4 py-2 text-left text-xs font-medium text-text-secondary">Type</th>
                  <th className="px-4 py-2 text-left text-xs font-medium text-text-secondary">Options</th>
                  <th className="px-4 py-2 text-right text-xs font-medium text-text-secondary">Actions</th>
                </tr>
              </thead>
              <tbody>
                {fields.map((f, idx) => (
                  <tr key={f.id} className="border-b border-border last:border-0 hover:bg-surface-secondary/50">
                    <td className="px-6 py-3">
                      <div className="flex flex-col gap-0.5">
                        <button
                          disabled={idx === 0}
                          onClick={() => move(idx, -1)}
                          className="text-text-tertiary hover:text-text-primary disabled:opacity-30"
                        >
                          <ChevronUp className="w-3.5 h-3.5" />
                        </button>
                        <button
                          disabled={idx === fields.length - 1}
                          onClick={() => move(idx, 1)}
                          className="text-text-tertiary hover:text-text-primary disabled:opacity-30"
                        >
                          <ChevronDown className="w-3.5 h-3.5" />
                        </button>
                      </div>
                    </td>
                    <td className="px-4 py-3 text-sm text-text-primary font-medium">{f.name}</td>
                    <td className="px-4 py-3 text-sm text-text-secondary font-mono">{f.key}</td>
                    <td className="px-4 py-3">
                      <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-surface-secondary text-text-secondary capitalize">
                        {f.field_type === 'string' ? 'Text' : f.field_type === 'number' ? 'Number' : 'Dropdown'}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-xs text-text-secondary">
                      {f.field_type === 'dropdown' && f.options?.length > 0
                        ? f.options.map(o => o.label).join(', ')
                        : '—'}
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center justify-end gap-1">
                        <button
                          onClick={() => openEdit(f)}
                          className="p-1 rounded text-text-tertiary hover:text-text-primary hover:bg-surface-secondary"
                        >
                          <Pencil className="w-3.5 h-3.5" />
                        </button>
                        <button
                          onClick={() => handleDelete(f)}
                          className="p-1 rounded text-text-tertiary hover:text-red-500 hover:bg-red-50"
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </div>
  )
}
