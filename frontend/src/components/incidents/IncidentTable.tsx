import { Link } from 'react-router-dom'
import { Badge } from '../ui/Badge'
import { Avatar } from '../ui/Avatar'
import type { Incident } from '../../api/types'

interface IncidentTableProps {
  incidents: Incident[]
  onIncidentClick?: (id: string) => void
}

/**
 * Incidents table with responsive design
 * Columns: Incident, Severity, Status, Created, Commander
 * Rows are clickable to navigate to incident detail
 */
export function IncidentTable({ incidents, onIncidentClick }: IncidentTableProps) {
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

  const formatTimeAgo = (dateString: string) => {
    const date = new Date(dateString)
    const now = new Date()
    const diff = now.getTime() - date.getTime()

    const hours = Math.floor(diff / (1000 * 60 * 60))
    const days = Math.floor(hours / 24)

    if (days > 0) return `${days}d ago`
    if (hours > 0) return `${hours}h ago`
    return 'Just now'
  }

  return (
    <div className="bg-white border border-border rounded-lg overflow-hidden">
      {/* Table Header */}
      <div className="grid grid-cols-12 gap-4 px-4 py-3 border-b border-border bg-surface-tertiary text-xs font-medium text-text-tertiary uppercase tracking-wider">
        <div className="col-span-1"></div>
        <div className="col-span-3">Incident</div>
        <div className="col-span-2">Severity</div>
        <div className="col-span-2">Status</div>
        <div className="col-span-1">Duration</div>
        <div className="col-span-2">Reported</div>
        <div className="col-span-1">Lead</div>
      </div>

      {/* Table Rows */}
      <div className="divide-y divide-border">
        {incidents.map((incident) => (
          <Link
            key={incident.id}
            to={`/incidents/${incident.id}`}
            onClick={() => onIncidentClick?.(incident.id)}
            className="grid grid-cols-12 gap-4 px-4 py-4 hover:bg-surface-secondary transition-colors cursor-pointer"
          >
            {/* Checkbox */}
            <div className="col-span-1 flex items-center">
              <input
                type="checkbox"
                className="h-4 w-4 rounded border-border text-brand-primary focus:ring-brand-primary"
                onClick={(e) => e.stopPropagation()}
              />
            </div>

            {/* Incident Number + Title */}
            <div className="col-span-3 space-y-1">
              <div className="text-xs text-text-tertiary">INC-{incident.incident_number}</div>
              <div className="text-sm font-medium text-text-primary line-clamp-2">
                {incident.title}
              </div>
            </div>

            {/* Severity */}
            <div className="col-span-2 flex items-center">
              <Badge variant={incident.severity} type="severity">
                {incident.severity}
              </Badge>
            </div>

            {/* Status */}
            <div className="col-span-2 flex items-center">
              <Badge variant={incident.status} type="status">
                {incident.status}
              </Badge>
            </div>

            {/* Duration */}
            <div className="col-span-1 flex items-center text-sm text-text-secondary">
              {formatDuration(incident.triggered_at)}
            </div>

            {/* Reported */}
            <div className="col-span-2 flex items-center text-sm text-text-secondary">
              {formatTimeAgo(incident.created_at)}
            </div>

            {/* Commander */}
            <div className="col-span-1 flex items-center gap-2">
              {incident.commander_id ? (
                <>
                  <Avatar name="Commander" size="sm" />
                  <span className="text-sm text-text-secondary truncate">Lead</span>
                </>
              ) : (
                <span className="text-sm text-text-tertiary">Unassigned</span>
              )}
            </div>
          </Link>
        ))}
      </div>
    </div>
  )
}
