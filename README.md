# OpenIncident

**Open-source incident management for teams who own their data.**

incident.io + PagerDuty, self-hosted, with BYO-AI.

---

## Why OpenIncident?

| Problem | Our Solution |
|---------|--------------|
| **$100k/year** on incident tooling for a 200-person team | **Free** open-source core, flat enterprise pricing |
| **Data sovereignty** concerns blocking SaaS adoption | **Self-hosted** — your data never leaves your infrastructure |
| **Tool fragmentation** — alerts here, incidents there, post-mortems somewhere else | **Unified platform** — alerts, incidents, scheduling, AI in one place |
| **Grafana OnCall archived** in March 2026 | **Spiritual successor** with full incident lifecycle |

---

## Features

### Core (Free, AGPLv3)

- **Alert Ingestion** — Prometheus, Grafana, CloudWatch, generic webhooks
- **Incident Management** — Full lifecycle with immutable timeline
- **Slack Integration** — Auto-create channels, bidirectional sync
- **On-Call Scheduling** — Rotations, layers, overrides
- **Escalation Policies** — Multi-tier escalation with timeouts
- **AI Summarization** — Incident summaries, post-mortem drafts (BYO OpenAI key)
- **Docker & Kubernetes** — Deploy anywhere

### Enterprise (Paid License)

- SSO/SAML (Okta, Azure AD, Google)
- SCIM user provisioning
- Audit log export
- Role-based access control (RBAC)
- Data retention policies
- Priority support

---

## Quick Start

### Prerequisites
- Docker & Docker Compose
- Slack workspace (for Slack integration)

### 1. Clone and Configure

```bash
git clone https://github.com/yourusername/openincident.git
cd openincident
cp .env.example .env
# Edit .env with your Slack credentials
```

### 2. Start Services

```bash
docker-compose up -d
```

### 3. Access the UI

Open http://localhost:3000

### 4. Configure Prometheus (Optional)

Add to your `alertmanager.yml`:

```yaml
receivers:
  - name: openincident
    webhook_configs:
      - url: http://localhost:8080/api/v1/webhooks/prometheus
        send_resolved: true
```

---

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Prometheus    │     │     Grafana     │     │   CloudWatch    │
└────────┬────────┘     └────────┬────────┘     └────────┬────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                                 ▼
                    ┌────────────────────────┐
                    │    OpenIncident API    │
                    │  (Go + Gin)            │
                    └────────────┬───────────┘
                                 │
              ┌──────────────────┼──────────────────┐
              │                  │                  │
              ▼                  ▼                  ▼
       ┌──────────┐       ┌──────────┐       ┌──────────┐
       │PostgreSQL│       │  Redis   │       │  Slack   │
       └──────────┘       └──────────┘       └──────────┘
```

---

## Documentation

| Document | Description |
|----------|-------------|
| [CLAUDE.md](docs/CLAUDE.md) | Project context and build guide |
| [PRODUCT.md](docs/PRODUCT.md) | Product vision, roadmap, business model |
| [ARCHITECTURE.md](docs/ARCHITECTURE.md) | System design, data models, APIs |
| [DECISIONS.md](docs/DECISIONS.md) | Architecture Decision Records |

---

## Roadmap

- [x] **v0.1** — Prometheus → Incident → Slack
- [ ] **v0.2** — Incident lifecycle, timeline
- [ ] **v0.3** — Multi-source alerts, routing
- [ ] **v0.4** — On-call rotations
- [ ] **v0.5** — Escalation policies
- [ ] **v0.6** — AI summarization
- [ ] **v0.7** — Post-mortem generation
- [ ] **v0.8** — Microsoft Teams
- [ ] **v0.9** — Enterprise features (SSO, RBAC)
- [ ] **v1.0** — Production ready

---

## API Example

### Create an Incident

```bash
curl -X POST http://localhost:8080/api/v1/incidents \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "title": "Database connection errors",
    "severity": "high",
    "description": "Multiple services reporting DB timeouts"
  }'
```

### Receive a Prometheus Alert

```bash
curl -X POST http://localhost:8080/api/v1/webhooks/prometheus \
  -H "Content-Type: application/json" \
  -d '{
    "receiver": "openincident",
    "status": "firing",
    "alerts": [{
      "status": "firing",
      "labels": {
        "alertname": "HighErrorRate",
        "severity": "critical"
      },
      "annotations": {
        "summary": "Error rate above 5%"
      },
      "startsAt": "2024-01-15T10:00:00Z"
    }]
  }'
```

---

## Configuration

### Environment Variables

```env
# Required
DATABASE_URL=postgresql://user:pass@localhost:5432/openincident
REDIS_URL=redis://localhost:6379

# Slack Integration
SLACK_BOT_TOKEN=xoxb-...
SLACK_SIGNING_SECRET=...

# Optional: AI Features
OPENAI_API_KEY=sk-...

# Optional: App Settings
PORT=8080
LOG_LEVEL=info
APP_ENV=production
```

### Slack App Setup

1. Create a new Slack app at https://api.slack.com/apps
2. Add these OAuth scopes:
   - `channels:manage`
   - `channels:read`
   - `channels:history`
   - `chat:write`
   - `commands`
   - `users:read`
3. Install to your workspace
4. Copy Bot Token and Signing Secret to `.env`

---

## Contributing

We welcome contributions! Please read our [Contributing Guide](CONTRIBUTING.md) first.

### Development Setup

```bash
# Clone repo
git clone https://github.com/yourusername/openincident.git
cd openincident

# Start dependencies
docker-compose up -d db redis

# Run backend
cd backend && go run ./cmd/openincident

# Run frontend (separate terminal)
cd frontend && npm install && npm run dev
```

### Running Tests

```bash
make test
```

---

## Comparison

| Feature | OpenIncident | incident.io | PagerDuty |
|---------|--------------|-------------|-----------|
| Alert management | ✅ | ❌ | ✅ |
| Incident coordination | ✅ | ✅ | ⚠️ |
| On-call scheduling | ✅ | ❌ | ✅ |
| Self-hosted | ✅ | ❌ | ❌ |
| Open source | ✅ | ❌ | ❌ |
| BYO AI/LLM | ✅ | ❌ | ❌ |
| Pricing | Free / Flat | Per-seat | Per-seat |

---

## License

- **Core**: [AGPLv3](LICENSE)
- **Enterprise**: Proprietary (contact us)

---

## Support

- **Community**: [GitHub Discussions](https://github.com/yourusername/openincident/discussions)
- **Issues**: [GitHub Issues](https://github.com/yourusername/openincident/issues)
- **Enterprise**: enterprise@openincident.io

---

Built with ❤️ for teams who believe incident data belongs to them.