import { type FormEvent, useEffect, useState } from 'react'
import { X, AlertTriangle } from 'lucide-react'
import { Button } from '../ui/Button'
import { createIncident } from '../../api/incidents'
import type { Incident } from '../../api/types'

interface CreateIncidentModalProps {
  isOpen: boolean
  onClose: () => void
  onCreated: (incident: Incident) => void
}

// ─── Severity icons ───────────────────────────────────────────────────────────

/** Signal-bar icon — filled indicates active bars */
function SignalBars({ filled, color }: { filled: 1 | 2 | 3; color: string }) {
  const bars = [
    { x: 1,  y: 14, h: 6  },
    { x: 7,  y: 9,  h: 11 },
    { x: 13, y: 4,  h: 16 },
  ]
  return (
    <svg width="20" height="20" viewBox="0 0 20 20" fill="none" aria-hidden>
      {bars.map((b, i) => (
        <rect
          key={i}
          x={b.x}
          y={b.y}
          width="4"
          height={b.h}
          rx="1.5"
          fill={i < filled ? color : '#D1D5DB'}
        />
      ))}
    </svg>
  )
}

/** Red alert badge for Critical */
function CriticalBadge() {
  return (
    <span className="inline-flex items-center justify-center w-5 h-5 rounded bg-red-600 text-white">
      <AlertTriangle className="w-3 h-3" strokeWidth={2.5} />
    </span>
  )
}

// ─── Severity config ──────────────────────────────────────────────────────────

type Severity = 'critical' | 'high' | 'medium' | 'low'

const SEVERITIES: {
  value: Severity
  label: string
  description: string
  icon: React.ReactNode
  selectedBg: string
  selectedBorder: string
  selectedText: string
}[] = [
  {
    value: 'critical',
    label: 'Critical',
    description: 'System down',
    icon: <CriticalBadge />,
    selectedBg: 'bg-red-50',
    selectedBorder: 'border-red-400',
    selectedText: 'text-red-700',
  },
  {
    value: 'high',
    label: 'High',
    description: 'Major impact',
    icon: <SignalBars filled={3} color="#F97316" />,
    selectedBg: 'bg-orange-50',
    selectedBorder: 'border-orange-400',
    selectedText: 'text-orange-700',
  },
  {
    value: 'medium',
    label: 'Medium',
    description: 'Partial impact',
    icon: <SignalBars filled={2} color="#EAB308" />,
    selectedBg: 'bg-amber-50',
    selectedBorder: 'border-amber-400',
    selectedText: 'text-amber-700',
  },
  {
    value: 'low',
    label: 'Low',
    description: 'Minor issue',
    icon: <SignalBars filled={1} color="#22C55E" />,
    selectedBg: 'bg-emerald-50',
    selectedBorder: 'border-emerald-400',
    selectedText: 'text-emerald-700',
  },
]

// ─── Modal ────────────────────────────────────────────────────────────────────

/**
 * Modal form for declaring a new incident manually.
 * Handles title validation, severity selection, optional summary,
 * loading/error states, and resets on close.
 */
export function CreateIncidentModal({ isOpen, onClose, onCreated }: CreateIncidentModalProps) {
  const [title, setTitle] = useState('')
  const [severity, setSeverity] = useState<Severity>('high')
  const [summary, setSummary] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!isOpen) return
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [isOpen, onClose])

  useEffect(() => {
    if (!isOpen) {
      setTitle('')
      setSeverity('high')
      setSummary('')
      setError(null)
    }
  }, [isOpen])

  if (!isOpen) return null

  const handleSubmit = async (e: FormEvent | React.MouseEvent) => {
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
        ai_enabled: false,
      })
      onCreated(incident)
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create incident')
    } finally {
      setIsSubmitting(false)
    }
  }

  const selectedSev = SEVERITIES.find((s) => s.value === severity)!

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm"
      onClick={onClose}
    >
      <div
        className="bg-white rounded-2xl shadow-2xl w-full max-w-md mx-4 overflow-hidden"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-labelledby="modal-title"
      >
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-5 border-b border-border">
          <h2 id="modal-title" className="text-base font-semibold text-text-primary">
            Declare Incident
          </h2>
          <button
            onClick={onClose}
            className="w-7 h-7 flex items-center justify-center rounded-full text-text-tertiary hover:text-text-primary hover:bg-surface-secondary transition-colors"
            aria-label="Close"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} className="px-6 py-5 space-y-5">
          {/* Error */}
          {error && (
            <div className="px-3 py-2 bg-red-50 border border-red-200 text-red-700 text-sm rounded-lg">
              {error}
            </div>
          )}

          {/* Title */}
          <div>
            <label htmlFor="incident-title" className="block text-sm font-medium text-text-primary mb-1.5">
              Title <span className="text-red-500">*</span>
            </label>
            <input
              id="incident-title"
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="e.g., API Gateway 5xx errors"
              className="w-full px-3 py-2.5 border border-border rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary/30 focus:border-brand-primary transition-colors"
              autoFocus
              disabled={isSubmitting}
            />
          </div>

          {/* Severity — custom visual picker */}
          <div>
            <label className="block text-sm font-medium text-text-primary mb-2">
              Severity
            </label>
            <div className="grid grid-cols-4 gap-2">
              {SEVERITIES.map((sev) => {
                const isSelected = severity === sev.value
                return (
                  <button
                    key={sev.value}
                    type="button"
                    onClick={() => setSeverity(sev.value)}
                    disabled={isSubmitting}
                    className={[
                      'flex flex-col items-center gap-1.5 px-2 py-3 rounded-xl border-2 transition-all text-center',
                      isSelected
                        ? `${sev.selectedBg} ${sev.selectedBorder} ${sev.selectedText}`
                        : 'border-border bg-white text-text-secondary hover:border-gray-300 hover:bg-surface-secondary',
                    ].join(' ')}
                  >
                    {sev.icon}
                    <span className="text-xs font-semibold leading-none">{sev.label}</span>
                    <span className={`text-[10px] leading-none ${isSelected ? 'opacity-70' : 'text-text-tertiary'}`}>
                      {sev.description}
                    </span>
                  </button>
                )
              })}
            </div>
            {/* Selected severity pill */}
            <div className={`mt-2 flex items-center gap-1.5 text-xs font-medium ${selectedSev.selectedText}`}>
              <span className="opacity-60">Selected:</span>
              <span>{selectedSev.label} — {selectedSev.description}</span>
            </div>
          </div>

          {/* Summary */}
          <div>
            <label htmlFor="incident-summary" className="block text-sm font-medium text-text-primary mb-1.5">
              Summary{' '}
              <span className="text-text-tertiary text-xs font-normal">(optional)</span>
            </label>
            <textarea
              id="incident-summary"
              value={summary}
              onChange={(e) => setSummary(e.target.value)}
              placeholder="Brief description of what's happening"
              rows={3}
              className="w-full px-3 py-2.5 border border-border rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary/30 focus:border-brand-primary transition-colors resize-none"
              disabled={isSubmitting}
            />
          </div>
        </form>

        {/* Footer */}
        <div className="flex items-center justify-end gap-3 px-6 py-4 border-t border-border bg-surface-secondary/50">
          <Button variant="secondary" onClick={onClose} disabled={isSubmitting}>
            Cancel
          </Button>
          <Button variant="primary" onClick={handleSubmit as never} loading={isSubmitting}>
            Declare incident
          </Button>
        </div>
      </div>
    </div>
  )
}
