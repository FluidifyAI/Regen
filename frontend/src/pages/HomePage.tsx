import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  DndContext,
  DragEndEvent,
  DragOverlay,
  DragStartEvent,
  PointerSensor,
  useSensor,
  useSensors,
  useDroppable,
} from '@dnd-kit/core'
import { SortableContext, verticalListSortingStrategy, useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { updateIncident } from '../api/incidents'
import { Button } from '../components/ui/Button'
import { IncidentCard } from '../components/incidents/IncidentCard'
import { CreateIncidentModal } from '../components/incidents/CreateIncidentModal'
import { ConfirmStatusChangeModal } from '../components/incidents/ConfirmStatusChangeModal'
import { SkeletonCard } from '../components/ui/Skeleton'
import { EmptyDashboard } from '../components/ui/EmptyState'
import { GeneralError } from '../components/ui/ErrorState'
import { useIncidents } from '../hooks/useIncidents'
import { useToast, ToastContainer } from '../components/ui/Toast'
import { Plus } from 'lucide-react'
import type { Incident } from '../api/types'

/**
 * Home dashboard with kanban-style incident board
 * Groups active incidents by status (triggered, acknowledged)
 * Shows resolved incidents in separate column
 */
type DragStatus = 'triggered' | 'acknowledged' | 'resolved' | 'canceled'

interface PendingDrag {
  incidentId: string
  newStatus: DragStatus
}

export function HomePage() {
  const navigate = useNavigate()
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [activeIncident, setActiveIncident] = useState<Incident | null>(null)
  const [pendingDrag, setPendingDrag] = useState<PendingDrag | null>(null)
  const { toasts, dismissToast, success, error: showError } = useToast()

  // Fetch active incidents (not canceled)
  const { incidents, loading, error, refetch } = useIncidents({
    limit: 100,
  })

  // Configure drag sensors
  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: {
        distance: 8, // 8px drag before activating (prevents accidental drags when clicking)
      },
    })
  )

  const handleDeclareIncident = () => setShowCreateModal(true)

  const handleIncidentCreated = (incident: Incident) => {
    navigate(`/incidents/${incident.id}`)
  }

  const handleDragStart = (event: DragStartEvent) => {
    const incidentId = event.active.id as string
    const incident = incidents.find((i) => i.id === incidentId)
    setActiveIncident(incident || null)
  }

  const VALID_STATUSES = ['triggered', 'acknowledged', 'resolved', 'canceled'] as const

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event
    setActiveIncident(null)

    if (!over) return

    const incidentId = active.id as string
    const overId = over.id as string

    // Resolve target status: over.id is either a column ID (status string)
    // or an incident card ID (UUID) when dropped onto another card
    let newStatus: DragStatus
    if (VALID_STATUSES.includes(overId as DragStatus)) {
      newStatus = overId as DragStatus
    } else {
      // Dropped on a card — find which column that card belongs to
      const targetCard = incidents.find((i) => i.id === overId)
      if (!targetCard) return
      newStatus = targetCard.status as DragStatus
    }

    // Find the incident being dragged
    const incident = incidents.find((i) => i.id === incidentId)
    if (!incident || incident.status === newStatus) return

    // Terminal statuses require modal confirmation before committing
    if (newStatus === 'resolved' || newStatus === 'canceled') {
      setPendingDrag({ incidentId, newStatus })
      return
    }

    commitDrag(incidentId, newStatus)
  }

  const commitDrag = async (incidentId: string, newStatus: DragStatus) => {
    setPendingDrag(null)
    try {
      await updateIncident(incidentId, { status: newStatus })
      await refetch()
      success(`Incident moved to ${newStatus}`)
    } catch (err) {
      await refetch()
      showError(err instanceof Error ? err.message : 'Failed to update status')
    }
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
      <ConfirmStatusChangeModal
        isOpen={pendingDrag !== null}
        newStatus={pendingDrag?.newStatus ?? 'resolved'}
        onConfirm={() => pendingDrag && commitDrag(pendingDrag.incidentId, pendingDrag.newStatus)}
        onCancel={() => setPendingDrag(null)}
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
          <DndContext
            sensors={sensors}
            onDragStart={handleDragStart}
            onDragEnd={handleDragEnd}
          >
            <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
              {/* Triggered Column */}
              <KanbanColumn
                id="triggered"
                title="Triggered"
                count={groupedIncidents.triggered.length}
                incidents={groupedIncidents.triggered}
                statusColor="bg-status-triggered"
              />

              {/* Acknowledged Column */}
              <KanbanColumn
                id="acknowledged"
                title="Acknowledged"
                count={groupedIncidents.acknowledged.length}
                incidents={groupedIncidents.acknowledged}
                statusColor="bg-status-acknowledged"
              />

              {/* Resolved Column */}
              <KanbanColumn
                id="resolved"
                title="Resolved"
                count={groupedIncidents.resolved.length}
                incidents={groupedIncidents.resolved}
                statusColor="bg-status-resolved"
              />
            </div>

            {/* Drag Overlay - shows dragged card while dragging */}
            <DragOverlay>
              {activeIncident ? (
                <div className="opacity-80 rotate-3 scale-105">
                  <IncidentCard incident={activeIncident} isDragging />
                </div>
              ) : null}
            </DragOverlay>
          </DndContext>
        )}
      </div>

      {/* Toast Notifications */}
      <ToastContainer toasts={toasts} onDismiss={dismissToast} />
    </div>
  )
}

/**
 * Kanban column component
 */
interface KanbanColumnProps {
  id: string
  title: string
  count: number
  incidents: Incident[]
  statusColor: string
}

function KanbanColumn({ id, title, count, incidents, statusColor }: KanbanColumnProps) {
  const { setNodeRef, isOver } = useDroppable({
    id: id,
  })

  return (
    <div className="flex flex-col">
      {/* Column Header */}
      <div className="flex items-center gap-2 mb-4">
        <div className={`w-3 h-3 rounded-full ${statusColor}`} />
        <h2 className="text-sm font-semibold text-text-primary">{title}</h2>
        <span className="text-xs text-text-tertiary">({count})</span>
      </div>

      {/* Droppable Column Content */}
      <div
        ref={setNodeRef}
        className={`space-y-3 min-h-[200px] rounded-lg border-2 border-dashed p-3 transition-colors ${
          isOver
            ? 'border-brand-primary bg-brand-primary/5'
            : 'border-transparent'
        }`}
      >
        <SortableContext items={incidents.map((i) => i.id)} strategy={verticalListSortingStrategy}>
          {incidents.length === 0 ? (
            <div className="text-center py-8 text-sm text-text-tertiary">
              No {title.toLowerCase()} incidents
            </div>
          ) : (
            incidents.map((incident) => (
              <DraggableIncidentCard key={incident.id} incident={incident} />
            ))
          )}
        </SortableContext>
      </div>
    </div>
  )
}

/**
 * Draggable wrapper for incident cards
 */
function DraggableIncidentCard({ incident }: { incident: Incident }) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: incident.id })

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  }

  return (
    <div ref={setNodeRef} style={style} {...attributes} {...listeners}>
      <IncidentCard incident={incident} isDragging={isDragging} />
    </div>
  )
}
