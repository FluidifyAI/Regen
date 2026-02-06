# Epic 004: Slack Integration - Implementation Plan

## Context

This epic implements the ChatService abstraction layer and Slack integration that enables automatic channel creation and message posting when incidents are created. This completes the v0.1 MVP goal: "Alert fires → incident auto-created → Slack channel appears with incident details."

**Why this change:**
- Delivers the final piece of CLAUDE.md v0.1: Slack channel auto-creation
- Establishes the ChatService interface pattern (ADR-005) for future Teams support
- Implements async job processing pattern (ADR-007) for resilient Slack operations
- Enables incident teams to collaborate in dedicated Slack channels

**User Story:**
As an on-call engineer, when a critical alert fires and an incident is created, I want a dedicated Slack channel to automatically appear with all incident details so that my team can immediately start collaborating in a focused space.

---

## Architectural Decisions

### 1. ChatService Interface Pattern (ADR-005)

**Abstract interface for chat operations:**
```go
type ChatService interface {
    CreateChannel(name, description string) (*Channel, error)
    PostMessage(channelID string, message Message) (string, error)
    UpdateMessage(channelID, messageTS string, message Message) error
    ArchiveChannel(channelID string) error
}
```

**Why:**
- Future-proof for Teams integration (v0.8)
- Testable with mock implementations
- Clean separation between business logic and chat platform specifics

### 2. Async Job Queue Pattern (ADR-007)

**Redis-backed queue for Slack operations:**
- Slack API calls are slow (200-500ms) and can fail
- Webhook handler must return quickly (<100ms)
- Failed Slack operations should not fail incident creation
- Retry logic with exponential backoff

**Job structure:**
```go
type SlackJob struct {
    Type       string // "create_channel", "post_message"
    IncidentID uuid.UUID
    Payload    map[string]interface{}
    Attempts   int
    CreatedAt  time.Time
}
```

### 3. Channel Naming Convention

**Format:** `inc-{number}-{slug}`

**Examples:**
- `inc-042-api-gateway-errors`
- `inc-123-database-connection-pool-exhausted`

**Rules:**
- Lowercase only
- Max 80 characters (Slack limit)
- Hyphens only (replace underscores, spaces)
- Truncate slug if needed, keep number intact
- Handle collisions by appending `-1`, `-2`, etc.

### 4. Graceful Degradation

**Application works without Slack:**
- If `SLACK_BOT_TOKEN` not set → log warning, continue
- Incidents still created, just no channel
- Timeline entry: `type=slack_channel_creation_failed`
- UI shows "Slack not configured" instead of channel link

---

## Implementation Steps

### Task 1: Define ChatService Interface (OI-021)

**Create:** `backend/internal/services/chat_service.go`

Define core abstractions:

```go
package services

// ChatService defines the interface for chat platform operations
type ChatService interface {
    // CreateChannel creates a new channel with the given name and description
    CreateChannel(name, description string) (*Channel, error)

    // PostMessage posts a message to the specified channel
    PostMessage(channelID string, message Message) (string, error)

    // UpdateMessage updates an existing message
    UpdateMessage(channelID, messageTS string, message Message) error

    // ArchiveChannel archives the specified channel
    ArchiveChannel(channelID string) error
}

// Channel represents a chat channel
type Channel struct {
    ID          string
    Name        string
    URL         string
}

// Message represents a chat message
type Message struct {
    Text   string
    Blocks []interface{} // Platform-specific block structures
    ThreadTS string      // For threaded replies
}
```

**Documentation:**
- Each method should document expected behavior
- Error conditions should be clearly defined
- Thread safety requirements

**Verification:** Compiles, interface is clear and documented

---

### Task 2: Implement SlackService (OI-022)

**Create:** `backend/internal/services/slack_service.go`

**Dependencies:**
```bash
go get github.com/slack-go/slack@latest
```

**Implementation:**

