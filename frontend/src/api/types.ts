// Core domain types matching backend models exactly

export interface Incident {
  id: string
  incident_number: number
  title: string
  slug: string
  status: 'triggered' | 'acknowledged' | 'resolved' | 'canceled'
  severity: 'critical' | 'high' | 'medium' | 'low'
  summary: string
  slack_channel_id?: string
  slack_channel_name?: string
  group_key?: string  // SHA256 hash for grouped incidents
  created_at: string
  triggered_at: string
  acknowledged_at?: string
  resolved_at?: string
  created_by_type: string
  created_by_id?: string
  commander_id?: string
}

export interface Alert {
  id: string
  external_id: string
  source: string
  status: 'firing' | 'resolved'
  severity: 'critical' | 'warning' | 'info'
  title: string
  description: string
  labels: Record<string, string>
  annotations: Record<string, string>
  started_at: string
  ended_at?: string
  received_at: string
}

export interface TimelineEntry {
  id: string
  incident_id: string
  timestamp: string
  type: string
  actor_type: string
  actor_id?: string
  content: Record<string, unknown>
}

// API response types

export interface PaginatedResponse<T> {
  data: T[]
  total: number
  limit: number
  offset: number
}

export interface IncidentDetailResponse extends Incident {
  alerts: Alert[]
  timeline: TimelineEntry[]
}

export interface ApiError {
  error: {
    code: string
    message: string
    details?: Record<string, unknown>
    request_id: string
  }
}

// Request payload types

export interface CreateIncidentRequest {
  title: string
  severity?: 'critical' | 'high' | 'medium' | 'low'
  description?: string
}

export interface UpdateIncidentRequest {
  status?: 'triggered' | 'acknowledged' | 'resolved' | 'canceled'
  severity?: 'critical' | 'high' | 'medium' | 'low'
  summary?: string
}

export interface CreateTimelineEntryRequest {
  type: 'message'
  content: Record<string, unknown>
}

// Query parameter types

export interface ListIncidentsParams {
  status?: string
  severity?: string
  limit?: number
  page?: number
  offset?: number
}

export interface ListTimelineParams {
  limit?: number
  page?: number
}

export interface ListAlertsParams {
  limit?: number
  page?: number
}

// Routing Rules

export interface RoutingRule {
  id: string
  name: string
  description: string
  enabled: boolean
  priority: number
  match_criteria: Record<string, unknown>
  actions: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CreateRoutingRuleRequest {
  name: string
  description?: string
  enabled?: boolean
  priority: number
  match_criteria: Record<string, unknown>
  actions: Record<string, unknown>
}

export interface UpdateRoutingRuleRequest {
  name?: string
  description?: string
  enabled?: boolean
  priority?: number
  match_criteria?: Record<string, unknown>
  actions?: Record<string, unknown>
}

export interface ListRoutingRulesParams {
  enabled?: 'true' | 'false'
}

// Type guards and utilities

export function isApiError(error: unknown): error is ApiError {
  return (
    typeof error === 'object' &&
    error !== null &&
    'error' in error &&
    typeof (error as ApiError).error === 'object' &&
    'code' in (error as ApiError).error &&
    'message' in (error as ApiError).error
  )
}
