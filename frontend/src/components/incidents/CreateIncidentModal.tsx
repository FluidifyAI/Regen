import { type FormEvent, useEffect, useState } from 'react'
import { X } from 'lucide-react'
import { Button } from '../ui/Button'
import { createIncident } from '../../api/incidents'
import type { Incident } from '../../api/types'

interface CreateIncidentModalProps {
  isOpen: boolean
  onClose: () => void
  onCreated: (incident: Incident) => void
}

/**
 * Modal form for declaring a new incident manually.
 * Handles title validation, severity selection, optional summary,
 * loading/error states, and resets on close.
 */
export function CreateIncidentModal({ isOpen, onClose, onCreated }: CreateIncidentModalProps) {
  const [title, setTitle] = useState('')
  const [severity, setSeverity] = useState<'critical' | 'high' | 'medium' | 'low'>('high')
  const [summary, setSummary] = useState('')
  const [aiEnabled, setAiEnabled] = useState(true)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Close on Escape key
  useEffect(() => {
    if (!isOpen) return
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [isOpen, onClose])

  // Reset form when modal closes
  useEffect(() => {
    if (!isOpen) {
      setTitle('')
      setSeverity('high')
      setSummary('')
      setAiEnabled(true)
      setError(null)
    }
  }, [isOpen])

  if (!isOpen) return null

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!title.trim()) {
      setError('Title is required')
      return
    }
    setIsSubmitting(true)
    setError(null)
    try {
      const incident = await createIncident({
        title: title.trim(),
        severity,
        description: summary.trim() || undefined,
        ai_enabled: aiEnabled,
      })
      onCreated(incident)
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create incident')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40"
      onClick={onClose}
    >
      <div
        className="bg-white rounded-lg shadow-xl w-full max-w-md mx-4"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-labelledby="modal-title"
      >
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-border">
          <h2 id="modal-title" className="text-lg font-semibold text-text-primary">
            Declare Incident
          </h2>
          <button
            onClick={onClose}
            className="text-text-tertiary hover:text-text-primary transition-colors"
            aria-label="Close"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} className="px-6 py-4 space-y-4">
          {/* Error */}
          {error && (
            <div className="px-3 py-2 bg-red-50 border border-red-200 text-red-700 text-sm rounded">
              {error}
            </div>
          )}

          {/* Title */}
          <div>
            <label htmlFor="incident-title" className="block text-sm font-medium text-text-primary mb-1">
              Title <span className="text-red-500">*</span>
            </label>
            <input
              id="incident-title"
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="e.g., API Gateway 5xx errors"
              className="w-full px-3 py-2 border border-border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary focus:border-transparent"
              autoFocus
              disabled={isSubmitting}
            />
          </div>

          {/* Severity */}
          <div>
            <label htmlFor="incident-severity" className="block text-sm font-medium text-text-primary mb-1">
              Severity
            </label>
            <select
              id="incident-severity"
              value={severity}
              onChange={(e) => setSeverity(e.target.value as typeof severity)}
              className="w-full px-3 py-2 border border-border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary focus:border-transparent"
              disabled={isSubmitting}
            >
              <option value="critical">🔴 Critical</option>
              <option value="high">🟠 High</option>
              <option value="medium">🟡 Medium</option>
              <option value="low">🟢 Low</option>
            </select>
          </div>

          {/* Summary */}
          <div>
            <label htmlFor="incident-summary" className="block text-sm font-medium text-text-primary mb-1">
              Summary <span className="text-text-tertiary text-xs">(optional)</span>
            </label>
            <textarea
              id="incident-summary"
              value={summary}
              onChange={(e) => setSummary(e.target.value)}
              placeholder="Brief description of what's happening"
              rows={3}
              className="w-full px-3 py-2 border border-border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary focus:border-transparent resize-none"
              disabled={isSubmitting}
            />
          </div>

          {/* AI Agents toggle */}
          <div className="flex items-center justify-between py-2 border-t border-border mt-2">
            <div>
              <p className="text-sm font-medium text-text-primary">AI Agents</p>
              <p className="text-xs text-text-tertiary">Auto-draft post-mortem when resolved</p>
            </div>
            <button
              type="button"
              role="switch"
              aria-checked={aiEnabled}
              onClick={() => setAiEnabled(v => !v)}
              className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${
                aiEnabled ? 'bg-brand-primary' : 'bg-gray-200'
              }`}
            >
              <span className={`inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform ${
                aiEnabled ? 'translate-x-4' : 'translate-x-0.5'
              }`} />
            </button>
          </div>
        </form>

        {/* Footer */}
        <div className="flex items-center justify-end gap-3 px-6 py-4 border-t border-border">
          <Button variant="secondary" onClick={onClose} disabled={isSubmitting}>
            Cancel
          </Button>
          <Button variant="primary" onClick={handleSubmit as never} loading={isSubmitting}>
            Create incident
          </Button>
        </div>
      </div>
    </div>
  )
}
