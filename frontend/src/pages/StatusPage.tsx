import { useState, useEffect, useCallback } from 'react'
import { useVisibilityAwareInterval } from '../hooks/useVisibilityAwareInterval'
import { CheckCircle, AlertTriangle, Clock, RefreshCw } from 'lucide-react'

interface StatusIncident {
  incident_number: number
  title: string
  severity: string
  status: string
  triggered_at: string
  resolved_at?: string
  duration_seconds?: number
}

interface StatusPageData {
  org_name: string
  generated_at: string
  active_incidents: StatusIncident[]
  recently_resolved: StatusIncident[]
}

const SEVERITY_COLOR: Record<string, string> = {
  critical: 'text-red-600 bg-red-50 border-red-200',
  high:     'text-orange-600 bg-orange-50 border-orange-200',
  medium:   'text-yellow-600 bg-yellow-50 border-yellow-200',
  low:      'text-blue-600 bg-blue-50 border-blue-200',
}

const SEVERITY_DOT: Record<string, string> = {
  critical: 'bg-red-500',
  high:     'bg-orange-500',
  medium:   'bg-yellow-500',
  low:      'bg-blue-500',
}

function formatDuration(secs: number): string {
  if (secs < 60) return `${secs}s`
  if (secs < 3600) return `${Math.floor(secs / 60)}m`
  const h = Math.floor(secs / 3600)
  const m = Math.floor((secs % 3600) / 60)
  return m > 0 ? `${h}h ${m}m` : `${h}h`
}

function formatTime(iso: string): string {
  return new Date(iso).toLocaleString(undefined, {
    month: 'short', day: 'numeric',
    hour: '2-digit', minute: '2-digit',
  })
}

function timeAgo(iso: string): string {
  const diff = Math.floor((Date.now() - new Date(iso).getTime()) / 1000)
  if (diff < 60) return 'just now'
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return `${Math.floor(diff / 86400)}d ago`
}

function IncidentRow({ incident, resolved }: { incident: StatusIncident; resolved?: boolean }) {
  const dotClass = SEVERITY_DOT[incident.severity] ?? 'bg-gray-400'
  const badgeClass = SEVERITY_COLOR[incident.severity] ?? 'text-gray-600 bg-gray-50 border-gray-200'

  return (
    <div className="flex items-start gap-4 py-4 border-b border-gray-100 last:border-0">
      <div className="mt-1.5 flex-shrink-0">
        <span className={`inline-block w-2.5 h-2.5 rounded-full ${dotClass} ${!resolved ? 'animate-pulse' : ''}`} />
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 flex-wrap">
          <span className="text-xs font-mono text-gray-400">INC-{incident.incident_number}</span>
          <span className={`text-xs font-medium px-2 py-0.5 rounded-full border ${badgeClass}`}>
            {incident.severity}
          </span>
          {resolved && (
            <span className="text-xs font-medium px-2 py-0.5 rounded-full bg-green-50 border border-green-200 text-green-700">
              resolved
            </span>
          )}
        </div>
        <p className="text-sm font-medium text-gray-900 mt-1">{incident.title}</p>
        <div className="flex items-center gap-3 mt-1 text-xs text-gray-500">
          <span className="flex items-center gap-1">
            <Clock className="w-3 h-3" />
            {resolved ? formatTime(incident.triggered_at) : timeAgo(incident.triggered_at)}
          </span>
          {incident.duration_seconds !== undefined && (
            <span>Duration: {formatDuration(incident.duration_seconds)}</span>
          )}
        </div>
      </div>
    </div>
  )
}

export function StatusPage() {
  const [data, setData] = useState<StatusPageData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [lastRefresh, setLastRefresh] = useState<Date>(new Date())

  const fetch = useCallback(async () => {
    try {
      const res = await window.fetch('/api/v1/status')
      if (!res.ok) throw new Error('Failed to load status')
      const json = await res.json() as StatusPageData
      setData(json)
      setError(null)
      setLastRefresh(new Date())
    } catch {
      setError('Unable to load status information.')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { fetch() }, [fetch])
  useVisibilityAwareInterval(fetch, 30_000)

  const isHealthy = !loading && !error && data && data.active_incidents.length === 0

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <div className="bg-white border-b border-gray-200">
        <div className="max-w-2xl mx-auto px-6 py-8">
          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-2xl font-bold text-gray-900">
                {data?.org_name ?? 'System'} Status
              </h1>
              <p className="text-sm text-gray-500 mt-1">
                Real-time incident status
              </p>
            </div>
            <button
              onClick={fetch}
              className="p-2 text-gray-400 hover:text-gray-600 transition-colors"
              title="Refresh"
            >
              <RefreshCw className="w-4 h-4" />
            </button>
          </div>

          {/* Overall health banner */}
          {!loading && !error && (
            <div className={`mt-6 flex items-center gap-3 px-4 py-3 rounded-lg ${
              isHealthy
                ? 'bg-green-50 border border-green-200'
                : 'bg-red-50 border border-red-200'
            }`}>
              {isHealthy ? (
                <>
                  <CheckCircle className="w-5 h-5 text-green-600 flex-shrink-0" />
                  <span className="text-sm font-medium text-green-800">All systems operational</span>
                </>
              ) : (
                <>
                  <AlertTriangle className="w-5 h-5 text-red-600 flex-shrink-0" />
                  <span className="text-sm font-medium text-red-800">
                    {data?.active_incidents.length} active incident{data && data.active_incidents.length !== 1 ? 's' : ''}
                  </span>
                </>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Content */}
      <div className="max-w-2xl mx-auto px-6 py-8 space-y-8">
        {loading && (
          <div className="space-y-3">
            {[1, 2, 3].map(i => (
              <div key={i} className="h-16 bg-white rounded-lg border border-gray-200 animate-pulse" />
            ))}
          </div>
        )}

        {error && (
          <div className="bg-white border border-gray-200 rounded-lg p-6 text-center text-sm text-gray-500">
            {error}
          </div>
        )}

        {data && !loading && (
          <>
            {/* Active incidents */}
            <section>
              <h2 className="text-xs font-semibold uppercase tracking-wider text-gray-500 mb-3">
                Active Incidents
              </h2>
              <div className="bg-white border border-gray-200 rounded-lg px-4">
                {data.active_incidents.length === 0 ? (
                  <p className="py-6 text-sm text-center text-gray-400">No active incidents</p>
                ) : (
                  data.active_incidents.map(inc => (
                    <IncidentRow key={inc.incident_number} incident={inc} />
                  ))
                )}
              </div>
            </section>

            {/* Recently resolved */}
            <section>
              <h2 className="text-xs font-semibold uppercase tracking-wider text-gray-500 mb-3">
                Resolved — Last 7 Days
              </h2>
              <div className="bg-white border border-gray-200 rounded-lg px-4">
                {data.recently_resolved.length === 0 ? (
                  <p className="py-6 text-sm text-center text-gray-400">No resolved incidents in the last 7 days</p>
                ) : (
                  data.recently_resolved.map(inc => (
                    <IncidentRow key={inc.incident_number} incident={inc} resolved />
                  ))
                )}
              </div>
            </section>
          </>
        )}

        {/* Footer */}
        <p className="text-center text-xs text-gray-400">
          Auto-refreshes every 30s · Last updated {lastRefresh.toLocaleTimeString()}
        </p>
      </div>
    </div>
  )
}
