# Post-Mortems

Fluidify Regen generates post-mortem drafts automatically from the incident timeline. No more staring at a blank document after a long incident.

## Overview

A post-mortem in Regen contains:

- **Summary** — what happened and what the impact was
- **Timeline** — key events from the incident timeline
- **Root cause** — what caused the incident
- **Contributing factors** — what made it worse or harder to detect
- **Action items** — specific follow-up tasks to prevent recurrence
- **Detection** — how and when the issue was discovered
- **Resolution** — what fixed it and how long it took

## Generating a draft

After resolving an incident, go to the incident detail page and click **Generate Post-Mortem**.

Regen uses the incident timeline, linked alerts, and Slack thread (if available) to generate a structured draft using OpenAI. You review and edit before publishing.

> OpenAI must be configured. Set `OPENAI_API_KEY` in your `.env` or via **Settings → System**.

Via API:

```bash
curl -X POST https://your-domain.com/api/v1/incidents/:id/postmortem/generate
```

## Templates

Post-mortem templates let you standardize the structure across your team. Create and manage templates under **Settings → Post-Mortem Templates**.

Each template defines:
- Required sections
- Guiding questions per section
- Default action item categories

When generating, Regen uses your active template as the structure and fills it with incident data.

### Example template sections

```
## Summary
What happened? What was the impact?

## Timeline
Key events, in order.

## Root Cause
The primary technical cause.

## Contributing Factors
- What made it harder to detect?
- What made it harder to resolve?

## Action Items
- [ ] Owner: fix by date
```

## Action items

Action items extracted from a post-mortem are tracked separately and visible on the post-mortem page. Each action item has:

- Description
- Owner (assignable to any Regen user)
- Due date
- Status (open / in progress / done)

## Editing and publishing

After generating, edit the draft directly in the UI. The editor supports Markdown.

Post-mortems have two states:
- **Draft** — work in progress, not visible to the broader team
- **Published** — shared internally

## Export

Post-mortems can be exported as JSON via the API:

```bash
curl https://your-domain.com/api/v1/incidents/:id/postmortem
```

Native export to Confluence and Notion is on the roadmap.
