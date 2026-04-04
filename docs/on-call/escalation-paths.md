# Escalation Policies

Escalation policies define what happens when no one responds to an incident. Regen automatically escalates through the policy steps until someone acknowledges.

## How it works

1. An incident is created (triggered state)
2. Regen immediately notifies the first step of the escalation policy
3. If no acknowledgement within the step's timeout, Regen moves to the next step
4. This continues until someone acknowledges or all steps are exhausted

## Creating a policy

1. Go to **On-Call → Escalation Policies → New Policy**
2. Add steps in order:

| Step | Target | Timeout |
|------|--------|---------|
| 1 | On-call from "Primary On-Call" schedule | 5 minutes |
| 2 | On-call from "Management" schedule | 10 minutes |
| 3 | Specific user: Alice Chen | — (last step, no timeout) |

3. Link the policy to a routing rule under **Settings → Routing Rules**

## Step targets

Each step can notify:

| Target type | Description |
|-------------|-------------|
| Schedule | Whoever is currently on call for that schedule |
| User | A specific named user (useful for final escalation) |
| Slack channel | Post to a channel (e.g. `#incidents-p0`) |

## Timeouts

The timeout is how long Regen waits for an acknowledgement before escalating to the next step. Set per step.

- Minimum: 1 minute
- Recommended: 5–15 minutes for primary, longer for subsequent steps
- Last step: no timeout (Regen stops escalating but the incident remains open)

## Repeat policy

Optionally configure the policy to repeat from the beginning if all steps are exhausted without an acknowledgement. Use for P0 incidents where the loop should keep paging until someone responds.

## Linking to routing rules

A policy only fires when an incident matches a routing rule that references it:

1. Go to **Settings → Routing Rules**
2. Create or edit a rule
3. Set **Action** to "Create incident"
4. Set **Escalation policy** to your policy

Multiple routing rules can reference the same escalation policy.

## Example: Standard SRE escalation

```
Step 1 — Primary On-Call schedule       → timeout 5 min
Step 2 — Secondary On-Call schedule     → timeout 10 min
Step 3 — SRE Lead (specific user)       → timeout 15 min
Step 4 — VP Engineering (specific user) → no timeout
```

## Escalation timeline entries

Every escalation action is recorded on the incident timeline:

- `escalation_notified` — notification sent to a step target
- `escalation_acknowledged` — incident was acknowledged, escalation stopped
- `escalation_exhausted` — all steps completed without acknowledgement
