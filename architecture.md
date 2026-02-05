# ARCHITECTURE.md — OpenIncident System Architecture

## Overview

OpenIncident is a self-hosted incident management platform built with Go backend, React frontend, PostgreSQL for persistence, and Redis for async operations. This document covers the complete architecture from v0.1 through v1.0.

---

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              EXTERNAL SYSTEMS                                │
├─────────────────────────────────────────────────────────────────────────────┤
│  Prometheus    Grafana    CloudWatch    Datadog    Custom Webhooks          │
│       │           │           │            │              │                  │
│       └───────────┴───────────┴────────────┴──────────────┘                  │
│                                    │                                         │
│                                    ▼                                         │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                        WEBHOOK INGESTION LAYER                       │    │
│  │  /api/v1/webhooks/prometheus                                        │    │
│  │  /api/v1/webhooks/grafana                                           │    │
│  │  /api/v1/webhooks/cloudwatch                                        │    │
│  │  /api/v1/webhooks/generic                                           │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                    │                                         │
│                                    ▼                                         │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                         ALERT PROCESSOR                              │    │
│  │  • Parse & normalize alerts                                          │    │
│  │  • Deduplication                                                     │    │
│  │  • Grouping rules                                                    │    │
│  │  • Routing rules                                                     │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                    │                                         │
│                                    ▼                                         │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                       INCIDENT ENGINE                                │    │
│  │  • Incident creation                                                 │    │
│  │  • State machine (triggered → ack → resolved)                       │    │
│  │  • Timeline management                                               │    │
│  │  • Escalation triggers                                               │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│           │                    │                      │                      │
│           ▼                    ▼                      ▼                      │
│  ┌─────────────┐    ┌─────────────────┐    ┌─────────────────┐             │
│  │   SLACK     │    │   SCHEDULER     │    │   AI ENGINE     │             │
│  │   SERVICE   │    │   SERVICE       │    │   SERVICE       │             │
│  │             │    │                 │    │                 │             │
│  │ • Channels  │    │ • On-call       │    │ • Summarization │             │
│  │ • Messages  │    │ • Rotations     │    │ • Post-mortems  │             │
│  │ • Commands  │    │ • Escalations   │    │ • Patterns      │             │
│  └─────────────┘    └─────────────────┘    └─────────────────┘             │
│           │                    │                      │                      │
│           └────────────────────┴──────────────────────┘                      │
│                                    │                                         │
│                                    ▼                                         │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                         DATA LAYER                                   │    │
│  │                                                                      │    │
│  │   PostgreSQL                          Redis                          │    │
│  │   • Incidents                         • Job queues                   │    │
│  │   • Alerts                            • Pub/sub                      │    │
│  │   • Timelines                         • Rate limiting                │    │
│  │   • Schedules                         • Session cache                │    │
│  │   • Users                                                            │    │
│  │   • Audit logs                                                       │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
│                                    ▲                                         │
│                                    │                                         │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                         WEB UI (React)                               │    │
│  │  • Incident dashboard                                                │    │
│  │  • Incident detail/timeline                                          │    │
│  │  • On-call schedules                                                 │    │
│  │  • Settings & integrations                                           │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Component Details

### 1. Webhook Ingestion Layer

**Purpose**: Receive alerts from external monitoring systems and normalize them into a standard format.

**Design Principles**:
- Each source gets its own endpoint with source-specific parsing
- All endpoints produce the same internal `Alert` struct
- Idempotency via `external_id` deduplication
- Async processing via Redis queue

**Endpoints**:

| Endpoint | Source | Payload Format |
|----------|--------|----------------|
| `POST /api/v1/webhooks/prometheus` | Prometheus Alertmanager | Alertmanager JSON |
| `POST /api/v1/webhooks/grafana` | Grafana Alerting | Grafana webhook JSON |
| `POST /api/v1/webhooks/cloudwatch` | AWS CloudWatch | SNS notification |
| `POST /api/v1/webhooks/generic` | Any | Generic schema |

**Prometheus Alertmanager Payload Example**:
```json
{
  "receiver": "openincident",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "HighErrorRate",
        "severity": "critical",
        "service": "api-gateway"
      },
      "annotations": {
        "summary": "Error rate above 5%",
        "description": "Service api-gateway error rate is 7.2%"
      },
      "startsAt": "2024-01-15T10:00:00Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "fingerprint": "abc123"
    }
  ]
}
```

