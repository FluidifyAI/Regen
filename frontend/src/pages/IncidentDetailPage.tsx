import { useState, useEffect } from 'react'
import { useParams, Link } from 'react-router-dom'
import { ChevronRight, Bell, ChevronDown } from 'lucide-react'
import { useIncidentDetail } from '../hooks/useIncidentDetail'
import { Timeline } from '../components/incidents/Timeline'
import { PropertiesPanel } from '../components/layout/PropertiesPanel'
import { StatusDropdown } from '../components/incidents/StatusDropdown'
import { SeverityDropdown } from '../components/incidents/SeverityDropdown'
import { AddTimelineEntry } from '../components/incidents/AddTimelineEntry'
import { GroupedAlerts } from '../components/incidents/GroupedAlerts'
import { AISummaryPanel } from '../components/incidents/AISummaryPanel'
import { PostMortemPanel } from '../components/incidents/PostMortemPanel'
import { ToastContainer, useToast } from '../components/ui/Toast'
import { GeneralError } from '../components/ui/ErrorState'
import { Button } from '../components/ui/Button'
import { listEscalationPolicies } from '../api/escalation'
import { apiClient } from '../api/client'
import type { EscalationPolicy } from '../api/types'

type TabType = 'activity' | 'alerts' | 'postmortem'

/**
 * Incident detail page with two-panel layout
 * Left: Content area with tabs (Activity, Alerts, Post-Mortem)
 * Right: Collapsible properties panel (includes Handoff Digest)
 */
