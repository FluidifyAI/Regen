# DECISIONS.md — Architecture Decision Records (ADRs)

This document records significant architectural and product decisions for OpenIncident. Each decision follows the ADR format: Context, Decision, Consequences.

---

## ADR-001: Go Backend Instead of Python/Node

**Date**: 2024-01-15  
**Status**: Accepted

### Context

Need to choose a backend language for a self-hosted incident management platform. Key requirements:
- Performance under load (100+ alerts/second)
- Simple deployment (minimal dependencies)
- Long-running processes (workers, schedulers)
- Team familiarity

Options considered:
1. Python + FastAPI
2. Node.js + Express/Fastify
3. Go + Gin/Echo

### Decision

**Go with Gin framework.**

### Consequences

**Positive:**
- Single binary deployment (no runtime dependencies)
- Excellent performance and low memory footprint
- Strong concurrency model for background workers
- incident.io and similar tools use Go (ecosystem alignment)

**Negative:**
- Slower initial development vs Python
- Smaller library ecosystem for some integrations
- Steeper learning curve for contributors

**Mitigations:**
- Use well-established libraries (GORM, go-redis, slack-go)
- Comprehensive documentation for contributors

---

## ADR-002: PostgreSQL as Primary Database

**Date**: 2024-01-15  
**Status**: Accepted

### Context

Need a database that supports:
- Complex queries (timelines, schedules)
- JSONB for flexible schemas
- Strong consistency (audit trails)
- Self-hosted friendly

Options considered:
1. PostgreSQL
2. MySQL
3. SQLite (for small deployments)
4. MongoDB

### Decision

**PostgreSQL only (no SQLite fallback).**

### Consequences

**Positive:**
- JSONB support for flexible fields (labels, annotations)
- Strong transaction support for audit integrity
- Excellent performance with proper indexing
- Battle-tested in production environments

