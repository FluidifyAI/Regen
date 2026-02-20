import { apiClient } from './client'
import type { AISummaryResponse, HandoffDigestResponse, AISettingsResponse } from './types'

/**
 * Generate (or regenerate) an AI summary for an incident.
 * Persists the summary on the incident and returns it.
 * Returns 503 if OPENAI_API_KEY is not configured on the server.
 */
export async function summarizeIncident(incidentId: string): Promise<AISummaryResponse> {
  return apiClient.post<AISummaryResponse>(`/api/v1/incidents/${incidentId}/summarize`)
}

/**
 * Generate a shift handoff digest for an incident.
 * The digest is not persisted — use it for display or posting to Slack.
 */
export async function generateHandoffDigest(incidentId: string): Promise<HandoffDigestResponse> {
  return apiClient.post<HandoffDigestResponse>(`/api/v1/incidents/${incidentId}/handoff-digest`)
}

/**
 * Check whether AI features are enabled on the server.
 * Use this to conditionally show AI controls in the UI.
 */
export async function getAISettings(): Promise<AISettingsResponse> {
  return apiClient.get<AISettingsResponse>('/api/v1/settings/ai')
}
