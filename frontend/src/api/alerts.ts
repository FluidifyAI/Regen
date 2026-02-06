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
