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

> The **one-stop alternative to PagerDuty + incident.io**, with **1-click import from Grafana OnCall/PagerDuty**.

---

<p align="center">
  <img src=".github/assets/demo.gif" alt="Fluidify Regen — incident kanban and Slack collaboration" width="960">
</p>

---

## Features

- Incident lifecycle with immutable timeline
- On-call rotations, layers, overrides
- Escalation policies with multi-step timeouts
- Alert ingestion — Prometheus, Grafana, CloudWatch, generic webhook
- Slack integration — channels, bot commands, timeline sync
- Microsoft Teams integration — Adaptive Cards, bot commands
- 1-click migration from Grafana OnCall/PagerDuty
- AI incident summaries + post-mortem drafts (BYO key — OpenAI/Anthropic/Ollama)
- AI Postmortems, Handoffs, Summaries sych with Slack/Teams
- SSO / SAML — Okta, Azure AD, Google Workspace — **free, always**
- Docker Compose + Kubernetes Helm chart
- PostgreSQL HA + Redis Sentinel support
- No limits on incidents/AI features

---

## Integrations

| Category | Tools |
|---|---|
| **Alert ingestion** | Prometheus Alertmanager · Grafana · AWS CloudWatch · Generic webhook |
| **Chat** | Slack · Microsoft Teams · Telegram |
| **AI** | OpenAI · Anthropic · Ollama (BYO key — local or cloud) |
| **Auth** | SAML 2.0 — Okta · Azure AD · Google Workspace · any compliant IdP |
| **Migration** | Grafana OnCall · PagerDuty |
| **Deploy** | Docker Compose · Kubernetes Helm · bare metal |

---

## Highlights of AI Capabilities

### Incident Summarization

<p align="center">
  <img src=".github/assets/ai-summary-ss.png" alt="Fluidify Regen — Create AI assisted summaries for efficient context transfer across engineers" width="960">
</p>

### Historical Pattern Matching

<p align="center">
  <img src=".github/assets/pattern-matching-ss.png" alt="Fluidify Regen — Pattern matching and learning with historical incidents" width="960">
</p>

### Post-Mortem Agent

<p align="center">
  <img src=".github/assets/pm-ss.png" alt="Fluidify Regen — Editable AI Postmortems for Incidents" width="960">
</p>

### Handoff Digest

<p align="center">
  <img src=".github/assets/handoff-ss.png" alt="Fluidify Regen — Create AI Assisted Handoff Digests for Incident handovers, briefings, status changes, pending actions, sycnhed with Slack/Teams" width="960">
</p>

---

## Fluidify Regen Vs Pagerduty/incident.io/Grafana Oncall

| | Regen | PagerDuty | incident.io | Grafana OnCall |
|---|---|---|---|---|
| Price | Free  | $21–50/user/mo | $30+/user/mo | Archived |
| Self-hosted | ✅ | ❌ | ❌ | ✅ (archived) |
| Open source | AGPLv3 | ❌ | ❌ | Apache 2.0 |
| SSO | ✅ Free | 💰 Paid | 💰 Paid | ✅ Free |
| BYO AI | ✅ | ❌ | ❌ | ❌ |
| Agent-native | ✅ | ❌ | ❌ | ❌ |
| Alert + incident + on-call in one | ✅ | 💰💰💰 Paid | 💰💰💰 Paid | 💰💰💰 Paid |
| 1-Click imports | ✅ | ❌ | ❌ | ❌ |

---

> ## Migrate in 1 click from
>
> - [PagerDuty](docs/migrations/pagerduty.md)
> - [Grafana Oncall](docs/migrations/grafana-oncall.md)

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
helm repo add fluidify https://charts.fluidify.ai
helm repo update
helm install fluidify-regen fluidify/fluidify-regen \
  --set ingress.host=incidents.your-domain.com \
  --set postgresql.auth.password=<strong-password>
```

For production HA (external DB, Redis Sentinel, zero-downtime deploys), see [docs/OPERATIONS.md](docs/OPERATIONS.md).

---

## Built for production

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
- **Authentication**: bcrypt (cost 12), timing-safe comparison, 5-attempt account lockout, HTTP-only SameSite=Strict session cookies
- **No SQL injection surface**: All database access uses GORM parameterized queries — no raw string interpolation
- **Webhook verification**: Slack (HMAC-SHA256 + replay protection), Teams (RSA/OIDC), CloudWatch (RSA + SSRF-safe cert validation)
- **Rate limiting**: Redis Lua script enforcing three tiers — 10/min on auth endpoints, 120/min unauthenticated, 600/min authenticated
- **Security headers**: CSP, HSTS (2 years), X-Frame-Options, X-Content-Type-Options, Permissions-Policy on every response
- **Container hardening**: non-root UID 1001, read-only filesystem, all Linux capabilities dropped
- **CORS**: explicit allowlist via `CORS_ALLOWED_ORIGINS`; dev-only fallback to localhost
- **Frontend**: no `dangerouslySetInnerHTML`, no secrets in bundle, session token never accessible to JavaScript

Review the **[Production Security Checklist](SECURITY.md#11-production-security-checklist)** — TLS, PostgreSQL password, Redis auth, and CORS origins for prerequisiste checklist.

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

> Missing something? [Open an issue](https://github.com/FluidifyAI/Regen/issues/new) — the generic webhook covers most tools today.

---

## License

[AGPLv3](LICENSE) — free forever, including SSO.

---

<p align="center">Built by <a href="https://fluidify.ai">FluidifyAI</a> · your incident data belongs to you</p>
