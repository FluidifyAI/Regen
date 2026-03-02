import { useState, useEffect } from 'react'
import { ClipboardList, AlertCircle, Copy, Check } from 'lucide-react'
import { AIButton } from '../ui/AIButton'
import { generateHandoffDigest, getAISettings } from '../../api/ai'

interface HandoffDigestProps {
  incidentId: string
  /** Latest timeline entry timestamp — used to detect whether a regeneration is needed. */
  lastActivityAt?: string
}

/**
 * Generates and displays a shift handoff document.
 * Digests are not persisted — generated on demand for the current shift.
 * Stale detection: if generated after the last activity, re-clicking Regenerate
 * shows a notice instead of consuming tokens.
 */
export function HandoffDigest({ incidentId, lastActivityAt }: HandoffDigestProps) {
  const [aiEnabled, setAiEnabled] = useState<boolean | null>(null)
  const [digest, setDigest] = useState<string>('')
  const [generatedAt, setGeneratedAt] = useState<string | undefined>()
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const [upToDateMsg, setUpToDateMsg] = useState(false)

  useEffect(() => {
    getAISettings()
      .then((s) => setAiEnabled(s.enabled))
      .catch(() => setAiEnabled(false))
  }, [])

  const isUpToDate = !!(
    digest &&
    generatedAt &&
    lastActivityAt &&
    new Date(generatedAt) >= new Date(lastActivityAt)
  )

  async function handleGenerate() {
    if (isUpToDate) {
      setUpToDateMsg(true)
      setTimeout(() => setUpToDateMsg(false), 3500)
      return
    }
    setLoading(true)
    setError(null)
    setUpToDateMsg(false)
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

  if (aiEnabled === null) return null

  return (
    <div className="border border-border rounded-lg overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 bg-white border-b border-border">
        <div className="flex items-center gap-2 flex-wrap">
          <div className="flex items-center gap-1.5">
            <ClipboardList className="w-4 h-4 text-brand-primary" />
            <span className="text-sm font-medium text-text-primary">Handoff Digest</span>
          </div>
          {generatedAt && (
            <span className="text-xs text-text-tertiary">
              · {new Date(generatedAt).toLocaleTimeString()}
            </span>
          )}
          {isUpToDate && (
            <span className="inline-flex items-center gap-0.5 text-xs text-green-600">
              <Check className="w-3 h-3" /> Up to date
            </span>
          )}
        </div>
        <div className="flex items-center gap-1.5 flex-shrink-0">
          {digest && (
            <button
              onClick={handleCopy}
              className="p-1.5 rounded text-text-tertiary hover:text-text-primary hover:bg-surface-secondary transition-colors"
              title="Copy digest"
            >
              {copied
                ? <Check className="w-3.5 h-3.5 text-green-500" />
                : <Copy className="w-3.5 h-3.5" />
              }
            </button>
          )}
          {aiEnabled && (
            <AIButton onClick={handleGenerate} loading={loading}>
              {digest ? 'Regenerate' : 'Generate Digest'}
            </AIButton>
          )}
        </div>
      </div>

      {/* Body */}
      <div className="px-4 py-3 bg-white">
        {upToDateMsg && (
          <p className="text-xs text-text-tertiary mb-2 flex items-center gap-1">
            <Check className="w-3 h-3 text-green-500" />
            No changes since last generation — using cached digest.
          </p>
        )}

        {!aiEnabled && (
          <div className="flex items-start gap-1.5 text-sm text-text-tertiary">
            <AlertCircle className="w-4 h-4 mt-0.5 flex-shrink-0 text-amber-500" />
            <span>
              AI not configured. Set{' '}
              <code className="font-mono text-xs bg-surface-secondary px-1 py-0.5 rounded">OPENAI_API_KEY</code>
              {' '}to enable.
            </span>
          </div>
        )}

        {aiEnabled && error && (
          <div className="flex items-start gap-1.5 text-sm text-red-600">
            <AlertCircle className="w-4 h-4 mt-0.5 flex-shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {aiEnabled && !digest && !loading && !error && (
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
