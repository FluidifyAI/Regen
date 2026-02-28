# AI Post-Mortem Agent — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the Post-Mortem Agent — an AI team member that auto-drafts a post-mortem when an incident resolves, notifies the incident channel, and DMs the commander.

**Architecture:** Agent users are seeded rows in the `users` table (`auth_source='ai'`). An `AICoordinator` goroutine subscribes to Redis pub/sub and routes `incident.resolved` events to the `PostMortemAgent`. The agent calls the existing `PostMortemService.GeneratePostMortem` as itself, then notifies via `SlackService.PostMessage` + `TeamsService` + `MultiChatService.SendDirectMessage`.

**Tech Stack:** Go 1.21, PostgreSQL (GORM), Redis pub/sub (`github.com/redis/go-redis/v9`), React + TypeScript, existing `PostMortemService`, `AIService`, `MultiChatService`.

**Design doc:** `docs/plans/2026-02-28-ai-postmortem-agent-design.md`

**Module path:** `github.com/openincident/openincident`

---

## Task 1: DB Migration 000025 + Model Updates

**Files:**
- Create: `backend/migrations/000025_add_agent_fields.up.sql`
- Create: `backend/migrations/000025_add_agent_fields.down.sql`
- Modify: `backend/internal/models/user.go`
- Modify: `backend/internal/models/incident.go`

### Step 1: Write the up migration

Create `backend/migrations/000025_add_agent_fields.up.sql`:

```sql
-- Add agent_type to distinguish AI agent users from human users.
-- NULL for all human users; non-null only for auth_source='ai' rows.
ALTER TABLE users ADD COLUMN IF NOT EXISTS agent_type VARCHAR(50);

-- Add active flag to users so agents can be toggled on/off from the UI.
-- Default true so all existing human users remain active.
ALTER TABLE users ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT true;

-- Add ai_enabled flag to incidents.
-- Default true so existing incidents and manually-created incidents get AI by default.
-- Set to false via: routing rules, per-integration default, or incident Properties panel.
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS ai_enabled BOOLEAN NOT NULL DEFAULT true;
```

### Step 2: Write the down migration

Create `backend/migrations/000025_add_agent_fields.down.sql`:

```sql
ALTER TABLE users DROP COLUMN IF EXISTS agent_type;
ALTER TABLE users DROP COLUMN IF EXISTS active;
ALTER TABLE incidents DROP COLUMN IF EXISTS ai_enabled;
```

### Step 3: Update the User model

In `backend/internal/models/user.go`, add two fields after `AuthSource`:

```go
// AgentType identifies AI agent accounts. NULL for all human users.
// Valid values: "postmortem", "triage", "comms", "oncall", "commander"
AgentType *string `gorm:"type:varchar(50);column:agent_type" json:"agent_type,omitempty"`

// Active controls whether this user (or agent) can operate.
// Defaults to true for all existing rows via migration.
Active bool `gorm:"not null;default:true" json:"active"`
```

### Step 4: Update the Incident model

In `backend/internal/models/incident.go`, add after the `GroupKey` field:

```go
// AIEnabled controls whether AI agents process this incident.
// Default true. Can be set false via routing rules, integration defaults, or the Properties panel.
AIEnabled bool `gorm:"not null;default:true;column:ai_enabled" json:"ai_enabled"`
```

### Step 5: Run the migration manually to verify it applies cleanly

```bash
cd backend && go run ./cmd/openincident/main.go migrate
```

Expected: migration 000025 applies, no errors.

### Step 6: Verify the schema

```bash
docker-compose exec db psql -U openincident -d openincident -c "\d users" | grep -E "agent_type|active"
docker-compose exec db psql -U openincident -d openincident -c "\d incidents" | grep ai_enabled
```

Expected: all three columns present.

### Step 7: Commit

```bash
git add backend/migrations/000025_add_agent_fields.up.sql \
        backend/migrations/000025_add_agent_fields.down.sql \
        backend/internal/models/user.go \
        backend/internal/models/incident.go
git commit -m "feat(agents): add agent_type, active to users and ai_enabled to incidents (migration 000025)"
```

---

## Task 2: UserRepository Extensions

**Files:**
- Modify: `backend/internal/repository/user_repository.go`

Add three methods to the `UserRepository` interface and implement them.

### Step 1: Write the failing test

Create `backend/internal/repository/user_repository_agent_test.go`:

```go
package repository_test

import (
    "testing"
    "github.com/openincident/openincident/internal/database"
    "github.com/openincident/openincident/internal/models"
    "github.com/openincident/openincident/internal/repository"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestUserRepository_CreateAgent(t *testing.T) {
    db := database.SetupTestDB(t)
    repo := repository.NewUserRepository(db)

    agentType := "postmortem"
    agent := &models.User{
        Email:      "agent-postmortem@system.internal",
        Name:       "Post-Mortem Agent",
        AuthSource: "ai",
        AgentType:  &agentType,
        Role:       models.UserRoleMember,
        Active:     true,
    }

    err := repo.CreateAgent(agent)
    require.NoError(t, err)
    assert.NotEmpty(t, agent.ID)

    found, err := repo.GetByEmail("agent-postmortem@system.internal")
    require.NoError(t, err)
    assert.Equal(t, "ai", found.AuthSource)
    assert.Equal(t, "postmortem", *found.AgentType)
}

func TestUserRepository_SetActive(t *testing.T) {
    db := database.SetupTestDB(t)
    repo := repository.NewUserRepository(db)

    agentType := "postmortem"
    agent := &models.User{
        Email: "agent-test@system.internal", Name: "Test Agent",
        AuthSource: "ai", AgentType: &agentType, Role: models.UserRoleMember, Active: true,
    }
    require.NoError(t, repo.CreateAgent(agent))

    require.NoError(t, repo.SetActive(agent.ID, false))
    found, err := repo.GetByID(agent.ID)
    require.NoError(t, err)
    assert.False(t, found.Active)

    require.NoError(t, repo.SetActive(agent.ID, true))
    found, err = repo.GetByID(agent.ID)
    require.NoError(t, err)
    assert.True(t, found.Active)
}

func TestUserRepository_ListAgents(t *testing.T) {
    db := database.SetupTestDB(t)
    repo := repository.NewUserRepository(db)

    agentType := "postmortem"
    require.NoError(t, repo.CreateAgent(&models.User{
        Email: "agent-pm@system.internal", Name: "Post-Mortem Agent",
        AuthSource: "ai", AgentType: &agentType, Role: models.UserRoleMember, Active: true,
    }))

    agents, err := repo.ListAgents()
    require.NoError(t, err)
    assert.Len(t, agents, 1)
    assert.Equal(t, "ai", agents[0].AuthSource)
}
```

### Step 2: Run to verify it fails

```bash
cd backend && go test ./internal/repository/... -run TestUserRepository_CreateAgent -v
```

