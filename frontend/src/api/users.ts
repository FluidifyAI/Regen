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

export interface SlackMember {
  id: string
  name: string
  email: string
  avatar: string
  already_imported: boolean
}

export async function listSlackMembers(): Promise<{ members: SlackMember[] }> {
  const res = await fetch('/api/v1/settings/slack/members', { credentials: 'include' })
  if (!res.ok) throw new Error('Failed to load Slack members')
  return res.json()
}
