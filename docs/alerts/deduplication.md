# Alert Deduplication & Grouping

Regen prevents alert storms from creating hundreds of duplicate incidents. Two mechanisms work together: **deduplication** (same alert firing again) and **grouping** (different alerts that belong to the same incident).

## Deduplication

When an alert arrives, Regen checks if an active alert with the same `external_id` already exists:

- **Match found** — Regen updates the existing alert (timestamps, annotations). No new alert or incident is created.
- **No match** — Regen creates a new alert and evaluates routing rules.

The `external_id` source depends on the integration:

| Source | ExternalID used |
|--------|----------------|
| Prometheus | `fingerprint` field |
| Grafana | `fingerprint` field |
| CloudWatch | `AlarmArn` |
| Generic | `external_id` field you provide |

## Grouping Rules

Grouping combines multiple related alerts into a single incident. Configure under **Settings → Grouping Rules**.

### Example: Group all alerts from the same service

```
Match labels:  service = payments-api
Group by:      service
Window:        5 minutes
```

If `payments-api` fires three different alerts within 5 minutes, Regen creates one incident and links all three alerts to it.

### How it works

1. A new alert arrives
2. Regen checks active grouping rules in priority order
3. If a rule matches, Regen looks for an open incident that already has alerts matching the same group key
4. **Match found** — the alert is linked to the existing incident (no new incident created)
5. **No match** — evaluated against routing rules to determine whether to create a new incident

### Grouping rule fields

| Field | Description |
|-------|-------------|
| Name | Human-readable label |
| Match labels | Conditions that must match for this rule to apply (key=value pairs) |
| Group by | Label keys used to compute the group key |
| Window | Time window — alerts in the same group that arrive within this window are linked to the same incident |
| Priority | Lower number = evaluated first |

## Routing Rules

Routing rules control what happens after deduplication and grouping. Configure under **Settings → Routing Rules**.

### Actions

| Action | Description |
|--------|-------------|
| Create incident | Automatically create an incident for matching alerts |
| Notify only | Record the alert but do not create an incident |
| Suppress | Silently drop the alert (useful for maintenance windows) |

### Example rules

**Create incidents for critical and warning alerts:**

```
Match: severity IN [critical, warning]
Action: Create incident
```

**Suppress info alerts during business hours:**

```
Match: severity = info
Action: Suppress
```

Rules are evaluated in priority order. The first matching rule wins.
