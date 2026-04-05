# Grafana OnCall → Regen: Field-by-Field Mapping

This document is the authoritative reference for how every Grafana OnCall API
object maps to the corresponding Regen data model. It is the source of truth for
the transformation layer in `backend/internal/integrations/oncall/transform.go`.

---

## Users

**OnCall endpoint:** `GET /api/v1/users`

| OnCall field | Regen field | Notes |
|---|---|---|
| `pk` | — | Used internally to resolve references in shifts/escalations; not stored |
| `email` | `users.email` | Lowercased; required — users without email are skipped |
| `name` | `users.name` | Falls back to `username` if empty |
| `username` | `users.name` | Fallback only |
| `role = "admin"` | `users.role = "admin"` | |
| `role = "user"` | `users.role = "member"` | |
| `role = "viewer"` | `users.role = "viewer"` | |
| `slack.user_id` | `users.slack_user_id` | Set if present |
| — | `users.auth_source = "local"` | All imported users use local auth |
| — | `users.active = true` | |
| — | `users.password_hash` | Random temporary password; user sets own via setup token |

**Conflicts:** If a user with the same email already exists in Regen, the user is
skipped and added to `conflicts[]`. Existing users are never overwritten.

**Not imported:**
- Notification policies (per-user paging rules) — planned for v1.1
- Teams ID — not available in OnCall API
- Phone number — OnCall does not expose this field

---

## Teams

**OnCall endpoint:** `GET /api/v1/teams`

Teams in Grafana OnCall have no direct equivalent in Regen. They are fetched
for context but not imported. The team name may be added as a tag or label in a
future release.

---

## Schedules

**OnCall endpoints:** `GET /api/v1/schedules` + `GET /api/v1/on_call_shifts`

### Schedule

| OnCall field | Regen field | Notes |
|---|---|---|
| `id` | — | Used to match shifts; not stored |
| `name` | `schedules.name` | Conflict if name already exists |
| `time_zone` | `schedules.timezone` | Falls back to `"UTC"` if empty |
| `type` | — | Not stored; Regen uses layer-based model for all types |
| `team` | — | Not stored |
| `shifts[]` | `schedule_layers[]` | Each shift becomes a layer |

### Shift → Schedule Layer

Each shift in `shifts[]` maps to a `schedule_layers` row:

| OnCall field | Regen field | Notes |
|---|---|---|
| `id` | — | Used to look up shift details |
| `name` | `schedule_layers.name` | Falls back to `"Layer"` if empty |
| `level` | `schedule_layers.order_index` | Layers sorted by level ascending |
| `rotation_start` | `schedule_layers.rotation_start` | ISO 8601 → `time.Time`; falls back to `start` |
| `duration` | `schedule_layers.shift_duration_seconds` | Seconds; defaults to 1 week (604800) if 0 |
| `frequency = "weekly"` | `schedule_layers.rotation_type = "weekly"` | |
| `frequency = "daily"` | `schedule_layers.rotation_type = "daily"` | |
| `frequency` = other | `schedule_layers.rotation_type = "weekly"` | Safe default |
| `rolling_users[][]` | `schedule_participants[]` | Flattened in rotation order |
| `users[]` | `schedule_participants[]` | Used when `rolling_users` is empty |
| `type = "override"` | — | Override shifts are skipped; handled separately |

### Participants

OnCall uses internal user IDs (`pk`) in `rolling_users` and `users`. During
import, these IDs are resolved to display names using the imported user map.
If a user ID cannot be resolved (user was skipped due to conflict or missing
email), the raw OnCall ID is used as the participant name — the admin can
correct this afterwards.

**Not imported:**
- `by_day`, `by_month`, `by_monthday` (complex recurrence rules) — mapped to
  weekly/daily with the same participants; full recurrence requires manual adjustment
- Schedule overrides — not imported in this version (future: import as `schedule_overrides`)
- iCal schedules (`type = "ical"`) — layers cannot be derived from an iCal URL;
  skipped with a warning

