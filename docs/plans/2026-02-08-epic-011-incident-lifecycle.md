# Epic 011: Incident Lifecycle & Slack Bidirectional Sync — Implementation Plan

## Context

This epic completes the v0.2 milestone by adding **bidirectional Slack integration** and the **create incident modal** in the frontend. The v0.1 release already delivered:

- Status workflow (triggered → acknowledged → resolved) with state machine validation
- Immutable timeline entries with server-generated timestamps
- One-way Slack: channel creation, message posting, status update messages
- Incident detail page with timeline, status dropdown, severity dropdown
- Block Kit messages with Acknowledge/Resolve buttons (buttons rendered but not yet functional)

**What's missing:** OpenIncident can *talk to* Slack but can't *listen to* Slack. This epic bridges that gap.

**User Stories:**
1. As an on-call engineer, I want to click "Acknowledge" in Slack so I don't have to switch to the web UI during an incident.
2. As a team lead, I want to run `/incident new` in Slack to declare an incident without leaving my chat.
3. As an incident commander, I want Slack channel messages to appear in the incident timeline so I have a complete audit trail.
4. As a user, I want to declare incidents from the web UI with a form rather than only through alerts.

---

## Architectural Decisions

### 1. Socket Mode over Events API

**Decision:** Use Slack Socket Mode (WebSocket) instead of the Events API (HTTP webhooks).

**Why:**
- Self-hosted apps may not have a public URL or SSL certificate
- Socket Mode uses outbound WebSocket — works behind firewalls/NAT
- No need for request signature verification (connection is authenticated)
- Simpler deployment: just set `SLACK_APP_TOKEN` (xapp-...)
- Trade-off: slightly higher latency (~100ms) vs Events API, but acceptable for incident management

**Configuration:**
```env
SLACK_APP_TOKEN=xapp-1-...   # Required for Socket Mode
SLACK_BOT_TOKEN=xoxb-...     # Existing, for API calls
```

### 2. Event Handler Architecture

**Pattern:** Central event dispatcher with typed handlers.

```go
type SlackEventHandler struct {
    incidentService services.IncidentService
    chatService     services.ChatService
}

// Dispatch routes Socket Mode events to typed handlers
func (h *SlackEventHandler) Dispatch(evt socketmode.Event) {
    switch evt.Type {
    case socketmode.EventTypeInteractive:
        h.handleInteraction(evt)
    case socketmode.EventTypeSlashCommand:
        h.handleSlashCommand(evt)
    case socketmode.EventTypeEventsAPI:
        h.handleEventsAPI(evt)
    }
}
```

**Why:** Single entry point with clear routing. Each handler is testable independently. New event types added without modifying dispatcher logic.

### 3. Echo Loop Prevention

**Problem:** When the bot posts a message (e.g., status update), Socket Mode receives that message event, which could create a timeline entry, which could trigger another Slack message...

**Solution:**
- Filter out all `bot_message` subtypes
- Filter messages where `user` matches the bot's own user ID
- Only process messages from `inc-*` channels (match by `slack_channel_id` in DB)
- Rate limit: max 1 timeline entry per second per channel

### 4. Button State Management

**Problem:** After a user clicks "Acknowledge", the button should change/disappear to prevent double-clicks and show current state.

**Solution:**
- Store the initial message `timestamp` (message_ts) in the incident record
- On button click: update incident → update original message with new blocks (buttons removed/changed)
- Use Slack's `response_url` for immediate feedback, then update the full message

---

## Implementation Steps

### Task 1: Slack Socket Mode Listener (OI-062)

**Create:** `backend/internal/services/slack_event_handler.go`
**Modify:** `backend/cmd/openincident/main.go`

**Implementation:**

