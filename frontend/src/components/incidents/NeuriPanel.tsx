import { useState, useEffect, useRef } from 'react'
import { Microscope, AlertCircle, Loader2, ChevronDown, ChevronUp } from 'lucide-react'
import { getNeuriSettings, triggerNeuriInvestigation, getNeuriResults } from '../../api/neuri'
import type { NeuriResult, RankedHypothesis } from '../../api/types'

const POLL_INTERVAL_MS = 5000
const TIMEOUT_MS = 90000

const HYPOTHESIS_COLORS: Record<string, string> = {
  CODE_CHANGE:            'bg-blue-100 text-blue-800',
  CONFIG_CHANGE:          'bg-orange-100 text-orange-800',
  DEPENDENCY_FAILURE:     'bg-red-100 text-red-800',
  RESOURCE_EXHAUSTION:    'bg-amber-100 text-amber-800',
  TRAFFIC_ANOMALY:        'bg-purple-100 text-purple-800',
  INFRASTRUCTURE_FAILURE: 'bg-red-100 text-red-800',
  DATA_ISSUE:             'bg-teal-100 text-teal-800',
  CAPACITY_LIMIT:         'bg-orange-100 text-orange-800',
  UNKNOWN:                'bg-gray-100 text-gray-600',
}

function hypothesisLabel(type: string): string {
  return type.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())
}

function hypothesisBadge(type: string): string {
  return HYPOTHESIS_COLORS[type] ?? 'bg-gray-100 text-gray-600'
}

interface NeuriPanelProps {
  incidentId: string
}

