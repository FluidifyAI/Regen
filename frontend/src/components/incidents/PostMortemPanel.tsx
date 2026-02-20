import { useState, useEffect, useCallback } from 'react'
import {
  FileText,
  Sparkles,
  Download,
  Send,
  AlertCircle,
  ChevronDown,
} from 'lucide-react'
import { Button } from '../ui/Button'
import {
  getPostMortem,
  generatePostMortem,
  updatePostMortem,
  getPostMortemExportUrl,
  listPostMortemTemplates,
} from '../../api/postmortems'
import { getAISettings } from '../../api/ai'
import { ActionItems } from './ActionItems'
import type { PostMortem, PostMortemTemplate } from '../../api/types'

interface PostMortemPanelProps {
  incidentId: string
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
export function PostMortemPanel({ incidentId }: PostMortemPanelProps) {
  const [pm, setPm] = useState<PostMortem | null>(null)
  const [loading, setLoading] = useState(true)
  const [aiEnabled, setAiEnabled] = useState<boolean | null>(null)
  const [templates, setTemplates] = useState<PostMortemTemplate[]>([])
  const [selectedTemplateId, setSelectedTemplateId] = useState<string>('')
  const [generating, setGenerating] = useState(false)
  const [saving, setSaving] = useState(false)
  const [publishing, setPublishing] = useState(false)
  const [editedContent, setEditedContent] = useState('')
  const [isDirty, setIsDirty] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetchPm = useCallback(async () => {
    try {
      const data = await getPostMortem(incidentId)
      setPm(data)
      if (data) {
        setEditedContent(data.content)
        setIsDirty(false)
      }
    } catch {
      setPm(null)
    }
  }, [incidentId])

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
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to generate post-mortem')
    } finally {
      setGenerating(false)
    }
  }

  async function handleSave(): Promise<boolean> {
    if (!pm) return false
    setSaving(true)
    setError(null)
    try {
      const updated = await updatePostMortem(incidentId, { content: editedContent })
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
    // Save content first if dirty; bail if save fails
    if (isDirty) {
      const saved = await handleSave()
      if (!saved) return
    }
    setPublishing(true)
    setError(null)
    try {
      const updated = await updatePostMortem(incidentId, { status: 'published' })
      setPm(updated)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to publish')
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

          {/* Generate / Regenerate */}
          {aiEnabled && (
            <GenerateButton
              hasExisting={!!pm}
              generating={generating}
              templates={templates}
              selectedTemplateId={selectedTemplateId}
              onTemplateChange={setSelectedTemplateId}
              onGenerate={handleGenerate}
            />
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

      {/* No post-mortem yet */}
      {!pm && aiEnabled !== false && (
        <div className="border border-dashed border-border rounded-lg p-8 text-center">
          <FileText className="w-8 h-8 text-text-tertiary mx-auto mb-3" />
          <p className="text-sm text-text-secondary mb-1">No post-mortem yet</p>
          <p className="text-xs text-text-tertiary">
            Use the Generate button above to create an AI-drafted post-mortem.
          </p>
        </div>
      )}

      {/* Content editor */}
      {pm && (
        <div className="border border-border rounded-lg overflow-hidden">
          <div className="flex items-center justify-between px-3 py-2 bg-surface-secondary border-b border-border">
            <span className="text-xs text-text-tertiary">
              {pm.template_name} · by {pm.generated_by === 'ai' ? 'AI' : 'Manual'}
              {pm.generated_at && ` · ${formatRelativeTime(pm.generated_at)}`}
            </span>
            {isDirty && !isPublished && (
              <Button variant="primary" size="sm" onClick={handleSave} loading={saving}>
                Save
              </Button>
            )}
          </div>
          <textarea
            value={editedContent}
            onChange={(e) => {
              setEditedContent(e.target.value)
              setIsDirty(true)
            }}
            readOnly={isPublished}
            className={`w-full min-h-96 p-4 font-mono text-sm text-text-primary resize-y focus:outline-none ${
              isPublished ? 'bg-surface-secondary text-text-secondary cursor-default' : 'bg-white'
            }`}
            placeholder="Post-mortem content..."
          />
        </div>
      )}

      {/* Action items */}
      {pm && (
        <ActionItems incidentId={incidentId} initialItems={pm.action_items} />
      )}
    </div>
  )
}

/**
 * GenerateButton with template selector dropdown
 */
function GenerateButton({
  hasExisting,
  generating,
  templates,
  selectedTemplateId,
  onTemplateChange,
  onGenerate,
}: {
  hasExisting: boolean
  generating: boolean
  templates: PostMortemTemplate[]
  selectedTemplateId: string
  onTemplateChange: (id: string) => void
  onGenerate: () => void
}) {
  return (
    <div className="flex items-center">
      <Button
        variant="primary"
        size="sm"
        onClick={onGenerate}
        loading={generating}
        disabled={generating}
        className="rounded-r-none border-r-0"
      >
        {!generating && <Sparkles className="w-3.5 h-3.5" />}
        {hasExisting ? 'Regenerate' : 'Generate'}
      </Button>
      {templates.length > 0 && (
        <div className="relative">
          <select
            value={selectedTemplateId}
            onChange={(e) => onTemplateChange(e.target.value)}
            disabled={generating}
            className="h-full pl-2 pr-6 py-1.5 text-sm border border-border border-l-0 rounded-l-none rounded-r bg-white text-text-secondary hover:bg-surface-secondary focus:outline-none focus:ring-1 focus:ring-brand-primary appearance-none cursor-pointer"
            title="Select template"
          >
            <option value="">Default template</option>
            {templates.map((t) => (
              <option key={t.id} value={t.id}>
                {t.name}
              </option>
            ))}
          </select>
          <ChevronDown className="absolute right-1 top-1/2 -translate-y-1/2 w-3 h-3 text-text-tertiary pointer-events-none" />
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