```go
package services

import (
    "fmt"
    "github.com/slack-go/slack"
)

type slackService struct {
    client *slack.Client
}

func NewSlackService(token string) (ChatService, error) {
    if token == "" {
        return nil, fmt.Errorf("slack bot token is required")
    }

    client := slack.New(token)

    // Validate token on startup
    auth, err := client.AuthTest()
    if err != nil {
        return nil, fmt.Errorf("slack auth failed: %w", err)
    }

    slog.Info("slack service initialized",
        "bot_id", auth.BotID,
        "team", auth.Team)

    return &slackService{client: client}, nil
}

func (s *slackService) CreateChannel(name, description string) (*Channel, error) {
    // Sanitize channel name
    sanitized := sanitizeChannelName(name)

    // Create channel
    channel, err := s.client.CreateConversation(slack.CreateConversationParams{
        ChannelName: sanitized,
        IsPrivate:   false,
    })
    if err != nil {
        // Handle name collision
        if isNameCollisionError(err) {
            return s.createChannelWithSuffix(sanitized)
        }
        return nil, fmt.Errorf("failed to create channel: %w", err)
    }

    // Set channel topic/description
    if description != "" {
        _, err = s.client.SetTopicOfConversation(channel.ID, description)
        if err != nil {
            slog.Warn("failed to set channel topic", "error", err)
        }
    }

    return &Channel{
        ID:   channel.ID,
        Name: channel.Name,
        URL:  fmt.Sprintf("https://app.slack.com/client/%s/%s", auth.TeamID, channel.ID),
    }, nil
}

func (s *slackService) PostMessage(channelID string, message Message) (string, error) {
    opts := []slack.MsgOption{
        slack.MsgOptionText(message.Text, false),
    }

    if len(message.Blocks) > 0 {
        opts = append(opts, slack.MsgOptionBlocks(message.Blocks...))
    }

    if message.ThreadTS != "" {
        opts = append(opts, slack.MsgOptionTS(message.ThreadTS))
    }

    _, timestamp, err := s.client.PostMessage(channelID, opts...)
    if err != nil {
        return "", fmt.Errorf("failed to post message: %w", err)
    }

    return timestamp, nil
}

// Helper functions
func sanitizeChannelName(name string) string {
    // Lowercase, replace spaces/underscores with hyphens
    // Remove invalid characters
    // Truncate to 80 characters
}

func isNameCollisionError(err error) bool {
    // Check if error is "name_taken"
}

func (s *slackService) createChannelWithSuffix(baseName string) (*Channel, error) {
    // Try baseName-1, baseName-2, etc.
}
```

**Error handling:**
- Rate limit errors → exponential backoff
- Auth errors → return immediately
- Network errors → retry with backoff

**Verification:**
- Unit tests with mock Slack API
- Integration test with real token (optional)

---

### Task 3: Implement Channel Name Sanitization (OI-023)

**Add to:** `backend/internal/services/slack_service.go`

**Function:**

```go
func sanitizeChannelName(name string) string {
    // 1. Convert to lowercase
    name = strings.ToLower(name)

    // 2. Replace invalid characters with hyphens
    reg := regexp.MustCompile(`[^a-z0-9-]`)
    name = reg.ReplaceAllString(name, "-")

    // 3. Replace multiple consecutive hyphens with single hyphen
    reg = regexp.MustCompile(`-+`)
    name = reg.ReplaceAllString(name, "-")

    // 4. Trim hyphens from start and end
    name = strings.Trim(name, "-")

    // 5. Truncate to 80 characters (Slack limit)
    if len(name) > 80 {
        name = name[:80]
        name = strings.TrimRight(name, "-")
    }

    return name
}

func formatIncidentChannelName(incidentNumber int, slug string) string {
    base := fmt.Sprintf("inc-%d-%s", incidentNumber, slug)
    return sanitizeChannelName(base)
}
```

**Test cases:**
- `inc-42-API Gateway Errors` → `inc-42-api-gateway-errors`
- `inc-123-database_connection_pool` → `inc-123-database-connection-pool`
- `inc-1-very-long-name-that-exceeds-eighty-characters...` → truncates correctly
- `inc-5-special!@#chars` → `inc-5-special-chars`

---

### Task 4: Implement Incident Message Formatting (OI-024)

**Create:** `backend/internal/services/slack_message_builder.go`

**Slack Block Kit message:**

