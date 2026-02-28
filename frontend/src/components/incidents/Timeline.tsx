import {
  MessageSquare,
  AlertCircle,
  CheckCircle,
  XCircle,
  UserPlus,
  Bell,
  Hash,
  Shield,
  FileText,
  Archive,
  Zap,
  Sparkles,
  AlertTriangle,
} from 'lucide-react'
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
 * Renders entry content based on type. No raw JSON is ever shown.
 */
function EntryContent({ entry }: { entry: TimelineEntry }) {
  const content = entry.content as Record<string, string>

  switch (entry.type) {
    case 'incident_created':
      return (
        <div className="text-sm text-text-secondary">
          Incident was created
          {content.created_by && (
            <> by <span className="font-medium">{content.created_by}</span></>
          )}
        </div>
      )

    case 'status_changed':
      return (
        <div className="text-sm text-text-secondary">
          Status changed from{' '}
          <span className="font-medium">{content.previous_status || content.old_status || 'unknown'}</span> to{' '}
          <span className="font-medium">{content.new_status || 'unknown'}</span>
        </div>
      )

    case 'severity_changed':
      return (
        <div className="text-sm text-text-secondary">
          Severity changed from{' '}
          <span className="font-medium">{content.previous_severity || content.old_severity || 'unknown'}</span> to{' '}
          <span className="font-medium">{content.new_severity || 'unknown'}</span>
        </div>
      )

    case 'message':
      return (
        <div className="text-sm text-text-primary whitespace-pre-wrap">
          {content.message || content.text || 'No message'}
        </div>
      )

    case 'slack_message':
      return (
        <div className="text-sm text-text-primary whitespace-pre-wrap">
          {content.text || content.message || 'No message'}
        </div>
      )

    case 'teams_message':
      return (
        <div className="text-sm text-text-primary whitespace-pre-wrap">
          {content.text || content.message || 'No message'}
        </div>
      )

    case 'slack_channel_created':
      return (
        <div className="text-sm text-text-secondary flex items-center gap-2 flex-wrap">
          <span>Incident channel created in Slack</span>
          {content.channel_name && (
            <a
              href={`https://slack.com/app_redirect?channel=${content.channel_id}`}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1 text-brand-primary hover:underline font-medium"
            >
              <Hash className="w-3 h-3" />
              {content.channel_name}
            </a>
          )}
        </div>
      )

    case 'teams_channel_created':
      return (
        <div className="text-sm text-text-secondary flex items-center gap-2 flex-wrap">
          <span>Incident channel created in Microsoft Teams</span>
          {content.channel_name && (
            <span className="inline-flex items-center gap-1 font-medium text-text-primary">
              <Hash className="w-3 h-3" />
              {content.channel_name}
            </span>
          )}
        </div>
      )

    case 'slack_channel_archived':
      return (
        <div className="text-sm text-text-secondary">
          Slack channel archived
        </div>
      )

    case 'slack_channel_creation_failed':
    case 'teams_channel_creation_failed':
      return (
        <div className="text-sm text-red-600">
          Failed to create channel
          {content.error && (
            <span className="text-text-tertiary font-mono text-xs ml-1">— {content.error}</span>
          )}
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

    case 'responder_added':
      return (
        <div className="text-sm text-text-secondary">
          <span className="font-medium">{content.user_name || content.responder || 'Responder'}</span>{' '}
          added as responder
        </div>
      )

    case 'escalated':
      return (
        <div className="text-sm text-text-secondary">
          Escalated to{' '}
          <span className="font-medium">{content.policy_name || content.schedule_name || 'escalation policy'}</span>
          {content.tier && <span className="text-text-tertiary"> (tier {content.tier})</span>}
        </div>
      )

    case 'summary_generated':
      return (
        <div className="text-sm text-text-secondary">
          AI summary generated
          {content.model && <span className="text-text-tertiary"> via {content.model}</span>}
        </div>
      )

    case 'postmortem_created':
      return (
        <div className="text-sm text-text-secondary">
          Post-mortem document created
        </div>
      )

    default: {
      // For any unknown type, try to extract a readable message rather than
      // dumping raw JSON. Look for common text keys first, then fall back gracefully.
      const message = content.message || content.text || content.description
      if (message) {
        return <div className="text-sm text-text-primary whitespace-pre-wrap">{message}</div>
      }
      const errorMsg = content.error
      if (errorMsg) {
        return <div className="text-sm text-red-600">{errorMsg}</div>
      }
      // Last resort: show the entry type name formatted nicely, no raw JSON
      return (
        <div className="text-sm text-text-tertiary italic">
          {entry.type.replace(/_/g, ' ')}
        </div>
      )
    }
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
    case 'incident_created':
      return { icon: Shield, color: 'border-brand-primary', label: 'created incident' }

    case 'status_changed':
      if (content.new_status === 'resolved') {
        return { icon: CheckCircle, color: 'border-status-resolved', label: 'marked as resolved' }
      }
      if (content.new_status === 'acknowledged') {
        return { icon: AlertCircle, color: 'border-status-acknowledged', label: 'acknowledged incident' }
      }
      if (content.new_status === 'canceled') {
        return { icon: XCircle, color: 'border-text-tertiary', label: 'canceled incident' }
      }
      return { icon: AlertCircle, color: 'border-status-triggered', label: 'changed status' }

    case 'severity_changed':
      return { icon: Bell, color: 'border-brand-primary', label: 'changed severity' }

    case 'message':
      return { icon: MessageSquare, color: 'border-border', label: 'posted' }

    case 'slack_message':
      return { icon: Hash, color: 'border-brand-primary', label: 'posted in Slack' }

    case 'teams_message':
      return { icon: Hash, color: 'border-brand-primary', label: 'posted in Teams' }

    case 'slack_channel_created':
    case 'teams_channel_created':
      return { icon: Hash, color: 'border-brand-primary', label: 'created channel' }

    case 'slack_channel_archived':
      return { icon: Archive, color: 'border-text-tertiary', label: 'archived channel' }

    case 'slack_channel_creation_failed':
    case 'teams_channel_creation_failed':
      return { icon: AlertTriangle, color: 'border-red-400', label: 'channel creation failed' }

    case 'alert_linked':
      return { icon: Bell, color: 'border-severity-critical', label: 'linked alert' }

    case 'commander_assigned':
      return { icon: UserPlus, color: 'border-brand-primary', label: 'assigned commander' }

    case 'responder_added':
      return { icon: UserPlus, color: 'border-brand-primary', label: 'added responder' }

    case 'escalated':
      return { icon: Zap, color: 'border-amber-400', label: 'escalated' }

    case 'summary_generated':
      return { icon: Sparkles, color: 'border-brand-primary', label: 'generated AI summary' }

    case 'postmortem_created':
      return { icon: FileText, color: 'border-brand-primary', label: 'created post-mortem' }

    default:
      return { icon: MessageSquare, color: 'border-border', label: 'updated' }
  }
}

/**
 * Get actor display name
 */
function getActorName(entry: TimelineEntry): string {
  if (entry.actor_type === 'system') return 'System'
  if (entry.actor_type === 'slack_bot') return 'Slack Bot'
  if (entry.actor_type === 'teams_bot') return 'Teams Bot'
  if (entry.actor_type === 'slack_user') {
    const content = entry.content as Record<string, string>
    return content.author_name || entry.actor_id || 'Slack User'
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

  const dateOnly = new Date(date.getFullYear(), date.getMonth(), date.getDate())
  const todayOnly = new Date(today.getFullYear(), today.getMonth(), today.getDate())
  const yesterdayOnly = new Date(yesterday.getFullYear(), yesterday.getMonth(), yesterday.getDate())

  if (dateOnly.getTime() === todayOnly.getTime()) return 'Today'
  if (dateOnly.getTime() === yesterdayOnly.getTime()) return 'Yesterday'

  return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })
}

/**
 * Format timestamp as time (e.g., "2:45 PM")
 */
function formatTime(timestamp: string): string {
  const date = new Date(timestamp)
  return date.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit', hour12: true })
}