### 2. Alert Processor

**Purpose**: Process incoming alerts, apply rules, and determine if an incident should be created.

**Processing Pipeline**:

```
Raw Alert → Parse → Normalize → Deduplicate → Group → Route → Create/Update Incident
```

**Deduplication Logic**:
- Key: `source + external_id` (e.g., Prometheus fingerprint)
- Window: Configurable (default 5 minutes)
- Same alert within window updates existing, doesn't create new

**Grouping Logic** (v0.3+):
- Group by labels (e.g., `service`, `cluster`)
- Time window grouping (alerts within X minutes)
- Single incident for grouped alerts

**Routing Rules** (v0.3+):
```yaml
routes:
  - match:
      severity: critical
      service: payments
    escalation_policy: payments-oncall
    auto_create_incident: true
    
  - match:
      severity: warning
    auto_create_incident: false
    notify_channel: "#alerts-warning"
```

### 3. Incident Engine

**Purpose**: Core incident lifecycle management.

**State Machine**:

```
                    ┌──────────────┐
                    │   TRIGGERED  │
                    └──────┬───────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
              ▼            ▼            ▼
       ┌──────────┐  ┌──────────┐  ┌──────────┐
       │   ACK    │  │ RESOLVED │  │ CANCELED │
       └────┬─────┘  └──────────┘  └──────────┘
            │
            ▼
       ┌──────────┐
       │ RESOLVED │
       └──────────┘
```

**Status Definitions**:

| Status | Description | Allowed Transitions |
|--------|-------------|---------------------|
| `triggered` | Incident created, awaiting response | `acknowledged`, `resolved`, `canceled` |
| `acknowledged` | Responder has taken ownership | `resolved` |
| `resolved` | Incident has been fixed | (terminal) |
| `canceled` | False positive or duplicate | (terminal) |

**Timeline Events**:

Every action creates an immutable timeline entry:

```go
type TimelineEntry struct {
    ID          uuid.UUID
    IncidentID  uuid.UUID
    Timestamp   time.Time  // Server-generated, immutable
    Type        string     // "status_change", "message", "alert_linked", etc.
    ActorType   string     // "user", "system", "slack_bot"
    ActorID     string
    Content     JSONB
}
```

### 4. Slack Service

**Purpose**: Bidirectional Slack integration for incident coordination.

**Capabilities**:

| Feature | Direction | Description |
|---------|-----------|-------------|
| Channel creation | Out | Auto-create `#inc-{id}-{slug}` on incident |
| Initial message | Out | Post incident details with action buttons |
| Status updates | Out | Post when status changes |
| Message sync | In | Capture Slack messages in timeline |
| Slash commands | In | `/incident new`, `/incident ack`, `/incident resolve` |
| Button actions | In | Acknowledge, resolve, escalate from Slack |

**Slack App Scopes**:
```
channels:manage
channels:read
channels:history
chat:write
commands
users:read
reactions:read
```

**Channel Naming Convention**:
```
#inc-{incident_number}-{slug}
Example: #inc-042-api-gateway-errors
```

**Initial Message Template**:
```
🚨 *Incident #042: High Error Rate on API Gateway*

*Severity:* 🔴 Critical
*Status:* Triggered
*Created:* 2024-01-15 10:00 UTC

*Description:*
Error rate on api-gateway has exceeded 5% threshold. Current rate: 7.2%

*Linked Alerts:*
• HighErrorRate (Prometheus) - firing since 10:00

────────────────────────
[Acknowledge] [Resolve] [Escalate]
```

### 5. Scheduler Service

**Purpose**: On-call scheduling, rotations, and escalations.

**Data Model**:

```
Schedule
├── Layers (ordered)
│   ├── Layer 1: Primary on-call
│   │   └── Rotations
│   │       ├── User A: Mon-Wed
│   │       ├── User B: Wed-Fri
│   │       └── User C: Fri-Mon
│   └── Layer 2: Secondary/Backup
│       └── Rotations...
└── Overrides (take precedence)
    └── User D: Jan 15-17 (covering for User A)
```

**Escalation Policy**:

