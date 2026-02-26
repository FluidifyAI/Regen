import { apiClient } from './client'

export interface CurrentUser {
  authenticated: boolean
  mode?: 'open'
  message?: string
  email?: string
  name?: string
  role?: 'admin' | 'member' | 'viewer'
  ssoEnabled?: boolean
}

export interface LoginRequest {
  email: string
  password: string
}

export async function getCurrentUser(): Promise<CurrentUser> {
  return apiClient.get<CurrentUser>('/api/v1/auth/me')
}

export async function login(req: LoginRequest): Promise<void> {
  return apiClient.post<void>('/api/v1/auth/login', req)
}