```go
package services

import (
    "log/slog"
    "github.com/slack-go/slack"
    "github.com/slack-go/slack/socketmode"
)

type SlackEventHandler struct {
    client          *socketmode.Client
    incidentService IncidentService
    chatService     ChatService
    botUserID       string
}

func NewSlackEventHandler(
    appToken string,
    botToken string,
    incidentService IncidentService,
    chatService ChatService,
) (*SlackEventHandler, error) {
    api := slack.New(botToken,
        slack.OptionAppLevelToken(appToken),
    )

    client := socketmode.New(api,
        socketmode.OptionDebug(false),
    )

    // Get bot's own user ID for echo prevention
    auth, err := api.AuthTest()
    if err != nil {
        return nil, fmt.Errorf("slack auth failed: %w", err)
    }

    return &SlackEventHandler{
        client:          client,
        incidentService: incidentService,
        chatService:     chatService,
        botUserID:       auth.UserID,
    }, nil
}

func (h *SlackEventHandler) Start() {
    go h.listen()
    go h.client.Run()
}

func (h *SlackEventHandler) listen() {
    for evt := range h.client.Events {
        switch evt.Type {
        case socketmode.EventTypeConnecting:
            slog.Info("slack socket mode connecting...")
        case socketmode.EventTypeConnected:
            slog.Info("slack socket mode connected")
        case socketmode.EventTypeConnectionError:
            slog.Error("slack socket mode connection error")
        case socketmode.EventTypeInteractive:
            h.handleInteraction(evt)
        case socketmode.EventTypeSlashCommand:
            h.handleSlashCommand(evt)
        case socketmode.EventTypeEventsAPI:
            h.handleEventsAPI(evt)
        }
    }
}
```

**Wire up in main.go:**
```go
// After chatService initialization
if cfg.SlackAppToken != "" && chatService != nil {
    eventHandler, err := services.NewSlackEventHandler(
        cfg.SlackAppToken,
        cfg.SlackBotToken,
        incidentService,
        chatService,
    )
    if err != nil {
        slog.Error("failed to create slack event handler", "error", err)
    } else {
        eventHandler.Start()
        slog.Info("slack socket mode enabled - bidirectional sync active")
    }
} else {
    slog.Warn("SLACK_APP_TOKEN not set - bidirectional Slack sync disabled (one-way only)")
}
```

**Add to config:**
```go
SlackAppToken string `env:"SLACK_APP_TOKEN"`
```

**Verification:**
- App starts and logs "slack socket mode connected"
- Without SLACK_APP_TOKEN: logs warning, continues in one-way mode
- Reconnects automatically after disconnect

---

### Task 2: Interactive Button Handlers (OI-063)

**Add to:** `backend/internal/services/slack_event_handler.go`

```go
func (h *SlackEventHandler) handleInteraction(evt socketmode.Event) {
    callback, ok := evt.Data.(slack.InteractionCallback)
    if !ok {
        return
    }
    h.client.Ack(*evt.Request)

    switch callback.Type {
    case slack.InteractionTypeBlockActions:
        for _, action := range callback.ActionCallback.BlockActions {
            switch action.ActionID {
            case "acknowledge":
                h.handleAcknowledge(callback, action)
            case "resolve":
                h.handleResolve(callback, action)
            }
        }
    case slack.InteractionTypeViewSubmission:
        h.handleModalSubmission(callback)
    }
}

func (h *SlackEventHandler) handleAcknowledge(
    callback slack.InteractionCallback,
    action *slack.BlockAction,
) {
    incidentID, err := uuid.Parse(action.Value)
    if err != nil {
        h.postEphemeral(callback.Channel.ID, callback.User.ID,
            "Failed to process action: invalid incident ID")
        return
    }

    // Update incident status
    updates := &models.IncidentUpdate{
        Status: stringPtr("acknowledged"),
    }
    incident, err := h.incidentService.UpdateIncident(incidentID, updates,
        "slack_user", callback.User.ID)
    if err != nil {
        h.postEphemeral(callback.Channel.ID, callback.User.ID,
            fmt.Sprintf("Cannot acknowledge: %s", err.Error()))
        return
    }

    // Update original message with new buttons
    h.updateIncidentMessage(callback.Channel.ID, callback.Message.Timestamp, incident)

    // Post confirmation to channel
    h.chatService.PostMessage(callback.Channel.ID, services.Message{
        Text: fmt.Sprintf("<@%s> acknowledged INC-%d",
            callback.User.ID, incident.IncidentNumber),
    })
}

func (h *SlackEventHandler) handleResolve(
    callback slack.InteractionCallback,
    action *slack.BlockAction,
) {
    // Similar to handleAcknowledge but with status="resolved"
    // ...
}

func (h *SlackEventHandler) updateIncidentMessage(
    channelID, messageTS string,
    incident *models.Incident,
) {
    // Rebuild Block Kit message with updated status
    message := h.messageBuilder.BuildIncidentStatusMessage(incident)
    h.chatService.UpdateMessage(channelID, messageTS, message)
}
```

