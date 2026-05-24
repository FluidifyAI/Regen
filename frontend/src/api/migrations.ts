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

// ── PagerDuty migration ───────────────────────────────────────────────────────

export interface PDMigrationRequest {
  api_key: string
  region?: 'us' | 'eu'
  force?: boolean
}

export interface PDPreviewSchedule {
  name: string
  timezone: string
  layer_count: number
  user_count: number
}

export interface PDPreviewPolicy {
  name: string
  tier_count: number
}

export interface PDPreviewResponse {
  schedules: PDPreviewSchedule[]
  policies: PDPreviewPolicy[]
  warnings: string[]
}

export interface PDReportSummary {
  schedules_found: number
  schedules_imported: number
  schedules_skipped: number
  layers_imported: number
  layers_skipped: number
  policies_found: number
  policies_imported: number
  policies_skipped: number
  tiers_imported: number
}

export interface PDImportResponse {
  imported_at: string
  summary: PDReportSummary
  warnings: string[]
  errors: string[]
}

export async function previewPagerDutyMigration(req: PDMigrationRequest): Promise<PDPreviewResponse> {
  return apiClient.post<PDPreviewResponse>('/api/v1/migrations/pagerduty/preview', req)
}

export async function importPagerDutyMigration(req: PDMigrationRequest): Promise<PDImportResponse> {
  return apiClient.post<PDImportResponse>('/api/v1/migrations/pagerduty/import', req)
}

// ── Opsgenie migration ────────────────────────────────────────────────────────

export interface OGMigrationRequest {
  api_key: string
  region: 'us' | 'eu'
  force?: boolean
}

export interface OGPreviewSchedule {
  name: string
  timezone: string
  rotation_count: number
  user_count: number
}

export interface OGPreviewPolicy {
  name: string
  rule_count: number
}

export interface OGPreviewResponse {
  schedules: OGPreviewSchedule[]
  policies: OGPreviewPolicy[]
  warnings: string[]
}

export interface OGReportSummary {
  schedules_found: number
  schedules_imported: number
  schedules_skipped: number
  layers_imported: number
  layers_skipped: number
  policies_found: number
  policies_imported: number
  policies_skipped: number
  tiers_imported: number
}

export interface OGImportResponse {
  imported_at: string
  summary: OGReportSummary
  warnings: string[]
  errors: string[]
}

export async function previewOpsgenieMigration(req: OGMigrationRequest): Promise<OGPreviewResponse> {
  return apiClient.post<OGPreviewResponse>('/api/v1/migrations/opsgenie/preview', req)
}

export async function importOpsgenieMigration(req: OGMigrationRequest): Promise<OGImportResponse> {
  return apiClient.post<OGImportResponse>('/api/v1/migrations/opsgenie/import', req)
}
