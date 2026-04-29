import { useState, useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  X, Copy, Check, Webhook,
  ExternalLink, Lock, MessageSquarePlus, Send, CheckCircle,
} from 'lucide-react'
import { apiClient } from '../api/client'
import { getSlackConfig, type SlackConfigStatus } from '../api/slack'
import { getTeamsConfig, type TeamsConfigStatus } from '../api/teams_config'
import { getTelegramConfig, type TelegramConfigStatus } from '../api/telegram_config'
import { SlackSetupModal } from '../components/SlackSetupModal'
import { TeamsSetupModal } from '../components/TeamsSetupModal'
import { TelegramSetupModal } from '../components/TelegramSetupModal'
import { useAuth } from '../hooks/useAuth'

// ─── Integration definitions ──────────────────────────────────────────────────

interface Integration {
  id: string
  name: string
  tagline: string
  description: string
  source: string
  webhookPath: string
  /** Simple Icons slug — if provided, renders a brand logo via CDN */
  logoSlug?: string
  /** Fallback Lucide icon when no logoSlug */
  icon?: React.ComponentType<{ className?: string }>
  docsUrl?: string
  setup: SetupSection[]
}

interface SetupSection {
  title: string
  content: string
  language?: string
}

interface ComingSoon {
  id: string
  name: string
  tagline: string
  /** Simple Icons slug for brand logo */
  logoSlug: string
}

const INTEGRATIONS: Integration[] = [
  {
    id: 'prometheus',
    name: 'Prometheus Alertmanager',
    tagline: 'Receive alerts from Prometheus Alertmanager',
    description:
      'Connect Alertmanager to automatically create incidents and escalations when your Prometheus alerts fire.',
    source: 'prometheus',
    webhookPath: '/api/v1/webhooks/prometheus',
    logoSlug: 'prometheus',
    docsUrl: 'https://prometheus.io/docs/alerting/latest/configuration/',
    setup: [
      {
        title: 'Add a receiver to alertmanager.yml',
        language: 'yaml',
        content: `receivers:
  - name: 'fluidify-alert'
    webhook_configs:
      - url: '__WEBHOOK_URL__'
        send_resolved: true

route:
  receiver: 'fluidify-alert'`,
      },
      {
        title: 'Reload Alertmanager',
        language: 'bash',
        content: `curl -X POST http://localhost:9093/-/reload`,
      },
    ],
  },
  {
    id: 'grafana',
    name: 'Grafana',
    tagline: 'Receive alerts from Grafana',
    description:
      'Forward Grafana alert notifications to Fluidify Regen via a webhook contact point.',
    source: 'grafana',
    webhookPath: '/api/v1/webhooks/grafana',
    logoSlug: 'grafana',
    docsUrl: 'https://grafana.com/docs/grafana/latest/alerting/alerting-rules/manage-contact-points/',
    setup: [
      {
        title: 'Add a Webhook contact point in Grafana',
        content:
          '1. Go to Alerting → Contact points → Add contact point\n2. Choose "Webhook" as the type\n3. Paste your webhook URL below\n4. Set HTTP Method to POST\n5. Save and attach the contact point to a Notification policy',
      },
    ],
  },
  {
    id: 'cloudwatch',
    name: 'Amazon CloudWatch',
    tagline: 'Receive alerts from AWS CloudWatch via SNS',
    description:
      'Route CloudWatch alarms through SNS to Fluidify Regen for unified incident management.',
    source: 'cloudwatch',
    webhookPath: '/api/v1/webhooks/cloudwatch',
    logoSlug: 'amazoncloudwatch',
    docsUrl: 'https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/AlarmThatSendsEmail.html',
    setup: [
      {
        title: 'Create an SNS subscription',
        language: 'bash',
        content: `# Create an SNS topic
aws sns create-topic --name fluidify-alert-alerts

# Subscribe the webhook as an HTTPS endpoint
aws sns subscribe \\
  --topic-arn <your-topic-arn> \\
  --protocol https \\
  --notification-endpoint '__WEBHOOK_URL__'`,
      },
      {
        title: 'Attach the SNS topic to a CloudWatch alarm',
        content:
          'In your CloudWatch alarm, set the SNS topic as the notification target for ALARM, OK, and INSUFFICIENT_DATA states.',
      },
    ],
  },
  {
    id: 'generic',
    name: 'Generic Webhook',
    tagline: 'Connect any tool using our webhook format',
    description:
      'Send alerts from any tool using the generic webhook schema — useful for custom scripts, internal tools, or monitoring systems without a native integration.',
    source: 'generic',
    webhookPath: '/api/v1/webhooks/generic',
    icon: Webhook,
    setup: [
      {
        title: 'POST to the webhook URL',
        language: 'bash',
        content: `curl -s -X POST '__WEBHOOK_URL__' \\
  -H 'Content-Type: application/json' \\
  -d '{
    "external_id": "alert-001",
    "source": "my-tool",
    "title": "High memory usage on web-01",
    "description": "Memory usage exceeded 90% threshold",
    "severity": "critical",
    "status": "firing",
    "started_at": "2024-01-01T12:00:00Z"
  }'`,
      },
      {
        title: 'Required fields',
        content:
          'external_id (string) — unique ID for deduplication\nsource (string) — name of your tool\ntitle (string) — short alert title\nstatus (string) — "firing" or "resolved"\n\nOptional: description, severity (critical/warning/info), started_at, ended_at, labels, annotations',
      },
    ],
  },
]

