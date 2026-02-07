import { useState } from 'react'
import { ChevronDown } from 'lucide-react'
import { updateIncident } from '../../api/incidents'
import { Badge } from '../ui/Badge'

type SeverityType = 'critical' | 'high' | 'medium' | 'low'

interface SeverityDropdownProps {
  incidentId: string
  currentSeverity: SeverityType
  onSeverityChange: (newSeverity: SeverityType) => void
  onSuccess: (message: string) => void
  onError: (message: string) => void
}

const SEVERITY_OPTIONS: SeverityType[] = ['critical', 'high', 'medium', 'low']

/**
 * Severity dropdown with optimistic updates
 * Updates UI immediately, calls API, rolls back on error
 */
export function SeverityDropdown({
  incidentId,
  currentSeverity,
  onSeverityChange,
  onSuccess,
  onError,
}: SeverityDropdownProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [isUpdating, setIsUpdating] = useState(false)

  const handleSeverityChange = async (newSeverity: SeverityType) => {
    if (newSeverity === currentSeverity || isUpdating) return

    setIsOpen(false)

    // Save previous severity for rollback
    const previousSeverity = currentSeverity

    // Optimistic update
    onSeverityChange(newSeverity)
    setIsUpdating(true)

    try {
      // Make API call
      await updateIncident(incidentId, { severity: newSeverity })

      // Show success toast
      onSuccess(`Severity updated to ${newSeverity}`)
    } catch (error) {
      // Rollback on error
      onSeverityChange(previousSeverity)
      onError(error instanceof Error ? error.message : 'Failed to update severity')
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
        <Badge variant={currentSeverity} type="severity">
          {currentSeverity}
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
            {SEVERITY_OPTIONS.map((severity) => (
              <button
                key={severity}
                onClick={() => handleSeverityChange(severity)}
                className={`w-full px-3 py-2 text-left text-sm hover:bg-surface-secondary transition-colors flex items-center gap-2 ${
                  severity === currentSeverity ? 'bg-surface-secondary' : ''
                }`}
              >
                <Badge variant={severity} type="severity">
                  {severity}
                </Badge>
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  )
}
