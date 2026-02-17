# PagerDuty Import Tool — Design Document

**Date:** 2026-02-17
**Epic:** OI-EPIC-020
**Release:** v0.5
**Status:** Approved

---

## Problem

Users migrating from PagerDuty need to recreate their on-call schedules and escalation policies
manually in OpenIncident. This is the single biggest friction point for adoption. The import tool
eliminates it.

---

## Scope

**Imported:**
- On-call schedules (daily + weekly rotation layers, participants)
- Escalation policies (rules → tiers, schedule and user targets)

**Not imported (by design):**
- Incidents and alerts (ephemeral — not migrated)
- PagerDuty services (no equivalent in OpenIncident v0.5)
- Historical escalation logs
- Team targets in escalation policies (no Teams model in v0.5; skip + warn)
- Custom rotation layers (complex hand-off rules OI can't model; skip + warn)

---

## Architecture

### CLI structure

The existing `openincident` binary is refactored to use **cobra** subcommands:

```
openincident serve                              # existing server (no behavior change)
openincident import pagerduty --api-key=<key>  # new import tool
```

This replaces the current single-entry `main.go` with a cobra root command. All server
startup logic moves into `commands/serve.go`.

### New packages

```
backend/
├── cmd/openincident/
│   ├── main.go                        # cobra root (thin — wires subcommands)
│   └── commands/
│       ├── serve.go                   # extracted server startup
│       └── import_pagerduty.go        # import subcommand + flags
│
└── internal/
    ├── integrations/
    │   └── pagerduty/
    │       ├── client.go              # HTTP client, auth, pagination, retry
    │       └── models.go             # PDSchedule, PDLayer, PDPolicy, PDUser
    └── importer/
        ├── validator.go              # ValidateSchedule / ValidatePolicy
        ├── schedule_importer.go      # PD → OI Schedule mapping + persist
        ├── policy_importer.go        # PD → OI EscalationPolicy mapping + persist
        └── report.go                 # ImportReport struct + JSON write
```

---

## PagerDuty API Client

**Base URL:** `https://api.pagerduty.com`

**Required headers:**
```
Authorization: Token token=<api_key>
Accept: application/vnd.pagerduty+json;version=2
```

**Endpoints used:**

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/users/me` | Validate API key |
| GET | `/users?limit=100&offset=N` | Build email → name lookup |
| GET | `/schedules?limit=100&offset=N` | List all schedules |
| GET | `/schedules/:id?include[]=users` | Full schedule with layer + user detail |
| GET | `/escalation_policies?limit=100&offset=N` | List all policies |
| GET | `/escalation_policies/:id?include[]=targets` | Full policy with rule + target detail |

**Pagination:** offset-based, max 100/page, loop while `more: true`.

**Rate limits:** 960 req/min. Retry on 429 with exponential backoff (1s, 2s, 4s, max 3 attempts).

---

## Mapping Rules

### Schedules

| PagerDuty field | OpenIncident field | Notes |
|-----------------|-------------------|-------|
| `schedule.name` | `schedule.name` | |
| `schedule.time_zone` | `schedule.timezone` | |
| `schedule.description` | `schedule.description` | |
| `layer.rotation_type` | `layer.rotation_type` | `daily`/`weekly` only; `custom` → skip + warn |
| `layer.rotation_turn_length_seconds` | `layer.shift_duration` | |
| `layer.start` | `layer.rotation_start` | |
| `user.email` (fallback: `user.name`) | `participant.user_name` | |

**Custom rotations:** `rotation_type: "custom"` uses `handoff_day` + `handoff_time` rules that
OpenIncident cannot model as a uniform repeating interval. These layers are skipped with a
user-friendly warning explaining manual recreation steps.

### Escalation Policies

| PagerDuty field | OpenIncident field | Notes |
|-----------------|-------------------|-------|
| `policy.name` | `policy.name` | |
| `policy.description` | `policy.description` | |
| `rule` (ordered array) | `tier` (tier_index = rule position) | |
| `rule.escalation_delay_in_minutes × 60` | `tier.timeout_seconds` | |
| target: `schedule_reference` | `target_type=schedule`, look up by name | Must match an imported schedule |
| target: `user_reference` | `target_type=users`, resolve via email | |
| target: `team_reference` | Skip + warn | No Teams model in v0.5 |
| multiple targets in one rule | `target_type=both` | If both schedule + user targets present |

---

## Import Flow

```
1. Validate API key (GET /users/me)
2. Fetch all users → build email:name map
3. Fetch all schedules:
   a. Validate each (check for unsupported rotations, conflicts)
   b. Import in single DB transaction (rollback all on failure)
4. Fetch all escalation policies:
   a. Validate each (resolve schedule references, check targets)
   b. Import in single DB transaction
5. Write pagerduty_import_report.json
6. Print summary to stdout
7. Exit 0 (success) | 1 (validation errors) | 2 (API/DB errors)
```

---

## Conflict Resolution

**Default behaviour:** if a schedule or policy with the same name already exists in
OpenIncident, skip it and log a warning.

**`--force` flag:** overwrite existing records by name.

---

## Import Report

Written to `pagerduty_import_report.json` (path overridable with `--output-report=<path>`):

```json
{
  "imported_at": "2026-02-17T10:00:00Z",
  "summary": {
    "schedules_found": 12,
    "schedules_imported": 10,
    "schedules_skipped": 2,
    "layers_imported": 18,
    "layers_skipped": 3,
    "policies_found": 8,
    "policies_imported": 7,
    "policies_skipped": 1,
    "tiers_imported": 22
  },
  "warnings": [
    "Schedule 'Platform On-Call' layer 'Weekend Layer': rotation_type=custom (handoff_day=saturday) — skipped. Create manually in OpenIncident UI.",
    "Policy 'Infra Default' tier 2: team target 'Infrastructure Team' not supported — skipped.",
    "Schedule 'Legacy Rota': name conflict — already exists. Use --force to overwrite."
  ],
  "errors": []
}
```

---

## Dry-run (deferred)

The `--dry-run` flag is accepted by the CLI but prints:

```
Dry-run mode is planned for a future release.
Run without --dry-run to perform the import.
```

Full dry-run execution (fetch + validate + report without DB writes) is a follow-up task.
This is documented in `docs/PAGERDUTY_IMPORT.md`.

---

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Import completed successfully (warnings may exist) |
| `1` | Validation errors (API key invalid, DB unreachable) |
| `2` | Partial import failure (some items failed; report written) |

---

## Testing

- **PagerDuty client:** `httptest.NewServer` mock with fixture JSON responses
- **Importers:** unit tests with in-memory mock repository
- **Validator:** table-driven tests covering all skip/warn conditions
- **CLI command:** integration test with test DB (similar to existing handler tests)

---

## Documentation

`docs/PAGERDUTY_IMPORT.md` covers:
1. Prerequisites (PagerDuty API key with read permissions)
2. Step-by-step: dry-run → review report → import → verify
3. What gets imported and what doesn't
4. Troubleshooting: invalid key, name conflicts, skipped layers
5. Manual migration steps for custom rotations and team targets

---

## Out of Scope (v0.5)

- Continuous sync with PagerDuty (one-time import only)
- Services, integrations, webhooks
- Historical incident/alert data
- Team targets (deferred to v0.6+ when Teams model exists)
- Full dry-run execution (stub only; deferred)
