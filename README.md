<p align="center">
  <picture>
    <source media="(prefers-color-scheme: light)" srcset=".github/assets/logo-light.svg">
    <img src=".github/assets/logo-dark.svg" alt="Fluidify" width="520">
  </picture>
</p>

<p align="center">
  Part of the <a href="https://fluidify.ai">FluidifyAI</a> open-source suite
</p>

<p align="center">
  <a href="https://github.com/FluidifyAI/Regen/actions/workflows/ci.yml"><img src="https://github.com/FluidifyAI/Regen/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/FluidifyAI/Regen/releases"><img src="https://img.shields.io/github/v/release/FluidifyAI/Regen" alt="Release"></a>
  <a href="https://discord.gg/b6PSdhzDa"><img src="https://img.shields.io/discord/1487668241718579342?label=discord&logo=discord&logoColor=white&color=5865F2" alt="Discord"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-AGPLv3-blue.svg" alt="License: AGPLv3"></a>
  <a href="https://github.com/FluidifyAI/Regen/pkgs/container/regen"><img src="https://img.shields.io/badge/docker-ready-0ea5e9" alt="Docker"></a>
  <a href="https://goreportcard.com/report/github.com/FluidifyAI/Regen/backend"><img src="https://goreportcard.com/badge/github.com/FluidifyAI/Regen/backend" alt="Go Report Card"></a>
</p>

---

> Unlimited alert noise reduction and incidents, unlimited on-call schedules, and unlimited AI postmortems and handoff digests.

> The **one-stop alternative to PagerDuty + incident.io**, with **1-click import from Grafana OnCall/Pagerduty**.

---

<p align="center">
  <img src=".github/assets/demo.gif" alt="Fluidify Regen — incident kanban and Slack collaboration" width="960">
</p>

---

## Features

- Alert ingestion — Prometheus, Grafana, CloudWatch, generic webhook
- Incident lifecycle with immutable timeline
- On-call rotations, layers, overrides
- Escalation policies with multi-step timeouts
- Slack integration — channels, bot commands, timeline sync
- Microsoft Teams integration — Adaptive Cards, bot commands
- AI incident summaries + post-mortem drafts (BYO OpenAI key)
- SSO / SAML — Okta, Azure AD, Google Workspace — **free, always**
- Docker Compose + Kubernetes Helm chart
- PostgreSQL HA + Redis Sentinel support
- 1 click import from Grafana Oncall/Pagerduty
- No limits on incidents/AI features

---

## AI Agents

### Incident Summarization

Reads the full incident timeline and linked Slack thread, then writes a concise summary of what happened, what was done, and current status. Useful for commanders joining mid-incident or shift handoffs.

```bash
curl -X POST http://localhost:8080/api/v1/incidents/INC-042/summarize \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### Historical Pattern Matching

Stop re-diagnosing solved problems. When an incident fires, Regen scans your history for matches — same service, alert fingerprint, timeline — and surfaces them in Slack:

> 🤖 **Regen:** This looks like INC-157 from November (Redis memory eviction, resolved in 18 min). [View timeline →]


### Post-Mortem Agent

Generates a structured post-mortem draft from the incident timeline, status changes, and linked alerts. Extracts contributing factors and action items automatically. Supports custom templates.

```bash
curl -X POST http://localhost:8080/api/v1/incidents/INC-042/postmortem/generate \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### Handoff Digest

Generates a shift-handoff briefing covering all open incidents, recent status changes, and pending action items — delivered to Slack or Teams at the start of each shift.

