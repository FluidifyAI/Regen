import { useState, useEffect } from 'react'
import { ChevronDown, ChevronUp, Hash, Clock, ExternalLink, Timer, Activity, AlertCircle } from 'lucide-react'
import { Badge } from '../ui/Badge'
import { Avatar } from '../ui/Avatar'
import { HandoffDigest } from '../incidents/HandoffDigest'
import type { Alert, TimelineEntry } from '../../api/types'
import { updateIncident } from '../../api/incidents'
import { useAuth } from '../../contexts/AuthContext'
import { listUsers, type UserSummary } from '../../api/users'

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
  teams_channel_id?: string
  teams_channel_name?: string
  created_at: string
  triggered_at: string
  acknowledged_at?: string
  resolved_at?: string
  commander_id?: string
  commander_name?: string
  // AI Agents (v0.9+)
  ai_enabled: boolean
  // Detail fields
  alerts: Alert[]
  timeline: TimelineEntry[]
}

interface PropertiesPanelProps {
  incident: Incident
  onIncidentUpdated?: () => void
  lastActivityAt?: string
}

/**
 * Collapsible properties panel for incident details.
 * Shows metadata, duration, last activity, linked alerts, and channel links.
 */
export function PropertiesPanel({ incident, onIncidentUpdated, lastActivityAt }: PropertiesPanelProps) {
  const [collapsed, setCollapsed] = useState(false)

  const { user: currentUser } = useAuth()
  const [users, setUsers] = useState<UserSummary[]>([])
  const [assigningCommander, setAssigningCommander] = useState(false)

  const lastActivityTs = getLastActivity(incident.timeline, incident.triggered_at)

  useEffect(() => {
    if (!collapsed) {
      listUsers().then(setUsers).catch(() => {})
    }
  }, [collapsed])

  async function handleClaimCommander() {
    if (!currentUser?.id) return
    setAssigningCommander(true)
    try {
      await updateIncident(incident.id, { commander_id: currentUser.id })
      onIncidentUpdated?.()
    } finally {
      setAssigningCommander(false)
    }
  }

  const CLEAR_COMMANDER = '00000000-0000-0000-0000-000000000000'

  async function handleAssignCommander(userId: string | null) {
    setAssigningCommander(true)
    try {
      await updateIncident(incident.id, { commander_id: userId ?? CLEAR_COMMANDER })
      onIncidentUpdated?.()
    } finally {
      setAssigningCommander(false)
    }
  }

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
        <div className="p-4 space-y-5">

          {/* ── Identity ─────────────────────────────── */}
          <div className="flex items-center justify-between">
            <span className="text-lg font-bold text-text-primary font-mono">
              INC-{incident.incident_number}
            </span>
            <Badge variant={incident.severity} type="severity">
              {incident.severity}
            </Badge>
          </div>

          {/* Status */}
          <PropertySection title="Status">
            <Badge variant={incident.status} type="status">
              {incident.status}
            </Badge>
          </PropertySection>

          {/* ── People ───────────────────────────────── */}
          <PropertySection title="Incident Commander">
            <div className="space-y-2">
              {incident.commander_id ? (
                <div className="flex items-center gap-2">
                  <Avatar name={incident.commander_name || 'Commander'} size="sm" />
                  <span className="text-sm text-text-primary flex-1 truncate">
                    {incident.commander_name || 'Unknown'}
                  </span>
                </div>
              ) : (
                <div className="flex items-center gap-2">
                  <span className="text-sm text-text-tertiary flex-1">Unassigned</span>
                  {currentUser?.id && (
                    <button
                      onClick={handleClaimCommander}
                      disabled={assigningCommander}
                      className="text-xs text-brand-primary hover:underline disabled:opacity-50 font-medium"
                    >
                      Claim
                    </button>
                  )}
                </div>
              )}

              {/* Assign / Reassign dropdown */}
              {users.length > 0 && (
                <select
                  value={incident.commander_id || ''}
                  onChange={(e) => handleAssignCommander(e.target.value || null)}
                  disabled={assigningCommander}
                  className="w-full text-xs border border-border rounded px-2 py-1 text-text-secondary bg-white disabled:opacity-50"
                >
                  <option value="">
                    {incident.commander_id ? 'Reassign commander…' : 'Assign commander…'}
                  </option>
                  {users.map((u) => (
                    <option key={u.id} value={u.id}>
                      {u.name || u.email}
                    </option>
                  ))}
                </select>
              )}
            </div>
          </PropertySection>

          {/* ── Time ─────────────────────────────────── */}
          <PropertySection title="Duration">
            <div className="flex items-center gap-2">
              <Timer className="w-4 h-4 text-text-tertiary" />
              <span className="text-sm text-text-primary">
                {formatDuration(incident.triggered_at, incident.resolved_at)}
              </span>
              {!incident.resolved_at && (
                <span className="text-xs text-amber-500 font-medium">ongoing</span>
              )}
            </div>
          </PropertySection>

          <PropertySection title="Timeline">
            <div className="space-y-2">
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
              <TimelineItem
                icon={<Activity className="w-4 h-4" />}
                label="Last activity"
                value={formatRelativeTime(lastActivityTs)}
              />
            </div>
          </PropertySection>

          {/* ── Channels ─────────────────────────────── */}
          {(incident.slack_channel_name || incident.teams_channel_name) && (
            <PropertySection title="Channels">
              <div className="space-y-2">
                {incident.slack_channel_name && (
                  <div className="flex items-center gap-2">
                    <Hash className="w-4 h-4 text-text-tertiary flex-shrink-0" />
                    <a
                      href={`https://slack.com/app_redirect?channel=${incident.slack_channel_id}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-sm text-brand-primary hover:underline inline-flex items-center gap-1 truncate"
                    >
                      {incident.slack_channel_name}
                      <ExternalLink className="w-3 h-3 flex-shrink-0" />
                    </a>
                  </div>
                )}
                {incident.teams_channel_name && (
                  <div className="flex items-center gap-2">
                    <Hash className="w-4 h-4 text-text-tertiary flex-shrink-0" />
                    <a
                      href={`https://teams.microsoft.com/l/channel/${encodeURIComponent(incident.teams_channel_id ?? '')}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-sm text-brand-primary hover:underline inline-flex items-center gap-1 truncate"
                    >
                      {incident.teams_channel_name}
                      <ExternalLink className="w-3 h-3 flex-shrink-0" />
                    </a>
                  </div>
                )}
              </div>
            </PropertySection>
          )}

          {/* ── Alerts ───────────────────────────────── */}
          {incident.alerts.length > 0 && (
            <PropertySection title="Linked Alerts">
              <div className="flex items-center gap-2">
                <AlertCircle className="w-4 h-4 text-text-tertiary" />
                <span className="text-sm text-text-primary">
                  {incident.alerts.length} alert{incident.alerts.length !== 1 ? 's' : ''}
                </span>
              </div>
            </PropertySection>
          )}

          {/* ── AI ───────────────────────────────────── */}
          <div className="flex items-center justify-between pt-2 border-t border-border">
            <div>
              <span className="text-xs font-medium text-text-tertiary uppercase tracking-wide">AI Agents</span>
              <p className="text-xs text-text-tertiary mt-0.5">Coming soon</p>
            </div>
            <span className="text-xs px-2 py-0.5 rounded-full bg-gray-100 text-gray-400 font-medium flex-shrink-0">
              Soon
            </span>
          </div>

          {/* Handoff Digest */}
          <HandoffDigest incidentId={incident.id} lastActivityAt={lastActivityAt} />

          {/* ── Debug ────────────────────────────────── */}
          <div className="pt-2 border-t border-border">
            <span className="text-xs font-mono text-text-tertiary break-all select-all">
              {incident.id}
            </span>
          </div>

        </div>
      )}
    </div>
  )
}

