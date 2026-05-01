import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { CheckCircle, AlertCircle, Eye, EyeOff, Loader2, Save } from 'lucide-react'
import { useAuth } from '../hooks/useAuth'
import { Button } from '../components/ui/Button'
import {
  getSystemSettings,
  updateSystemSettings,
  updateTelemetrySettings,
  testOpenAIKey,
  SystemSettings,
} from '../api/settings'

const COMMON_TIMEZONES = [
  'UTC',
  'America/New_York',
  'America/Chicago',
  'America/Denver',
  'America/Los_Angeles',
  'America/Toronto',
  'America/Vancouver',
  'America/Sao_Paulo',
  'Europe/London',
  'Europe/Paris',
  'Europe/Berlin',
  'Europe/Amsterdam',
  'Europe/Stockholm',
  'Europe/Madrid',
  'Europe/Rome',
  'Asia/Kolkata',
  'Asia/Dubai',
  'Asia/Singapore',
  'Asia/Tokyo',
  'Asia/Seoul',
  'Asia/Shanghai',
  'Australia/Sydney',
  'Australia/Melbourne',
  'Pacific/Auckland',
]

export function SystemSettingsPage() {
  const { user: currentUser } = useAuth()
  const navigate = useNavigate()

  const [settings, setSettings] = useState<SystemSettings | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  // Instance fields
  const [instanceName, setInstanceName] = useState('')
  const [timezone, setTimezone] = useState('UTC')
  const [savingInstance, setSavingInstance] = useState(false)
  const [instanceSaved, setInstanceSaved] = useState(false)

  // AI fields
  const [openaiKey, setOpenaiKey] = useState('')
  const [showKey, setShowKey] = useState(false)
  const [testingKey, setTestingKey] = useState(false)
  const [testResult, setTestResult] = useState<{ ok: boolean; message: string } | null>(null)
  const [savingAI, setSavingAI] = useState(false)
  const [aiSaved, setAISaved] = useState(false)

  // Telemetry
  const [savingTelemetry, setSavingTelemetry] = useState(false)

  useEffect(() => {
    if (currentUser && currentUser.role !== 'admin') {
      navigate('/')
    }
  }, [currentUser, navigate])

  useEffect(() => {
    load()
  }, [])

  async function load() {
    setLoading(true)
    try {
      const data = await getSystemSettings()
      setSettings(data)
      setInstanceName(data.instance_name || '')
      setTimezone(data.timezone || 'UTC')
      setError('')
    } catch {
      setError('Failed to load system settings')
    } finally {
      setLoading(false)
    }
  }

  async function handleSaveInstance() {
    setSavingInstance(true)
    setInstanceSaved(false)
    try {
      await updateSystemSettings({ instance_name: instanceName, timezone })
      setInstanceSaved(true)
      setTimeout(() => setInstanceSaved(false), 3000)
    } catch {
      setError('Failed to save instance settings')
    } finally {
      setSavingInstance(false)
    }
  }

  async function handleTestKey() {
    if (!openaiKey.trim()) return
    setTestingKey(true)
    setTestResult(null)
    try {
      const res = await testOpenAIKey(openaiKey.trim())
      setTestResult({ ok: res.ok, message: res.ok ? 'Key is valid — connection successful.' : (res.error ?? 'Key validation failed.') })
    } catch {
      setTestResult({ ok: false, message: 'Could not reach the server to test the key.' })
    } finally {
      setTestingKey(false)
    }
  }

  async function handleSaveAI() {
    if (!openaiKey.trim()) return
    setSavingAI(true)
    setAISaved(false)
    try {
      await updateSystemSettings({ openai_api_key: openaiKey.trim() })
      setAISaved(true)
      setOpenaiKey('')
      setTestResult(null)
      await load() // refresh to show new masked key
      setTimeout(() => setAISaved(false), 3000)
    } catch {
      setError('Failed to save AI settings')
    } finally {
      setSavingAI(false)
    }
  }

  async function handleClearAI() {
    setSavingAI(true)
    try {
      await updateSystemSettings({ openai_api_key: '' })
      setOpenaiKey('')
      setTestResult(null)
      await load()
    } catch {
      setError('Failed to remove AI key')
    } finally {
      setSavingAI(false)
    }
  }

  async function handleToggleTelemetry(enabled: boolean) {
    setSavingTelemetry(true)
    try {
      await updateTelemetrySettings(enabled)
      await load()
    } catch {
      setError('Failed to update telemetry preference')
    } finally {
      setSavingTelemetry(false)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 className="w-6 h-6 animate-spin text-text-tertiary" />
      </div>
    )
  }

  return (
    <div className="max-w-2xl mx-auto py-8 px-6 space-y-10">
      <div>
        <h1 className="text-2xl font-semibold text-text-primary">System Settings</h1>
        <p className="mt-1 text-sm text-text-secondary">Configure your Fluidify Regen instance.</p>
      </div>

      {error && (
        <div className="flex items-center gap-2 p-3 rounded-lg bg-red-50 border border-red-200 text-red-700 text-sm">
          <AlertCircle className="w-4 h-4 flex-shrink-0" />
          {error}
        </div>
      )}

      {/* ── Instance ─────────────────────────────────────────────────────── */}
      <section className="bg-surface-primary border border-border-primary rounded-xl p-6 space-y-5">
        <div>
          <h2 className="text-base font-semibold text-text-primary">Instance</h2>
          <p className="text-sm text-text-secondary mt-0.5">Display name and timezone for your organization.</p>
        </div>

        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-text-primary mb-1">Instance name</label>
            <input
              type="text"
              value={instanceName}
              onChange={(e) => setInstanceName(e.target.value)}
              placeholder="My Company"
              className="w-full px-3 py-2 rounded-lg border border-border-primary bg-surface-secondary text-text-primary text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary"
            />
            <p className="mt-1 text-xs text-text-tertiary">Shown in the sidebar and notification subjects.</p>
          </div>

          <div>
            <label className="block text-sm font-medium text-text-primary mb-1">Default timezone</label>
            <select
              value={timezone}
              onChange={(e) => setTimezone(e.target.value)}
              className="w-full px-3 py-2 rounded-lg border border-border-primary bg-surface-secondary text-text-primary text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary"
            >
              {COMMON_TIMEZONES.map((tz) => (
                <option key={tz} value={tz}>{tz}</option>
              ))}
            </select>
            <p className="mt-1 text-xs text-text-tertiary">Used for on-call schedule display and report generation.</p>
          </div>
        </div>

        <div className="flex items-center gap-3">
          <Button onClick={handleSaveInstance} disabled={savingInstance} size="sm">
            {savingInstance ? <Loader2 className="w-4 h-4 animate-spin mr-1" /> : <Save className="w-4 h-4 mr-1" />}
            Save
          </Button>
          {instanceSaved && (
            <span className="flex items-center gap-1 text-sm text-green-600">
              <CheckCircle className="w-4 h-4" /> Saved
            </span>
          )}
        </div>
      </section>

      {/* ── AI / OpenAI ──────────────────────────────────────────────────── */}
      <section className="bg-surface-primary border border-border-primary rounded-xl p-6 space-y-5">
        <div>
          <h2 className="text-base font-semibold text-text-primary">AI — OpenAI</h2>
          <p className="text-sm text-text-secondary mt-0.5">
            Enable AI-powered incident summaries and post-mortem drafts. Your key is stored encrypted and never logged.
          </p>
        </div>

        {/* Current key status */}
        {settings?.ai_key_configured ? (
          <div className="flex items-center justify-between p-3 rounded-lg bg-green-50 border border-green-200">
            <div className="flex items-center gap-2 text-sm text-green-700">
              <CheckCircle className="w-4 h-4" />
              <span>OpenAI key configured — ends in <code className="font-mono font-semibold">...{settings.ai_key_last4}</code></span>
            </div>
            <button
              onClick={handleClearAI}
              disabled={savingAI}
              className="text-xs text-red-600 hover:text-red-800 font-medium"
            >
              Remove key
            </button>
          </div>
        ) : (
          <div className="flex items-center gap-2 p-3 rounded-lg bg-amber-50 border border-amber-200 text-amber-700 text-sm">
            <AlertCircle className="w-4 h-4 flex-shrink-0" />
            No OpenAI key configured — AI features are disabled.
          </div>
        )}

        {/* Key input */}
        <div>
          <label className="block text-sm font-medium text-text-primary mb-1">
            {settings?.ai_key_configured ? 'Replace API key' : 'Add API key'}
          </label>
          <div className="relative">
            <input
              type={showKey ? 'text' : 'password'}
              value={openaiKey}
              onChange={(e) => { setOpenaiKey(e.target.value); setTestResult(null) }}
              placeholder="sk-..."
              className="w-full px-3 py-2 pr-10 rounded-lg border border-border-primary bg-surface-secondary text-text-primary text-sm font-mono focus:outline-none focus:ring-2 focus:ring-brand-primary"
            />
            <button
              type="button"
              onClick={() => setShowKey(!showKey)}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-text-tertiary hover:text-text-secondary"
            >
              {showKey ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
            </button>
          </div>
          <p className="mt-1 text-xs text-text-tertiary">
            Get your key from{' '}
            <span className="font-medium text-text-secondary">platform.openai.com → API keys</span>.
            You provide the key; we don&apos;t proxy usage.
          </p>
        </div>

        {/* Test result */}
        {testResult && (
          <div className={`flex items-center gap-2 p-3 rounded-lg text-sm border ${
            testResult.ok
              ? 'bg-green-50 border-green-200 text-green-700'
              : 'bg-red-50 border-red-200 text-red-700'
          }`}>
            {testResult.ok
              ? <CheckCircle className="w-4 h-4 flex-shrink-0" />
              : <AlertCircle className="w-4 h-4 flex-shrink-0" />}
            {testResult.message}
          </div>
        )}

        <div className="flex items-center gap-3">
          <Button
            variant="secondary"
            size="sm"
            onClick={handleTestKey}
            disabled={!openaiKey.trim() || testingKey}
          >
            {testingKey && <Loader2 className="w-4 h-4 animate-spin mr-1" />}
            Test key
          </Button>
          <Button
            size="sm"
            onClick={handleSaveAI}
            disabled={!openaiKey.trim() || savingAI}
          >
            {savingAI ? <Loader2 className="w-4 h-4 animate-spin mr-1" /> : <Save className="w-4 h-4 mr-1" />}
            Save key
          </Button>
          {aiSaved && (
            <span className="flex items-center gap-1 text-sm text-green-600">
              <CheckCircle className="w-4 h-4" /> Saved
            </span>
          )}
        </div>
      </section>

      {/* ── Telemetry ────────────────────────────────────────────────────── */}
      <section className="bg-surface-primary border border-border-primary rounded-xl p-6 space-y-4">
        <div>
          <h2 className="text-base font-semibold text-text-primary">Telemetry</h2>
          <p className="text-sm text-text-secondary mt-0.5">
            Anonymous usage statistics help Fluidify understand adoption and improve the product.
            No incident content, hostnames, credentials, or PII is ever collected.
          </p>
        </div>

        {settings?.telemetry_env_lock ? (
          <div className="flex items-center gap-2 p-3 rounded-lg bg-amber-50 border border-amber-200 text-amber-700 text-sm">
            <AlertCircle className="w-4 h-4 flex-shrink-0" />
            Telemetry is disabled by the <code className="font-mono font-semibold">REGEN_NO_TELEMETRY</code> environment variable.
          </div>
        ) : (
          <label className="flex items-start gap-3 cursor-pointer">
            <div className="mt-0.5">
              <input
                type="checkbox"
                className="sr-only"
                checked={settings?.telemetry_enabled ?? true}
                disabled={savingTelemetry}
                onChange={(e) => handleToggleTelemetry(e.target.checked)}
              />
              <div
                onClick={() => !savingTelemetry && handleToggleTelemetry(!(settings?.telemetry_enabled ?? true))}
                className={`w-10 h-6 rounded-full relative transition-colors cursor-pointer ${
                  (settings?.telemetry_enabled ?? true) ? 'bg-brand-primary' : 'bg-border-primary'
                } ${savingTelemetry ? 'opacity-50 cursor-not-allowed' : ''}`}
              >
                <span className={`absolute top-1 w-4 h-4 rounded-full bg-white shadow transition-transform ${
                  (settings?.telemetry_enabled ?? true) ? 'translate-x-5' : 'translate-x-1'
                }`} />
              </div>
            </div>
            <div>
              <div className="text-sm font-medium text-text-primary">Share anonymous usage data</div>
              <div className="text-xs text-text-tertiary mt-0.5">
                Sends daily aggregate stats: page visit counts, features enabled, instance size.
                Permanently disable via <code className="font-mono">REGEN_NO_TELEMETRY=1</code>.
              </div>
            </div>
          </label>
        )}
      </section>
    </div>
  )
}
