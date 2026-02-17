import { useState, useEffect, useCallback } from 'react'
import { getEscalationPolicy } from '../api/escalation'
import type { EscalationPolicy } from '../api/types'

interface UseEscalationPolicyResult {
  policy: EscalationPolicy | null
  loading: boolean
  error: string | null
  refetch: () => Promise<void>
}

/**
 * Hook for fetching a single escalation policy with its tiers.
 */
export function useEscalationPolicy(id: string): UseEscalationPolicyResult {
  const [policy, setPolicy] = useState<EscalationPolicy | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchPolicy = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const data = await getEscalationPolicy(id)
      setPolicy(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch escalation policy')
      setPolicy(null)
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => {
    fetchPolicy()
  }, [fetchPolicy])

  return { policy, loading, error, refetch: fetchPolicy }
}
