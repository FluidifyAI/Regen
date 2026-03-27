# CLAUDE.md — Project Context for Claude Code

> **Read this file completely before starting any work.**
> This is the source of truth for what we're building and how.

---

## What Is This Project?

**Fluidify Regen** is an open-source incident management platform that combines:
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
| Community | AGPLv3 | Everything: alerts, incidents, scheduling, Slack/Teams, AI, **SSO/SAML** |
| Enterprise | Proprietary | SCIM provisioning, audit log export, RBAC, retention policies, SLA support |

**The OSS version must be a fully functional Ferrari. Enterprise is insurance and valet service.**

> **Why SSO is free:** Gating SSO is user-hostile — it's a security hygiene requirement, not a power feature.
> Teams that need SCIM + SOC2-ready audit logs will pay regardless. SSO free = more enterprise evaluators
> self-hosting = more eventual paying customers. We stay off [sso.tax](https://sso.tax).

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
fluidify-regen/
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
│   ├── cmd/regen/
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
- [x] Project setup (Go, PostgreSQL, Redis, Docker Compose)
- [x] Prometheus webhook endpoint
- [x] Alert model and storage
- [x] Incident auto-creation from alerts
- [x] Slack channel auto-creation
- [x] Basic incident list UI

**v0.2 — Incident Lifecycle (Weeks 4–5)**
- [x] Status workflow (triggered → acknowledged → resolved)
- [x] Timeline entries (immutable)
- [x] Slack bidirectional sync
- [x] Manual incident creation from Slack (/incident new)
- [x] Incident detail page

**v0.3 — Multi-Source Alerts (Weeks 6–8)**
- [x] Grafana webhook
- [x] CloudWatch webhook
- [x] Generic webhook
- [x] Alert deduplication
- [x] Alert grouping rules
- [x] Routing rules

### Phase 2: On-Call (v0.4–v0.5)

**v0.4 — Rotations (Weeks 9–11)**
- [x] Schedule model
- [x] Layer-based rotations
- [x] Override scheduling
- [x] Calendar UI
- [x] Who's on call API
- [x] Slack shift notifications

**v0.5 — Escalations (Weeks 12–14)**
- [x] Escalation policy model
- [x] Multi-step escalation
- [x] Timeout triggers
- [x] Escalation UI
- [ ] PagerDuty import — **PENDING** (OI-EPIC-020): API client, schedule/policy mapping, CLI command, validation, docs — not yet implemented

### Phase 3: AI (v0.6–v0.7)

**v0.6 — Summarization (Weeks 15–17)**
- [x] OpenAI integration (BYO key) — `internal/integrations/openai/`
- [x] Incident summary generation — `POST /api/v1/incidents/:id/summarize`
- [x] Slack thread summarization
- [x] Handoff digest
- [x] Summary in UI

**v0.7 — Post-Mortems (Weeks 18–20)**
- [x] Post-mortem model — migrations 000017–000019
- [x] Auto-generated drafts — `POST /api/v1/incidents/:id/postmortem/generate`
- [x] Template system — `PostMortemTemplatesPage`, CRUD API
- [x] Action item extraction
- [ ] Export (Confluence/Notion) — **PENDING**: not implemented; post-mortems exportable as JSON only

### Phase 4: Enterprise (v0.8–v1.0)

**v0.8 — Teams Integration (Weeks 21–23)**
- [x] Teams channel auto-creation (parallel to Slack, async goroutine)
- [x] Teams bot commands (`@Bot ack`, `resolve`, `new`, `status`)
- [x] Adaptive Card posted on incident creation and status change
- [x] `MultiChatService` fan-out for DMs (shift notifier, escalation worker)
- [x] Initial card post unblocked via Bot Framework Proactive Messaging (resolved in v0.9 hardening — see below)

**v0.9 — Enterprise Features + Teams Hardening (Weeks 24–26)**

*OSS (free):*
- [x] SSO/SAML — SAML 2.0 SP, works with Okta, Azure AD, Google Workspace (JIT provisioning, backwards-compatible no-op when disabled)
- [x] Frontend auth — `AuthContext`, `useAuth` hook, `AuthGate` (proactive session check), `LoginPage` with SSO button, user display + logout in Sidebar

*Enterprise (paid):*
- [ ] SCIM provisioning
- [ ] Audit log export (SOC2-ready)
- [ ] RBAC (viewer / responder / admin roles)
- [ ] Retention policies

*Teams Integration Hardening (backlog from v0.8):*
- [x] Replace Graph API channel posting with **Bot Framework Proactive Messaging** (true Slack parity — same bot credentials, second OAuth scope `https://api.botframework.com/.default`, no `ChannelMessage.Send` permission required). Requires `TEAMS_SERVICE_URL` env var (region-specific relay URL). New methods: `PostToChannel`, `PostToConversation`, `UpdateConversationMessage`. DB: `teams_conversation_id` + `teams_activity_id` columns (migration 000022). `PostToChannel` collapses channel-create + initial post into a single Bot Framework API call (the relay requires a non-empty `activity` in the `ConversationParameters` body).
- [x] Sync UI timeline notes → Teams channel (`postTimelineNoteToTeams` mirrors `postTimelineNoteToSlack`)
- [x] Sync Teams `@Bot` replies → UI timeline (non-command messages in `TeamsEventHandler` saved as timeline entries)
- [x] `make teams-app-package` — generates ready-to-sideload `fluidify-regen-teams-app.zip` from `TEAMS_APP_ID`; pure-Python PNG generation (no Pillow), fresh GUID for Teams app `id` (must differ from `botId`). See `scripts/teams-app-package.sh`.
- [x] Security hardening pass: JWT `iss` validation (`https://api.botframework.com`), JWKS client timeout, `io.LimitReader` on API responses (4 MB cap), panic recovery on all Teams goroutines, `UpdateTeamsPostingIDs` atomic write, `GetByTeamsConversationID` for correct bot-command lookup
- [ ] Proper channel archive on resolve — **documented limitation**: Graph API cannot archive standard channels; current behaviour renames to `[RESOLVED]`. True archive requires private channel model.
- [ ] Auto-invite specific users to Teams channel — **documented limitation**: no-op for standard channels; Graph API `TeamMember` adds to Team, not channel. Private channel model required.

**v1.0 — Production Ready (Weeks 27–28)**
- [x] Kubernetes Helm chart — `deploy/helm/fluidify-regen/` (Deployment, Service, Ingress, HPA, migration Job, ConfigMap, Secret, NOTES.txt)
- [x] HA documentation — `docs/OPERATIONS.md` (K8s HA, PostgreSQL HA, Redis Sentinel, zero-downtime deploys, observability, sizing)
- [x] Security hardening — CORS allowlist (`CORS_ALLOWED_ORIGINS`), HSTS, security headers, rate limiting (3 tiers), webhook signing; `docs/SECURITY.md`
- [x] Performance tuning — N+1 fix in escalation engine, 8 new DB indexes (migration 000023), `GetAlerts()` bounded at 500, redundant pre-check removed
- [ ] Public launch — **PENDING**: README final polish, Docker Hub image push, announcement

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

### 6. Teams Integration Architecture (critical — read before touching Teams code)

Teams uses **two separate HTTP clients** with different OAuth scopes inside `TeamsService`:

| Client | OAuth scope | Used for |
|---|---|---|
| `graphClient` | `https://graph.microsoft.com/.default` | Channel CRUD (Graph API) |
| `botfwClient` | `https://api.botframework.com/.default` | Posting messages (Bot Framework relay) |

`ChannelMessage.Send` in Graph API only works with delegated (user) auth — app-only tokens get 403. Bot Framework Proactive Messaging uses its own scope and works with client credentials, giving full Slack parity.

**Two distinct Teams IDs are stored per incident — never confuse them:**

| Field | Format | Source | Used for |
|---|---|---|---|
| `teams_channel_id` | `19:xxx@thread.tacv2` | Graph API `CreateChannel` response | Channel management (archive, Graph reads) |
| `teams_conversation_id` | `19:xxx@thread.tacv2;messageid=NNN` | Bot Framework `POST /v3/conversations` response | Posting follow-up messages |
| `teams_activity_id` | `NNN` (numeric string) | Bot Framework `POST /v3/conversations` response | Updating the root Adaptive Card |

Bot commands (`ack`, `resolve`, `status`) receive `activity.Conversation.ID` which is the **conversation ID**, not the channel ID. Always use `GetByTeamsConversationID` in bot command handlers — `GetByTeamsChannelID` will always return not-found.

**`POST /v3/conversations` payload requirements** (learned from live testing):
- `tenantId` must appear at the **top level** of the request body (not just inside `channelData`)
- `activity` must be included and must have non-empty `text` or at least one attachment — an empty `activity` returns `BadSyntax`
- The Bot Framework relay endpoint is **region-specific**: `smba.trafficmanager.net/amer|emea|in|apac`. Default is `amer`; India tenants need `/in/`
- The relay returns both `id` (conversationID) and `activityId` in a single response when `activity` is included — use this to avoid a second round-trip

**Azure setup order matters** (each step unblocks the next):
1. App Registration → gets App ID + secret
2. Graph API permissions + admin consent → validates on startup via `getTeam()`
3. Azure Bot Service resource (separate from App Registration) → makes "BotFramework" appear under "APIs my organization uses" in the portal
4. Bot Framework API permission + admin consent → unblocks `botfwClient` token acquisition (fixes 401)
5. Bot sideloaded into the team → unblocks `POST /v3/conversations` (fixes 400 BadSyntax)

Missing step 3 is the most common setup failure — without it, the Bot Framework API permission simply doesn't exist in the tenant to grant.

### 3. Integrate, Don't Replace
- We sit alongside Prometheus, Grafana, Datadog
- Don't ask users to replace their observability stack
- Webhooks are the universal integration point

### 4. AI is Optional
- Product works 100% without AI configured
- BYO API key model (user's data, user's cost)
- Abstract provider interface for future local LLM support

### 5. Enterprise Release Strategy (Open Core)

**Problem:** If enterprise features are pushed to the public AGPLv3 repo, they become open source.

**Decision: Two-repo model**

| Repo | Visibility | License | Contents |
|------|-----------|---------|----------|
| `fluidify/regen` | Public | AGPLv3 | Everything through v0.9 OSS |
| `fluidify/regen-ee` | Private | Commercial | SCIM, audit logs, RBAC, retention |

The private EE repo imports the public OSS repo as a Go module and adds enterprise packages on top. Paid customers receive a Docker image built from the combined codebase. OSS users never see enterprise code.

**When to create the EE repo:** Before writing the first line of SCIM/audit/RBAC code.

**Why not mono-repo with license headers (GitLab model)?**
GitLab can do this because they have legal and engineering resources to enforce it. For a pre-v1.0 project, two repos is simpler, legally cleaner, and easier to explain to contributors.

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
DATABASE_URL=postgresql://regen:secret@localhost:5432/regen

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
We're building v0.1 of Fluidify Regen.
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