**Key design decisions:**
- `action.Value` contains the incident UUID (set when buttons were rendered in v0.1)
- Reuse `incidentService.UpdateIncident()` for state machine validation
- `postEphemeral` for errors (only visible to the clicking user)
- Public message for success (team sees who acknowledged)

**Verification:**
- Click Acknowledge → incident status changes, message updates, confirmation posted
- Click Acknowledge on already-acknowledged → ephemeral error
- Click Resolve → incident resolved, buttons removed

---

### Task 3: Slash Command Handler (OI-064)

**Add to:** `backend/internal/services/slack_event_handler.go`

```go
func (h *SlackEventHandler) handleSlashCommand(evt socketmode.Event) {
    cmd, ok := evt.Data.(slack.SlashCommand)
    if !ok {
        return
    }
    h.client.Ack(*evt.Request)

    parts := strings.Fields(cmd.Text)
    if len(parts) == 0 {
        h.sendHelpResponse(cmd)
        return
    }

    switch parts[0] {
    case "new":
        h.openCreateIncidentModal(cmd)
    case "list":
        h.listOpenIncidents(cmd)
    case "help":
        h.sendHelpResponse(cmd)
    default:
        h.sendHelpResponse(cmd)
    }
}

func (h *SlackEventHandler) openCreateIncidentModal(cmd slack.SlashCommand) {
    // Pre-fill title from remaining text after "new"
    prefillTitle := strings.TrimPrefix(cmd.Text, "new ")

    modalView := slack.ModalViewRequest{
        Type:       slack.VTModal,
        Title:      plainText("Declare Incident"),
        Submit:     plainText("Create"),
        Close:      plainText("Cancel"),
        CallbackID: "create_incident",
        Blocks: slack.Blocks{
            BlockSet: []slack.Block{
                // Title input
                slack.NewInputBlock("title", plainText("Title"),
                    nil,
                    slack.NewPlainTextInputBlockElement(
                        plainText("e.g., API Gateway 5xx errors"),
                        "title_input",
                    ),
                ),
                // Severity dropdown
                slack.NewInputBlock("severity", plainText("Severity"),
                    nil,
                    slack.NewOptionsSelectBlockElement(
                        slack.OptTypeStatic,
                        plainText("Select severity"),
                        "severity_input",
                        severityOptions()...,
                    ),
                ),
                // Summary (optional)
                slack.NewInputBlock("summary", plainText("Summary"),
                    nil,
                    slack.NewPlainTextInputBlockElement(
                        plainText("Brief description of the incident"),
                        "summary_input",
                    ),
                ).WithOptional(true),
            },
        },
    }

    if prefillTitle != "" {
        // Set initial value for title
        modalView.Blocks.BlockSet[0].(*slack.InputBlock).
            Element.(*slack.PlainTextInputBlockElement).
            InitialValue = prefillTitle
    }

    _, err := h.client.OpenView(cmd.TriggerID, modalView)
    if err != nil {
        slog.Error("failed to open modal", "error", err)
    }
}

func (h *SlackEventHandler) handleModalSubmission(callback slack.InteractionCallback) {
    if callback.View.CallbackID != "create_incident" {
        return
    }

    // Extract form values
    values := callback.View.State.Values
    title := values["title"]["title_input"].Value
    severity := values["severity"]["severity_input"].SelectedOption.Value
    summary := values["summary"]["summary_input"].Value

    // Create incident
    incident, err := h.incidentService.CreateIncident(&models.CreateIncidentRequest{
        Title:    title,
        Severity: severity,
        Summary:  summary,
    }, "slack_user", callback.User.ID)

    if err != nil {
        slog.Error("failed to create incident from slash command", "error", err)
        return
    }

    // Post confirmation to user
    slog.Info("incident created from slash command",
        "incident_id", incident.ID,
        "incident_number", incident.IncidentNumber,
        "created_by", callback.User.ID)
}
```

