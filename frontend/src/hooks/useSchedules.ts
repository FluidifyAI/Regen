import { useState, useEffect, useCallback } from 'react'
import { listSchedules } from '../api/schedules'
import type { Schedule } from '../api/types'

interface UseSchedulesResult {
  schedules: Schedule[]
  loading: boolean
  error: string | null
  total: number
  refetch: () => Promise<void>
}

/**
 * Hook for fetching the list of all schedules.
 * Auto-fetches on mount; exposes refetch for manual invalidation after mutations.
 */
export function useSchedules(): UseSchedulesResult {
  const [schedules, setSchedules] = useState<Schedule[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [total, setTotal] = useState(0)

  const fetchSchedules = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await listSchedules()
      setSchedules(response.data)
      setTotal(response.total)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch schedules')
      setSchedules([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchSchedules()
  }, [fetchSchedules])

  return { schedules, loading, error, total, refetch: fetchSchedules }
}