```go
package services

import (
    "fmt"
    "github.com/slack-go/slack"
    "github.com/openincident/internal/models"
)

type SlackMessageBuilder struct{}

func NewSlackMessageBuilder() *SlackMessageBuilder {
    return &SlackMessageBuilder{}
}

func (b *SlackMessageBuilder) BuildIncidentCreatedMessage(
    incident *models.Incident,
    alerts []models.Alert,
) Message {
    blocks := []slack.Block{
        // Header block with emoji and title
        slack.NewHeaderBlock(
            slack.NewTextBlockObject(
                slack.PlainTextType,
                fmt.Sprintf("%s INC-%d: %s",
                    getSeverityEmoji(incident.Severity),
                    incident.IncidentNumber,
                    incident.Title),
                false,
                false,
            ),
        ),

        // Divider
        slack.NewDividerBlock(),

        // Details section
        slack.NewSectionBlock(
            nil,
            []*slack.TextBlockObject{
                slack.NewTextBlockObject(
                    slack.MarkdownType,
                    fmt.Sprintf("*Severity:* %s %s",
                        getSeverityEmoji(incident.Severity),
                        incident.Severity),
                    false,
                    false,
                ),
                slack.NewTextBlockObject(
                    slack.MarkdownType,
                    fmt.Sprintf("*Status:* %s", incident.Status),
                    false,
                    false,
                ),
                slack.NewTextBlockObject(
                    slack.MarkdownType,
                    fmt.Sprintf("*Created:* <!date^%d^{date_short_pretty} at {time}|%s>",
                        incident.TriggeredAt.Unix(),
                        incident.TriggeredAt.Format("2006-01-02 15:04:05")),
                    false,
                    false,
                ),
            },
            nil,
        ),

        // Divider
        slack.NewDividerBlock(),
    }

    // Add linked alerts section if any
    if len(alerts) > 0 {
        alertsText := "*Linked Alerts:*\n"
        for _, alert := range alerts {
            alertsText += fmt.Sprintf("• %s: %s\n", alert.Source, alert.Title)
        }

        blocks = append(blocks, slack.NewSectionBlock(
            slack.NewTextBlockObject(
                slack.MarkdownType,
                alertsText,
                false,
                false,
            ),
            nil,
            nil,
        ))

        blocks = append(blocks, slack.NewDividerBlock())
    }

    // Action buttons
    blocks = append(blocks, slack.NewActionBlock(
        "incident_actions",
        slack.NewButtonBlockElement(
            "acknowledge",
            incident.ID.String(),
            slack.NewTextBlockObject(
                slack.PlainTextType,
                "👀 Acknowledge",
                false,
                false,
            ),
        ).WithStyle(slack.StylePrimary),
        slack.NewButtonBlockElement(
            "resolve",
            incident.ID.String(),
            slack.NewTextBlockObject(
                slack.PlainTextType,
                "✅ Resolve",
                false,
                false,
            ),
        ).WithStyle(slack.StyleDanger),
    ))

    return Message{
        Text: fmt.Sprintf("INC-%d: %s", incident.IncidentNumber, incident.Title),
        Blocks: blocksToInterfaces(blocks),
    }
}

func getSeverityEmoji(severity models.IncidentSeverity) string {
    switch severity {
    case models.IncidentSeverityCritical:
        return "🔴"
    case models.IncidentSeverityHigh:
        return "🟠"
    case models.IncidentSeverityMedium:
        return "🟡"
    case models.IncidentSeverityLow:
        return "🟢"
    default:
        return "⚪"
    }
}

func blocksToInterfaces(blocks []slack.Block) []interface{} {
    result := make([]interface{}, len(blocks))
    for i, block := range blocks {
        result[i] = block
    }
    return result
}
```

**Verification:**
- Test with Block Kit Builder: https://app.slack.com/block-kit-builder
- Renders correctly on mobile and desktop
- Buttons are clickable (handler implementation later)

---

### Task 5: Connect Incident Creation to Slack (OI-025)

**Modify:** `backend/internal/services/incident_service.go`

**Add Slack integration:**

