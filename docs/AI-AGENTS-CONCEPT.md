# AI Agent Layer — Concept Document

> **Status:** Idea under consideration — not yet scheduled
> **Date:** 2026-02-27
> **Author:** Brainstormed with Claude
>
> This document captures a product concept for an embedded AI operations team
> inside OpenIncident. It is not an implementation plan. Read it, sit with it,
> decide if and when it belongs on the roadmap.

---

## The Idea in One Paragraph

Today, OpenIncident's AI is passive — a human presses a button, AI summarizes
a Slack thread. The concept here is different: a set of AI agents that live
inside OpenIncident as permanent team members, each responsible for a domain
of incident operations, capable of taking real actions through the same APIs
any human would use, and able to ask for human approval before acting on
anything consequential. The human doesn't operate the tool. The agents do.
The human supervises.

---

## What Problem This Solves

Incident response has predictable, repeatable work that humans still do manually
every time:

- An alert fires at 3am → someone has to decide if it's real
- A real incident → someone has to open it, set severity, page the right person
- Mid-incident → someone has to post status updates to Slack
- After resolution → someone has to write the post-mortem, assign action items

This work is cognitively draining, time-sensitive, and often falls on whoever
is on-call whether or not they have context. AI agents that can handle the
routine parts — and hand off to humans only when judgment is required — would
meaningfully reduce on-call burden.

---

## The Agent Roster

Five specialized agents, each owning a domain:

### 1. Alert Triage Agent
**Trigger:** New alert arrives
**Decisions:** Is this alert real or noise? Has it been seen before? What severity
does it deserve? Does it need an incident, or should it be grouped with an
existing one?
**Actions:** Set alert severity, group with existing incident, or escalate to
Incident Commander Agent

### 2. Incident Commander Agent
**Trigger:** Triage Agent escalates, or a human manually declares an incident
**Decisions:** What severity? Who should be incident commander? Who should be
paged? Which escalation policy applies?
**Actions:** Create incident, set severity, assign commander, trigger escalation,
update status as incident progresses

### 3. Comms Agent
**Trigger:** Incident status changes, timeline entries, significant developments
**Decisions:** Does this change warrant a stakeholder update? What should it say?
Who needs to know?
**Actions:** Post Slack/Teams updates, draft stakeholder messages, keep the
incident channel current without human needing to type

### 4. On-Call Agent
**Trigger:** Schedule changes, override requests, shift handoffs
**Decisions:** Is the current on-call rotation healthy? Is anyone being overloaded?
Is a handoff brief needed?
**Actions:** Suggest schedule overrides, generate handoff digests, flag burnout
risk to managers

### 5. Post-Mortem Agent
**Trigger:** Incident resolved
**Decisions:** What were the key timeline moments? What was the likely root cause?
What action items should be assigned and to whom?
**Actions:** Draft post-mortem from timeline, propose root cause, generate
action items with suggested owners

---

## How Agents Act: The Approval Model

Agents operate in one of two modes depending on stakes:

**Direct execution** (low-stakes, high-confidence):
- Grouping a duplicate alert
- Posting a routine status update to Slack
- Generating a post-mortem draft

**Approval required** (high-stakes or uncertain):
- Paging an on-call engineer at 3am
- Setting incident severity to Critical
- Assigning an incident commander
- Triggering an escalation policy

When approval is required, the agent finds the right stakeholder and reaches
out through **wherever they are** — Slack, Teams, or the app UI. It doesn't
wait for the human to log in; it goes to them. The human responds with
approve / reject / modify. Only then does the agent act.

---

## How Agents Are Architected

### Orchestrator Model

A single `AICoordinator` service subscribes to the system's event stream. When
an event arrives (alert created, incident updated, incident resolved), the
coordinator decides which agent handles it and passes the event with full
context. Agents don't talk to each other directly — the coordinator sequences
their work. This makes the system auditable: every decision has a clear chain.

```
Event → AICoordinator → routes to Agent → Agent returns Action
                                        → Action requires approval?
                                              Yes → ApprovalGateway → Human
                                              No  → Execute via API
```

### Agent Identities

Each agent is a real user account in OpenIncident with `auth_source = "ai"`.
This means:

- **Full auditability**: every action shows in the incident timeline as
  "Alert Triage Agent did X" — no special code paths, no mystery
- **Standard permissions**: agents use the same API as any user — nothing
  privileged, nothing hidden
- **Manageable**: agents appear in `/settings/users` and can be
  deactivated like any team member

### LLM Interface

Agents use function calling (tools) via the existing OpenAI integration.
Each agent gets a focused system prompt and a specific set of tools that
map to OpenIncident API calls. The LLM reasons about context and picks
tools; the tools execute real actions.

The LLM provider interface is already abstracted — agents work with
OpenAI today, local Ollama tomorrow, without changing agent code.

### Memory: Start Simple

For v1, agents have no memory across incidents. Each incident is handled
fresh using only the data in the current incident. The agent interface
is designed so that a memory module (vector search over past incidents)
can be plugged in later without changing agent code.

---

## What This Is NOT

- Not a chatbot. There is no conversational UI.
- Not autonomous without guardrails. High-stakes actions always require
  human approval.
- Not a replacement for on-call engineers. It handles the mechanical work
  so engineers can focus on the hard parts.
- Not an OpenAI dependency. BYO LLM model, same as today.

---

## Why This Could Matter for the Product

Most incident management tools bolt AI on as a feature (summarize this,
draft that). An AI agent layer as a first-class component would be a
genuine architectural differentiator:

- **Self-hosted AI ops team** — organisations get autonomous incident
  response without sending data to a third-party SaaS AI
- **On-call burden reduction** — the agents handle the 3am page triage
  so engineers sleep better
- **Consistent process** — agents follow the same workflow every time,
  no matter who is on-call
- **Onboarding** — new team members supervised by agents that already
  know the system's incident history

This aligns directly with OpenIncident's core pitch: everything incident.io
and PagerDuty do, self-hosted, BYO-AI.

---

## Open Questions Before Committing

1. **v1.0 scope** — Is this a v1.0 feature or a v1.1/v2.0 feature? The
   core product (alerts, incidents, on-call, Slack, Teams) needs to be
   rock solid first.

2. **LLM cost** — Agents making decisions on every alert will consume
   tokens. Need to think about cost controls (only invoke LLM when
   confidence from heuristics is low enough).

3. **Trust calibration** — How do users tune how much they trust the
   agents? Some teams will want agents to do everything; others will want
   agents to only suggest. Needs a per-agent autonomy setting.

4. **Failure modes** — What happens when an agent makes a wrong call?
   How is that surfaced, corrected, and learned from?

5. **Regulatory concerns** — Some compliance-heavy customers (the primary
   target market) may require human sign-off on every incident action.
   The approval model handles this but needs explicit documentation.

---

## Suggested Next Step

Do nothing yet. Let this sit. Come back to it after v1.0 ships with a
clearer sense of:

- What the most painful manual work actually is (instrument it in v1.0)
- What customers ask for most in early enterprise conversations
- Whether the LLM cost model is viable at the usage patterns you see

When the time is right, this document is the starting point for the
implementation plan.
