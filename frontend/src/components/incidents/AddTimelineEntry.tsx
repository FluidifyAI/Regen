import { useState, useRef } from 'react'
import { Plus, Paperclip, X } from 'lucide-react'
import { addTimelineEntry } from '../../api/timeline'
import { uploadAttachment } from '../../api/attachments'
import { Button } from '../ui/Button'

interface AddTimelineEntryProps {
  incidentId: string
  onSuccess: (message: string) => void
  onError: (message: string) => void
  onEntryAdded: () => void
}

export function AddTimelineEntry({
  incidentId,
  onSuccess,
  onError,
  onEntryAdded,
}: AddTimelineEntryProps) {
  const [isExpanded, setIsExpanded] = useState(false)
  const [message, setMessage] = useState('')
  const [files, setFiles] = useState<File[]>([])
  const [isSubmitting, setIsSubmitting] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()

    if ((!message.trim() && files.length === 0) || isSubmitting) return

    setIsSubmitting(true)

    try {
      // Upload files first — if this fails, the note is never posted
      for (const file of files) {
        await uploadAttachment(incidentId, file)
      }

      if (message.trim()) {
        await addTimelineEntry(incidentId, {
          type: 'message',
          content: { message: message.trim() },
        })
      }

      const successMsg = files.length > 0 && message.trim()
        ? 'Note and attachment added'
        : files.length > 0
          ? `${files.length} file${files.length > 1 ? 's' : ''} attached`
          : 'Note added to timeline'

      onSuccess(successMsg)
      setMessage('')
      setFiles([])
      setIsExpanded(false)
      onEntryAdded()
    } catch (error) {
      onError(error instanceof Error ? error.message : 'Failed to add note')
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleCancel = () => {
    setMessage('')
    setFiles([])
    setIsExpanded(false)
  }

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!e.target.files) return
    setFiles((prev) => [...prev, ...Array.from(e.target.files!)])
    e.target.value = ''
  }

  const removeFile = (index: number) => {
    setFiles((prev) => prev.filter((_, i) => i !== index))
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

      {/* Attached file chips */}
      {files.length > 0 && (
        <div className="flex flex-wrap gap-1.5 mt-2">
          {files.map((f, i) => (
            <span
              key={i}
              className="inline-flex items-center gap-1 px-2 py-0.5 text-xs bg-surface-secondary border border-border rounded-full text-text-secondary"
            >
              <Paperclip className="w-3 h-3" />
              {f.name}
              {!isSubmitting && (
                <button
                  type="button"
                  onClick={() => removeFile(i)}
                  className="ml-0.5 hover:text-red-500 transition-colors"
                >
                  <X className="w-3 h-3" />
                </button>
              )}
            </span>
          ))}
        </div>
      )}

      <div className="flex items-center justify-between mt-3">
        {/* Attach file button */}
        <button
          type="button"
          onClick={() => fileInputRef.current?.click()}
          disabled={isSubmitting}
          className="flex items-center gap-1.5 px-2 py-1 text-xs text-text-tertiary hover:text-text-primary hover:bg-surface-secondary rounded transition-colors disabled:opacity-50"
          title="Attach file"
        >
          <Paperclip className="w-3.5 h-3.5" />
          Attach
        </button>
        <input
          ref={fileInputRef}
          type="file"
          multiple
          className="hidden"
          accept="image/*,application/pdf,text/plain,text/csv,application/json,application/zip"
          onChange={handleFileChange}
        />

        <div className="flex items-center gap-2">
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
            disabled={(!message.trim() && files.length === 0) || isSubmitting}
            loading={isSubmitting}
          >
            Add note
          </Button>
        </div>
      </div>
    </form>
  )
}
