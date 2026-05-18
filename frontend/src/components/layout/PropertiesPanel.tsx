import { useState, useEffect, useRef } from 'react'
import {
  ChevronDown, ChevronUp, Hash, Clock, ExternalLink, Timer,
  Activity, AlertCircle, Type, ChevronRight, User, Search, X,
} from 'lucide-react'
import { Badge } from '../ui/Badge'
import { Avatar } from '../ui/Avatar'
import { HandoffDigest } from '../incidents/HandoffDigest'
import type { Alert, TimelineEntry } from '../../api/types'
import { updateIncident } from '../../api/incidents'
import { useAuth } from '../../contexts/AuthContext'
import { listUsers, type UserSummary } from '../../api/users'
import { listCustomFields, type CustomFieldDefinition } from '../../api/customFields'

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
  ai_enabled: boolean
  custom_fields?: Record<string, string>
  alerts: Alert[]
  timeline: TimelineEntry[]
}

interface PropertiesPanelProps {
  incident: Incident
  onIncidentUpdated?: () => void
  lastActivityAt?: string
}

// ── Section collapse ───────────────────────────────────────────────────────────

type SectionKey = 'status' | 'commander' | 'time' | 'customFields' | 'channels' | 'alerts'

const SECTION_STORAGE_KEY = 'properties-panel-sections-v2'

function useSectionCollapse() {
  const defaults: Record<SectionKey, boolean> = {
    status: true, commander: true, time: true,
    customFields: true, channels: true, alerts: true,
  }
  const [open, setOpen] = useState<Record<SectionKey, boolean>>(() => {
    try {
      const saved = localStorage.getItem(SECTION_STORAGE_KEY)
      return saved ? { ...defaults, ...JSON.parse(saved) } : defaults
    } catch {
      return defaults
    }
  })
  function toggle(key: SectionKey) {
    setOpen(prev => {
      const next = { ...prev, [key]: !prev[key] }
      localStorage.setItem(SECTION_STORAGE_KEY, JSON.stringify(next))
      return next
    })
  }
  return { open, toggle }
}

// ── Component ──────────────────────────────────────────────────────────────────