Expected: FAIL — `CreateAgent` not defined.

### Step 3: Add methods to the interface

In `backend/internal/repository/user_repository.go`, add to the `UserRepository` interface:

```go
// CreateAgent inserts an AI agent user. No password hash is set.
CreateAgent(user *models.User) error

// SetActive enables or disables a user (used for agent on/off toggle).
SetActive(id uuid.UUID, active bool) error

// ListAgents returns all users with auth_source='ai'.
ListAgents() ([]models.User, error)
```

### Step 4: Implement the three methods

After the existing `CreateLocal` implementation:

```go
func (r *userRepository) CreateAgent(user *models.User) error {
    return r.db.Create(user).Error
}

func (r *userRepository) SetActive(id uuid.UUID, active bool) error {
    return r.db.Model(&models.User{}).
        Where("id = ?", id).
        Update("active", active).Error
}

func (r *userRepository) ListAgents() ([]models.User, error) {
    var agents []models.User
    err := r.db.Where("auth_source = ?", "ai").Order("name").Find(&agents).Error
    return agents, err
}
```

### Step 5: Run to verify tests pass

```bash
cd backend && go test ./internal/repository/... -run "TestUserRepository_CreateAgent|TestUserRepository_SetActive|TestUserRepository_ListAgents" -v
```

Expected: PASS (3 tests).

### Step 6: Commit

```bash
git add backend/internal/repository/user_repository.go \
        backend/internal/repository/user_repository_agent_test.go
git commit -m "feat(agents): add CreateAgent, SetActive, ListAgents to UserRepository"
```

---

## Task 3: Agent Seeder

**Files:**
- Create: `backend/internal/coordinator/seeder.go`
- Create: `backend/internal/coordinator/seeder_test.go`

Ensures the Post-Mortem Agent user exists on every startup. Idempotent — safe to call multiple times.

### Step 1: Write the failing test

Create `backend/internal/coordinator/seeder_test.go`:

```go
package coordinator_test

import (
    "testing"
    "github.com/openincident/openincident/internal/coordinator"
    "github.com/openincident/openincident/internal/database"
    "github.com/openincident/openincident/internal/repository"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestSeedAgents_CreatesPostMortemAgent(t *testing.T) {
    db := database.SetupTestDB(t)
    userRepo := repository.NewUserRepository(db)

    err := coordinator.SeedAgents(userRepo)
    require.NoError(t, err)

    agent, err := userRepo.GetByEmail("agent-postmortem@system.internal")
    require.NoError(t, err)
    assert.Equal(t, "Post-Mortem Agent", agent.Name)
    assert.Equal(t, "ai", agent.AuthSource)
    assert.NotNil(t, agent.AgentType)
    assert.Equal(t, "postmortem", *agent.AgentType)
    assert.True(t, agent.Active)
}

func TestSeedAgents_IsIdempotent(t *testing.T) {
    db := database.SetupTestDB(t)
    userRepo := repository.NewUserRepository(db)

    // Call twice — should not error or create duplicates
    require.NoError(t, coordinator.SeedAgents(userRepo))
    require.NoError(t, coordinator.SeedAgents(userRepo))

    agents, err := userRepo.ListAgents()
    require.NoError(t, err)
    assert.Len(t, agents, 1)
}
```

### Step 2: Run to verify it fails

```bash
cd backend && go test ./internal/coordinator/... -run TestSeedAgents -v
```

Expected: FAIL — package `coordinator` not found.

### Step 3: Implement the seeder

Create `backend/internal/coordinator/seeder.go`:

```go
package coordinator

import (
    "errors"
    "log/slog"

    "github.com/openincident/openincident/internal/models"
    "github.com/openincident/openincident/internal/repository"
    "gorm.io/gorm"
)

const postMortemAgentEmail = "agent-postmortem@system.internal"

// SeedAgents ensures all AI agent user accounts exist in the database.
// Safe to call on every startup — existing agents are left unchanged.
func SeedAgents(userRepo repository.UserRepository) error {
    if err := seedAgent(userRepo, postMortemAgentEmail, "Post-Mortem Agent", "postmortem"); err != nil {
        return err
    }
    slog.Info("AI agents seeded")
    return nil
}

func seedAgent(userRepo repository.UserRepository, email, name, agentType string) error {
    _, err := userRepo.GetByEmail(email)
    if err == nil {
        return nil // already exists
    }
    if !errors.Is(err, gorm.ErrRecordNotFound) {
        return err
    }
    at := agentType
    agent := &models.User{
        Email:      email,
        Name:       name,
        AuthSource: "ai",
        AgentType:  &at,
        Role:       models.UserRoleMember,
        Active:     true,
    }
    if err := userRepo.CreateAgent(agent); err != nil {
        return err
    }
    slog.Info("seeded AI agent", "name", name, "email", email)
    return nil
}
```

### Step 4: Run to verify tests pass

```bash
cd backend && go test ./internal/coordinator/... -run TestSeedAgents -v
```

Expected: PASS (2 tests).

### Step 5: Commit

```bash
git add backend/internal/coordinator/seeder.go \
        backend/internal/coordinator/seeder_test.go
git commit -m "feat(agents): add agent seeder — idempotent Post-Mortem Agent bootstrap"
```

---

## Task 4: AICoordinator Infrastructure

**Files:**
- Create: `backend/internal/coordinator/coordinator.go`
- Create: `backend/internal/coordinator/coordinator_test.go`

The coordinator subscribes to Redis pub/sub and routes events to agents. Checks `ai_enabled` before routing.

### Step 1: Write the failing test

Create `backend/internal/coordinator/coordinator_test.go`:

```go
package coordinator_test

import (
    "context"
    "encoding/json"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/openincident/openincident/internal/coordinator"
    "github.com/stretchr/testify/assert"
)

// fakeAgent records whether Handle was called
type fakeAgent struct{ called bool; incidentID uuid.UUID }
func (f *fakeAgent) Handle(ctx context.Context, incidentID uuid.UUID) { f.called = true; f.incidentID = incidentID }

func TestCoordinator_RoutesResolvedEvent(t *testing.T) {
    // This is an integration test — requires Redis.
    // Skip in unit test runs without REDIS_URL set.
    t.Skip("integration test — run with make test-integration")
}

func TestCoordinator_SkipsWhenAIDisabled(t *testing.T) {
    incidentID := uuid.New()
    payload, _ := json.Marshal(map[string]interface{}{
        "incident_id": incidentID.String(),
        "ai_enabled":  false,
    })

    agent := &fakeAgent{}
    c := coordinator.NewTestCoordinator(agent)
    c.RoutePayload("events:incident.resolved", payload)

    assert.False(t, agent.called, "agent should not be called when ai_enabled=false")
}

func TestCoordinator_CallsAgentWhenAIEnabled(t *testing.T) {
    incidentID := uuid.New()
    payload, _ := json.Marshal(map[string]interface{}{
        "incident_id": incidentID.String(),
        "ai_enabled":  true,
    })

    agent := &fakeAgent{}
    c := coordinator.NewTestCoordinator(agent)
    c.RoutePayload("events:incident.resolved", payload)

    assert.True(t, agent.called)
    assert.Equal(t, incidentID, agent.incidentID)
}
```