export function NeuriPanel({ incidentId }: NeuriPanelProps) {
  const [configured, setConfigured] = useState<boolean | null>(null)
  const [result, setResult] = useState<NeuriResult | null>(null)
  const [triggering, setTriggering] = useState(false)
  const [polling, setPolling] = useState(false)
  const [timedOut, setTimedOut] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [showAllHypotheses, setShowAllHypotheses] = useState(false)

  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Check if Neuri is configured and load any existing result on mount
  useEffect(() => {
    getNeuriSettings()
      .then((s) => setConfigured(!!s.webhook_url))
      .catch(() => setConfigured(false))

    getNeuriResults(incidentId)
      .then((r) => { if (r.results.length > 0) setResult(r.results[0] ?? null) })
      .catch(() => {})
  }, [incidentId])

  function stopPolling() {
    if (pollRef.current) { clearInterval(pollRef.current); pollRef.current = null }
    if (timeoutRef.current) { clearTimeout(timeoutRef.current); timeoutRef.current = null }
    setPolling(false)
  }

  function startPolling() {
    setPolling(true)
    setTimedOut(false)

    pollRef.current = setInterval(async () => {
      try {
        const r = await getNeuriResults(incidentId)
        if (r.results.length > 0) {
          setResult(r.results[0] ?? null)
          stopPolling()
        }
      } catch {
        // keep polling — transient errors shouldn't stop us
      }
    }, POLL_INTERVAL_MS)

    timeoutRef.current = setTimeout(() => {
      stopPolling()
      setTimedOut(true)
    }, TIMEOUT_MS)
  }

  useEffect(() => () => stopPolling(), [])

  async function handleTrigger() {
    setTriggering(true)
    setError(null)
    setTimedOut(false)
    try {
      await triggerNeuriInvestigation(incidentId)
      startPolling()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to start investigation')
    } finally {
      setTriggering(false)
    }
  }

  // Still loading config — render nothing to avoid layout shift
  if (configured === null) return null

  return (
    <div className="mt-4 pt-4 border-t border-border">
      {/* Header row */}
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-1.5">
          <Microscope className="w-3.5 h-3.5 text-brand-primary" />
          <span className="text-xs font-semibold text-text-tertiary uppercase tracking-wider">Neuri Analysis</span>
          {polling && (
            <span className="flex items-center gap-1 text-xs text-text-tertiary">
              <Loader2 className="w-3 h-3 animate-spin" /> Investigating…
            </span>
          )}
        </div>

        {configured && !result && !polling && (
          <button
            onClick={handleTrigger}
            disabled={triggering}
            className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded text-white text-xs font-medium bg-brand-primary disabled:opacity-50 disabled:cursor-not-allowed hover:opacity-90 transition-opacity"
          >
            {triggering
              ? <><Loader2 className="w-3 h-3 animate-spin" /> Starting…</>
              : <><Microscope className="w-3 h-3" /> Run Investigation</>
            }
          </button>
        )}

        {configured && result && !polling && (
          <button
            onClick={handleTrigger}
            disabled={triggering}
            className="text-xs text-text-tertiary hover:text-text-primary transition-colors disabled:opacity-50"
          >
            {triggering ? 'Starting…' : 'Re-run'}
          </button>
        )}
      </div>

      {/* Not configured */}
      {!configured && (
        <div className="flex items-start gap-1.5 text-xs text-text-tertiary">
          <AlertCircle className="w-3.5 h-3.5 mt-0.5 flex-shrink-0 text-amber-500" />
          <span>Neuri not configured. Set the webhook URL in <strong>Settings → System → Neuri</strong>.</span>
        </div>
      )}

      {/* Error */}
      {error && (
        <div className="flex items-start gap-1.5 text-xs text-red-600">
          <AlertCircle className="w-3.5 h-3.5 mt-0.5 flex-shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {/* Timeout */}
      {timedOut && !result && (
        <div className="flex items-start gap-1.5 text-xs text-amber-600">
          <AlertCircle className="w-3.5 h-3.5 mt-0.5 flex-shrink-0" />
          <span>Investigation timed out (90s). Neuri may still be running — refresh in a moment or re-run.</span>
        </div>
      )}

      {/* Polling skeleton */}
      {polling && !result && (
        <div className="space-y-1.5 mt-1">
          <div className="h-2.5 bg-surface-tertiary rounded animate-pulse w-full" />
          <div className="h-2.5 bg-surface-tertiary rounded animate-pulse w-4/5" />
          <div className="h-2.5 bg-surface-tertiary rounded animate-pulse w-3/5" />
        </div>
      )}

      {/* Idle — no result yet */}
      {configured && !result && !polling && !error && !timedOut && (
        <p className="text-xs text-text-tertiary">
          Run an AI root cause investigation on this incident using Neuri.
        </p>
      )}

      {/* Result card */}
      {result && <ResultCard result={result} showAll={showAllHypotheses} onToggle={() => setShowAllHypotheses(v => !v)} />}
    </div>
  )
}

function ResultCard({
  result,
  showAll,
  onToggle,
}: {
  result: NeuriResult
  showAll: boolean
  onToggle: () => void
}) {
  const ranked: RankedHypothesis[] = Array.isArray(result.ranked_hypotheses) ? result.ranked_hypotheses : []
  const visible = showAll ? ranked : ranked.slice(0, 3)

  return (
    <div className="mt-1 space-y-3">
      {/* Primary finding */}
      <div className="flex items-center gap-2 flex-wrap">
        <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-semibold ${hypothesisBadge(result.top_hypothesis)}`}>
          {hypothesisLabel(result.top_hypothesis)}
        </span>
        <span className="text-xs font-semibold text-text-primary">
          {Math.round(result.confidence * 100)}% confidence
        </span>
      </div>

      {/* Summary */}
      <p className="text-sm text-text-secondary leading-relaxed">{result.summary}</p>

      {/* Ranked list */}
      {ranked.length > 0 && (
        <div className="space-y-1">
          <p className="text-xs font-semibold text-text-tertiary uppercase tracking-wider">All hypotheses</p>
          {visible.map((h) => (
            <div key={h.type} className="flex items-center gap-2">
              <div className="w-24 h-1.5 bg-surface-tertiary rounded-full overflow-hidden">
                <div
                  className="h-full bg-brand-primary rounded-full"
                  style={{ width: `${Math.round(h.confidence * 100)}%` }}
                />
              </div>
              <span className="text-xs text-text-secondary w-8 text-right">{Math.round(h.confidence * 100)}%</span>
              <span className={`text-xs px-1.5 py-0.5 rounded ${hypothesisBadge(h.type)}`}>
                {hypothesisLabel(h.type)}
              </span>
            </div>
          ))}
          {ranked.length > 3 && (
            <button
              onClick={onToggle}
              className="flex items-center gap-0.5 text-xs text-text-tertiary hover:text-text-primary transition-colors mt-1"
            >
              {showAll
                ? <><ChevronUp className="w-3 h-3" /> Show less</>
                : <><ChevronDown className="w-3 h-3" /> {ranked.length - 3} more</>
              }
            </button>
          )}
        </div>
      )}
    </div>
  )
}
