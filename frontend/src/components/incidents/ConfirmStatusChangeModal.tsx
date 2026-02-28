import { AlertTriangle } from 'lucide-react'

type StatusType = 'triggered' | 'acknowledged' | 'resolved' | 'canceled'

interface ConfirmStatusChangeModalProps {
  isOpen: boolean
  newStatus: StatusType
  onConfirm: () => void
  onCancel: () => void
}

const CONFIRM_COPY: Record<string, { title: string; description: string; confirmLabel: string }> = {
  resolved: {
    title: 'Resolve this incident?',
    description: 'The incident will be marked as resolved. A timeline entry will be created and any active escalations will stop.',
    confirmLabel: 'Resolve incident',
  },
  canceled: {
    title: 'Cancel this incident?',
    description: 'The incident will be canceled. This cannot be undone.',
    confirmLabel: 'Cancel incident',
  },
}

/**
 * Modal confirmation dialog for terminal status changes (resolved, canceled).
 * Used by StatusDropdown and Kanban drag-to-column.
 */
export function ConfirmStatusChangeModal({
  isOpen,
  newStatus,
  onConfirm,
  onCancel,
}: ConfirmStatusChangeModalProps) {
  if (!isOpen) return null

  const copy = CONFIRM_COPY[newStatus] ?? {
    title: `Change status to ${newStatus}?`,
    description: 'Are you sure you want to change the incident status?',
    confirmLabel: 'Confirm',
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-black/50"
        onClick={onCancel}
      />

      {/* Dialog */}
      <div className="relative bg-white rounded-xl border border-border shadow-xl w-full max-w-md p-6">
        {/* Icon + Title */}
        <div className="flex items-start gap-4 mb-4">
          <div className="flex-shrink-0 w-10 h-10 rounded-full bg-amber-100 flex items-center justify-center">
            <AlertTriangle className="w-5 h-5 text-amber-600" />
          </div>
          <div>
            <h2 className="text-base font-semibold text-text-primary">{copy.title}</h2>
            <p className="text-sm text-text-secondary mt-1 leading-relaxed">{copy.description}</p>
          </div>
        </div>

        {/* Actions */}
        <div className="flex justify-end gap-3 mt-6">
          <button
            onClick={onCancel}
            className="px-4 py-2 rounded-lg border border-border text-sm font-medium text-text-secondary hover:text-text-primary hover:bg-surface-secondary transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={onConfirm}
            className="px-4 py-2 rounded-lg bg-brand-primary hover:bg-brand-primary/90 text-white text-sm font-medium transition-colors"
          >
            {copy.confirmLabel}
          </button>
        </div>
      </div>
    </div>
  )
}
