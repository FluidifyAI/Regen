import { apiClient } from './client'
import type {
  RoutingRule,
  CreateRoutingRuleRequest,
  UpdateRoutingRuleRequest,
  ListRoutingRulesParams,
} from './types'

/**
 * List routing rules with optional enabled filter
 */
export async function listRoutingRules(
  params?: ListRoutingRulesParams
): Promise<{ data: RoutingRule[]; total: number }> {
  return apiClient.get<{ data: RoutingRule[]; total: number }>('/api/v1/routing-rules', params)
}

/**
 * Get a single routing rule by ID
 */
export async function getRoutingRule(id: string): Promise<RoutingRule> {
  return apiClient.get<RoutingRule>(`/api/v1/routing-rules/${id}`)
}

/**
 * Create a new routing rule
 */
export async function createRoutingRule(body: CreateRoutingRuleRequest): Promise<RoutingRule> {
  return apiClient.post<RoutingRule>('/api/v1/routing-rules', body)
}

/**
 * Update an existing routing rule (partial update)
 */
export async function updateRoutingRule(
  id: string,
  body: UpdateRoutingRuleRequest
): Promise<RoutingRule> {
  return apiClient.patch<RoutingRule>(`/api/v1/routing-rules/${id}`, body)
}

/**
 * Delete a routing rule by ID
 */
export async function deleteRoutingRule(id: string): Promise<void> {
  return apiClient.delete<void>(`/api/v1/routing-rules/${id}`)
}
