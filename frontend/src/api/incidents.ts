import { apiClient } from './client'
import type {
  Incident,
  IncidentDetailResponse,
  PaginatedResponse,
  CreateIncidentRequest,
  UpdateIncidentRequest,
  ListIncidentsParams,
} from './types'

/**
 * List incidents with optional filtering and pagination
 */
export async function listIncidents(
  params?: ListIncidentsParams
): Promise<PaginatedResponse<Incident>> {
  return apiClient.get<PaginatedResponse<Incident>>('/api/v1/incidents', params)
}

/**
 * Get a single incident by ID or incident number
 * Returns full details including linked alerts and timeline
 */
export async function getIncident(id: string | number): Promise<IncidentDetailResponse> {
  return apiClient.get<IncidentDetailResponse>(`/api/v1/incidents/${id}`)
}

/**
 * Create a new incident manually
 */
export async function createIncident(body: CreateIncidentRequest): Promise<Incident> {
  return apiClient.post<Incident>('/api/v1/incidents', body)
}

/**
 * Update an incident (status, severity, or summary)
 */
export async function updateIncident(
  id: string,
  body: UpdateIncidentRequest
): Promise<Incident> {
  return apiClient.patch<Incident>(`/api/v1/incidents/${id}`, body)
}