const COMING_SOON: ComingSoon[] = [
  { id: 'datadog',   name: 'Datadog',          tagline: 'Monitors, events & alerts',        logoSlug: 'datadog'       },
  { id: 'pagerduty', name: 'PagerDuty',         tagline: 'Import schedules & escalations',   logoSlug: 'pagerduty'     },
  { id: 'opsgenie',  name: 'Opsgenie',          tagline: 'Alerts & on-call schedules',       logoSlug: 'opsgenie'      },
  { id: 'newrelic',  name: 'New Relic',         tagline: 'APM alerts & anomalies',           logoSlug: 'newrelic'      },
  { id: 'sentry',    name: 'Sentry',            tagline: 'Error & performance alerts',       logoSlug: 'sentry'        },
  { id: 'dynatrace', name: 'Dynatrace',         tagline: 'AI-powered problem detection',     logoSlug: 'dynatrace'     },
  { id: 'splunk',    name: 'Splunk On-Call',    tagline: 'VictorOps schedules & alerts',     logoSlug: 'splunk'        },
  { id: 'zabbix',    name: 'Zabbix',            tagline: 'Infrastructure monitoring alerts', logoSlug: 'zabbix'        },
  { id: 'elastic',   name: 'Elastic / Kibana',  tagline: 'Watcher & rule-based alerts',      logoSlug: 'elastic'       },
  { id: 'nagios',    name: 'Nagios',            tagline: 'Host & service check alerts',      logoSlug: 'nagios'        },
  { id: 'uptime',    name: 'Uptime Kuma',       tagline: 'Uptime monitoring notifications',  logoSlug: 'uptimekuma'    },
  { id: 'better',    name: 'Betterstack',       tagline: 'Uptime & on-call management',      logoSlug: 'betterstack'   },
]

// ─── Helpers ──────────────────────────────────────────────────────────────────

function simpleIconUrl(slug: string): string {
  return `https://cdn.jsdelivr.net/npm/simple-icons@latest/icons/${slug}.svg`
}

function webhookUrl(path: string): string {
  const base = import.meta.env.VITE_API_URL || window.location.origin
  return `${base}${path}`
}

function renderSetupContent(content: string, webhookPath: string): string {
  return content.replace(/__WEBHOOK_URL__/g, webhookUrl(webhookPath))
}

function buildGitHubIssueUrl(toolName: string, useCase: string, impact: string): string {
  const title = `Integration Request: ${toolName}`
  const body = [
    `## Integration Request: ${toolName}`,
    '',
    '**Tool / service:** ' + toolName,
    '',
    '**Use case:**',
    useCase || '_No description provided_',
    '',
    '**Business impact:** ' + (impact || 'Not specified'),
    '',
    '---',
    '_Requested via the Fluidify Regen integration hub_',
  ].join('\n')
  const params = new URLSearchParams({ title, body, labels: 'integration-request' })
  return `https://github.com/fluidify/regen/issues/new?${params}`
}

// ─── Brand logo component ─────────────────────────────────────────────────────

/**
 * Shows a Simple Icons brand logo; falls back to a monogram on load failure.
 */
