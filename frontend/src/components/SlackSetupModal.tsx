import { useState } from 'react'
import { X, ExternalLink, CheckCircle, Loader2, AlertCircle } from 'lucide-react'
import {
  testSlackToken,
  saveSlackConfig,
  type SaveSlackConfigRequest,
  type SlackTestResult,
} from '../api/slack'

interface Props {
  onClose: () => void
  onConnected: () => void
}

function slackManifest(appUrl: string): string {
  // Slack requires HTTPS for webhook URLs. For local dev (http:// or localhost),
  // we use Socket Mode which doesn't need public URLs.
  const isLocal =
    appUrl.startsWith('http://') || appUrl.includes('localhost') || appUrl.includes('127.0.0.1')
  const useSocketMode = isLocal

  const manifest: Record<string, unknown> = {
    _metadata: {
      major_version: 1,
      minor_version: 1,
    },
    display_information: {
      name: 'OpenIncident',
      description: 'Incident management — alert routing, on-call scheduling, and Slack coordination',
      background_color: '#0F172A',
      long_description:
        'OpenIncident is an open-source incident management platform. This bot creates dedicated Slack channels for each incident, posts status updates, and accepts /incident commands for managing incidents directly from Slack.',
    },
    features: {
      app_home: {
        home_tab_enabled: false,
        messages_tab_enabled: true,
        messages_tab_read_only_enabled: false,
      },
      bot_user: {
        display_name: 'OpenIncident',
        always_online: true,
      },
      slash_commands: [
        {
          command: '/incident',
          ...(useSocketMode ? {} : { url: `${appUrl}/api/v1/webhooks/slack/commands` }),
          description: 'Manage incidents from Slack',
          usage_hint: 'new [title] | ack | resolve | status',
          should_escape: false,
        },
      ],
    },
    oauth_config: {
      redirect_urls: [`${appUrl}/api/v1/auth/slack/callback`],
      scopes: {
        bot: [
          'channels:history',
          'channels:manage',
          'channels:read',
          'channels:write.invites',
          'chat:write',
          'chat:write.public',
          'commands',
          'groups:history',
          'groups:read',
          'groups:write',
          'groups:write.invites',
          'im:write',
          'mpim:write',
          'users:read',
          'users:read.email',
        ],
        user: ['openid', 'email', 'profile'],
      },
    },
    settings: {
      ...(useSocketMode
        ? {
            socket_mode_enabled: true,
          }
        : {
            event_subscriptions: {
              request_url: `${appUrl}/api/v1/webhooks/slack/events`,
              bot_events: [
                'message.channels',
                'message.groups',
                'app_mention',
                'member_joined_channel',
              ],
            },
            interactivity: {
              is_enabled: true,
              request_url: `${appUrl}/api/v1/webhooks/slack/interactive`,
            },
            socket_mode_enabled: false,
          }),
      org_deploy_enabled: false,
      token_rotation_enabled: false,
    },
  }

  return JSON.stringify(manifest, null, 2)
}

