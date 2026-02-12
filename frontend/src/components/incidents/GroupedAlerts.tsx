import { AlertCircle, Link2, Layers } from 'lucide-react'
import { Badge } from '../ui/Badge'
import type { Alert, Incident } from '../../api/types'

interface GroupedAlertsProps {
  alerts: Alert[]
  incident: Incident
}

/**
 * Display alerts with grouping visualization
 * Shows visual indicators when alerts are grouped together via grouping rules
 */
export function GroupedAlerts({ alerts, incident }: GroupedAlertsProps) {
  if (alerts.length === 0) {
    return (
      <div className="text-center py-12">
        <AlertCircle className="w-12 h-12 text-text-tertiary mx-auto mb-3 opacity-50" />
        <p className="text-sm text-text-tertiary">No alerts linked to this incident</p>
      </div>
    )
  }

  const isGrouped = !!incident.group_key
  const sources = [...new Set(alerts.map((a) => a.source))]
  const isCrossSource = sources.length > 1

  return (
    <div className="space-y-6">
      {/* Grouping Header - Only show if incident has group_key */}
      {isGrouped && (
        <div className="bg-brand-light border border-brand-primary/20 rounded-lg p-4">
          <div className="flex items-start gap-3">
            <Layers className="w-5 h-5 text-brand-primary flex-shrink-0 mt-0.5" />
            <div className="flex-1">
              <h3 className="text-sm font-medium text-text-primary mb-1">
                Grouped Incident
              </h3>
              <p className="text-sm text-text-secondary mb-3">
                {alerts.length} alert{alerts.length !== 1 ? 's' : ''} from{' '}
                {isCrossSource ? (
                  <span className="font-medium text-brand-primary">
                    {sources.length} different sources
                  </span>
                ) : (
                  <span className="font-medium">{sources[0]}</span>
                )}{' '}
                grouped together using alert grouping rules.
              </p>

              {/* Source badges for cross-source correlation */}
              {isCrossSource && (
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="text-xs text-text-tertiary">Sources:</span>
                  {sources.map((source) => (
                    <span
                      key={source}
                      className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-surface-secondary text-text-primary border border-border"
                    >
                      {source}
                    </span>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Alerts List */}
      <div className="space-y-4">
        {alerts.map((alert, index) => (
          <div key={alert.id} className="relative">
            {/* Connector line for grouped alerts */}
            {isGrouped && index > 0 && (
              <div className="absolute left-2 -top-4 w-0.5 h-4 bg-brand-primary/30" />
            )}

            <div
              className={`border rounded-lg p-4 transition-colors ${
                isGrouped
                  ? 'border-brand-primary/30 bg-brand-light/30 hover:bg-brand-light/50'
                  : 'border-border bg-white hover:bg-surface-secondary'
              }`}
            >
              <div className="flex items-start gap-3">
                {/* Grouping indicator icon */}
                {isGrouped && (
                  <div className="flex-shrink-0 mt-0.5">
                    <Link2 className="w-4 h-4 text-brand-primary" />
                  </div>
                )}

                <div className="flex-1 min-w-0">
                  {/* Alert header */}
                  <div className="flex items-start justify-between gap-3 mb-2">
                    <div className="flex items-center gap-2 min-w-0 flex-1">
                      <AlertCircle className={`w-4 h-4 flex-shrink-0 ${getSeverityIconColor(alert.severity)}`} />
                      <span className="text-sm font-medium text-text-primary truncate">
                        {alert.title}
                      </span>
                    </div>
                    <div className="flex items-center gap-2 flex-shrink-0">
                      <Badge
                        variant={alert.severity as 'critical' | 'high' | 'medium' | 'low'}
                        type="severity"
                      >
                        {alert.severity}
                      </Badge>
                      {alert.status === 'resolved' && (
                        <Badge variant="resolved" type="status">Resolved</Badge>
                      )}
                    </div>
                  </div>

                  {/* Alert description */}
                  {alert.description && (
                    <p className="text-sm text-text-secondary mb-3 line-clamp-2">
                      {alert.description}
                    </p>
                  )}

                  {/* Alert metadata */}
                  <div className="flex items-center gap-4 text-xs text-text-tertiary flex-wrap">
                    <span className="flex items-center gap-1.5">
                      <span className="font-medium">Source:</span>
                      <span className={isCrossSource ? 'text-brand-primary font-medium' : ''}>
                        {alert.source}
                      </span>
                    </span>
                    <span>•</span>
                    <span>{formatDateTime(alert.started_at)}</span>
                    {alert.ended_at && (
                      <>
                        <span>•</span>
                        <span className="text-status-resolved font-medium">
                          Resolved {formatDateTime(alert.ended_at)}
                        </span>
                      </>
                    )}
                  </div>

                  {/* Show alert labels if present */}
                  {Object.keys(alert.labels).length > 0 && (
                    <div className="mt-3 pt-3 border-t border-border/50">
                      <details className="group">
                        <summary className="text-xs text-text-tertiary cursor-pointer hover:text-text-primary select-none">
                          <span className="group-open:hidden">Show labels ({Object.keys(alert.labels).length})</span>
                          <span className="hidden group-open:inline">Hide labels</span>
                        </summary>
                        <div className="mt-2 grid grid-cols-2 gap-2">
                          {Object.entries(alert.labels).slice(0, 10).map(([key, value]) => (
                            <div key={key} className="text-xs">
                              <span className="text-text-tertiary font-mono">{key}:</span>{' '}
                              <span className="text-text-secondary font-mono">{value}</span>
                            </div>
                          ))}
                          {Object.keys(alert.labels).length > 10 && (
                            <div className="text-xs text-text-tertiary col-span-2">
                              + {Object.keys(alert.labels).length - 10} more labels
                            </div>
                          )}
                        </div>
                      </details>
                    </div>
                  )}
                </div>
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* Group Key Display (for debugging/advanced users) */}
      {isGrouped && incident.group_key && (
        <details className="bg-surface-secondary border border-border rounded-lg p-4">
          <summary className="text-xs font-medium text-text-tertiary cursor-pointer hover:text-text-primary select-none">
            Advanced: Group Key
          </summary>
          <div className="mt-2">
            <p className="text-xs text-text-secondary mb-2">
              This incident was grouped using the following key (SHA256 hash of alert labels):
            </p>
            <code className="block text-xs font-mono bg-white border border-border rounded p-2 text-text-primary break-all">
              {incident.group_key}
            </code>
            <p className="text-xs text-text-tertiary mt-2">
              All alerts with the same group key within the configured time window are grouped into this incident.
            </p>
          </div>
        </details>
      )}
    </div>
  )
}

/**
 * Get severity icon color
 */
function getSeverityIconColor(severity: string): string {
  switch (severity) {
    case 'critical':
      return 'text-severity-critical'
    case 'warning':
    case 'high':
      return 'text-severity-warning'
    case 'info':
    case 'medium':
    case 'low':
      return 'text-severity-info'
    default:
      return 'text-text-tertiary'
  }
}

/**
 * Format timestamp as date and time
 */
function formatDateTime(timestamp: string): string {
  const date = new Date(timestamp)
  return date.toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
    hour12: true,
  })
}
