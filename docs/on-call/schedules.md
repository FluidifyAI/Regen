# On-Call Schedules

Schedules define who is on call at any given time. Regen supports multi-layer rotations, custom timezones, and schedule overrides.

## Concepts

**Schedule** — a named rotation (e.g. "Primary On-Call", "Database Team").

**Layer** — a rotation within a schedule. Layers allow you to build complex patterns:
- Layer 1: Weekly rotation (primary responder)
- Layer 2: Weekly rotation (backup, offset by half a week)

**Participant** — a user who appears in the rotation.

**Override** — a one-off swap that replaces the normal rotation for a specific time window. Used for holidays, planned absences, and coverage trades.

## Creating a schedule

1. Go to **On-Call → Schedules → New Schedule**
2. Set name and timezone
3. Add a layer:
   - Select rotation type: **Daily**, **Weekly**, or **Custom**
   - Set rotation start date/time
   - Add participants in rotation order
4. Save

The calendar view shows who is on call for the next 30 days.

## Rotation types

| Type | Description |
|------|-------------|
| Daily | Each participant is on call for 24 hours, rotating every day |
| Weekly | Each participant is on call for 7 days |
| Custom | Set a specific shift duration (e.g. 12 hours, 3 days) |

## Multi-layer schedules

Use multiple layers when you need a primary + backup structure:

```
Layer 1 (Primary)  — Alice → Bob → Charlie → ...  (weekly)
Layer 2 (Backup)   — Dave → Eve → Frank → ...     (weekly, offset 3.5 days)
```

When both layers have the same person, that person is both primary and backup for that window.

## Schedule overrides

Overrides let you swap coverage without changing the underlying rotation.

**Create an override:**
1. Open the schedule calendar
2. Click on the time window you want to override
3. Select the covering user and the exact start/end time
4. Save

Overrides are shown in a different colour on the calendar and appear on the timeline if the overriding user acknowledges an incident during that window.

## Timezone support

Each schedule has its own timezone. Participants see their shifts in their local timezone in the UI.

Regen stores all times in UTC internally. Display conversion is handled per-user.

## Who's on call API

```bash
GET /api/v1/schedules/:id/oncall
```

Response:
```json
{
  "schedule_id": "...",
  "schedule_name": "Primary On-Call",
  "current_oncall": [
    {
      "user_id": "...",
      "name": "Alice Chen",
      "email": "alice@yourcompany.com",
      "layer": "Primary",
      "since": "2024-01-15T00:00:00Z",
      "until": "2024-01-22T00:00:00Z"
    }
  ]
}
```

## Slack shift notifications

Regen posts a Slack DM to the incoming on-call user at the start of their shift:

> 🔔 **Your on-call shift has started**
> Schedule: Primary On-Call
> Your shift runs until Monday 9:00 AM (7 days)

Configure the Slack notification in **Settings → Integrations → Slack**.