export function PropertiesPanel({ incident, onIncidentUpdated, lastActivityAt }: PropertiesPanelProps) {
  const [panelCollapsed, setPanelCollapsed] = useState(false)
  const { open, toggle } = useSectionCollapse()

  // Custom fields state
  const [customFieldDefs, setCustomFieldDefs] = useState<CustomFieldDefinition[]>([])
  const [editingCfKey, setEditingCfKey] = useState<string | null>(null)
  const [cfDraft, setCfDraft] = useState('')
  const [flashingCfKey, setFlashingCfKey] = useState<string | null>(null)

  useEffect(() => {
    listCustomFields().then(setCustomFieldDefs).catch(() => {})
  }, [])

  async function saveCfValue(key: string, value: string) {
    const existing = incident.custom_fields ?? {}
    const updated = { ...existing }
    if (value.trim() === '') {
      delete updated[key]
    } else {
      updated[key] = value.trim()
    }
    await updateIncident(incident.id, { custom_fields: updated } as never)
    onIncidentUpdated?.()
    setEditingCfKey(null)
    setFlashingCfKey(key)
    setTimeout(() => setFlashingCfKey(null), 700)
  }

  // Commander state
  const { user: currentUser } = useAuth()
  const [users, setUsers] = useState<UserSummary[]>([])
  const [assigningCommander, setAssigningCommander] = useState(false)
  const [showCommanderSearch, setShowCommanderSearch] = useState(false)
  const [commanderQuery, setCommanderQuery] = useState('')
  const searchInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    listUsers().then(setUsers).catch(() => {})
  }, [])

  useEffect(() => {
    if (!showCommanderSearch) return
    const t = setTimeout(() => searchInputRef.current?.focus(), 40)
    return () => clearTimeout(t)
  }, [showCommanderSearch])

  const filteredUsers = commanderQuery.trim()
    ? users.filter(u => (u.name || u.email).toLowerCase().includes(commanderQuery.toLowerCase()))
    : users

  const CLEAR_COMMANDER = '00000000-0000-0000-0000-000000000000'

  async function handleAssignCommander(userId: string | null) {
    setAssigningCommander(true)
    setShowCommanderSearch(false)
    setCommanderQuery('')
    try {
      await updateIncident(incident.id, { commander_id: userId ?? CLEAR_COMMANDER })
      onIncidentUpdated?.()
    } finally {
      setAssigningCommander(false)
    }
  }

  const lastActivityTs = getLastActivity(incident.timeline, incident.triggered_at)

  return (
    <div className="bg-white border-l border-border h-full flex flex-col overflow-hidden">
      {/* Panel header */}
      <div className="sticky top-0 bg-white border-b border-border px-4 py-3 z-10 flex-shrink-0">
        <button
          onClick={() => setPanelCollapsed(!panelCollapsed)}
          className="flex items-center justify-between w-full text-left hover:opacity-70 transition-opacity"
        >
          <h2 className="text-sm font-semibold text-text-primary">Properties</h2>
          {panelCollapsed
            ? <ChevronDown className="w-4 h-4 text-text-tertiary" />
            : <ChevronUp className="w-4 h-4 text-text-tertiary" />
          }
        </button>
      </div>

      {!panelCollapsed && (
        <div className="flex-1 overflow-y-auto">

          {/* STATUS */}
          <Section title="Status" sectionKey="status" open={open.status} onToggle={() => toggle('status')}>
            <Badge variant={incident.status} type="status">{incident.status}</Badge>
          </Section>

          {/* COMMANDER */}
          <Section title="Commander" sectionKey="commander" open={open.commander} onToggle={() => toggle('commander')}>
            <div>
              {incident.commander_id ? (
                <div className="group flex items-center gap-2">
                  <Avatar name={incident.commander_name || 'Commander'} size="sm" />
                  <span className="text-sm text-text-primary flex-1 truncate">
                    {incident.commander_name || 'Unknown'}
                  </span>
                  <button
                    onClick={() => setShowCommanderSearch(v => !v)}
                    disabled={assigningCommander}
                    className="opacity-0 group-hover:opacity-100 transition-opacity text-xs text-brand-primary hover:underline disabled:opacity-30"
                  >
                    Reassign
                  </button>
                </div>
              ) : (
                <div className="flex items-center gap-2">
                  <div className="w-6 h-6 rounded-full bg-surface-tertiary border border-border flex items-center justify-center flex-shrink-0">
                    <User className="w-3.5 h-3.5 text-text-tertiary" />
                  </div>
                  <span className="text-sm text-text-tertiary flex-1">Unassigned</span>
                  <div className="flex items-center gap-2 flex-shrink-0">
                    {currentUser?.id && (
                      <button
                        onClick={() => handleAssignCommander(currentUser.id!)}
                        disabled={assigningCommander}
                        className="text-xs text-brand-primary hover:underline disabled:opacity-40 font-medium"
                      >
                        Claim
                      </button>
                    )}
                    <button
                      onClick={() => setShowCommanderSearch(v => !v)}
                      disabled={assigningCommander}
                      className="text-xs text-text-tertiary hover:text-text-primary disabled:opacity-40"
                    >
                      Assign
                    </button>
                  </div>
                </div>
              )}

              {/* Inline user search */}
              {showCommanderSearch && (
                <div className="mt-2 border border-border rounded-lg overflow-hidden shadow-sm bg-white">
                  <div className="flex items-center gap-2 px-2.5 py-2 border-b border-border">
                    <Search className="w-3.5 h-3.5 text-text-tertiary flex-shrink-0" />
                    <input
                      ref={searchInputRef}
                      type="text"
                      value={commanderQuery}
                      onChange={e => setCommanderQuery(e.target.value)}
                      placeholder="Search users…"
                      className="flex-1 text-xs bg-transparent outline-none text-text-primary placeholder:text-text-tertiary"
                    />
                    <button onClick={() => { setShowCommanderSearch(false); setCommanderQuery('') }}>
                      <X className="w-3.5 h-3.5 text-text-tertiary hover:text-text-primary transition-colors" />
                    </button>
                  </div>
                  <div className="max-h-40 overflow-y-auto py-1">
                    {incident.commander_id && (
                      <button
                        onClick={() => handleAssignCommander(null)}
                        className="w-full text-left px-3 py-1.5 text-xs text-red-500 hover:bg-surface-secondary transition-colors"
                      >
                        Remove commander
                      </button>
                    )}
                    {filteredUsers.length === 0 ? (
                      <p className="px-3 py-2 text-xs text-text-tertiary italic">No users found</p>
                    ) : (
                      filteredUsers.map(u => (
                        <button
                          key={u.id}
                          onClick={() => handleAssignCommander(u.id)}
                          className="w-full text-left px-3 py-1.5 text-xs text-text-primary hover:bg-surface-secondary transition-colors flex items-center gap-2"
                        >
                          <Avatar name={u.name || u.email} size="sm" />
                          <span className="truncate">{u.name || u.email}</span>
                        </button>
                      ))
                    )}
                  </div>
                </div>
              )}
            </div>
          </Section>

          {/* TIME */}
          <Section title="Time" sectionKey="time" open={open.time} onToggle={() => toggle('time')}>
            <div className="space-y-2">
              <div className="flex items-center gap-2">
                <Timer className="w-3.5 h-3.5 text-text-tertiary flex-shrink-0" />
                <span className="text-sm text-text-primary font-medium tabular-nums">
                  {formatDuration(incident.triggered_at, incident.resolved_at)}
                </span>
                {!incident.resolved_at && (
                  <span className="text-xs text-amber-500 font-medium">ongoing</span>
                )}
              </div>
              <TimeRow label="Triggered" ts={incident.triggered_at} />
              {incident.acknowledged_at && <TimeRow label="Acknowledged" ts={incident.acknowledged_at} />}
              {incident.resolved_at && <TimeRow label="Resolved" ts={incident.resolved_at} />}
              <div className="flex items-center justify-between">
                <span className="text-xs text-text-tertiary flex items-center gap-1.5">
                  <Activity className="w-3.5 h-3.5 flex-shrink-0" />
                  Last activity
                </span>
                <span className="text-xs text-text-secondary tabular-nums">
                  {formatRelativeTime(lastActivityTs)}
                </span>
              </div>
            </div>
          </Section>

          {/* CUSTOM FIELDS */}
          {customFieldDefs.length > 0 && (
            <Section title="Custom Fields" sectionKey="customFields" open={open.customFields} onToggle={() => toggle('customFields')}>
              <div className="bg-surface-secondary border border-border rounded-lg overflow-hidden">
                {customFieldDefs.map((def, idx) => {
                  const currentValue = incident.custom_fields?.[def.key] ?? ''
                  const isEditing = editingCfKey === def.key
                  const isFlashing = flashingCfKey === def.key
                  return (
                    <div
                      key={def.key}
                      className={[
                        'flex items-center gap-2 px-3 py-2 transition-colors duration-300',
                        idx < customFieldDefs.length - 1 ? 'border-b border-border' : '',
                        isFlashing ? 'bg-brand-primary-light' : '',
                      ].join(' ')}
                    >
                      <FieldTypeIcon type={def.field_type} />
                      <span
                        className="text-xs text-text-secondary flex-shrink-0 truncate"
                        style={{ width: '72px' }}
                        title={def.name}
                      >
                        {def.name}
                      </span>
                      {isEditing ? (
                        <div className="flex-1 min-w-0">
                          {def.field_type === 'dropdown' ? (
                            <select
                              autoFocus
                              value={cfDraft}
                              onChange={e => setCfDraft(e.target.value)}
                              onBlur={() => saveCfValue(def.key, cfDraft)}
                              className="w-full px-1.5 py-0.5 text-xs border border-brand-primary rounded bg-white focus:outline-none"
                            >
                              <option value="">— clear —</option>
                              {def.options.map(opt => (
                                <option key={opt.value} value={opt.value}>{opt.label}</option>
                              ))}
                            </select>
                          ) : (
                            <input
                              autoFocus
                              type={def.field_type === 'number' ? 'number' : 'text'}
                              value={cfDraft}
                              onChange={e => setCfDraft(e.target.value)}
                              onBlur={() => saveCfValue(def.key, cfDraft)}
                              onKeyDown={e => {
                                if (e.key === 'Enter') saveCfValue(def.key, cfDraft)
                                if (e.key === 'Escape') setEditingCfKey(null)
                              }}
                              className="w-full px-1.5 py-0.5 text-xs border border-brand-primary rounded bg-white focus:outline-none"
                            />
                          )}
                        </div>
                      ) : (
                        <button
                          onClick={() => { setEditingCfKey(def.key); setCfDraft(currentValue) }}
                          className="flex-1 text-right text-xs truncate hover:text-brand-primary transition-colors"
                          title={currentValue || 'Click to set'}
                        >
                          {currentValue
                            ? <span className="text-text-primary">{currentValue}</span>
                            : <span className="text-text-tertiary italic">+ Add value</span>
                          }
                        </button>
                      )}
                    </div>
                  )
                })}
              </div>
            </Section>
          )}

          {/* CHANNELS */}
          {(incident.slack_channel_name || incident.teams_channel_name) && (
            <Section title="Channels" sectionKey="channels" open={open.channels} onToggle={() => toggle('channels')}>
              <div className="space-y-2">
                {incident.slack_channel_name && (
                  <a
                    href={`https://slack.com/app_redirect?channel=${incident.slack_channel_id}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex items-center gap-2 text-sm text-brand-primary hover:underline"
                  >
                    <Hash className="w-3.5 h-3.5 flex-shrink-0" />
                    <span className="truncate flex-1">{incident.slack_channel_name}</span>
                    <ExternalLink className="w-3 h-3 flex-shrink-0" />
                  </a>
                )}
                {incident.teams_channel_name && (
                  <a
                    href={`https://teams.microsoft.com/l/channel/${encodeURIComponent(incident.teams_channel_id ?? '')}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex items-center gap-2 text-sm text-brand-primary hover:underline"
                  >
                    <Hash className="w-3.5 h-3.5 flex-shrink-0" />
                    <span className="truncate flex-1">{incident.teams_channel_name}</span>
                    <ExternalLink className="w-3 h-3 flex-shrink-0" />
                  </a>
                )}
              </div>
            </Section>
          )}

          {/* ALERTS */}
          {incident.alerts.length > 0 && (
            <Section title="Linked Alerts" sectionKey="alerts" open={open.alerts} onToggle={() => toggle('alerts')}>
              <div className="flex items-center gap-2">
                <AlertCircle className="w-4 h-4 text-text-tertiary" />
                <span className="text-sm text-text-primary">
                  {incident.alerts.length} alert{incident.alerts.length !== 1 ? 's' : ''}
                </span>
              </div>
            </Section>
          )}

          {/* HANDOFF DIGEST */}
          <div className="px-4 py-3 border-t border-border">
            <HandoffDigest incidentId={incident.id} lastActivityAt={lastActivityAt} />
          </div>

          {/* DEBUG ID */}
          <div className="px-4 py-3 border-t border-border">
            <span className="text-xs font-mono text-text-tertiary break-all select-all">{incident.id}</span>
          </div>

        </div>
      )}
    </div>
  )
}

