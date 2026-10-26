# Changelog

All notable changes to Fluidify Regen are documented here.

## [1.2.4] — 2026-10-26

### Added

- **Teams escalation pages are Adaptive Cards** — the page shows the alert title (coloured by severity), severity, source and tier, with a **View Alert** button. Incident-sourced escalations get a card with the tier and a link to the incidents list. Slack pages are unchanged. (#195, GitHub #188)

### Documentation

- The Teams guide pointed to `scripts/teams-app-package.sh`, which does not exist. It now describes the in-app download (Settings → Integrations → Teams → Download Teams App Package), which needs the credentials saved first. (#196, GitHub #188)

### Known limitations

- Teams escalation cards have no Acknowledge button.
- Users brought in with Import from Teams have an Entra Object ID in `teams_user_id`, which Bot Framework rejects for DMs. Set the Teams user ID (`29:...`) by hand for now.

## [1.2.3] — 2026-10-26

### Bug fixes

- **Escalation pages had no alert details** — the escalation engine was never given a way to load the alert, so pages arrived as "You're being paged:" with no title and the fallback emoji. This affected Slack and Teams pages and the push notification text. Pages now carry the alert's title, severity and source. If the alert cannot be loaded, a warning is logged and the bare page is still sent. (#193, GitHub #188)

### Known limitations

- Teams escalation pages are plain text, without an Acknowledge button or a link to the incident. Tracked on GitHub #188.

## [1.2.2] — 2026-10-24

### Bug fixes

- **Escalating a manually declared incident crashed the server** — an escalation with no alert dereferenced a nil alert while building the page, so every replica crash-looped on the 30-second worker tick. These escalations now send a page without the alert Acknowledge button. (#189, GitHub #188)
- **Teams on-call DMs were not delivered** — the card was sent inline on conversation create, which Bot Framework accepts with a 200 but does not deliver, and Regen logged success. The DM now opens the conversation and posts the card to it, and reports success only from that response. (#190, GitHub #188)
- **Teams escalation and shift DMs could not work** — they created a Graph 1:1 chat, which application permissions cannot do. They now go through Bot Framework as plain text, using the recipient's `teams_user_id`. A user with no `teams_user_id` now logs an error instead of being skipped silently. (#190, GitHub #188)
- **`teams_user_id` format** — the `29:` prefix is now added only when missing, so both `29:<id>` and the bare ID work. The field holds the Teams user ID, not the Entra Object ID; the Teams guide now says so. (#190, GitHub #188)

### Upgrade notes

- Teams DMs have not been tested against a real tenant by the maintainers. Reports are welcome on GitHub #188.
- Users brought in with Import from Teams have an Entra Object ID in `teams_user_id`, which Bot Framework rejects for DMs. Replace it with the Teams user ID (`29:...`) until this is handled.
- Teams escalation pages are plain text, without an Acknowledge button.

## [1.2.1] — 2026-10-21

### Bug fixes

- **Instance timezone ignored the container's `TZ`** — with nothing saved, Settings → System showed a hard-coded UTC. It now follows `TZ` when that is a valid timezone name (otherwise UTC). A saved value still wins. (#185, GitHub #183)
- **New schedules always defaulted to UTC** — the create-schedule form and the onboarding wizard now default to the instance timezone, then the browser's. (#185, GitHub #182)
- **A schedule starting on a given day began part-way through it** — the layer start date was saved as midnight UTC whatever timezone was chosen, so a Perth schedule starting on Monday began at 08:00 and the preview showed the previous person on day one. The date is now midnight in the schedule's timezone, and the preview is computed in that timezone. The default start date also uses today in the schedule's timezone rather than the UTC date. (#185, GitHub #184)

### Upgrade notes

- Schedules created before this release keep the start they were saved with (midnight UTC). To move one to local midnight, an admin can re-save the layer start.

### Known limitations

- The on-call calendar still draws its day columns in the viewer's timezone rather than the schedule's. Tracked in GitHub #186.

## [1.2.0] — 2026-10-18

### Added

- **Admins can edit schedule holidays** — hide or restore imported holidays, rename them, and add their own from a Holidays panel on the schedule page. Country feeds keep syncing daily but no longer undo these edits. Hidden holidays show dimmed on the calendar for admins only. New admin-only endpoints: `POST`, `PATCH` and `DELETE /api/v1/schedules/:id/holidays`. Adds migration `000045`. (#180)
- **Slack DM from a routing rule** — a rule can name an alert label that holds an email (`notify_email_label`). If it matches an active Regen user, that person gets a Slack DM, even when the rule suppresses the incident. Addresses that are not Regen users are ignored. (#175)
- **Mobile PWA** — installable web app with push notifications and five mobile screens. (#159)
- **OpenTelemetry tracing, off by default** — set `OTEL_EXPORTER_OTLP_ENDPOINT` to enable. Covers HTTP, database, Redis and outbound HTTP calls, workers and async work, and LLM completions (GenAI span conventions), with W3C `traceparent` propagation, RED metrics with trace exemplars, and trace-aware logs. Browser error tracking and real-user monitoring with trace propagation from the frontend. An optional collector with tail sampling and PII scrubbing ships in the dev compose (`--profile observability`). See `docs/OBSERVABILITY.md`. (#162–#173)

### Bug fixes

- **On-call calendar showed a layer starting a day late** — the start day was drawn as empty because the calendar picked the first slice of the day. It now shows the person on call for most of the day. (#179, GitHub #178)
- **Push handlers checked the request body before authentication** — authentication now runs first. (#158)
- **`OTEL_EXPORTER_OTLP_ENDPOINT` without a scheme** is now normalized instead of failing. (#166)

### Known limitations

- Holidays are not tied to a state or region, so a feed such as Australia's includes every state's holidays. Admins can now hide the ones that do not apply; region-aware feeds are planned (GitHub #177).

## [1.1.0] — 2026-08-08

### Mobile

- **Push notification infrastructure** — on-call engineers receive push alerts on Android and iOS (via Firebase Cloud Messaging). Set `GOOGLE_APPLICATION_CREDENTIALS` to a Firebase service account JSON to enable; omit it and nothing changes. (#155)
  - Device token registration: `POST /api/v1/push/register` and `POST /api/v1/push/unregister`
  - Triggers: incident created (on-call responder), escalation tier fired, incident resolved (commander)
  - 20-token-per-user cap with automatic stale-token cleanup (90-day inactivity sweep)
  - Foundation for OPE-235 (PWA) and OPE-236 (React Native app)

## [1.0.2] — 2026-08-01

### Bug fixes

- **Login broken on plain-HTTP self-hosted installs** — session cookie was set with `Secure=true` unconditionally, causing browsers to silently drop it over HTTP. Now detected per-request via `r.TLS` / `X-Forwarded-Proto`. (#151)
- **Timezone picker missing most of the world** — hardcoded 30-entry list replaced with a searchable combobox backed by `Intl.supportedValuesOf('timeZone')` (~600 IANA zones). Brisbane, Perth, Adelaide, Darwin and every other missing zone now appear. (#152)

## [1.0.1] — 2026-07-12

### Security

- **Go 1.25.12 toolchain** — resolves GO-2026-5856 (Encrypted Client Hello privacy leak in `crypto/tls`). Upgrade from v1.0.0: `docker pull ghcr.io/fluidifyai/regen:1.0.1` or `helm upgrade ... --version 1.0.1`.

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