### Step 2: Run to verify it fails

```bash
cd backend && go test ./internal/coordinator/... -run TestCoordinator -v
```

Expected: FAIL — `coordinator.NewTestCoordinator` not defined.

### Step 3: Implement the coordinator

Create `backend/internal/coordinator/coordinator.go`:

```go
package coordinator

import (
    "context"
    "encoding/json"
    "log/slog"

    "github.com/google/uuid"
    "github.com/redis/go-redis/v9"
)

// Agent is the interface all AI agents implement.
type Agent interface {
    Handle(ctx context.Context, incidentID uuid.UUID)
}

// resolvedPayload is the JSON published to "events:incident.resolved".
type resolvedPayload struct {
    IncidentID string `json:"incident_id"`
    AIEnabled  bool   `json:"ai_enabled"`
}

// AICoordinator subscribes to Redis and routes incident events to agents.
type AICoordinator struct {
    redis           *redis.Client
    postMortemAgent Agent
}

// New creates an AICoordinator wired to real Redis and the provided agents.
func New(redisClient *redis.Client, postMortemAgent Agent) *AICoordinator {
    return &AICoordinator{redis: redisClient, postMortemAgent: postMortemAgent}
}

// NewTestCoordinator creates an AICoordinator for unit tests (no Redis).
func NewTestCoordinator(postMortemAgent Agent) *AICoordinator {
    return &AICoordinator{postMortemAgent: postMortemAgent}
}

// Start subscribes to the event stream and blocks until ctx is cancelled.
// Run this in a goroutine: go coordinator.Start(appCtx)
func (c *AICoordinator) Start(ctx context.Context) {
    sub := c.redis.PSubscribe(ctx, "events:incident.*")
    defer sub.Close()
    slog.Info("AI coordinator started")

    for {
        select {
        case <-ctx.Done():
            slog.Info("AI coordinator stopped")
            return
        case msg, ok := <-sub.Channel():
            if !ok {
                return
            }
            go c.RoutePayload(msg.Channel, []byte(msg.Payload))
        }
    }
}

// RoutePayload parses a raw event payload and dispatches to the correct agent.
// Exported for testability (called from Start and unit tests).
func (c *AICoordinator) RoutePayload(channel string, payload []byte) {
    switch channel {
    case "events:incident.resolved":
        var p resolvedPayload
        if err := json.Unmarshal(payload, &p); err != nil {
            slog.Error("coordinator: failed to parse resolved payload", "error", err)
            return
        }
        if !p.AIEnabled {
            slog.Info("coordinator: AI disabled for incident, skipping", "incident_id", p.IncidentID)
            return
        }
        id, err := uuid.Parse(p.IncidentID)
        if err != nil {
            slog.Error("coordinator: invalid incident_id", "incident_id", p.IncidentID)
            return
        }
        slog.Info("coordinator: routing to post-mortem agent", "incident_id", id)
        go func() {
            defer func() {
                if r := recover(); r != nil {
                    slog.Error("coordinator: post-mortem agent panicked", "recover", r)
                }
            }()
            c.postMortemAgent.Handle(context.Background(), id)
        }()
    }
}
```

### Step 4: Run to verify tests pass

```bash
cd backend && go test ./internal/coordinator/... -run "TestCoordinator_Skips|TestCoordinator_Calls" -v
```

Expected: PASS (2 tests). The integration test is skipped.

### Step 5: Commit

```bash
git add backend/internal/coordinator/coordinator.go \
        backend/internal/coordinator/coordinator_test.go
git commit -m "feat(agents): add AICoordinator with Redis pub/sub routing and ai_enabled gate"
```

---

## Task 5: Post-Mortem Agent

**Files:**
- Create: `backend/internal/coordinator/agents/postmortem.go`
- Create: `backend/internal/coordinator/agents/postmortem_test.go`

### Step 1: Write the failing test

Create `backend/internal/coordinator/agents/postmortem_test.go`:

```go
package agents_test

import (
    "context"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/openincident/openincident/internal/coordinator/agents"
    "github.com/openincident/openincident/internal/models"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// --- Fakes ---

type fakeIncidentRepo struct{ incident *models.Incident }
func (f *fakeIncidentRepo) GetByID(id uuid.UUID) (*models.Incident, error) { return f.incident, nil }

type fakePostMortemSvc struct {
    called      bool
    returnError error
    existing    bool // simulates postmortem already existing
}
func (f *fakePostMortemSvc) GetPostMortem(incidentID uuid.UUID) (*models.PostMortem, error) {
    if f.existing { return &models.PostMortem{}, nil }
    return nil, gorm.ErrRecordNotFound
}
func (f *fakePostMortemSvc) GeneratePostMortem(incident *models.Incident, templateID *uuid.UUID, createdByID string) (*models.PostMortem, error) {
    f.called = true
    return &models.PostMortem{ID: uuid.New()}, f.returnError
}

type fakeAISvc struct{ enabled bool }
func (f *fakeAISvc) IsEnabled() bool { return f.enabled }

// --- Tests ---

func TestPostMortemAgent_SkipsWhenAIDisabled(t *testing.T) {
    agent := agents.NewPostMortemAgent(agents.PostMortemAgentDeps{
        AgentUserID:   uuid.New(),
        AISvc:         &fakeAISvc{enabled: false},
        IncidentRepo:  &fakeIncidentRepo{incident: &models.Incident{ID: uuid.New(), Status: "resolved"}},
        PostMortemSvc: &fakePostMortemSvc{},
        WaitDuration:  0,
    })
    pmSvc := &fakePostMortemSvc{}
    agent.SetPostMortemSvc(pmSvc)
    agent.Handle(context.Background(), uuid.New())
    assert.False(t, pmSvc.called, "should not generate when AI disabled")
}

func TestPostMortemAgent_SkipsWhenPostMortemExists(t *testing.T) {
    pmSvc := &fakePostMortemSvc{existing: true}
    agent := agents.NewPostMortemAgent(agents.PostMortemAgentDeps{
        AgentUserID:   uuid.New(),
        AISvc:         &fakeAISvc{enabled: true},
        IncidentRepo:  &fakeIncidentRepo{incident: &models.Incident{ID: uuid.New(), Status: "resolved"}},
        PostMortemSvc: pmSvc,
        WaitDuration:  0,
    })
    agent.Handle(context.Background(), uuid.New())
    assert.False(t, pmSvc.called, "should not overwrite existing post-mortem")
}

func TestPostMortemAgent_GeneratesWhenAllConditionsMet(t *testing.T) {
    resolvedAt := time.Now().Add(-30 * time.Minute)
    triggeredAt := resolvedAt.Add(-30 * time.Minute)
    incident := &models.Incident{
        ID: uuid.New(), Status: "resolved",
        TriggeredAt: triggeredAt, ResolvedAt: &resolvedAt,
    }
    pmSvc := &fakePostMortemSvc{existing: false}
    agent := agents.NewPostMortemAgent(agents.PostMortemAgentDeps{
        AgentUserID:   uuid.New(),
        AISvc:         &fakeAISvc{enabled: true},
        IncidentRepo:  &fakeIncidentRepo{incident: incident},
        PostMortemSvc: pmSvc,
        WaitDuration:  0,
    })
    agent.Handle(context.Background(), incident.ID)
    assert.True(t, pmSvc.called, "should generate post-mortem")
}

func TestPostMortemAgent_SkipsShortIncidents(t *testing.T) {
    resolvedAt := time.Now().Add(-1 * time.Minute)
    triggeredAt := resolvedAt.Add(-2 * time.Minute) // only 2 minutes — below 5m threshold
    incident := &models.Incident{
        ID: uuid.New(), Status: "resolved",
        TriggeredAt: triggeredAt, ResolvedAt: &resolvedAt,
    }
    pmSvc := &fakePostMortemSvc{existing: false}
    agent := agents.NewPostMortemAgent(agents.PostMortemAgentDeps{
        AgentUserID:   uuid.New(),
        AISvc:         &fakeAISvc{enabled: true},
        IncidentRepo:  &fakeIncidentRepo{incident: incident},
        PostMortemSvc: pmSvc,
        WaitDuration:  0,
    })
    agent.Handle(context.Background(), incident.ID)
    assert.False(t, pmSvc.called, "should skip incidents under 5 minutes")
}
```

