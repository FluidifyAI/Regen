# Fluidify Regen

**Open-source incident management for teams who own their data.**

[![License: AGPLv3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-ready-blue)](https://github.com/FluidifyAI/Regen/pkgs/container/regen)

Alert routing + incident lifecycle + on-call scheduling + AI post-mortems. One self-hosted platform, no seat tax.

> **Grafana OnCall was archived in March 2026.** Fluidify Regen is the drop-in successor.

---

![Fluidify Regen — Incident Detail](docs/screenshot.png)

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

## Integrations

| Alerts | Chat | AI |
|---|---|---|
| Prometheus Alertmanager | Slack | OpenAI (BYO key) |
| Grafana | Microsoft Teams | |
| AWS CloudWatch | Telegram | |
| Generic webhook | | |

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
