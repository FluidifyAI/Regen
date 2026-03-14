import { useState, useEffect, useCallback, useRef } from 'react'
import {
  FileText,
  Sparkles,
  Download,
  Send,
  AlertCircle,
  ChevronDown,
  PenLine,
} from 'lucide-react'
import { Button } from '../ui/Button'
import {
  getPostMortem,
  generatePostMortem,
  updatePostMortem,
  getPostMortemExportUrl,
  listPostMortemTemplates,
  createPostMortem,
  enhancePostMortem,
} from '../../api/postmortems'
import { getAISettings } from '../../api/ai'
import { ActionItems } from './ActionItems'
import { CommentsThread } from './CommentsThread'
import type { PostMortem, PostMortemTemplate } from '../../api/types'

interface PostMortemPanelProps {
  incidentId: string
  onPostMortemLoaded?: (exists: boolean) => void
}

/**
 * PostMortemPanel manages the full post-mortem lifecycle for an incident:
 * - Fetch existing post-mortem (if any)
 * - Generate via AI with optional template selector
 * - Edit markdown content inline
 * - Publish (status → published)
 * - Export as .md download
 * - Action items CRUD
 */
export function PostMortemPanel({ incidentId, onPostMortemLoaded }: PostMortemPanelProps) {
  const [pm, setPm] = useState<PostMortem | null>(null)
  const [loading, setLoading] = useState(true)
  const [aiEnabled, setAiEnabled] = useState<boolean | null>(null)
  const [templates, setTemplates] = useState<PostMortemTemplate[]>([])
  const [selectedTemplateId, setSelectedTemplateId] = useState<string>('')
  const [generating, setGenerating] = useState(false)
  const [enhancing, setEnhancing] = useState(false)
  const [creating, setCreating] = useState(false)
  const [saving, setSaving] = useState(false)
  const [publishing, setPublishing] = useState(false)
  const [editedContent, setEditedContent] = useState('')
  const [isDirty, setIsDirty] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [editorMode, setEditorMode] = useState<'write' | 'preview'>('preview')

  const fetchPm = useCallback(async () => {
    try {
      const data = await getPostMortem(incidentId)
      setPm(data)
      onPostMortemLoaded?.(!!data)
      if (data) {
        setEditedContent(data.content)
        setIsDirty(false)
      }
    } catch {
      setPm(null)
      onPostMortemLoaded?.(false)
    }
  }, [incidentId, onPostMortemLoaded])

  useEffect(() => {
    setLoading(true)
    Promise.all([
      fetchPm(),
      getAISettings().then((s) => setAiEnabled(s.enabled)).catch(() => setAiEnabled(false)),
      listPostMortemTemplates().then(setTemplates).catch(() => setTemplates([])),
    ]).finally(() => setLoading(false))
  }, [fetchPm])

  async function handleGenerate() {
    setGenerating(true)
    setError(null)
    try {
      const data = await generatePostMortem(
        incidentId,
        selectedTemplateId ? { template_id: selectedTemplateId } : undefined
      )
      setPm(data)
      setEditedContent(data.content)
      setIsDirty(false)
      setEditorMode('preview')
      onPostMortemLoaded?.(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to generate post-mortem')
    } finally {
      setGenerating(false)
    }
  }

  async function handleCreate() {
    setCreating(true)
    setError(null)
    try {
      const data = await createPostMortem(incidentId)
      setPm(data)
      setEditedContent(data.content)
      setIsDirty(false)
      setEditorMode('write')
      onPostMortemLoaded?.(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create post-mortem')
    } finally {
      setCreating(false)
    }
  }

  async function handleEnhance() {
    if (!pm) return
    setEnhancing(true)
    setError(null)
    try {
      const data = await enhancePostMortem(incidentId, editedContent)
      setPm(data)
      setEditedContent(data.content)
      setIsDirty(true)
      setEditorMode('preview')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to enhance post-mortem')
    } finally {
      setEnhancing(false)
    }
  }

  async function handleSave(statusOverride?: 'draft' | 'published'): Promise<boolean> {
    if (!pm) return false
    setSaving(true)
    setError(null)
    try {
      const updated = await updatePostMortem(incidentId, {
        content: editedContent,
        ...(statusOverride ? { status: statusOverride } : {}),
      })
      setPm(updated)
      setIsDirty(false)
      return true
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save')
      return false
    } finally {
      setSaving(false)
    }
  }

  async function handlePublish() {
    if (!pm) return
    setPublishing(true)
    setError(null)
    try {
      await handleSave('published')
    } finally {
      setPublishing(false)
    }
  }

  if (loading) {
    return (
      <div className="space-y-3 animate-pulse">
        <div className="h-8 w-48 bg-surface-tertiary rounded" />
        <div className="h-64 bg-surface-tertiary rounded" />
      </div>
    )
  }

  const isPublished = pm?.status === 'published'

  return (
    <div className="space-y-4">
      {/* Error banner */}
      {error && (
        <div className="flex items-start gap-2 text-sm text-red-600 bg-red-50 border border-red-200 rounded-lg p-3">
          <AlertCircle className="w-4 h-4 mt-0.5 flex-shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {/* Header row */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <FileText className="w-4 h-4 text-text-tertiary" />
          <span className="text-sm font-medium text-text-primary">Post-Mortem</span>
          {pm && (
            <span
              className={`text-xs px-2 py-0.5 rounded-full font-medium ${
                isPublished
                  ? 'bg-green-100 text-green-700'
                  : 'bg-amber-100 text-amber-700'
              }`}
            >
              {isPublished ? 'Published' : 'Draft'}
            </span>
          )}
        </div>

        <div className="flex items-center gap-2">
          {pm && (
            <>
              {/* Export button */}
              <a
                href={getPostMortemExportUrl(incidentId)}
                download
                className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium rounded bg-white border border-border text-text-secondary hover:bg-surface-secondary transition-colors"
              >
                <Download className="w-3.5 h-3.5" />
                Export
              </a>

              {/* Publish button (only when draft) */}
              {!isPublished && (
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={handlePublish}
                  loading={publishing}
                  disabled={publishing}
                >
                  {!publishing && <Send className="w-3.5 h-3.5" />}
                  Publish
                </Button>
              )}
            </>
          )}

          {/* AI dropdown (only when PM exists) */}
          {pm && aiEnabled && (
            <AIActionsDropdown
              generating={generating}
              enhancing={enhancing}
              templates={templates}
              selectedTemplateId={selectedTemplateId}
              onTemplateChange={setSelectedTemplateId}
              onGenerate={handleGenerate}
              onEnhance={handleEnhance}
            />
          )}
          {/* Generate button when no PM yet */}
          {!pm && aiEnabled && (
            <Button
              variant="secondary"
              size="sm"
              onClick={handleGenerate}
              loading={generating}
              disabled={generating || creating}
            >
              {!generating && <Sparkles className="w-3.5 h-3.5" />}
              Generate with AI
            </Button>
          )}
        </div>
      </div>

      {/* AI disabled notice when no post-mortem exists */}
      {!pm && aiEnabled === false && (
        <div className="flex items-start gap-2 text-sm text-text-tertiary bg-surface-secondary border border-border rounded-lg p-4">
          <AlertCircle className="w-4 h-4 mt-0.5 flex-shrink-0 text-amber-500" />
          <span>
            AI features are disabled. Set the{' '}
            <code className="font-mono text-xs bg-white px-1 py-0.5 rounded border border-border">
              OPENAI_API_KEY
            </code>{' '}
            environment variable to generate post-mortems automatically.
          </span>
        </div>
      )}

      {/* No post-mortem yet — write-first empty state */}
      {!pm && (
        <div className="border border-dashed border-border rounded-lg p-8 text-center space-y-4">
          <FileText className="w-8 h-8 text-text-tertiary mx-auto" />
          <div>
            <p className="text-sm font-medium text-text-primary mb-1">No post-mortem yet</p>
            <p className="text-xs text-text-tertiary">Write it yourself or let AI draft it from the incident timeline.</p>
          </div>
          <div className="flex items-center justify-center gap-3 flex-wrap">
            <Button
              variant="primary"
              size="sm"
              onClick={handleCreate}
              loading={creating}
              disabled={creating || generating}
            >
              {!creating && <PenLine className="w-3.5 h-3.5" />}
              Start writing
            </Button>
            {aiEnabled && (
              <Button
                variant="secondary"
                size="sm"
                onClick={handleGenerate}
                loading={generating}
                disabled={generating || creating}
              >
                {!generating && <Sparkles className="w-3.5 h-3.5" />}
                Generate with AI
              </Button>
            )}
          </div>
        </div>
      )}

      {/* Content editor */}
      {pm && (
        <div className="border border-border rounded-lg overflow-hidden">
          {/* Toolbar */}
          <div className="flex items-center gap-3 px-3 py-2 bg-surface-secondary border-b border-border">
            {/* Write / Preview toggle */}
            <div className="flex rounded border border-border overflow-hidden text-xs">
              <button
                onClick={() => setEditorMode('write')}
                className={`px-2.5 py-1 font-medium transition-colors ${
                  editorMode === 'write'
                    ? 'bg-white text-text-primary'
                    : 'text-text-tertiary hover:text-text-primary'
                }`}
              >
                Write
              </button>
              <button
                onClick={() => setEditorMode('preview')}
                className={`px-2.5 py-1 font-medium border-l border-border transition-colors ${
                  editorMode === 'preview'
                    ? 'bg-white text-text-primary'
                    : 'text-text-tertiary hover:text-text-primary'
                }`}
              >
                Preview
              </button>
            </div>
            {/* Meta info */}
            <span className="text-xs text-text-tertiary flex-1">
              {pm.template_name} · by {pm.generated_by === 'ai' ? 'AI' : 'Manual'}
              {pm.generated_at && ` · ${formatRelativeTime(pm.generated_at)}`}
            </span>
            {isDirty && (
              <Button
                variant="primary"
                size="sm"
                onClick={() => handleSave(isPublished ? 'draft' : undefined)}
                loading={saving}
              >
                {isPublished ? 'Save & revert to draft' : 'Save'}
              </Button>
            )}
          </div>
          {/* Write mode: editable textarea */}
          {editorMode === 'write' ? (
            <textarea
              value={editedContent}
              onChange={(e) => {
                setEditedContent(e.target.value)
                setIsDirty(true)
              }}
              className="w-full min-h-96 p-4 font-mono text-sm text-text-primary resize-y focus:outline-none bg-white"
              placeholder="Post-mortem content..."
            />
          ) : (
            /* Preview mode: render markdown as React elements */
            <div className="w-full min-h-96 p-4 bg-white">
              <MarkdownPreview content={editedContent} />
            </div>
          )}
        </div>
      )}

      {/* Action items */}
      {pm && (
        <ActionItems incidentId={incidentId} initialItems={pm.action_items} />
      )}

      {/* Discussion thread */}
      {pm && (
        <CommentsThread incidentId={incidentId} />
      )}
    </div>
  )
}

/**
 * AIActionsDropdown — "AI ▾" button that opens a dropdown with Regenerate and Enhance options.
 */
function AIActionsDropdown({
  generating,
  enhancing,
  templates,
  selectedTemplateId,
  onTemplateChange,
  onGenerate,
  onEnhance,
}: {
  generating: boolean
  enhancing: boolean
  templates: PostMortemTemplate[]
  selectedTemplateId: string
  onTemplateChange: (id: string) => void
  onGenerate: () => void
  onEnhance: () => void
}) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [])

  const busy = generating || enhancing

  return (
    <div className="relative" ref={ref}>
      <Button
        variant="secondary"
        size="sm"
        onClick={() => setOpen((o) => !o)}
        disabled={busy}
        loading={busy}
        className="gap-1"
      >
        {!busy && <Sparkles className="w-3.5 h-3.5" />}
        AI
        {!busy && <ChevronDown className="w-3 h-3" />}
      </Button>

      {open && (
        <div className="absolute right-0 top-full mt-1 w-56 bg-white border border-border rounded-lg shadow-lg z-10 overflow-hidden">
          {/* Template selector */}
          {templates.length > 0 && (
            <div className="px-3 py-2 border-b border-border">
              <label className="text-[10px] text-text-tertiary font-medium uppercase tracking-wide block mb-1">Template</label>
              <select
                value={selectedTemplateId}
                onChange={(e) => onTemplateChange(e.target.value)}
                className="w-full text-xs border border-border rounded px-2 py-1 bg-surface-secondary focus:outline-none"
              >
                <option value="">Default</option>
                {templates.map((t) => (
                  <option key={t.id} value={t.id}>{t.name}</option>
                ))}
              </select>
            </div>
          )}
          <button
            onClick={() => { onGenerate(); setOpen(false) }}
            className="w-full text-left px-3 py-2.5 text-xs text-text-primary hover:bg-surface-secondary transition-colors flex items-center gap-2"
          >
            <Sparkles className="w-3.5 h-3.5 text-text-tertiary" />
            <div>
              <p className="font-medium">Regenerate from timeline</p>
              <p className="text-text-tertiary">Rebuild from incident data</p>
            </div>
          </button>
          <button
            onClick={() => { onEnhance(); setOpen(false) }}
            className="w-full text-left px-3 py-2.5 text-xs text-text-primary hover:bg-surface-secondary transition-colors flex items-center gap-2 border-t border-border"
          >
            <PenLine className="w-3.5 h-3.5 text-text-tertiary" />
            <div>
              <p className="font-medium">Enhance with AI</p>
              <p className="text-text-tertiary">Improve what you've written</p>
            </div>
          </button>
        </div>
      )}
    </div>
  )
}

function formatRelativeTime(isoString: string): string {
  const date = new Date(isoString)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffMins = Math.floor(diffMs / 60000)
  if (diffMins < 1) return 'just now'
  if (diffMins < 60) return `${diffMins}m ago`
  const diffHours = Math.floor(diffMins / 60)
  if (diffHours < 24) return `${diffHours}h ago`
  return date.toLocaleDateString()
}

// ── Markdown preview renderer ─────────────────────────────────────────────────
// Converts markdown to React elements — no raw HTML injection.

/** Parses inline markdown (**bold**, *italic*, `code`) into React nodes. */
function parseInline(text: string, baseKey: string): React.ReactNode[] {
  const parts: React.ReactNode[] = []
  const re = /(\*\*(.+?)\*\*|\*(.+?)\*|`([^`]+)`)/g
  let last = 0
  let i = 0
  for (const m of text.matchAll(re)) {
    if ((m.index ?? 0) > last) parts.push(text.slice(last, m.index))
    const k = `${baseKey}-${i++}`
    if (m[0].startsWith('**'))
      parts.push(<strong key={k}>{m[2]}</strong>)
    else if (m[0].startsWith('*'))
      parts.push(<em key={k}>{m[3]}</em>)
    else
      parts.push(<code key={k} className="bg-gray-100 px-1 py-0.5 rounded text-xs font-mono">{m[4]}</code>)
    last = (m.index ?? 0) + m[0].length
  }
  if (last < text.length) parts.push(text.slice(last))
  return parts
}

/** Renders a markdown string as structured React block elements. */
function MarkdownPreview({ content }: { content: string }) {
  const nodes: React.ReactNode[] = []
  let key = 0
  let inCode = false
  let codeLines: string[] = []
  let listItems: React.ReactNode[] = []
  let listType: 'ul' | 'ol' | null = null

  const flushList = () => {
    if (!listType) return
    const k = key++
    nodes.push(
      listType === 'ul'
        ? <ul key={k} className="list-disc list-inside space-y-0.5 my-1.5 text-sm text-text-secondary">{[...listItems]}</ul>
        : <ol key={k} className="list-decimal list-inside space-y-0.5 my-1.5 text-sm text-text-secondary">{[...listItems]}</ol>
    )
    listItems = []
    listType = null
  }

  for (const line of content.split('\n')) {
    if (line.startsWith('```')) {
      if (inCode) {
        nodes.push(
          <pre key={key++} className="bg-gray-50 border border-gray-200 rounded-md px-3 py-2 my-2 text-xs font-mono overflow-x-auto whitespace-pre-wrap">
            <code>{codeLines.join('\n')}</code>
          </pre>
        )
        codeLines = []
        inCode = false
      } else {
        flushList()
        inCode = true
      }
      continue
    }
    if (inCode) { codeLines.push(line); continue }

    if (line.startsWith('### ')) {
      flushList()
      nodes.push(<h3 key={key++} className="text-sm font-semibold text-text-primary mt-4 mb-1">{parseInline(line.slice(4), String(key))}</h3>)
      continue
    }
    if (line.startsWith('## ')) {
      flushList()
      nodes.push(<h2 key={key++} className="text-base font-semibold text-text-primary mt-5 mb-1">{parseInline(line.slice(3), String(key))}</h2>)
      continue
    }
    if (line.startsWith('# ')) {
      flushList()
      nodes.push(<h1 key={key++} className="text-lg font-bold text-text-primary mt-6 mb-2">{parseInline(line.slice(2), String(key))}</h1>)
      continue
    }

    const ulMatch = line.match(/^[-*] (.*)/)
    if (ulMatch) {
      if (listType === 'ol') flushList()
      listType = 'ul'
      listItems.push(<li key={key++}>{parseInline(ulMatch[1] ?? '', String(key))}</li>)
      continue
    }
    const olMatch = line.match(/^\d+[.] (.*)/)
    if (olMatch) {
      if (listType === 'ul') flushList()
      listType = 'ol'
      listItems.push(<li key={key++}>{parseInline(olMatch[1] ?? '', String(key))}</li>)
      continue
    }

    if (line.trim() === '') {
      flushList()
      nodes.push(<div key={key++} className="my-1" />)
      continue
    }

    flushList()
    nodes.push(<p key={key++} className="text-sm text-text-secondary leading-relaxed">{parseInline(line, String(key))}</p>)
  }

  flushList()
  return <>{nodes}</>
}
