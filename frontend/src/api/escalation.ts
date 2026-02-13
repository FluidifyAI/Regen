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
