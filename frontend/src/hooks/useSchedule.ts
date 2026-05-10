import { useState, useEffect, useCallback } from 'react'
import { getSchedule, getOnCall, listOverrides, listUnavailabilities } from '../api/schedules'
import type { Schedule, OnCallResponse, ScheduleOverride, ScheduleUnavailability } from '../api/types'

interface UseScheduleResult {
  schedule: Schedule | null
  onCall: OnCallResponse | null
  overrides: ScheduleOverride[]
  unavailabilities: ScheduleUnavailability[]
  loading: boolean
  error: string | null
  refetch: () => Promise<void>
}

export function useSchedule(id: string): UseScheduleResult {
  const [schedule, setSchedule] = useState<Schedule | null>(null)
  const [onCall, setOnCall] = useState<OnCallResponse | null>(null)
  const [overrides, setOverrides] = useState<ScheduleOverride[]>([])
  const [unavailabilities, setUnavailabilities] = useState<ScheduleUnavailability[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchAll = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [scheduleData, onCallData, overridesData, unavailData] = await Promise.all([
        getSchedule(id),
        getOnCall(id).catch(() => null),
        listOverrides(id).catch(() => ({ data: [], total: 0 })),
        listUnavailabilities(id).catch(() => [] as ScheduleUnavailability[]),
      ])
      setSchedule(scheduleData)
      setOnCall(onCallData)
      setOverrides(overridesData.data)
      setUnavailabilities(unavailData)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch schedule')
      setSchedule(null)
      setOnCall(null)
      setOverrides([])
      setUnavailabilities([])
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => {
    fetchAll()
  }, [fetchAll])

  return { schedule, onCall, overrides, unavailabilities, loading, error, refetch: fetchAll }
}
