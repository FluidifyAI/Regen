# CLAUDE.md — Project Context for Claude Code

> **Read this file completely before starting any work.**
> This is the source of truth for what we're building and how.

---

## What Is This Project?

**OpenIncident** is an open-source incident management platform that combines:
- **Alert Management** (like PagerDuty/Opsgenie) — receive, dedupe, route alerts
- **Incident Coordination** (like incident.io) — lifecycle, timeline, Slack integration
- **On-Call Scheduling** — rotations, escalations, overrides
- **AI Assistance** — summarization, post-mortem drafting (BYO API key)

**One-liner:** "incident.io + PagerDuty, open-source, self-hosted, BYO-AI."

---

## Why Does This Exist?

1. **The SaaS Tax**: incident.io/PagerDuty charge $30-50/user. $100k/year for a 200-person team.
2. **Data Sovereignty**: Regulated industries can't send incident data to third-party SaaS.
3. **Tool Fragmentation**: Separate tools for alerting, incidents, and post-mortems.
4. **Grafana OnCall Archived**: March 2026, OSS users need an alternative.

---

## Business Model

**Open Core** (like PostHog, Supabase, GitLab):

| Tier | License | Features |
|------|---------|----------|
| Community | AGPLv3 | Everything: alerts, incidents, scheduling, Slack, AI |
| Enterprise | Proprietary | SSO/SAML, audit logs, RBAC, SCIM, retention policies |

**The OSS version must be a fully functional Ferrari. Enterprise is insurance and valet service.**

---

## Tech Stack

| Component | Technology | Why |
|-----------|------------|-----|
| Backend | Go + Gin | Single binary, performance |
| Database | PostgreSQL | JSONB, strong consistency, audit trails |
| Queue/Cache | Redis | Async jobs, caching |
| Frontend | React + TypeScript | Standard, maintainable |
| Chat | Slack (first), Teams (v0.8) | Where incidents happen |
| AI | OpenAI API (BYO key) | No infra to manage |
| Deployment | Docker Compose + Kubernetes | Self-hosted flexibility |

---

## Project Structure

```
openincident/
├── CLAUDE.md                 # THIS FILE
├── README.md                 # Public docs
├── LICENSE                   # AGPLv3
├── Makefile
├── docker-compose.yml
│
├── docs/
│   ├── PRODUCT.md            # Vision, roadmap
│   ├── ARCHITECTURE.md       # System design
│   └── DECISIONS.md          # ADRs
│
├── backend/
│   ├── Dockerfile
│   ├── go.mod
│   ├── cmd/openincident/
│   │   └── main.go
│   ├── internal/
│   │   ├── config/
│   │   ├── api/
│   │   │   ├── routes.go
│   │   │   ├── middleware/
│   │   │   └── handlers/
│   │   ├── models/
│   │   ├── services/
│   │   ├── repository/
│   │   └── worker/
│   └── migrations/
│
├── frontend/
│   ├── Dockerfile
│   ├── package.json
│   ├── src/
│   │   ├── App.tsx
│   │   ├── api/
│   │   ├── components/
│   │   └── pages/
│
└── deploy/
    ├── docker-compose.prod.yml
    └── kubernetes/
```

---

## Data Models

### Alert
```go
type Alert struct {
    ID          uuid.UUID
    ExternalID  string      // From source (e.g., Prometheus fingerprint)
    Source      string      // "prometheus", "grafana", etc.
    Status      string      // "firing", "resolved"
    Severity    string      // "critical", "warning", "info"
    Title       string
    Description string
    Labels      JSONB
    Annotations JSONB
    RawPayload  JSONB
    StartedAt   time.Time
    EndedAt     *time.Time
    ReceivedAt  time.Time   // IMMUTABLE, server-generated
}
```

### Incident
```go
type Incident struct {
    ID              uuid.UUID
    IncidentNumber  int         // INC-001, INC-002
    Title           string
    Slug            string      // URL/channel friendly
    Status          string      // "triggered", "acknowledged", "resolved"
    Severity        string      // "critical", "high", "medium", "low"
    Summary         string
    
    SlackChannelID   string
    SlackChannelName string
    
    CreatedAt       time.Time   // IMMUTABLE
    TriggeredAt     time.Time   // IMMUTABLE
    AcknowledgedAt  *time.Time
    ResolvedAt      *time.Time
    
    CreatedByType   string      // "system", "user"
    CreatedByID     string
    CommanderID     *uuid.UUID
}
```

