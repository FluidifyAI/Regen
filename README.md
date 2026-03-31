<p align="center">
  <picture>
    <source media="(prefers-color-scheme: light)" srcset=".github/assets/logo-light.svg">
    <img src=".github/assets/logo-dark.svg" alt="Fluidify" width="520">
  </picture>
</p>

<p align="center">
  <strong>Open-source incident management — self-hosted, agent-native, free forever.</strong>
</p>

<p align="center">
  <a href="https://github.com/FluidifyAI/Regen/actions/workflows/ci.yml"><img src="https://github.com/FluidifyAI/Regen/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/FluidifyAI/Regen/releases"><img src="https://img.shields.io/badge/release-v1.0.0-6366f1" alt="Release"></a>
  <a href="https://discord.gg/b6PSdhzDa"><img src="https://img.shields.io/discord/1487668241718579342?label=discord&logo=discord&logoColor=white&color=5865F2" alt="Discord"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-AGPLv3-blue.svg" alt="License: AGPLv3"></a>
  <a href="https://github.com/FluidifyAI/Regen/pkgs/container/regen"><img src="https://img.shields.io/badge/docker-ready-0ea5e9" alt="Docker"></a>
  <a href="https://goreportcard.com/report/github.com/FluidifyAI/Regen"><img src="https://goreportcard.com/badge/github.com/FluidifyAI/Regen" alt="Go Report Card"></a>
</p>

---

> **Grafana OnCall was archived in March 2026.** Fluidify Regen is the drop-in successor.

---

## Quick Start

```bash
git clone https://github.com/FluidifyAI/Regen.git
cd Regen
docker-compose up -d
```

Open **http://localhost:8080** — the API and UI are ready. No configuration required to start receiving alerts.

To connect Slack, Teams, or configure SSO, use the in-app setup wizard under **Settings → Integrations**.

### Send a test alert

```bash
curl -X POST http://localhost:8080/api/v1/webhooks/prometheus \
  -H "Content-Type: application/json" \
  -d '{
    "receiver": "fluidify-regen",
    "status": "firing",
    "alerts": [{
      "status": "firing",
      "labels": {"alertname": "TestAlert", "severity": "critical"},
      "annotations": {"summary": "Test alert from curl"},
      "startsAt": "2024-01-01T00:00:00Z"
    }]
  }'
```

An incident is created automatically. If Slack is configured, a channel appears within seconds.

---

## Features

| | Community (AGPLv3, free) | Enterprise (paid) |
|---|---|---|
| Alert ingestion (Prometheus, Grafana, CloudWatch, generic) | ✅ | ✅ |
| Incident lifecycle with immutable timeline | ✅ | ✅ |
| On-call rotations, layers, overrides | ✅ | ✅ |
| Escalation policies | ✅ | ✅ |
| Slack integration (channels, bot commands, timeline sync) | ✅ | ✅ |
| Microsoft Teams integration (Adaptive Cards, bot commands) | ✅ | ✅ |
| AI incident summaries + post-mortem drafts (BYO OpenAI key) | ✅ | ✅ |
| SSO / SAML (Okta, Azure AD, Google Workspace) | ✅ | ✅ |
| Docker Compose + Kubernetes Helm chart | ✅ | ✅ |
| SCIM user provisioning | ❌ | ✅ |
| Audit log export (SOC2-ready) | ❌ | ✅ |
| Role-based access control (RBAC) | ❌ | ✅ |
| Retention policies | ❌ | ✅ |
| Priority support + SLA | ❌ | ✅ |

