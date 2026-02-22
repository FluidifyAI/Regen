import { apiClient } from './client'

export interface CurrentUser {
  authenticated: boolean
  mode?: 'open'
  message?: string
  email?: string
  name?: string
}

export async function getCurrentUser(): Promise<CurrentUser> {
  return apiClient.get<CurrentUser>('/api/v1/auth/me')
}