### Step 2: Run to verify it fails

```bash
cd backend && go test ./internal/coordinator/agents/... -run TestPostMortemAgent -v
```

Expected: FAIL — package not found.

### Step 3: Implement the agent

Create `backend/internal/coordinator/agents/postmortem.go`:

```go
package agents

import (
    "context"
    "errors"
    "log/slog"
    "time"

    "github.com/google/uuid"
    "github.com/openincident/openincident/internal/models"
    "github.com/openincident/openincident/internal/repository"
    "github.com/openincident/openincident/internal/services"
    "gorm.io/gorm"
)

const minIncidentDuration = 5 * time.Minute
const defaultWaitDuration = 60 * time.Second

// PostMortemAgentDeps holds all dependencies for the Post-Mortem Agent.
// WaitDuration is configurable so tests can set it to 0.
type PostMortemAgentDeps struct {
    AgentUserID   uuid.UUID
    AISvc         interface{ IsEnabled() bool }
    IncidentRepo  interface{ GetByID(uuid.UUID) (*models.Incident, error) }
    PostMortemSvc interface {
        GetPostMortem(uuid.UUID) (*models.PostMortem, error)
        GeneratePostMortem(*models.Incident, *uuid.UUID, string) (*models.PostMortem, error)
    }
    TimelineRepo  repository.TimelineRepository // may be nil in tests
    SlackSvc      services.ChatService          // may be nil
    TeamsSvc      interface {                   // may be nil
        PostToConversation(conversationID string, msg services.Message) error
        SendDirectMessage(username string, msg services.Message) error
    }
    MultiChat    services.ChatService // for DM to commander; may be nil
    UserRepo     repository.UserRepository // for commander lookup; may be nil
    FrontendURL  string
    WaitDuration time.Duration
}

// PostMortemAgent auto-drafts a post-mortem when an incident resolves.
type PostMortemAgent struct {
    deps PostMortemAgentDeps
}

func NewPostMortemAgent(deps PostMortemAgentDeps) *PostMortemAgent {
    if deps.WaitDuration == 0 {
        deps.WaitDuration = defaultWaitDuration
    }
    return &PostMortemAgent{deps: deps}
}

// SetPostMortemSvc replaces the post-mortem service (used in tests).
func (a *PostMortemAgent) SetPostMortemSvc(svc interface {
    GetPostMortem(uuid.UUID) (*models.PostMortem, error)
    GeneratePostMortem(*models.Incident, *uuid.UUID, string) (*models.PostMortem, error)
}) {
    a.deps.PostMortemSvc = svc
}

// Handle is called by the AICoordinator when an incident resolves.
func (a *PostMortemAgent) Handle(ctx context.Context, incidentID uuid.UUID) {
    // Step 1: wait for timeline to settle
    if a.deps.WaitDuration > 0 {
        select {
        case <-ctx.Done():
            return
        case <-time.After(a.deps.WaitDuration):
        }
    }

    // Step 2: precondition — AI configured
    if !a.deps.AISvc.IsEnabled() {
        slog.Info("post-mortem agent: AI not configured, skipping", "incident_id", incidentID)
        return
    }

    // Step 3: fetch incident
    incident, err := a.deps.IncidentRepo.GetByID(incidentID)
    if err != nil {
        slog.Error("post-mortem agent: failed to fetch incident", "incident_id", incidentID, "error", err)
        return
    }

    // Step 4: precondition — incident duration >= 5 minutes
    if incident.ResolvedAt != nil {
        duration := incident.ResolvedAt.Sub(incident.TriggeredAt)
        if duration < minIncidentDuration {
            slog.Info("post-mortem agent: incident too short, skipping",
                "incident_id", incidentID, "duration", duration)
            return
        }
    }

    // Step 5: precondition — no existing post-mortem
    _, err = a.deps.PostMortemSvc.GetPostMortem(incidentID)
    if err == nil {
        slog.Info("post-mortem agent: post-mortem already exists, skipping", "incident_id", incidentID)
        return
    }
    if !errors.Is(err, gorm.ErrRecordNotFound) {
        slog.Error("post-mortem agent: failed to check existing post-mortem", "error", err)
        return
    }

    // Step 6: generate
    pm, err := a.deps.PostMortemSvc.GeneratePostMortem(incident, nil, a.deps.AgentUserID.String())
    if err != nil {
        slog.Error("post-mortem agent: generation failed", "incident_id", incidentID, "error", err)
        return
    }
    slog.Info("post-mortem agent: draft created", "incident_id", incidentID, "postmortem_id", pm.ID)

    // Step 7: write timeline entry
    a.writeTimelineEntry(incident, pm)

    // Step 8: notify
    a.notify(incident, pm)
}

func (a *PostMortemAgent) writeTimelineEntry(incident *models.Incident, pm *models.PostMortem) {
    if a.deps.TimelineRepo == nil {
        return
    }
    entry := &models.TimelineEntry{
        IncidentID: incident.ID,
        Timestamp:  time.Now().UTC(),
        Type:       "postmortem_drafted",
        ActorType:  "ai_agent",
        ActorID:    a.deps.AgentUserID.String(),
        Content: models.JSONB{
            "postmortem_id": pm.ID.String(),
            "agent":         "postmortem",
        },
    }
    if err := a.deps.TimelineRepo.Create(entry); err != nil {
        slog.Warn("post-mortem agent: failed to write timeline entry", "error", err)
    }
}

func (a *PostMortemAgent) notify(incident *models.Incident, pm *models.PostMortem) {
    link := a.deps.FrontendURL + "/incidents/" + incident.ID.String() + "/postmortem"
    channelMsg := services.Message{
        Text: "🤖 *Post-Mortem Agent*\n\n" +
            "I've drafted a post-mortem for INC-" + itoa(incident.IncidentNumber) + " · _" + incident.Title + "_\n\n" +
            "Severity: " + string(incident.Severity) + "\n" +
            "Review and edit → " + link,
    }

    // Post to Slack incident channel
    if a.deps.SlackSvc != nil && incident.SlackChannelID != "" {
        if _, err := a.deps.SlackSvc.PostMessage(incident.SlackChannelID, channelMsg); err != nil {
            slog.Warn("post-mortem agent: slack channel post failed", "error", err)
        }
    }

    // Post to Teams incident conversation
    if a.deps.TeamsSvc != nil && incident.TeamsConversationID != nil {
        if err := a.deps.TeamsSvc.PostToConversation(*incident.TeamsConversationID, channelMsg); err != nil {
            slog.Warn("post-mortem agent: teams channel post failed", "error", err)
        }
    }

    // DM the incident commander
    if a.deps.MultiChat != nil && a.deps.UserRepo != nil && incident.CommanderID != nil {
        commander, err := a.deps.UserRepo.GetByID(*incident.CommanderID)
        if err != nil {
            slog.Warn("post-mortem agent: commander not found for DM", "commander_id", incident.CommanderID)
            return
        }
        dmMsg := services.Message{
            Text: "🤖 *Post-Mortem Agent*\n\n" +
                "As incident commander for INC-" + itoa(incident.IncidentNumber) +
                ", I've drafted a post-mortem for your review.\n\n" +
                "Review → " + link,
        }
        if err := a.deps.MultiChat.SendDirectMessage(commander.Email, dmMsg); err != nil {
            slog.Warn("post-mortem agent: DM to commander failed", "error", err)
        }
    }
}

func itoa(n int) string {
    return fmt.Sprintf("%d", n)
}
```

