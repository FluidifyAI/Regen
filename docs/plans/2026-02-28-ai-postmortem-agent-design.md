# AI Post-Mortem Agent — Design Document

> **Status:** Design validated — ready for implementation planning
> **Date:** 2026-02-28
> **Supersedes:** `docs/AI-AGENTS-CONCEPT.md` (for the Post-Mortem Agent only)

---

## Goal

Build the first AI agent in OpenIncident: a Post-Mortem Agent that automatically
drafts a post-mortem when an incident resolves, posts a notification to the
incident channel, and DMs the incident commander — using a full agent identity
model that serves as the foundation for all future agents.

**Why this agent first:** Lowest risk (drafts only, no approval required),
highest immediate value (eliminates post-incident paperwork), and the existing
OpenAI + post-mortem infrastructure is already 80% there.

**Estimated build time:** 6–8 weeks for this agent + full foundation.

---

## Architecture Overview

```
IncidentHandler.UpdateStatus("resolved")
    → publish to Redis: "events:incident.resolved" { incident_id }

AICoordinator (goroutine, started in main.go)
    → subscribe to "events:incident.*"
    → check incident.ai_enabled flag
    → route to PostMortemAgent

PostMortemAgent
    → wait 60s (timeline settle)
    → check preconditions
    → fetch full context
    → call PostMortemService.Generate() as agent user
    → write timeline entry
    → notify channel + DM commander via MultiChatService
```

---

## Section 1: Agent Identity Model

Every AI agent is a real row in the `users` table. Two new columns added:

```sql
ALTER TABLE users ADD COLUMN auth_source VARCHAR(20) NOT NULL DEFAULT 'local';
-- values: 'local', 'saml', 'ai'

ALTER TABLE users ADD COLUMN agent_type VARCHAR(50);
-- values: NULL for humans, 'postmortem', 'triage', 'comms', 'oncall', 'commander'
```

On first startup (alongside migrations), the backend seeds the Post-Mortem Agent:

```go
AgentSeed{
    Name:      "Post-Mortem Agent",
    Email:     "agent-postmortem@system.internal",
    Role:      "member",
    AuthSource: "ai",
    AgentType: "postmortem",
    Active:    true,
}
```

No password. No session. Cannot log in. Acts through the same API calls any
human user would make, with its own UUID as the actor.

**Why this model:**
- Every action appears in the incident timeline as "Post-Mortem Agent did X"
- No special code paths — agents use the same repositories, services, and API handlers
- Manageable from `/settings/users` like any team member
- Foundation is reusable: adding the next agent is one seed record + one agent file

---

## Section 2: AICoordinator & Event System

### Event Bus: Redis Pub/Sub

Uses the existing Redis instance (`REDIS_URL`). No new infrastructure.

**Publisher** — inline in `IncidentHandler`, after status is written to DB:
```go
// internal/api/handlers/incidents.go
if newStatus == "resolved" {
    payload, _ := json.Marshal(map[string]string{"incident_id": incident.ID.String()})
    redisClient.Publish(ctx, "events:incident.resolved", payload)
}
```
Non-blocking. If Redis is down, the publish fails silently — the incident is
still resolved. The agent misses this one; a human can generate manually.

**Subscriber** — `AICoordinator`, started as a goroutine in `main.go`:
```go
// internal/coordinator/coordinator.go
func (c *AICoordinator) Start(ctx context.Context) {
    sub := c.redis.Subscribe(ctx, "events:incident.*")
    for msg := range sub.Channel() {
        go c.route(ctx, msg)
    }
}
```

**Routing:**
```go
func (c *AICoordinator) route(ctx context.Context, msg redis.Message) {
    switch msg.Channel {
    case "events:incident.resolved":
        incident := c.repo.GetIncident(ctx, incidentID)
        if !incident.AIEnabled {
            log.Info("AI disabled for incident, skipping")
            return
        }
        c.postMortemAgent.Handle(ctx, incidentID)
    }
}
```

### File Structure

```
internal/
  coordinator/
    coordinator.go          ← AICoordinator, Start(), route()
    agents/
      postmortem.go         ← PostMortemAgent.Handle()
```

---

## Section 3: Post-Mortem Agent Implementation

`PostMortemAgent.Handle()` runs five steps:

### Step 1: Wait 60 seconds
Timeline entries from Slack/Teams may still be syncing. A 60-second delay
ensures the draft has complete context.
```go
time.Sleep(60 * time.Second)
```