> SSO is free. Gating SSO behind a paid tier is user-hostile. We stay off [sso.tax](https://sso.tax).

---

## AI Agents

Fluidify Regen ships with AI agents that work autonomously during and after incidents. All agents use your own OpenAI key — your incident data never leaves your infrastructure.

### Incident Summarization Agent

Reads the full incident timeline and any linked Slack thread, then writes a concise summary of what happened, what was done, and current status. Useful for commanders joining mid-incident or for shift handoffs.

```bash
curl -X POST http://localhost:8080/api/v1/incidents/INC-042/summarize \
  -H "Authorization: Bearer YOUR_TOKEN"
```

```json
{
  "summary": "Database replica lag exceeded 60s at 14:32 UTC, triggering alert. On-call engineer acknowledged at 14:38. Root cause identified as a long-running migration on the primary. Migration was killed at 14:51, replication caught up by 14:59. No data loss.",
  "generated_at": "2024-01-15T15:02:00Z"
}
```

### Post-Mortem Agent

Generates a structured post-mortem draft from the incident timeline, status changes, and linked alerts. Extracts contributing factors and action items automatically. Supports custom templates.

```bash
curl -X POST http://localhost:8080/api/v1/incidents/INC-042/postmortem/generate \
  -H "Authorization: Bearer YOUR_TOKEN"
```

The draft appears in the UI immediately, ready to edit and publish.

### Handoff Digest

Generates a shift-handoff briefing covering all open incidents, recent status changes, and pending action items — delivered to Slack or Teams at the start of each shift.

### What makes this different

| | Fluidify Regen | PagerDuty AI | incident.io AI |
|---|---|---|---|
| BYO API key | ✅ | ❌ | ❌ |
| Data stays on your infra | ✅ | ❌ | ❌ |
| Custom post-mortem templates | ✅ | ❌ | ✅ |
| Open source prompts | ✅ | ❌ | ❌ |
| Cost | Your OpenAI bill | Bundled in seat price | Bundled in seat price |

---

## Integrations

### Available now

| Category | Integrations |
|---|---|
| **Alert ingestion** | Prometheus Alertmanager, Grafana, AWS CloudWatch, Generic webhook |
| **Chat & incident channels** | Slack, Microsoft Teams, Telegram |
| **AI** | OpenAI (BYO key — GPT-4o, GPT-4, GPT-3.5) |
| **Auth** | SAML 2.0 (Okta, Azure AD, Google Workspace, any compliant IdP) |
| **Deployment** | Docker Compose, Kubernetes (Helm) |

### Coming soon

| Category | Integrations |
|---|---|
| **Alert ingestion** | Datadog, New Relic, Sentry, Dynatrace, Elastic / Kibana, Zabbix, Nagios, Uptime Kuma, Betterstack |
| **Migration / import** | PagerDuty (schedules + escalation policies), Opsgenie, Splunk On-Call (VictorOps) |
| **Post-mortem export** | Confluence, Notion, Jira |
| **AI providers** | Anthropic Claude, local LLMs via Ollama |
| **Chat** | Discord |

> Missing an integration? [Open an issue](https://github.com/FluidifyAI/Regen/issues) or send a PR — the generic webhook covers most tools today.

---

## Kubernetes

```bash
helm install fluidify-regen deploy/helm/fluidify-regen \
  --set ingress.host=incidents.your-domain.com \
  --set postgresql.auth.password=<strong-password>
```

For production (external DB + Redis), managed PostgreSQL, HA configuration, and observability setup, see [docs/OPERATIONS.md](docs/OPERATIONS.md).

---

## Comparison

| | Fluidify Regen | PagerDuty | incident.io |
|---|---|---|---|
| Price | Free / flat enterprise | ~$21–50/user/mo | ~$30+/user/mo |
| Self-hosted | ✅ | ❌ | ❌ |
| Open source | AGPLv3 | ❌ | ❌ |
| SSO | Free | Paid tier | Paid tier |
| BYO AI | ✅ | ❌ | ❌ |
| Alert + incident + on-call in one tool | ✅ | ⚠️ | ⚠️ |

---

## Roadmap

### Shipping next

- **PagerDuty import** — migrate schedules and escalation policies with a single CLI command
- **Confluence / Notion export** — publish post-mortems directly from the UI
- **RBAC** — viewer / responder / admin roles (Enterprise)
- **SCIM provisioning** — automated user lifecycle via Okta, Azure AD (Enterprise)
- **Audit log export** — SOC2-ready tamper-evident logs (Enterprise)

### AI agent roadmap

The current agents (summarization, post-mortems, handoff) are the foundation. The direction is fully autonomous incident response:

| Agent | What it does |
|---|---|
| **Triage agent** | Auto-assigns severity, tags, and suggested commander based on alert patterns |
| **Root cause agent** | Correlates metrics, logs, and recent deploys to surface likely root causes |
| **Runbook agent** | Matches the incident to known runbooks and surfaces the relevant steps |
| **Noise reduction agent** | Learns alert patterns over time and suppresses known-noisy, low-signal alerts |
| **Conversational agent** | Answer questions mid-incident in Slack/Teams: "What changed in the last 2 hours?" |

> These are on the roadmap — not yet shipped. Star the repo to follow along.

---

## Contributing

Issues, PRs, and feature requests are welcome.

```bash
# Start backend + dependencies
docker-compose up -d db redis api

# Run frontend locally with hot reload
cd frontend && npm install && npm run dev
```

See [Makefile](Makefile) for all available commands (`make help`).

---

## License

- **Community**: [AGPLv3](LICENSE)
- **Enterprise**: Proprietary — [enterprise@fluidify.ai](mailto:enterprise@fluidify.ai)

---

Built for teams who believe incident data belongs to them.
