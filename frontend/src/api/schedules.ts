import { apiClient } from './client'
import type {
  Schedule,
  ScheduleOverride,
  OnCallResponse,
  TimelineSegment,
  CreateScheduleRequest,
  UpdateScheduleRequest,
  CreateLayerRequest,
  CreateOverrideRequest,
} from './types'

// Top 30 most-used IANA timezone names + UTC, alphabetical
export const COMMON_TIMEZONES = [
  'Africa/Cairo',
  'America/Chicago',
  'America/Denver',
  'America/Los_Angeles',
  'America/New_York',
  'America/Sao_Paulo',
  'Asia/Bangkok',
  'Asia/Dubai',
  'Asia/Hong_Kong',
  'Asia/Jakarta',
  'Asia/Karachi',
  'Asia/Kolkata',
  'Asia/Seoul',
  'Asia/Shanghai',
  'Asia/Singapore',
  'Asia/Tokyo',
  'Australia/Melbourne',
  'Australia/Sydney',
  'Europe/Amsterdam',
  'Europe/Berlin',
  'Europe/Istanbul',
  'Europe/London',
  'Europe/Madrid',
  'Europe/Moscow',
  'Europe/Paris',
  'Europe/Rome',
  'Pacific/Auckland',
  'Pacific/Honolulu',
  'US/Eastern',
  'UTC',
]

export async function listSchedules(): Promise<{ data: Schedule[]; total: number }> {
  return apiClient.get<{ data: Schedule[]; total: number }>('/api/v1/schedules')
}

export async function getSchedule(id: string): Promise<Schedule> {
  return apiClient.get<Schedule>(`/api/v1/schedules/${id}`)
}

export async function createSchedule(body: CreateScheduleRequest): Promise<Schedule> {
  return apiClient.post<Schedule>('/api/v1/schedules', body)
}

export async function updateSchedule(id: string, body: UpdateScheduleRequest): Promise<Schedule> {
  return apiClient.patch<Schedule>(`/api/v1/schedules/${id}`, body)
}

export async function deleteSchedule(id: string): Promise<void> {
  return apiClient.delete<void>(`/api/v1/schedules/${id}`)
}

export async function createLayer(scheduleId: string, body: CreateLayerRequest): Promise<void> {
  return apiClient.post<void>(`/api/v1/schedules/${scheduleId}/layers`, body)
}

export async function deleteLayer(scheduleId: string, layerId: string): Promise<void> {
  return apiClient.delete<void>(`/api/v1/schedules/${scheduleId}/layers/${layerId}`)
}

export async function getOnCall(scheduleId: string): Promise<OnCallResponse> {
  return apiClient.get<OnCallResponse>(`/api/v1/schedules/${scheduleId}/oncall`)
}

export async function getTimeline(
  scheduleId: string,
  from: string,
  to: string,
): Promise<{ schedule_id: string; from: string; to: string; segments: TimelineSegment[] }> {
  return apiClient.get(`/api/v1/schedules/${scheduleId}/oncall/timeline`, { from, to })
}

export async function listOverrides(
  scheduleId: string,
): Promise<{ data: ScheduleOverride[]; total: number }> {
  return apiClient.get<{ data: ScheduleOverride[]; total: number }>(
    `/api/v1/schedules/${scheduleId}/overrides`,
  )
}

export async function createOverride(
  scheduleId: string,
  body: CreateOverrideRequest,
): Promise<ScheduleOverride> {
  return apiClient.post<ScheduleOverride>(`/api/v1/schedules/${scheduleId}/overrides`, body)
}

export async function deleteOverride(scheduleId: string, overrideId: string): Promise<void> {
  return apiClient.delete<void>(`/api/v1/schedules/${scheduleId}/overrides/${overrideId}`)
}
