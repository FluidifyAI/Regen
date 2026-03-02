import { useState } from 'react'
import { ChevronDown, ChevronUp, Hash, Clock, ExternalLink, Timer, Activity, AlertCircle } from 'lucide-react'
import { Badge } from '../ui/Badge'
import { Avatar } from '../ui/Avatar'
import { HandoffDigest } from '../incidents/HandoffDigest'
import type { Alert, TimelineEntry } from '../../api/types'
import { updateIncident } from '../../api/incidents'

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
  // Optimistic toggle — flips immediately, reverts on error
  const [aiEnabled, setAiEnabled] = useState(incident.ai_enabled)

  // Sync if parent refetches with a new value
  if (aiEnabled !== incident.ai_enabled && !collapsed) {
    setAiEnabled(incident.ai_enabled)
  }

  const lastActivityTs = getLastActivity(incident.timeline, incident.triggered_at)

  async function handleAIToggle() {
    const next = !aiEnabled
    setAiEnabled(next) // optimistic
    try {
      await updateIncident(incident.id, { ai_enabled: next })
      onIncidentUpdated?.()
    } catch {
      setAiEnabled(!next) // revert
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
            {incident.commander_id ? (
              <div className="flex items-center gap-2">
                <Avatar name="Commander" size="sm" />
                <span className="text-sm text-text-primary">Commander</span>
              </div>
            ) : (
              <span className="text-sm text-text-tertiary">Unassigned</span>
            )}
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
              <p className="text-xs text-text-tertiary mt-0.5">
                {aiEnabled ? 'Auto post-mortem on resolve' : 'Disabled for this incident'}
              </p>
            </div>
            <button
              type="button"
              role="switch"
              aria-checked={aiEnabled}
              onClick={handleAIToggle}
              className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors flex-shrink-0 ${
                aiEnabled ? 'bg-brand-primary' : 'bg-gray-200'
              }`}
              title={aiEnabled ? 'AI agents enabled — click to disable' : 'AI agents disabled — click to enable'}
            >
              <span className={`inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform ${
                aiEnabled ? 'translate-x-4' : 'translate-x-0.5'
              }`} />
            </button>
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
