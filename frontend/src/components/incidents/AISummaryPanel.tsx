import { useState, useEffect } from 'react'
import { Sparkles, RefreshCw, AlertCircle, Copy, Check } from 'lucide-react'
import { Button } from '../ui/Button'
import { summarizeIncident, getAISettings } from '../../api/ai'

interface AISummaryPanelProps {
  incidentId: string
  existingSummary?: string
  existingSummaryGeneratedAt?: string
  onSummaryGenerated?: (summary: string) => void
}

/**
 * AISummaryPanel shows the AI-generated incident summary and allows regeneration.
 *
 * Behaviour:
 * - On first render, checks if AI is enabled (GET /settings/ai). If not, shows a
 *   disabled state explaining how to enable it.
 * - If a summary already exists (from the incident response), it is displayed immediately
 *   without making an extra API call.
 * - "Generate summary" / "Regenerate" calls the summarize endpoint and updates the display.
 */
export function AISummaryPanel({
  incidentId,
  existingSummary,
  existingSummaryGeneratedAt,
  onSummaryGenerated,
}: AISummaryPanelProps) {
  const [aiEnabled, setAiEnabled] = useState<boolean | null>(null) // null = checking
  const [summary, setSummary] = useState<string>(existingSummary ?? '')
  const [generatedAt, setGeneratedAt] = useState<string | undefined>(existingSummaryGeneratedAt)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  // Check AI availability on mount
  useEffect(() => {
    getAISettings()
      .then((s) => setAiEnabled(s.enabled))
      .catch(() => setAiEnabled(false))
  }, [])

  // Sync external prop changes (e.g. after page refetch)
  useEffect(() => {
    if (existingSummary) setSummary(existingSummary)
    if (existingSummaryGeneratedAt) setGeneratedAt(existingSummaryGeneratedAt)
  }, [existingSummary, existingSummaryGeneratedAt])

  async function handleGenerate() {
    setLoading(true)
    setError(null)
    try {
      const result = await summarizeIncident(incidentId)
      setSummary(result.summary)
      setGeneratedAt(result.generated_at)
      onSummaryGenerated?.(result.summary)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to generate summary')
    } finally {
      setLoading(false)
    }
  }

  async function handleCopy() {
    if (!summary) return
    await navigator.clipboard.writeText(summary)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  // Still checking AI availability
  if (aiEnabled === null) {
    return (
      <div className="border border-border rounded-lg p-4 bg-surface-secondary animate-pulse">
        <div className="h-4 w-32 bg-surface-tertiary rounded" />
      </div>
    )
  }

  return (
    <div className="border border-border rounded-lg overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 bg-white border-b border-border">
        <div className="flex items-center gap-2">
          <Sparkles className="w-4 h-4 text-brand-primary" />
          <span className="text-sm font-medium text-text-primary">AI Summary</span>
          {generatedAt && (
            <span className="text-xs text-text-tertiary">
              · {formatRelativeTime(generatedAt)}
            </span>
          )}
        </div>
        <div className="flex items-center gap-2">
          {summary && (
            <button
              onClick={handleCopy}
              className="p-1.5 rounded text-text-tertiary hover:text-text-primary hover:bg-surface-secondary transition-colors"
              title="Copy summary"
            >
              {copied ? (
                <Check className="w-3.5 h-3.5 text-green-500" />
              ) : (
                <Copy className="w-3.5 h-3.5" />
              )}
            </button>
          )}
          <Button
            variant="ghost"
            size="sm"
            onClick={handleGenerate}
            loading={loading}
            disabled={!aiEnabled || loading}
            className="text-xs"
          >
            {!loading && <RefreshCw className="w-3.5 h-3.5" />}
            {summary ? 'Regenerate' : 'Generate'}
          </Button>
        </div>
      </div>

      {/* Body */}
      <div className="px-4 py-3 bg-white">
        {!aiEnabled && (
          <div className="flex items-start gap-2 text-sm text-text-tertiary">
            <AlertCircle className="w-4 h-4 mt-0.5 flex-shrink-0 text-amber-500" />
            <span>
              AI features are disabled. Set the{' '}
              <code className="font-mono text-xs bg-surface-secondary px-1 py-0.5 rounded">
                OPENAI_API_KEY
              </code>{' '}
              environment variable to enable them.
            </span>
          </div>
        )}

        {aiEnabled && error && (
          <div className="flex items-start gap-2 text-sm text-red-600">
            <AlertCircle className="w-4 h-4 mt-0.5 flex-shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {aiEnabled && !error && !summary && !loading && (
          <p className="text-sm text-text-tertiary">
            Generate an AI-powered summary of this incident using the timeline, alerts, and Slack thread context.
          </p>
        )}

        {aiEnabled && loading && !summary && (
          <div className="space-y-2">
            <div className="h-3 bg-surface-tertiary rounded animate-pulse w-full" />
            <div className="h-3 bg-surface-tertiary rounded animate-pulse w-5/6" />
            <div className="h-3 bg-surface-tertiary rounded animate-pulse w-4/6" />
          </div>
        )}

        {summary && (
          <p className={`text-sm text-text-secondary leading-relaxed whitespace-pre-wrap ${loading ? 'opacity-50' : ''}`}>
            {summary}
          </p>
        )}
      </div>
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
