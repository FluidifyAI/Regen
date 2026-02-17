import { useState, useEffect, useCallback } from 'react'
import { listEscalationPolicies } from '../api/escalation'
import type { EscalationPolicy } from '../api/types'

interface UseEscalationPoliciesResult {
  policies: EscalationPolicy[]
  loading: boolean
  error: string | null
  total: number
  refetch: () => Promise<void>
}

/**
 * Hook for fetching all escalation policies.
 * Auto-fetches on mount; expose refetch for manual invalidation after mutations.
 */
export function useEscalationPolicies(): UseEscalationPoliciesResult {
  const [policies, setPolicies] = useState<EscalationPolicy[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [total, setTotal] = useState(0)

  const fetchPolicies = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await listEscalationPolicies()
      setPolicies(response.data)
      setTotal(response.total)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch escalation policies')
      setPolicies([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchPolicies()
  }, [fetchPolicies])

  return { policies, loading, error, total, refetch: fetchPolicies }
}
