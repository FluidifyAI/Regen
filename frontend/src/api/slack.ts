import { apiClient } from './client'

export interface SlackConfigStatus {
  configured: boolean
  workspace_id?: string
  workspace_name?: string
  bot_user_id?: string
  has_bot_token: boolean
  has_app_token: boolean
  has_oauth_config: boolean
  connected_at?: string
}

export interface SlackTestResult {
  workspace_id: string
  workspace_name: string
  bot_user_id: string
  bot_username: string
}

export interface SaveSlackConfigRequest {
  bot_token: string
  signing_secret: string
  app_token?: string
  workspace_id?: string
  workspace_name?: string
  bot_user_id?: string
  oauth_client_id?: string
  oauth_client_secret?: string
}

export async function getSlackConfig(): Promise<SlackConfigStatus> {
  return apiClient.get<SlackConfigStatus>('/api/v1/settings/slack')
}

export async function testSlackToken(botToken: string): Promise<SlackTestResult> {
  return apiClient.post<SlackTestResult>('/api/v1/settings/slack/test', { bot_token: botToken })
}

export async function saveSlackConfig(req: SaveSlackConfigRequest): Promise<SlackConfigStatus> {
  return apiClient.post<SlackConfigStatus>('/api/v1/settings/slack', req)
}

export async function deleteSlackConfig(): Promise<void> {
  return apiClient.delete<void>('/api/v1/settings/slack')
}

export async function getSlackOAuthStatus(): Promise<{ enabled: boolean; client_id?: string }> {
  return apiClient.get('/api/v1/auth/slack/config')
}
