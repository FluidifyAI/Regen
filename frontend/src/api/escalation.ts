import { apiClient } from './client'
import type {
  EscalationPolicy,
  ListEscalationPoliciesResponse,
  CreateEscalationPolicyRequest,
  UpdateEscalationPolicyRequest,
  EscalationTier,
  CreateEscalationTierRequest,
  UpdateEscalationTierRequest,
} from './types'

export async function listEscalationPolicies(): Promise<ListEscalationPoliciesResponse> {
  return apiClient.get<ListEscalationPoliciesResponse>('/api/v1/escalation-policies')
}

export async function getEscalationPolicy(id: string): Promise<EscalationPolicy> {
  return apiClient.get<EscalationPolicy>(`/api/v1/escalation-policies/${id}`)
}

export async function createEscalationPolicy(
  req: CreateEscalationPolicyRequest
): Promise<EscalationPolicy> {
  return apiClient.post<EscalationPolicy>('/api/v1/escalation-policies', req)
}

export async function updateEscalationPolicy(
  id: string,
  req: UpdateEscalationPolicyRequest
): Promise<EscalationPolicy> {
  return apiClient.patch<EscalationPolicy>(`/api/v1/escalation-policies/${id}`, req)
}

export async function deleteEscalationPolicy(id: string): Promise<void> {
  return apiClient.delete<void>(`/api/v1/escalation-policies/${id}`)
}

export async function createEscalationTier(
  policyId: string,
  req: CreateEscalationTierRequest
): Promise<EscalationTier> {
  return apiClient.post<EscalationTier>(`/api/v1/escalation-policies/${policyId}/tiers`, req)
}

export async function updateEscalationTier(
  policyId: string,
  tierId: string,
  req: UpdateEscalationTierRequest
): Promise<EscalationTier> {
  return apiClient.patch<EscalationTier>(
    `/api/v1/escalation-policies/${policyId}/tiers/${tierId}`,
    req
  )
}

export async function deleteEscalationTier(policyId: string, tierId: string): Promise<void> {
  return apiClient.delete<void>(`/api/v1/escalation-policies/${policyId}/tiers/${tierId}`)
}

// ── Severity rules ────────────────────────────────────────────────────────────

export interface EscalationSeverityRule {
  id: string
  severity: string
  escalation_policy_id: string
  created_at: string
  updated_at: string
}

export async function listSeverityRules(): Promise<{ data: EscalationSeverityRule[] }> {
  return apiClient.get('/api/v1/escalation-policies/severity-rules')
}

export async function upsertSeverityRule(
  severity: string,
  escalationPolicyId: string,
): Promise<EscalationSeverityRule> {
  return apiClient.put(`/api/v1/escalation-policies/severity-rules/${severity}`, {
    escalation_policy_id: escalationPolicyId,
  })
}

export async function deleteSeverityRule(severity: string): Promise<void> {
  await apiClient.delete(`/api/v1/escalation-policies/severity-rules/${severity}`)
}

// ── Global escalation settings ────────────────────────────────────────────────

export async function getEscalationSettings(): Promise<{ global_fallback_policy_id: string | null }> {
  return apiClient.get('/api/v1/settings/escalation')
}

export async function updateEscalationSettings(
  globalFallbackPolicyId: string | null,
): Promise<void> {
  await apiClient.put('/api/v1/settings/escalation', {
    global_fallback_policy_id: globalFallbackPolicyId,
  })
}
