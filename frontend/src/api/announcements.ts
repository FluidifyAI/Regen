import { apiClient } from './client'

export interface Announcement {
  id: string
  type: 'release' | 'pro_upsell' | 'info'
  title: string
  body: string
  cta_url?: string
  cta_label?: string
  expires_at?: string
}

export async function getAnnouncements(): Promise<Announcement[]> {
  const res = await apiClient.get<{ announcements: Announcement[] }>('/api/v1/announcements')
  return res.announcements ?? []
}
