import { apiClient } from './client'

export interface UserRecord {
  id: string
  email: string
  name: string
  role: 'admin' | 'member' | 'viewer'
  auth_source: 'saml' | 'local' | 'deactivated'
  last_login_at?: string
  created_at: string
}

export interface CreateUserPayload {
  email: string
  name: string
  role: string
  password: string
}

export async function listUsers(): Promise<UserRecord[]> {
  const res = await apiClient.get<{ users: UserRecord[] }>('/api/v1/settings/users')
  return res.users
}

export async function createUser(payload: CreateUserPayload): Promise<{ user: UserRecord; setup_token: string }> {
  return apiClient.post('/api/v1/settings/users', payload)
}

export async function updateUser(id: string, payload: { name?: string; role?: string; password?: string }): Promise<void> {
  await apiClient.patch(`/api/v1/settings/users/${id}`, payload)
}

export async function deactivateUser(id: string): Promise<void> {
  await apiClient.delete(`/api/v1/settings/users/${id}`)
}

export async function resetUserPassword(id: string): Promise<{ setup_token: string }> {
  return apiClient.post(`/api/v1/settings/users/${id}/reset-password`, {})
}