export function SlackSetupModal({ onClose, onConnected }: Props) {
  const [step, setStep] = useState<1 | 2 | 3>(1)
  const [showManifest, setShowManifest] = useState(false)

  const [botToken, setBotToken] = useState('')
  const [signingSecret, setSigningSecret] = useState('')
  const [appToken, setAppToken] = useState('')
  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<SlackTestResult | null>(null)
  const [testError, setTestError] = useState('')
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState('')

  const [oauthClientId, setOauthClientId] = useState('')
  const [oauthClientSecret, setOauthClientSecret] = useState('')
  const [savingOAuth, setSavingOAuth] = useState(false)

  const appUrl = window.location.origin
  const isLocal =
    appUrl.startsWith('http://') ||
    appUrl.includes('localhost') ||
    appUrl.includes('127.0.0.1')
  const manifest = slackManifest(appUrl)
  const manifestPortalUrl = `https://api.slack.com/apps?new_app=1&manifest_json=${encodeURIComponent(manifest)}`

  async function handleTest() {
    if (!botToken) return
    setTesting(true)
    setTestError('')
    setTestResult(null)
    try {
      const result = await testSlackToken(botToken)
      setTestResult(result)
    } catch (e) {
      setTestError(e instanceof Error ? e.message : 'Connection test failed')
    } finally {
      setTesting(false)
    }
  }

  async function handleSave() {
    if (!botToken || !signingSecret) return
    setSaving(true)
    setSaveError('')
    try {
      const req: SaveSlackConfigRequest = {
        bot_token: botToken,
        signing_secret: signingSecret,
        app_token: appToken || undefined,
        workspace_id: testResult?.workspace_id,
        workspace_name: testResult?.workspace_name,
        bot_user_id: testResult?.bot_user_id,
      }
      await saveSlackConfig(req)
      setStep(3)
    } catch (e) {
      setSaveError(e instanceof Error ? e.message : 'Failed to save config')
    } finally {
      setSaving(false)
    }
  }

  async function handleSaveOAuth() {
    if (!oauthClientId || !oauthClientSecret) return
    setSavingOAuth(true)
    try {
      await saveSlackConfig({
        bot_token: botToken,
        signing_secret: signingSecret,
        app_token: appToken || undefined,
        workspace_id: testResult?.workspace_id,
        workspace_name: testResult?.workspace_name,
        bot_user_id: testResult?.bot_user_id,
        oauth_client_id: oauthClientId,
        oauth_client_secret: oauthClientSecret,
      })
    } catch {
      // non-fatal — user can retry
    } finally {
      setSavingOAuth(false)
    }
  }

  const inputClass =
    'w-full h-9 rounded-lg bg-surface-secondary border border-border text-text-primary text-sm px-3 placeholder-text-tertiary focus:outline-none focus:ring-2 focus:ring-brand-primary'

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="bg-surface-primary rounded-xl border border-border w-full max-w-lg shadow-2xl">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-border">
          <div>
            <h2 className="text-base font-semibold text-text-primary">Connect Slack</h2>
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

        {/* Step 1 — Create Slack App */}
        {step === 1 && (
          <div className="px-6 py-5 space-y-4">
            <p className="text-sm text-text-secondary">
              Create a Slack app using our pre-configured manifest. This automatically sets up all
              required permissions, slash commands, and event subscriptions.
            </p>

            {isLocal && (
              <div className="rounded-lg bg-amber-50 border border-amber-200 px-3 py-2.5 text-xs text-amber-800 space-y-1">
                <p className="font-medium">Local / development environment detected</p>
                <p>
                  The manifest uses <strong>Socket Mode</strong> (no public URLs needed). Slack
                  will connect to your server over a persistent WebSocket — ideal for localhost.
                  For production, deploy to an HTTPS URL and re-run this wizard.
                </p>
              </div>
            )}

            <ol className="text-sm text-text-secondary space-y-2 list-decimal list-inside">
              <li>Click the button below to open Slack's App Portal</li>
              <li>Select your workspace from the dropdown</li>
              <li>
                Review the pre-filled settings and click{' '}
                <strong className="text-text-primary">"Create"</strong>
              </li>
              <li>
                Go to <strong className="text-text-primary">Settings → Install App</strong> and
                install it to your workspace
              </li>
              {isLocal && (
                <li>
                  Go to{' '}
                  <strong className="text-text-primary">Settings → Basic Information</strong> →
                  App-Level Tokens → generate a token with{' '}
                  <code className="bg-surface-secondary px-1 rounded">connections:write</code> scope
                  (needed for Socket Mode)
                </li>
              )}
            </ol>

            <div className="text-xs text-text-tertiary space-y-1">
              <p className="font-medium text-text-secondary">What gets configured:</p>
              <ul className="space-y-0.5 list-disc list-inside">
                <li>Bot scopes: channel management, messaging, user lookup</li>
                <li>User scopes: OpenID Connect (enables "Sign in with Slack")</li>
                <li>Slash command: /incident new | ack | resolve | status</li>
                {!isLocal && <li>Event subscriptions: message sync, mentions</li>}
                {isLocal && <li>Socket Mode: bidirectional real-time sync</li>}
              </ul>
            </div>

            <a
              href={manifestPortalUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center justify-center gap-2 w-full h-10 rounded-lg bg-brand-primary hover:bg-brand-primary-hover text-white text-sm font-medium transition-colors"
            >
              Open Slack App Portal
              <ExternalLink className="w-3.5 h-3.5" />
            </a>
            <button
              onClick={() => setShowManifest(!showManifest)}
              className="text-xs text-text-tertiary hover:text-text-secondary transition-colors"
            >
              {showManifest ? '▲ Hide' : '▼ Show'} manifest JSON (advanced)
            </button>
            {showManifest && (
              <pre className="bg-surface-secondary rounded-lg p-3 text-xs text-text-secondary overflow-auto max-h-48 font-mono">
                {manifest}
              </pre>
            )}
          </div>
        )}

        {/* Step 2 — Paste Tokens */}
        {step === 2 && (
          <div className="px-6 py-5 space-y-4">
            <p className="text-sm text-text-secondary">
              From your Slack app settings, copy and paste the tokens below.
            </p>
            <div className="space-y-3">
              <div>
                <label className="block text-xs font-medium text-text-secondary mb-1">
                  Bot Token{' '}
                  <span className="text-text-tertiary font-normal">
                    (OAuth &amp; Permissions → Bot User OAuth Token)
                  </span>
                </label>
                <input
                  type="password"
                  value={botToken}
                  onChange={(e) => setBotToken(e.target.value)}
                  placeholder="xoxb-..."
                  className={inputClass}
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-text-secondary mb-1">
                  Signing Secret{' '}
                  <span className="text-text-tertiary font-normal">
                    (Basic Information → App Credentials)
                  </span>
                </label>
                <input
                  type="password"
                  value={signingSecret}
                  onChange={(e) => setSigningSecret(e.target.value)}
                  placeholder="••••••••••••••••"
                  className={inputClass}
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-text-secondary mb-1">
                  App-Level Token{' '}
                  <span className="text-text-tertiary font-normal">
                    (optional — enables Socket Mode for bidirectional sync)
                  </span>
                </label>
                <input
                  type="password"
                  value={appToken}
                  onChange={(e) => setAppToken(e.target.value)}
                  placeholder="xapp-..."
                  className={inputClass}
                />
              </div>
            </div>

            <button
              onClick={handleTest}
              disabled={!botToken || testing}
              className="flex items-center gap-2 px-4 py-2 rounded-lg border border-border text-sm font-medium text-text-primary hover:bg-surface-secondary transition-colors disabled:opacity-50"
            >
              {testing && <Loader2 className="w-4 h-4 animate-spin" />}
              Test Connection
            </button>

            {testResult && (
              <div className="flex items-start gap-2 rounded-lg bg-green-50 border border-green-200 px-3 py-2">
                <CheckCircle className="w-4 h-4 text-green-600 flex-shrink-0 mt-0.5" />
                <span className="text-sm text-green-700">
                  Connected to <strong>{testResult.workspace_name}</strong> as @
                  {testResult.bot_username}
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
                <p className="text-sm font-medium text-green-700">Slack connected</p>
                {testResult && (
                  <p className="text-xs text-green-600 mt-0.5">
                    Workspace: {testResult.workspace_name} · Bot: @{testResult.bot_username}
                  </p>
                )}
              </div>
            </div>

            <div className="text-sm text-text-secondary space-y-1">
              <p className="font-medium text-text-primary">What's next:</p>
              <ul className="space-y-1 list-disc list-inside">
                <li>
                  Invite{' '}
                  <code className="bg-surface-secondary px-1 rounded text-xs">@openincident</code>{' '}
                  to your #incidents channel
                </li>
                <li>Trigger a test alert to verify the integration</li>
                <li>Incident channels will be created automatically on new incidents</li>
              </ul>
            </div>

            <div className="border-t border-border pt-4">
              <p className="text-sm font-medium text-text-primary mb-0.5">
                Slack Login{' '}
                <span className="text-text-tertiary font-normal text-xs">(optional)</span>
              </p>
              <p className="text-xs text-text-tertiary mb-3">
                Let team members sign in with their Slack account. Requires a separate OAuth app or
                the same app with <code className="bg-surface-secondary px-1 rounded">openid</code>{' '}
                scope added.
              </p>
              <div className="space-y-2">
                <input
                  type="text"
                  value={oauthClientId}
                  onChange={(e) => setOauthClientId(e.target.value)}
                  placeholder="OAuth Client ID"
                  className={inputClass}
                />
                <input
                  type="password"
                  value={oauthClientSecret}
                  onChange={(e) => setOauthClientSecret(e.target.value)}
                  placeholder="OAuth Client Secret"
                  className={inputClass}
                />
                <button
                  onClick={handleSaveOAuth}
                  disabled={!oauthClientId || !oauthClientSecret || savingOAuth}
                  className="flex items-center gap-2 px-3 py-1.5 rounded-lg border border-border text-xs font-medium text-text-primary hover:bg-surface-secondary transition-colors disabled:opacity-50"
                >
                  {savingOAuth && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
                  Save OAuth Config
                </button>
              </div>
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
              disabled={!botToken || !signingSecret || saving}
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
