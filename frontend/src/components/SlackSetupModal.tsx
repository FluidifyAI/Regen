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
      name: 'Fluidify Regen',
      description: 'Incident management — alert routing, on-call scheduling, and Slack coordination',
      background_color: '#ffffff',
      long_description:
        'Fluidify Regen is an open-source incident management platform. This bot creates dedicated Slack channels for each incident, posts status updates, and accepts /incident commands for managing incidents directly from Slack.',
    },
    features: {
      app_home: {
        home_tab_enabled: false,
        messages_tab_enabled: true,
        messages_tab_read_only_enabled: false,
      },
      bot_user: {
        display_name: 'Fluidify Regen',
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
        oauth_client_id: oauthClientId || undefined,
        oauth_client_secret: oauthClientSecret || undefined,
      }
      await saveSlackConfig(req)
      setStep(3)
    } catch (e) {
      setSaveError(e instanceof Error ? e.message : 'Failed to save config')
    } finally {
      setSaving(false)
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
          <div className="px-6 py-5 space-y-4 overflow-y-auto max-h-[70vh]">
            <p className="text-sm text-text-secondary">
              Open your app at{' '}
              <a
                href="https://api.slack.com/apps"
                target="_blank"
                rel="noopener noreferrer"
                className="text-brand-primary hover:underline inline-flex items-center gap-0.5"
              >
                api.slack.com/apps <ExternalLink className="w-3 h-3" />
              </a>{' '}
              and copy the values below.
            </p>

            {/* Bot Token */}
            <div>
              <label className="block text-xs font-medium text-text-secondary mb-1">
                Bot Token <span className="text-red-500">*</span>
              </label>
              <p className="text-xs text-text-tertiary mb-1.5">
                Sidebar → <strong>OAuth &amp; Permissions</strong> → scroll to{' '}
                <strong>OAuth Tokens</strong> section → copy{' '}
                <strong>Bot User OAuth Token</strong> (starts with <code className="bg-surface-secondary px-1 rounded">xoxb-</code>)
              </p>
              <input
                type="password"
                value={botToken}
                onChange={(e) => setBotToken(e.target.value)}
                placeholder="xoxb-..."
                className={inputClass}
              />
            </div>

            {/* Signing Secret */}
            <div>
              <label className="block text-xs font-medium text-text-secondary mb-1">
                Signing Secret <span className="text-red-500">*</span>
              </label>
              <p className="text-xs text-text-tertiary mb-1.5">
                Sidebar → <strong>Basic Information</strong> → <strong>App Credentials</strong>{' '}
                section → click <strong>Show</strong> next to <strong>Signing Secret</strong> → copy
              </p>
              <input
                type="password"
                value={signingSecret}
                onChange={(e) => setSigningSecret(e.target.value)}
                placeholder="••••••••••••••••"
                className={inputClass}
              />
            </div>

            {/* OAuth Client ID + Secret */}
            <div className="rounded-lg border border-border bg-surface-secondary/50 px-3 py-3 space-y-3">
              <div>
                <p className="text-xs font-medium text-text-primary mb-0.5">Slack Login (recommended)</p>
                <p className="text-xs text-text-tertiary">
                  Lets team members sign in with their Slack account. Both values are on the{' '}
                  <strong>Basic Information</strong> page under <strong>App Credentials</strong>.
                </p>
              </div>
              <div>
                <label className="block text-xs font-medium text-text-secondary mb-1">
                  OAuth Client ID
                </label>
                <p className="text-xs text-text-tertiary mb-1.5">
                  <strong>Basic Information</strong> → <strong>App Credentials</strong> → copy{' '}
                  <strong>Client ID</strong>
                </p>
                <input
                  type="text"
                  value={oauthClientId}
                  onChange={(e) => setOauthClientId(e.target.value)}
                  placeholder="1234567890.1234567890"
                  className={inputClass}
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-text-secondary mb-1">
                  OAuth Client Secret
                </label>
                <p className="text-xs text-text-tertiary mb-1.5">
                  <strong>Basic Information</strong> → <strong>App Credentials</strong> → click{' '}
                  <strong>Show</strong> next to <strong>Client Secret</strong> → copy
                </p>
                <input
                  type="password"
                  value={oauthClientSecret}
                  onChange={(e) => setOauthClientSecret(e.target.value)}
                  placeholder="••••••••••••••••"
                  className={inputClass}
                />
              </div>
            </div>

            {/* App-Level Token (Socket Mode) */}
            {isLocal && (
              <div>
                <label className="block text-xs font-medium text-text-secondary mb-1">
                  App-Level Token{' '}
                  <span className="text-text-tertiary font-normal">(required for Socket Mode)</span>
                </label>
                <p className="text-xs text-text-tertiary mb-1.5">
                  <strong>Basic Information</strong> → <strong>App-Level Tokens</strong> section →
                  click <strong>Generate Token and Scopes</strong> → add{' '}
                  <code className="bg-surface-secondary px-1 rounded">connections:write</code> scope
                  → copy the token (starts with <code className="bg-surface-secondary px-1 rounded">xapp-</code>)
                </p>
                <input
                  type="password"
                  value={appToken}
                  onChange={(e) => setAppToken(e.target.value)}
                  placeholder="xapp-..."
                  className={inputClass}
                />
              </div>
            )}

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

            {oauthClientId && (
              <div className="border-t border-border pt-4 flex items-center gap-2 text-xs text-green-700">
                <CheckCircle className="w-4 h-4 text-green-600 flex-shrink-0" />
                Slack Login enabled — team members can sign in with Slack
              </div>
            )}
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
