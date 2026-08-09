import { useState, useEffect, useRef } from 'react'
import { useParams, Link } from 'react-router-dom'
import { ChevronRight, Bell, ChevronDown, CheckCircle, XCircle } from 'lucide-react'
import { useIncidentDetail } from '../hooks/useIncidentDetail'
import { Timeline } from '../components/incidents/Timeline'
import { PropertiesPanel } from '../components/layout/PropertiesPanel'
import { StatusDropdown } from '../components/incidents/StatusDropdown'
import { SeverityDropdown } from '../components/incidents/SeverityDropdown'
import { ConfirmStatusChangeModal } from '../components/incidents/ConfirmStatusChangeModal'
import { AddTimelineEntry } from '../components/incidents/AddTimelineEntry'
import { GroupedAlerts } from '../components/incidents/GroupedAlerts'
import { AISummaryPanel } from '../components/incidents/AISummaryPanel'
import { NeuriPanel } from '../components/incidents/NeuriPanel'
import { PostMortemPanel } from '../components/incidents/PostMortemPanel'
import { AttachmentsPanel } from '../components/incidents/AttachmentsPanel'
import { ToastContainer, useToast } from '../components/ui/Toast'
import { GeneralError } from '../components/ui/ErrorState'
import { Button } from '../components/ui/Button'
import { Badge } from '../components/ui/Badge'
import { listEscalationPolicies } from '../api/escalation'
import { getPostMortem } from '../api/postmortems'
import { getNeuriResults } from '../api/neuri'
import { updateIncident } from '../api/incidents'
import { apiClient } from '../api/client'
import type { EscalationPolicy, NeuriResult } from '../api/types'

type TabType = 'activity' | 'alerts' | 'investigation' | 'postmortem' | 'attachments'
type StatusType = 'triggered' | 'acknowledged' | 'resolved' | 'canceled'

const SEVERITY_TINT: Record<string, string> = {
  critical: 'rgba(220,38,38,0.045)',
  high:     'rgba(234,88,12,0.045)',
  medium:   'rgba(245,158,11,0.045)',
  low:      'rgba(59,130,246,0.045)',
  resolved: 'rgba(22,163,74,0.045)',
  canceled: 'rgba(100,116,139,0.045)',
}

const SEVERITY_STRIPE: Record<string, string> = {
  critical: '#DC2626',
  high:     '#EA580C',
  medium:   '#F59E0B',
  low:      '#3B82F6',
}

function headerGradient(severity: string, status: string): string {
  const key = (status === 'resolved' || status === 'canceled') ? status : severity
  const tint = SEVERITY_TINT[key] ?? SEVERITY_TINT.low
  return `linear-gradient(180deg, ${tint} 0%, transparent 200px)`
}

