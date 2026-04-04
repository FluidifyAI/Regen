# Being On-Call

This page is for engineers who are on call — how to get paged, how to respond, and how to use Regen during an incident.

## Getting notified

When an incident is triggered and you are the on-call responder, you will receive:

1. **A Slack DM** from the Regen bot with the incident details and a link
2. **A message in the incident Slack channel** (auto-created for every incident)
3. **An Adaptive Card in Microsoft Teams** (if your team uses Teams)

The notification includes: incident title, severity, triggering alert, and a direct link to the incident.

## Responding from Slack

You will be added to the incident channel automatically. From there:

**Acknowledge the incident:**
```
/incident ack
```
or click the **Acknowledge** button in the card.

This stops the escalation timer and signals to the team that you are working it.

**Set yourself as incident commander:**

Click **Make me Lead** in the Slack channel card.

**Add a note to the timeline:**

Click **Add Note** or type in the channel — messages are synced to the incident timeline.

**Get an AI summary** (if OpenAI is configured):
```
@Fluidify Regen summary
```

**Resolve when done:**
```
/incident resolve
```

## Responding from the UI

Open the incident at `https://your-domain.com/incidents/:id`

The detail page shows:
- Current status and severity
- Linked alerts with labels
- Full timeline in chronological order
- AI-generated summary (if configured)
- Linked post-mortem (after resolution)

## Checking who else is on call

```
/incident status
```

Shows the current incident state, commander, and the on-call schedule for context.

## Handing off

Before your shift ends, use the **Handoff Digest**:

1. Open the incident in the UI
2. Click **Generate Handoff Digest**
3. Regen summarises what happened, current status, and open action items
4. Copy it to Slack or email it to the incoming responder

## Override your own shift

If you need to hand off early:

1. Go to **On-Call → Schedules**
2. Find your schedule and click on your shift
3. Click **Create Override**
4. Select the covering person and time window

The covering person is notified via Slack DM.

## Tips for effective on-call response

- **Acknowledge fast** — even if you don't have an answer yet. It stops escalation and tells the team someone is on it.
- **Add notes as you go** — don't wait until the end. Real-time timeline entries are invaluable for post-mortems and handoffs.
- **Set severity correctly** — update it if the initial auto-assigned severity is wrong. It affects visibility and escalation.
- **Link the root cause alert** — if multiple alerts fired, mark the one that caused the others.
- **Resolve cleanly** — only resolve when the issue is actually fixed, not just when symptoms disappear.
