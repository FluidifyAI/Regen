import { useState } from 'react'
import { ChevronDown, ChevronUp, Hash, Calendar, Clock, ExternalLink } from 'lucide-react'
import { Badge } from '../ui/Badge'
import { Avatar } from '../ui/Avatar'

type StatusType = 'triggered' | 'acknowledged' | 'resolved' | 'canceled'
type SeverityType = 'critical' | 'high' | 'medium' | 'low'

interface Incident {
  id: string
  incident_number: number
  title: string
  status: StatusType
  severity: SeverityType
  summary: string
  slack_channel_id?: string
  slack_channel_name?: string
  created_at: string
  triggered_at: string
  acknowledged_at?: string
  resolved_at?: string
  commander_id?: string
}

interface PropertiesPanelProps {
  incident: Incident
}

/**
 * Collapsible properties panel for incident details
 * Shows metadata, status, severity, timestamps, and related info
 */
export function PropertiesPanel({ incident }: PropertiesPanelProps) {
  const [collapsed, setCollapsed] = useState(false)

  return (
    <div className="bg-white border-l border-border h-full overflow-y-auto">
      {/* Header */}
      <div className="sticky top-0 bg-white border-b border-border px-4 py-3 z-10">
        <button
          onClick={() => setCollapsed(!collapsed)}
          className="flex items-center justify-between w-full text-left hover:opacity-70 transition-opacity"
        >
          <h2 className="text-sm font-semibold text-text-primary">Properties</h2>
          {collapsed ? (
            <ChevronDown className="w-4 h-4 text-text-tertiary" />
          ) : (
            <ChevronUp className="w-4 h-4 text-text-tertiary" />
          )}
        </button>
      </div>

      {/* Content */}
      {!collapsed && (
        <div className="p-4 space-y-6">
          {/* Status */}
          <PropertySection title="Status">
            <Badge variant={incident.status} type="status">
              {incident.status}
            </Badge>
          </PropertySection>

          {/* Severity */}
          <PropertySection title="Severity">
            <Badge variant={incident.severity} type="severity">
              {incident.severity}
            </Badge>
          </PropertySection>

          {/* Commander */}
          <PropertySection title="Incident Commander">
            {incident.commander_id ? (
              <div className="flex items-center gap-2">
                <Avatar name="Commander" size="sm" />
                <span className="text-sm text-text-primary">Commander</span>
              </div>
            ) : (
              <span className="text-sm text-text-tertiary">Unassigned</span>
            )}
          </PropertySection>

          {/* Timeline */}
          <PropertySection title="Timeline">
            <div className="space-y-2">
              <TimelineItem
                icon={<Calendar className="w-4 h-4" />}
                label="Created"
                value={formatDateTime(incident.created_at)}
              />
              <TimelineItem
                icon={<Clock className="w-4 h-4" />}
                label="Triggered"
                value={formatDateTime(incident.triggered_at)}
              />
              {incident.acknowledged_at && (
                <TimelineItem
                  icon={<Clock className="w-4 h-4" />}
                  label="Acknowledged"
                  value={formatDateTime(incident.acknowledged_at)}
                />
              )}
              {incident.resolved_at && (
                <TimelineItem
                  icon={<Clock className="w-4 h-4" />}
                  label="Resolved"
                  value={formatDateTime(incident.resolved_at)}
                />
              )}
            </div>
          </PropertySection>

          {/* Slack Channel */}
          {incident.slack_channel_name && (
            <PropertySection title="Slack Channel">
              <div className="flex items-center gap-2">
                <Hash className="w-4 h-4 text-text-tertiary" />
                <a
                  href={`https://slack.com/app_redirect?channel=${incident.slack_channel_id}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-sm text-brand-primary hover:underline inline-flex items-center gap-1"
                >
                  #{incident.slack_channel_name}
                  <ExternalLink className="w-3 h-3" />
                </a>
              </div>
            </PropertySection>
          )}

          {/* Incident Number */}
          <PropertySection title="Incident Number">
            <span className="text-sm font-mono text-text-primary">
              INC-{incident.incident_number}
            </span>
          </PropertySection>

          {/* Incident ID */}
          <PropertySection title="Incident ID">
            <span className="text-xs font-mono text-text-tertiary break-all">
              {incident.id}
            </span>
          </PropertySection>
        </div>
      )}
    </div>
  )
}

/**
 * Property section with title
 */
function PropertySection({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <h3 className="text-xs font-medium text-text-tertiary uppercase tracking-wider mb-2">
        {title}
      </h3>
      <div>{children}</div>
    </div>
  )
}

/**
 * Timeline item with icon, label, and value
 */
function TimelineItem({
  icon,
  label,
  value,
}: {
  icon: React.ReactNode
  label: string
  value: string
}) {
  return (
    <div className="flex items-start gap-2">
      <div className="text-text-tertiary mt-0.5">{icon}</div>
      <div className="flex-1 min-w-0">
        <div className="text-xs text-text-tertiary">{label}</div>
        <div className="text-sm text-text-primary">{value}</div>
      </div>
    </div>
  )
}

/**
 * Format timestamp as date and time
 */
function formatDateTime(timestamp: string): string {
  const date = new Date(timestamp)
  return date.toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
    hour12: true,
  })
}