### Timeline Entry (IMMUTABLE)
```go
type TimelineEntry struct {
    ID          uuid.UUID
    IncidentID  uuid.UUID
    Timestamp   time.Time   // IMMUTABLE, server-generated
    Type        string      // "status_changed", "message", "alert_linked"
    ActorType   string      // "user", "system", "slack_bot"
    ActorID     string
    Content     JSONB
}
```

### Schedule
```go
type Schedule struct {
    ID          uuid.UUID
    Name        string
    Description string
    Timezone    string
    Layers      []ScheduleLayer
    Overrides   []ScheduleOverride
}

type ScheduleLayer struct {
    ID            uuid.UUID
    ScheduleID    uuid.UUID
    Name          string
    OrderIndex    int
    RotationType  string      // "daily", "weekly", "custom"
    RotationStart time.Time
    ShiftDuration time.Duration
    Participants  []User
}
```

---

## API Endpoints

### Health
```
GET  /health              → { "status": "ok" }
GET  /ready               → { "status": "ready", "database": "ok", "redis": "ok" }
```

### Webhooks
```
POST /api/v1/webhooks/prometheus    → Alertmanager payload
POST /api/v1/webhooks/grafana       → Grafana webhook
POST /api/v1/webhooks/cloudwatch    → SNS notification
POST /api/v1/webhooks/generic       → Generic schema
```

### Incidents
```
GET    /api/v1/incidents            → List incidents
GET    /api/v1/incidents/:id        → Get incident
POST   /api/v1/incidents            → Create incident (manual)
PATCH  /api/v1/incidents/:id        → Update incident
GET    /api/v1/incidents/:id/timeline → Get timeline
POST   /api/v1/incidents/:id/timeline → Add timeline entry
POST   /api/v1/incidents/:id/alerts   → Link alert
```

### Alerts
```
GET    /api/v1/alerts               → List alerts
GET    /api/v1/alerts/:id           → Get alert
```

### Schedules (v0.4+)
```
GET    /api/v1/schedules            → List schedules
GET    /api/v1/schedules/:id        → Get schedule
POST   /api/v1/schedules            → Create schedule
GET    /api/v1/schedules/:id/oncall → Who's on call
POST   /api/v1/schedules/:id/overrides → Create override
```

### Escalation Policies (v0.5+)
```
GET    /api/v1/escalation-policies
GET    /api/v1/escalation-policies/:id
POST   /api/v1/escalation-policies
```

### AI (v0.6+)
```
POST   /api/v1/incidents/:id/summarize        → Generate summary
POST   /api/v1/incidents/:id/postmortem/generate → Generate post-mortem
```

---

## Phased Build Plan

### Phase 1: Foundation (v0.1–v0.3)

**v0.1 — Alert to Slack (Weeks 1–3)**
- [ ] Project setup (Go, PostgreSQL, Redis, Docker Compose)
- [ ] Prometheus webhook endpoint
- [ ] Alert model and storage
- [ ] Incident auto-creation from alerts
- [ ] Slack channel auto-creation
- [ ] Basic incident list UI

**v0.2 — Incident Lifecycle (Weeks 4–5)**
- [ ] Status workflow (triggered → acknowledged → resolved)
- [ ] Timeline entries (immutable)
- [ ] Slack bidirectional sync
- [ ] Manual incident creation from Slack (/incident new)
- [ ] Incident detail page

**v0.3 — Multi-Source Alerts (Weeks 6–8)**
- [ ] Grafana webhook
- [ ] CloudWatch webhook
- [ ] Generic webhook
- [ ] Alert deduplication
- [ ] Alert grouping rules
- [ ] Routing rules

### Phase 2: On-Call (v0.4–v0.5)

**v0.4 — Rotations (Weeks 9–11)**
- [ ] Schedule model
- [ ] Layer-based rotations
- [ ] Override scheduling
- [ ] Calendar UI
- [ ] Who's on call API
- [ ] Slack shift notifications

**v0.5 — Escalations (Weeks 12–14)**
- [ ] Escalation policy model
- [ ] Multi-step escalation
- [ ] Timeout triggers
- [ ] Escalation UI
- [ ] PagerDuty import

### Phase 3: AI (v0.6–v0.7)

**v0.6 — Summarization (Weeks 15–17)**
- [ ] OpenAI integration (BYO key)
- [ ] Incident summary generation
- [ ] Slack thread summarization
- [ ] Handoff digest
- [ ] Summary in UI

**v0.7 — Post-Mortems (Weeks 18–20)**
- [ ] Post-mortem model
- [ ] Auto-generated drafts
- [ ] Template system
- [ ] Action item extraction
- [ ] Export (Confluence/Notion)

