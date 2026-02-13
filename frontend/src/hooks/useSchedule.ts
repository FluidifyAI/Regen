import { useState, useEffect, useCallback } from 'react'
import { getSchedule, getOnCall, listOverrides } from '../api/schedules'
import type { Schedule, OnCallResponse, ScheduleOverride } from '../api/types'

interface UseScheduleResult {
  schedule: Schedule | null
  onCall: OnCallResponse | null
  overrides: ScheduleOverride[]
  loading: boolean
  error: string | null
  refetch: () => Promise<void>
}

/**
 * Hook for fetching a single schedule's full detail.
 * Parallel-fetches: schedule (with layers), current on-call, and upcoming overrides.
 * On-call and overrides are fetched with .catch() fallbacks so a missing/empty
 * schedule still loads cleanly.
 */
export function useSchedule(id: string): UseScheduleResult {
  const [schedule, setSchedule] = useState<Schedule | null>(null)
  const [onCall, setOnCall] = useState<OnCallResponse | null>(null)
  const [overrides, setOverrides] = useState<ScheduleOverride[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchAll = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [scheduleData, onCallData, overridesData] = await Promise.all([
        getSchedule(id),
        getOnCall(id).catch(() => null),
        listOverrides(id).catch(() => ({ data: [], total: 0 })),
      ])
      setSchedule(scheduleData)
      setOnCall(onCallData)
      setOverrides(overridesData.data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch schedule')
      setSchedule(null)
      setOnCall(null)
      setOverrides([])
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => {
    fetchAll()
  }, [fetchAll])

  return { schedule, onCall, overrides, loading, error, refetch: fetchAll }
}
