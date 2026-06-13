import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { CheckCircle, AlertCircle, Eye, EyeOff, Loader2, Save, ExternalLink, Tag } from 'lucide-react'
import { getHealth } from '../api/health'
import { useAuth } from '../hooks/useAuth'
import { Button } from '../components/ui/Button'
import {
  getSystemSettings,
  updateSystemSettings,
  updateTelemetrySettings,
  testAIKey,
  AIProvider,
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

  const [appVersion, setAppVersion] = useState<string | null>(null)
  const [settings, setSettings] = useState<SystemSettings | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  // Instance fields
  const [instanceName, setInstanceName] = useState('')
  const [timezone, setTimezone] = useState('UTC')
  const [savingInstance, setSavingInstance] = useState(false)
  const [instanceSaved, setInstanceSaved] = useState(false)

  // AI fields
  const [aiProvider, setAIProvider] = useState<AIProvider>('openai')
  const [openaiKey, setOpenaiKey] = useState('')
  const [anthropicKey, setAnthropicKey] = useState('')
  const [ollamaURL, setOllamaURL] = useState('')
  const [ollamaModel, setOllamaModel] = useState('')
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
    getHealth().then((d) => setAppVersion(d.version)).catch(() => {})
  }, [])

  async function load() {
    setLoading(true)
    try {
      const data = await getSystemSettings()
      setSettings(data)
      setInstanceName(data.instance_name || '')
      setTimezone(data.timezone || 'UTC')
      setAIProvider((data.ai_provider as AIProvider) || 'openai')
      setOllamaURL(data.ollama_base_url || '')
      setOllamaModel(data.ollama_model || '')
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

  function aiTestPayload() {
    if (aiProvider === 'anthropic') return { provider: aiProvider as AIProvider, api_key: anthropicKey.trim() }
    if (aiProvider === 'ollama') return { provider: aiProvider as AIProvider, ollama_base_url: ollamaURL.trim() }
    return { provider: aiProvider as AIProvider, api_key: openaiKey.trim() }
  }

  function aiSavePayload() {
    if (aiProvider === 'anthropic') return { ai_provider: aiProvider, anthropic_api_key: anthropicKey.trim() }
    if (aiProvider === 'ollama') return { ai_provider: aiProvider, ollama_base_url: ollamaURL.trim(), ollama_model: ollamaModel.trim() || 'llama3' }
    return { ai_provider: aiProvider, openai_api_key: openaiKey.trim() }
  }

  function aiInputValid() {
    if (aiProvider === 'anthropic') return anthropicKey.trim().length > 0
    if (aiProvider === 'ollama') return ollamaURL.trim().length > 0
    return openaiKey.trim().length > 0
  }

  async function handleTestKey() {
    if (!aiInputValid()) return
    setTestingKey(true)
    setTestResult(null)
    try {
      const res = await testAIKey(aiTestPayload())
      setTestResult({ ok: res.ok, message: res.ok ? 'Connection successful.' : (res.error ?? 'Validation failed.') })
    } catch {
      setTestResult({ ok: false, message: 'Could not reach the server.' })
    } finally {
      setTestingKey(false)
    }
  }

  async function handleSaveAI() {
    if (!aiInputValid()) return
    setSavingAI(true)
    setAISaved(false)
    try {
      await updateSystemSettings(aiSavePayload())
      setAISaved(true)
      setOpenaiKey('')
      setAnthropicKey('')
      setTestResult(null)
      await load()
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
      await updateSystemSettings({ openai_api_key: '', anthropic_api_key: '', ollama_base_url: '' })
      setOpenaiKey('')
      setAnthropicKey('')
      setOllamaURL('')
      setTestResult(null)
      await load()
    } catch {
      setError('Failed to remove AI configuration')
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

      {/* ── Status Page ──────────────────────────────────────────────────── */}
      <section className="bg-surface-primary border border-border-primary rounded-xl p-6 space-y-3">
        <div>
          <h2 className="text-base font-semibold text-text-primary">Status Page</h2>
          <p className="text-sm text-text-secondary mt-0.5">
            A public, read-only page showing active and recently resolved incidents. No login required.
          </p>
        </div>
        <a
          href="/status"
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-2 text-sm text-brand-primary hover:underline"
        >
          <ExternalLink className="w-4 h-4" />
          {window.location.origin}/status
        </a>
      </section>

      {/* ── AI Provider ──────────────────────────────────────────────────── */}
      <section className="bg-surface-primary border border-border-primary rounded-xl p-6 space-y-5">
        <div>
          <h2 className="text-base font-semibold text-text-primary">AI Provider</h2>
          <p className="text-sm text-text-secondary mt-0.5">
            Enable AI-powered incident summaries and post-mortem drafts. BYO key — your data never leaves your infrastructure unless you choose a cloud provider.
          </p>
        </div>

        {/* Current status */}
        {settings?.ai_key_configured ? (
          <div className="flex items-center justify-between p-3 rounded-lg bg-green-50 border border-green-200">
            <div className="flex items-center gap-2 text-sm text-green-700">
              <CheckCircle className="w-4 h-4" />
              <span>
                {settings.ai_provider === 'ollama'
                  ? 'Ollama configured'
                  : settings.ai_provider === 'anthropic'
                    ? 'Anthropic key configured'
                    : <>OpenAI key configured — ends in <code className="font-mono font-semibold">...{settings.ai_key_last4}</code></>
                }
              </span>
            </div>
            <button onClick={handleClearAI} disabled={savingAI} className="text-xs text-red-600 hover:text-red-800 font-medium">
              Remove
            </button>
          </div>
        ) : (
          <div className="flex items-center gap-2 p-3 rounded-lg bg-amber-50 border border-amber-200 text-amber-700 text-sm">
            <AlertCircle className="w-4 h-4 flex-shrink-0" />
            No AI provider configured — AI features are disabled.
          </div>
        )}

        {/* Provider selector */}
        <div>
          <label className="block text-sm font-medium text-text-primary mb-1">Provider</label>
          <select
            value={aiProvider}
            onChange={(e) => { setAIProvider(e.target.value as AIProvider); setTestResult(null) }}
            className="w-full px-3 py-2 rounded-lg border border-border-primary bg-surface-secondary text-text-primary text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary"
          >
            <option value="openai">OpenAI</option>
            <option value="anthropic">Anthropic</option>
            <option value="ollama">Ollama (local / self-hosted)</option>
          </select>
        </div>

        {/* Per-provider credential inputs */}
        {aiProvider === 'openai' && (
          <div>
            <label className="block text-sm font-medium text-text-primary mb-1">
              {settings?.ai_key_configured && settings.ai_provider === 'openai' ? 'Replace API key' : 'API key'}
            </label>
            <div className="relative">
              <input
                type={showKey ? 'text' : 'password'}
                value={openaiKey}
                onChange={(e) => { setOpenaiKey(e.target.value); setTestResult(null) }}
                placeholder="sk-..."
                className="w-full px-3 py-2 pr-10 rounded-lg border border-border-primary bg-surface-secondary text-text-primary text-sm font-mono focus:outline-none focus:ring-2 focus:ring-brand-primary"
              />
              <button type="button" onClick={() => setShowKey(!showKey)} className="absolute right-3 top-1/2 -translate-y-1/2 text-text-tertiary hover:text-text-secondary">
                {showKey ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
              </button>
            </div>
            <p className="mt-1 text-xs text-text-tertiary">Get your key from <span className="font-medium text-text-secondary">platform.openai.com → API keys</span>.</p>
          </div>
        )}

        {aiProvider === 'anthropic' && (
          <div>
            <label className="block text-sm font-medium text-text-primary mb-1">
              {settings?.ai_key_configured && settings.ai_provider === 'anthropic' ? 'Replace API key' : 'API key'}
            </label>
            <div className="relative">
              <input
                type={showKey ? 'text' : 'password'}
                value={anthropicKey}
                onChange={(e) => { setAnthropicKey(e.target.value); setTestResult(null) }}
                placeholder="sk-ant-..."
                className="w-full px-3 py-2 pr-10 rounded-lg border border-border-primary bg-surface-secondary text-text-primary text-sm font-mono focus:outline-none focus:ring-2 focus:ring-brand-primary"
              />
              <button type="button" onClick={() => setShowKey(!showKey)} className="absolute right-3 top-1/2 -translate-y-1/2 text-text-tertiary hover:text-text-secondary">
                {showKey ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
              </button>
            </div>
            <p className="mt-1 text-xs text-text-tertiary">Get your key from <span className="font-medium text-text-secondary">console.anthropic.com → API keys</span>. Default model: claude-haiku-4-5-20251001.</p>
          </div>
        )}

        {aiProvider === 'ollama' && (
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-text-primary mb-1">Base URL</label>
              <input
                type="text"
                value={ollamaURL}
                onChange={(e) => { setOllamaURL(e.target.value); setTestResult(null) }}
                placeholder="http://localhost:11434"
                className="w-full px-3 py-2 rounded-lg border border-border-primary bg-surface-secondary text-text-primary text-sm font-mono focus:outline-none focus:ring-2 focus:ring-brand-primary"
              />
              <p className="mt-1 text-xs text-text-tertiary">URL where Ollama is reachable from this server.</p>
            </div>
            <div>
              <label className="block text-sm font-medium text-text-primary mb-1">Model</label>
              <input
                type="text"
                value={ollamaModel}
                onChange={(e) => setOllamaModel(e.target.value)}
                placeholder="llama3"
                className="w-full px-3 py-2 rounded-lg border border-border-primary bg-surface-secondary text-text-primary text-sm font-mono focus:outline-none focus:ring-2 focus:ring-brand-primary"
              />
              <p className="mt-1 text-xs text-text-tertiary">Recommended: llama3.1:8b (minimum) or llama3.1:70b (best quality).</p>
            </div>
          </div>
        )}

        {/* Test result */}
        {testResult && (
          <div className={`flex items-center gap-2 p-3 rounded-lg text-sm border ${
            testResult.ok ? 'bg-green-50 border-green-200 text-green-700' : 'bg-red-50 border-red-200 text-red-700'
          }`}>
            {testResult.ok ? <CheckCircle className="w-4 h-4 flex-shrink-0" /> : <AlertCircle className="w-4 h-4 flex-shrink-0" />}
            {testResult.message}
          </div>
        )}

        <div className="flex items-center gap-3">
          <Button variant="secondary" size="sm" onClick={handleTestKey} disabled={!aiInputValid() || testingKey}>
            {testingKey && <Loader2 className="w-4 h-4 animate-spin mr-1" />}
            Test connection
          </Button>
          <Button size="sm" onClick={handleSaveAI} disabled={!aiInputValid() || savingAI}>
            {savingAI ? <Loader2 className="w-4 h-4 animate-spin mr-1" /> : <Save className="w-4 h-4 mr-1" />}
            Save
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

      <section className="bg-surface-primary border border-border-primary rounded-xl p-6">
        <div className="flex items-center gap-2 mb-4">
          <Tag className="w-4 h-4 text-text-secondary" />
          <h2 className="text-base font-semibold text-text-primary">About</h2>
        </div>
        <div className="space-y-3">
          <div className="flex items-center justify-between py-2 border-b border-border-primary">
            <span className="text-sm text-text-secondary">Version</span>
            <span className="text-sm font-mono font-medium text-text-primary">{appVersion ?? '—'}</span>
          </div>
          <div className="flex items-center justify-between py-2 border-b border-border-primary">
            <span className="text-sm text-text-secondary">License</span>
            <a
              href="https://github.com/FluidifyAI/Regen/blob/main/LICENSE"
              target="_blank"
              rel="noopener noreferrer"
              className="text-sm text-brand-primary hover:underline flex items-center gap-1"
            >
              AGPLv3 <ExternalLink className="w-3 h-3" />
            </a>
          </div>
          <div className="flex items-center justify-between py-2">
            <span className="text-sm text-text-secondary">Source</span>
            <a
              href="https://github.com/FluidifyAI/Regen"
              target="_blank"
              rel="noopener noreferrer"
              className="text-sm text-brand-primary hover:underline flex items-center gap-1"
            >
              github.com/FluidifyAI/Regen <ExternalLink className="w-3 h-3" />
            </a>
          </div>
        </div>
      </section>
    </div>
  )
}
