# Fluidify Regen v1.0.0 — GitHub Release Body

> Copy this into the GitHub Release description when cutting the v1.0.0 tag.

---

## Fluidify Regen v1.0.0

The first stable release of Fluidify Regen — open-source incident management and on-call scheduling, self-hosted, free forever.

### Why we built this

PagerDuty and incident.io charge $30–50/user/month. For a 200-person engineering team, that's $120,000/year for software that runs on someone else's infrastructure with your incident data. Grafana OnCall was the best self-hosted alternative — until Grafana archived it in March 2026, leaving ~50,000 users without a migration path.

Regen fills that gap. Full alert ingestion, on-call rotations, escalation policies, Slack/Teams integration, AI post-mortems, and 1-click migration from PagerDuty, Opsgenie, and Grafana OnCall — in a single Docker image, on your own servers.

**SSO is free.** Gating SAML behind an enterprise tier is user-hostile. Security hygiene shouldn't require a procurement process.

### What's new since v0.11.0

**On-call**
- Schedule overrides UI with drag-to-create and conflict detection
- Public holidays — import national calendars, suppress paging during holidays
- Individual leave and unavailability — auto-redistributes shifts
- Timezone UX improvements across all schedule forms and the calendar

**Alert routing**
- RE2 regex in routing rule match criteria — match on label values, annotation content, alert title, and description
- Fixes: suppress toggle, priority defaulting, optional webhook fingerprint

**Migrations**
- PagerDuty 1-click import with EU region support
- Opsgenie 1-click import with EU region support
- Grafana OnCall migration guide for the ~50,000 affected users

**Slack**
- Migrated from Socket Mode to HTTP Events API — `SLACK_APP_TOKEN` no longer needed
- Threaded timeline notes — notes post as threads, not separate messages
- Emoji reaction transitions — ✅ to acknowledge, 🔴 to resolve from Slack

**AI**
- Multi-provider connector: OpenAI, Anthropic, or self-hosted Ollama
- Post-mortem template selector at generation time
- AI cost tracking per user in the Analytics dashboard

**Incident management**
- File attachments on incidents
- Incident detail page redesign — severity tint, sticky action bar, cleaner properties panel
- Public status page for stakeholder-facing incident communication

**Security**
- Patched SAML signature-bypass in `goxmldsig`
- Fixed pgx SQL injection CVE (GO-2026-5004) — upgraded to `pgx/v5 v5.9.2`
- Go 1.25.11 toolchain — closes 7 stdlib CVEs
- 12 npm transitive vulnerability fixes

**Documentation**
- Opsgenie migration guide
- 10 production failure runbooks (Slack down, DB lost, alert flood, missed escalation, Redis unavailable, SAML, empty schedule, Helm rollback, migration failed, OOM)

### Upgrade from v0.11.0

```bash
# Docker Compose
docker pull ghcr.io/fluidifyai/regen:1.0.0
make start

# Kubernetes
helm upgrade fluidify-regen fluidify/regen --version 1.0.0 -n fluidify --reuse-values
```

Migrations run automatically. No environment variable changes required. `SLACK_APP_TOKEN` is now unused — safe to remove or leave as-is.

### No breaking changes

v1.0.0 is fully backwards-compatible with v0.11.0.

### What's next

v1.1 focuses on the MCP server — external agents (Claude, GPT, custom bots) will be able to call Regen over MCP to query incident history, triage context, on-call state, and service health profiles. Agents become first-class actors, with their own identity and audit trail in the timeline.

---

**Full changelog:** [CHANGELOG.md](https://github.com/FluidifyAI/Regen/blob/main/CHANGELOG.md)  
**Docker image:** `ghcr.io/fluidifyai/regen:1.0.0`  
**Helm chart:** `helm install regen fluidify/regen --version 1.0.0`  
**Discord:** https://discord.gg/b6PSdhzDa
