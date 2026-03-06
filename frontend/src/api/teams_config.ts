import { apiClient } from './client'

export interface TeamsConfigStatus {
  configured: boolean
  team_id?: string
  team_name?: string
  tenant_id?: string
  app_id?: string
  service_url?: string
  has_password: boolean
  connected_at?: string
}

export interface SaveTeamsConfigRequest {
  app_id: string
  app_password: string
  tenant_id: string
  team_id: string
  bot_user_id?: string
  service_url?: string
  team_name?: string
}

export interface TeamsTestResult {
  team_id: string
  team_name: string
}

export async function getTeamsConfig(): Promise<TeamsConfigStatus> {
  return apiClient.get<TeamsConfigStatus>('/api/v1/settings/teams/config')
}

export async function testTeamsConfig(req: {
  app_id: string
  app_password: string
  tenant_id: string
  team_id: string
}): Promise<TeamsTestResult> {
  return apiClient.post<TeamsTestResult>('/api/v1/settings/teams/config/test', req)
}

export async function saveTeamsConfig(req: SaveTeamsConfigRequest): Promise<TeamsConfigStatus> {
  return apiClient.put<TeamsConfigStatus>('/api/v1/settings/teams/config', req)
}

export async function deleteTeamsConfig(): Promise<void> {
  return apiClient.delete<void>('/api/v1/settings/teams/config')
}
