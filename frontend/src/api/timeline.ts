import { apiClient } from './client'
import type {
  TimelineEntry,
  PaginatedResponse,
  CreateTimelineEntryRequest,
  ListTimelineParams,
} from './types'

/**
 * Get timeline entries for an incident
 * Ordered chronologically (oldest first)
 */
export async function getTimeline(
  incidentId: string,
  params?: ListTimelineParams
): Promise<PaginatedResponse<TimelineEntry>> {
  return apiClient.get<PaginatedResponse<TimelineEntry>>(
    `/api/v1/incidents/${incidentId}/timeline`,
    params
  )
}

/**
 * Add a manual timeline entry (user note/message)
 */
export async function addTimelineEntry(
  incidentId: string,
  body: CreateTimelineEntryRequest
): Promise<TimelineEntry> {
  return apiClient.post<TimelineEntry>(`/api/v1/incidents/${incidentId}/timeline`, body)
}
