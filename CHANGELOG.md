# Changelog

All notable changes to Fluidify Regen are documented here.

## [1.0.0] — 2026-07-12

First stable release. Everything you need to run production on-call and incident management, self-hosted, for free.

### Foundation

- **Open-core model** — repo split into AGPLv3 OSS (`FluidifyAI/Regen`) and proprietary Pro (`FluidifyAI/regen-pro`). OSS is a fully functional product, not a limited trial.
- **Ed25519 Pro licence key validator** — cryptographic licence system for Pro feature gating; offline validation, seat enforcement
- **Production readiness checklist** (`docs/OPERATIONS.md`) — 10-section checklist covering install, health checks, DB, security, rate limiting, backups, and monitoring
- **Pre-commit security gates** — secret scanning and SAST on every commit; no credentials can reach the repo

### On-Call & Scheduling

- **Schedule overrides UI polish** — drag-to-create overrides, conflict detection, override reason field
- **Public holidays** — import national holiday calendars into schedules; holiday shifts auto-suppress paging
- **Individual leave / unavailability** — engineers mark themselves unavailable; schedule auto-skips them and redistributes shifts
- **Timezone UX** — schedule forms show timezone hints, anchor labels, and a quick-fill for common zones; calendar renders in the viewer's local timezone

### Alert Routing

- **Regex matching in routing rules** — label and annotation values now accept RE2 regex patterns; `*` wildcard for key-exists checks
- **Annotation, title, and description matching** — routing rules can now match on `annotations`, `title`, and `description` fields in addition to labels
- **Routing rule fixes** — suppress toggle, priority defaulting, and optional webhook fingerprint

### Integrations

- **PagerDuty 1-click import** — import all schedules and escalation policies in under 60 seconds; EU region support; idempotent re-runs
- **Opsgenie 1-click import** — same experience for Opsgenie; EU region; user matching by email
- **Grafana OnCall migration** — for the ~50,000 Grafana OnCall users who need a new home after the March 2026 archive
- **Slack: HTTP Events API** — migrated from Socket Mode to HTTP Events API; `SLACK_APP_TOKEN` is no longer required and does nothing
- **Slack: threaded timeline notes** — incident timeline notes post as threads under the main incident message, not as separate top-level messages
- **Slack: emoji reaction transitions** — react with ✅ to acknowledge, 🔴 to resolve directly from Slack
- **Neuri integration** (beta) — trigger AI root cause analysis from the incident detail page; results appear in the timeline

### AI

- **Multi-provider AI connector** — connect OpenAI, Anthropic, or a self-hosted Ollama instance; switch providers without code changes
- **Post-mortem template selector** — choose a template at generation time; templates configurable in Settings
- **AI cost tracking** — track per-user AI spend; displayed in the Analytics dashboard

### Incident Management

- **Incident attachments** — upload files to incidents; stored and downloadable from the timeline
- **Incident detail redesign** — header, severity tint, sticky action bar, and restructured properties panel
- **Public status page** — share a read-only incident status page with stakeholders without giving them Regen access
- **Guard nullable API fields** — alert labels and JSONB fields that are `null` in the database no longer crash the UI

### Security

- **SAML signature-bypass fix** — patched `goxmldsig` CVE; SAML assertions with missing or invalid signatures are now correctly rejected
- **pgx SQL injection fix** — upgraded `pgx/v5` to v5.9.2 (GO-2026-5004); parameterised query bypass in `pgx < 5.9.2` is closed
- **Go 1.25.11** — toolchain upgrade; closes 7 Go stdlib CVEs in `net`, `crypto/x509`, and `html/template`
- **npm audit fixes** — 12 transitive frontend dependency vulnerabilities resolved

### UI / UX

- **Running version in UI** — current version shown in System Settings; AGPLv3 notice on login and logout pages
- **Sidebar polish** — admin nav section collapses by default; scrollbar removed
- **PostHog session replay fix** — brand logo now renders correctly in PostHog session recordings

### Documentation

- **Opsgenie migration guide** — step-by-step with API examples and troubleshooting
- **10 failure runbooks** — Slack down, DB lost, alert flood, missed escalation, Redis unavailable, SAML broken, empty schedule, Helm rollback, migration failed, OOM
- **Slack setup guide corrected** — documents HTTP Events API, not Socket Mode

### Breaking changes

None. v1.0.0 is backwards-compatible with v0.11.0.

### Upgrade from v0.11.0

1. Pull the new image: `docker pull ghcr.io/fluidifyai/regen:1.0.0`
2. Run `make start` (Docker Compose) or `helm upgrade` (Kubernetes) — migrations run automatically
3. No environment variable changes required
4. `SLACK_APP_TOKEN` is now unused — you can remove it, but leaving it set causes no harm

---

## [0.11.0] and earlier

See [GitHub Releases](https://github.com/FluidifyAI/Regen/releases) for prior release notes.