**Slack App Setup Required:**
1. Go to Slack app settings → Slash Commands → Create New Command
2. Command: `/incident`
3. Request URL: not needed for Socket Mode
4. Short Description: "Manage OpenIncident incidents"
5. Usage Hint: `[new|list|help] [title]`

**Verification:**
- `/incident new High CPU` → opens modal with pre-filled title
- Submit modal → incident created, channel created
- `/incident list` → ephemeral message with open incidents
- `/incident help` → usage instructions

---

### Task 4: Message-to-Timeline Sync (OI-065)

**Add to:** `backend/internal/services/slack_event_handler.go`

```go
func (h *SlackEventHandler) handleEventsAPI(evt socketmode.Event) {
    eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
    if !ok {
        return
    }
    h.client.Ack(*evt.Request)

    switch ev := eventsAPIEvent.InnerEvent.Data.(type) {
    case *slackevents.MessageEvent:
        h.handleChannelMessage(ev)
    }
}

func (h *SlackEventHandler) handleChannelMessage(ev *slackevents.MessageEvent) {
    // Echo prevention: skip bot messages
    if ev.SubType == "bot_message" || ev.User == h.botUserID || ev.User == "" {
        return
    }

    // Only process messages in incident channels
    incident, err := h.incidentService.GetIncidentBySlackChannelID(ev.Channel)
    if err != nil || incident == nil {
        return // Not an incident channel
    }

    // Get user info for display name
    userInfo, err := h.client.GetUserInfo(ev.User)
    displayName := ev.User
    avatarURL := ""
    if err == nil {
        displayName = userInfo.Profile.DisplayName
        if displayName == "" {
            displayName = userInfo.RealName
        }
        avatarURL = userInfo.Profile.Image72
    }

    // Create timeline entry
    content := map[string]interface{}{
        "text":        ev.Text,
        "author_id":   ev.User,
        "author_name": displayName,
        "avatar_url":  avatarURL,
        "message_ts":  ev.TimeStamp,
        "is_thread":   ev.ThreadTimeStamp != "" && ev.ThreadTimeStamp != ev.TimeStamp,
    }
    if ev.ThreadTimeStamp != "" {
        content["thread_ts"] = ev.ThreadTimeStamp
    }

    h.incidentService.CreateTimelineEntry(&models.TimelineEntry{
        IncidentID: incident.ID,
        Type:       "slack_message",
        ActorType:  "slack_user",
        ActorID:    ev.User,
        Content:    content,
    })
}
```

**New repository method needed:**
```go
// In incident_repository.go
GetBySlackChannelID(channelID string) (*models.Incident, error)
```

**Slack App Setup Required:**
1. Event Subscriptions → Subscribe to bot events:
   - `message.channels` (messages in public channels)

**Verification:**
- Post message in incident channel → timeline entry appears in web UI
- Bot messages not captured (no echo loop)
- Messages in non-incident channels ignored
- Thread replies captured with parent reference

---

### Task 5: Update Slack Message on API/UI Status Change (OI-066)

**Modify:** `backend/internal/services/incident_service.go`

**Current state:** `PostStatusUpdateToSlack()` posts a new message but doesn't update the original.

