import { Link } from 'react-router-dom'
import { Badge } from '../ui/Badge'
import { Avatar } from '../ui/Avatar'
import { Clock } from 'lucide-react'
import type { Incident } from '../../api/types'

interface IncidentCardProps {
  incident: Incident
}

/**
 * Incident card for kanban board
 * Shows incident number, title, severity badge, commander, and duration
 * Clickable to navigate to incident detail
 */
export function IncidentCard({ incident }: IncidentCardProps) {
  const formatDuration = (startDate: string) => {
    const start = new Date(startDate)
    const now = new Date()
    const diff = now.getTime() - start.getTime()

    const hours = Math.floor(diff / (1000 * 60 * 60))
    const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60))

    if (hours > 24) {
      const days = Math.floor(hours / 24)
      return `${days}d ${hours % 24}h`
    }
    if (hours > 0) {
      return `${hours}h ${minutes}m`
    }
    return `${minutes}m`
  }

  return (
    <Link
      to={`/incidents/${incident.id}`}
      className="block bg-white border border-border rounded-lg p-4 shadow-sm hover:shadow-md transition-shadow cursor-pointer"
    >
      {/* Header */}
      <div className="flex items-center justify-between mb-3">
        <span className="text-xs text-text-tertiary font-medium">
          INC-{incident.incident_number}
        </span>
        <Badge variant={incident.severity} type="severity">
          {incident.severity}
        </Badge>
      </div>

      {/* Title */}
      <h3 className="text-sm font-medium text-text-primary mb-3 line-clamp-2">
        {incident.title}
      </h3>

      {/* Footer */}
      <div className="flex items-center justify-between pt-3 border-t border-border">
        <div className="flex items-center gap-1.5 text-xs text-text-secondary">
          <Clock className="w-3.5 h-3.5" />
          {formatDuration(incident.triggered_at)}
        </div>

        {incident.commander_id ? (
          <Avatar name="Commander" size="sm" />
        ) : (
          <div className="h-6 w-6 rounded-full bg-surface-tertiary flex items-center justify-center">
            <span className="text-xs text-text-tertiary">?</span>
          </div>
        )}
      </div>
    </Link>
  )
}
