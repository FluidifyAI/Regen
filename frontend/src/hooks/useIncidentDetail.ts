import { useState, useEffect, useCallback } from 'react'
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
}

interface UseIncidentDetailResult {
  incident: IncidentDetail | null
  loading: boolean
  error: string | null
  refetch: () => void
}

export function useIncidentDetail(id: string): UseIncidentDetailResult {
  const [incident, setIncident] = useState<IncidentDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

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

  useEffect(() => {
    fetchIncident()
  }, [fetchIncident])

  return {
    incident,
    loading,
    error,
    refetch: fetchIncident,
  }
}
