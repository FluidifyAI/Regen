import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { CheckCircle, Copy, ChevronDown, ChevronRight, AlertTriangle } from 'lucide-react'
import { useAuth } from '../hooks/useAuth'
import { Button } from '../components/ui/Button'
import {
  previewOnCallMigration,
  importOnCallMigration,
  OnCallPreviewResponse,
  OnCallImportResponse,
  UserSetupToken,
  ConflictItem,
  SkippedItem,
} from '../api/migrations'

type Step = 'connect' | 'previewing' | 'preview' | 'importing' | 'done'

export function SettingsMigrationsPage() {
  const { user: currentUser } = useAuth()
  const navigate = useNavigate()

  const [step, setStep] = useState<Step>('connect')
  const [oncallURL, setOncallURL] = useState('')
  const [apiToken, setApiToken] = useState('')
  const [showToken, setShowToken] = useState(false)
  const [error, setError] = useState('')
  const [preview, setPreview] = useState<OnCallPreviewResponse | null>(null)
  const [result, setResult] = useState<OnCallImportResponse | null>(null)
  const [expandedSections, setExpandedSections] = useState<Record<string, boolean>>({})

  if (currentUser?.role !== 'admin') {
    navigate('/')
    return null
  }

  const toggleSection = (key: string) =>
    setExpandedSections((prev) => ({ ...prev, [key]: !prev[key] }))

  async function handlePreview() {
    setError('')
    if (!oncallURL.trim() || !apiToken.trim()) {
      setError('Both URL and API token are required.')
      return
    }
    setStep('previewing')
    try {
      const data = await previewOnCallMigration({ oncall_url: oncallURL.trim(), api_token: apiToken.trim() })
      setPreview(data)
      setStep('preview')
    } catch (e: unknown) {
      setError(extractError(e))
      setStep('connect')
    }
  }

  async function handleImport() {
    if (!preview) return
    setError('')
    setStep('importing')
    try {
      const data = await importOnCallMigration({ oncall_url: oncallURL.trim(), api_token: apiToken.trim() })
      setResult(data)
      setStep('done')
    } catch (e: unknown) {
      setError(extractError(e))
      setStep('preview')
    }
  }

  return (
    <div className="flex-1 overflow-auto">
      {/* Header */}
      <div className="border-b border-border bg-surface-primary px-6 py-4">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-text-primary">Migrations</h1>
            <p className="mt-1 text-sm text-text-secondary">
              Import your existing on-call configuration from Grafana OnCall OSS.
            </p>
          </div>
        </div>
      </div>

      <div className="px-6 py-6 max-w-3xl">
        {/* Step indicator */}
        <StepIndicator current={step} />

        {error && (
          <div className="mt-4 flex items-start gap-3 rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-800">
            <AlertTriangle className="mt-0.5 h-4 w-4 flex-shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {/* ── Step 1: Connect ────────────────────────────────────────────────── */}
        {(step === 'connect' || step === 'previewing') && (
          <div className="mt-6 rounded-xl border border-border bg-surface-primary p-6">
            <h2 className="text-lg font-semibold text-text-primary">Connect to Grafana OnCall</h2>
            <p className="mt-1 text-sm text-text-secondary">
              Enter your Grafana OnCall instance URL and API token. Your data is fetched
              directly — nothing is stored until you confirm the import.
            </p>

            <div className="mt-5 space-y-4">
              <div>
                <label className="block text-sm font-medium text-text-primary mb-1">
                  Grafana OnCall URL
                </label>
                <input
                  type="url"
                  value={oncallURL}
                  onChange={(e) => setOncallURL(e.target.value)}
                  placeholder="https://grafana.yourcompany.com"
                  className="w-full rounded-lg border border-border bg-surface-secondary px-3 py-2 text-sm text-text-primary placeholder:text-text-tertiary focus:outline-none focus:ring-2 focus:ring-brand-accent"
                  disabled={step === 'previewing'}
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-text-primary mb-1">
                  API Token
                </label>
                <div className="relative">
                  <input
                    type={showToken ? 'text' : 'password'}
                    value={apiToken}
                    onChange={(e) => setApiToken(e.target.value)}
                    placeholder="glsa_xxxxxxxxxxxx"
                    className="w-full rounded-lg border border-border bg-surface-secondary px-3 py-2 pr-20 text-sm text-text-primary placeholder:text-text-tertiary focus:outline-none focus:ring-2 focus:ring-brand-accent"
                    disabled={step === 'previewing'}
                  />
                  <button
                    type="button"
                    onClick={() => setShowToken(!showToken)}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-text-secondary hover:text-text-primary"
                  >
                    {showToken ? 'Hide' : 'Show'}
                  </button>
                </div>
                <p className="mt-1 text-xs text-text-tertiary">
                  Find this in Grafana OnCall → Settings → API Tokens → Create.
                </p>
              </div>
            </div>

            <div className="mt-6">
              <Button
                onClick={handlePreview}
                disabled={step === 'previewing'}
                className="w-full sm:w-auto"
              >
                {step === 'previewing' ? 'Fetching preview…' : 'Preview import'}
              </Button>
            </div>
          </div>
        )}

        {/* ── Step 2: Preview ────────────────────────────────────────────────── */}
        {(step === 'preview' || step === 'importing') && preview && (
          <div className="mt-6 space-y-4">
            <div className="rounded-xl border border-border bg-surface-primary p-6">
              <h2 className="text-lg font-semibold text-text-primary">Preview import</h2>
              <p className="mt-1 text-sm text-text-secondary">
                The following will be created in Regen. Review before proceeding.
              </p>

              {/* Summary cards */}
              <div className="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-4">
                <SummaryCard label="Users" count={preview.users.count} />
                <SummaryCard label="Schedules" count={preview.schedules.count} />
                <SummaryCard label="Escalation policies" count={preview.escalation_policies.count} />
                <SummaryCard label="Webhooks" count={preview.webhooks.count} />
              </div>

              {/* Conflicts */}
              {preview.conflicts.length > 0 && (
                <div className="mt-4 rounded-lg border border-amber-200 bg-amber-50 p-4">
                  <div className="flex items-center gap-2 text-sm font-medium text-amber-800">
                    <AlertTriangle className="h-4 w-4" />
                    {preview.conflicts.length} item{preview.conflicts.length !== 1 ? 's' : ''} will be skipped (already exist in Regen)
                  </div>
                  <ul className="mt-2 space-y-1">
                    {preview.conflicts.map((c, i) => (
                      <li key={i} className="text-xs text-amber-700">
                        <span className="font-medium capitalize">{c.type}</span>: {c.name} — {c.reason}
                      </li>
                    ))}
                  </ul>
                </div>
              )}

              {/* Collapsible detail tables */}
              <div className="mt-4 space-y-2">
                <CollapsibleTable
                  title="Users to create"
                  count={preview.users.count}
                  expanded={!!expandedSections['users']}
                  onToggle={() => toggleSection('users')}
                  columns={['Email', 'Name', 'Role']}
                  rows={preview.users.items.map((u) => [u.email, u.name || '—', u.role])}
                />
                <CollapsibleTable
                  title="Schedules to create"
                  count={preview.schedules.count}
                  expanded={!!expandedSections['schedules']}
                  onToggle={() => toggleSection('schedules')}
                  columns={['Name', 'Timezone']}
                  rows={preview.schedules.items.map((s) => [s.name, s.timezone])}
                />
                <CollapsibleTable
                  title="Escalation policies to create"
                  count={preview.escalation_policies.count}
                  expanded={!!expandedSections['policies']}
                  onToggle={() => toggleSection('policies')}
                  columns={['Name']}
                  rows={preview.escalation_policies.items.map((p) => [p.name])}
                />
                <CollapsibleTable
                  title="Webhook URL updates required"
                  count={preview.webhooks.count}
                  expanded={!!expandedSections['webhooks']}
                  onToggle={() => toggleSection('webhooks')}
                  columns={['Integration', 'New Regen URL']}
                  rows={preview.webhooks.items.map((w) => [w.name, w.new_url])}
                />
              </div>
            </div>

            <div className="flex gap-3">
              <Button
                variant="secondary"
                onClick={() => setStep('connect')}
                disabled={step === 'importing'}
              >
                Back
              </Button>
              <Button
                onClick={handleImport}
                disabled={step === 'importing'}
              >
                {step === 'importing' ? 'Importing…' : 'Import everything'}
              </Button>
            </div>
          </div>
        )}

        {/* ── Step 3: Done ───────────────────────────────────────────────────── */}
        {step === 'done' && result && (
          <ImportResultsPanel result={result} />
        )}
      </div>
    </div>
  )
}

// ── Sub-components ────────────────────────────────────────────────────────────

function StepIndicator({ current }: { current: Step }) {
  const steps = [
    { id: 'connect', label: 'Connect' },
    { id: 'preview', label: 'Preview' },
    { id: 'done', label: 'Done' },
  ]
  const active = current === 'connect' || current === 'previewing' ? 0
    : current === 'preview' || current === 'importing' ? 1
    : 2

  return (
    <div className="flex items-center gap-0 mt-2">
      {steps.map((s, i) => (
        <div key={s.id} className="flex items-center">
          <div className={`flex h-8 w-8 items-center justify-center rounded-full text-sm font-medium
            ${i < active ? 'bg-brand-accent text-white' : i === active ? 'border-2 border-brand-accent text-brand-accent' : 'border-2 border-border text-text-tertiary'}
          `}>
            {i < active ? <CheckCircle className="h-4 w-4" /> : i + 1}
          </div>
          {!i && <div className={`w-8 h-0.5 ${active > 0 ? 'bg-brand-accent' : 'bg-border'}`} />}
          {i === 1 && <div className={`w-8 h-0.5 ${active > 1 ? 'bg-brand-accent' : 'bg-border'}`} />}
          {!i && null}
        </div>
      ))}
    </div>
  )
}

function SummaryCard({ label, count }: { label: string; count: number }) {
  return (
    <div className="rounded-lg border border-border bg-surface-secondary p-4 text-center">
      <div className="text-2xl font-bold text-text-primary">{count}</div>
      <div className="mt-1 text-xs text-text-secondary">{label}</div>
    </div>
  )
}

interface CollapsibleTableProps {
  title: string
  count: number
  expanded: boolean
  onToggle: () => void
  columns: string[]
  rows: string[][]
}

function CollapsibleTable({ title, count, expanded, onToggle, columns, rows }: CollapsibleTableProps) {
  if (count === 0) return null
  return (
    <div className="rounded-lg border border-border overflow-hidden">
      <button
        className="flex w-full items-center justify-between px-4 py-3 bg-surface-secondary text-sm font-medium text-text-primary hover:bg-surface-tertiary transition-colors"
        onClick={onToggle}
      >
        <span>{title} ({count})</span>
        {expanded ? <ChevronDown className="h-4 w-4 text-text-secondary" /> : <ChevronRight className="h-4 w-4 text-text-secondary" />}
      </button>
      {expanded && (
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead className="bg-surface-tertiary">
              <tr>
                {columns.map((col) => (
                  <th key={col} className="px-4 py-2 text-left font-medium text-text-secondary">{col}</th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {rows.map((row, i) => (
                <tr key={i} className="hover:bg-surface-secondary">
                  {row.map((cell, j) => (
                    <td key={j} className="px-4 py-2 text-text-primary font-mono">{cell}</td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

function ImportResultsPanel({ result }: { result: OnCallImportResponse }) {
  const [copiedToken, setCopiedToken] = useState<string | null>(null)
  const [copiedWebhook, setCopiedWebhook] = useState<string | null>(null)
  const navigate = useNavigate()

  function copyText(text: string, key: string, setCopied: (k: string | null) => void) {
    navigator.clipboard.writeText(text).then(() => {
      setCopied(key)
      setTimeout(() => setCopied(null), 2000)
    })
  }

  function copyAllTokensCSV(tokens: UserSetupToken[]) {
    const csv = ['Email,Setup Link', ...tokens.map((t) =>
      `${t.email},${window.location.origin}/setup?token=${t.setup_token}`
    )].join('\n')
    navigator.clipboard.writeText(csv)
  }

  return (
    <div className="mt-6 space-y-6">
      {/* Success banner */}
      <div className="flex items-start gap-4 rounded-xl border border-green-200 bg-green-50 p-5">
        <CheckCircle className="mt-0.5 h-6 w-6 flex-shrink-0 text-green-600" />
        <div>
          <div className="text-base font-semibold text-green-900">Import complete</div>
          <div className="mt-1 text-sm text-green-800">
            {result.imported.users} user{result.imported.users !== 1 ? 's' : ''},{' '}
            {result.imported.schedules} schedule{result.imported.schedules !== 1 ? 's' : ''},{' '}
            {result.imported.escalation_policies} escalation
            {result.imported.escalation_policies !== 1 ? ' policies' : ' policy'} created.
          </div>
          {result.conflicts.length > 0 && (
            <div className="mt-1 text-sm text-green-700">
              {result.conflicts.length} item{result.conflicts.length !== 1 ? 's' : ''} skipped (already existed).
            </div>
          )}
        </div>
      </div>

      {/* Webhook update table */}
      {result.webhooks.length > 0 && (
        <div className="rounded-xl border border-border bg-surface-primary p-5">
          <h3 className="text-base font-semibold text-text-primary">Update webhook URLs</h3>
          <p className="mt-1 text-sm text-text-secondary">
            Your alerts won't reach Regen until you update these URLs in Grafana Alertmanager
            or your monitoring tool.
          </p>
          <div className="mt-4 overflow-x-auto rounded-lg border border-border">
            <table className="w-full text-sm">
              <thead className="bg-surface-secondary">
                <tr>
                  <th className="px-4 py-2 text-left text-xs font-medium text-text-secondary">Integration</th>
                  <th className="px-4 py-2 text-left text-xs font-medium text-text-secondary">Old OnCall URL</th>
                  <th className="px-4 py-2 text-left text-xs font-medium text-text-secondary">New Regen URL</th>
                  <th className="px-4 py-2" />
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {result.webhooks.map((w, i) => (
                  <tr key={i} className="hover:bg-surface-secondary">
                    <td className="px-4 py-3 font-medium text-text-primary">{w.name}</td>
                    <td className="px-4 py-3 font-mono text-xs text-text-secondary truncate max-w-[200px]">{w.old_url}</td>
                    <td className="px-4 py-3 font-mono text-xs text-brand-accent">{w.new_url}</td>
                    <td className="px-4 py-3">
                      <button
                        onClick={() => copyText(w.new_url, w.new_url, setCopiedWebhook)}
                        className="flex items-center gap-1 text-xs text-text-secondary hover:text-text-primary transition-colors"
                        title="Copy new URL"
                      >
                        <Copy className="h-3.5 w-3.5" />
                        {copiedWebhook === w.new_url ? 'Copied' : 'Copy'}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* User setup tokens */}
      {result.setup_tokens && result.setup_tokens.length > 0 && (
        <div className="rounded-xl border border-border bg-surface-primary p-5">
          <div className="flex items-center justify-between">
            <h3 className="text-base font-semibold text-text-primary">Share setup links</h3>
            <button
              onClick={() => copyAllTokensCSV(result.setup_tokens)}
              className="text-xs text-text-secondary hover:text-text-primary transition-colors flex items-center gap-1"
            >
              <Copy className="h-3.5 w-3.5" />
              Copy all as CSV
            </button>
          </div>
          <p className="mt-1 text-sm text-text-secondary">
            Each imported user needs to set their own password. Share these one-time links.
          </p>
          <div className="mt-4 overflow-x-auto rounded-lg border border-border">
            <table className="w-full text-sm">
              <thead className="bg-surface-secondary">
                <tr>
                  <th className="px-4 py-2 text-left text-xs font-medium text-text-secondary">Email</th>
                  <th className="px-4 py-2 text-left text-xs font-medium text-text-secondary">Setup link</th>
                  <th className="px-4 py-2" />
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {result.setup_tokens.map((t, i) => {
                  const setupLink = `${window.location.origin}/setup?token=${t.setup_token}`
                  return (
                    <tr key={i} className="hover:bg-surface-secondary">
                      <td className="px-4 py-3 text-text-primary">{t.email}</td>
                      <td className="px-4 py-3 font-mono text-xs text-text-secondary truncate max-w-[300px]">{setupLink}</td>
                      <td className="px-4 py-3">
                        <button
                          onClick={() => copyText(setupLink, t.email, setCopiedToken)}
                          className="flex items-center gap-1 text-xs text-text-secondary hover:text-text-primary transition-colors"
                        >
                          <Copy className="h-3.5 w-3.5" />
                          {copiedToken === t.email ? 'Copied' : 'Copy'}
                        </button>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Next steps checklist */}
      <div className="rounded-xl border border-border bg-surface-primary p-5">
        <h3 className="text-base font-semibold text-text-primary">Next steps</h3>
        <ul className="mt-3 space-y-2 text-sm text-text-secondary">
          {result.webhooks.length > 0 && (
            <li className="flex items-start gap-2">
              <span className="mt-0.5 flex-shrink-0 text-brand-accent">→</span>
              Update webhook URLs in Grafana Alertmanager or your monitoring tool (table above)
            </li>
          )}
          {result.setup_tokens && result.setup_tokens.length > 0 && (
            <li className="flex items-start gap-2">
              <span className="mt-0.5 flex-shrink-0 text-brand-accent">→</span>
              Share setup links with your team members so they can set their passwords
            </li>
          )}
          {result.imported.schedules > 0 && (
            <li className="flex items-start gap-2">
              <span className="mt-0.5 flex-shrink-0 text-brand-accent">→</span>
              Verify your on-call schedules look correct in the{' '}
              <button onClick={() => navigate('/on-call')} className="text-brand-accent underline hover:no-underline">
                On-call section
              </button>
            </li>
          )}
          <li className="flex items-start gap-2">
            <span className="mt-0.5 flex-shrink-0 text-brand-accent">→</span>
            Send a test alert end-to-end to confirm alerts route correctly
          </li>
        </ul>
      </div>

      {/* CTA buttons */}
      <div className="flex gap-3">
        <Button onClick={() => navigate('/on-call')}>Go to schedules</Button>
        <Button variant="secondary" onClick={() => navigate('/settings/users')}>Go to users</Button>
      </div>

      {/* Skipped / conflicts summary */}
      {(result.skipped?.length > 0 || result.conflicts?.length > 0) && (
        <SkippedConflictsPanel skipped={result.skipped} conflicts={result.conflicts} />
      )}
    </div>
  )
}

function SkippedConflictsPanel({
  skipped,
  conflicts,
}: {
  skipped: SkippedItem[]
  conflicts: ConflictItem[]
}) {
  const [expanded, setExpanded] = useState(false)
  const total = (skipped?.length ?? 0) + (conflicts?.length ?? 0)
  if (total === 0) return null
  return (
    <div className="rounded-xl border border-border bg-surface-primary overflow-hidden">
      <button
        className="flex w-full items-center justify-between px-5 py-4 text-sm font-medium text-text-secondary hover:bg-surface-secondary transition-colors"
        onClick={() => setExpanded(!expanded)}
      >
        <span>{total} item{total !== 1 ? 's' : ''} skipped or in conflict</span>
        {expanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
      </button>
      {expanded && (
        <div className="px-5 pb-4 space-y-1 text-xs text-text-secondary">
          {conflicts?.map((c, i) => (
            <div key={i}><span className="font-medium capitalize text-amber-600">{c.type}</span>: {c.name} — {c.reason}</div>
          ))}
          {skipped?.map((s, i) => (
            <div key={i}><span className="font-medium capitalize text-text-tertiary">{s.type}</span>: {s.name} — {s.reason}</div>
          ))}
        </div>
      )}
    </div>
  )
}

function extractError(e: unknown): string {
  if (e instanceof Error) return e.message
  return String(e)
}
