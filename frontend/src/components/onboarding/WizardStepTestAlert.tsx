import { useState, useEffect, useRef } from 'react'
import { Loader2, Copy, Check, CheckCircle, ExternalLink } from 'lucide-react'
import { listAlerts } from '../../api/alerts'

interface Props {
  onComplete: () => void
  onSkip: () => void
}

const POLL_INTERVAL_MS = 3000
const TIMEOUT_MS = 120_000

export function WizardStepTestAlert({ onComplete, onSkip }: Props) {
  const webhookUrl = `${window.location.origin}/api/v1/webhooks/generic`
  const curlCommand = `curl -s -X POST ${webhookUrl} \\
  -H 'Content-Type: application/json' \\
  -d '{"title":"Test alert from wizard","severity":"warning","status":"firing"}'`

  const [urlCopied, setUrlCopied] = useState(false)
  const [cmdCopied, setCmdCopied] = useState(false)
  const [alertDetected, setAlertDetected] = useState(false)
  const [alertId, setAlertId] = useState<string | null>(null)
  const [timedOut, setTimedOut] = useState(false)

  const startTime = useRef(Date.now())
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    let stopped = false

    async function poll() {
      if (stopped) return
      const now = Date.now()
      if (now - startTime.current >= TIMEOUT_MS) {
        setTimedOut(true)
        return
      }
      try {
        const res = await listAlerts({ limit: 5 })
        const first = res.data?.[0]
        if (!stopped && first) {
          setAlertDetected(true)
          setAlertId(first.id)
          return
        }
      } catch {
        // ignore poll errors
      }
      if (!stopped) {
        timer.current = setTimeout(poll, POLL_INTERVAL_MS)
      }
    }

    poll()

    return () => {
      stopped = true
      if (timer.current) clearTimeout(timer.current)
    }
  }, [])

  function copyUrl() {
    navigator.clipboard.writeText(webhookUrl)
    setUrlCopied(true)
    setTimeout(() => setUrlCopied(false), 2000)
  }

  function copyCmd() {
    navigator.clipboard.writeText(curlCommand)
    setCmdCopied(true)
    setTimeout(() => setCmdCopied(false), 2000)
  }

  if (alertDetected) {
    return (
      <div className="space-y-4">
        <div className="flex items-start gap-3 rounded-lg bg-green-50 border border-green-200 px-4 py-3">
          <CheckCircle className="w-5 h-5 text-green-600 flex-shrink-0 mt-0.5" />
          <div>
            <p className="text-sm font-medium text-green-700">Alert received!</p>
            <p className="text-xs text-green-600 mt-0.5">Your webhook is working. Regen is ready to receive alerts.</p>
          </div>
        </div>
        {alertId && (
          <a
            href={`/alerts/${alertId}`}
            className="inline-flex items-center gap-1.5 text-sm text-brand-primary hover:underline"
          >
            View the alert <ExternalLink className="w-3.5 h-3.5" />
          </a>
        )}
        <div className="flex justify-end">
          <button onClick={onComplete} className="px-4 py-2 rounded-lg bg-brand-primary hover:bg-brand-primary-hover text-white text-sm font-medium transition-colors">
            Continue →
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <p className="text-sm text-text-secondary">
        Send a test alert to your Regen webhook to make sure everything is connected.
      </p>

      <div>
        <label className="block text-xs font-medium text-text-secondary mb-1">Webhook URL</label>
        <div className="flex items-center gap-2">
          <code className="flex-1 h-9 flex items-center px-3 rounded-lg bg-surface-secondary border border-border text-xs font-mono text-text-primary truncate">
            {webhookUrl}
          </code>
          <button onClick={copyUrl} className="flex-shrink-0 p-2 rounded-lg border border-border hover:bg-surface-secondary transition-colors" title="Copy URL">
            {urlCopied ? <Check className="w-4 h-4 text-green-600" /> : <Copy className="w-4 h-4 text-text-tertiary" />}
          </button>
        </div>
      </div>

      <div>
        <div className="flex items-center justify-between mb-1">
          <label className="text-xs font-medium text-text-secondary">Test with curl</label>
          <button onClick={copyCmd} className="flex items-center gap-1 text-xs text-text-tertiary hover:text-text-secondary transition-colors">
            {cmdCopied ? <Check className="w-3 h-3 text-green-600" /> : <Copy className="w-3 h-3" />}
            {cmdCopied ? 'Copied' : 'Copy'}
          </button>
        </div>
        <pre className="bg-surface-secondary rounded-lg px-3 py-2.5 text-xs font-mono text-text-secondary overflow-x-auto whitespace-pre-wrap">
          {curlCommand}
        </pre>
      </div>

      <div className="flex items-center gap-2 text-sm text-text-secondary">
        {!timedOut && <Loader2 className="w-4 h-4 animate-spin text-brand-primary flex-shrink-0" />}
        {timedOut
          ? <span className="text-text-tertiary">Timed out waiting. You can send the test later.</span>
          : <span>Waiting for alert… (auto-completes when received)</span>
        }
      </div>

      <div className="flex items-center justify-between pt-1">
        <button onClick={onSkip} className="text-sm text-text-tertiary hover:text-text-secondary transition-colors">
          Skip for now →
        </button>
        {timedOut && (
          <button onClick={onComplete} className="px-4 py-2 rounded-lg bg-brand-primary hover:bg-brand-primary-hover text-white text-sm font-medium transition-colors">
            Mark as done →
          </button>
        )}
      </div>
    </div>
  )
}
