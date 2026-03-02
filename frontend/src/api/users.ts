import { apiClient } from './client'

export interface UserSummary {
  id: string
  name: string
  email: string
  role: string
}

export async function listUsers(): Promise<UserSummary[]> {
  const res = await apiClient.get<{ users: UserSummary[] }>('/api/v1/users')
  return res.users
}