export function IncidentDetailPage() {
  const { id } = useParams<{ id: string }>()
  const [activeTab, setActiveTab] = useState<TabType>('activity')
  const [hasPostMortem, setHasPostMortem] = useState(false)
  const { toasts, dismissToast, success, error: showError } = useToast()
  const [showEscalateModal, setShowEscalateModal] = useState(false)
  const [escalatePolicies, setEscalatePolicies] = useState<EscalationPolicy[]>([])
  const [selectedPolicyId, setSelectedPolicyId] = useState('')
  const [escalating, setEscalating] = useState(false)

  useEffect(() => {
    listEscalationPolicies().then(r => setEscalatePolicies(r.data)).catch(() => {})
  }, [])

  async function handleEscalate() {
    if (!selectedPolicyId || !id) return
    setEscalating(true)
    try {
      await apiClient.post(`/api/v1/incidents/${id}/escalate`, {
        escalation_policy_id: selectedPolicyId,
      })
      setShowEscalateModal(false)
      setSelectedPolicyId('')
      success('Escalation triggered')
    } catch {
      showError('Failed to trigger escalation')
    } finally {
      setEscalating(false)
    }
  }

  const { incident, loading, error, refetch } = useIncidentDetail(id || '')

  // Only show skeleton on initial load, not during refetch
  if (loading && !incident) {
    return (
      <div className="flex h-full">
        {/* Content Area */}
        <div className="flex-1 overflow-y-auto">
          <div className="max-w-5xl mx-auto px-6 py-6">
            <SkeletonLoader />
          </div>
        </div>
        {/* Properties Panel skeleton */}
        <div className="hidden lg:block w-80">
          <div className="bg-white border-l border-border h-full" />
        </div>
      </div>
    )
  }

  if (error || !incident) {
    return (
      <div className="flex h-full items-center justify-center">
        <GeneralError
          message={error || 'Incident not found'}
          onRetry={refetch}
        />
      </div>
    )
  }

  // Latest timeline activity timestamp for AI stale detection
  const lastActivityAt = incident.timeline.length > 0
    ? incident.timeline.reduce((a, b) =>
        new Date(a.timestamp) > new Date(b.timestamp) ? a : b
      ).timestamp
    : incident.triggered_at

  return (
    <div className="flex h-full">
      {/* Content Area */}
      <div className="flex-1 overflow-y-auto bg-surface-secondary">
        <div className="max-w-5xl mx-auto px-6 py-6">
          {/* Breadcrumb */}
          <nav className="flex items-center gap-2 text-sm mb-4">
            <Link to="/" className="text-text-tertiary hover:text-text-primary">
              Home
            </Link>
            <ChevronRight className="w-4 h-4 text-text-tertiary" />
            <Link to="/incidents" className="text-text-tertiary hover:text-text-primary">
              Incidents
            </Link>
            <ChevronRight className="w-4 h-4 text-text-tertiary" />
            <span className="text-text-primary font-medium">
              INC-{incident.incident_number}
            </span>
          </nav>

          {/* Page Header */}
          <div className="bg-white border border-border rounded-lg p-6 mb-6">
            <div className="flex items-start justify-between mb-4">
              <div className="flex-1">
                <div className="flex items-center gap-3 mb-2">
                  <span className="text-sm text-text-tertiary font-medium">
                    INC-{incident.incident_number}
                  </span>
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
                  <Button
                    variant="ghost"
                    onClick={() => setShowEscalateModal(true)}
                  >
                    <Bell className="w-4 h-4 mr-1" />
                    Escalate
                  </Button>
                </div>
                <h1 className="text-2xl font-semibold text-text-primary mb-2">
                  {incident.title}
                </h1>
                {incident.summary && (
                  <p className="text-sm text-text-secondary">{incident.summary}</p>
                )}
              </div>
            </div>

            {/* AI Summary — inline in header */}
            <AISummaryPanel
              incidentId={incident.id}
              existingSummary={incident.ai_summary}
              existingSummaryGeneratedAt={incident.ai_summary_generated_at}
              lastActivityAt={lastActivityAt}
              onSummaryGenerated={refetch}
            />
          </div>

          {/* Tabs */}
          <div className="border-b border-border mb-6">
            <div className="flex gap-6">
              <TabButton
                active={activeTab === 'activity'}
                onClick={() => setActiveTab('activity')}
                label="Activity"
                count={incident.timeline.length}
              />
              <TabButton
                active={activeTab === 'alerts'}
                onClick={() => setActiveTab('alerts')}
                label="Alerts"
                count={incident.alerts.length}
              />
              <TabButton
                active={activeTab === 'postmortem'}
                onClick={() => setActiveTab('postmortem')}
                label="Post-Mortem"
                count={hasPostMortem ? 1 : 0}
              />
            </div>
          </div>

          {/* Tab Content */}
          <div className="bg-white border border-border rounded-lg p-6">
            {activeTab === 'activity' && (
              <div className="space-y-6">
                <AddTimelineEntry
                  incidentId={incident.id}
                  onSuccess={success}
                  onError={showError}
                  onEntryAdded={refetch}
                />
                <Timeline entries={incident.timeline} />
              </div>
            )}
            {activeTab === 'alerts' && (
              <GroupedAlerts alerts={incident.alerts} incident={incident} />
            )}
            {activeTab === 'postmortem' && (
              <PostMortemPanel incidentId={incident.id} onPostMortemLoaded={setHasPostMortem} />
            )}
          </div>
        </div>
      </div>

      {/* Properties Panel — hidden on mobile, visible on large screens */}
      <div className="hidden lg:block w-80 flex-shrink-0">
        <PropertiesPanel incident={incident} onIncidentUpdated={refetch} lastActivityAt={lastActivityAt} />
      </div>

      {/* Toast Notifications */}
      <ToastContainer toasts={toasts} onDismiss={dismissToast} />

      {/* Escalate Modal */}
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

/**
 * Tab button component
 */
function TabButton({
  active,
  onClick,
  label,
  count,
}: {
  active: boolean
  onClick: () => void
  label: string
  count: number
}) {
  return (
    <button
      onClick={onClick}
      className={`pb-3 border-b-2 transition-colors ${
        active
          ? 'border-brand-primary text-text-primary font-medium'
          : 'border-transparent text-text-tertiary hover:text-text-primary'
      }`}
    >
      {label}{' '}
      <span
        className={`text-sm ${active ? 'text-text-secondary' : 'text-text-tertiary'}`}
      >
        ({count})
      </span>
    </button>
  )
}


/**
 * Loading skeleton
 */
function SkeletonLoader() {
  return (
    <div className="space-y-6">
      {/* Breadcrumb skeleton */}
      <div className="h-4 w-48 bg-surface-tertiary rounded animate-pulse" />

      {/* Header skeleton */}
      <div className="bg-white border border-border rounded-lg p-6">
        <div className="flex gap-3 mb-2">
          <div className="h-5 w-20 bg-surface-tertiary rounded animate-pulse" />
          <div className="h-5 w-16 bg-surface-tertiary rounded animate-pulse" />
          <div className="h-5 w-24 bg-surface-tertiary rounded animate-pulse" />
        </div>
        <div className="h-8 w-96 bg-surface-tertiary rounded animate-pulse mb-2" />
        <div className="h-4 w-full bg-surface-tertiary rounded animate-pulse" />
      </div>

      {/* Tabs skeleton */}
      <div className="flex gap-6 border-b border-border pb-3">
        <div className="h-5 w-24 bg-surface-tertiary rounded animate-pulse" />
        <div className="h-5 w-20 bg-surface-tertiary rounded animate-pulse" />
      </div>

      {/* Content skeleton */}
      <div className="bg-white border border-border rounded-lg p-6">
        <div className="space-y-4">
          <div className="h-4 w-32 bg-surface-tertiary rounded animate-pulse" />
          <div className="h-20 bg-surface-tertiary rounded animate-pulse" />
          <div className="h-20 bg-surface-tertiary rounded animate-pulse" />
        </div>
      </div>
    </div>
  )
}