```go
type IncidentService interface {
    CreateIncidentFromAlert(alert *models.Alert) (*models.Incident, error)
    ShouldCreateIncident(severity models.AlertSeverity) bool

    // NEW: Slack integration
    CreateSlackChannelForIncident(incident *models.Incident, alerts []models.Alert) error
}

type incidentService struct {
    incidentRepo  repository.IncidentRepository
    timelineRepo  repository.TimelineRepository
    alertRepo     repository.AlertRepository
    chatService   ChatService // NEW
    messageBuilder *SlackMessageBuilder // NEW
    db            *gorm.DB
}

func NewIncidentService(
    incidentRepo repository.IncidentRepository,
    timelineRepo repository.TimelineRepository,
    alertRepo repository.AlertRepository,
    chatService ChatService, // NEW - can be nil
    db *gorm.DB,
) IncidentService {
    return &incidentService{
        incidentRepo:  incidentRepo,
        timelineRepo:  timelineRepo,
        alertRepo:     alertRepo,
        chatService:   chatService,
        messageBuilder: NewSlackMessageBuilder(),
        db:            db,
    }
}

func (s *incidentService) CreateIncidentFromAlert(alert *models.Alert) (*models.Incident, error) {
    // ... existing code creates incident, links alert, creates timeline entry

    // NEW: Create Slack channel (non-blocking)
    if s.chatService != nil {
        go func() {
            alerts := []models.Alert{*alert}
            if err := s.CreateSlackChannelForIncident(incident, alerts); err != nil {
                slog.Error("failed to create slack channel",
                    "incident_id", incident.ID,
                    "error", err)
            }
        }()
    }

    return incident, nil
}

func (s *incidentService) CreateSlackChannelForIncident(
    incident *models.Incident,
    alerts []models.Alert,
) error {
    // 1. Format channel name
    channelName := formatIncidentChannelName(incident.IncidentNumber, incident.Slug)
    description := fmt.Sprintf("Incident #%d: %s", incident.IncidentNumber, incident.Title)

    // 2. Create channel
    channel, err := s.chatService.CreateChannel(channelName, description)
    if err != nil {
        // Create timeline entry for failure
        s.createTimelineEntry(incident.ID, "slack_channel_creation_failed", map[string]interface{}{
            "error": err.Error(),
        })
        return fmt.Errorf("failed to create slack channel: %w", err)
    }

    // 3. Update incident with Slack details
    err = s.incidentRepo.UpdateSlackChannel(incident.ID, channel.ID, channel.Name)
    if err != nil {
        slog.Error("failed to update incident with slack channel",
            "incident_id", incident.ID,
            "channel_id", channel.ID,
            "error", err)
    }

    // 4. Post initial message
    message := s.messageBuilder.BuildIncidentCreatedMessage(incident, alerts)
    _, err = s.chatService.PostMessage(channel.ID, message)
    if err != nil {
        slog.Error("failed to post initial message",
            "incident_id", incident.ID,
            "channel_id", channel.ID,
            "error", err)
    }

    // 5. Create timeline entry for success
    s.createTimelineEntry(incident.ID, "slack_channel_created", map[string]interface{}{
        "channel_id": channel.ID,
        "channel_name": channel.Name,
        "channel_url": channel.URL,
    })

    slog.Info("slack channel created for incident",
        "incident_id", incident.ID,
        "incident_number", incident.IncidentNumber,
        "channel_id", channel.ID,
        "channel_name", channel.Name)

    return nil
}

func (s *incidentService) createTimelineEntry(
    incidentID uuid.UUID,
    entryType string,
    content map[string]interface{},
) {
    entry := &models.TimelineEntry{
        IncidentID: incidentID,
        Type:       entryType,
        ActorType:  "system",
        ActorID:    "slack_service",
        Content:    content,
    }

    if err := s.timelineRepo.Create(entry); err != nil {
        slog.Error("failed to create timeline entry",
            "incident_id", incidentID,
            "type", entryType,
            "error", err)
    }
}
```

**Add to IncidentRepository:**

```go
// In backend/internal/repository/incident_repository.go
UpdateSlackChannel(id uuid.UUID, channelID, channelName string) error
```

