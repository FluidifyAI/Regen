import { useState, useEffect, useCallback } from 'react'
import { listIncidents } from '../api/incidents'
import type { Incident, ListIncidentsParams } from '../api/types'

interface UseIncidentsResult {
  incidents: Incident[]
  loading: boolean
  error: string | null
  total: number
  refetch: () => void
}

/**
 * Hook for fetching and managing incidents list
 * Supports filtering by status, severity, and pagination
 * Auto-refetches when filters change
 */
export function useIncidents(params: ListIncidentsParams = {}): UseIncidentsResult {
  const [incidents, setIncidents] = useState<Incident[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [total, setTotal] = useState(0)

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
    params.offset,
  ])

  useEffect(() => {
    fetchIncidents()
  }, [fetchIncidents])

  return {
    incidents,
    loading,
    error,
    total,
    refetch: fetchIncidents,
  }
}
