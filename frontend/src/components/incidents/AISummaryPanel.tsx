import { useState, useEffect } from 'react'
import { Sparkles, AlertCircle, Copy, Check } from 'lucide-react'
import { AIButton } from '../ui/AIButton'
import { summarizeIncident, getAISettings } from '../../api/ai'

interface AISummaryPanelProps {
  incidentId: string
  existingSummary?: string
  existingSummaryGeneratedAt?: string
  /** Latest timeline entry timestamp — used to detect whether a regeneration is needed. */
  lastActivityAt?: string
  onSummaryGenerated?: (summary: string) => void
}

/**
 * Inline AI summary section for the incident header.
 * Stale detection: if the existing summary was generated after the last activity,
 * clicking Regenerate shows an "up to date" notice instead of consuming tokens.
 */
export function AISummaryPanel({
  incidentId,
  existingSummary,
  existingSummaryGeneratedAt,
  lastActivityAt,
  onSummaryGenerated,
}: AISummaryPanelProps) {
  const [aiEnabled, setAiEnabled] = useState<boolean | null>(null)
  const [summary, setSummary] = useState<string>(existingSummary ?? '')
  const [generatedAt, setGeneratedAt] = useState<string | undefined>(existingSummaryGeneratedAt)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const [upToDateMsg, setUpToDateMsg] = useState(false)

  useEffect(() => {
    getAISettings()
      .then((s) => setAiEnabled(s.enabled))
      .catch(() => setAiEnabled(false))
  }, [])

  useEffect(() => {
    if (existingSummary) setSummary(existingSummary)
    if (existingSummaryGeneratedAt) setGeneratedAt(existingSummaryGeneratedAt)
  }, [existingSummary, existingSummaryGeneratedAt])

  const isUpToDate = !!(
    generatedAt &&
    lastActivityAt &&
    new Date(generatedAt) >= new Date(lastActivityAt)
  )

  async function handleGenerate() {
    if (isUpToDate && summary) {
      setUpToDateMsg(true)
      setTimeout(() => setUpToDateMsg(false), 3500)
      return
    }
    setLoading(true)
    setError(null)
    setUpToDateMsg(false)
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

  // Still checking AI availability — render nothing to avoid layout shift
  if (aiEnabled === null) return null

  return (
    <div className="mt-4 pt-4 border-t border-border">
      {/* Row: label + meta + actions */}
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2 flex-wrap">
          <div className="flex items-center gap-1.5">
            <Sparkles className="w-3.5 h-3.5 text-brand-primary" />
            <span className="text-xs font-semibold text-text-tertiary uppercase tracking-wider">AI Summary</span>
          </div>
          {generatedAt && (
            <span className="text-xs text-text-tertiary">· {formatRelativeTime(generatedAt)}</span>
          )}
          {isUpToDate && (
            <span className="inline-flex items-center gap-0.5 text-xs text-green-600">
              <Check className="w-3 h-3" /> Up to date
            </span>
          )}
        </div>
        <div className="flex items-center gap-1.5 flex-shrink-0">
          {summary && (
            <button
              onClick={handleCopy}
              className="p-1 rounded text-text-tertiary hover:text-text-primary hover:bg-surface-secondary transition-colors"
              title="Copy summary"
            >
              {copied
                ? <Check className="w-3.5 h-3.5 text-green-500" />
                : <Copy className="w-3.5 h-3.5" />
              }
            </button>
          )}
          {aiEnabled && (
            <AIButton onClick={handleGenerate} loading={loading}>
              {summary ? 'Regenerate' : 'Generate'}
            </AIButton>
          )}
        </div>
      </div>

      {/* Body */}
      {upToDateMsg && (
        <p className="text-xs text-text-tertiary mb-2 flex items-center gap-1">
          <Check className="w-3 h-3 text-green-500" />
          No changes since last generation — using cached summary.
        </p>
      )}

      {!aiEnabled && (
        <div className="flex items-start gap-1.5 text-xs text-text-tertiary">
          <AlertCircle className="w-3.5 h-3.5 mt-0.5 flex-shrink-0 text-amber-500" />
          <span>
            AI not configured. Set{' '}
            <code className="font-mono bg-surface-secondary px-1 py-0.5 rounded">OPENAI_API_KEY</code>
            {' '}to enable.
          </span>
        </div>
      )}

      {aiEnabled && error && (
        <div className="flex items-start gap-1.5 text-xs text-red-600">
          <AlertCircle className="w-3.5 h-3.5 mt-0.5 flex-shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {aiEnabled && !summary && !loading && !error && (
        <p className="text-xs text-text-tertiary">
          Generate an AI summary using the incident timeline, alerts, and chat context.
        </p>
      )}

      {aiEnabled && loading && !summary && (
        <div className="space-y-1.5">
          <div className="h-2.5 bg-surface-tertiary rounded animate-pulse w-full" />
          <div className="h-2.5 bg-surface-tertiary rounded animate-pulse w-5/6" />
          <div className="h-2.5 bg-surface-tertiary rounded animate-pulse w-4/6" />
        </div>
      )}

      {summary && (
        <p className={`text-sm text-text-secondary leading-relaxed whitespace-pre-wrap ${loading ? 'opacity-50' : ''}`}>
          {summary}
        </p>
      )}
    </div>
  )
}

function formatRelativeTime(isoString: string): string {
  const date = new Date(isoString)
  const diffMins = Math.floor((Date.now() - date.getTime()) / 60000)
  if (diffMins < 1) return 'just now'
  if (diffMins < 60) return `${diffMins}m ago`
  const diffHours = Math.floor(diffMins / 60)
  if (diffHours < 24) return `${diffHours}h ago`
  return date.toLocaleDateString()
}