**Verification:**
- Incident created → Slack channel appears within 1-2 seconds
- Channel name matches convention
- Initial message posted
- Timeline entries created

---

### Task 6: Implement Redis Job Queue (OI-026)

**Create:** `backend/internal/worker/slack_worker.go`

**Redis queue structure:**

```go
package worker

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
    "github.com/google/uuid"
)

const (
    SlackJobQueue = "slack:jobs"
    SlackJobDLQ   = "slack:dlq"
)

type SlackJob struct {
    ID         string                 `json:"id"`
    Type       string                 `json:"type"`
    IncidentID uuid.UUID              `json:"incident_id"`
    Payload    map[string]interface{} `json:"payload"`
    Attempts   int                    `json:"attempts"`
    MaxRetries int                    `json:"max_retries"`
    CreatedAt  time.Time              `json:"created_at"`
}

type SlackWorker struct {
    redis       *redis.Client
    chatService services.ChatService
    ctx         context.Context
    cancel      context.CancelFunc
}

func NewSlackWorker(
    redis *redis.Client,
    chatService services.ChatService,
) *SlackWorker {
    ctx, cancel := context.WithCancel(context.Background())

    return &SlackWorker{
        redis:       redis,
        chatService: chatService,
        ctx:         ctx,
        cancel:      cancel,
    }
}

func (w *SlackWorker) Start() {
    slog.Info("slack worker started")

    for {
        select {
        case <-w.ctx.Done():
            slog.Info("slack worker stopped")
            return
        default:
            w.processJob()
        }
    }
}

func (w *SlackWorker) Stop() {
    slog.Info("stopping slack worker")
    w.cancel()
}

func (w *SlackWorker) processJob() {
    // BLPOP with 5 second timeout
    result, err := w.redis.BLPop(w.ctx, 5*time.Second, SlackJobQueue).Result()
    if err != nil {
        if err != redis.Nil {
            slog.Error("failed to pop job from queue", "error", err)
        }
        return
    }

    if len(result) < 2 {
        return
    }

    jobData := result[1]
    var job SlackJob
    if err := json.Unmarshal([]byte(jobData), &job); err != nil {
        slog.Error("failed to unmarshal job", "error", err)
        return
    }

    // Execute job
    if err := w.executeJob(&job); err != nil {
        w.handleJobFailure(&job, err)
    } else {
        slog.Info("job completed successfully",
            "job_id", job.ID,
            "type", job.Type,
            "incident_id", job.IncidentID)
    }
}

func (w *SlackWorker) executeJob(job *SlackJob) error {
    job.Attempts++

    switch job.Type {
    case "create_channel":
        return w.executeCreateChannel(job)
    case "post_message":
        return w.executePostMessage(job)
    default:
        return fmt.Errorf("unknown job type: %s", job.Type)
    }
}

func (w *SlackWorker) handleJobFailure(job *SlackJob, err error) {
    slog.Error("job failed",
        "job_id", job.ID,
        "type", job.Type,
        "incident_id", job.IncidentID,
        "attempt", job.Attempts,
        "error", err)

    // Retry with exponential backoff
    if job.Attempts < job.MaxRetries {
        delay := time.Duration(job.Attempts*job.Attempts) * time.Second
        time.Sleep(delay)

        // Re-queue job
        if err := w.enqueueJob(job); err != nil {
            slog.Error("failed to re-queue job", "job_id", job.ID, "error", err)
        }
    } else {
        // Move to DLQ
        jobData, _ := json.Marshal(job)
        w.redis.RPush(w.ctx, SlackJobDLQ, jobData)

        slog.Error("job moved to DLQ after max retries",
            "job_id", job.ID,
            "type", job.Type,
            "incident_id", job.IncidentID)
    }
}

func (w *SlackWorker) enqueueJob(job *SlackJob) error {
    jobData, err := json.Marshal(job)
    if err != nil {
        return err
    }

    return w.redis.RPush(w.ctx, SlackJobQueue, jobData).Err()
}

// Job-specific execution
func (w *SlackWorker) executeCreateChannel(job *SlackJob) error {
    // Extract payload fields
    // Call chatService.CreateChannel()
}

func (w *SlackWorker) executePostMessage(job *SlackJob) error {
    // Extract payload fields
    // Call chatService.PostMessage()
}
```

