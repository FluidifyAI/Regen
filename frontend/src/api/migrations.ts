import { apiClient } from './client'

// ── Request ───────────────────────────────────────────────────────────────────

export interface OnCallMigrationRequest {
  oncall_url: string
  api_token: string
}

// ── Preview types ─────────────────────────────────────────────────────────────

export interface ConflictItem {
  type: string
  name: string
  reason: string
}

export interface SkippedItem {
  type: string
  name: string
  reason: string
}

export interface WebhookMapping {
  name: string
  oncall_type: string
  old_url: string
  new_url: string
  regen_source: string
}

export interface PreviewUser {
  id: string
  email: string
  name: string
  role: 'admin' | 'member' | 'viewer'
}

export interface PreviewSchedule {
  id: string
  name: string
  timezone: string
}

export interface PreviewEscalationPolicy {
  id: string
  name: string
  description: string
}

interface EntitySummary<T> {
  count: number
  items: T[]
}

export interface OnCallPreviewResponse {
  users: EntitySummary<PreviewUser>
  schedules: EntitySummary<PreviewSchedule>
  escalation_policies: EntitySummary<PreviewEscalationPolicy>
  webhooks: EntitySummary<WebhookMapping>
  conflicts: ConflictItem[]
  skipped: SkippedItem[]
}

// ── Import result ─────────────────────────────────────────────────────────────

export interface UserSetupToken {
  email: string
  setup_token: string
}

export interface OnCallImportResponse {
  imported: {
    users: number
    schedules: number
    escalation_policies: number
  }
  skipped: SkippedItem[]
  conflicts: ConflictItem[]
  webhooks: WebhookMapping[]
  setup_tokens: UserSetupToken[]
}

// ── API functions ─────────────────────────────────────────────────────────────

export async function previewOnCallMigration(
  req: OnCallMigrationRequest,
): Promise<OnCallPreviewResponse> {
  return apiClient.post<OnCallPreviewResponse>('/api/v1/migrations/oncall/preview', req)
}

export async function importOnCallMigration(
  req: OnCallMigrationRequest,
): Promise<OnCallImportResponse> {
  return apiClient.post<OnCallImportResponse>('/api/v1/migrations/oncall/import', req)
}
