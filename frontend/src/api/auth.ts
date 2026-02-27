import { apiClient } from './client'

export interface CurrentUser {
  authenticated: boolean
  mode?: 'open'
  message?: string
  id?: string
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

export interface BootstrapRequest {
  name: string
  email: string
  password: string
}

export async function bootstrap(req: BootstrapRequest): Promise<void> {
  await apiClient.post<unknown>('/api/v1/auth/bootstrap', req)
}

export async function exchangeSetupToken(token: string): Promise<void> {
  return apiClient.post<void>('/api/v1/auth/login/setup-token', { token })
}

export async function logout(): Promise<void> {
  await apiClient.post<unknown>('/api/v1/auth/logout')
}