```yaml
escalation_policy:
  name: "payments-oncall"
  repeat: 3
  steps:
    - delay: 0
      targets:
        - type: schedule
          id: "payments-primary"
    - delay: 5m
      targets:
        - type: schedule
          id: "payments-secondary"
    - delay: 15m
      targets:
        - type: user
          id: "engineering-manager"
```

**Who's On-Call Query**:
```
GET /api/v1/schedules/{id}/oncall?at={timestamp}

Response:
{
  "schedule_id": "...",
  "at": "2024-01-15T10:00:00Z",
  "oncall": [
    {
      "layer": 1,
      "user": { "id": "...", "name": "Alice", "email": "alice@..." }
    },
    {
      "layer": 2,
      "user": { "id": "...", "name": "Bob", "email": "bob@..." }
    }
  ]
}
```

### 6. AI Engine

**Purpose**: AI-powered summarization and post-mortem generation.

**Design Principles**:
- AI is optional — product works fully without it
- BYO API key — no data sent to our servers
- Pluggable providers — OpenAI now, local LLMs later

**Summarization Flow**:

```
Incident Timeline → Extract Key Events → Build Prompt → LLM → Summary
```

**Summary Prompt Template**:
```
You are an incident management assistant. Summarize this incident timeline for a handoff between engineers.

Incident: {title}
Severity: {severity}
Duration: {duration}
Status: {status}

Timeline:
{timeline_entries}

Provide:
1. Brief summary (2-3 sentences)
2. Key actions taken
3. Current state
4. Recommended next steps (if unresolved)
```

**Post-Mortem Generation** (v0.7):

Template sections auto-populated from incident data:
- Summary (AI-generated)
- Timeline (from incident timeline)
- Impact (from annotations + AI analysis)
- Root cause (AI-suggested, human-edited)
- Action items (AI-extracted from Slack messages)

---

## Data Models

### Core Entities