---

## Escalation Policies

**OnCall endpoints:** `GET /api/v1/escalation_chains` + `GET /api/v1/escalation_policies`

### Escalation Chain → Escalation Policy

| OnCall field | Regen field | Notes |
|---|---|---|
| `id` | — | Used to group steps |
| `name` | `escalation_policies.name` | Conflict if name already exists |
| `team` | — | Not stored |
| — | `escalation_policies.description = "Imported from Grafana OnCall"` | |
| — | `escalation_policies.enabled = true` | |

### Escalation Step → Escalation Tier

Steps within a chain map to tiers, sorted by `step` index:

| OnCall step type | Regen tier | Notes |
|---|---|---|
| `notify_persons` | `target_type = "users"`, `user_names = [...]` | Persons resolved to display names |
| `notify_person_next_each_time` | `target_type = "users"`, `user_names = [...]` | Same mapping; Regen rotates automatically |
| `notify_on_call_from_schedule` | `target_type = "schedule"`, `schedule_id = <uuid>` | Schedule must have been imported; skipped if not |
| `wait` | — | Skipped; wait duration is absorbed conceptually into the next tier's `timeout_seconds` |
| `resolve_incident` | — | Not mappable; skipped |
| `notify_whole_channel` | — | Not mappable; skipped |

**Tier timeout:** `timeout_seconds` defaults to 300 (5 min) unless the step has
an explicit `duration` field set.

**Not imported:**
- Steps that reference users or schedules not present in the import (conflict/skip)
- Notify channel/team steps
- `notify_if_time_from_to` steps (time-window conditions — not yet supported in Regen)

---

## Integrations (Webhook URLs)

**OnCall endpoint:** `GET /api/v1/integrations`

Grafana OnCall integrations are inbound webhook sources. They do not create new
Regen objects — instead, the import returns a mapping table showing what URL in
Regen to use as the replacement.

| OnCall `type` | Regen webhook path | Notes |
|---|---|---|
| `alertmanager` | `/api/v1/webhooks/prometheus` | Standard Prometheus Alertmanager format |
| `grafana` | `/api/v1/webhooks/grafana` | Grafana Unified Alerting format |
| `cloudwatch` | `/api/v1/webhooks/cloudwatch` | AWS CloudWatch via SNS |
| anything else | `/api/v1/webhooks/generic` | Use Regen generic webhook format |

The user must manually update their Alertmanager / Grafana contact point
configuration to point at the new Regen URLs.

**Not imported:**
- `inbound_email` integrations — Regen does not have an inbound email receiver
- `escalation_chain` linked to an integration — user must manually link the
  imported escalation policy to a routing rule in Regen

---

## Alert Groups (incident history)

**OnCall endpoint:** `GET /api/v1/alert_groups`

Not imported in this version. Resolved alert groups can optionally be imported
as read-only historical incidents in a future release.

---

## Summary: what transfers, what doesn't

| Entity | Status | Notes |
|---|---|---|
| Users | ✅ Full | Email, name, role, Slack ID |
| Teams | ❌ Not imported | No Regen equivalent; future tag/label |
| Schedules (rotation layers) | ✅ Full | Timezone, layers, participants, rotation type |
| Schedule overrides | ⚠️ Skipped | Planned for future release |
| iCal schedules | ⚠️ Skipped | Cannot derive layers from iCal URL |
| Escalation policies | ✅ Full | Chains → policies, steps → tiers |
| Complex escalation steps (wait, whole-team) | ⚠️ Skipped | Not mappable to Regen model |
| Integrations (webhook URLs) | ✅ Mapped | New Regen URLs provided; user updates source |
| Notification policies | ❌ Not imported | Per-user paging rules — Regen v1.1 |
| Alert groups / history | ❌ Not imported | Optional future feature |
| Mobile push settings | ❌ Not applicable | Neither product has mobile push |
