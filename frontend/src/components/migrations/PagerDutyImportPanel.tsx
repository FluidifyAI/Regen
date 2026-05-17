import { useState } from 'react'
import { AlertTriangle, CheckCircle, ChevronDown, ChevronRight, Eye, EyeOff, Loader2 } from 'lucide-react'
import { Button } from '../ui/Button'
import {
  previewPagerDutyMigration,
  importPagerDutyMigration,
  PDPreviewResponse,
  PDImportResponse,
} from '../../api/migrations'

type Step = 'idle' | 'previewing' | 'preview' | 'importing' | 'done'

interface PagerDutyImportPanelProps {
  onComplete?: () => void
}

function extractError(e: unknown): string {
  if (e && typeof e === 'object' && 'message' in e) return String((e as { message: unknown }).message)
  return 'An unexpected error occurred.'
}

export function PagerDutyImportPanel({ onComplete }: PagerDutyImportPanelProps) {
  const [step, setStep] = useState<Step>('idle')
  const [apiKey, setApiKey] = useState('')
  const [showKey, setShowKey] = useState(false)
  const [force, setForce] = useState(false)
  const [error, setError] = useState('')
  const [preview, setPreview] = useState<PDPreviewResponse | null>(null)
  const [result, setResult] = useState<PDImportResponse | null>(null)
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})

  const toggle = (key: string) => setExpanded((prev) => ({ ...prev, [key]: !prev[key] }))

  async function handlePreview() {
    setError('')
    if (!apiKey.trim()) {
      setError('API key is required.')
      return
    }
    setStep('previewing')
    try {
      const data = await previewPagerDutyMigration({ api_key: apiKey.trim() })
      setPreview(data)
      setStep('preview')
    } catch (e) {
      setError(extractError(e))
      setStep('idle')
    }
  }

  async function handleImport() {
    if (!preview) return
    setError('')
    setStep('importing')
    try {
      const data = await importPagerDutyMigration({ api_key: apiKey.trim(), force })
      setResult(data)
      setStep('done')
    } catch (e) {
      setError(extractError(e))
      setStep('preview')
    }
  }

  return (
    <div className="space-y-4">
      {/* ── Error banner ─────────────────────────────────────────────────── */}
      {error && (
        <div className="flex items-start gap-3 rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-800">
          <AlertTriangle className="mt-0.5 h-4 w-4 flex-shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {/* ── Step 1: API key input ─────────────────────────────────────────── */}
      {(step === 'idle' || step === 'previewing') && (
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-text-primary mb-1">
              PagerDuty API Key
            </label>
            <p className="text-xs text-text-secondary mb-2">
              Create a read-only API key in PagerDuty → Integrations → API Access Keys.
            </p>
            <div className="relative">
              <input
                type={showKey ? 'text' : 'password'}
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                placeholder="u+xxxxxxxxxxxxxxxxxxxx"
                className="w-full rounded-lg border border-border bg-surface-secondary px-3 py-2 pr-10 text-sm text-text-primary placeholder-text-tertiary focus:border-accent-primary focus:outline-none"
              />
              <button
                type="button"
                onClick={() => setShowKey((v) => !v)}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-text-secondary hover:text-text-primary"
              >
                {showKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
              </button>
            </div>
          </div>

          <Button
            onClick={handlePreview}
            disabled={step === 'previewing' || !apiKey.trim()}
            className="w-full"
          >
            {step === 'previewing' ? (
              <><Loader2 className="mr-2 h-4 w-4 animate-spin" /> Fetching from PagerDuty…</>
            ) : (
              'Preview Import'
            )}
          </Button>
        </div>
      )}

      {/* ── Step 2: Preview ──────────────────────────────────────────────── */}
      {(step === 'preview' || step === 'importing') && preview && (
        <div className="space-y-4">
          <div className="rounded-lg border border-border bg-surface-primary p-4">
            <h3 className="text-sm font-semibold text-text-primary mb-3">What will be imported</h3>

            {/* Schedules */}
            <button
              className="flex w-full items-center justify-between py-2 text-sm text-text-primary hover:text-accent-primary"
              onClick={() => toggle('schedules')}
            >
              <span className="font-medium">
                Schedules <span className="text-text-secondary">({preview.schedules.length})</span>
              </span>
              {expanded.schedules ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
            </button>
            {expanded.schedules && (
              <ul className="mb-2 ml-2 space-y-1">
                {preview.schedules.map((s) => (
                  <li key={s.name} className="text-xs text-text-secondary">
                    {s.name} — {s.layer_count} layer{s.layer_count !== 1 ? 's' : ''}, {s.user_count} user{s.user_count !== 1 ? 's' : ''}
                  </li>
                ))}
              </ul>
            )}

            {/* Escalation policies */}
            <button
              className="flex w-full items-center justify-between py-2 text-sm text-text-primary hover:text-accent-primary"
              onClick={() => toggle('policies')}
            >
              <span className="font-medium">
                Escalation Policies <span className="text-text-secondary">({preview.policies.length})</span>
              </span>
              {expanded.policies ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
            </button>
            {expanded.policies && (
              <ul className="mb-2 ml-2 space-y-1">
                {preview.policies.map((p) => (
                  <li key={p.name} className="text-xs text-text-secondary">
                    {p.name} — {p.tier_count} tier{p.tier_count !== 1 ? 's' : ''}
                  </li>
                ))}
              </ul>
            )}

            {/* Warnings */}
            {preview.warnings.length > 0 && (
              <div className="mt-3 rounded-md bg-yellow-50 border border-yellow-200 p-3">
                <p className="text-xs font-medium text-yellow-800 mb-1">Notes</p>
                {preview.warnings.map((w, i) => (
                  <p key={i} className="text-xs text-yellow-700">{w}</p>
                ))}
              </div>
            )}
          </div>

          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              id="pd-force"
              checked={force}
              onChange={(e) => setForce(e.target.checked)}
              className="rounded border-border"
            />
            <label htmlFor="pd-force" className="text-sm text-text-secondary">
              Overwrite records with conflicting names (--force)
            </label>
          </div>

          <div className="flex gap-3">
            <Button variant="ghost" onClick={() => { setStep('idle'); setPreview(null) }}>
              Back
            </Button>
            <Button
              onClick={handleImport}
              disabled={step === 'importing'}
              className="flex-1"
            >
              {step === 'importing' ? (
                <><Loader2 className="mr-2 h-4 w-4 animate-spin" /> Importing…</>
              ) : (
                'Confirm Import'
              )}
            </Button>
          </div>
        </div>
      )}

      {/* ── Step 3: Done ─────────────────────────────────────────────────── */}
      {step === 'done' && result && (
        <div className="space-y-4">
          <div className="flex items-start gap-3 rounded-lg border border-green-200 bg-green-50 p-4">
            <CheckCircle className="mt-0.5 h-5 w-5 text-green-600 flex-shrink-0" />
            <div>
              <p className="text-sm font-semibold text-green-800">Import complete</p>
              <p className="text-xs text-green-700 mt-0.5">
                {result.summary.schedules_imported} schedule{result.summary.schedules_imported !== 1 ? 's' : ''} and{' '}
                {result.summary.policies_imported} escalation {result.summary.policies_imported !== 1 ? 'policies' : 'policy'} imported.
              </p>
            </div>
          </div>

          {result.warnings.length > 0 && (
            <div className="rounded-md bg-yellow-50 border border-yellow-200 p-3">
              <p className="text-xs font-medium text-yellow-800 mb-1">Warnings ({result.warnings.length})</p>
              {result.warnings.map((w, i) => (
                <p key={i} className="text-xs text-yellow-700">{w}</p>
              ))}
            </div>
          )}

          {result.errors.length > 0 && (
            <div className="rounded-md bg-red-50 border border-red-200 p-3">
              <p className="text-xs font-medium text-red-800 mb-1">Errors ({result.errors.length})</p>
              {result.errors.map((e, i) => (
                <p key={i} className="text-xs text-red-700">{e}</p>
              ))}
            </div>
          )}

          {onComplete && (
            <Button onClick={onComplete} className="w-full">
              Continue
            </Button>
          )}
        </div>
      )}
    </div>
  )
}
