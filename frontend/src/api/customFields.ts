import { apiClient } from './client'

export type FieldType = 'string' | 'number' | 'dropdown'

export interface DropdownOption {
  label: string
  value: string
}

export interface CustomFieldDefinition {
  id: string
  name: string
  key: string
  field_type: FieldType
  options: DropdownOption[]
  display_order: number
  created_at: string
  updated_at: string
}

export interface CreateCustomFieldPayload {
  name: string
  key: string
  field_type: FieldType
  options?: DropdownOption[]
  display_order?: number
}

export async function listCustomFields(): Promise<CustomFieldDefinition[]> {
  return apiClient.get<CustomFieldDefinition[]>('/api/v1/custom-fields')
}

export async function createCustomField(payload: CreateCustomFieldPayload): Promise<CustomFieldDefinition> {
  return apiClient.post<CustomFieldDefinition>('/api/v1/custom-fields', payload)
}

export async function updateCustomField(
  id: string,
  payload: CreateCustomFieldPayload,
): Promise<CustomFieldDefinition> {
  return apiClient.put<CustomFieldDefinition>(`/api/v1/custom-fields/${id}`, payload)
}

export async function deleteCustomField(id: string): Promise<void> {
  return apiClient.delete(`/api/v1/custom-fields/${id}`)
}

export async function reorderCustomFields(items: { id: string; order: number }[]): Promise<void> {
  return apiClient.patch('/api/v1/custom-fields/reorder', items)
}
