import { useState, useEffect, useCallback, useRef } from 'react'
import { Paperclip, Upload, Trash2, Download, AlertCircle, FileText } from 'lucide-react'
import {
  listAttachments,
  uploadAttachment,
  deleteAttachment,
  formatFileSize,
  isImageMime,
} from '../../api/attachments'
import type { Attachment } from '../../api/types'

interface AttachmentsPanelProps {
  incidentId: string
  onCountChange?: (count: number) => void
}

interface UploadingFile {
  name: string
  progress: number
  error?: string
}

export function AttachmentsPanel({ incidentId, onCountChange }: AttachmentsPanelProps) {
  const [attachments, setAttachments] = useState<Attachment[]>([])
  const [loading, setLoading] = useState(true)
  const [uploading, setUploading] = useState<UploadingFile[]>([])
  const [dragOver, setDragOver] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const load = useCallback(async () => {
    try {
      const data = await listAttachments(incidentId)
      setAttachments(data)
      onCountChange?.(data.length)
    } catch {
      // silently fail — panel shows empty state
    } finally {
      setLoading(false)
    }
  }, [incidentId, onCountChange])

  useEffect(() => { load() }, [load])

  // Clipboard paste — active only while this panel is mounted
  useEffect(() => {
    function handlePaste(e: ClipboardEvent) {
      const items = e.clipboardData?.items
      if (!items) return
      for (const item of Array.from(items)) {
        if (item.kind === 'file' && item.type.startsWith('image/')) {
          const file = item.getAsFile()
          if (!file) continue
          const ts = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19)
          const named = new File([file], `screenshot-${ts}.png`, { type: file.type })
          handleUpload(named)
        }
      }
    }
    document.addEventListener('paste', handlePaste)
    return () => document.removeEventListener('paste', handlePaste)
  }, [incidentId]) // eslint-disable-line react-hooks/exhaustive-deps

  async function handleUpload(file: File) {
    const MAX = 10 * 1024 * 1024
    if (file.size > MAX) {
      setUploading((u) => [...u, { name: file.name, progress: 0, error: 'File exceeds 10 MB limit' }])
      setTimeout(() => setUploading((u) => u.filter((f) => f.name !== file.name)), 4000)
      return
    }

    setUploading((u) => [...u, { name: file.name, progress: 0 }])
    try {
      const att = await uploadAttachment(incidentId, file, (pct) => {
        setUploading((u) => u.map((f) => f.name === file.name ? { ...f, progress: pct } : f))
      })
      setAttachments((prev) => {
        const next = [...prev, att]
        onCountChange?.(next.length)
        return next
      })
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Upload failed'
      setUploading((u) => u.map((f) => f.name === file.name ? { ...f, error: msg } : f))
      setTimeout(() => setUploading((u) => u.filter((f) => f.name !== file.name)), 5000)
      return
    }
    setUploading((u) => u.filter((f) => f.name !== file.name))
  }

  function handleFiles(files: FileList | null) {
    if (!files) return
    Array.from(files).forEach(handleUpload)
  }

  async function handleDelete(att: Attachment) {
    if (!window.confirm(`Delete "${att.file_name}"?`)) return
    try {
      await deleteAttachment(incidentId, att.id)
      setAttachments((prev) => {
        const next = prev.filter((x) => x.id !== att.id)
        onCountChange?.(next.length)
        return next
      })
    } catch {
      // ignore — attachment stays in list
    }
  }

  if (loading) {
    return (
      <div className="space-y-3 animate-pulse">
        <div className="h-24 bg-surface-tertiary rounded-lg" />
        <div className="h-12 bg-surface-tertiary rounded-lg" />
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {/* Upload zone */}
      <div
        onDragOver={(e) => { e.preventDefault(); setDragOver(true) }}
        onDragLeave={() => setDragOver(false)}
        onDrop={(e) => { e.preventDefault(); setDragOver(false); handleFiles(e.dataTransfer.files) }}
        onClick={() => fileInputRef.current?.click()}
        className={`border-2 border-dashed rounded-lg p-6 text-center cursor-pointer transition-colors ${
          dragOver
            ? 'border-brand-primary bg-brand-primary/5'
            : 'border-border hover:border-brand-primary hover:bg-surface-secondary'
        }`}
      >
        <Upload className="w-6 h-6 text-text-tertiary mx-auto mb-2" />
        <p className="text-sm font-medium text-text-primary">Drop files or click to browse</p>
        <p className="text-xs text-text-tertiary mt-1">
          Or paste a screenshot with <kbd className="px-1 py-0.5 bg-surface-tertiary rounded text-[10px] font-mono">Ctrl+V</kbd>
        </p>
        <p className="text-xs text-text-tertiary">Images, PDF, text · max 10 MB</p>
        <input
          ref={fileInputRef}
          type="file"
          multiple
          className="hidden"
          accept="image/*,application/pdf,text/plain,text/csv,application/json,application/zip"
          onChange={(e) => handleFiles(e.target.files)}
        />
      </div>

      {/* Uploading progress rows */}
      {uploading.map((u) => (
        <div key={u.name} className="flex items-center gap-3 p-3 border border-border rounded-lg bg-surface-secondary">
          <Paperclip className="w-4 h-4 text-text-tertiary flex-shrink-0" />
          <div className="flex-1 min-w-0">
            <p className="text-sm text-text-primary truncate">{u.name}</p>
            {u.error ? (
              <p className="text-xs text-red-500 flex items-center gap-1">
                <AlertCircle className="w-3 h-3" /> {u.error}
              </p>
            ) : (
              <div className="mt-1 h-1.5 bg-surface-tertiary rounded-full overflow-hidden">
                <div
                  className="h-full bg-brand-primary transition-all"
                  style={{ width: `${u.progress}%` }}
                />
              </div>
            )}
          </div>
        </div>
      ))}

      {/* Empty state */}
      {attachments.length === 0 && uploading.length === 0 && (
        <div className="text-center py-8 text-text-tertiary">
          <Paperclip className="w-6 h-6 mx-auto mb-2 opacity-40" />
          <p className="text-sm">No attachments yet</p>
        </div>
      )}

      {/* Attachment list */}
      {attachments.map((att) => (
        <div key={att.id} className="flex items-start gap-3 p-3 border border-border rounded-lg hover:bg-surface-secondary transition-colors group">
          {isImageMime(att.mime_type) ? (
            <img
              src={att.download_url}
              alt={att.file_name}
              className="w-12 h-12 object-cover rounded border border-border flex-shrink-0"
            />
          ) : (
            <div className="w-12 h-12 rounded border border-border bg-surface-tertiary flex items-center justify-center flex-shrink-0">
              <FileText className="w-5 h-5 text-text-tertiary" />
            </div>
          )}
          <div className="flex-1 min-w-0">
            <p className="text-sm font-medium text-text-primary truncate">{att.file_name}</p>
            <p className="text-xs text-text-tertiary">
              {formatFileSize(att.file_size)} · {att.uploaded_by} · {formatRelativeTime(att.created_at)}
            </p>
          </div>
          <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
            <a
              href={att.download_url}
              download={att.file_name}
              onClick={(e) => e.stopPropagation()}
              className="p-1.5 rounded hover:bg-surface-tertiary text-text-tertiary hover:text-text-primary transition-colors"
              title="Download"
            >
              <Download className="w-4 h-4" />
            </a>
            <button
              onClick={() => handleDelete(att)}
              className="p-1.5 rounded hover:bg-red-50 text-text-tertiary hover:text-red-600 transition-colors"
              title="Delete"
            >
              <Trash2 className="w-4 h-4" />
            </button>
          </div>
        </div>
      ))}
    </div>
  )
}

function formatRelativeTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 1) return 'just now'
  if (mins < 60) return `${mins}m ago`
  const hrs = Math.floor(mins / 60)
  if (hrs < 24) return `${hrs}h ago`
  return new Date(iso).toLocaleDateString()
}
