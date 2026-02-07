import { useState } from 'react'
import { Plus } from 'lucide-react'
import { addTimelineEntry } from '../../api/timeline'
import { Button } from '../ui/Button'

interface AddTimelineEntryProps {
  incidentId: string
  onSuccess: (message: string) => void
  onError: (message: string) => void
  onEntryAdded: () => void
}

/**
 * Form for adding timeline entries (notes/messages)
 * Shows input field with submit button
 */
export function AddTimelineEntry({
  incidentId,
  onSuccess,
  onError,
  onEntryAdded,
}: AddTimelineEntryProps) {
  const [isExpanded, setIsExpanded] = useState(false)
  const [message, setMessage] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()

    if (!message.trim() || isSubmitting) return

    setIsSubmitting(true)

    try {
      await addTimelineEntry(incidentId, {
        type: 'message',
        content: { message: message.trim() },
      })

      onSuccess('Note added to timeline')
      setMessage('')
      setIsExpanded(false)
      onEntryAdded() // Trigger refetch
    } catch (error) {
      onError(error instanceof Error ? error.message : 'Failed to add note')
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleCancel = () => {
    setMessage('')
    setIsExpanded(false)
  }

  if (!isExpanded) {
    return (
      <button
        onClick={() => setIsExpanded(true)}
        className="flex items-center gap-2 px-4 py-2 text-sm text-text-secondary hover:text-text-primary hover:bg-surface-secondary rounded-lg transition-colors"
      >
        <Plus className="w-4 h-4" />
        Add a note
      </button>
    )
  }

  return (
    <form onSubmit={handleSubmit} className="border border-border rounded-lg p-4">
      <textarea
        value={message}
        onChange={(e) => setMessage(e.target.value)}
        placeholder="Add a note to the timeline..."
        rows={3}
        autoFocus
        disabled={isSubmitting}
        className="w-full px-3 py-2 border border-border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary focus:border-transparent resize-none disabled:opacity-50"
      />

      <div className="flex items-center justify-end gap-2 mt-3">
        <Button
          type="button"
          variant="ghost"
          onClick={handleCancel}
          disabled={isSubmitting}
        >
          Cancel
        </Button>
        <Button
          type="submit"
          variant="primary"
          disabled={!message.trim() || isSubmitting}
          loading={isSubmitting}
        >
          Add note
        </Button>
      </div>
    </form>
  )
}
