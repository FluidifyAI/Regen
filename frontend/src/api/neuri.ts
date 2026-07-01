import { apiClient } from './client'
import type { NeuriSettingsResponse, NeuriSettingsRequest, NeuriResultListResponse, NeuriTriggerResponse } from './types'

export async function getNeuriSettings(): Promise<NeuriSettingsResponse> {
  return apiClient.get<NeuriSettingsResponse>('/api/v1/settings/neuri')
}

export async function updateNeuriSettings(req: NeuriSettingsRequest): Promise<void> {
  return apiClient.patch('/api/v1/settings/neuri', req)
}

export async function triggerNeuriInvestigation(incidentId: string): Promise<NeuriTriggerResponse> {
  return apiClient.post<NeuriTriggerResponse>('/api/v1/neuri/investigate', { incident_id: incidentId })
}

export async function getNeuriResults(incidentId: string): Promise<NeuriResultListResponse> {
  return apiClient.get<NeuriResultListResponse>(`/api/v1/neuri/result?incident_id=${incidentId}`)
}
