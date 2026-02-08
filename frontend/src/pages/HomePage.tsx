import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button } from '../components/ui/Button'
import { IncidentCard } from '../components/incidents/IncidentCard'
import { CreateIncidentModal } from '../components/incidents/CreateIncidentModal'
import { SkeletonCard } from '../components/ui/Skeleton'
import { EmptyDashboard } from '../components/ui/EmptyState'
import { GeneralError } from '../components/ui/ErrorState'
import { useIncidents } from '../hooks/useIncidents'
import { Plus } from 'lucide-react'
import type { Incident } from '../api/types'

/**
 * Home dashboard with kanban-style incident board
 * Groups active incidents by status (triggered, acknowledged)
 * Shows resolved incidents in separate column
 */
export function HomePage() {
  const navigate = useNavigate()
  const [showCreateModal, setShowCreateModal] = useState(false)

  // Fetch active incidents (not canceled)
  const { incidents, loading, error, refetch } = useIncidents({
    limit: 100,
  })

  const handleDeclareIncident = () => setShowCreateModal(true)

  const handleIncidentCreated = (incident: Incident) => {
    navigate(`/incidents/${incident.id}`)
  }

  // Group incidents by status
  const groupedIncidents = {
    triggered: incidents.filter((i) => i.status === 'triggered'),
    acknowledged: incidents.filter((i) => i.status === 'acknowledged'),
    resolved: incidents.filter((i) => i.status === 'resolved'),
  }

  const activeIncidentsCount =
    groupedIncidents.triggered.length + groupedIncidents.acknowledged.length

  return (
    <div className="flex flex-col h-full">
      <CreateIncidentModal
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        onCreated={handleIncidentCreated}
      />
      {/* Page Header */}
      <div className="border-b border-border bg-white px-6 py-4">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-text-primary">Home</h1>
            <p className="text-sm text-text-secondary mt-1">
              {loading ? 'Loading...' : `${activeIncidentsCount} active incident${activeIncidentsCount !== 1 ? 's' : ''}`}
            </p>
          </div>
          <Button variant="primary" onClick={handleDeclareIncident}>
            <Plus className="w-4 h-4" />
            Declare incident
          </Button>
        </div>
      </div>

      {/* Content Area */}
      <div className="flex-1 overflow-y-auto p-6">
        {loading && (
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            <div>
              <h2 className="text-sm font-semibold text-text-primary mb-4">Triggered</h2>
              <SkeletonCard count={3} />
            </div>
            <div>
              <h2 className="text-sm font-semibold text-text-primary mb-4">Acknowledged</h2>
              <SkeletonCard count={2} />
            </div>
            <div>
              <h2 className="text-sm font-semibold text-text-primary mb-4">Resolved</h2>
              <SkeletonCard count={1} />
            </div>
          </div>
        )}

        {!loading && error && (
          <GeneralError message={error} onRetry={refetch} />
        )}

        {!loading && !error && activeIncidentsCount === 0 && <EmptyDashboard />}

        {!loading && !error && activeIncidentsCount > 0 && (
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            {/* Triggered Column */}
            <KanbanColumn
              title="Triggered"
              count={groupedIncidents.triggered.length}
              incidents={groupedIncidents.triggered}
              statusColor="bg-status-triggered"
            />

            {/* Acknowledged Column */}
            <KanbanColumn
              title="Acknowledged"
              count={groupedIncidents.acknowledged.length}
              incidents={groupedIncidents.acknowledged}
              statusColor="bg-status-acknowledged"
            />

            {/* Resolved Column */}
            <KanbanColumn
              title="Resolved"
              count={groupedIncidents.resolved.length}
              incidents={groupedIncidents.resolved}
              statusColor="bg-status-resolved"
            />
          </div>
        )}
      </div>
    </div>
  )
}

/**
 * Kanban column component
 */
interface KanbanColumnProps {
  title: string
  count: number
  incidents: Incident[]
  statusColor: string
}

function KanbanColumn({ title, count, incidents, statusColor }: KanbanColumnProps) {
  return (
    <div className="flex flex-col">
      {/* Column Header */}
      <div className="flex items-center gap-2 mb-4">
        <div className={`w-3 h-3 rounded-full ${statusColor}`} />
        <h2 className="text-sm font-semibold text-text-primary">{title}</h2>
        <span className="text-xs text-text-tertiary">({count})</span>
      </div>

      {/* Column Content */}
      <div className="space-y-3">
        {incidents.length === 0 ? (
          <div className="text-center py-8 text-sm text-text-tertiary">
            No {title.toLowerCase()} incidents
          </div>
        ) : (
          incidents.map((incident) => (
            <IncidentCard key={incident.id} incident={incident} />
          ))
        )}
      </div>
    </div>
  )
}
