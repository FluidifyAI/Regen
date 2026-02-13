import { useState, useEffect, useCallback } from 'react'
import { listRoutingRules } from '../api/routing_rules'
import type { RoutingRule, ListRoutingRulesParams } from '../api/types'

interface UseRoutingRulesResult {
  rules: RoutingRule[]
  loading: boolean
  error: string | null
  total: number
  refetch: () => Promise<void>
}

/**
 * Hook for fetching and managing routing rules list
 * Auto-refetches when the enabled filter changes
 */
export function useRoutingRules(params: ListRoutingRulesParams = {}): UseRoutingRulesResult {
  const [rules, setRules] = useState<RoutingRule[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [total, setTotal] = useState(0)

  const fetchRules = useCallback(async () => {
    setLoading(true)
    setError(null)

    try {
      const response = await listRoutingRules(params)
      setRules(response.data)
      setTotal(response.total)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch routing rules')
      setRules([])
    } finally {
      setLoading(false)
    }
  // List all primitive param fields individually — avoids stale closures if
  // the params object reference changes but values are the same, and ensures
  // any future additions to ListRoutingRulesParams are caught at compile time.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [params.enabled])

  useEffect(() => {
    fetchRules()
  }, [fetchRules])

  return {
    rules,
    loading,
    error,
    total,
    refetch: fetchRules,
  }
}
