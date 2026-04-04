import { apiClient } from './client'

export interface UserRecord {
  id: string
  email: string
  name: string
  role: 'admin' | 'member' | 'viewer'
  auth_source: 'saml' | 'local' | 'ai' | 'deactivated'
  last_login_at?: string
  created_at: string
  slack_user_id?: string
  teams_user_id?: string
}

export interface CreateUserPayload {
  email: string
  name: string
  role: string
  password: string
  slackUserId?: string
  teamsUserId?: string
}

export async function listUsers(): Promise<UserRecord[]> {
  const res = await apiClient.get<{ users: UserRecord[] }>('/api/v1/settings/users')
  return res.users
}

export async function createUser(payload: CreateUserPayload): Promise<{ user: UserRecord; setup_token: string }> {
  return apiClient.post('/api/v1/settings/users', {
    email: payload.email,
    name: payload.name,
    role: payload.role,
    password: payload.password,
    slack_user_id: payload.slackUserId ?? '',
    teams_user_id: payload.teamsUserId ?? '',
  })
}

export async function updateUser(id: string, payload: { name?: string; role?: string; password?: string; slackUserId?: string; teamsUserId?: string }): Promise<void> {
  await apiClient.patch(`/api/v1/settings/users/${id}`, {
    ...(payload.name !== undefined ? { name: payload.name } : {}),
    ...(payload.role !== undefined ? { role: payload.role } : {}),
    ...(payload.password !== undefined ? { password: payload.password } : {}),
    ...(payload.slackUserId !== undefined ? { slack_user_id: payload.slackUserId } : {}),
    ...(payload.teamsUserId !== undefined ? { teams_user_id: payload.teamsUserId } : {}),
  })
}

export async function deactivateUser(id: string): Promise<void> {
  await apiClient.delete(`/api/v1/settings/users/${id}`)
}

export async function resetUserPassword(id: string): Promise<{ setup_token: string }> {
  return apiClient.post(`/api/v1/settings/users/${id}/reset-password`, {})
}

// ── System Settings ───────────────────────────────────────────────────────────

export interface SystemSettings {
  instance_name: string
  timezone: string
  ai_key_configured: boolean
  ai_key_last4: string
}

export async function getSystemSettings(): Promise<SystemSettings> {
  return apiClient.get<SystemSettings>('/api/v1/settings/system')
}

export async function updateSystemSettings(payload: {
  instance_name?: string
  timezone?: string
  openai_api_key?: string
}): Promise<void> {
  await apiClient.patch('/api/v1/settings/system', payload)
}

export async function testOpenAIKey(apiKey: string): Promise<{ ok: boolean; error?: string }> {
  return apiClient.post<{ ok: boolean; error?: string }>('/api/v1/settings/system/ai/test', { api_key: apiKey })
}
