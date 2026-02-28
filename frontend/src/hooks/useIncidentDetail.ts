import { useState, useEffect, useCallback, useRef } from 'react'
import { getIncident } from '../api/incidents'
import type { Alert, TimelineEntry } from '../api/types'

type StatusType = 'triggered' | 'acknowledged' | 'resolved' | 'canceled'
type SeverityType = 'critical' | 'high' | 'medium' | 'low'

interface IncidentDetail {
  id: string
  incident_number: number
  title: string
  slug: string
  status: StatusType
  severity: SeverityType
  summary: string
  slack_channel_id?: string
  slack_channel_name?: string
  created_at: string
  triggered_at: string
  acknowledged_at?: string
  resolved_at?: string
  created_by_type: string
  created_by_id?: string
  commander_id?: string
  alerts: Alert[]
  timeline: TimelineEntry[]
  // AI Summarization (v0.6+)
  ai_summary?: string
  ai_summary_generated_at?: string
  // AI Agents (v0.9+)
  ai_enabled: boolean
}

interface UseIncidentDetailResult {
  incident: IncidentDetail | null
  loading: boolean
  error: string | null
  refetch: () => Promise<void>
}

/** Poll every 20 seconds. Stops automatically when the incident reaches a terminal state. */
const POLL_INTERVAL_MS = 20_000
const TERMINAL_STATUSES: StatusType[] = ['resolved', 'canceled']

export function useIncidentDetail(id: string): UseIncidentDetailResult {
  const [incident, setIncident] = useState<IncidentDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Ref so the polling interval can read the latest status without a stale closure
  const incidentRef = useRef<IncidentDetail | null>(null)
  incidentRef.current = incident

  const fetchIncident = useCallback(async () => {
    if (!id) return

    setLoading(true)
    setError(null)

    try {
      const data = await getIncident(id)
      setIncident(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch incident')
      setIncident(null)
    } finally {
      setLoading(false)
    }
  }, [id])

  // Initial fetch
  useEffect(() => {
    fetchIncident()
  }, [fetchIncident])

  // Background polling — silent (no loading state, no error overlay on failure)
  // Only runs while the incident is in an active (non-terminal) state.
  useEffect(() => {
    if (!id) return

    const timer = setInterval(async () => {
      const current = incidentRef.current
      if (current && TERMINAL_STATUSES.includes(current.status)) return

      try {
        const data = await getIncident(id)
        setIncident(data)
      } catch {
        // Silently ignore poll failures — keep the last known data visible
      }
    }, POLL_INTERVAL_MS)

    return () => clearInterval(timer)
  }, [id])

  return {
    incident,
    loading,
    error,
    refetch: fetchIncident,
  }
}
