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
- Docker & Kubernetes deployment
- Community support

### Enterprise Tier ($15k–$50k/year flat license)

Governance and compliance features:

- **SSO/SAML**: Okta, Azure AD, Google Workspace
- **SCIM Provisioning**: Automated user management
- **Audit Logs**: Immutable, exportable, SOC2-ready
- **RBAC**: Fine-grained role-based access control
- **Data Retention Policies**: Configurable retention windows
- **Priority Support**: SLA-backed response times
- **Custom Integrations**: Professional services

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

**v0.1 — Alert to Slack (Weeks 1–3)**
- Prometheus Alertmanager webhook ingestion
- Incident auto-creation from alerts
- Slack channel auto-creation
- Basic web UI for incident list
- Docker Compose deployment

**v0.2 — Incident Lifecycle (Weeks 4–5)**
- Incident status workflow (triggered → acknowledged → resolved)
- Incident timeline with all events
- Slack bidirectional sync (messages appear in timeline)
- Manual incident declaration from Slack
- Incident severity levels

**v0.3 — Multi-Source Alerts (Weeks 6–8)**
- Grafana Alertmanager integration
- CloudWatch integration
- Generic webhook endpoint (catch-all)
- Alert grouping and deduplication
- Alert routing rules

### Phase 2: On-Call & Scheduling (v0.4–v0.5)

**Timeline**: Weeks 9–14

**v0.4 — On-Call Rotations (Weeks 9–11)**
- Schedule creation (daily, weekly, custom)
- Rotation management
- Override scheduling
- On-call calendar view
- Slack notifications for shift changes

**v0.5 — Escalation Policies (Weeks 12–14)**
- Multi-tier escalation chains
- Escalation timeout rules
- Fallback responders
- Escalation path visualization
- PagerDuty schedule import

### Phase 3: AI & Intelligence (v0.6–v0.7)

**Timeline**: Weeks 15–20

**v0.6 — AI Summarization (Weeks 15–17)**
- Incident summary generation (OpenAI integration)
- Slack thread summarization
- Timeline digest for handoffs
- Configurable summary triggers
- BYO API key support

**v0.7 — Post-Mortem Automation (Weeks 18–20)**
- Auto-generated post-mortem drafts
- Template customization
- Timeline → post-mortem section mapping
- Action item extraction
- Confluence/Notion export

### Phase 4: Enterprise & Polish (v0.8–v1.0)

**Timeline**: Weeks 21–28

**v0.8 — Teams Integration (Weeks 21–23)** ✅ shipped
- Teams channel auto-creation in parallel with Slack (async goroutines)
- Teams bot commands: `@Bot ack`, `resolve`, `new`, `status`
- Adaptive Cards on incident creation and status change
- `MultiChatService` fan-out for DMs (shift notifier, escalation worker)
- **Known limitation:** `ChannelMessage.Send` is a delegated-only Graph API permission — initial card post is blocked. Deferred to v0.9 via Incoming Webhooks.

**v0.9 — Enterprise Features + Teams Hardening (Weeks 24–26)**

*Enterprise:*
- [ ] SSO/SAML integration
- [ ] RBAC (role-based access control)
- [ ] Audit log export
- [ ] SCIM user provisioning
- [ ] Data retention policies

*Teams Integration Hardening (backlog from v0.8):*
- [ ] Replace Graph API message posting with **Incoming Webhooks** per channel (standard workaround for delegated-only `ChannelMessage.Send`; webhook URL stored at channel creation time)
- [ ] Sync UI timeline notes → Teams channel (parity with Slack's bidirectional note sync)
- [ ] Sync Teams `@Bot` replies → UI timeline (inbound parity with Slack Socket Mode)
- [ ] Proper channel archive on resolve (Graph API cannot archive standard channels; evaluate private channel model or rename convention)
- [ ] Auto-invite specific users to Teams channel (no-op for standard channels; needs private channel model or DM fallback)

**v1.0 — Production Ready (Weeks 27–28)**
- Kubernetes Helm chart
- High availability documentation
- Backup and restore procedures
- Performance optimization
- Security hardening guide
- Public launch

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
| Teams message posting | `ChannelMessage.Send` is delegated-only in Microsoft Graph API — cannot be granted to an app registration | Use Incoming Webhooks per channel (v0.9) |
| Teams channel archive | Graph API cannot archive standard channels | Rename to `[RESOLVED] inc-N-...` as best-effort today; evaluate private channel model in v0.9 |
| Teams user invites | Standard channels visible to all Team members — explicit invites are a no-op | Private channel model or DM fallback in v0.9 |
| Teams timeline sync | UI notes sync to Slack only today | Add Teams fan-out in v0.9 |
| AI provider | OpenAI only | Interface already abstracted; local LLM (Ollama) planned post-v1.0 |

---

*This document is the source of truth for product direction. Update it as decisions are made.*