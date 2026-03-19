import { useState } from 'react'
import { X, CheckCircle, Loader2, AlertCircle, ExternalLink, Download } from 'lucide-react'
import {
  testTeamsConfig,
  saveTeamsConfig,
  downloadTeamsAppPackage,
  type TeamsTestResult,
} from '../api/teams_config'

interface Props {
  onClose: () => void
  onConnected: () => void
}

export function TeamsSetupModal({ onClose, onConnected }: Props) {
  const [step, setStep] = useState<1 | 2 | 3>(1)

  const [appId, setAppId] = useState('')
  const [appPassword, setAppPassword] = useState('')
  const [tenantId, setTenantId] = useState('')
  const [teamId, setTeamId] = useState('')
  const [serviceUrl, setServiceUrl] = useState('https://smba.trafficmanager.net/amer/')

  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<TeamsTestResult | null>(null)
  const [downloading, setDownloading] = useState(false)

  async function handleDownloadPackage() {
    setDownloading(true)
    try {
      await downloadTeamsAppPackage()
    } finally {
      setDownloading(false)
    }
  }
  const [testError, setTestError] = useState('')

  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState('')

  const inputClass =
    'w-full h-9 rounded-lg bg-surface-secondary border border-border text-text-primary text-sm px-3 placeholder-text-tertiary focus:outline-none focus:ring-2 focus:ring-brand-primary'

  async function handleTest() {
    if (!appId || !appPassword || !tenantId || !teamId) return
    setTesting(true)
    setTestError('')
    setTestResult(null)
    try {
      const result = await testTeamsConfig({ app_id: appId, app_password: appPassword, tenant_id: tenantId, team_id: teamId })
      setTestResult(result)
    } catch (e) {
      setTestError(e instanceof Error ? e.message : 'Connection test failed')
    } finally {
      setTesting(false)
    }
  }

  async function handleSave() {
    if (!appId || !appPassword || !tenantId || !teamId) return
    setSaving(true)
    setSaveError('')
    try {
      await saveTeamsConfig({
        app_id: appId,
        app_password: appPassword,
        tenant_id: tenantId,
        team_id: teamId,
        service_url: serviceUrl || undefined,
        team_name: testResult?.team_name,
      })
      setStep(3)
    } catch (e) {
      setSaveError(e instanceof Error ? e.message : 'Failed to save config')
    } finally {
      setSaving(false)
    }
  }

  const canTest = appId.trim() !== '' && appPassword.trim() !== '' && tenantId.trim() !== '' && teamId.trim() !== ''

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="bg-surface-primary rounded-xl border border-border w-full max-w-lg shadow-2xl">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-border">
          <div>
            <h2 className="text-base font-semibold text-text-primary">Connect Microsoft Teams</h2>
            <p className="text-xs text-text-tertiary mt-0.5">Step {step} of 3</p>
          </div>
          <button
            onClick={onClose}
            className="p-2 rounded hover:bg-surface-secondary transition-colors"
            aria-label="Close"
          >
            <X className="w-4 h-4 text-text-tertiary" />
          </button>
        </div>

        {/* Step 1 — Azure setup */}
        {step === 1 && (
          <div className="px-6 py-5 space-y-4 overflow-y-auto max-h-[70vh]">
            <p className="text-sm text-text-secondary">
              Fluidify Regen connects to Teams via a registered Azure App and a Bot Framework bot.
              Follow these steps in the Azure portal before entering credentials.
            </p>

            <ol className="text-sm text-text-secondary space-y-3 list-decimal list-inside">
              <li>
                Open{' '}
                <a
                  href="https://portal.azure.com"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-brand-primary hover:underline inline-flex items-center gap-0.5"
                >
                  portal.azure.com <ExternalLink className="w-3 h-3" />
                </a>
                {' '}→ <strong>Azure Active Directory</strong> → <strong>App registrations</strong> → <strong>New registration</strong>
              </li>
              <li>
                Name it <code className="bg-surface-secondary px-1 rounded">Fluidify Regen</code>,
                set <strong>Accounts in this organizational directory only</strong>, then click <strong>Register</strong>.
                Copy the <strong>Application (client) ID</strong> and <strong>Directory (tenant) ID</strong>.
              </li>
              <li>
                Under <strong>Certificates &amp; secrets</strong> → <strong>New client secret</strong>.
                Copy the secret <strong>Value</strong> (shown once only).
              </li>
              <li>
                Under <strong>API permissions</strong> → <strong>Add a permission</strong> →
                <strong>Microsoft Graph</strong> → Application permissions → add{' '}
                <code className="bg-surface-secondary px-1 rounded">Team.ReadBasic.All</code> and{' '}
                <code className="bg-surface-secondary px-1 rounded">Channel.Create</code>.
                Grant admin consent.
              </li>
              <li>
                Create an <strong>Azure Bot</strong> resource (search "Azure Bot" in the portal).
                Use your App Registration's client ID as the <em>Microsoft App ID</em>.
                Under <strong>Channels</strong>, enable <strong>Microsoft Teams</strong>.
              </li>
              <li>
                In your bot's <strong>Configuration</strong>, set the messaging endpoint to{' '}
                <code className="bg-surface-secondary px-1 rounded text-xs break-all">
                  {window.location.origin}/api/v1/webhooks/teams/events
                </code>
              </li>
              <li>
                In Teams, go to <strong>Apps</strong> → sideload the bot into your target team
                (or use the Teams admin portal for tenant-wide install).
                Copy the <strong>Team ID</strong> from the team URL or admin portal.
              </li>
            </ol>

            <div className="rounded-lg bg-amber-50 border border-amber-200 px-3 py-2.5 text-xs text-amber-800">
              <p className="font-medium">Bot Framework API permission required</p>
              <p className="mt-0.5">
                After creating the Azure Bot resource, go back to your App Registration →
                <strong>API permissions</strong> → find <strong>Bot Framework</strong> under
                "APIs my organization uses" → add{' '}
                <code className="bg-amber-100 px-0.5 rounded">https://api.botframework.com/.default</code> and grant consent.
                This unblocks proactive messaging.
              </p>
            </div>
          </div>
        )}

        {/* Step 2 — Enter credentials */}
        {step === 2 && (
          <div className="px-6 py-5 space-y-4 overflow-y-auto max-h-[70vh]">
            <p className="text-sm text-text-secondary">
              Enter the credentials from your Azure App Registration. Click <strong>Test</strong> to
              validate before saving.
            </p>

            <div>
              <label className="block text-xs font-medium text-text-secondary mb-1">
                Application (Client) ID <span className="text-red-500">*</span>
              </label>
              <p className="text-xs text-text-tertiary mb-1.5">
                Azure portal → App registrations → your app → <strong>Overview</strong> → Application (client) ID
              </p>
              <input
                type="text"
                value={appId}
                onChange={(e) => { setAppId(e.target.value); setTestResult(null) }}
                placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
                className={inputClass}
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-text-secondary mb-1">
                Client Secret <span className="text-red-500">*</span>
              </label>
              <p className="text-xs text-text-tertiary mb-1.5">
                App registration → <strong>Certificates &amp; secrets</strong> → secret <strong>Value</strong>
              </p>
              <input
                type="password"
                value={appPassword}
                onChange={(e) => { setAppPassword(e.target.value); setTestResult(null) }}
                placeholder="••••••••••••••••"
                className={inputClass}
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-text-secondary mb-1">
                Directory (Tenant) ID <span className="text-red-500">*</span>
              </label>
              <p className="text-xs text-text-tertiary mb-1.5">
                App registration → <strong>Overview</strong> → Directory (tenant) ID
              </p>
              <input
                type="text"
                value={tenantId}
                onChange={(e) => { setTenantId(e.target.value); setTestResult(null) }}
                placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
                className={inputClass}
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-text-secondary mb-1">
                Team ID <span className="text-red-500">*</span>
              </label>
              <p className="text-xs text-text-tertiary mb-1.5">
                Teams admin portal → Teams → select team → copy ID, or from the team's URL
              </p>
              <input
                type="text"
                value={teamId}
                onChange={(e) => { setTeamId(e.target.value); setTestResult(null) }}
                placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
                className={inputClass}
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-text-secondary mb-1">
                Bot Framework Service URL{' '}
                <span className="text-text-tertiary font-normal">(optional)</span>
              </label>
              <p className="text-xs text-text-tertiary mb-1.5">
                Region-specific relay URL. Default is <code className="bg-surface-secondary px-1 rounded">amer</code>.
                Change to <code className="bg-surface-secondary px-1 rounded">emea</code> or{' '}
                <code className="bg-surface-secondary px-1 rounded">in</code> if your tenant is in Europe or India.
              </p>
              <input
                type="text"
                value={serviceUrl}
                onChange={(e) => setServiceUrl(e.target.value)}
                placeholder="https://smba.trafficmanager.net/amer/"
                className={inputClass}
              />
            </div>

            <button
              onClick={handleTest}
              disabled={!canTest || testing}
              className="flex items-center gap-2 px-4 py-2 rounded-lg border border-border text-sm font-medium text-text-primary hover:bg-surface-secondary transition-colors disabled:opacity-50"
            >
              {testing && <Loader2 className="w-4 h-4 animate-spin" />}
              Test Connection
            </button>

            {testResult && (
              <div className="flex items-start gap-2 rounded-lg bg-green-50 border border-green-200 px-3 py-2">
                <CheckCircle className="w-4 h-4 text-green-600 flex-shrink-0 mt-0.5" />
                <span className="text-sm text-green-700">
                  Connected to team <strong>{testResult.team_name}</strong>
                </span>
              </div>
            )}
            {testError && (
              <div className="flex items-start gap-2 rounded-lg bg-red-50 border border-red-200 px-3 py-2">
                <AlertCircle className="w-4 h-4 text-red-600 flex-shrink-0 mt-0.5" />
                <span className="text-sm text-red-700">{testError}</span>
              </div>
            )}
            {saveError && (
              <p className="text-sm text-red-600">{saveError}</p>
            )}
          </div>
        )}

        {/* Step 3 — Connected */}
        {step === 3 && (
          <div className="px-6 py-5 space-y-4">
            <div className="flex items-start gap-3 rounded-lg bg-green-50 border border-green-200 px-4 py-3">
              <CheckCircle className="w-5 h-5 text-green-600 flex-shrink-0 mt-0.5" />
              <div>
                <p className="text-sm font-medium text-green-700">Microsoft Teams connected</p>
                {testResult && (
                  <p className="text-xs text-green-600 mt-0.5">Team: {testResult.team_name}</p>
                )}
              </div>
            </div>

            <div className="text-sm text-text-secondary space-y-1">
              <p className="font-medium text-text-primary">What's next:</p>
              <ul className="space-y-1 list-disc list-inside">
                <li>Incident channels will be created automatically in your team</li>
                <li>The bot will post Adaptive Cards for new incidents and status changes</li>
                <li>Use{' '}
                  <code className="bg-surface-secondary px-1 rounded text-xs">@Fluidify Regen ack</code> or{' '}
                  <code className="bg-surface-secondary px-1 rounded text-xs">resolve</code> in any incident channel
                </li>
                <li>Restart the server to apply the new credentials</li>
              </ul>
            </div>

            <div className="rounded-lg border border-border bg-surface-secondary p-4 space-y-2">
              <p className="text-sm font-medium text-text-primary">Install the bot in Teams</p>
              <p className="text-xs text-text-secondary">
                Download the app package and sideload it into your Microsoft Teams team to enable bot commands.
              </p>
              <button
                onClick={handleDownloadPackage}
                disabled={downloading}
                className="flex items-center gap-2 px-3 py-1.5 rounded-lg border border-border bg-white hover:bg-surface-secondary disabled:opacity-50 text-sm font-medium text-text-primary transition-colors"
              >
                {downloading
                  ? <Loader2 className="w-4 h-4 animate-spin" />
                  : <Download className="w-4 h-4" />
                }
                {downloading ? 'Generating…' : 'Download Teams App Package'}
              </button>
            </div>
          </div>
        )}

        {/* Footer */}
        <div className="flex items-center justify-between px-6 py-4 border-t border-border">
          <button
            onClick={step === 1 ? onClose : () => setStep((s) => (s - 1) as 1 | 2 | 3)}
            className="px-3 py-1.5 text-sm text-text-secondary hover:text-text-primary transition-colors"
          >
            {step === 1 ? 'Cancel' : '← Back'}
          </button>

          {step === 1 && (
            <button
              onClick={() => setStep(2)}
              className="px-4 py-1.5 rounded-lg bg-brand-primary hover:bg-brand-primary-hover text-white text-sm font-medium transition-colors"
            >
              Next →
            </button>
          )}
          {step === 2 && (
            <button
              onClick={handleSave}
              disabled={!canTest || !testResult || saving}
              title={!testResult ? 'Run the connection test first' : undefined}
              className="flex items-center gap-2 px-4 py-1.5 rounded-lg bg-brand-primary hover:bg-brand-primary-hover disabled:opacity-50 text-white text-sm font-medium transition-colors"
            >
              {saving && <Loader2 className="w-4 h-4 animate-spin" />}
              Save &amp; Continue →
            </button>
          )}
          {step === 3 && (
            <button
              onClick={onConnected}
              className="px-4 py-1.5 rounded-lg bg-brand-primary hover:bg-brand-primary-hover text-white text-sm font-medium transition-colors"
            >
              Done
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
