import { useState } from 'react'
import { Loader2, AlertCircle, CheckCircle } from 'lucide-react'
import { importOnCallMigration } from '../../api/migrations'
import { createSchedule, createLayer, COMMON_TIMEZONES } from '../../api/schedules'
import { PagerDutyImportPanel } from '../migrations/PagerDutyImportPanel'

interface Props {
  hasSchedule: boolean
  onComplete: () => void
  onSkip: () => void
}

type Mode = 'choose' | 'grafana' | 'pagerduty' | 'manual'

export function WizardStepSchedule({ hasSchedule, onComplete, onSkip }: Props) {
  const [mode, setMode] = useState<Mode>('choose')

  if (hasSchedule) {
    return (
      <div className="space-y-4">
        <div className="flex items-start gap-3 rounded-lg bg-green-50 border border-green-200 px-4 py-3">
          <CheckCircle className="w-5 h-5 text-green-600 flex-shrink-0 mt-0.5" />
          <div>
            <p className="text-sm font-medium text-green-700">You already have a schedule</p>
            <p className="text-xs text-green-600 mt-0.5">Your on-call rotation is ready to go.</p>
          </div>
        </div>
        <div className="flex justify-end">
          <button
            onClick={onComplete}
            className="px-4 py-2 rounded-lg bg-brand-primary hover:bg-brand-primary-hover text-white text-sm font-medium transition-colors"
          >
            Continue →
          </button>
        </div>
      </div>
    )
  }

  if (mode === 'grafana') return <GrafanaImport onComplete={onComplete} onBack={() => setMode('choose')} />
  if (mode === 'manual') return <ManualSchedule onComplete={onComplete} onBack={() => setMode('choose')} />

  if (mode === 'pagerduty') return (
    <div className="space-y-4">
      <div className="flex items-center gap-3 mb-2">
        <button onClick={() => setMode('choose')} className="text-sm text-text-tertiary hover:text-text-secondary transition-colors">
          ← Back
        </button>
        <h3 className="text-sm font-semibold text-text-primary">Import from PagerDuty</h3>
      </div>
      <PagerDutyImportPanel onComplete={onComplete} />
    </div>
  )

  return (
    <div className="space-y-4">
      <p className="text-sm text-text-secondary">
        Set up your first on-call schedule so alerts know who to page.
      </p>

      <div className="grid gap-3">
        <button
          onClick={() => setMode('grafana')}
          className="flex items-start gap-3 text-left rounded-lg border border-border bg-surface-primary hover:border-brand-primary hover:bg-surface-secondary/50 px-4 py-3 transition-colors"
        >
          <div className="mt-0.5 w-8 h-8 rounded-lg bg-orange-100 flex items-center justify-center flex-shrink-0">
            <span className="text-orange-700 text-xs font-bold">GC</span>
          </div>
          <div>
            <p className="text-sm font-medium text-text-primary">Import from Grafana OnCall</p>
            <p className="text-xs text-text-tertiary mt-0.5">Migrate schedules, escalation policies, and users</p>
          </div>
        </button>

        <button
          onClick={() => setMode('pagerduty')}
          className="flex items-start gap-3 text-left rounded-lg border border-border bg-surface-primary hover:border-brand-primary hover:bg-surface-secondary/50 px-4 py-3 transition-colors"
        >
          <div className="mt-0.5 w-8 h-8 rounded-lg bg-green-100 flex items-center justify-center flex-shrink-0">
            <span className="text-green-700 text-xs font-bold">PD</span>
          </div>
          <div>
            <p className="text-sm font-medium text-text-primary">Import from PagerDuty</p>
            <p className="text-xs text-text-tertiary mt-0.5">Migrate schedules and escalation policies</p>
          </div>
        </button>

        <div className="flex items-start gap-3 rounded-lg border border-border bg-surface-secondary/40 px-4 py-3 opacity-60 cursor-not-allowed">
          <div className="mt-0.5 w-8 h-8 rounded-lg bg-blue-100 flex items-center justify-center flex-shrink-0">
            <span className="text-blue-700 text-xs font-bold">OG</span>
          </div>
          <div>
            <p className="text-sm font-medium text-text-primary">Opsgenie</p>
            <p className="text-xs text-text-tertiary mt-0.5">Coming soon</p>
          </div>
        </div>

        <button
          onClick={() => setMode('manual')}
          className="flex items-start gap-3 text-left rounded-lg border border-border bg-surface-primary hover:border-brand-primary hover:bg-surface-secondary/50 px-4 py-3 transition-colors"
        >
          <div className="mt-0.5 w-8 h-8 rounded-lg bg-surface-secondary flex items-center justify-center flex-shrink-0">
            <span className="text-text-tertiary text-xs font-bold">+</span>
          </div>
          <div>
            <p className="text-sm font-medium text-text-primary">Create manually</p>
            <p className="text-xs text-text-tertiary mt-0.5">Set up a simple rotation from scratch</p>
          </div>
        </button>
      </div>

      <div className="flex justify-end">
        <button onClick={onSkip} className="text-sm text-text-tertiary hover:text-text-secondary transition-colors">
          Skip for now →
        </button>
      </div>
    </div>
  )
}