**Modify incident_service.go to enqueue jobs instead of direct calls:**

```go
func (s *incidentService) CreateSlackChannelForIncident(...) error {
    // Instead of direct chatService calls, enqueue jobs
    job := &worker.SlackJob{
        ID:         uuid.New().String(),
        Type:       "create_channel",
        IncidentID: incident.ID,
        Payload: map[string]interface{}{
            "channel_name": channelName,
            "description":  description,
            "incident_number": incident.IncidentNumber,
            "alerts": alerts,
        },
        MaxRetries: 3,
        CreatedAt:  time.Now(),
    }

    return s.enqueueSlackJob(job)
}
```

**Dependencies:**
```bash
go get github.com/redis/go-redis/v9@latest
```

**Verification:**
- Jobs are queued to Redis
- Worker consumes jobs
- Failed jobs are retried
- DLQ contains permanently failed jobs

---

### Task 7: Add Slack Configuration Validation (OI-027)

**Create:** `backend/internal/services/slack_validator.go`

```go
package services

import (
    "fmt"
    "log/slog"
    "strings"

    "github.com/slack-go/slack"
)

type SlackValidator struct {
    client *slack.Client
}

func NewSlackValidator(token string) *SlackValidator {
    return &SlackValidator{
        client: slack.New(token),
    }
}

func (v *SlackValidator) ValidateToken() error {
    auth, err := v.client.AuthTest()
    if err != nil {
        return fmt.Errorf("slack token is invalid: %w", err)
    }

    slog.Info("slack token validated",
        "bot_id", auth.BotID,
        "bot_user_id", auth.UserID,
        "team", auth.Team,
        "team_id", auth.TeamID)

    return nil
}

func (v *SlackValidator) ValidateScopes() error {
    requiredScopes := []string{
        "channels:manage",  // Create channels
        "channels:read",    // Read channel info
        "chat:write",       // Post messages
    }

    auth, err := v.client.AuthTest()
    if err != nil {
        return err
    }

    // Note: slack.AuthTest() doesn't return scopes directly
    // We'll try to call a method for each required scope

    // Test channels:read
    _, _, err = v.client.GetConversations(&slack.GetConversationsParameters{
        Limit: 1,
    })
    if err != nil {
        return fmt.Errorf("missing 'channels:read' scope: %w", err)
    }

    // Additional scope checks...

    slog.Info("slack scopes validated", "required_scopes", requiredScopes)
    return nil
}
```

**Modify main.go to validate on startup:**

```go
func main() {
    cfg := config.Load()

    // ... database connection

    // Initialize Slack (optional, graceful degradation)
    var chatService services.ChatService
    if cfg.SlackBotToken != "" {
        validator := services.NewSlackValidator(cfg.SlackBotToken)

        if err := validator.ValidateToken(); err != nil {
            slog.Error("slack token validation failed", "error", err)
            slog.Warn("continuing without slack integration - incidents will be created but no channels will be created")
        } else if err := validator.ValidateScopes(); err != nil {
            slog.Error("slack scope validation failed", "error", err)
            slog.Warn("continuing without slack integration - please check bot permissions")
        } else {
            var err error
            chatService, err = services.NewSlackService(cfg.SlackBotToken)
            if err != nil {
                slog.Error("failed to initialize slack service", "error", err)
                slog.Warn("continuing without slack integration")
            } else {
                slog.Info("slack integration enabled")
            }
        }
    } else {
        slog.Warn("SLACK_BOT_TOKEN not set - running in degraded mode without slack integration")
    }

    // ... continue with services initialization
}
```

**Verification:**
- With valid token → validates successfully
- With invalid token → clear error message
- Without token → warning, continues without Slack
- Missing scopes → clear error message

---

## Critical Files

