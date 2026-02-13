import { apiClient } from './client'
import type { Alert, PaginatedResponse, ListAlertsParams } from './types'

/**
 * List all alerts with optional pagination
 */
export async function listAlerts(params?: ListAlertsParams): Promise<PaginatedResponse<Alert>> {
  return apiClient.get<PaginatedResponse<Alert>>('/api/v1/alerts', params)
}

/**
 * Get a single alert by ID
 */
export async function getAlert(id: string): Promise<Alert> {
  return apiClient.get<Alert>(`/api/v1/alerts/${id}`)
}

/**
 * Acknowledge an alert via the escalation engine.
 */
export async function acknowledgeAlert(
  id: string,
  userName: string,
  via?: string
): Promise<{ id: string; acknowledged_by: string; acknowledged_via: string; acknowledged_at: string }> {
  return apiClient.post(`/api/v1/alerts/${id}/acknowledge`, {
    user_name: userName,
    acknowledged_via: via,
  })
}
