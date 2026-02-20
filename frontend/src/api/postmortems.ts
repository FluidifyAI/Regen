import { apiClient } from './client'
import type {
  PostMortemTemplate,
  PostMortem,
  ActionItem,
  CreatePostMortemTemplateRequest,
  UpdatePostMortemTemplateRequest,
  GeneratePostMortemRequest,
  UpdatePostMortemRequest,
  CreateActionItemRequest,
  UpdateActionItemRequest,
} from './types'

// Templates

export async function listPostMortemTemplates(): Promise<PostMortemTemplate[]> {
  const resp = await apiClient.get<{ data: PostMortemTemplate[] }>('/api/v1/post-mortem-templates')
  return resp.data
}

export async function getPostMortemTemplate(id: string): Promise<PostMortemTemplate> {
  return apiClient.get<PostMortemTemplate>(`/api/v1/post-mortem-templates/${id}`)
}

export async function createPostMortemTemplate(
  body: CreatePostMortemTemplateRequest
): Promise<PostMortemTemplate> {
  return apiClient.post<PostMortemTemplate>('/api/v1/post-mortem-templates', body)
}

export async function updatePostMortemTemplate(
  id: string,
  body: UpdatePostMortemTemplateRequest
): Promise<PostMortemTemplate> {
  return apiClient.patch<PostMortemTemplate>(`/api/v1/post-mortem-templates/${id}`, body)
}

export async function deletePostMortemTemplate(id: string): Promise<void> {
  return apiClient.delete<void>(`/api/v1/post-mortem-templates/${id}`)
}

// Post-Mortems

export async function getPostMortem(incidentId: string): Promise<PostMortem | null> {
  try {
    return await apiClient.get<PostMortem>(`/api/v1/incidents/${incidentId}/postmortem`)
  } catch (err) {
    if (err instanceof Error && 'response' in err) {
      const apiErr = (err as Error & { response: { error: { code: string } } }).response
      if (apiErr?.error?.code === 'not_found') return null
    }
    throw err
  }
}

export async function generatePostMortem(
  incidentId: string,
  body?: GeneratePostMortemRequest
): Promise<PostMortem> {
  return apiClient.post<PostMortem>(`/api/v1/incidents/${incidentId}/postmortem/generate`, body ?? {})
}

export async function updatePostMortem(
  incidentId: string,
  body: UpdatePostMortemRequest
): Promise<PostMortem> {
  return apiClient.patch<PostMortem>(`/api/v1/incidents/${incidentId}/postmortem`, body)
}

export function getPostMortemExportUrl(incidentId: string): string {
  return `/api/v1/incidents/${incidentId}/postmortem/export`
}

// Action Items

export async function createActionItem(
  incidentId: string,
  body: CreateActionItemRequest
): Promise<ActionItem> {
  return apiClient.post<ActionItem>(
    `/api/v1/incidents/${incidentId}/postmortem/action-items`,
    body
  )
}

export async function updateActionItem(
  incidentId: string,
  itemId: string,
  body: UpdateActionItemRequest
): Promise<ActionItem> {
  return apiClient.patch<ActionItem>(
    `/api/v1/incidents/${incidentId}/postmortem/action-items/${itemId}`,
    body
  )
}

export async function deleteActionItem(incidentId: string, itemId: string): Promise<void> {
  return apiClient.delete<void>(
    `/api/v1/incidents/${incidentId}/postmortem/action-items/${itemId}`
  )
}