> Want to help build this? The agent scaffolding is open. **[See the roadmap issues →](https://github.com/FluidifyAI/Regen/issues)**

---

## Integrations

| Category | Tools |
|---|---|
| **Alert ingestion** | Prometheus Alertmanager · Grafana · AWS CloudWatch · Generic webhook |
| **Chat** | Slack · Microsoft Teams · Telegram |
| **AI** | OpenAI GPT-4o / GPT-4 / GPT-3.5 (BYO key) |
| **Auth** | SAML 2.0 — Okta · Azure AD · Google Workspace · any compliant IdP |
| **Deploy** | Docker Compose · Kubernetes Helm · bare metal |

> Missing something? [Open an issue](https://github.com/FluidifyAI/Regen/issues/new) — the generic webhook covers most tools today.

---

## Comparison

| | Fluidify Regen | PagerDuty | incident.io | Grafana OnCall |
|---|---|---|---|---|
| Price | Free / flat enterprise | ~$21–50/user/mo | ~$30+/user/mo | Archived |
| Self-hosted | ✅ | ❌ | ❌ | ✅ (archived) |
| Open source | AGPLv3 | ❌ | ❌ | Apache 2.0 |
| SSO | ✅ Free | 💰 Paid tier | 💰 Paid tier | ✅ Free |
| BYO AI | ✅ | ❌ | ❌ | ❌ |
| Agent-native | ✅ | ❌ | ❌ | ❌ |
| Alert + incident + on-call in one | ✅ | ⚠️ | ⚠️ | ⚠️ |
| 1-Click imports | ✅ | ❌ | ❌ | ❌ |

---

## Coming from Grafana OnCall?

Grafana OnCall was archived in March 2026. Fluidify Regen is built to be the drop-in OSS successor — same self-hosted model.

Point your Alertmanager at Regen and you're receiving alerts in minutes:

```yaml
# alertmanager.yml
receivers:
  - name: fluidify-regen
    webhook_configs:
      - url: http://your-regen-host:8080/api/v1/webhooks/prometheus
```

**One-click migration from Grafana OnCall** — import your users, on-call schedules, and escalation policies in under 60 seconds:

1. Go to **Settings → Migrations**
2. Enter your Grafana OnCall URL and API token
3. Preview exactly what will be imported, then click **Import everything**

Your new Regen webhook URLs are shown immediately — just update them in Grafana Alertmanager and you're live. [Full migration guide →](docs/migrations/grafana-oncall.md)

---


## Install

Three ways to run — pick what fits your stack:

### Docker (fastest)

```bash
docker pull ghcr.io/fluidifyai/regen:latest
```

Need the full stack? One command:

```bash
curl -O https://raw.githubusercontent.com/FluidifyAI/Regen/main/docker-compose.yml
docker-compose up -d
```

Open **http://localhost:8080** — API and UI are ready. No configuration required to start receiving alerts.

### Docker Compose (recommended for self-hosting)

```bash
git clone https://github.com/FluidifyAI/Regen.git
cd Regen
cp .env.example .env   # edit as needed
docker-compose up -d
```

### Kubernetes (Helm)

```bash
helm install fluidify-regen deploy/helm/fluidify-regen \
  --set ingress.host=incidents.your-domain.com \
  --set postgresql.auth.password=<strong-password>
```

For production HA (external DB, Redis Sentinel, zero-downtime deploys), see [docs/OPERATIONS.md](docs/OPERATIONS.md).

## Built for production

Fluidify Regen is designed to run as reliably as the tools it monitors.

### Benchmark results (HA stack · Apple M2 / Colima · 2026-03-31)

| Scenario | Result |
|---|---|
| Webhook ingestion p99 | **< 10 ms** (target: < 200 ms) |
| Webhook sustained p50 / p95 | **1.55 ms / 2.82 ms** |
| API reads p95 (list / detail) | **4.42 ms / 2.83 ms** |
| Peak throughput (burst test) | **3,917 RPS — 0 × 5xx** |
| PostgreSQL failover RTO | **11 s** (Patroni + HAProxy, target: < 60 s) |
| Redis failover RTO | **5 s** (Sentinel 3-node quorum) |
| In-flight requests lost on rolling deploy | **0** |

> Production numbers will be higher — these were captured on a single-machine local HA stack.
> Reproduce yourself: `make load-test` and `make chaos-db`. Full methodology in [docs/RELIABILITY.md](docs/RELIABILITY.md).

### How it stays up

- **Zero-downtime deploys** — rolling restarts drain in-flight requests before pod shutdown (SIGTERM → 30 s drain → exit)
- **PostgreSQL HA** — Patroni manages automatic primary election; HAProxy re-routes to the new primary within one health-check interval (3 s). No app restart, no config change.
- **Redis Sentinel** — 3-node quorum detects primary loss; workers reconnect to new master automatically
- **Kubernetes-native** — HPA, health-gated rolling deploys, resource limits out of the box
- **Webhook flood protection** — rate limiter returns 429 before the DB sees load spikes; validated at 3,917 RPS with zero OOM events
- **Full observability** — `/metrics` (Prometheus) + pre-built Grafana dashboard in `deploy/grafana/`

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

An incident is created automatically. If Slack is configured, a dedicated channel appears within seconds.

---

## Security

Fluidify Regen is built with security as a first-class concern:

- **Authentication**: bcrypt (cost 12), timing-safe comparison, 5-attempt account lockout, HTTP-only SameSite=Strict session cookies
- **No SQL injection surface**: All database access uses GORM parameterized queries — no raw string interpolation
- **Webhook verification**: Slack (HMAC-SHA256 + replay protection), Teams (RSA/OIDC), CloudWatch (RSA + SSRF-safe cert validation)
- **Rate limiting**: Redis Lua script enforcing three tiers — 10/min on auth endpoints, 120/min unauthenticated, 600/min authenticated
- **Security headers**: CSP, HSTS (2 years), X-Frame-Options, X-Content-Type-Options, Permissions-Policy on every response
- **Container hardening**: non-root UID 1001, read-only filesystem, all Linux capabilities dropped
- **CORS**: explicit allowlist via `CORS_ALLOWED_ORIGINS`; dev-only fallback to localhost
- **Frontend**: no `dangerouslySetInnerHTML`, no secrets in bundle, session token never accessible to JavaScript

Before going to production, review the **[Production Security Checklist](SECURITY.md#11-production-security-checklist)** — TLS, PostgreSQL password, Redis auth, and CORS origins must all be configured.

Full security architecture: [SECURITY.md](SECURITY.md)

---

## Contributing

Issues, PRs, and feature requests are welcome. If you're coming from Grafana OnCall, your experience building on that platform is exactly what we need.

```bash
# Start backend + dependencies
docker-compose up -d db redis

# Run backend with hot reload
cd backend && go run ./cmd/regen/... serve

# Run frontend with hot reload
cd frontend && npm install && npm run dev
```

See [CONTRIBUTING.md](CONTRIBUTING.md) and [Makefile](Makefile) (`make help`) for all commands. For bigger changes, [open a discussion first](https://github.com/FluidifyAI/Regen/discussions).

---

## License

[AGPLv3](LICENSE) — free forever, including SSO.

---

<p align="center">Built by <a href="https://fluidify.ai">FluidifyAI</a> · your incident data belongs to you</p>
