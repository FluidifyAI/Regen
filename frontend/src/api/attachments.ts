import { apiClient } from './client'
import type { Attachment } from './types'

export async function listAttachments(incidentId: string): Promise<Attachment[]> {
  const resp = await apiClient.get<{ data: Attachment[] }>(`/api/v1/incidents/${incidentId}/attachments`)
  return resp.data
}

export async function uploadAttachment(
  incidentId: string,
  file: File,
  onProgress?: (pct: number) => void
): Promise<Attachment> {
  return new Promise((resolve, reject) => {
    const form = new FormData()
    form.append('file', file)

    const BASE_URL = import.meta.env.VITE_API_URL || ''
    const xhr = new XMLHttpRequest()
    xhr.open('POST', `${BASE_URL}/api/v1/incidents/${incidentId}/attachments`)
    xhr.withCredentials = true

    if (onProgress) {
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) onProgress(Math.round((e.loaded / e.total) * 100))
      }
    }

    xhr.onload = () => {
      if (xhr.status === 201) {
        resolve(JSON.parse(xhr.responseText) as Attachment)
      } else {
        try {
          const err = JSON.parse(xhr.responseText)
          reject(new Error(err?.error?.message ?? `Upload failed (${xhr.status})`))
        } catch {
          reject(new Error(`Upload failed (${xhr.status})`))
        }
      }
    }
    xhr.onerror = () => reject(new Error('Network error during upload'))
    xhr.send(form)
  })
}

export async function deleteAttachment(incidentId: string, attachmentId: string): Promise<void> {
  return apiClient.delete<void>(`/api/v1/incidents/${incidentId}/attachments/${attachmentId}`)
}

export function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

export function isImageMime(mimeType: string): boolean {
  return mimeType.startsWith('image/')
}

const ALLOWED_MIME_PREFIXES = ['image/', 'text/plain', 'text/csv', 'application/pdf', 'application/json', 'application/zip']

export function isAllowedMime(file: File): boolean {
  const mime = file.type
  // Empty type (common for .exe, .bin, unknown files) is not allowed
  if (!mime) return false
  return ALLOWED_MIME_PREFIXES.some((allowed) =>
    allowed.endsWith('/') ? mime.startsWith(allowed) : mime === allowed
  )
}