### Step 2: Precondition checks
Skip (log reason, no error) if:
- `OPENAI_API_KEY` not configured
- Post-mortem already exists for this incident (don't overwrite human work)
- Incident duration < 5 minutes (likely a test or false alarm)
- Agent user is inactive (toggled off in Settings)

### Step 3: Fetch full context
Richer than the current "generate" button:
- Full incident record (title, severity, duration, commander)
- All timeline entries in chronological order
- Linked alerts (source, labels, severity)
- Participant list (who responded)

### Step 4: Call service layer directly
```go
pm, err := s.postMortemService.Generate(ctx, GenerateRequest{
    IncidentID: incidentID,
    ActorID:    s.agentUser.ID,   // agent's UUID, not a human's
    Context:    fullContext,
})
```
No HTTP round-trip. Direct service call. The resulting `PostMortem` row has
`created_by = <agent UUID>` — identical to a human pressing the button.

### Step 5: Write timeline entry
```go
TimelineEntry{
    IncidentID: incidentID,
    Type:       "postmortem_drafted",
    ActorType:  "ai_agent",
    ActorID:    agentUser.ID.String(),
    Content:    map[string]string{
        "postmortem_id": pm.ID.String(),
        "agent":         "postmortem",
    },
}
```
Renders in the timeline UI as: *"Post-Mortem Agent drafted a post-mortem"*
with an AI badge. No new rendering code — actor display already handles
different actor types.

---

## Section 4: Notifications

Two notifications fire after the draft is created, using the existing
`MultiChatService` fan-out.

### Channel post (Slack + Teams incident channel)

```
🤖 Post-Mortem Agent

I've drafted a post-mortem for INC-042 · "Database latency spike"

Duration: 2h 14m · Severity: High · 8 timeline events captured

Review and edit → https://your-instance/incidents/<id>/postmortem
```

Posted to `incidents.slack_channel_id` and `incidents.teams_conversation_id`
via `MultiChatService.PostMessage()`. Same code path as status update notifications.

### DM to incident commander

```
🤖 Post-Mortem Agent

As incident commander for INC-042, I've drafted a post-mortem for your review.
It includes a proposed root cause and suggested action items with owners.

Review → https://your-instance/incidents/<id>/postmortem
```

Looks up `incident.commander_id` → `users.slack_user_id` / `users.teams_user_id`.

### Graceful degradation

| Condition | Behaviour |
|---|---|
| No Slack configured | Skip Slack, post to Teams |
| No Teams configured | Skip Teams, post to Slack |
| Neither configured | Skip both — draft still created |
| Commander has no chat ID linked | Skip DM — channel post already notified |
| Both channels fail | Log warning, draft still created |

Notifications are best-effort. The draft creation is the primary action.

---

## Section 5: Agent Management UI

### Location: `/settings/users`

A new **"AI Team Members"** section below the human users table — same page,
visually separated with a section heading. No new route.

### Agent row columns

| Column | Value |
|---|---|
| Name | "Post-Mortem Agent" with `🤖 AI` badge |
| Domain | "Post-mortems" |
| Status | Active / Inactive toggle |
| Last action | "Drafted post-mortem for INC-042 · 3h ago" |
| Actions | Enable/Disable only |

### What admins cannot do (locked)
- Change role (always `member`)
- Reset password (none exists)
- Delete (seeded agents are permanent — deactivate instead)

### Active/Inactive toggle
Calls `PATCH /api/v1/agents/:id/status` → flips `users.active` boolean.
AICoordinator checks this flag before routing. Inactive = event consumed,
dropped with a log entry. No queue buildup.

### "What does this agent do?" expandable row
Inline expand (not tooltip) showing:
- Trigger: "Fires when an incident is resolved"
- Actions: "Drafts a post-mortem, notifies the incident channel and commander"
- Link to documentation

---

## Section 6: Per-Incident AI Opt-Out

### Data model

```sql
ALTER TABLE incidents ADD COLUMN ai_enabled BOOLEAN NOT NULL DEFAULT true;
```

Default `true`. Existing incidents inherit `true` via migration default.

### Four entry points to set the flag

**1. Manual incident creation form**
"AI Agents" toggle in the create modal, default On.

**2. Incident Properties panel**
Toggle alongside severity and commander. Editable at any point during or
after the incident. Sends `PATCH /api/v1/incidents/:id` with `{ "ai_enabled": false }`.
Writes a timeline entry: *"AI Agents disabled for this incident by [user]"* — auditable.

**3. Routing rule configuration**
New `ai_enabled` field on routing rules. When an alert matches a rule,
the incident is created with that rule's `ai_enabled` value.
Example: "Prometheus staging alerts → ai_enabled: false"

**4. Per-integration default**
New `ai_default_enabled` field on integration/webhook source config.
Fallback when no routing rule matches.

### Resolution order for alert-generated incidents

```
Routing rule match?  → use rule's ai_enabled
No routing rule      → use integration's ai_default_enabled
No integration cfg   → default true
```

### AICoordinator gate

```go
incident, _ := repo.GetIncident(ctx, incidentID)
if !incident.AIEnabled {
    log.Info("AI disabled for incident, skipping", "incident_id", incidentID)
    return
}
```

Checked once before routing to any agent. Agents never see disabled incidents.

---

## What This Is NOT

- Not a chatbot — no conversational UI
- Not autonomous for high-stakes actions — paging engineers still requires human approval (future agents)
- Not an OpenAI dependency — BYO key, same as today; degrades gracefully if not configured
- Not a replacement for human post-mortem writing — it drafts, humans edit

---

## Future Agents (same foundation)

Once this ships, adding the next agent is:
1. One seed record in the agent seeder
2. One new file: `internal/coordinator/agents/<name>.go`
3. One new Redis channel subscription in the coordinator

The identity model, event system, opt-out flag, and management UI are already in place.

**Next recommended agent:** Comms Agent — auto-posts Slack/Teams status updates
when incident status changes. Same infrastructure, zero new architecture.

---

## Open Questions (deferred)

1. **LLM cost controls** — invoke only when heuristic confidence is low?
   Deferred: monitor token usage in v1, add controls if needed.

2. **Per-agent autonomy settings** — some teams want agents to only suggest,
   not act. Deferred: v1 is draft-only (suggestion by nature). Relevant when
   agents take real actions (Comms Agent auto-posting).

3. **Agent memory across incidents** — vector search over past incidents for
   better root cause analysis. Deferred: v1 is stateless, interface designed
   for memory module to plug in later.

4. **Failure mode surfacing** — when agent makes a wrong call, how is it
   corrected and learned from? Deferred: v1 agents only draft, humans always
   have final edit.
