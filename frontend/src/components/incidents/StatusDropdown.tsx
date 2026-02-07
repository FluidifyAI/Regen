import { useState } from 'react'
import { ChevronDown } from 'lucide-react'
import { updateIncident } from '../../api/incidents'
import { Badge } from '../ui/Badge'

type StatusType = 'triggered' | 'acknowledged' | 'resolved' | 'canceled'

interface StatusDropdownProps {
  incidentId: string
  currentStatus: StatusType
  onStatusChange: (newStatus: StatusType) => void
  onSuccess: (message: string) => void
  onError: (message: string) => void
}

const STATUS_OPTIONS: StatusType[] = ['triggered', 'acknowledged', 'resolved', 'canceled']

/**
 * Status dropdown with optimistic updates
 * Updates UI immediately, calls API, rolls back on error
 */
export function StatusDropdown({
  incidentId,
  currentStatus,
  onStatusChange,
  onSuccess,
  onError,
}: StatusDropdownProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [isUpdating, setIsUpdating] = useState(false)

  const handleStatusChange = async (newStatus: StatusType) => {
    if (newStatus === currentStatus || isUpdating) return

    setIsOpen(false)

    // Save previous status for rollback
    const previousStatus = currentStatus

    // Optimistic update
    onStatusChange(newStatus)
    setIsUpdating(true)

    try {
      // Make API call
      await updateIncident(incidentId, { status: newStatus })

      // Show success toast
      onSuccess(`Status updated to ${newStatus}`)
    } catch (error) {
      // Rollback on error
      onStatusChange(previousStatus)
      onError(error instanceof Error ? error.message : 'Failed to update status')
    } finally {
      setIsUpdating(false)
    }
  }

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
          {/* Backdrop */}
          <div
            className="fixed inset-0 z-10"
            onClick={() => setIsOpen(false)}
          />

          {/* Dropdown Menu */}
          <div className="absolute top-full left-0 mt-2 w-48 bg-white border border-border rounded-lg shadow-lg z-20 py-1">
            {STATUS_OPTIONS.map((status) => (
              <button
                key={status}
                onClick={() => handleStatusChange(status)}
                className={`w-full px-3 py-2 text-left text-sm hover:bg-surface-secondary transition-colors flex items-center gap-2 ${
                  status === currentStatus ? 'bg-surface-secondary' : ''
                }`}
              >
                <Badge variant={status} type="status">
                  {status}
                </Badge>
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  )
}
