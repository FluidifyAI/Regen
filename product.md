# PRODUCT.md — OpenIncident Product Vision

## Executive Summary

**OpenIncident** is an open-source incident management platform that combines alert management (PagerDuty/Opsgenie territory) with incident coordination (incident.io territory) in a single, self-hosted solution.

**One-liner:** "incident.io + PagerDuty, open-source, self-hosted, BYO-AI."

---

## The Problem

### Market Gap

The 2026 incident management market has a massive misalignment:

1. **The "SaaS Tax"**: incident.io and PagerDuty charge $30–$50/user. For a 200-person engineering team, this is $100k/year for a tool used only during emergencies.

2. **The Privacy Barrier**: Regulated sectors (Fintech, Health, Gov) cannot send internal Slack transcripts or service maps to a third-party SaaS AI for summarization.

3. **Tool Fragmentation**: Teams use separate tools for alerting (PagerDuty), incident coordination (incident.io), and post-mortems (Confluence). Context is scattered.

4. **Vendor Lock-in**: Proprietary tools hold your incident history hostage. Switching costs are enormous.

### Who Feels This Pain

| Persona | Pain | Current Workaround |
|---------|------|-------------------|
| **SRE/DevOps Engineer** | Alert fatigue, context switching between tools | Manual processes, tribal knowledge |
| **Engineering Manager** | No unified view of incidents, MTTR metrics scattered | Spreadsheets, manual reporting |
| **VP Engineering/CTO** | $100k+ annual spend on incident tooling | Accept the cost or use inferior tools |
| **CISO/Compliance** | Incident data in third-party SaaS, AI privacy concerns | Block adoption of modern tools |

---

## The Solution

### What We're Building

A unified incident management platform that provides:

1. **Alert Ingestion & Routing** — Receive alerts from any monitoring tool, deduplicate, and route intelligently
2. **Incident Lifecycle Management** — Declare, coordinate, and resolve incidents with full audit trail
3. **On-Call Scheduling** — Manage rotations, escalations, and overrides
4. **Chat-Native Coordination** — Auto-create Slack/Teams channels, sync updates bidirectionally
5. **AI-Powered Assistance** — Summarization, post-mortem drafting, pattern detection (BYO-LLM)
6. **Self-Hosted & Sovereign** — Your data never leaves your infrastructure

### What Makes Us Different

| Feature | PagerDuty | incident.io | OpenIncident |
|---------|-----------|-------------|--------------|
| Alert Management | ✅ | ❌ (relies on PD) | ✅ |
| Incident Coordination | ⚠️ Basic | ✅ | ✅ |
| On-Call Scheduling | ✅ | ❌ | ✅ |
| Self-Hosted | ❌ | ❌ | ✅ |
| Open Source | ❌ | ❌ | ✅ |
| BYO-AI/LLM | ❌ | ❌ | ✅ |
| Data Sovereignty | ❌ | ❌ | ✅ |
| Pricing | Per-seat | Per-seat | Free / Flat license |

---

## Target Market

### Primary: Mid-Market & Enterprise with Compliance Requirements

- **Company Size**: 100–5000 employees
- **Engineering Team**: 50–500 engineers
- **Industries**: Fintech, Healthcare, Government, Defense, Retail
- **Common Trait**: Cannot or will not send incident data to third-party SaaS

### Secondary: Cost-Conscious Scale-ups

- **Company Size**: 50–500 employees
- **Engineering Team**: 20–100 engineers
- **Common Trait**: Hitting PagerDuty/incident.io pricing wall, strong engineering culture

### Tertiary: Open Source Enthusiasts

- **Profile**: Teams who prefer open-source infrastructure
- **Value**: Transparency, no vendor lock-in, community contribution

---

## Business Model: Open Core

Following the PostHog/Supabase/GitLab playbook:

### Free Tier (AGPLv3)

Everything needed to run world-class incident management:

- Unlimited users
- Unlimited incidents
- All alert integrations
- On-call scheduling
- Slack/Teams integration
- AI summarization (bring your own API key)
- **SSO/SAML** (Okta, Azure AD, Google Workspace) — free, always
- Docker & Kubernetes deployment
- Community support

> **Why SSO is free:** Gating SSO punishes good security practice and gets you listed on [sso.tax](https://sso.tax).
> Teams that need SCIM + SOC2 audit logs for compliance will pay for Enterprise regardless.
> Free SSO lowers the evaluation barrier and grows the top of funnel.

### Enterprise Tier ($15k–$50k/year flat license)

Governance, compliance, and operations features:

- **SCIM Provisioning**: Automated user lifecycle (joiners/leavers synced from Okta/Azure AD)
- **Audit Log Export**: Immutable, exportable, SOC2/ISO27001-ready
- **RBAC**: Fine-grained role-based access control (viewer, responder, admin)
- **Data Retention Policies**: Configurable retention windows per data type
- **Priority Support**: SLA-backed response times (1h critical, 4h high)
- **Custom Integrations**: Professional services for bespoke alert sources

### Managed Private Cloud (Custom Pricing)

For enterprises who want us to operate it:

- We deploy and manage in your AWS/GCP/Azure account
- Your data never leaves your cloud
- Usage-based + service fee

---

## Competitive Positioning

### vs. incident.io

incident.io is excellent but:
- SaaS-only (data leaves your control)
- Per-seat pricing (expensive at scale)
- No alert management (requires PagerDuty)
- AI features use their infrastructure (privacy concerns)

**Our pitch**: "incident.io's UX, but self-hosted with full data sovereignty."

### vs. PagerDuty

PagerDuty is the incumbent but:
- Expensive and complex pricing
- Incident coordination is bolted-on, not native
- Legacy architecture, slow innovation
- No modern AI capabilities

**Our pitch**: "Modern incident coordination with PagerDuty's alerting, at a fraction of the cost."

### vs. Grafana OnCall (archived)

Grafana OnCall OSS was archived in March 2026, forcing users to Grafana Cloud.

**Our pitch**: "The spiritual successor to Grafana OnCall, but with full incident lifecycle."

### vs. Building In-House

Many teams consider building their own:
- 6–12 months of engineering time
- Ongoing maintenance burden
- Never gets the polish of a dedicated product

**Our pitch**: "Deploy in an afternoon, customize forever."

---

## Product Principles

### 1. Open Source Core Must Be Complete

The OSS version must be a "fully functional Ferrari" — not a crippled demo. Paid features are "insurance and valet service" (governance, compliance, support), not core functionality.

### 2. Self-Hosted First

Every architectural decision assumes self-hosted deployment. SaaS convenience features cannot compromise self-hosted capability.

### 3. Integrate, Don't Replace

We integrate with existing tools (Prometheus, Grafana, Datadog, Jira). We don't ask teams to rip and replace their observability stack.

### 4. AI is a Feature, Not the Product

AI capabilities (summarization, post-mortems) enhance the product but aren't the core value. The product must work perfectly without AI.

### 5. Audit Trail is Sacred

Every action is logged. Timestamps are server-generated and immutable. This isn't a feature — it's foundational architecture.

---

## Feature Roadmap

### Phase 1: The Foundation (v0.1–v0.3)

**Timeline**: Weeks 1–8

**v0.1 — Alert to Slack (Weeks 1–3)** ✅ shipped
- Prometheus Alertmanager webhook ingestion
- Incident auto-creation from alerts
- Slack channel auto-creation
- Basic web UI for incident list
- Docker Compose deployment

**v0.2 — Incident Lifecycle (Weeks 4–5)** ✅ shipped
- Incident status workflow (triggered → acknowledged → resolved)
- Incident timeline with all events
- Slack bidirectional sync (messages appear in timeline)
- Manual incident declaration from Slack
- Incident severity levels

**v0.3 — Multi-Source Alerts (Weeks 6–8)** ✅ shipped
- Grafana Alertmanager integration
- CloudWatch integration
- Generic webhook endpoint (catch-all)
- Alert grouping and deduplication
- Alert routing rules

### Phase 2: On-Call & Scheduling (v0.4–v0.5)

**Timeline**: Weeks 9–14

**v0.4 — On-Call Rotations (Weeks 9–11)** ✅ shipped
- Schedule creation (daily, weekly, custom)
- Rotation management
- Override scheduling
- On-call calendar view
- Slack notifications for shift changes

**v0.5 — Escalation Policies (Weeks 12–14)** ⚠️ partial
- ✅ Multi-tier escalation chains
- ✅ Escalation timeout rules
- ✅ Fallback responders
- ✅ Escalation path visualization
- ❌ PagerDuty schedule import — **pending** (OI-EPIC-020; not yet implemented)

### Phase 3: AI & Intelligence (v0.6–v0.7)

**Timeline**: Weeks 15–20

**v0.6 — AI Summarization (Weeks 15–17)** ✅ shipped
- Incident summary generation (OpenAI BYO key — `internal/integrations/openai/`)
- Slack thread summarization
- Timeline digest for handoffs
- BYO API key support

**v0.7 — Post-Mortem Automation (Weeks 18–20)** ⚠️ partial
- ✅ Auto-generated post-mortem drafts (`POST /api/v1/incidents/:id/postmortem/generate`)
- ✅ Template customization (`PostMortemTemplatesPage`, full CRUD API)
- ✅ Timeline → post-mortem section mapping
- ✅ Action item extraction
- ❌ Confluence/Notion export — **pending** (post-mortems exportable as JSON only today)

### Phase 4: Enterprise & Polish (v0.8–v1.0)

**Timeline**: Weeks 21–28

**v0.8 — Teams Integration (Weeks 21–23)** ✅ shipped
- Teams channel auto-creation in parallel with Slack (async goroutines)
- Teams bot commands: `@Bot ack`, `resolve`, `new`, `status`
- Adaptive Cards on incident creation and status change
- `MultiChatService` fan-out for DMs (shift notifier, escalation worker)
- Bot Framework Proactive Messaging (resolved delegated-only Graph API limitation from original plan)

**v0.9 — Enterprise Features + Teams Hardening (Weeks 24–26)** ⚠️ partial

*OSS (free for all):*
- ✅ SSO/SAML — SAML 2.0 SP via `crewjam/saml`; Okta, Azure AD, Google Workspace; JIT provisioning; no-op when unconfigured
- ✅ Frontend auth — `AuthContext`, `AuthGate`, `LoginPage` with SSO button, user display + logout

*Enterprise (paid) — pending:*
- ❌ SCIM user provisioning
- ❌ Audit log export (SOC2-ready)
- ❌ RBAC (viewer / responder / admin roles)
- ❌ Data retention policies

*Teams Integration — known limitations (documented):*
- ❌ Proper channel archive on resolve — Graph API cannot archive standard channels; current behaviour renames to `[RESOLVED]`. Private channel model required.
- ❌ Auto-invite specific users — no-op for standard channels; Graph API `TeamMember` adds to Team, not channel. Private channel model required.

**v1.0 — Production Ready (Weeks 27–28)** ⚠️ partial
- ✅ Kubernetes Helm chart (`deploy/helm/openincident/` — Deployment, Service, Ingress, HPA, migration Job)
- ✅ High availability documentation (`docs/OPERATIONS.md`)
- ✅ Security hardening — CORS allowlist, HSTS, rate limiting, webhook signing, `docs/SECURITY.md`
- ✅ Performance optimization — N+1 fix (escalation engine), 8 new DB indexes (migration 000023), bounded queries
- ❌ Public launch — **pending**: README final polish, Docker Hub image push, announcement

### Future: Post-v1.0

**Service Catalog**
- Auto-discovery from Kubernetes
- Service ownership mapping
- Service → alert → incident linking

**Status Pages**
- Public status page generation
- Subscriber notifications
- Incident → status page automation

**Analytics & Reporting**
- MTTR/MTTA dashboards
- Incident trends
- On-call burden metrics
- SLA tracking

**Advanced AI**
- Local LLM support (Ollama)
- Runbook suggestions
- Similar incident detection
- Root cause hypothesis

---

## Success Metrics

### Product Metrics

| Metric | Target (v1.0) |
|--------|---------------|
| Time to deploy | < 30 minutes |
| Incidents handled without issues | > 99% |
| Slack channel creation latency | < 3 seconds |
| Alert → Incident latency | < 1 second |
| UI page load time | < 500ms |

### Business Metrics (Post-Launch)

| Metric | 6-Month Target |
|--------|----------------|
| GitHub stars | 1,000 |
| Self-hosted deployments | 100 |
| Enterprise inquiries | 20 |
| Enterprise customers | 3–5 |
| ARR | $50k–$150k |

---

## Go-to-Market Strategy

### Phase 1: Build in Public

- GitHub repo with "Rebel Manifesto" positioning
- Weekly dev logs on Twitter/LinkedIn
- Ship one visible feature every 2 weeks

### Phase 2: Community Building

- Reward first 100 contributors with unique merch
- Discord/Slack community for users
- Office hours for implementation help

### Phase 3: Content & SEO

- "Migrating from PagerDuty" guide
- "Self-hosted incident.io alternative" landing page
- Comparison pages for each competitor

### Phase 4: Enterprise Outreach

- Direct outreach to compliance-heavy companies
- Case studies from early adopters
- Security whitepaper and SOC2 readiness guide

---

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| incident.io releases self-hosted version | Low | High | Move fast, build community moat |
| Slack API changes break integration | Medium | High | Abstract Slack service, monitor API changes |
| Scope creep delays v1.0 | High | Medium | Strict phase gates, cut features not dates |
| No enterprise customers in 6 months | Medium | High | Validate pricing with LOIs before v1.0 |
| Competitor open-sources similar product | Low | Medium | Focus on community, UX, and velocity |

---

## Technical Requirements Summary

- **Backend**: Go (performance, single binary)
- **Database**: PostgreSQL (reliability, audit trails)
- **Cache/Queue**: Redis (async operations)
- **Frontend**: React + TypeScript (maintainability)
- **Deployment**: Docker Compose + Kubernetes
- **AI**: OpenAI API (with BYO key), future local LLM support

---

## Appendix: Naming Alternatives

If "OpenIncident" has trademark issues:

- **IncidentOps**
- **Respond** / **ResponderHQ**
- **AlertStation**
- **OnCallHub**
- **IncidentForge**
- **PageFree** (cheeky PagerDuty reference)

---

## Known Limitations & Integration Notes

| Area | Limitation | Planned Fix |
|------|-----------|-------------|
| Teams message posting | `ChannelMessage.Send` is delegated-only in Microsoft Graph API | ✅ **Resolved (v0.9)** — Bot Framework Proactive Messaging; no Graph permission required |
| Teams channel archive | Graph API cannot archive standard channels | ⚠️ **Open** — renames to `[RESOLVED] inc-N-...`; true archive needs private channel model |
| Teams user invites | Standard channels visible to all Team members — explicit invites are a no-op | ⚠️ **Open** — private channel model or DM fallback needed |
| Teams timeline sync | UI notes → Teams was missing parity with Slack | ✅ **Resolved (v0.9)** — `postTimelineNoteToTeams` ships with full fan-out |
| AI provider | OpenAI only | Interface abstracted; local LLM (Ollama) planned post-v1.0 |
| PagerDuty import | Not yet implemented | ⚠️ **Open** — OI-EPIC-020; API client, mapping, CLI, docs all pending |
| Post-mortem export | JSON only; no Confluence/Notion push | ⚠️ **Open** — planned post-v1.0 |

---

*This document is the source of truth for product direction. Update it as decisions are made.*