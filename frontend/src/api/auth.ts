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

export async function updateMe(req: { name?: string; current_password?: string; new_password?: string }): Promise<void> {
  await apiClient.patch<unknown>('/api/v1/auth/me', req)
}

export async function forgotPassword(email: string): Promise<{ setup_token?: string }> {
  return apiClient.post<{ ok: boolean; setup_token?: string }>('/api/v1/auth/forgot-password', { email })
}
