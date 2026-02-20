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
  // Teams integration (v0.8+)
  teams_channel_id?: string
  teams_channel_name?: string
  group_key?: string  // SHA256 hash for grouped incidents
  created_at: string
  triggered_at: string
  acknowledged_at?: string
  resolved_at?: string
  created_by_type: string
  created_by_id?: string
  commander_id?: string

  // AI Summarization (v0.6+)
  ai_summary?: string
  ai_summary_generated_at?: string
}

// AI response types (v0.6+)
export interface AISummaryResponse {
  incident_id: string
  summary: string
  generated_at: string
  model: string
  context_sources: string[]
}

export interface HandoffDigestResponse {
  incident_id: string
  incident_title: string
  status: string
  severity: string
  digest: string
  generated_at: string
}

export interface AISettingsResponse {
  enabled: boolean
}

export interface TeamsSettingsResponse {
  enabled: boolean
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
  // Escalation fields (v0.5)
  escalation_policy_id?: string
  acknowledgment_status?: 'pending' | 'acknowledged' | 'completed'
  acknowledged_by?: string
  acknowledged_at?: string
  acknowledged_via?: string
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

// Schedules (v0.4)

export interface Schedule {
  id: string
  name: string
  description: string
  timezone: string
  notification_channel: string
  created_at: string
  updated_at: string
  layers?: ScheduleLayer[]
}

export interface ScheduleLayer {
  id: string
  schedule_id: string
  name: string
  order_index: number
  rotation_type: 'daily' | 'weekly' | 'custom'
  rotation_start: string
  shift_duration_seconds: number
  created_at: string
  participants?: ScheduleParticipant[]
}

export interface ScheduleParticipant {
  id: string
  layer_id: string
  user_name: string
  order_index: number
  created_at: string
}

export interface ScheduleOverride {
  id: string
  schedule_id: string
  override_user: string
  start_time: string
  end_time: string
  created_by: string
  created_at: string
}

export interface OnCallResponse {
  schedule_id: string
  at: string
  user_name: string
  is_override: boolean
}

export interface TimelineSegment {
  start: string
  end: string
  user_name: string
  is_override: boolean
}

export interface CreateScheduleRequest {
  name: string
  description?: string
  timezone?: string
  notification_channel?: string
}

export interface UpdateScheduleRequest {
  name?: string
  description?: string
  timezone?: string
  notification_channel?: string
}

export interface CreateLayerRequest {
  name: string
  order_index?: number
  rotation_type: 'daily' | 'weekly' | 'custom'
  rotation_start?: string
  shift_duration_seconds?: number
  participants?: Array<{ user_name: string; order_index: number }>
}

export interface CreateOverrideRequest {
  override_user: string
  start_time: string
  end_time: string
  created_by?: string
}

// Escalation Policies (v0.5)

export type EscalationTargetType = 'schedule' | 'users' | 'both'

export interface EscalationTier {
  id: string
  policy_id: string
  tier_index: number
  timeout_seconds: number
  target_type: EscalationTargetType
  schedule_id?: string
  user_names: string[]
  created_at: string
}

export interface EscalationPolicy {
  id: string
  name: string
  description: string
  enabled: boolean
  tiers: EscalationTier[]
  created_at: string
  updated_at: string
}

export interface CreateEscalationPolicyRequest {
  name: string
  description?: string
  enabled?: boolean
}

export interface UpdateEscalationPolicyRequest {
  name?: string
  description?: string
  enabled?: boolean
}

export interface CreateEscalationTierRequest {
  timeout_seconds: number
  target_type: EscalationTargetType
  schedule_id?: string
  user_names?: string[]
}

export interface UpdateEscalationTierRequest {
  timeout_seconds?: number
  target_type?: EscalationTargetType
  schedule_id?: string
  user_names?: string[]
}

export interface ListEscalationPoliciesResponse {
  data: EscalationPolicy[]
  total: number
}

// Post-Mortem Templates (v0.7+)

export interface PostMortemTemplate {
  id: string
  name: string
  description: string
  sections: string[]
  is_built_in: boolean
  created_at: string
  updated_at: string
}

export interface CreatePostMortemTemplateRequest {
  name: string
  description?: string
  sections: string[]
}

export interface UpdatePostMortemTemplateRequest {
  name?: string
  description?: string
  sections?: string[]
}

// Post-Mortems (v0.7+)

export type PostMortemStatus = 'draft' | 'published'
export type ActionItemStatus = 'open' | 'in_progress' | 'closed'

export interface ActionItem {
  id: string
  post_mortem_id: string
  title: string
  owner?: string
  due_date?: string
  status: ActionItemStatus
  created_at: string
  updated_at: string
}

export interface PostMortem {
  id: string
  incident_id: string
  template_id?: string
  template_name: string
  status: PostMortemStatus
  content: string
  generated_by: string
  generated_at?: string
  published_at?: string
  created_by_id: string
  created_at: string
  updated_at: string
  action_items: ActionItem[]
}

export interface GeneratePostMortemRequest {
  template_id?: string
}

export interface UpdatePostMortemRequest {
  content?: string
  status?: PostMortemStatus
}

export interface CreateActionItemRequest {
  title: string
  owner?: string
  due_date?: string
}

export interface UpdateActionItemRequest {
  title?: string
  owner?: string
  due_date?: string
  status?: ActionItemStatus
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