function BrandLogo({ slug, name, size = 'md' }: { slug: string; name: string; size?: 'sm' | 'md' }) {
  const [failed, setFailed] = useState(false)
  const dim = size === 'sm' ? 'w-5 h-5' : 'w-6 h-6'

  if (failed) {
    return (
      <span className={`${dim} flex items-center justify-center text-xs font-bold text-text-secondary`}>
        {name[0]?.toUpperCase()}
      </span>
    )
  }

  return (
    <img
      src={simpleIconUrl(slug)}
      alt={name}
      className={`${dim} object-contain`}
      onError={() => setFailed(true)}
    />
  )
}

// ─── Integration icon (brand logo or Lucide fallback) ─────────────────────────

function IntegrationIcon({
  integration, size = 'md',
}: {
  integration: Pick<Integration, 'logoSlug' | 'icon' | 'name'>
  size?: 'sm' | 'md'
}) {
  const containerDim = size === 'sm' ? 'w-8 h-8' : 'w-10 h-10'
  const iconDim = size === 'sm' ? 'w-4 h-4' : 'w-5 h-5'

  return (
    <div className={`${containerDim} rounded-lg border border-border bg-white flex items-center justify-center flex-shrink-0 shadow-sm`}>
      {integration.logoSlug ? (
        <BrandLogo slug={integration.logoSlug} name={integration.name} size={size === 'sm' ? 'sm' : 'md'} />
      ) : integration.icon ? (
        <integration.icon className={`${iconDim} text-text-secondary`} />
      ) : (
        <span className="text-xs font-bold text-text-secondary">{integration.name[0]}</span>
      )}
    </div>
  )
}

// ─── Copy button ──────────────────────────────────────────────────────────────

function CopyButton({ text, className = '' }: { text: string; className?: string }) {
  const [copied, setCopied] = useState(false)
  return (
    <button
      onClick={() => {
        navigator.clipboard.writeText(text).then(() => {
          setCopied(true)
          setTimeout(() => setCopied(false), 2000)
        })
      }}
      title="Copy to clipboard"
      className={`p-1.5 rounded transition-colors ${copied ? 'text-emerald-600' : 'text-text-tertiary hover:text-text-primary'} ${className}`}
    >
      {copied ? <Check className="w-4 h-4" /> : <Copy className="w-4 h-4" />}
    </button>
  )
}

// ─── Request integration modal ────────────────────────────────────────────────