**Changes needed:**

1. **Store initial message_ts** — When the incident message is first posted to Slack, save the message timestamp:

```go
// In CreateSlackChannelForIncident(), after PostMessage:
messageTS, err := s.chatService.PostMessage(channel.ID, message)
if err == nil {
    // Store message_ts for future updates
    s.incidentRepo.UpdateSlackMessageTS(incident.ID, messageTS)
}
```

2. **Add SlackMessageTS field to Incident model:**
```go
type Incident struct {
    // ... existing fields
    SlackMessageTS string `json:"slack_message_ts" gorm:"column:slack_message_ts"`
}
```

3. **Database migration:**
```sql
ALTER TABLE incidents ADD COLUMN slack_message_ts VARCHAR(64);
```

4. **Update original message on status change:**
```go
func (s *incidentService) UpdateIncident(...) {
    // ... existing status change logic

    // Update Slack message with new status
    if incident.SlackChannelID != "" && incident.SlackMessageTS != "" {
        message := s.messageBuilder.BuildIncidentStatusMessage(incident)
        s.chatService.UpdateMessage(incident.SlackChannelID, incident.SlackMessageTS, message)
    }
}
```

5. **Build updated message with modified buttons:**
```go
func (b *SlackMessageBuilder) BuildIncidentStatusMessage(incident *models.Incident) Message {
    // Same as BuildIncidentCreatedMessage but:
    // - Status field shows current status with timestamp
    // - If acknowledged: remove Acknowledge button, keep Resolve
    // - If resolved: remove all action buttons
    // - Add "Acknowledged by X at Y" / "Resolved by X at Y" fields
}
```

**Verification:**
- Change status via web UI → Slack message updates
- Acknowledge from UI → Acknowledge button disappears in Slack
- Resolve from UI → all buttons removed in Slack
- No Slack update for incidents without channels

---

### Task 6: Create Incident Modal in Frontend (OI-067)

**Create:** `frontend/src/components/incidents/CreateIncidentModal.tsx`
**Modify:** `frontend/src/pages/IncidentsListPage.tsx`, `frontend/src/pages/HomePage.tsx`

```tsx
// CreateIncidentModal.tsx
interface CreateIncidentModalProps {
  isOpen: boolean;
  onClose: () => void;
  onCreated: (incident: Incident) => void;
}

export function CreateIncidentModal({ isOpen, onClose, onCreated }: CreateIncidentModalProps) {
  const [title, setTitle] = useState('');
  const [severity, setSeverity] = useState('high');
  const [summary, setSummary] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setIsSubmitting(true);
    setError(null);

    try {
      const incident = await createIncident({ title, severity, summary });
      onCreated(incident);
      onClose();
      // Reset form
      setTitle('');
      setSeverity('high');
      setSummary('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create incident');
    } finally {
      setIsSubmitting(false);
    }
  };

  // Modal with form, severity dropdown, submit/cancel buttons
  // Focus trap, escape key handler, backdrop click
}
```

**Wire up in IncidentsListPage:**
```tsx
const [showCreateModal, setShowCreateModal] = useState(false);

const handleDeclareIncident = () => setShowCreateModal(true);

const handleIncidentCreated = (incident: Incident) => {
  navigate(`/incidents/${incident.id}`);
};

// In JSX:
<CreateIncidentModal
  isOpen={showCreateModal}
  onClose={() => setShowCreateModal(false)}
  onCreated={handleIncidentCreated}
/>
```

**Verification:**
- Click "Declare incident" → modal opens
- Fill form → submit → navigates to new incident
- Empty title → validation error
- API error → error shown in modal
- Escape key → modal closes

---

### Task 7: Slack Channel Link in Detail Page (OI-068)

**Modify:** `frontend/src/components/incidents/PropertiesPanel.tsx`

