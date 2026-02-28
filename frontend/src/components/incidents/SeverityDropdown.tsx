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
  onRefetch: () => Promise<void>
}

const SEVERITY_OPTIONS: SeverityType[] = ['critical', 'high', 'medium', 'low']

const SEVERITY_INFO: Record<SeverityType, { description: string; dotColor: string }> = {
  critical: {
    description: 'Major customer impact, all hands on deck',
    dotColor: 'bg-red-500',
  },
  high: {
    description: 'Significant impact, core team engaged',
    dotColor: 'bg-orange-500',
  },
  medium: {
    description: 'Partial impact or degraded performance',
    dotColor: 'bg-yellow-500',
  },
  low: {
    description: 'Minor issue, low customer impact',
    dotColor: 'bg-green-500',
  },
}

/**
 * Severity dropdown with descriptions for each level.
 * Updates UI immediately, calls API, rolls back on error.
 */
export function SeverityDropdown({
  incidentId,
  currentSeverity,
  onSeverityChange,
  onSuccess,
  onError,
  onRefetch,
}: SeverityDropdownProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [isUpdating, setIsUpdating] = useState(false)

  const handleSeverityChange = async (newSeverity: SeverityType) => {
    if (newSeverity === currentSeverity || isUpdating) return

    setIsOpen(false)
    setIsUpdating(true)
    const previousSeverity = currentSeverity

    try {
      await updateIncident(incidentId, { severity: newSeverity })
      await onRefetch()
      onSuccess(`Severity updated to ${newSeverity}`)
    } catch (error) {
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
          <div className="fixed inset-0 z-10" onClick={() => setIsOpen(false)} />
          <div className="absolute top-full left-0 mt-2 w-64 bg-white border border-border rounded-lg shadow-lg z-20 py-1">
            {SEVERITY_OPTIONS.map((severity) => {
              const info = SEVERITY_INFO[severity]
              const isSelected = severity === currentSeverity
              return (
                <button
                  key={severity}
                  onClick={() => handleSeverityChange(severity)}
                  className={`w-full px-3 py-2.5 text-left hover:bg-surface-secondary transition-colors flex items-start gap-3 ${
                    isSelected ? 'bg-surface-secondary' : ''
                  }`}
                >
                  <span className={`mt-1.5 flex-shrink-0 w-2 h-2 rounded-full ${info.dotColor}`} />
                  <div>
                    <div className="flex items-center gap-2">
                      <Badge variant={severity} type="severity">{severity}</Badge>
                      {isSelected && (
                        <span className="text-xs text-text-tertiary">current</span>
                      )}
                    </div>
                    <p className="text-xs text-text-secondary mt-0.5 leading-relaxed">
                      {info.description}
                    </p>
                  </div>
                </button>
              )
            })}
          </div>
        </>
      )}
    </div>
  )
}