function RequestModal({ initialTool = '', onClose }: { initialTool?: string; onClose: () => void }) {
  const [tool, setTool] = useState(initialTool)
  const [useCase, setUseCase] = useState('')
  const [impact, setImpact] = useState('')
  const [submitted, setSubmitted] = useState(false)
  const toolRef = useRef<HTMLInputElement>(null)

  useEffect(() => { setTimeout(() => toolRef.current?.focus(), 50) }, [])
  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [onClose])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!tool.trim()) return
    window.open(buildGitHubIssueUrl(tool.trim(), useCase, impact), '_blank', 'noopener,noreferrer')
    setSubmitted(true)
  }

  const inputClass =
    'w-full px-3 py-2 border border-border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary focus:border-transparent'

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />
      <div
        className="relative z-10 w-full max-w-md bg-white rounded-xl shadow-2xl mx-4 overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header — brand blue, consistent with rest of app */}
        <div className="bg-brand-primary px-6 py-5">
          <div className="flex items-start justify-between">
            <div className="flex items-center gap-3">
              <div className="w-9 h-9 rounded-lg bg-white/15 flex items-center justify-center">
                <MessageSquarePlus className="w-5 h-5 text-white" />
              </div>
              <div>
                <h2 className="text-base font-semibold text-white">Request an integration</h2>
                <p className="text-xs text-white/70 mt-0.5">Opens a pre-filled GitHub issue</p>
              </div>
            </div>
            <button onClick={onClose} className="p-1 rounded hover:bg-white/15 text-white/70 hover:text-white transition-colors">
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>

        {submitted ? (
          <div className="px-6 py-8 text-center space-y-3">
            <div className="w-12 h-12 rounded-full bg-emerald-100 flex items-center justify-center mx-auto">
              <Check className="w-6 h-6 text-emerald-600" />
            </div>
            <p className="font-medium text-text-primary">GitHub opened in a new tab</p>
            <p className="text-sm text-text-secondary">
              Submit the pre-filled issue to officially request <strong>{tool}</strong>. We track upvotes to prioritise what to build next.
            </p>
            <button
              onClick={onClose}
              className="mt-2 px-4 py-2 bg-brand-primary text-white rounded-lg text-sm font-medium hover:bg-brand-primary-hover transition-colors"
            >
              Done
            </button>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="px-6 py-5 space-y-4">
            <div>
              <label className="block text-sm font-medium text-text-primary mb-1">
                Tool or service name <span className="text-red-500">*</span>
              </label>
              <input
                ref={toolRef}
                value={tool}
                onChange={(e) => setTool(e.target.value)}
                placeholder="e.g. Datadog, PagerDuty, Splunk…"
                className={inputClass}
                required
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-text-primary mb-1">How would you use it?</label>
              <textarea
                value={useCase}
                onChange={(e) => setUseCase(e.target.value)}
                placeholder="Describe your use case — what alerts you want to route, how many people are affected, etc."
                rows={3}
                className={`${inputClass} resize-none`}
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-text-primary mb-1">How important is this?</label>
              <select value={impact} onChange={(e) => setImpact(e.target.value)} className={inputClass}>
                <option value="">Select…</option>
                <option value="Nice to have">Nice to have</option>
                <option value="Important — affects our workflow">Important — affects our workflow</option>
                <option value="Critical — blocking adoption">Critical — blocking adoption</option>
              </select>
            </div>
            <p className="text-xs text-text-tertiary bg-surface-secondary rounded-lg px-3 py-2">
              Clicking below opens GitHub with a pre-filled issue. Submit it and add a 👍 to help us prioritise.
            </p>
            <div className="flex justify-end gap-3 pt-1">
              <button type="button" onClick={onClose} className="px-4 py-2 text-sm text-text-secondary hover:text-text-primary transition-colors">
                Cancel
              </button>
              <button
                type="submit"
                disabled={!tool.trim()}
                className="inline-flex items-center gap-2 px-4 py-2 bg-brand-primary hover:bg-brand-primary-hover text-white rounded-lg text-sm font-medium transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
              >
                <Send className="w-3.5 h-3.5" />
                Open GitHub issue
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  )
}

// ─── Detail panel ─────────────────────────────────────────────────────────────

