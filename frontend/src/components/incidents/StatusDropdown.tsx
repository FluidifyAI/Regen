import { useState } from 'react'
import { ChevronDown, AlertTriangle } from 'lucide-react'
import { updateIncident } from '../../api/incidents'
import { Badge } from '../ui/Badge'

type StatusType = 'triggered' | 'acknowledged' | 'resolved' | 'canceled'

interface StatusDropdownProps {
  incidentId: string
  currentStatus: StatusType
  onStatusChange: (newStatus: StatusType) => void
  onSuccess: (message: string) => void
  onError: (message: string) => void
  onRefetch: () => Promise<void>
}

const STATUS_OPTIONS: StatusType[] = ['triggered', 'acknowledged', 'resolved', 'canceled']

// Terminal statuses that require a confirmation step before committing
const CONFIRM_REQUIRED: StatusType[] = ['resolved', 'canceled']

const CONFIRM_MESSAGES: Partial<Record<StatusType, string>> = {
  resolved: 'Mark this incident as resolved?',
  canceled: 'Cancel this incident? This cannot be undone.',
}

/**
 * Status dropdown with inline confirmation for terminal status changes.
 * resolved/canceled show a confirmation step before committing.
 */
export function StatusDropdown({
  incidentId,
  currentStatus,
  onStatusChange,
  onSuccess,
  onError,
  onRefetch,
}: StatusDropdownProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [isUpdating, setIsUpdating] = useState(false)
  const [pendingStatus, setPendingStatus] = useState<StatusType | null>(null)

  const handleStatusClick = (status: StatusType) => {
    if (status === currentStatus || isUpdating) return

    if (CONFIRM_REQUIRED.includes(status)) {
      // Show inline confirmation instead of immediately committing
      setPendingStatus(status)
    } else {
      commitStatusChange(status)
    }
  }

  const commitStatusChange = async (newStatus: StatusType) => {
    setIsOpen(false)
    setPendingStatus(null)
    setIsUpdating(true)
    const previousStatus = currentStatus

    try {
      await updateIncident(incidentId, { status: newStatus })
      await onRefetch()
      onSuccess(`Status updated to ${newStatus}`)
    } catch (error) {
      onStatusChange(previousStatus)
      onError(error instanceof Error ? error.message : 'Failed to update status')
    } finally {
      setIsUpdating(false)
    }
  }

  const cancelPending = () => setPendingStatus(null)

  return (
    <div className="relative">
      <button
        onClick={() => !isUpdating && setIsOpen(!isOpen)}
        disabled={isUpdating}
        className={`flex items-center gap-2 px-3 py-2 rounded-lg border border-border hover:bg-surface-secondary transition-colors ${
          isUpdating ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'
        }`}
      >
        <Badge variant={currentStatus} type="status">
          {currentStatus}
        </Badge>
        <ChevronDown className="w-4 h-4 text-text-tertiary" />
      </button>

      {isOpen && !isUpdating && (
        <>
          <div
            className="fixed inset-0 z-10"
            onClick={() => { setIsOpen(false); setPendingStatus(null) }}
          />
          <div className="absolute top-full left-0 mt-2 w-56 bg-white border border-border rounded-lg shadow-lg z-20 py-1">
            {pendingStatus ? (
              /* Inline confirmation step */
              <div className="px-3 py-3">
                <div className="flex items-start gap-2 mb-3">
                  <AlertTriangle className="w-4 h-4 text-amber-500 flex-shrink-0 mt-0.5" />
                  <p className="text-sm text-text-primary leading-snug">
                    {CONFIRM_MESSAGES[pendingStatus]}
                  </p>
                </div>
                <div className="flex gap-2">
                  <button
                    onClick={() => commitStatusChange(pendingStatus)}
                    className="flex-1 px-3 py-1.5 rounded-md bg-brand-primary hover:bg-brand-primary/90 text-white text-xs font-medium transition-colors"
                  >
                    Confirm
                  </button>
                  <button
                    onClick={cancelPending}
                    className="flex-1 px-3 py-1.5 rounded-md border border-border text-text-secondary hover:text-text-primary text-xs font-medium transition-colors"
                  >
                    Cancel
                  </button>
                </div>
              </div>
            ) : (
              /* Normal status list */
              STATUS_OPTIONS.map((status) => (
                <button
                  key={status}
                  onClick={() => handleStatusClick(status)}
                  className={`w-full px-3 py-2 text-left text-sm hover:bg-surface-secondary transition-colors flex items-center gap-2 ${
                    status === currentStatus ? 'bg-surface-secondary' : ''
                  }`}
                >
                  <Badge variant={status} type="status">
                    {status}
                  </Badge>
                </button>
              ))
            )}
          </div>
        </>
      )}
    </div>
  )
}
