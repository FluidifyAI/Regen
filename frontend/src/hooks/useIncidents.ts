import { useState, useEffect, useCallback, useRef } from 'react'
import { listIncidents } from '../api/incidents'
import type { Incident, ListIncidentsParams } from '../api/types'

interface UseIncidentsResult {
  incidents: Incident[]
  loading: boolean
  error: string | null
  total: number
  refetch: () => Promise<void>
}

const ACTIVE_STATUSES = new Set(['triggered', 'acknowledged'])
const POLL_INTERVAL_MS = 10_000

/**
 * Hook for fetching and managing incidents list.
 * Supports filtering by status, severity, and pagination.
 * Polls every 10 seconds when any displayed incident is active so new
 * incidents and status changes appear without a manual refresh.
 */
export function useIncidents(params: ListIncidentsParams = {}): UseIncidentsResult {
  const [incidents, setIncidents] = useState<Incident[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [total, setTotal] = useState(0)

  const incidentsRef = useRef<Incident[]>([])
  incidentsRef.current = incidents

  const fetchIncidents = useCallback(async () => {
    setLoading(true)
    setError(null)

    try {
      const response = await listIncidents(params)
      setIncidents(response.data)
      setTotal(response.total)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch incidents')
      setIncidents([])
    } finally {
      setLoading(false)
    }
  }, [
    params.status,
    params.severity,
    params.limit,
    params.page,
  ])

  useEffect(() => {
    fetchIncidents()
  }, [fetchIncidents])

  // Background polling — silent, stops when all visible incidents are terminal
  // or when the filter is already narrowed to a terminal status.
  useEffect(() => {
    if (params.status && !ACTIVE_STATUSES.has(params.status)) return

    const timer = setInterval(async () => {
      const current = incidentsRef.current
      // If every incident on screen is terminal, no need to keep polling
      if (current.length > 0 && current.every((i) => !ACTIVE_STATUSES.has(i.status))) return

      try {
        const response = await listIncidents(params)
        setIncidents(response.data)
        setTotal(response.total)
      } catch {
        // Silently ignore poll failures
      }
    }, POLL_INTERVAL_MS)

    return () => clearInterval(timer)
  }, [
    params.status,
    params.severity,
    params.limit,
    params.page,
  ])

  return {
    incidents,
    loading,
    error,
    total,
    refetch: fetchIncidents,
  }
}