Add `"fmt"` to imports.

### Step 4: Run to verify tests pass

```bash
cd backend && go test ./internal/coordinator/agents/... -run TestPostMortemAgent -v
```

Expected: PASS (4 tests).

### Step 5: Commit

```bash
git add backend/internal/coordinator/agents/postmortem.go \
        backend/internal/coordinator/agents/postmortem_test.go
git commit -m "feat(agents): implement Post-Mortem Agent with preconditions, generation, notifications"
```

---

## Task 6: Publish Redis Event in ResolveIncident

**Files:**
- Modify: `backend/internal/services/incident_service.go`

Publish `events:incident.resolved` after a successful resolve. Both `ResolveIncident` (used by Slack/Teams bots) and the `UpdateIncident` path (used by the API handler when status → resolved) must publish.

### Step 1: Read the existing ResolveIncident implementation

It's at line ~1131 in `incident_service.go`. It currently:
1. Gets the incident
2. Calls `incidentRepo.UpdateStatus(id, "resolved")`
3. Creates a timeline entry
4. Fires `postStatusUpdateToTeams` in a goroutine

### Step 2: Locate the UpdateIncident resolved-status path

Search for the block at line ~740 that handles `IncidentStatusResolved`. It updates `incident.ResolvedAt` and calls `PostStatusUpdateToSlack`.

### Step 3: Add publishResolved helper

After the `recoverAsyncPanic` helper function in `incident_service.go`, add:

```go
// publishResolved publishes an "incident.resolved" event to Redis so the
// AICoordinator can trigger the Post-Mortem Agent. Best-effort: log and continue on error.
func publishResolved(incidentID uuid.UUID, aiEnabled bool) {
    payload, err := json.Marshal(map[string]interface{}{
        "incident_id": incidentID.String(),
        "ai_enabled":  aiEnabled,
    })
    if err != nil {
        slog.Error("publishResolved: marshal failed", "error", err)
        return
    }
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    if err := redisClient.Publish(ctx, "events:incident.resolved", payload).Err(); err != nil {
        slog.Warn("publishResolved: redis publish failed (agent will not run)", "error", err)
    }
}
```

Add `"encoding/json"` to imports if not present. The `redisClient` is `redis.Client` from `github.com/openincident/openincident/internal/redis` — import it as `appredis "github.com/openincident/openincident/internal/redis"` and use `appredis.Client`.

### Step 4: Call publishResolved in ResolveIncident

In `ResolveIncident` (line ~1131), after the `createTimelineEntry` call:

```go
// Publish event for AICoordinator (Post-Mortem Agent et al.)
go publishResolved(id, incident.AIEnabled)
```

### Step 5: Call publishResolved in UpdateIncident

In the `UpdateIncident` method, find the block that handles `IncidentStatusResolved` (around line ~740). After setting `incident.ResolvedAt`, add:

```go
go publishResolved(incident.ID, incident.AIEnabled)
```

### Step 6: Build to verify no compile errors

```bash
cd backend && go build ./...
```

Expected: builds cleanly.

### Step 7: Commit

```bash
git add backend/internal/services/incident_service.go
git commit -m "feat(agents): publish events:incident.resolved to Redis on resolution"
```

---

## Task 7: Wire AICoordinator in serve.go

**Files:**
- Modify: `backend/cmd/openincident/commands/serve.go`

### Step 1: Import the coordinator package

Add to imports in `serve.go`:

```go
"github.com/openincident/openincident/internal/coordinator"
"github.com/openincident/openincident/internal/coordinator/agents"
appredis "github.com/openincident/openincident/internal/redis"
```

### Step 2: Seed agents after migrations

After the `database.RunMigrations(...)` call and before `api.SetupRoutes(...)`:

```go
// Seed AI agent user accounts (idempotent).
if err := coordinator.SeedAgents(userRepo); err != nil {
    slog.Error("failed to seed AI agents", "error", err)
    // Non-fatal — agents just won't run until next restart when DB is healthy.
}
```

### Step 3: Build and wire the PostMortemAgent

After `worker.StartAll(...)`, add:

```go
// Start the AI coordinator — routes incident events to AI agents.
{
    // Look up the Post-Mortem Agent's UUID from the DB.
    pmAgentUser, err := userRepo.GetByEmail("agent-postmortem@system.internal")
    if err != nil {
        slog.Warn("post-mortem agent user not found, AI coordinator not starting", "error", err)
    } else if pmAgentUser.Active {
        // Build per-provider chat services for the agent (channel posts, not DMs).
        var agentSlackSvc services.ChatService
        if cfg.SlackBotToken != "" {
            agentSlackSvc, _ = services.NewSlackService(cfg.SlackBotToken)
        }

        // Build multi-chat for DMs (reuses existing pattern from worker.StartAll).
        activeChatSvcs := make([]services.ChatService, 0, 2)
        if agentSlackSvc != nil { activeChatSvcs = append(activeChatSvcs, agentSlackSvc) }
        if teamsSvc != nil { activeChatSvcs = append(activeChatSvcs, teamsSvc) }
        var multiChat services.ChatService
        if len(activeChatSvcs) == 1 { multiChat = activeChatSvcs[0] }
        if len(activeChatSvcs) == 2 { multiChat = services.NewMultiChatService(activeChatSvcs...) }

        pmRepo := repository.NewPostMortemRepository(database.DB)
        pmTemplateRepo := repository.NewPostMortemTemplateRepository(database.DB)
        incidentRepo := repository.NewIncidentRepository(database.DB)
        timelineRepo := repository.NewTimelineRepository(database.DB)
        aiSvc := services.NewAIService(cfg.OpenAIAPIKey, cfg.OpenAIModel, 1000, 3000)

        incidentSvc := services.NewIncidentService(incidentRepo, timelineRepo,
            repository.NewAlertRepository(database.DB), agentSlackSvc, teamsSvc, aiSvc)
        pmSvc := services.NewPostMortemService(pmRepo, pmTemplateRepo, incidentSvc, aiSvc)

        pmAgent := agents.NewPostMortemAgent(agents.PostMortemAgentDeps{
            AgentUserID:   pmAgentUser.ID,
            AISvc:         aiSvc,
            IncidentRepo:  incidentRepo,
            PostMortemSvc: pmSvc,
            TimelineRepo:  timelineRepo,
            SlackSvc:      agentSlackSvc,
            TeamsSvc:      teamsSvc,
            MultiChat:     multiChat,
            UserRepo:      userRepo,
            FrontendURL:   cfg.FrontendURL,
        })

        coord := coordinator.New(appredis.Client, pmAgent)
        go coord.Start(appCtx)
        slog.Info("AI coordinator started")
    }
}
```

### Step 4: Ensure FrontendURL is in config

Check `backend/internal/config/config.go` for `FrontendURL`. If missing, add:

```go
FrontendURL string `env:"FRONTEND_URL" envDefault:"http://localhost:3000"`
```

### Step 5: Build to verify

```bash
cd backend && go build ./...
```

Expected: builds cleanly.

### Step 6: Commit

```bash
git add backend/cmd/openincident/commands/serve.go \
        backend/internal/config/config.go
git commit -m "feat(agents): wire AICoordinator and PostMortemAgent into serve.go startup"
```

---

## Task 8: Agent Management API

**Files:**
- Create: `backend/internal/api/handlers/agents.go`
- Create: `backend/internal/api/handlers/agents_test.go`
- Modify: `backend/internal/api/routes.go`

### Step 1: Write the failing test

Create `backend/internal/api/handlers/agents_test.go`:

```go
package handlers_test

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/openincident/openincident/internal/api/handlers"
    "github.com/openincident/openincident/internal/database"
    "github.com/openincident/openincident/internal/models"
    "github.com/openincident/openincident/internal/repository"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestAgentsHandler_List(t *testing.T) {
    db := database.SetupTestDB(t)
    userRepo := repository.NewUserRepository(db)
    agentType := "postmortem"
    require.NoError(t, userRepo.CreateAgent(&models.User{
        Email: "agent-pm@system.internal", Name: "Post-Mortem Agent",
        AuthSource: "ai", AgentType: &agentType, Role: models.UserRoleMember, Active: true,
    }))

    gin.SetMode(gin.TestMode)
    r := gin.New()
    h := handlers.NewAgentsHandler(userRepo)
    r.GET("/api/v1/agents", h.List)

    w := httptest.NewRecorder()
    r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/agents", nil))

    assert.Equal(t, http.StatusOK, w.Code)
    var body []map[string]interface{}
    require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
    assert.Len(t, body, 1)
    assert.Equal(t, "Post-Mortem Agent", body[0]["name"])
}

func TestAgentsHandler_SetStatus(t *testing.T) {
    db := database.SetupTestDB(t)
    userRepo := repository.NewUserRepository(db)
    agentType := "postmortem"
    agent := &models.User{
        Email: "agent-pm2@system.internal", Name: "Post-Mortem Agent",
        AuthSource: "ai", AgentType: &agentType, Role: models.UserRoleMember, Active: true,
    }
    require.NoError(t, userRepo.CreateAgent(agent))

    gin.SetMode(gin.TestMode)
    r := gin.New()
    h := handlers.NewAgentsHandler(userRepo)
    r.PATCH("/api/v1/agents/:id/status", h.SetStatus)

    body := `{"active": false}`
    w := httptest.NewRecorder()
    req := httptest.NewRequest("PATCH", "/api/v1/agents/"+agent.ID.String()+"/status",
        strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    r.ServeHTTP(w, req)

    assert.Equal(t, http.StatusOK, w.Code)
    found, err := userRepo.GetByID(agent.ID)
    require.NoError(t, err)
    assert.False(t, found.Active)
}
```

### Step 2: Run to verify it fails

```bash
cd backend && go test ./internal/api/handlers/... -run TestAgentsHandler -v
```

Expected: FAIL — `handlers.NewAgentsHandler` not defined.

### Step 3: Implement the handler

Create `backend/internal/api/handlers/agents.go`:

```go
package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/openincident/openincident/internal/repository"
)

// AgentsHandler manages AI agent CRUD endpoints.
type AgentsHandler struct {
    userRepo repository.UserRepository
}

func NewAgentsHandler(userRepo repository.UserRepository) *AgentsHandler {
    return &AgentsHandler{userRepo: userRepo}
}

// List returns all AI agent users.
// GET /api/v1/agents
func (h *AgentsHandler) List(c *gin.Context) {
    agents, err := h.userRepo.ListAgents()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list agents"})
        return
    }
    c.JSON(http.StatusOK, agents)
}

// SetStatus enables or disables an AI agent.
// PATCH /api/v1/agents/:id/status
// Body: { "active": true|false }
func (h *AgentsHandler) SetStatus(c *gin.Context) {
    id, err := uuid.Parse(c.Param("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
        return
    }

    var body struct {
        Active bool `json:"active"`
    }
    if err := c.ShouldBindJSON(&body); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
        return
    }

    agent, err := h.userRepo.GetByID(id)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
        return
    }
    if agent.AuthSource != "ai" {
        c.JSON(http.StatusForbidden, gin.H{"error": "not an AI agent"})
        return
    }

    if err := h.userRepo.SetActive(id, body.Active); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update agent status"})
        return
    }

    agent.Active = body.Active
    c.JSON(http.StatusOK, agent)
}
```