// ── Subcomponents ─────────────────────────────────────────────────────────────

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

// ── Helpers ───────────────────────────────────────────────────────────────────

/** Returns the timestamp of the most recent timeline entry, or falls back to triggered_at. */
function getLastActivity(timeline: TimelineEntry[], fallback: string): string {
  if (timeline.length === 0) return fallback
  const latest = timeline.reduce((a, b) =>
    new Date(a.timestamp) > new Date(b.timestamp) ? a : b
  )
  return latest.timestamp
}

/** Format a timestamp as absolute date + time, e.g. "Feb 5, 2026, 2:45 PM" */
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

/** Format duration between two timestamps. If endTs is omitted, use now. */
function formatDuration(startTs: string, endTs?: string): string {
  const start = new Date(startTs)
  const end = endTs ? new Date(endTs) : new Date()
  const diffSeconds = Math.max(0, Math.floor((end.getTime() - start.getTime()) / 1000))

  if (diffSeconds < 60) return `${diffSeconds}s`
  if (diffSeconds < 3600) return `${Math.floor(diffSeconds / 60)}m`
  const hours = Math.floor(diffSeconds / 3600)
  const mins = Math.floor((diffSeconds % 3600) / 60)
  return mins > 0 ? `${hours}h ${mins}m` : `${hours}h`
}

/** Format a timestamp as relative time, e.g. "3m ago" */
function formatRelativeTime(timestamp: string): string {
  const diffSeconds = Math.max(0, Math.floor((Date.now() - new Date(timestamp).getTime()) / 1000))

  if (diffSeconds < 60) return 'Just now'
  if (diffSeconds < 3600) return `${Math.floor(diffSeconds / 60)}m ago`
  if (diffSeconds < 86400) return `${Math.floor(diffSeconds / 3600)}h ago`
  return `${Math.floor(diffSeconds / 86400)}d ago`
}
