import { useState } from 'react'
import { ChevronDown } from 'lucide-react'
import { updateIncident } from '../../api/incidents'
import { Badge } from '../ui/Badge'
import { ConfirmStatusChangeModal } from './ConfirmStatusChangeModal'

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

// Terminal statuses that require modal confirmation before committing
const CONFIRM_REQUIRED: StatusType[] = ['resolved', 'canceled']

/**
 * Status dropdown with modal confirmation for terminal status changes.
 * resolved/canceled show a confirmation modal before committing.
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
    setIsOpen(false)

    if (CONFIRM_REQUIRED.includes(status)) {
      setPendingStatus(status)
    } else {
      commitStatusChange(status)
    }
  }

  const commitStatusChange = async (newStatus: StatusType) => {
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

  return (
    <>
      <ConfirmStatusChangeModal
        isOpen={pendingStatus !== null}
        newStatus={pendingStatus ?? 'resolved'}
        onConfirm={() => pendingStatus && commitStatusChange(pendingStatus)}
        onCancel={() => setPendingStatus(null)}
      />

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
              onClick={() => setIsOpen(false)}
            />
            <div className="absolute top-full left-0 mt-2 w-48 bg-white border border-border rounded-lg shadow-lg z-20 py-1">
              {STATUS_OPTIONS.map((status) => (
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
              ))}
            </div>
          </>
        )}
      </div>
    </>
  )
}