### Step 4: Register routes in routes.go

In `backend/internal/api/routes.go`, after the settings users routes, add:

```go
// Agent management (AI agents)
agentsHandler := handlers.NewAgentsHandler(userRepo)
v1.GET("/agents", agentsHandler.List)
v1.PATCH("/agents/:id/status", agentsHandler.SetStatus)
```

Note: `userRepo` is already initialized earlier in `SetupRoutes`.

### Step 5: Run to verify tests pass

```bash
cd backend && go test ./internal/api/handlers/... -run TestAgentsHandler -v
```

Expected: PASS (2 tests).

### Step 6: Build

```bash
cd backend && go build ./...
```

### Step 7: Commit

```bash
git add backend/internal/api/handlers/agents.go \
        backend/internal/api/handlers/agents_test.go \
        backend/internal/api/routes.go
git commit -m "feat(agents): add GET /api/v1/agents and PATCH /api/v1/agents/:id/status"
```

---

## Task 9: Routing Rule ai_enabled Propagation

**Files:**
- Modify: `backend/internal/services/routing_engine.go`
- Modify: `backend/internal/services/incident_service.go` (CreateIncidentFromAlertWithGrouping)
- Modify: `backend/internal/api/handlers/dto/incident_request.go`

When a routing rule fires, the `ai_enabled` key in its `actions` JSONB is propagated to the newly created incident. The default (when not set in the rule) is `true`.

### Step 1: Update the routing engine to extract ai_enabled

In `backend/internal/services/routing_engine.go`, find the `Apply` method or wherever routing actions are read. Add extraction of `ai_enabled`:

```go
// extractAIEnabled reads the ai_enabled key from routing rule actions.
// Returns true if the key is absent (opt-out requires explicit false).
func extractAIEnabled(actions models.JSONB) bool {
    if v, ok := actions["ai_enabled"]; ok {
        if b, ok := v.(bool); ok {
            return b
        }
    }
    return true
}
```

### Step 2: Update CreateIncidentFromAlertWithGrouping to accept ai_enabled

In `incident_service.go`, find `CreateIncidentFromAlertWithGrouping`. Add `aiEnabled bool` to the parameters it passes when creating the incident:

```go
// When creating the incident from an alert, pass through the ai_enabled flag
// resolved from the routing rule.
incident := &models.Incident{
    ...
    AIEnabled: aiEnabled,
}
```

The routing engine should pass `extractAIEnabled(rule.Actions)` when calling this.

### Step 3: Update incident DTOs for manual creation

In `backend/internal/api/handlers/dto/incident_request.go`, add to `CreateIncidentRequest`:

```go
// AIEnabled controls whether AI agents process this incident. Defaults to true.
AIEnabled *bool `json:"ai_enabled"`
```

Add to `UpdateIncidentRequest`:

```go
// AIEnabled can toggle AI agent processing on/off after creation.
AIEnabled *bool `json:"ai_enabled"`
```

### Step 4: Update the incident handler to pass ai_enabled

In `backend/internal/api/handlers/incidents.go`, find the `CreateIncident` handler. When building `CreateIncidentParams`, pass `ai_enabled`:

```go
aiEnabled := true
if req.AIEnabled != nil {
    aiEnabled = *req.AIEnabled
}
// Pass aiEnabled to incidentRepo or CreateIncident service call
```

And in `UpdateIncident`, when building `UpdateIncidentParams`, include `AIEnabled`.

### Step 5: Build

```bash
cd backend && go build ./...
```

### Step 6: Commit

```bash
git add backend/internal/services/routing_engine.go \
        backend/internal/services/incident_service.go \
        backend/internal/api/handlers/dto/incident_request.go \
        backend/internal/api/handlers/incidents.go
git commit -m "feat(agents): propagate ai_enabled from routing rules to incidents; expose in create/update DTOs"
```

---

## Task 10: Frontend — Types and CreateIncidentModal Toggle

**Files:**
- Modify: `frontend/src/api/types.ts`
- Modify: `frontend/src/components/incidents/CreateIncidentModal.tsx`

### Step 1: Add ai_enabled to frontend types

In `frontend/src/api/types.ts`:

Add to `Incident` interface (after `ai_summary_generated_at`):
```typescript
// AI agent processing flag (v1.0+)
ai_enabled: boolean
```

Add to `CreateIncidentRequest` interface:
```typescript
ai_enabled?: boolean
```

Add to `UpdateIncidentRequest` interface:
```typescript
ai_enabled?: boolean
```

Add new type for agent management:
```typescript
export interface AIAgent {
  id: string
  name: string
  email: string
  agent_type: string
  active: boolean
  updated_at: string
}
```

### Step 2: Add API functions

In `frontend/src/api/client.ts`, add:

```typescript
export async function listAgents(): Promise<AIAgent[]> {
  return apiClient.get<AIAgent[]>('/api/v1/agents')
}

export async function setAgentStatus(id: string, active: boolean): Promise<AIAgent> {
  return apiClient.patch<AIAgent>(`/api/v1/agents/${id}/status`, { active })
}
```

### Step 3: Add the toggle to CreateIncidentModal

In `frontend/src/components/incidents/CreateIncidentModal.tsx`:

Add state:
```typescript
const [aiEnabled, setAiEnabled] = useState(true)
```

Add to the form JSX (before the submit button):
```tsx
{/* AI Agents toggle */}
<div className="flex items-center justify-between py-2 border-t border-border mt-2">
  <div>
    <p className="text-sm font-medium text-text-primary">AI Agents</p>
    <p className="text-xs text-text-tertiary">Auto-draft post-mortem when resolved</p>
  </div>
  <button
    type="button"
    role="switch"
    aria-checked={aiEnabled}
    onClick={() => setAiEnabled(v => !v)}
    className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${
      aiEnabled ? 'bg-brand-primary' : 'bg-gray-200'
    }`}
  >
    <span className={`inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform ${
      aiEnabled ? 'translate-x-4' : 'translate-x-0.5'
    }`} />
  </button>
</div>
```

Include `ai_enabled: aiEnabled` in the `createIncident(...)` call payload.

### Step 4: TypeScript check

```bash
cd frontend && npx tsc --noEmit 2>&1 | grep -v PostMortemPanel
```

Expected: no errors.

### Step 5: Commit

```bash
git add frontend/src/api/types.ts \
        frontend/src/api/client.ts \
        frontend/src/components/incidents/CreateIncidentModal.tsx