// ── Subcomponents ─────────────────────────────────────────────────────────────

function Section({
  title,
  sectionKey: _k,
  open,
  onToggle,
  children,
}: {
  title: string
  sectionKey: SectionKey
  open: boolean
  onToggle: () => void
  children: React.ReactNode
}) {
  return (
    <div className="border-b border-border">
      <button
        onClick={onToggle}
        className="flex items-center justify-between w-full px-4 py-2.5 hover:bg-surface-secondary/60 transition-colors"
      >
        <span className="text-xs font-semibold text-text-tertiary uppercase tracking-wider">{title}</span>
        <ChevronRight
          className={`w-3.5 h-3.5 text-text-tertiary transition-transform duration-200 ${open ? 'rotate-90' : ''}`}
        />
      </button>
      {/* CSS grid trick: grid-rows-[0fr]/[1fr] + overflow-hidden = smooth collapse, no JS height */}
      <div className={`grid transition-all duration-200 ease-in-out ${open ? 'grid-rows-[1fr]' : 'grid-rows-[0fr]'}`}>
        <div className="overflow-hidden">
          <div className="px-4 pb-3">{children}</div>
        </div>
      </div>
    </div>
  )
}

function FieldTypeIcon({ type }: { type: string }) {
  if (type === 'number') return <Hash className="w-3.5 h-3.5 text-text-tertiary flex-shrink-0" />
  if (type === 'dropdown') return <ChevronDown className="w-3.5 h-3.5 text-text-tertiary flex-shrink-0" />
  return <Type className="w-3.5 h-3.5 text-text-tertiary flex-shrink-0" />
}