Add a Slack channel link row:
```tsx
{incident.slack_channel_name && (
  <PropertyRow
    label="Slack Channel"
    icon={<SlackIcon />}
    value={
      <a
        href={`https://app.slack.com/client/${teamId}/${incident.slack_channel_id}`}
        target="_blank"
        rel="noopener noreferrer"
      >
        #{incident.slack_channel_name}
      </a>
    }
  />
)}
```

**Verification:**
- Incident with Slack channel → clickable link shown
- Incident without Slack channel → row not shown
- Link opens Slack in new tab

---

### Task 8: Channel Archival on Resolution (OI-069)

**Modify:** `backend/internal/services/incident_service.go`

```go
func (s *incidentService) UpdateIncident(...) {
    // ... after status change to "resolved"

    if newStatus == "resolved" && incident.SlackChannelID != "" {
        go s.scheduleChannelArchival(incident)
    }
}

func (s *incidentService) scheduleChannelArchival(incident *models.Incident) {
    delay := s.config.SlackArchiveDelay // Default: 24h

    // Post final message
    s.chatService.PostMessage(incident.SlackChannelID, services.Message{
        Text: fmt.Sprintf("This incident has been resolved. Channel will be archived in %s.",
            delay.String()),
    })

    // Wait then archive
    time.Sleep(delay)

    // Re-check status (might have been re-opened)
    current, err := s.incidentRepo.GetByID(incident.ID)
    if err != nil || current.Status != "resolved" {
        return // Incident re-opened, don't archive
    }

    if err := s.chatService.ArchiveChannel(incident.SlackChannelID); err != nil {
        slog.Error("failed to archive channel", "error", err)
        return
    }

    s.createTimelineEntry(incident.ID, "slack_channel_archived", map[string]interface{}{
        "channel_id": incident.SlackChannelID,
    })
}
```

**Config:** `SLACK_ARCHIVE_DELAY=24h` (Go duration format)

**Verification:**
- Resolve incident → message posted about upcoming archival
- After delay → channel archived
- Re-open incident before delay → archival cancelled

---

### Task 9: Integration Tests (OI-070)

**Create:** `backend/internal/services/slack_event_handler_test.go`

```go
// Mock implementations
type MockSocketModeClient struct { /* ... */ }
type MockChatService struct { /* ... */ }

func TestHandleAcknowledgeButton(t *testing.T) { /* ... */ }
func TestHandleResolveButton(t *testing.T) { /* ... */ }
func TestHandleInvalidTransition(t *testing.T) { /* ... */ }
func TestSlashCommandNew(t *testing.T) { /* ... */ }
func TestSlashCommandList(t *testing.T) { /* ... */ }
func TestMessageToTimeline(t *testing.T) { /* ... */ }
func TestBotMessageIgnored(t *testing.T) { /* ... */ }
func TestChannelArchivalOnResolve(t *testing.T) { /* ... */ }
```

---

### Task 10: Documentation (OI-071)

Update:
- `.env.example` — add `SLACK_APP_TOKEN`, `SLACK_ARCHIVE_DELAY`
- `README.md` — v0.2 features section
- New: `docs/slack-setup.md` — step-by-step Slack app configuration guide

---

## Critical Files

| File | Action | Purpose |
|------|--------|---------|
| `backend/internal/services/slack_event_handler.go` | **Create** | Socket Mode listener + event handlers |
| `backend/internal/services/incident_service.go` | **Modify** | Message TS storage, channel archival |
| `backend/internal/services/slack_message_builder.go` | **Modify** | Status-aware message builder |
| `backend/internal/models/incident.go` | **Modify** | Add SlackMessageTS field |
| `backend/internal/repository/incident_repository.go` | **Modify** | GetBySlackChannelID, UpdateSlackMessageTS |
| `backend/cmd/openincident/main.go` | **Modify** | Wire up Socket Mode |
| `backend/internal/config/config.go` | **Modify** | Add SlackAppToken, SlackArchiveDelay |
| `frontend/src/components/incidents/CreateIncidentModal.tsx` | **Create** | Declare incident form |
| `frontend/src/pages/IncidentsListPage.tsx` | **Modify** | Wire up create modal |
| `frontend/src/pages/HomePage.tsx` | **Modify** | Wire up create modal |
| `frontend/src/components/incidents/PropertiesPanel.tsx` | **Modify** | Slack channel link |
| `backend/migrations/XXXX_add_slack_message_ts.sql` | **Create** | New column migration |

---

## Implementation Order

```
Phase 1: Foundation (OI-062)
└── Socket Mode listener — everything else depends on this