| File | Action | Purpose |
|------|--------|---------|
| `backend/internal/services/chat_service.go` | Create | Abstract ChatService interface |
| `backend/internal/services/slack_service.go` | Create | Slack implementation of ChatService |
| `backend/internal/services/slack_message_builder.go` | Create | Slack Block Kit message formatting |
| `backend/internal/services/slack_validator.go` | Create | Token and scope validation |
| `backend/internal/services/incident_service.go` | Modify | Add Slack channel creation to incident flow |
| `backend/internal/worker/slack_worker.go` | Create | Redis-backed async job processor |
| `backend/internal/repository/incident_repository.go` | Modify | Add UpdateSlackChannel method |
| `backend/cmd/openincident/main.go` | Modify | Initialize Slack service and worker |
| `backend/go.mod` | Modify | Add slack-go and redis dependencies |

---

## Implementation Order

1. **OI-021**: ChatService interface (foundation)
2. **OI-022**: SlackService implementation (core Slack integration)
3. **OI-023**: Channel name sanitization (utility)
4. **OI-024**: Message formatting with Block Kit (UI)
5. **OI-027**: Configuration validation (safety)
6. **OI-025**: Connect to incident creation (integration)
7. **OI-026**: Redis job queue (async processing)

**Note:** Task 6 (OI-026) is deferred to last because synchronous implementation (Task 5) provides value immediately. Queue is an optimization for reliability.

---

## Success Criteria

✅ ChatService interface defined with clear contracts
✅ SlackService implements ChatService interface
✅ Incident creation triggers Slack channel creation
✅ Channel naming follows `inc-{number}-{slug}` convention
✅ Channel names sanitized and truncated correctly
✅ Initial message posted with rich Block Kit formatting
✅ Severity displayed with appropriate emojis
✅ Linked alerts shown in message
✅ Action buttons rendered (Acknowledge, Resolve)
✅ Timeline entries created for Slack operations
✅ Incident record updated with channel ID and name
✅ Redis job queue handles async operations
✅ Failed jobs retried with exponential backoff
✅ DLQ contains permanently failed jobs
✅ Slack errors do not fail incident creation
✅ Application works without Slack configured (degraded mode)
✅ Clear error messages for invalid tokens or missing scopes
✅ Startup validation checks token and scopes

---

## Testing Strategy

### Unit Tests
- `sanitizeChannelName()` with various inputs
- `formatIncidentChannelName()` edge cases
- Message builder generates correct Block Kit JSON
- Job queue serialization/deserialization

### Integration Tests
- Create incident → Slack channel created
- Channel naming with collision handling
- Message posting with rich formatting
- Worker processes jobs from Redis

### Manual Testing
```bash
# 1. Start services
make dev

# 2. Trigger webhook (creates incident)
curl -X POST http://localhost:8080/api/v1/webhooks/prometheus \
  -H "Content-Type: application/json" \
  -d '{...}'

# 3. Check Slack
# - New channel should appear: #inc-001-high-cpu-usage
# - Initial message posted with incident details
# - Action buttons visible

# 4. Check database
SELECT slack_channel_id, slack_channel_name FROM incidents WHERE incident_number = 1;

# 5. Check timeline
SELECT type, content FROM timeline_entries WHERE incident_id = '...';
```

---

## Risk Mitigation

**Slack API failures:** Async queue with retries ensures resilience

**Rate limiting:** Exponential backoff in worker prevents API bans

**Token issues:** Validation on startup with clear error messages

**Graceful degradation:** Application works without Slack configured

**Channel name collisions:** Automatic suffix appending handles duplicates

---

## Future Enhancements (Post-v0.1)

- Interactive button handlers (acknowledge/resolve from Slack)
- Bidirectional sync (Slack messages → timeline entries)
- Thread management for alert updates
- Channel archiving when incident resolved
- Custom channel templates per team
- Slack command support (`/incident new`, `/incident list`)

---

## Definition of Done

- [ ] All 7 tasks (OI-021 to OI-027) completed
- [ ] Code formatted with `gofmt` and passes `go vet`
- [ ] Unit tests written and passing
- [ ] Integration tests passing
- [ ] Manual testing completed
- [ ] Documentation updated (.env.example, README)
- [ ] Committed with conventional commit messages
- [ ] v0.1 milestone complete: Alert → Incident → Slack channel flow works end-to-end
