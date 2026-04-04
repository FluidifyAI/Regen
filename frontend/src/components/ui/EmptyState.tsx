import { Shield, CheckCircle, Clock, Sparkles } from 'lucide-react'
import { Button } from './Button'

interface EmptyStateProps {
  icon?: 'shield' | 'check' | 'clock'
  title: string
  description: string
  actionLabel?: string
  onAction?: () => void
}

/**
 * Reusable empty state component for when no data is present
 * Variants: incidents list, home dashboard, timeline
 * Centered with muted colors and optional action button
 */
export function EmptyState({
  icon = 'shield',
  title,
  description,
  actionLabel,
  onAction,
}: EmptyStateProps) {
  const iconMap = {
    shield: Shield,
    check: CheckCircle,
    clock: Clock,
  }

  const IconComponent = iconMap[icon]

  return (
    <div className="flex items-center justify-center min-h-[400px] px-4">
      <div className="text-center max-w-md">
        <IconComponent className="w-12 h-12 mx-auto mb-4 text-text-tertiary" />
        <h3 className="text-lg font-semibold text-text-primary mb-2">{title}</h3>
        <p className="text-sm text-text-secondary mb-6">{description}</p>
        {actionLabel && onAction && (
          <Button variant="primary" onClick={onAction}>
            {actionLabel}
          </Button>
        )}
      </div>
    </div>
  )
}

/**
 * Pre-configured empty state for incidents list.
 * hasFilters=true → "no matches" message; false → true first-time empty state.
 */
export function EmptyIncidentsList({
  onDeclare,
  hasFilters = false,
}: {
  onDeclare?: () => void
  hasFilters?: boolean
}) {
  if (hasFilters) {
    return (
      <EmptyState
        icon="shield"
        title="No incidents match your filters"
        description="Try adjusting your search, status, or severity filters to find what you're looking for."
      />
    )
  }
  return (
    <EmptyState
      icon="shield"
      title="No incidents yet"
      description="Declare your first incident manually, or connect a monitoring tool to receive alerts automatically."
      actionLabel={onDeclare ? 'Declare incident' : undefined}
      onAction={onDeclare}
    />
  )
}

/**
 * Pre-configured empty state for home dashboard
 */
export function EmptyDashboard({
  canLoadSampleData,
  onLoadSampleData,
  loadingSampleData,
}: {
  canLoadSampleData?: boolean
  onLoadSampleData?: () => void
  loadingSampleData?: boolean
}) {
  return (
    <div className="flex items-center justify-center min-h-[400px] px-4">
      <div className="text-center max-w-md">
        <CheckCircle className="w-12 h-12 mx-auto mb-4 text-text-tertiary" />
        <h3 className="text-lg font-semibold text-text-primary mb-2">No active incidents</h3>
        <p className="text-sm text-text-secondary mb-6">
          When incidents are declared, they will appear here.
        </p>
        {canLoadSampleData && onLoadSampleData && (
          <div className="mt-2">
            <p className="text-xs text-text-tertiary mb-3">
              New here? Load sample data to see how incidents, on-call schedules, and escalation policies work together.
            </p>
            <Button variant="secondary" onClick={onLoadSampleData} disabled={loadingSampleData}>
              <Sparkles className="w-4 h-4 mr-2" />
              {loadingSampleData ? 'Loading…' : 'Load sample data'}
            </Button>
          </div>
        )}
      </div>
    </div>
  )
}

/**
 * Pre-configured empty state for timeline
 */
export function EmptyTimeline() {
  return (
    <EmptyState
      icon="clock"
      title="No timeline entries yet"
      description="Activity will appear here as the incident progresses."
    />
  )
}