Phase 2: Core Bidirectional (OI-063, OI-064, OI-065) — can be parallel
├── Interactive button handlers (Acknowledge/Resolve from Slack)
├── Slash command handler (/incident new, list, help)
└── Message-to-timeline sync

Phase 3: Polish (OI-066, OI-067, OI-068) — can be parallel
├── Update Slack message on API/UI status change
├── Create incident modal in frontend
└── Slack channel link in detail page

Phase 4: Lifecycle (OI-069)
└── Channel archival on resolution

Phase 5: Quality (OI-070, OI-071) — can be parallel
├── Integration tests
└── Documentation
```

---

## Database Migration

```sql
-- Migration: add_slack_message_ts
ALTER TABLE incidents ADD COLUMN slack_message_ts VARCHAR(64);

-- Index for channel ID lookup (message-to-timeline sync)
CREATE INDEX idx_incidents_slack_channel_id ON incidents(slack_channel_id)
    WHERE slack_channel_id IS NOT NULL AND slack_channel_id != '';
```

---

## Slack App Configuration Changes

The existing Slack app from v0.1 needs these additions:

1. **Enable Socket Mode** (Settings → Socket Mode → Enable)
2. **Generate App-Level Token** (Settings → Basic Information → App-Level Tokens)
   - Scope: `connections:write`
3. **Add Slash Command** (Features → Slash Commands → Create New)
   - Command: `/incident`
   - Description: "Manage OpenIncident incidents"
4. **Subscribe to Events** (Features → Event Subscriptions → Subscribe to Bot Events)
   - `message.channels` — messages in public channels
5. **Add OAuth Scopes** (Features → OAuth & Permissions → Bot Token Scopes)
   - `commands` — slash commands
   - `channels:history` — read channel messages

---

## Success Criteria

- [ ] Socket Mode connects and receives events
- [ ] Acknowledge button click → incident acknowledged, message updated
- [ ] Resolve button click → incident resolved, buttons removed
- [ ] `/incident new` → modal opens → incident created with Slack channel
- [ ] `/incident list` → shows open incidents
- [ ] Slack messages in incident channels → timeline entries in web UI
- [ ] Bot messages not captured (no echo loop)
- [ ] Status change from web UI → Slack message updated
- [ ] "Declare incident" modal works in frontend
- [ ] Slack channel link shown in incident detail page
- [ ] Resolved incidents → channel archived after delay
- [ ] All tests pass without Slack credentials
- [ ] Application works without SLACK_APP_TOKEN (one-way mode)

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Socket Mode disconnects | Automatic reconnect with exponential backoff (built into slack-go) |
| Echo loops in message sync | Multi-layer filtering: bot_message subtype, bot user ID, channel prefix |
| Rate limiting on user lookups | Cache user info (display name, avatar) with TTL |
| Stale buttons after status change | Update message immediately on every status transition |
| Channel archival during re-investigation | Re-check incident status before archiving |
| Modal submission race condition | Optimistic UI + server-side idempotency |

---

## Definition of Done

- [x] All 10 tasks (OI-062 to OI-071) completed
- [x] Database migration applied (000005_add_slack_message_ts)
- [x] Code formatted and passes linting
- [x] Unit and integration tests passing
- [x] Documentation updated (README, .env.example)
- [ ] Manual testing of all Slack interactions
- [ ] Committed with conventional commit messages
- [ ] v0.2 milestone complete: Full bidirectional Slack sync working