```sql
-- Alerts from monitoring systems
CREATE TABLE alerts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id     VARCHAR(255) NOT NULL,
    source          VARCHAR(50) NOT NULL,  -- 'prometheus', 'grafana', etc.
    fingerprint     VARCHAR(255),
    status          VARCHAR(20) NOT NULL,  -- 'firing', 'resolved'
    severity        VARCHAR(20) NOT NULL,  -- 'critical', 'warning', 'info'
    title           VARCHAR(500) NOT NULL,
    description     TEXT,
    labels          JSONB DEFAULT '{}',
    annotations     JSONB DEFAULT '{}',
    raw_payload     JSONB NOT NULL,
    started_at      TIMESTAMPTZ NOT NULL,
    ended_at        TIMESTAMPTZ,
    received_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),  -- Immutable
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE(source, external_id)
);

-- Incidents
CREATE TABLE incidents (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_number     SERIAL,  -- Human-readable: INC-001
    title               VARCHAR(500) NOT NULL,
    slug                VARCHAR(100) NOT NULL,  -- URL/channel friendly
    status              VARCHAR(20) NOT NULL DEFAULT 'triggered',
    severity            VARCHAR(20) NOT NULL DEFAULT 'medium',
    summary             TEXT,
    
    -- Slack
    slack_channel_id    VARCHAR(50),
    slack_channel_name  VARCHAR(100),
    
    -- Timestamps (immutable once set)
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    triggered_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    acknowledged_at     TIMESTAMPTZ,
    resolved_at         TIMESTAMPTZ,
    
    -- Ownership
    created_by_type     VARCHAR(20) NOT NULL,  -- 'system', 'user'
    created_by_id       VARCHAR(100),
    commander_id        UUID REFERENCES users(id),
    
    -- Metadata
    labels              JSONB DEFAULT '{}',
    custom_fields       JSONB DEFAULT '{}'
);

-- Link alerts to incidents (many-to-many)
CREATE TABLE incident_alerts (
    incident_id     UUID REFERENCES incidents(id) ON DELETE CASCADE,
    alert_id        UUID REFERENCES alerts(id) ON DELETE CASCADE,
    linked_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    linked_by_type  VARCHAR(20) NOT NULL,
    linked_by_id    VARCHAR(100),
    
    PRIMARY KEY (incident_id, alert_id)
);

-- Incident timeline (immutable audit log)
CREATE TABLE timeline_entries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id     UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT NOW(),  -- Immutable
    type            VARCHAR(50) NOT NULL,
    actor_type      VARCHAR(20) NOT NULL,
    actor_id        VARCHAR(100),
    content         JSONB NOT NULL,
    
    -- No UPDATE allowed on this table
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Users
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           VARCHAR(255) NOT NULL UNIQUE,
    name            VARCHAR(255) NOT NULL,
    slack_user_id   VARCHAR(50),
    phone           VARCHAR(50),
    timezone        VARCHAR(50) DEFAULT 'UTC',
    role            VARCHAR(20) DEFAULT 'member',  -- Enterprise: 'admin', 'member', 'viewer'
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- On-call schedules
CREATE TABLE schedules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    timezone        VARCHAR(50) NOT NULL DEFAULT 'UTC',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Schedule layers (primary, secondary, etc.)
CREATE TABLE schedule_layers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    schedule_id     UUID NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
    name            VARCHAR(255) NOT NULL,
    order_index     INTEGER NOT NULL,
    rotation_type   VARCHAR(20) NOT NULL,  -- 'daily', 'weekly', 'custom'
    rotation_start  TIMESTAMPTZ NOT NULL,
    shift_duration  INTERVAL NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Layer participants (who rotates in this layer)
CREATE TABLE layer_participants (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    layer_id        UUID NOT NULL REFERENCES schedule_layers(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    order_index     INTEGER NOT NULL,
    
    UNIQUE(layer_id, user_id)
);

-- Schedule overrides (vacation coverage, etc.)
CREATE TABLE schedule_overrides (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    schedule_id     UUID NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    start_time      TIMESTAMPTZ NOT NULL,
    end_time        TIMESTAMPTZ NOT NULL,
    reason          TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Escalation policies
CREATE TABLE escalation_policies (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    repeat_count    INTEGER DEFAULT 3,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Escalation steps
CREATE TABLE escalation_steps (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    policy_id           UUID NOT NULL REFERENCES escalation_policies(id) ON DELETE CASCADE,
    step_order          INTEGER NOT NULL,
    delay_seconds       INTEGER NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Escalation step targets
CREATE TABLE escalation_targets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    step_id         UUID NOT NULL REFERENCES escalation_steps(id) ON DELETE CASCADE,
    target_type     VARCHAR(20) NOT NULL,  -- 'user', 'schedule'
    target_id       UUID NOT NULL
);

-- Audit log (Enterprise feature)
CREATE TABLE audit_logs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actor_type      VARCHAR(20) NOT NULL,
    actor_id        VARCHAR(100),
    action          VARCHAR(100) NOT NULL,
    resource_type   VARCHAR(50) NOT NULL,
    resource_id     VARCHAR(100),
    changes         JSONB,
    ip_address      INET,
    user_agent      TEXT,
    
    -- Partition by month for performance
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### Timeline Entry Types

| Type | Description | Content Schema |
|------|-------------|----------------|
| `incident_created` | Incident was created | `{ "trigger": "alert" \| "manual" }` |
| `status_changed` | Status transition | `{ "from": "triggered", "to": "acknowledged" }` |
| `severity_changed` | Severity changed | `{ "from": "high", "to": "critical" }` |
| `alert_linked` | Alert linked to incident | `{ "alert_id": "...", "alert_title": "..." }` |
| `message` | User message (from Slack/UI) | `{ "text": "...", "source": "slack" }` |
| `responder_added` | Responder joined | `{ "user_id": "...", "user_name": "..." }` |
| `escalated` | Incident escalated | `{ "to_user": "...", "reason": "..." }` |
| `summary_generated` | AI summary created | `{ "summary": "..." }` |
| `postmortem_created` | Post-mortem started | `{ "postmortem_id": "..." }` |

---

## API Specification

### Base URL
```
/api/v1
```

### Authentication

**v0.1–v0.8**: API key in header
```
Authorization: Bearer <api_key>
```

**v0.9+ (Enterprise)**: OAuth2/OIDC via SSO

### Endpoints

#### Health & Info

```
GET /health
Response: { "status": "ok" }

GET /ready
Response: { "status": "ready", "database": "ok", "redis": "ok" }

GET /info
Response: { "version": "0.1.0", "build": "abc123" }
```

#### Webhooks

```
POST /api/v1/webhooks/prometheus
Content-Type: application/json
Body: <Alertmanager payload>
Response: { "received": 3, "incidents_created": 1 }

