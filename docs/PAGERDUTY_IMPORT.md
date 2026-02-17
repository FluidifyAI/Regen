# PagerDuty Import Tool

> Import schedules and escalation policies from PagerDuty into OpenIncident in one command.

---

## Prerequisites

| Requirement | Notes |
|-------------|-------|
| OpenIncident v0.5+ | Database schema must include escalation tables (migrations 000013–000015) |
| PagerDuty API key | Read-only scope is sufficient. Scopes required: `schedules.read`, `escalation_policies.read`, `users.read` |
| `DATABASE_URL` set | Same environment variable used by `openincident serve` |

---

## Quick Start

```bash
# 1. Validate connectivity (no database writes)
openincident import pagerduty --api-key=u+YOUR_KEY --dry-run

# 2. Import (skips records that already exist by name)
openincident import pagerduty --api-key=u+YOUR_KEY

# 3. If re-running after partial import, overwrite name conflicts
openincident import pagerduty --api-key=u+YOUR_KEY --force
```

---

## What Gets Imported

### Schedules

Each PagerDuty schedule becomes an OpenIncident **Schedule** with one or more **ScheduleLayers**.

| PagerDuty | OpenIncident | Notes |
|-----------|--------------|-------|
| Schedule name | `Schedule.Name` | Must be unique; conflicts skipped (or `--force`-overwritten) |
| Schedule timezone | `Schedule.Timezone` | Defaults to `UTC` if blank |
| Layer name | `ScheduleLayer.Name` | |
| `rotation_turn_length_seconds = 86400` | `RotationType = daily` | |
| `rotation_turn_length_seconds = 604800` | `RotationType = weekly` | |
| `rotation_turn_length_seconds = 0` | **Skipped** | Custom rotations cannot be modelled as a uniform interval. Create manually. |
| Layer users | `ScheduleParticipant` rows | Resolved via email → display name map |

### Escalation Policies

Each PagerDuty escalation policy becomes an OpenIncident **EscalationPolicy** with **EscalationTiers**.

| PagerDuty | OpenIncident | Notes |
|-----------|--------------|-------|
| Policy name | `EscalationPolicy.Name` | Must be unique |
| Policy description | `EscalationPolicy.Description` | |
| Rule | `EscalationTier` | One tier per escalation rule |
| `escalation_delay_in_minutes` | `TimeoutSeconds` (×60) | |
| `schedule_reference` target | `TargetType = schedule`, `ScheduleID` resolved by name | |
| `user_reference` target | `TargetType = users`, `UserNames` array | |
| Both target types in one rule | `TargetType = both` | |
| `team_reference` target | **Skipped** — warning in report | Team targets are not supported in v0.5 |

---

## Import Report

After each run a JSON report is written (default: `oi-import-report.json`).

```json
{
  "imported_at": "2026-02-17T14:00:00Z",
  "summary": {
    "schedules_found": 5,
    "schedules_imported": 4,
    "schedules_skipped": 1,
    "layers_imported": 7,
    "layers_skipped": 2,
    "policies_found": 3,
    "policies_imported": 3,
    "policies_skipped": 0,
    "tiers_imported": 9
  },
  "warnings": [
    "Schedule \"Platform On-Call\" layer 1 (\"Custom\"): rotation_turn_length_seconds=0 (custom rotation) — skipped. Create manually in OpenIncident UI.",
    "Policy \"Infra Default\" tier 0: team target \"Backend Team\" not supported in v0.5 — skipped."
  ],
  "errors": []
}
```

Use `--report path/to/file.json` to control the output location.

---

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--api-key` | *(required)* | PagerDuty API v2 key |
| `--force` | `false` | Overwrite records whose names already exist in OpenIncident |
| `--dry-run` | `false` | Fetch from PagerDuty and print counts; skip all database writes |
| `--report` | `oi-import-report.json` | Path to write the JSON import report |

---

## Conflict Resolution

The tool compares **names** (not PagerDuty IDs) to detect conflicts.

| Situation | Default | With `--force` |
|-----------|---------|----------------|
| Schedule name already exists | Skip + warn | Import anyway |
| Policy name already exists | Skip + warn | Import anyway |

> **Note:** `--force` does not update the existing record — it creates a new one alongside it. If you need a clean re-import, delete the existing records from the UI first.

---

## Creating a PagerDuty API Key

1. Go to **PagerDuty → Integrations → API Access Keys**
2. Click **Create New API Key**
3. Description: `OpenIncident Import`
4. Type: **Read-only** (sufficient for import)
5. Copy the key — it will only be shown once

---

## Troubleshooting

### `PagerDuty API key validation failed`
The key is invalid or expired. Generate a new one in PagerDuty.

### `connecting to database: ...`
`DATABASE_URL` is not set or the database is unreachable. Run `openincident import pagerduty` with the same environment as `openincident serve`.

### Schedule layers all skipped
Your PagerDuty schedules use custom rotations (`rotation_turn_length_seconds=0`). Recreate the layers manually in **OpenIncident → On-Call**.

### Policy tiers show no schedule link
The schedule imported with a different name than what the policy tier references. The tool matches by name: ensure the schedule was imported before the policy, and that names match exactly.