export function IncidentDetailPage() {
  const { id } = useParams<{ id: string }>()
  const [activeTab, setActiveTab] = useState<TabType>('activity')
  const [hasPostMortem, setHasPostMortem] = useState(false)
  const [neuriResult, setNeuriResult] = useState<NeuriResult | null>(null)
  const [attachmentCount, setAttachmentCount] = useState(0)
  const { toasts, dismissToast, success, error: showError } = useToast()
  const [showEscalateModal, setShowEscalateModal] = useState(false)
  const [escalatePolicies, setEscalatePolicies] = useState<EscalationPolicy[]>([])
  const [selectedPolicyId, setSelectedPolicyId] = useState('')
  const [escalating, setEscalating] = useState(false)

  // Mobile quick-action bar state
  const [mobileActionPending, setMobileActionPending] = useState<StatusType | null>(null)
  const [mobileActionUpdating, setMobileActionUpdating] = useState(false)

  // IntersectionObserver drives sticky bar visibility — no scroll listeners
  const titleRef = useRef<HTMLHeadingElement>(null)
  const [stickyVisible, setStickyVisible] = useState(false)

  useEffect(() => {
    const el = titleRef.current
    if (!el) return
    const observer = new IntersectionObserver(
      (entries) => setStickyVisible(!(entries[0]?.isIntersecting ?? true)),
      { threshold: 0, rootMargin: '-56px 0px 0px 0px' }
    )
    observer.observe(el)
    return () => observer.disconnect()
  }, [])

  useEffect(() => {
    listEscalationPolicies().then(r => setEscalatePolicies(r.data)).catch(() => {})
  }, [])

  useEffect(() => {
    if (!id) return
    getPostMortem(id).then(pm => setHasPostMortem(!!pm)).catch(() => {})
    getNeuriResults(id).then(r => setNeuriResult(r.results[0] ?? null)).catch(() => {})
  }, [id])

  async function handleEscalate() {
    if (!selectedPolicyId || !id) return
    setEscalating(true)
    try {
      await apiClient.post(`/api/v1/incidents/${id}/escalate`, { escalation_policy_id: selectedPolicyId })
      setShowEscalateModal(false)
      setSelectedPolicyId('')
      success('Escalation triggered')
    } catch {
      showError('Failed to trigger escalation')
    } finally {
      setEscalating(false)
    }
  }

  async function handleMobileAction(newStatus: StatusType) {
    if (!id) return
    // resolved/canceled need confirmation — show modal first
    if (newStatus === 'resolved' || newStatus === 'canceled') {
      setMobileActionPending(newStatus)
      return
    }
    commitMobileAction(newStatus)
  }

  async function commitMobileAction(newStatus: StatusType) {
    if (!id) return
    setMobileActionPending(null)
    setMobileActionUpdating(true)
    try {
      await updateIncident(id, { status: newStatus })
      await refetch()
      success(`Status updated to ${newStatus}`)
    } catch (err) {
      showError(err instanceof Error ? err.message : 'Failed to update status')
    } finally {
      setMobileActionUpdating(false)
    }
  }

  const { incident, loading, error, refetch } = useIncidentDetail(id || '')

  if (loading && !incident) {
    return (
      <div className="flex h-full">
        <div className="flex-1 overflow-y-auto">
          <div className="max-w-5xl mx-auto px-4 sm:px-6 py-6"><SkeletonLoader /></div>
        </div>
        <div className="hidden lg:block w-80 flex-shrink-0">
          <div className="bg-white border-l border-border h-full" />
        </div>
      </div>
    )
  }

  if (error || !incident) {
    return (
      <div className="flex h-full items-center justify-center">
        <GeneralError message={error || 'Incident not found'} onRetry={refetch} />
      </div>
    )
  }

  const lastActivityAt = incident.timeline.length > 0
    ? incident.timeline.reduce((a, b) =>
        new Date(a.timestamp) > new Date(b.timestamp) ? a : b
      ).timestamp
    : incident.triggered_at

  const stripeColor = SEVERITY_STRIPE[incident.severity] ?? '#94A3B8'

  // Determine what quick action to offer on mobile
  const mobileAction: { label: string; status: StatusType; icon: typeof CheckCircle } | null =
    incident.status === 'triggered'
      ? { label: 'Acknowledge', status: 'acknowledged', icon: CheckCircle }
      : incident.status === 'acknowledged'
        ? { label: 'Resolve', status: 'resolved', icon: XCircle }
        : null

  return (
    <div className="flex h-full overflow-hidden">

      {/* Mobile quick-action confirmation modal */}
      <ConfirmStatusChangeModal
        isOpen={mobileActionPending !== null}
        newStatus={mobileActionPending ?? 'resolved'}
        onConfirm={() => mobileActionPending && commitMobileAction(mobileActionPending)}
        onCancel={() => setMobileActionPending(null)}
      />

      {/* ── Main content column ──────────────────────────────────────────── */}
      {/* pb-16 on mobile reserves space above the sticky action bar */}
      <div className="flex-1 overflow-y-auto bg-surface-secondary relative pb-16 sm:pb-0">

        {/* Sticky compact bar (IntersectionObserver-driven, no scroll listener) */}
        <div
          className={`sticky top-0 z-20 bg-white/95 backdrop-blur-sm border-b border-border shadow-sm
            transition-all duration-200 ease-out overflow-hidden
            ${stickyVisible ? 'opacity-100 translate-y-0 h-12' : 'opacity-0 -translate-y-1 pointer-events-none h-0'}`}
        >
          <div className="flex items-center gap-2 px-4 sm:px-6 h-12 max-w-5xl">
            <span className="text-xs font-mono text-text-tertiary flex-shrink-0">
              INC-{incident.incident_number}
            </span>
            <span className="w-px h-3.5 bg-border flex-shrink-0" />
            <span className="text-sm font-medium text-text-primary truncate flex-1 min-w-0">
              {incident.title}
            </span>
            <div className="flex items-center gap-1.5 sm:gap-2 flex-shrink-0">
              <Badge variant={incident.status} type="status">{incident.status}</Badge>
              <span className="hidden sm:inline-flex"><Badge variant={incident.severity} type="severity">{incident.severity}</Badge></span>
              <Button variant="ghost" size="sm" onClick={() => setShowEscalateModal(true)} className="hidden sm:flex">
                <Bell className="w-3.5 h-3.5 mr-1" />
                Escalate
              </Button>
            </div>
          </div>
        </div>

        {/* ── Header zone — severity-tinted, no card wrapper ──────────── */}
        <div
          className="px-4 sm:px-6 pt-6 pb-5"
          style={{ background: headerGradient(incident.severity, incident.status) }}
        >
          {/* Breadcrumb */}
          <nav className="flex items-center gap-1.5 text-sm mb-5">
            <Link to="/" className="text-text-tertiary hover:text-text-primary transition-colors hidden sm:inline">
              Home
            </Link>
            <ChevronRight className="w-3.5 h-3.5 text-text-tertiary hidden sm:inline" />
            <Link to="/incidents" className="text-text-tertiary hover:text-text-primary transition-colors">
              Incidents
            </Link>
            <ChevronRight className="w-3.5 h-3.5 text-text-tertiary" />
            <span className="text-text-secondary font-medium">INC-{incident.incident_number}</span>
          </nav>

          {/* Title with left severity stripe */}
          <div className="flex gap-3 sm:gap-4 mb-3 items-start">
            <div
              className="w-1 rounded-full flex-shrink-0 mt-1.5"
              style={{ backgroundColor: stripeColor, minHeight: '36px' }}
              aria-hidden
            />
            <h1
              ref={titleRef}
              className="text-xl sm:text-3xl font-bold text-text-primary leading-tight"
            >
              {incident.title}
            </h1>
          </div>

          {/* Summary */}
          {incident.summary && (
            <p className="text-sm text-text-secondary mb-4 pl-4 sm:pl-5 leading-relaxed">
              {incident.summary}
            </p>
          )}

          {/* Action row */}
          <div className="flex items-center gap-2 sm:gap-3 pl-4 sm:pl-5 flex-wrap">
            <SeverityDropdown
              incidentId={incident.id}
              currentSeverity={incident.severity}
              onSeverityChange={() => {}}
              onSuccess={success}
              onError={showError}
              onRefetch={refetch}
            />
            <StatusDropdown
              incidentId={incident.id}
              currentStatus={incident.status}
              onStatusChange={() => {}}
              onSuccess={success}
              onError={showError}
              onRefetch={refetch}
            />
            <Button variant="ghost" onClick={() => setShowEscalateModal(true)}>
              <Bell className="w-4 h-4 mr-1.5" />
              Escalate
            </Button>
          </div>
        </div>

        {/* ── Tabs — horizontally scrollable on mobile ───────────────── */}
        <div className="border-b border-border px-4 sm:px-6 bg-surface-secondary overflow-x-auto">
          <div className="flex gap-4 sm:gap-6 min-w-max">
            <TabButton active={activeTab === 'activity'} onClick={() => setActiveTab('activity')} label="Activity" count={incident.timeline.length} />
            <TabButton active={activeTab === 'alerts'} onClick={() => setActiveTab('alerts')} label="Alerts" count={incident.alerts.length} />
            <TabButton active={activeTab === 'investigation'} onClick={() => setActiveTab('investigation')} label="Neuri" count={neuriResult ? 1 : 0} />
            <TabButton active={activeTab === 'postmortem'} onClick={() => setActiveTab('postmortem')} label="Post-Mortem" count={hasPostMortem ? 1 : 0} />
            <TabButton active={activeTab === 'attachments'} onClick={() => setActiveTab('attachments')} label="Attachments" count={attachmentCount} />
          </div>
        </div>

        {/* ── Tab content — flat white, no card border ───────────────── */}
        <div className="bg-white">
          <div className="max-w-5xl mx-auto px-4 sm:px-6 py-4 sm:py-6">
            {activeTab === 'activity' && (
              <div className="space-y-6">
                <AISummaryPanel
                  incidentId={incident.id}
                  existingSummary={incident.ai_summary}
                  existingSummaryGeneratedAt={incident.ai_summary_generated_at}
                  lastActivityAt={lastActivityAt}
                  onSummaryGenerated={refetch}
                />
                {neuriResult && <NeuriActivityCard result={neuriResult} onViewFull={() => setActiveTab('investigation')} />}
                <AddTimelineEntry
                  incidentId={incident.id}
                  onSuccess={success}
                  onError={showError}
                  onEntryAdded={refetch}
                />
                <Timeline entries={incident.timeline} />
              </div>
            )}
            {activeTab === 'investigation' && (
              <NeuriPanel incidentId={incident.id} onResultLoaded={setNeuriResult} />
            )}
            {activeTab === 'alerts' && (
              <GroupedAlerts alerts={incident.alerts} incident={incident} />
            )}
            {activeTab === 'postmortem' && (
              <PostMortemPanel incidentId={incident.id} onPostMortemLoaded={setHasPostMortem} />
            )}
            {activeTab === 'attachments' && (
              <AttachmentsPanel incidentId={incident.id} onCountChange={setAttachmentCount} />
            )}
          </div>
        </div>
      </div>

      {/* ── Properties panel ────────────────────────────────────────────── */}
      <div className="hidden lg:block w-80 flex-shrink-0">
        <PropertiesPanel incident={incident} onIncidentUpdated={refetch} lastActivityAt={lastActivityAt} />
      </div>

      {/* ── Mobile sticky bottom action bar ─────────────────────────────── */}
      {mobileAction && (
        <div className="sm:hidden fixed bottom-0 inset-x-0 z-50 bg-white border-t border-border px-4 py-3 flex items-center gap-3 shadow-lg">
          <div className="flex-1 min-w-0">
            <p className="text-xs text-text-tertiary leading-none mb-0.5">INC-{incident.incident_number}</p>
            <p className="text-sm font-medium text-text-primary truncate">{incident.title}</p>
          </div>
          <button
            onClick={() => handleMobileAction(mobileAction.status)}
            disabled={mobileActionUpdating}
            className="flex items-center gap-1.5 bg-indigo-600 hover:bg-indigo-500 active:bg-indigo-700 disabled:opacity-50 text-white text-sm font-semibold px-4 py-2 rounded-lg flex-shrink-0 transition-colors"
          >
            <mobileAction.icon className="w-4 h-4" aria-hidden />
            {mobileActionUpdating ? '…' : mobileAction.label}
          </button>
        </div>
      )}

      <ToastContainer toasts={toasts} onDismiss={dismissToast} />

      {/* ── Escalate modal ───────────────────────────────────────────────── */}
      {showEscalateModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
          <div className="bg-white border border-border rounded-xl shadow-xl w-full max-w-sm mx-4 p-6">
            <h2 className="text-lg font-semibold text-text-primary mb-1">Escalate Incident</h2>
            <p className="text-sm text-text-secondary mb-4">
              Trigger an escalation policy to notify the on-call team.
            </p>
            <div className="relative mb-4">
              <select
                className="w-full px-3 py-2 pr-8 rounded-lg border border-border bg-surface-secondary text-text-primary text-sm appearance-none focus:outline-none focus:ring-2 focus:ring-brand-primary"
                value={selectedPolicyId}
                onChange={e => setSelectedPolicyId(e.target.value)}
              >
                <option value="">— Choose escalation policy —</option>
                {escalatePolicies.map(p => (
                  <option key={p.id} value={p.id}>{p.name}</option>
                ))}
              </select>
              <ChevronDown className="absolute right-2.5 top-2.5 w-4 h-4 text-text-tertiary pointer-events-none" />
            </div>
            <div className="flex justify-end gap-2">
              <Button variant="ghost" onClick={() => { setShowEscalateModal(false); setSelectedPolicyId('') }} disabled={escalating}>
                Cancel
              </Button>
              <Button onClick={handleEscalate} disabled={escalating || !selectedPolicyId}>
                {escalating ? 'Escalating…' : 'Escalate'}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

// ── Subcomponents ─────────────────────────────────────────────────────────────

const HYPOTHESIS_COLORS: Record<string, string> = {
  CODE_CHANGE:            'bg-blue-100 text-blue-800',
  CONFIG_CHANGE:          'bg-orange-100 text-orange-800',
  DEPENDENCY_FAILURE:     'bg-red-100 text-red-800',
  RESOURCE_EXHAUSTION:    'bg-amber-100 text-amber-800',
  TRAFFIC_ANOMALY:        'bg-purple-100 text-purple-800',
  INFRASTRUCTURE_FAILURE: 'bg-red-100 text-red-800',
  DATA_ISSUE:             'bg-teal-100 text-teal-800',
  CAPACITY_LIMIT:         'bg-orange-100 text-orange-800',
  UNKNOWN:                'bg-gray-100 text-gray-600',
}

function NeuriActivityCard({ result, onViewFull }: { result: NeuriResult; onViewFull: () => void }) {
  const label = result.top_hypothesis.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase())
  const badge = HYPOTHESIS_COLORS[result.top_hypothesis] ?? 'bg-gray-100 text-gray-600'
  return (
    <div className="rounded-lg border border-border bg-surface-secondary p-4 space-y-2">
      <div className="flex items-start justify-between gap-2 flex-wrap">
        <div className="flex items-center gap-2 flex-wrap">
          <span className="text-xs font-semibold text-text-tertiary uppercase tracking-wider">Neuri Investigation</span>
          <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-semibold ${badge}`}>{label}</span>
          <span className="text-xs font-semibold text-text-primary">{Math.round(result.confidence * 100)}% confidence</span>
        </div>
        <button onClick={onViewFull} className="text-xs text-brand-primary hover:underline flex-shrink-0">View full →</button>
      </div>
      <p className="text-sm text-text-secondary leading-relaxed">{result.summary}</p>
    </div>
  )
}

function TabButton({ active, onClick, label, count }: {
  active: boolean; onClick: () => void; label: string; count: number
}) {
  return (
    <button
      onClick={onClick}
      className={`pb-3 border-b-2 text-sm transition-colors whitespace-nowrap ${
        active
          ? 'border-brand-primary text-text-primary font-medium'
          : 'border-transparent text-text-tertiary hover:text-text-primary'
      }`}
    >
      {label}{' '}
      <span className={active ? 'text-text-secondary' : 'text-text-tertiary'}>({count})</span>
    </button>
  )
}

function SkeletonLoader() {
  return (
    <div className="space-y-6">
      <div className="h-4 w-48 bg-surface-tertiary rounded animate-pulse" />
      <div className="space-y-3">
        <div className="flex gap-3">
          <div className="h-6 w-20 bg-surface-tertiary rounded animate-pulse" />
          <div className="h-6 w-16 bg-surface-tertiary rounded animate-pulse" />
          <div className="h-6 w-24 bg-surface-tertiary rounded animate-pulse" />
        </div>
        <div className="h-9 w-96 bg-surface-tertiary rounded animate-pulse" />
        <div className="h-4 w-full max-w-xl bg-surface-tertiary rounded animate-pulse" />
      </div>
      <div className="flex gap-6 border-b border-border pb-3">
        <div className="h-5 w-24 bg-surface-tertiary rounded animate-pulse" />
        <div className="h-5 w-20 bg-surface-tertiary rounded animate-pulse" />
      </div>
      <div className="bg-white p-6 space-y-4">
        <div className="h-4 w-32 bg-surface-tertiary rounded animate-pulse" />
        <div className="h-20 bg-surface-tertiary rounded animate-pulse" />
        <div className="h-20 bg-surface-tertiary rounded animate-pulse" />
      </div>
    </div>
  )
}