POST /api/v1/webhooks/grafana
POST /api/v1/webhooks/cloudwatch
POST /api/v1/webhooks/generic
```

#### Incidents

```
# List incidents
GET /api/v1/incidents
Query params:
  - status: string (filter by status)
  - severity: string (filter by severity)
  - created_after: timestamp
  - created_before: timestamp
  - limit: int (default 50)
  - offset: int (default 0)
Response: {
  "incidents": [...],
  "total": 150,
  "limit": 50,
  "offset": 0
}

# Get incident
GET /api/v1/incidents/{id}
Response: {
  "id": "...",
  "incident_number": 42,
  "title": "...",
  "status": "triggered",
  "severity": "critical",
  "timeline": [...],
  "alerts": [...],
  "slack_channel": { "id": "...", "name": "..." }
}

# Create incident (manual)
POST /api/v1/incidents
Body: {
  "title": "Manual incident",
  "severity": "high",
  "description": "Something is broken"
}

# Update incident
PATCH /api/v1/incidents/{id}
Body: {
  "status": "acknowledged",
  "severity": "critical",
  "summary": "Updated summary"
}

# Add timeline entry
POST /api/v1/incidents/{id}/timeline
Body: {
  "type": "message",
  "content": { "text": "Investigating the issue" }
}

# Get incident timeline
GET /api/v1/incidents/{id}/timeline
Query params:
  - types: string[] (filter by type)
  - limit: int
Response: { "entries": [...] }
```

#### Alerts

```
# List alerts
GET /api/v1/alerts
Query params:
  - status: string
  - source: string
  - severity: string
  - limit: int
  - offset: int

# Get alert
GET /api/v1/alerts/{id}

# Link alert to incident
POST /api/v1/incidents/{incident_id}/alerts
Body: { "alert_id": "..." }
```

#### Schedules (v0.4+)

```
# List schedules
GET /api/v1/schedules

# Get schedule
GET /api/v1/schedules/{id}

# Who's on call
GET /api/v1/schedules/{id}/oncall
Query params:
  - at: timestamp (default: now)

# Create schedule
POST /api/v1/schedules
Body: {
  "name": "Primary On-Call",
  "timezone": "America/New_York",
  "layers": [...]
}

# Create override
POST /api/v1/schedules/{id}/overrides
Body: {
  "user_id": "...",
  "start_time": "...",
  "end_time": "...",
  "reason": "Vacation coverage"
}
```

#### Escalation Policies (v0.5+)

```
# List policies
GET /api/v1/escalation-policies

# Get policy
GET /api/v1/escalation-policies/{id}

# Create policy
POST /api/v1/escalation-policies
Body: {
  "name": "Engineering Escalation",
  "repeat_count": 3,
  "steps": [
    { "delay_seconds": 0, "targets": [...] },
    { "delay_seconds": 300, "targets": [...] }
  ]
}
```

#### AI (v0.6+)

```
# Generate incident summary
POST /api/v1/incidents/{id}/summarize
Response: {
  "summary": "...",
  "key_events": [...],
  "tokens_used": 1500
}

# Generate post-mortem draft
POST /api/v1/incidents/{id}/postmortem/generate
Response: {
  "postmortem": {
    "summary": "...",
    "timeline": [...],
    "impact": "...",
    "root_cause": "...",
    "action_items": [...]
  }
}
```

#### Users

```
GET /api/v1/users
GET /api/v1/users/{id}
POST /api/v1/users
PATCH /api/v1/users/{id}
DELETE /api/v1/users/{id}

GET /api/v1/users/me  # Current user
```

---

## Deployment Architecture

### Docker Compose (Development & Small Deployments)

```yaml
version: '3.8'

services:
  api:
    build: ./backend
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgresql://openincident:secret@db:5432/openincident
      - REDIS_URL=redis://redis:6379
      - SLACK_BOT_TOKEN=${SLACK_BOT_TOKEN}
      - SLACK_SIGNING_SECRET=${SLACK_SIGNING_SECRET}
    depends_on:
      - db
      - redis

  worker:
    build: ./backend
    command: ["./openincident", "worker"]
    environment:
      - DATABASE_URL=postgresql://openincident:secret@db:5432/openincident
      - REDIS_URL=redis://redis:6379
    depends_on:
      - db
      - redis

  web:
    build: ./frontend
    ports:
      - "3000:80"
    depends_on:
      - api

  db:
    image: postgres:15
    environment:
      - POSTGRES_USER=openincident
      - POSTGRES_PASSWORD=secret
      - POSTGRES_DB=openincident
    volumes:
      - postgres_data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    volumes:
      - redis_data:/data

