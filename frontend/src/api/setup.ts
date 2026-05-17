import { apiClient } from './client'

export interface SetupStatus {
  demo_data_available: boolean
  slack_connected: boolean
  has_schedule: boolean
}

export async function getSetupStatus(): Promise<SetupStatus> {
  return apiClient.get<SetupStatus>('/api/v1/setup/status')
}