function DetailPanel({
  integration, connected, alertCount, onClose,
}: {
  integration: Integration
  connected: boolean
  alertCount: number
  onClose: () => void
}) {
  const url = webhookUrl(integration.webhookPath)

  return (
    <div className="fixed inset-0 z-50 flex">
      <div className="flex-1 bg-black/30" onClick={onClose} />
      <div className="w-full max-w-xl bg-white shadow-2xl flex flex-col overflow-y-auto">
        {/* Header */}
        <div className="bg-brand-primary px-6 py-5 flex-shrink-0">
          <div className="flex items-start justify-between">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-lg bg-white/15 flex items-center justify-center">
                <IntegrationIcon integration={integration} size="sm" />
              </div>
              <div>
                <h2 className="text-lg font-semibold text-white">{integration.name}</h2>
                <p className="text-sm text-white/75">{integration.tagline}</p>
              </div>
            </div>
            <button onClick={onClose} className="p-1 rounded hover:bg-white/15 text-white/70 hover:text-white transition-colors">
              <X className="w-5 h-5" />
            </button>
          </div>
          <div className="flex items-center gap-3 mt-4">
            {connected ? (
              <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-emerald-500/20 text-emerald-100 text-xs font-medium">
                <span className="w-1.5 h-1.5 rounded-full bg-emerald-300" />
                Connected · {alertCount.toLocaleString()} alert{alertCount !== 1 ? 's' : ''} received
              </span>
            ) : (
              <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-white/10 text-white/70 text-xs font-medium">
                Not connected
              </span>
            )}
            {integration.docsUrl && (
              <a href={integration.docsUrl} target="_blank" rel="noopener noreferrer"
                className="inline-flex items-center gap-1 text-xs text-white/70 hover:text-white transition-colors">
                <ExternalLink className="w-3 h-3" />Docs
              </a>
            )}
          </div>
        </div>

        {/* Body */}
        <div className="flex-1 px-6 py-5 space-y-6">
          <p className="text-sm text-text-secondary">{integration.description}</p>

          <div>
            <h3 className="text-xs font-semibold uppercase tracking-wide text-text-tertiary mb-2">Webhook URL</h3>
            <div className="flex items-center gap-2 bg-surface-secondary rounded-lg px-3 py-2 border border-border">
              <code className="flex-1 text-sm font-mono text-text-primary break-all">{url}</code>
              <CopyButton text={url} />
            </div>
          </div>

          <div className="space-y-5">
            <h3 className="text-xs font-semibold uppercase tracking-wide text-text-tertiary">Setup instructions</h3>
            {integration.setup.map((section, i) => (
              <div key={i}>
                <div className="flex items-center gap-2 mb-2">
                  <span className="w-5 h-5 rounded-full bg-brand-primary text-white text-xs flex items-center justify-center font-medium flex-shrink-0">
                    {i + 1}
                  </span>
                  <p className="text-sm font-medium text-text-primary">{section.title}</p>
                </div>
                <div className="relative group">
                  {section.language ? (
                    <pre className="bg-gray-950 text-gray-100 text-xs rounded-lg px-4 py-3 overflow-x-auto leading-relaxed font-mono whitespace-pre-wrap">
                      {renderSetupContent(section.content, integration.webhookPath)}
                    </pre>
                  ) : (
                    <div className="bg-surface-secondary border border-border rounded-lg px-4 py-3 text-sm text-text-secondary whitespace-pre-line leading-relaxed">
                      {renderSetupContent(section.content, integration.webhookPath)}
                    </div>
                  )}
                  {section.language && (
                    <CopyButton
                      text={renderSetupContent(section.content, integration.webhookPath)}
                      className="absolute top-2 right-2 opacity-0 group-hover:opacity-100 bg-gray-800 hover:bg-gray-700"
                    />
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}

// ─── Integration card ─────────────────────────────────────────────────────────

function IntegrationCard({
  integration, connected, onClick,
}: { integration: Integration; connected: boolean; onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      className="text-left w-full bg-white border border-border rounded-xl p-5 hover:border-brand-primary hover:shadow-sm transition-all group"
    >
      <div className="flex items-center gap-3 mb-3">
        <IntegrationIcon integration={integration} />
        <div className="flex-1 min-w-0 flex items-center gap-2 flex-wrap">
          <span className="font-medium text-text-primary text-sm group-hover:text-brand-primary transition-colors">
            {integration.name}
          </span>
          {connected && (
            <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-emerald-50 text-emerald-700 text-xs font-medium border border-emerald-200">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-500" />
              Connected
            </span>
          )}
        </div>
      </div>
      <p className="text-sm text-text-secondary leading-relaxed">{integration.tagline}</p>
    </button>
  )
}

// ─── Coming-soon card ─────────────────────────────────────────────────────────

function ComingSoonCard({
  item, onRequest,
}: { item: ComingSoon; onRequest: (name: string) => void }) {
  const [imgFailed, setImgFailed] = useState(false)

  return (
    <button
      onClick={() => onRequest(item.name)}
      className="text-left w-full bg-white border border-dashed border-border rounded-xl p-5 hover:border-brand-primary hover:bg-surface-secondary/60 transition-all group"
    >
      <div className="flex items-center gap-3 mb-3">
        {/* Clean white icon container — no colored background */}
        <div className="w-10 h-10 rounded-lg border border-border bg-white shadow-sm flex items-center justify-center flex-shrink-0">
          {imgFailed ? (
            <span className="text-sm font-bold text-text-tertiary">{item.name[0]}</span>
          ) : (
            <img
              src={simpleIconUrl(item.logoSlug)}
              alt={item.name}
              className="w-6 h-6 object-contain opacity-80"
              onError={() => setImgFailed(true)}
            />
          )}
        </div>
        <div className="flex-1 min-w-0 flex items-center gap-2 flex-wrap">
          <span className="font-medium text-text-secondary text-sm">{item.name}</span>
          <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-surface-tertiary text-text-tertiary text-xs font-medium">
            <Lock className="w-2.5 h-2.5" />
            Coming soon
          </span>
        </div>
      </div>
      <p className="text-sm text-text-tertiary leading-relaxed">{item.tagline}</p>
      <p className="mt-2 text-xs text-brand-primary font-medium opacity-0 group-hover:opacity-100 transition-opacity">
        Click to request →
      </p>
    </button>
  )
}

// ─── Request CTA banner ───────────────────────────────────────────────────────

function RequestCTABanner({ onOpen }: { onOpen: () => void }) {
  return (
    <div className="rounded-xl bg-brand-primary-light border border-brand-primary/20 px-8 py-6 flex items-center justify-between gap-6">
      <div>
        <p className="text-sm font-semibold text-text-primary">Don't see your tool?</p>
        <p className="mt-1 text-sm text-text-secondary max-w-md">
          Vote for integrations you need — requests go directly to our backlog as GitHub issues.
          The most-upvoted ones get built first.
        </p>
      </div>
      <button
        onClick={onOpen}
        className="flex-shrink-0 inline-flex items-center gap-2 px-5 py-2.5 bg-brand-primary hover:bg-brand-primary-hover text-white rounded-lg text-sm font-medium transition-colors shadow-sm"
      >
        <MessageSquarePlus className="w-4 h-4" />
        Request an integration
      </button>
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

interface AlertCountResponse { data: unknown[]; total: number }

export function IntegrationsPage() {
  const { user: currentUser } = useAuth()
  const navigate = useNavigate()
  const [selected, setSelected] = useState<Integration | null>(null)
  const [requestModal, setRequestModal] = useState<{ open: boolean; tool: string }>({ open: false, tool: '' })
  const [alertCounts, setAlertCounts] = useState<Record<string, number>>({})
  const [search, setSearch] = useState('')
  const [slackStatus, setSlackStatus] = useState<SlackConfigStatus | null>(null)
  const [showSlackModal, setShowSlackModal] = useState(false)
  const [slackRedirectCopied, setSlackRedirectCopied] = useState(false)
  const [teamsStatus, setTeamsStatus] = useState<TeamsConfigStatus | null>(null)
  const [showTeamsModal, setShowTeamsModal] = useState(false)
  const [telegramStatus, setTelegramStatus] = useState<TelegramConfigStatus | null>(null)
  const [showTelegramModal, setShowTelegramModal] = useState(false)

  useEffect(() => {
    if (currentUser && currentUser.role !== 'admin') {
      navigate('/')
    }
  }, [currentUser, navigate])

  useEffect(() => {
    getSlackConfig().then(setSlackStatus).catch(() => {})
    getTeamsConfig().then(setTeamsStatus).catch(() => {})
    getTelegramConfig().then(setTelegramStatus).catch(() => setTelegramStatus({ configured: false, has_token: false }))
  }, [])

  useEffect(() => {
    Promise.allSettled(
      INTEGRATIONS.map((i) =>
        apiClient
          .get<AlertCountResponse>(`/api/v1/alerts`, { source: i.source, limit: 1 })
          .then((res) => ({ source: i.source, total: res.total }))
          .catch(() => ({ source: i.source, total: 0 })),
      ),
    ).then((results) => {
      const counts: Record<string, number> = {}
      results.forEach((r) => { if (r.status === 'fulfilled') counts[r.value.source] = r.value.total })
      setAlertCounts(counts)
    })
  }, [])

  const query = search.trim().toLowerCase()
  const filteredLive = query
    ? INTEGRATIONS.filter((i) => i.name.toLowerCase().includes(query) || i.tagline.toLowerCase().includes(query))
    : INTEGRATIONS
  const filteredSoon = query
    ? COMING_SOON.filter((i) => i.name.toLowerCase().includes(query) || i.tagline.toLowerCase().includes(query))
    : COMING_SOON

  const connected = filteredLive.filter((i) => (alertCounts[i.source] ?? 0) > 0)
  const notConnected = filteredLive.filter((i) => (alertCounts[i.source] ?? 0) === 0)

  return (
    <div className="flex flex-col h-full">
      {selected && (
        <DetailPanel
          integration={selected}
          connected={(alertCounts[selected.source] ?? 0) > 0}
          alertCount={alertCounts[selected.source] ?? 0}
          onClose={() => setSelected(null)}
        />
      )}
      {requestModal.open && (
        <RequestModal
          initialTool={requestModal.tool}
          onClose={() => setRequestModal({ open: false, tool: '' })}
        />
      )}
      {showSlackModal && (
        <SlackSetupModal
          onClose={() => setShowSlackModal(false)}
          onConnected={() => {
            setShowSlackModal(false)
            getSlackConfig().then(setSlackStatus).catch(() => {})
          }}
        />
      )}
      {showTeamsModal && (
        <TeamsSetupModal
          onClose={() => setShowTeamsModal(false)}
          onConnected={() => {
            setShowTeamsModal(false)
            getTeamsConfig().then(setTeamsStatus).catch(() => {})
          }}
        />
      )}
      {showTelegramModal && (
        <TelegramSetupModal
          onClose={() => setShowTelegramModal(false)}
          onSaved={(status) => {
            setTelegramStatus(status)
            setShowTelegramModal(false)
          }}
          existing={telegramStatus ?? undefined}
        />
      )}

      {/* Page Header */}
      <div className="border-b border-border bg-surface-primary px-6 py-4">
        <h1 className="text-2xl font-semibold text-text-primary">Integrations</h1>
        <p className="mt-1 text-sm text-text-secondary">
          Connect your monitoring tools to automatically create incidents and escalations.
        </p>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-6 space-y-8">
        {/* Search */}
        <div className="relative max-w-sm">
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search integrations…"
            className="w-full pl-9 pr-3 py-2 border border-border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-primary focus:border-transparent"
          />
          <svg className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-tertiary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-4.35-4.35M17 11A6 6 0 1 1 5 11a6 6 0 0 1 12 0z" />
          </svg>
        </div>

        {/* Chat integrations */}
        <section>
          <h2 className="text-sm font-semibold text-text-primary mb-3">Chat</h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {/* Slack card */}
            <div className="rounded-xl border border-border bg-surface-primary p-4 flex flex-col gap-3">
              <div className="flex items-start justify-between">
                <div className="flex items-center gap-3">
                  <img
                    src={simpleIconUrl('slack')}
                    alt="Slack"
                    className="w-8 h-8 flex-shrink-0"
                    onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
                  />
                  <div>
                    <p className="text-sm font-semibold text-text-primary">Slack</p>
                    <p className="text-xs text-text-tertiary">Incident channels &amp; alerts</p>
                  </div>
                </div>
                {slackStatus?.configured && (
                  <span className="flex items-center gap-1 text-xs text-green-600 font-medium">
                    <CheckCircle className="w-3.5 h-3.5" />
                    Connected
                  </span>
                )}
              </div>
              {slackStatus?.configured && slackStatus.workspace_name && (
                <p className="text-xs text-text-tertiary">
                  Workspace: <span className="text-text-secondary font-medium">{slackStatus.workspace_name}</span>
                </p>
              )}
              {slackStatus?.configured && (
                <div className="space-y-1">
                  <p className="text-xs text-text-tertiary">OAuth Redirect URL</p>
                  <div className="flex items-center gap-1.5">
                    <code className="flex-1 text-xs bg-surface-secondary border border-border rounded px-2 py-1 text-text-secondary font-mono truncate">
                      {window.location.origin}/api/v1/auth/slack/callback
                    </code>
                    <button
                      onClick={() => {
                        navigator.clipboard.writeText(`${window.location.origin}/api/v1/auth/slack/callback`)
                        setSlackRedirectCopied(true)
                        setTimeout(() => setSlackRedirectCopied(false), 2000)
                      }}
                      className="flex-shrink-0 p-1 rounded hover:bg-surface-secondary transition-colors"
                      title="Copy redirect URL"
                    >
                      {slackRedirectCopied
                        ? <Check className="w-3 h-3 text-green-600" />
                        : <Copy className="w-3 h-3 text-text-tertiary" />
                      }
                    </button>
                  </div>
                  <p className="text-xs text-text-tertiary">
                    Register this in your Slack app under <strong>OAuth &amp; Permissions → Redirect URLs</strong>
                  </p>
                </div>
              )}
              <button
                onClick={() => setShowSlackModal(true)}
                className="mt-auto flex items-center justify-center gap-1.5 px-3 py-1.5 rounded-lg border border-border text-xs font-medium text-text-primary hover:bg-surface-secondary transition-colors"
              >
                {slackStatus?.configured ? 'Reconfigure' : 'Connect Slack'}
              </button>
            </div>

            {/* Microsoft Teams card */}
            <div className="rounded-xl border border-border bg-surface-primary p-4 flex flex-col gap-3">
              <div className="flex items-start justify-between">
                <div className="flex items-center gap-3">
                  <img
                    src={simpleIconUrl('microsoftteams')}
                    alt="Microsoft Teams"
                    className="w-8 h-8 flex-shrink-0"
                    onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
                  />
                  <div>
                    <p className="text-sm font-semibold text-text-primary">Microsoft Teams</p>
                    <p className="text-xs text-text-tertiary">Incident channels &amp; Adaptive Cards</p>
                  </div>
                </div>
                {teamsStatus?.configured && (
                  <span className="flex items-center gap-1 text-xs text-green-600 font-medium">
                    <CheckCircle className="w-3.5 h-3.5" />
                    Connected
                  </span>
                )}
              </div>
              {teamsStatus?.configured && teamsStatus.team_name && (
                <p className="text-xs text-text-tertiary">
                  Team: <span className="text-text-secondary font-medium">{teamsStatus.team_name}</span>
                </p>
              )}
              <button
                onClick={() => setShowTeamsModal(true)}
                className="mt-auto flex items-center justify-center gap-1.5 px-3 py-1.5 rounded-lg border border-border text-xs font-medium text-text-primary hover:bg-surface-secondary transition-colors"
              >
                {teamsStatus?.configured ? 'Reconfigure' : 'Connect Teams'}
              </button>
            </div>

            {/* Telegram card */}
            <div className="rounded-xl border border-border bg-surface-primary p-4 flex flex-col gap-3">
              <div className="flex items-start justify-between">
                <div className="flex items-center gap-3">
                  <img
                    src={simpleIconUrl('telegram')}
                    alt="Telegram"
                    className="w-8 h-8 flex-shrink-0"
                    onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
                  />
                  <div>
                    <p className="text-sm font-semibold text-text-primary">Telegram</p>
                    <p className="text-xs text-text-tertiary">Incident notifications to any group</p>
                  </div>
                </div>
                {telegramStatus?.configured && (
                  <span className="flex items-center gap-1 text-xs text-green-600 font-medium">
                    <CheckCircle className="w-3.5 h-3.5" />
                    Connected
                  </span>
                )}
              </div>
              {telegramStatus?.configured && telegramStatus.chat_name && (
                <p className="text-xs text-text-tertiary">
                  Group: <span className="text-text-secondary font-medium">{telegramStatus.chat_name}</span>
                </p>
              )}
              <button
                onClick={() => setShowTelegramModal(true)}
                className="mt-auto flex items-center justify-center gap-1.5 px-3 py-1.5 rounded-lg border border-border text-xs font-medium text-text-primary hover:bg-surface-secondary transition-colors"
              >
                {telegramStatus?.configured ? 'Reconfigure' : 'Connect Telegram'}
              </button>
            </div>
          </div>
        </section>

        {/* Connected */}
        {connected.length > 0 && (
          <section>
            <h2 className="text-sm font-semibold text-text-primary mb-3">Connected</h2>
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
              {connected.map((i) => (
                <IntegrationCard key={i.id} integration={i} connected onClick={() => setSelected(i)} />
              ))}
            </div>
          </section>
        )}

        {/* Available */}
        {filteredLive.length > 0 && (
          <section>
            <h2 className="text-sm font-semibold text-text-primary mb-3">
              {connected.length > 0 ? 'Connect to your other tools' : 'Available integrations'}
            </h2>
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
              {notConnected.map((i) => (
                <IntegrationCard key={i.id} integration={i} connected={false} onClick={() => setSelected(i)} />
              ))}
            </div>
          </section>
        )}

        {/* Coming soon */}
        {filteredSoon.length > 0 && (
          <section>
            <div className="flex items-center gap-3 mb-3">
              <h2 className="text-sm font-semibold text-text-primary">Coming soon</h2>
              <span className="text-xs text-text-tertiary">Click any card to request it</span>
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
              {filteredSoon.map((i) => (
                <ComingSoonCard key={i.id} item={i} onRequest={(name) => setRequestModal({ open: true, tool: name })} />
              ))}
            </div>
          </section>
        )}

        {/* No results */}
        {filteredLive.length === 0 && filteredSoon.length === 0 && (
          <div className="text-center py-12">
            <p className="text-text-secondary text-sm mb-3">No integrations match "{search}"</p>
            <button
              onClick={() => setRequestModal({ open: true, tool: search })}
              className="text-sm text-brand-primary hover:underline"
            >
              Request this integration →
            </button>
          </div>
        )}

        {/* CTA banner */}
        <RequestCTABanner onOpen={() => setRequestModal({ open: true, tool: '' })} />
      </div>
    </div>
  )
}