### Phase 4: Enterprise (v0.8–v1.0)

**v0.8 — Teams Integration (Weeks 21–23)**
- [ ] Teams channel creation
- [ ] Teams bot
- [ ] Bidirectional sync
- [ ] Teams notifications

**v0.9 — Enterprise Features (Weeks 24–26)**
- [ ] SSO/SAML
- [ ] RBAC
- [ ] Audit log export
- [ ] SCIM provisioning
- [ ] Retention policies

**v1.0 — Production Ready (Weeks 27–28)**
- [ ] Kubernetes Helm chart
- [ ] HA documentation
- [ ] Security hardening
- [ ] Performance tuning
- [ ] Public launch

---

## Key Architectural Principles

### 1. Immutable Audit Trail
- All `received_at`, `created_at`, `timestamp` fields are server-generated
- Timeline entries cannot be updated or deleted
- This is non-negotiable for compliance

### 2. Slack-First, Chat-Agnostic
- Build Slack integration first
- Use `ChatService` interface for abstraction
- Teams comes in v0.8 without rewriting core

### 3. Integrate, Don't Replace
- We sit alongside Prometheus, Grafana, Datadog
- Don't ask users to replace their observability stack
- Webhooks are the universal integration point

### 4. AI is Optional
- Product works 100% without AI configured
- BYO API key model (user's data, user's cost)
- Abstract provider interface for future local LLM support

---

## Development Commands

```bash
# Start all services
make dev

# Run backend only
make backend

# Run frontend only
make frontend

# Run database migrations
make migrate

# Run tests
make test

# Build production binaries
make build

# Build Docker images
make docker

# Format code
make fmt

# Lint
make lint
```

---

## Environment Variables

```env
# Database
DATABASE_URL=postgresql://openincident:secret@localhost:5432/openincident

# Redis
REDIS_URL=redis://localhost:6379

# Slack
SLACK_BOT_TOKEN=xoxb-...
SLACK_SIGNING_SECRET=...
SLACK_APP_TOKEN=xapp-...  # For socket mode (optional)

# AI (optional)
OPENAI_API_KEY=sk-...     # User provides this

# App
APP_ENV=development       # development, production
LOG_LEVEL=info
PORT=8080
```

---

## How to Work With This Project

### Starting a New Feature

1. Read relevant sections of PRODUCT.md and ARCHITECTURE.md
2. Check DECISIONS.md for related decisions
3. Implement in small, testable chunks
4. Update documentation if behavior changes

### Code Style

- **Go**: `gofmt`, `golangci-lint`
- **TypeScript**: Prettier, ESLint, strict mode
- **Commits**: Conventional commits (feat:, fix:, docs:)
- **No premature abstraction**: Simple first, refactor when patterns emerge

### Testing

- Unit tests for services and handlers
- Integration tests for API endpoints
- E2E tests for critical flows (alert → incident → Slack)

---

## Commands to Give Claude Code

### Starting v0.1
```
Read CLAUDE.md, PRODUCT.md, ARCHITECTURE.md, and DECISIONS.md.
We're building v0.1 of OpenIncident.
Start with project setup: create directory structure, go.mod, docker-compose.yml with PostgreSQL and Redis.
```

### Continuing Work
```
Continue with v0.1. Next step: Create the Prometheus webhook handler that receives Alertmanager payloads and stores them as alerts.
```

### Adding Features
```
Implement incident auto-creation: when a critical or warning alert is received, automatically create an incident and link the alert to it.
```

### Debugging
```
The Slack channel creation is failing with error X. Debug and fix.
```

---

## Reference Documentation

For deeper details, see:
- **PRODUCT.md** — Full product vision, roadmap, GTM strategy
- **ARCHITECTURE.md** — System design, all data models, full API spec
- **DECISIONS.md** — Why we made specific technical choices

---

## What Success Looks Like

### v0.1 Success
User can:
1. Point Prometheus Alertmanager at webhook URL
2. Alert fires → incident auto-created
3. Slack channel appears with incident details
4. View incidents in web UI
5. Acknowledge/resolve from UI
6. Status updates post to Slack

### v1.0 Success
User can:
1. Receive alerts from any monitoring tool
2. Manage full incident lifecycle with timeline
3. Run on-call rotations with escalations
4. Get AI-generated summaries and post-mortems
5. Deploy on Kubernetes with HA
6. (Enterprise) SSO, audit logs, RBAC

---

*This is a living document. Update it as the project evolves.*