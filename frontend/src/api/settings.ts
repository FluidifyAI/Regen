import { apiClient } from './client'

export interface UserRecord {
  id: string
  email: string
  name: string
  role: 'admin' | 'member' | 'viewer'
  auth_source: 'saml' | 'local' | 'deactivated'
  last_login_at?: string
  created_at: string
  slack_user_id?: string
}

export interface CreateUserPayload {
  email: string
  name: string
  role: string
  password: string
  slackUserId?: string
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
  })
}

export async function updateUser(id: string, payload: { name?: string; role?: string; password?: string; slackUserId?: string }): Promise<void> {
  await apiClient.patch(`/api/v1/settings/users/${id}`, {
    ...(payload.name !== undefined ? { name: payload.name } : {}),
    ...(payload.role !== undefined ? { role: payload.role } : {}),
    ...(payload.password !== undefined ? { password: payload.password } : {}),
    ...(payload.slackUserId !== undefined ? { slack_user_id: payload.slackUserId } : {}),
  })
}

export async function deactivateUser(id: string): Promise<void> {
  await apiClient.delete(`/api/v1/settings/users/${id}`)
}

export async function resetUserPassword(id: string): Promise<{ setup_token: string }> {
  return apiClient.post(`/api/v1/settings/users/${id}/reset-password`, {})
}