function TimeRow({ label, ts }: { label: string; ts: string }) {
  return (
    <div className="flex items-center justify-between">
      <span className="text-xs text-text-tertiary flex items-center gap-1.5">
        <Clock className="w-3.5 h-3.5 flex-shrink-0" />
        {label}
      </span>
      <span className="text-xs text-text-secondary tabular-nums" title={formatDateTime(ts)}>
        {formatRelativeTime(ts)}
      </span>
    </div>
  )
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function getLastActivity(timeline: TimelineEntry[], fallback: string): string {
  if (timeline.length === 0) return fallback
  return timeline.reduce((a, b) =>
    new Date(a.timestamp) > new Date(b.timestamp) ? a : b
  ).timestamp
}

function formatDateTime(ts: string): string {
  return new Date(ts).toLocaleString('en-US', {
    month: 'short', day: 'numeric', year: 'numeric',
    hour: 'numeric', minute: '2-digit', hour12: true,
  })
}

function formatDuration(startTs: string, endTs?: string): string {
  const diff = Math.max(0, Math.floor(
    ((endTs ? new Date(endTs) : new Date()).getTime() - new Date(startTs).getTime()) / 1000
  ))
  if (diff < 60) return `${diff}s`
  if (diff < 3600) return `${Math.floor(diff / 60)}m`
  const h = Math.floor(diff / 3600)
  const m = Math.floor((diff % 3600) / 60)
  return m > 0 ? `${h}h ${m}m` : `${h}h`
}

function formatRelativeTime(ts: string): string {
  const diff = Math.max(0, Math.floor((Date.now() - new Date(ts).getTime()) / 1000))
  if (diff < 60) return 'Just now'
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return `${Math.floor(diff / 86400)}d ago`
}
