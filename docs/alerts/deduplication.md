# Alert Deduplication & Grouping

Regen prevents alert storms from creating hundreds of duplicate incidents. Two mechanisms work together: **deduplication** (same alert firing again) and **grouping** (different alerts that belong to the same incident).

## Deduplication

When an alert arrives, Regen deduplicates by the stable pair `(source, external_id)`, including alerts previously resolved:

- **Match found** — Regen updates the existing alert status, title, description and end time. A firing alert already linked to an active incident does not create another incident.
- **Firing after closure** — Regen evaluates routing and grouping again and creates or joins a new active incident. Earlier incident links and timelines are retained; the alert record represents its latest state. The original `received_at` remains unchanged.
- **No match** — Regen creates a new alert. Only firing alerts evaluate routing rules for incident creation.

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

Rules are evaluated in priority order. The first matching rule wins. Match criteria support exact matching, RE2 regex on label/annotation values, and `*` wildcards.

→ **Full reference:** [Alert Routing Rules](routing.md)

## Resolution and delivery retries

An incident created automatically from alerts resolves only when all its linked alerts are resolved. A firing linked alert keeps a triggered or acknowledged incident active. Manually created incidents are excluded from automatic resolution.

Alert status, incident links and lifecycle timeline entries commit together. PostgreSQL serializes webhook lifecycle decisions with a transaction-scoped advisory lock, including concurrent first deliveries and grouped alerts from different sources. External notifications run after commit. This deliberately trades parallel lifecycle writes for consistency; unrelated webhooks may briefly wait for the current transaction.

Replayed firing or resolved messages are idempotent within the current state. The contract has no event sequence number: a firing received after a resolved message is treated as a recurrence, including an old firing delivered out of order.

Existing inconsistent records are not changed by a migration or background repair. A subsequent resolved delivery reconciles an active automatic incident, and a firing with only closed incident links starts a new lifecycle. Historical incident links remain attached to the shared alert record, so alert details in an older incident reflect the alert's latest state rather than an immutable per-occurrence snapshot. Repairing old records without another webhook requires a separately reviewed operation.