function GrafanaImport({ onComplete, onBack }: { onComplete: () => void; onBack: () => void }) {
  const [url, setUrl] = useState('')
  const [token, setToken] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [imported, setImported] = useState<{ schedules: number; users: number } | null>(null)

  async function handleImport() {
    if (!url || !token) return
    setLoading(true)
    setError('')
    try {
      const result = await importOnCallMigration({ oncall_url: url, api_token: token })
      setImported({ schedules: result.imported.schedules, users: result.imported.users })
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Import failed')
    } finally {
      setLoading(false)
    }
  }

  const inputClass = 'w-full h-9 rounded-lg bg-surface-secondary border border-border text-text-primary text-sm px-3 placeholder-text-tertiary focus:outline-none focus:ring-2 focus:ring-brand-primary'

  if (imported) {
    return (
      <div className="space-y-4">
        <div className="flex items-start gap-3 rounded-lg bg-green-50 border border-green-200 px-4 py-3">
          <CheckCircle className="w-5 h-5 text-green-600 flex-shrink-0 mt-0.5" />
          <div>
            <p className="text-sm font-medium text-green-700">Import complete</p>
            <p className="text-xs text-green-600 mt-0.5">
              {imported.schedules} schedule{imported.schedules !== 1 ? 's' : ''} and {imported.users} user{imported.users !== 1 ? 's' : ''} imported
            </p>
          </div>
        </div>
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
      <p className="text-sm text-text-secondary">Enter your Grafana OnCall instance URL and API token.</p>

      <div>
        <label className="block text-xs font-medium text-text-secondary mb-1">Grafana OnCall URL</label>
        <input type="url" value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://your-grafana.example.com" className={inputClass} />
      </div>
      <div>
        <label className="block text-xs font-medium text-text-secondary mb-1">API Token</label>
        <input type="password" value={token} onChange={(e) => setToken(e.target.value)} placeholder="glsa_..." className={inputClass} />
      </div>

      {error && (
        <div className="flex items-start gap-2 rounded-lg bg-red-50 border border-red-200 px-3 py-2">
          <AlertCircle className="w-4 h-4 text-red-600 flex-shrink-0 mt-0.5" />
          <span className="text-sm text-red-700">{error}</span>
        </div>
      )}

      <div className="flex items-center justify-between pt-2">
        <button onClick={onBack} className="px-3 py-1.5 text-sm text-text-secondary hover:text-text-primary transition-colors">← Back</button>
        <button
          onClick={handleImport}
          disabled={!url || !token || loading}
          className="flex items-center gap-2 px-4 py-2 rounded-lg bg-brand-primary hover:bg-brand-primary-hover disabled:opacity-50 text-white text-sm font-medium transition-colors"
        >
          {loading && <Loader2 className="w-4 h-4 animate-spin" />}
          Import →
        </button>
      </div>
    </div>
  )
}

function ManualSchedule({ onComplete, onBack }: { onComplete: () => void; onBack: () => void }) {
  const [name, setName] = useState('')
  const [timezone, setTimezone] = useState('UTC')
  const [rotationType, setRotationType] = useState<'daily' | 'weekly'>('weekly')
  const [participantName, setParticipantName] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  async function handleCreate() {
    if (!name) return
    setSaving(true)
    setError('')
    try {
      const schedule = await createSchedule({ name, timezone })
      if (participantName) {
        await createLayer(schedule.id, {
          name: 'Primary',
          rotation_type: rotationType,
          participants: [{ user_name: participantName, order_index: 0 }],
        })
      }
      onComplete()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to create schedule')
    } finally {
      setSaving(false)
    }
  }

  const inputClass = 'w-full h-9 rounded-lg bg-surface-secondary border border-border text-text-primary text-sm px-3 placeholder-text-tertiary focus:outline-none focus:ring-2 focus:ring-brand-primary'

  return (
    <div className="space-y-4">
      <div>
        <label className="block text-xs font-medium text-text-secondary mb-1">Schedule Name <span className="text-red-500">*</span></label>
        <input type="text" value={name} onChange={(e) => setName(e.target.value)} placeholder="Primary On-Call" className={inputClass} />
      </div>

      <div>
        <label className="block text-xs font-medium text-text-secondary mb-1">Timezone</label>
        <select value={timezone} onChange={(e) => setTimezone(e.target.value)} className={inputClass}>
          {COMMON_TIMEZONES.map((tz) => <option key={tz} value={tz}>{tz}</option>)}
        </select>
      </div>

      <div>
        <label className="block text-xs font-medium text-text-secondary mb-1">Rotation</label>
        <div className="flex gap-2">
          {(['daily', 'weekly'] as const).map((t) => (
            <button
              key={t}
              onClick={() => setRotationType(t)}
              className={`px-3 py-1.5 rounded-lg text-sm border transition-colors capitalize ${rotationType === t ? 'border-brand-primary bg-brand-primary/10 text-brand-primary' : 'border-border text-text-secondary hover:bg-surface-secondary'}`}
            >
              {t}
            </button>
          ))}
        </div>
      </div>

      <div>
        <label className="block text-xs font-medium text-text-secondary mb-1">First Participant (optional)</label>
        <input type="text" value={participantName} onChange={(e) => setParticipantName(e.target.value)} placeholder="username or email" className={inputClass} />
      </div>

      {error && (
        <div className="flex items-start gap-2 rounded-lg bg-red-50 border border-red-200 px-3 py-2">
          <AlertCircle className="w-4 h-4 text-red-600 flex-shrink-0 mt-0.5" />
          <span className="text-sm text-red-700">{error}</span>
        </div>
      )}

      <div className="flex items-center justify-between pt-2">
        <button onClick={onBack} className="px-3 py-1.5 text-sm text-text-secondary hover:text-text-primary transition-colors">← Back</button>
        <button
          onClick={handleCreate}
          disabled={!name || saving}
          className="flex items-center gap-2 px-4 py-2 rounded-lg bg-brand-primary hover:bg-brand-primary-hover disabled:opacity-50 text-white text-sm font-medium transition-colors"
        >
          {saving && <Loader2 className="w-4 h-4 animate-spin" />}
          Create Schedule →
        </button>
      </div>
    </div>
  )
}