volumes:
  postgres_data:
  redis_data:
```

### Kubernetes (Production)

```
openincident/
├── Chart.yaml
├── values.yaml
└── templates/
    ├── deployment-api.yaml
    ├── deployment-worker.yaml
    ├── deployment-web.yaml
    ├── service-api.yaml
    ├── service-web.yaml
    ├── ingress.yaml
    ├── configmap.yaml
    ├── secret.yaml
    ├── hpa-api.yaml
    └── pdb-api.yaml
```

**High Availability Setup**:
- API: 3+ replicas with HPA
- Worker: 2+ replicas
- PostgreSQL: Managed (RDS/Cloud SQL) or HA cluster
- Redis: Managed (ElastiCache) or Sentinel

---

## Security Considerations

### Authentication & Authorization

| Version | Auth Method |
|---------|-------------|
| v0.1–v0.8 | API keys (simple) |
| v0.9+ | SSO/SAML (enterprise) |

### API Key Management

```go
// API keys are hashed with bcrypt
// Only hash stored in database
// Rate limiting per API key
```

### Data Protection

- All timestamps server-generated (prevents manipulation)
- Timeline entries immutable (no UPDATE/DELETE)
- Audit log for all changes (enterprise)
- Encryption at rest (PostgreSQL TDE)
- TLS for all connections

### Slack Security

- Verify Slack signatures on all requests
- Bot token stored encrypted
- Minimal OAuth scopes
- No message content stored by default (configurable)

---

## Performance Considerations

### Expected Load

| Metric | Target |
|--------|--------|
| Alerts/second | 100 |
| Concurrent incidents | 1000 |
| Timeline entries/incident | 500 |
| API latency (p99) | < 200ms |
| Webhook processing | < 1s |

### Optimization Strategies

1. **Database Indexing**
   ```sql
   CREATE INDEX idx_incidents_status ON incidents(status);
   CREATE INDEX idx_incidents_created_at ON incidents(created_at);
   CREATE INDEX idx_alerts_source_external ON alerts(source, external_id);
   CREATE INDEX idx_timeline_incident ON timeline_entries(incident_id, timestamp);
   ```

2. **Connection Pooling**
   - PgBouncer for PostgreSQL
   - Redis connection pool

3. **Async Processing**
   - Slack API calls via Redis queue
   - AI summarization via background jobs
   - Webhook responses immediate, processing async

4. **Caching**
   - On-call calculations cached (1 minute TTL)
   - Schedule data cached
   - User data cached

---

## Monitoring & Observability

### Metrics (Prometheus)

```
# API metrics
openincident_http_requests_total{method, path, status}
openincident_http_request_duration_seconds{method, path}

# Business metrics
openincident_incidents_total{status, severity}
openincident_incident_duration_seconds{severity}
openincident_alerts_received_total{source}
openincident_slack_messages_sent_total
openincident_escalations_triggered_total

# System metrics
openincident_db_connections_active
openincident_redis_connections_active
openincident_worker_jobs_processed_total{type}
openincident_worker_jobs_failed_total{type}
```

### Logging

Structured JSON logging:
```json
{
  "timestamp": "2024-01-15T10:00:00Z",
  "level": "info",
  "message": "Incident created",
  "incident_id": "...",
  "trigger": "alert",
  "severity": "critical",
  "trace_id": "..."
}
```

### Tracing

OpenTelemetry integration for distributed tracing across:
- Webhook → Alert Processor → Incident Engine → Slack

---

## Future Architecture Considerations

### Multi-Tenancy (if needed)

Schema-per-tenant or row-level security for SaaS offering.

### Global Distribution

- Read replicas for PostgreSQL
- Redis Cluster for caching
- CDN for static assets

### Plugin System

Consider plugin architecture for:
- Custom alert sources
- Custom notification channels
- Custom AI providers

---

*This document should be updated as architectural decisions are made. Major changes require an ADR in DECISIONS.md.*