git commit -m "feat(agents): add ai_enabled to Incident types, CreateIncidentModal toggle, agent API functions"
```

---

## Task 11: Frontend — PropertiesPanel ai_enabled Toggle

**Files:**
- Modify: `frontend/src/components/layout/PropertiesPanel.tsx`

### Step 1: Locate the PropertiesPanel

Read `frontend/src/components/layout/PropertiesPanel.tsx`. Find where severity and commander fields are rendered.

### Step 2: Add the AI Agents toggle row

Find the last property row in the panel and add after it:

```tsx
{/* AI Agents */}
<div className="flex items-center justify-between py-2 border-t border-border">
  <span className="text-xs text-text-tertiary uppercase tracking-wide">AI Agents</span>
  <button
    type="button"
    role="switch"
    aria-checked={incident.ai_enabled}
    onClick={async () => {
      try {
        await updateIncident(incident.id, { ai_enabled: !incident.ai_enabled })
        onIncidentUpdated?.()
      } catch {
        // toast error if toast system is available
      }
    }}
    className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${
      incident.ai_enabled ? 'bg-brand-primary' : 'bg-gray-200'
    }`}
    title={incident.ai_enabled ? 'AI agents enabled — click to disable' : 'AI agents disabled — click to enable'}
  >
    <span className={`inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform ${
      incident.ai_enabled ? 'translate-x-4' : 'translate-x-0.5'
    }`} />
  </button>
</div>
```

Import `updateIncident` from the API client if not already imported.

### Step 3: TypeScript check

```bash
cd frontend && npx tsc --noEmit 2>&1 | grep -v PostMortemPanel
```

Expected: no errors.

### Step 4: Commit

```bash
git add frontend/src/components/layout/PropertiesPanel.tsx
git commit -m "feat(agents): add AI Agents on/off toggle to incident Properties panel"
```

---

## Task 12: Frontend — Settings/Users AI Team Members Section

**Files:**
- Modify: `frontend/src/pages/SettingsUsersPage.tsx`

### Step 1: Add agent data fetching

In `SettingsUsersPage`, add state and effect:

```typescript
const [agents, setAgents] = useState<AIAgent[]>([])

useEffect(() => {
  listAgents().then(setAgents).catch(() => {})
}, [])
```

Import `listAgents`, `setAgentStatus`, `AIAgent` from the API.

### Step 2: Add a toggleAgent handler

```typescript
async function handleToggleAgent(agent: AIAgent) {
  try {
    await setAgentStatus(agent.id, !agent.active)
    setAgents(prev => prev.map(a => a.id === agent.id ? { ...a, active: !a.active } : a))
  } catch {
    setError('Failed to update agent status')
  }
}
```

### Step 3: Add the AI Team Members section JSX

After the human users table and before the closing `</div>`, add:

```tsx
{/* AI Team Members */}
{agents.length > 0 && (
  <div className="mt-8">
    <h3 className="text-sm font-semibold text-text-primary mb-3">AI Team Members</h3>
    <div className="border border-border rounded-lg overflow-hidden">
      <table className="w-full text-sm">
        <thead>
          <tr className="bg-gray-50 border-b border-border">
            <th className="text-left px-4 py-2.5 font-medium text-text-secondary">Agent</th>
            <th className="text-left px-4 py-2.5 font-medium text-text-secondary">Domain</th>
            <th className="text-right px-4 py-2.5 font-medium text-text-secondary">Status</th>
          </tr>
        </thead>
        <tbody>
          {agents.map(agent => (
            <tr key={agent.id} className="border-b border-border last:border-0">
              <td className="px-4 py-3">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium text-text-primary">{agent.name}</span>
                  <span className="text-xs px-1.5 py-0.5 rounded bg-violet-100 text-violet-700 font-medium">
                    🤖 AI
                  </span>
                </div>
                <p className="text-xs text-text-tertiary mt-0.5">{agent.email}</p>
              </td>
              <td className="px-4 py-3 text-text-secondary capitalize">
                {agent.agent_type === 'postmortem' ? 'Post-mortems' : agent.agent_type}
              </td>
              <td className="px-4 py-3 text-right">
                <button
                  type="button"
                  role="switch"
                  aria-checked={agent.active}
                  onClick={() => handleToggleAgent(agent)}
                  className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${
                    agent.active ? 'bg-brand-primary' : 'bg-gray-200'
                  }`}
                  title={agent.active ? 'Enabled — click to disable' : 'Disabled — click to enable'}
                >
                  <span className={`inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform ${
                    agent.active ? 'translate-x-4' : 'translate-x-0.5'
                  }`} />
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  </div>
)}
```

### Step 4: TypeScript check

```bash
cd frontend && npx tsc --noEmit 2>&1 | grep -v PostMortemPanel
```

Expected: no errors.

### Step 5: Commit

```bash
git add frontend/src/pages/SettingsUsersPage.tsx
git commit -m "feat(agents): add AI Team Members section to Settings/Users with active toggle"
```

---

## Task 13: End-to-End Smoke Test

Manual test sequence to verify the full flow works before opening a PR.

### Step 1: Start services

```bash
make dev
```

### Step 2: Verify agent is seeded

```bash
docker-compose exec db psql -U openincident -d openincident \
  -c "SELECT id, name, auth_source, agent_type, active FROM users WHERE auth_source = 'ai';"
```

Expected: 1 row — Post-Mortem Agent.

### Step 3: Verify agents endpoint

```bash
curl -s http://localhost:8080/api/v1/agents | jq .
```

Expected: JSON array with one agent.

### Step 4: Create and resolve a test incident

```bash
# Create
curl -s -X POST http://localhost:8080/api/v1/incidents \
  -H "Content-Type: application/json" \
  -d '{"title": "Test AI agent incident", "severity": "high"}' | jq .id

# Wait a moment, then resolve (replace <id> with actual ID)
curl -s -X PATCH http://localhost:8080/api/v1/incidents/<id> \
  -H "Content-Type: application/json" \
  -d '{"status": "resolved"}'
```

### Step 5: Wait 70 seconds, then check for post-mortem

```bash
curl -s http://localhost:8080/api/v1/incidents/<id>/postmortem | jq .status
```

Expected: `"draft"` — created by the Post-Mortem Agent.

### Step 6: Check backend logs

```bash
docker-compose logs backend | grep "post-mortem agent"
```

Expected: "post-mortem agent: draft created" log line.

### Step 7: Verify agent is togglable

Go to `http://localhost:3000/settings/users` — AI Team Members section should show Post-Mortem Agent with an active toggle. Toggle it off, resolve another incident, verify no post-mortem is created.

### Step 8: Open PR

```bash
git push origin feature/ai-postmortem-agent
gh pr create --title "feat(agents): Post-Mortem Agent — auto-draft on resolve" \
  --body "Implements the AI Post-Mortem Agent as designed in docs/plans/2026-02-28-ai-postmortem-agent-design.md"
```
