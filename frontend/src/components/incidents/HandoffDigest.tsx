import { useState } from 'react'
import { ClipboardList, RefreshCw, AlertCircle, Copy, Check } from 'lucide-react'
import { Button } from '../ui/Button'
import { generateHandoffDigest } from '../../api/ai'

interface HandoffDigestProps {
  incidentId: string
  aiEnabled: boolean
}

/**
 * HandoffDigest generates and displays a shift handoff document.
 * Unlike AISummaryPanel, digests are not persisted — they are generated on demand
 * for the current shift and are expected to be regenerated each time.
 */
export function HandoffDigest({ incidentId, aiEnabled }: HandoffDigestProps) {
  const [digest, setDigest] = useState<string>('')
  const [generatedAt, setGeneratedAt] = useState<string | undefined>()
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  async function handleGenerate() {
    setLoading(true)
    setError(null)
    try {
      const result = await generateHandoffDigest(incidentId)
      setDigest(result.digest)
      setGeneratedAt(result.generated_at)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to generate handoff digest')
    } finally {
      setLoading(false)
    }
  }

  async function handleCopy() {
    if (!digest) return
    await navigator.clipboard.writeText(digest)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="border border-border rounded-lg overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 bg-white border-b border-border">
        <div className="flex items-center gap-2">
          <ClipboardList className="w-4 h-4 text-brand-primary" />
          <span className="text-sm font-medium text-text-primary">Handoff Digest</span>
          {generatedAt && (
            <span className="text-xs text-text-tertiary">
              · generated {new Date(generatedAt).toLocaleTimeString()}
            </span>
          )}
        </div>
        <div className="flex items-center gap-2">
          {digest && (
            <button
              onClick={handleCopy}
              className="p-1.5 rounded text-text-tertiary hover:text-text-primary hover:bg-surface-secondary transition-colors"
              title="Copy digest"
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
            {digest ? 'Regenerate' : 'Generate Digest'}
          </Button>
        </div>
      </div>

      {/* Body */}
      <div className="px-4 py-3 bg-white">
        {!aiEnabled && (
          <div className="flex items-start gap-2 text-sm text-text-tertiary">
            <AlertCircle className="w-4 h-4 mt-0.5 flex-shrink-0 text-amber-500" />
            <span>
              AI features must be enabled to generate handoff digests.
            </span>
          </div>
        )}

        {aiEnabled && error && (
          <div className="flex items-start gap-2 text-sm text-red-600">
            <AlertCircle className="w-4 h-4 mt-0.5 flex-shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {aiEnabled && !error && !digest && !loading && (
          <p className="text-sm text-text-tertiary">
            Generate a structured handoff document for the incoming on-call engineer. Includes current situation, key events, open concerns, and recommended next steps.
          </p>
        )}

        {aiEnabled && loading && !digest && (
          <div className="space-y-2">
            {[...Array(5)].map((_, i) => (
              <div
                key={i}
                className="h-3 bg-surface-tertiary rounded animate-pulse"
                style={{ width: `${85 - i * 8}%` }}
              />
            ))}
          </div>
        )}

        {digest && (
          <pre className={`text-sm text-text-secondary leading-relaxed whitespace-pre-wrap font-sans ${loading ? 'opacity-50' : ''}`}>
            {digest}
          </pre>
        )}
      </div>
    </div>
  )
}
