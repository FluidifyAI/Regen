import { MessageSquare, AlertCircle, CheckCircle, XCircle, UserPlus, Bell } from 'lucide-react'
import { Avatar } from '../ui/Avatar'
import type { TimelineEntry } from '../../api/types'

interface TimelineProps {
  entries: TimelineEntry[]
}

/**
 * Timeline component with date grouping
 * Groups entries by date and displays chronologically (newest first)
 */
export function Timeline({ entries }: TimelineProps) {
  // Group entries by date
  const groupedEntries = groupEntriesByDate(entries)

  if (entries.length === 0) {
    return (
      <div className="text-center py-12">
        <p className="text-sm text-text-tertiary">No activity yet</p>
      </div>
    )
  }

  return (
    <div className="space-y-8">
      {Object.entries(groupedEntries).map(([dateLabel, dateEntries]) => (
        <div key={dateLabel}>
          {/* Date Header */}
          <div className="sticky top-0 bg-white z-10 py-2 mb-4">
            <h3 className="text-xs font-semibold text-text-tertiary uppercase tracking-wider">
              {dateLabel}
            </h3>
          </div>

          {/* Timeline Entries */}
          <div className="space-y-4 relative">
            {/* Vertical line */}
            <div className="absolute left-4 top-0 bottom-0 w-0.5 bg-border" />

            {dateEntries.map((entry) => (
              <TimelineEntryItem key={entry.id} entry={entry} />
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}

/**
 * Individual timeline entry
 */
function TimelineEntryItem({ entry }: { entry: TimelineEntry }) {
  const { icon: Icon, color, label } = getEntryMetadata(entry)
  const formattedTime = formatTime(entry.timestamp)

  return (
    <div className="relative flex gap-3 pl-11">
      {/* Icon */}
      <div
        className={`absolute left-0 w-8 h-8 rounded-full bg-white border-2 ${color} flex items-center justify-center`}
      >
        <Icon className={`w-4 h-4 ${color.replace('border-', 'text-')}`} />
      </div>

      {/* Content */}
      <div className="flex-1 bg-white border border-border rounded-lg p-4">
        {/* Header */}
        <div className="flex items-center justify-between mb-2">
          <div className="flex items-center gap-2">
            <Avatar name={getActorName(entry)} size="sm" />
            <span className="text-sm font-medium text-text-primary">
              {getActorName(entry)}
            </span>
            <span className="text-sm text-text-secondary">{label}</span>
          </div>
          <span className="text-xs text-text-tertiary">{formattedTime}</span>
        </div>

        {/* Entry Content */}
        <EntryContent entry={entry} />
      </div>
    </div>
  )
}

/**
 * Renders entry content based on type
 */
function EntryContent({ entry }: { entry: TimelineEntry }) {
  const content = entry.content as Record<string, string>

  switch (entry.type) {
    case 'status_changed':
      return (
        <div className="text-sm text-text-secondary">
          Status changed from{' '}
          <span className="font-medium">{content.old_status || 'unknown'}</span> to{' '}
          <span className="font-medium">{content.new_status || 'unknown'}</span>
        </div>
      )

    case 'severity_changed':
      return (
        <div className="text-sm text-text-secondary">
          Severity changed from{' '}
          <span className="font-medium">{content.old_severity || 'unknown'}</span> to{' '}
          <span className="font-medium">{content.new_severity || 'unknown'}</span>
        </div>
      )

    case 'message':
      return (
        <div className="text-sm text-text-primary whitespace-pre-wrap">
          {content.message || content.text || 'No message'}
        </div>
      )

    case 'alert_linked':
      return (
        <div className="text-sm text-text-secondary">
          Alert linked:{' '}
          <span className="font-medium font-mono text-xs">
            {content.alert_id || content.external_id || 'unknown'}
          </span>
        </div>
      )

    case 'commander_assigned':
      return (
        <div className="text-sm text-text-secondary">
          Assigned{' '}
          <span className="font-medium">{content.commander_name || 'commander'}</span> as
          incident commander
        </div>
      )

    default:
      return (
        <div className="text-sm text-text-secondary">
          {JSON.stringify(entry.content)}
        </div>
      )
  }
}

/**
 * Get icon, color, and label for entry type
 */
function getEntryMetadata(entry: TimelineEntry): {
  icon: typeof MessageSquare
  color: string
  label: string
} {
  const content = entry.content as Record<string, string>

  switch (entry.type) {
    case 'status_changed':
      if (content.new_status === 'resolved') {
        return {
          icon: CheckCircle,
          color: 'border-status-resolved',
          label: 'marked as resolved',
        }
      }
      if (content.new_status === 'acknowledged') {
        return {
          icon: AlertCircle,
          color: 'border-status-acknowledged',
          label: 'acknowledged incident',
        }
      }
      if (content.new_status === 'canceled') {
        return {
          icon: XCircle,
          color: 'border-text-tertiary',
          label: 'canceled incident',
        }
      }
      return {
        icon: AlertCircle,
        color: 'border-status-triggered',
        label: 'changed status',
      }

    case 'severity_changed':
      return {
        icon: Bell,
        color: 'border-brand-primary',
        label: 'changed severity',
      }

    case 'message':
      return {
        icon: MessageSquare,
        color: 'border-border',
        label: 'posted',
      }

    case 'alert_linked':
      return {
        icon: Bell,
        color: 'border-severity-critical',
        label: 'linked alert',
      }

    case 'commander_assigned':
      return {
        icon: UserPlus,
        color: 'border-brand-primary',
        label: 'assigned commander',
      }

    default:
      return {
        icon: MessageSquare,
        color: 'border-border',
        label: 'updated',
      }
  }
}

/**
 * Get actor display name
 */
function getActorName(entry: TimelineEntry): string {
  if (entry.actor_type === 'system') {
    return 'System'
  }
  if (entry.actor_type === 'slack_bot') {
    return 'Slack Bot'
  }
  return entry.actor_id || 'Unknown User'
}

/**
 * Group timeline entries by date
 */
function groupEntriesByDate(entries: TimelineEntry[]): Record<string, TimelineEntry[]> {
  const grouped: Record<string, TimelineEntry[]> = {}

  // Sort entries newest first
  const sorted = [...entries].sort(
    (a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime()
  )

  sorted.forEach((entry) => {
    const dateLabel = getDateLabel(entry.timestamp)
    if (!grouped[dateLabel]) {
      grouped[dateLabel] = []
    }
    grouped[dateLabel].push(entry)
  })

  return grouped
}

/**
 * Get human-readable date label (Today, Yesterday, or date)
 */
function getDateLabel(timestamp: string): string {
  const date = new Date(timestamp)
  const today = new Date()
  const yesterday = new Date(today)
  yesterday.setDate(yesterday.getDate() - 1)

  // Reset time parts for date comparison
  const dateOnly = new Date(date.getFullYear(), date.getMonth(), date.getDate())
  const todayOnly = new Date(today.getFullYear(), today.getMonth(), today.getDate())
  const yesterdayOnly = new Date(yesterday.getFullYear(), yesterday.getMonth(), yesterday.getDate())

  if (dateOnly.getTime() === todayOnly.getTime()) {
    return 'Today'
  }
  if (dateOnly.getTime() === yesterdayOnly.getTime()) {
    return 'Yesterday'
  }

  // Format as "Feb 5, 2026"
  return date.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

/**
 * Format timestamp as time (e.g., "2:45 PM")
 */
function formatTime(timestamp: string): string {
  const date = new Date(timestamp)
  return date.toLocaleTimeString('en-US', {
    hour: 'numeric',
    minute: '2-digit',
    hour12: true,
  })
}
