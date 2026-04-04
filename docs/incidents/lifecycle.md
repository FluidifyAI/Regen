# Incident Lifecycle

Every incident in Fluidify Regen moves through a defined lifecycle. Each transition is recorded on an immutable timeline — nothing is deleted, everything is auditable.

## States

```
triggered → acknowledged → resolved
```

| State | Meaning |
|-------|---------|
| **Triggered** | Incident created — either automatically from an alert or manually. A Slack channel is opened and the team is notified. |
| **Acknowledged** | Someone is actively working it. The on-call escalation timer stops. |
| **Resolved** | The incident is closed. Timeline is sealed. Post-mortem can be drafted. |

## Creating an incident

**Automatically** — when a firing alert matches a routing rule with action "Create incident". Regen sets the title from the alert name, severity from the alert severity, and links the alert.

**From Slack:**
```
/incident new Payments API returning 500s
```

**From the UI** — click **New Incident** on the Incidents page.

**Via API:**
```bash
curl -X POST https://your-domain.com/api/v1/incidents \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Payments API returning 500s",
    "severity": "critical"
  }'
```

## Acknowledging

**From Slack** (in the incident channel):
```
/incident ack
```
or click **Acknowledge** in the Adaptive Card.

**From the UI** — click **Acknowledge** on the incident detail page.

**Via API:**
```bash
curl -X PATCH https://your-domain.com/api/v1/incidents/:id \
  -d '{"status": "acknowledged"}'
```

## Resolving

**From Slack:**
```
/incident resolve
```

**From the UI** — click **Resolve** on the incident detail page.

**Via API:**
```bash
curl -X PATCH https://your-domain.com/api/v1/incidents/:id \
  -d '{"status": "resolved"}'
```

When resolved, Regen:
1. Sets `resolved_at` timestamp
2. Adds a timeline entry
3. Posts the resolution to the Slack channel
4. Renames the channel to `[RESOLVED] #inc-042-...`
5. Calculates MTTR

## The Timeline

Every action on an incident is recorded as an immutable timeline entry:

| Entry type | Created by |
|------------|-----------|
| `status_changed` | Any status transition |
| `message` | Notes added via Slack or UI |
| `alert_linked` | When an alert is associated |
| `commander_assigned` | When incident lead is set |
| `ai_summary` | When AI generates a summary |

Timeline entries cannot be edited or deleted. This is intentional — it preserves a trustworthy audit trail.

## Severity levels

| Severity | Meaning |
|----------|---------|
| **Critical** | Production down, data loss risk, major customer impact |
| **High** | Significant degradation, partial outage |
| **Medium** | Minor degradation, workaround available |
| **Low** | Cosmetic issue, no user impact |

Severity can be updated at any time during the incident lifecycle.

## Incident commander

Each incident can have one **commander** — the person leading the response. Assign from Slack with **Make me Lead** or from the UI.

The commander is displayed prominently on the incident and in the Slack channel description.

## AI assistance

When OpenAI is configured, Regen can:

- **Summarize** — compress the timeline and Slack thread into a concise summary
- **Handoff digest** — generate a status update for shift handoffs
- **Draft post-mortem** — after resolution, generate a structured post-mortem from the timeline

Trigger from the incident detail page or via API.