**Negative:**
- More operational overhead than SQLite
- Requires external process (can't embed)

**Rationale:**
SQLite seems simpler but creates two code paths. PostgreSQL is universally available (Docker, managed services) and any serious deployment will need it anyway.

---

## ADR-003: Immutable Timeline Entries

**Date**: 2024-01-15  
**Status**: Accepted

### Context

Incident timelines serve as audit trails. In outsourced operations, this data may be used for:
- SLA disputes
- Compliance audits
- Post-incident reviews

### Decision

**Timeline entries are immutable. No UPDATE or DELETE operations.**

Implementation:
```sql
-- No UPDATE trigger
CREATE OR REPLACE FUNCTION prevent_timeline_update()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'Timeline entries cannot be modified';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER no_timeline_update
    BEFORE UPDATE ON timeline_entries
    FOR EACH ROW EXECUTE FUNCTION prevent_timeline_update();
```

### Consequences

**Positive:**
- Tamper-proof audit trail
- Builds trust with compliance teams
- Simplifies data model (append-only)

**Negative:**
- Cannot fix mistakes (must add correction entry)
- Storage grows indefinitely (need retention policies)

**Mitigation:**
- "Correction" timeline entry type for mistakes
- Configurable retention with archival (enterprise)

---

## ADR-004: Server-Generated Timestamps

**Date**: 2024-01-15  
**Status**: Accepted

### Context

Timestamps are critical for:
- MTTR/MTTA calculations
- SLA tracking
- Incident sequencing

If clients can set timestamps, they can manipulate metrics.

### Decision

**All critical timestamps are server-generated and immutable.**

Affected fields:
- `alerts.received_at`
- `incidents.created_at`
- `incidents.triggered_at`
- `timeline_entries.timestamp`
- `audit_logs.timestamp`

### Consequences

**Positive:**
- Prevents timestamp manipulation
- Consistent time source across distributed systems
- Reliable metrics

**Negative:**
- Cannot backfill historical data with original timestamps
- Clock skew issues if servers aren't synced

**Mitigation:**
- Require NTP on all servers
- Document that historical imports use import time, not original time

---

## ADR-005: Slack-First, Chat-Agnostic Architecture

**Date**: 2024-01-15  
**Status**: Accepted

### Context

Slack is the dominant chat platform for engineering teams. However:
- Microsoft Teams is required in many enterprises
- Future platforms may emerge
- Vendor lock-in is a concern

### Decision

**Build Slack integration first, but with abstraction layer.**

```go
// Chat service interface
type ChatService interface {
    CreateChannel(ctx context.Context, name string) (*Channel, error)
    PostMessage(ctx context.Context, channelID string, msg Message) error
    UpdateMessage(ctx context.Context, channelID, msgID string, msg Message) error
    // ...
}

// Implementations
type SlackService struct { ... }
type TeamsService struct { ... }  // v0.8
```

### Consequences

**Positive:**
- Can add Teams without rewriting core logic
- Clear separation of concerns
- Testable with mock implementations

**Negative:**
- Abstraction adds complexity
- Some Slack-specific features may not map to other platforms
- Upfront design work before Teams is even built

**Mitigation:**
- Keep abstraction minimal initially
- Accept that some features may be Slack-only

---

## ADR-006: Open Core Licensing (AGPLv3 + Commercial)

**Date**: 2024-01-15  
**Status**: Accepted

### Context

Need a sustainable business model that:
- Encourages adoption (free for most users)
- Protects against cloud providers hosting it as a service
- Generates revenue from enterprises

Options considered:
1. MIT/Apache (fully permissive)
2. AGPLv3 (copyleft)
3. BSL (time-delayed open source)
4. Proprietary with source-available

### Decision

**AGPLv3 for core, proprietary for enterprise features.**

| Component | License |
|-----------|---------|
| Core platform | AGPLv3 |
| SSO/SAML module | Proprietary |
| Audit log export | Proprietary |
| RBAC engine | Proprietary |
| SCIM provisioning | Proprietary |

### Consequences

**Positive:**
- AGPL prevents cloud providers from offering as SaaS without contributing
- Core is fully open and auditable
- Clear value prop for enterprise (compliance features)
- Follows GitLab/Grafana model

**Negative:**
- AGPL may deter some enterprise contributions
- Legal complexity in maintaining dual license
- Must clearly separate OSS and proprietary code

**Mitigation:**
- Enterprise features in separate Go packages
- CLA for contributions
- Clear documentation on license boundaries

---

## ADR-007: Redis for Async Operations

**Date**: 2024-01-15  
**Status**: Accepted

### Context

Need async processing for:
- Slack API calls (rate limited)
- AI summarization (slow)
- Escalation timers
- Notification delivery

Options considered:
1. PostgreSQL-based queue (simple but limited)
2. Redis + worker pattern
3. RabbitMQ (full message broker)
4. Kafka (overkill)

### Decision

**Redis with simple job queue pattern.**

Using go-redis with custom job queue:
```go
// Job structure
type Job struct {
    ID        string
    Type      string  // "slack_message", "ai_summary", etc.
    Payload   []byte
    CreatedAt time.Time
    Attempts  int
}
```

### Consequences

**Positive:**
- Simple to operate (already need Redis for caching)
- Fast enough for our scale
- Easy to understand and debug

**Negative:**
- No guaranteed delivery (jobs can be lost on crash)
- Limited visibility into queue state
- May need to upgrade to proper queue later

**Mitigation:**
- Persist critical jobs to PostgreSQL as backup
- Implement retry logic with exponential backoff
- Monitor queue depth

---

## ADR-008: OpenAI API for AI Features (BYO Key)

**Date**: 2024-01-15  
**Status**: Accepted

### Context

AI features (summarization, post-mortems) need an LLM. Options:
1. Build/host our own model
2. Use OpenAI API
3. Support multiple providers
4. Local LLMs (Ollama)

### Decision

**OpenAI API first, with BYO API key. Local LLM support in future.**

```go
type AIProvider interface {
    Complete(ctx context.Context, prompt string) (string, error)
    CountTokens(text string) int
}

type OpenAIProvider struct {
    apiKey string  // User's key, not ours
}
```

### Consequences

**Positive:**
- No AI infrastructure to operate
- Best-in-class model quality
- User pays directly (no margin/markup)
- Maintains data sovereignty (user's key, their data policy)

**Negative:**
- Requires internet connectivity
- OpenAI dependency
- Users need to manage API keys

**Future:**
- v0.7+: Add Ollama support for air-gapped environments
- Abstract provider interface now to enable this

---

## ADR-009: Monorepo Structure

**Date**: 2024-01-15  
**Status**: Accepted

### Context

Project has multiple components:
- Go backend (API + workers)
- React frontend
- Documentation
- Deployment configs

Options:
1. Monorepo (all in one)
2. Separate repos per component

### Decision

**Monorepo.**

```
openincident/
├── backend/
├── frontend/
├── docs/
├── deploy/
└── scripts/
```

### Consequences

**Positive:**
- Atomic commits across frontend/backend
- Simpler CI/CD
- Easier for contributors to understand full system
- Single version number

**Negative:**
- Larger repo size
- All contributors see all code
- Build times may increase

**Mitigation:**
- Use build caching
- Selective CI (only build changed components)

---

## ADR-010: No GraphQL (REST Only)

**Date**: 2024-01-15  
**Status**: Accepted

### Context

API design choice between REST and GraphQL.

### Decision

**REST API only.**

### Consequences

**Positive:**
- Simpler to implement and understand
- Better caching (HTTP caching)
- Easier to document (OpenAPI)
- Lower learning curve for integrators

**Negative:**
- Multiple round trips for complex data
- Over-fetching on some endpoints
- No subscription support (need SSE/WebSocket separately)

**Rationale:**
Our API surface is relatively simple. GraphQL complexity not justified. If needed, can add GraphQL layer later without changing backend.

---

## ADR-011: Incident Numbers Are Sequential

**Date**: 2024-01-15  
**Status**: Accepted

### Context

Incidents need human-readable identifiers. Options:
1. UUID only
2. Sequential integer (INC-001)
3. Date-based (INC-2024-001)
4. Custom prefix + sequence

### Decision

**Sequential integer with INC- prefix.**

```sql
incident_number SERIAL  -- INC-001, INC-002, etc.
```

### Consequences

**Positive:**
- Easy to reference in conversation ("INC-42")
- Indicates rough volume/age
- Simple to implement

**Negative:**
- Reveals total incident count (competitive info)
- Gaps if incidents deleted
- Potential overflow (unlikely)

**Mitigation:**
- Never delete incidents (only cancel/archive)
- Integer overflow at ~2 billion is fine

---

## ADR-012: Multi-Tenant Architecture Decision (Deferred)

**Date**: 2024-01-15  
**Status**: Deferred

### Context

Should the system support multiple tenants (organizations) in a single deployment?

### Decision

**Defer. Build single-tenant first.**

### Rationale

- Primary use case is self-hosted (one org per deployment)
- Multi-tenancy adds significant complexity
- Can add later if SaaS offering materializes

### Future Considerations

If needed, prefer schema-per-tenant over row-level security for:
- Data isolation guarantees
- Per-tenant backup/restore
- Easier compliance

---

## ADR-013: Feature Flags for Enterprise Features

**Date**: 2024-01-15  
**Status**: Accepted

### Context

Need to differentiate OSS and Enterprise features in same codebase.

### Decision

**Compile-time feature flags via Go build tags.**

```go
// +build enterprise

package sso

func EnableSAML() { ... }
```

```bash
# OSS build
go build ./...

# Enterprise build
go build -tags enterprise ./...
```

### Consequences

**Positive:**
- Enterprise code not in OSS binary
- Clear separation in codebase
- No runtime license checking needed

**Negative:**
- Two build artifacts to maintain
- Testing both configurations needed

---

## Template for New Decisions

```markdown
## ADR-XXX: Title

**Date**: YYYY-MM-DD  
**Status**: Proposed | Accepted | Deprecated | Superseded

### Context

What is the issue we're addressing?

### Decision

What is the change we're proposing?

### Consequences

**Positive:**
- Benefit 1
- Benefit 2

**Negative:**
- Drawback 1
- Drawback 2

**Mitigations:**
- How we address the negatives
```

---

*Add new decisions at the end. Never delete or modify accepted decisions — supersede them with new ADRs instead.*