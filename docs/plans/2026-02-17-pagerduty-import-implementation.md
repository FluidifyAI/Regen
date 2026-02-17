# PagerDuty Import Tool — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `openincident import pagerduty --api-key=<key>` CLI command that migrates PagerDuty on-call schedules and escalation policies into OpenIncident in one shot.

**Architecture:** Refactor the existing single-file `main.go` into a cobra root with two subcommands (`serve` and `import pagerduty`). The importer lives in two new packages — `internal/integrations/pagerduty` (HTTP client + PD types) and `internal/importer` (validation, mapping, DB persistence, report). All importer code is framework-agnostic (no Gin, no HTTP server), making it fully unit-testable.

**Tech Stack:** Go 1.24, `github.com/spf13/cobra` (new dep), `net/http` for PD REST API, GORM for DB persistence, `database/sql` transactions, `encoding/json` for report output, `github.com/stretchr/testify` for tests, `net/http/httptest` for PD client mocks.

---

## Task 1: Add cobra and create thin root command

**Files:**
- Modify: `backend/go.mod` / `backend/go.sum` (via `go get`)
- Modify: `backend/cmd/openincident/main.go`
- Create: `backend/cmd/openincident/commands/` (directory)

**Step 1: Add cobra dependency**

```bash
cd backend
go get github.com/spf13/cobra@latest
```

Expected: `go get: added github.com/spf13/cobra v1.x.x`

**Step 2: Replace main.go with thin cobra root**

Replace the entire contents of `backend/cmd/openincident/main.go` with:

```go
package main

import (
	"fmt"
	"os"

	"github.com/openincident/openincident/cmd/openincident/commands"
)

func main() {
	if err := commands.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

**Step 3: Create `commands/root.go`**

Create `backend/cmd/openincident/commands/root.go`:

```go
package commands

import "github.com/spf13/cobra"

// NewRootCmd returns the cobra root command. All subcommands are attached here.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "openincident",
		Short: "OpenIncident — open-source incident management platform",
	}

	root.AddCommand(newServeCmd())
	root.AddCommand(newImportCmd())

	return root
}
```

**Step 4: Verify it compiles**

```bash
cd backend
go build ./cmd/openincident/...
```

Expected: no output (clean build)

**Step 5: Commit**

```bash
git add backend/go.mod backend/go.sum \
        backend/cmd/openincident/main.go \
        backend/cmd/openincident/commands/root.go
git commit -m "chore(cli): add cobra root command skeleton (epic-020)"
```

---

## Task 2: Extract serve subcommand

**Files:**
- Create: `backend/cmd/openincident/commands/serve.go`

The serve command contains everything currently in the old `main()` body, plus the `setupLogging` helper.

**Step 1: Create `commands/serve.go`**

Create `backend/cmd/openincident/commands/serve.go`:

```go
package commands

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/openincident/openincident/internal/api"
	"github.com/openincident/openincident/internal/config"
	"github.com/openincident/openincident/internal/database"
	"github.com/openincident/openincident/internal/metrics"
	"github.com/openincident/openincident/internal/redis"
	"github.com/openincident/openincident/internal/worker"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the OpenIncident HTTP server",
		RunE:  runServe,
	}
}

func runServe(_ *cobra.Command, _ []string) error {
	if err := godotenv.Load(); err != nil {
		slog.Warn("no .env file found")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	setupLogging(cfg.LogLevel)

	slog.Info("starting OpenIncident",
		"version", "0.1.0",
		"environment", cfg.Environment,
		"port", cfg.Port,
	)

	dbLogLevel := "info"
	if cfg.Environment == "production" {
		dbLogLevel = "warn"
	}
	dbConfig := database.Config{
		URL:          cfg.DatabaseURL,
		MaxOpenConns: cfg.DBMaxOpenConns,
		MaxIdleConns: cfg.DBMaxIdleConns,
		ConnMaxLife:  cfg.DBConnMaxLife,
		LogLevel:     dbLogLevel,
	}
	if err := database.Connect(dbConfig); err != nil {
		return err
	}
	defer database.Close()

	if err := redis.Connect(redis.Config{URL: cfg.RedisURL}); err != nil {
		return err
	}
	defer redis.Close()

	slog.Info("running database migrations...")
	if err := database.RunMigrations(database.DB, "./migrations"); err != nil {
		return err
	}

	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	api.SetupRoutes(router, database.DB, cfg)

	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	worker.StartAll(appCtx, database.DB, cfg)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		metrics.UpdateBusinessMetrics(database.DB)
		for range ticker.C {
			metrics.UpdateBusinessMetrics(database.DB)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")
	appCancel()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func setupLogging(level string) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	slog.SetDefault(slog.New(handler))
}
```

**Step 2: Build and smoke-test**

```bash
cd backend
go build ./cmd/openincident/...
./openincident --help
```

Expected output includes: `serve` and `import` subcommands listed.

```bash
./openincident serve --help
```

Expected: shows "Start the OpenIncident HTTP server"

**Step 3: Commit**

```bash
git add backend/cmd/openincident/commands/serve.go
git commit -m "refactor(cli): extract serve subcommand into commands/serve.go"
```

---

## Task 3: PagerDuty API models

**Files:**
- Create: `backend/internal/integrations/pagerduty/models.go`

These structs map JSON responses from the PagerDuty REST API v2. Use only the fields the importer needs — not a complete mirror of the PD schema.

**Step 1: Create `models.go`**

Create `backend/internal/integrations/pagerduty/models.go`:

```go
// Package pagerduty provides a minimal HTTP client and data models for the
// PagerDuty REST API v2, scoped to the fields required by the import tool.
package pagerduty

import "time"

// PDUser is a PagerDuty user (used for email → name lookup).
type PDUser struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// PDSchedule is a PagerDuty on-call schedule (list view).
type PDSchedule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	TimeZone    string `json:"time_zone"`
}

// PDScheduleDetail is a full schedule including layers and users.
type PDScheduleDetail struct {
	ID                   string            `json:"id"`
	Name                 string            `json:"name"`
	Description          string            `json:"description"`
	TimeZone             string            `json:"time_zone"`
	ScheduleLayers       []PDScheduleLayer `json:"schedule_layers"`
}

// PDScheduleLayer is one rotation layer within a schedule.
type PDScheduleLayer struct {
	ID                         string        `json:"id"`
	Name                       string        `json:"name"`
	Start                      time.Time     `json:"start"`
	RotationTurnLengthSeconds  int           `json:"rotation_turn_length_seconds"`
	RotationVirtualStart       time.Time     `json:"rotation_virtual_start"`
	Users                      []PDLayerUser `json:"users"`
}

// PDLayerUser is a user entry within a schedule layer.
type PDLayerUser struct {
	User PDUser `json:"user"`
}

// PDEscalationPolicy is a PagerDuty escalation policy (list view).
type PDEscalationPolicy struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// PDEscalationPolicyDetail is a full policy including rules and targets.
type PDEscalationPolicyDetail struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Description       string            `json:"description"`
	EscalationRules   []PDEscalationRule `json:"escalation_rules"`
}

// PDEscalationRule is one tier in an escalation policy.
type PDEscalationRule struct {
	EscalationDelayInMinutes int         `json:"escalation_delay_in_minutes"`
	Targets                  []PDTarget  `json:"targets"`
}

// PDTarget is a notification target within an escalation rule.
type PDTarget struct {
	Type string `json:"type"` // "schedule_reference", "user_reference", "team_reference"
	ID   string `json:"id"`
	Name string `json:"name"`
}

// listUsersResponse is the paginated response for GET /users.
type listUsersResponse struct {
	Users  []PDUser `json:"users"`
	More   bool     `json:"more"`
	Offset int      `json:"offset"`
	Limit  int      `json:"limit"`
}

// listSchedulesResponse is the paginated response for GET /schedules.
type listSchedulesResponse struct {
	Schedules []PDSchedule `json:"schedules"`
	More      bool         `json:"more"`
	Offset    int          `json:"offset"`
	Limit     int          `json:"limit"`
}

// scheduleDetailResponse wraps GET /schedules/:id.
type scheduleDetailResponse struct {
	Schedule PDScheduleDetail `json:"schedule"`
}

// listPoliciesResponse is the paginated response for GET /escalation_policies.
type listPoliciesResponse struct {
	EscalationPolicies []PDEscalationPolicy `json:"escalation_policies"`
	More               bool                 `json:"more"`
	Offset             int                  `json:"offset"`
	Limit              int                  `json:"limit"`
}

// policyDetailResponse wraps GET /escalation_policies/:id.
type policyDetailResponse struct {
	EscalationPolicy PDEscalationPolicyDetail `json:"escalation_policy"`
}
```

**Step 2: Compile check**

```bash
cd backend
go build ./internal/integrations/pagerduty/...
```

Expected: clean (no errors)

**Step 3: Commit**

```bash
git add backend/internal/integrations/pagerduty/models.go
git commit -m "feat(importer): add PagerDuty API v2 data models"
```

---

## Task 4: PagerDuty HTTP client

**Files:**
- Create: `backend/internal/integrations/pagerduty/client.go`

**Step 1: Create `client.go`**

Create `backend/internal/integrations/pagerduty/client.go`:

```go
package pagerduty

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	baseURL   = "https://api.pagerduty.com"
	pageLimit = 100
	maxRetries = 3
)

// Client is a minimal PagerDuty API v2 HTTP client.
type Client struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string // overridable for testing
}

// NewClient creates a PagerDuty client with the given API key.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
	}
}

// ValidateAPIKey checks the API key by calling GET /users/me.
// Returns an error if the key is invalid or the API is unreachable.
func (c *Client) ValidateAPIKey() error {
	resp, err := c.get("/users/me", nil)
	if err != nil {
		return fmt.Errorf("API key validation failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid PagerDuty API key")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API key validation returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// FetchUsers returns all PagerDuty users, paginating automatically.
// Returns a map of email → display name for user resolution.
func (c *Client) FetchUsers() (map[string]string, error) {
	emailToName := make(map[string]string)
	offset := 0
	for {
		params := map[string]string{
			"limit":  fmt.Sprintf("%d", pageLimit),
			"offset": fmt.Sprintf("%d", offset),
		}
		resp, err := c.get("/users", params)
		if err != nil {
			return nil, fmt.Errorf("fetching users (offset=%d): %w", offset, err)
		}
		var result listUsersResponse
		if err := decodeJSON(resp, &result); err != nil {
			return nil, err
		}
		for _, u := range result.Users {
			emailToName[u.Email] = u.Name
		}
		if !result.More {
			break
		}
		offset += pageLimit
	}
	return emailToName, nil
}

// FetchSchedules returns all PagerDuty schedules (list view, no layer detail).
func (c *Client) FetchSchedules() ([]PDSchedule, error) {
	var all []PDSchedule
	offset := 0
	for {
		params := map[string]string{
			"limit":  fmt.Sprintf("%d", pageLimit),
			"offset": fmt.Sprintf("%d", offset),
		}
		resp, err := c.get("/schedules", params)
		if err != nil {
			return nil, fmt.Errorf("fetching schedules (offset=%d): %w", offset, err)
		}
		var result listSchedulesResponse
		if err := decodeJSON(resp, &result); err != nil {
			return nil, err
		}
		all = append(all, result.Schedules...)
		if !result.More {
			break
		}
		offset += pageLimit
	}
	return all, nil
}

// FetchScheduleDetail returns a full schedule including layers and users.
func (c *Client) FetchScheduleDetail(id string) (*PDScheduleDetail, error) {
	params := map[string]string{"include[]": "users"}
	resp, err := c.get("/schedules/"+id, params)
	if err != nil {
		return nil, fmt.Errorf("fetching schedule %s: %w", id, err)
	}
	var result scheduleDetailResponse
	if err := decodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Schedule, nil
}

// FetchEscalationPolicies returns all PagerDuty escalation policies (list view).
func (c *Client) FetchEscalationPolicies() ([]PDEscalationPolicy, error) {
	var all []PDEscalationPolicy
	offset := 0
	for {
		params := map[string]string{
			"limit":  fmt.Sprintf("%d", pageLimit),
			"offset": fmt.Sprintf("%d", offset),
		}
		resp, err := c.get("/escalation_policies", params)
		if err != nil {
			return nil, fmt.Errorf("fetching policies (offset=%d): %w", offset, err)
		}
		var result listPoliciesResponse
		if err := decodeJSON(resp, &result); err != nil {
			return nil, err
		}
		all = append(all, result.EscalationPolicies...)
		if !result.More {
			break
		}
		offset += pageLimit
	}
	return all, nil
}

// FetchEscalationPolicyDetail returns a full policy including rules and targets.
func (c *Client) FetchEscalationPolicyDetail(id string) (*PDEscalationPolicyDetail, error) {
	params := map[string]string{"include[]": "targets"}
	resp, err := c.get("/escalation_policies/"+id, params)
	if err != nil {
		return nil, fmt.Errorf("fetching policy %s: %w", id, err)
	}
	var result policyDetailResponse
	if err := decodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.EscalationPolicy, nil
}

// get executes a GET request against the PD API with retry on 429 (rate limit).
func (c *Client) get(path string, params map[string]string) (*http.Response, error) {
	url := c.baseURL + path
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token token="+c.apiKey)
	req.Header.Set("Accept", "application/vnd.pagerduty+json;version=2")

	if len(params) > 0 {
		q := req.URL.Query()
		for k, v := range params {
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}

	backoff := time.Second
	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			time.Sleep(backoff)
			backoff *= 2
			continue
		}
		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("PagerDuty API %s returned HTTP %d: %s", path, resp.StatusCode, string(body))
		}
		return resp, nil
	}
	return nil, fmt.Errorf("PagerDuty API %s: exceeded %d retries", path, maxRetries)
}

func decodeJSON(resp *http.Response, dst any) error {
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decoding PagerDuty response: %w", err)
	}
	return nil
}
```

**Step 2: Build check**

```bash
cd backend
go build ./internal/integrations/pagerduty/...
```

Expected: clean

**Step 3: Commit**

```bash
git add backend/internal/integrations/pagerduty/client.go
git commit -m "feat(importer): implement PagerDuty API client with pagination and retry"
```

---

## Task 5: PagerDuty client tests

**Files:**
- Create: `backend/internal/integrations/pagerduty/client_test.go`

**Step 1: Write the tests**

Create `backend/internal/integrations/pagerduty/client_test.go`:

```go
package pagerduty

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeServer creates an httptest server that serves the given handler.
// The returned client is pre-configured to hit the test server.
func makeServer(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewClient("test-key")
	c.baseURL = srv.URL
	return c, srv
}

func TestValidateAPIKey_Success(t *testing.T) {
	c, _ := makeServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/users/me", r.URL.Path)
		assert.Equal(t, "Token token=test-key", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"user":{"id":"U1","email":"a@b.com","name":"Alice"}}`))
	})
	require.NoError(t, c.ValidateAPIKey())
}

func TestValidateAPIKey_Invalid(t *testing.T) {
	c, _ := makeServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	err := c.ValidateAPIKey()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid PagerDuty API key")
}

func TestFetchUsers_SinglePage(t *testing.T) {
	c, _ := makeServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/users", r.URL.Path)
		json.NewEncoder(w).Encode(listUsersResponse{
			Users: []PDUser{
				{ID: "U1", Email: "alice@example.com", Name: "Alice"},
				{ID: "U2", Email: "bob@example.com", Name: "Bob"},
			},
			More: false,
		})
	})
	result, err := c.FetchUsers()
	require.NoError(t, err)
	assert.Equal(t, "Alice", result["alice@example.com"])
	assert.Equal(t, "Bob", result["bob@example.com"])
}

func TestFetchUsers_Pagination(t *testing.T) {
	calls := 0
	c, _ := makeServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		offset := r.URL.Query().Get("offset")
		if offset == "0" || offset == "" {
			json.NewEncoder(w).Encode(listUsersResponse{
				Users:  []PDUser{{ID: "U1", Email: "alice@example.com", Name: "Alice"}},
				More:   true,
				Offset: 0,
			})
		} else {
			json.NewEncoder(w).Encode(listUsersResponse{
				Users:  []PDUser{{ID: "U2", Email: "bob@example.com", Name: "Bob"}},
				More:   false,
				Offset: 100,
			})
		}
	})
	result, err := c.FetchUsers()
	require.NoError(t, err)
	assert.Equal(t, 2, len(result))
	assert.Equal(t, 2, calls)
}

func TestFetchSchedules_SinglePage(t *testing.T) {
	c, _ := makeServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/schedules", r.URL.Path)
		json.NewEncoder(w).Encode(listSchedulesResponse{
			Schedules: []PDSchedule{{ID: "S1", Name: "Primary Rota", TimeZone: "UTC"}},
			More:      false,
		})
	})
	schedules, err := c.FetchSchedules()
	require.NoError(t, err)
	assert.Len(t, schedules, 1)
	assert.Equal(t, "Primary Rota", schedules[0].Name)
}

func TestFetchScheduleDetail(t *testing.T) {
	c, _ := makeServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/schedules/S1", r.URL.Path)
		assert.Equal(t, "users", r.URL.Query().Get("include[]"))
		json.NewEncoder(w).Encode(scheduleDetailResponse{
			Schedule: PDScheduleDetail{
				ID:       "S1",
				Name:     "Primary Rota",
				TimeZone: "UTC",
				ScheduleLayers: []PDScheduleLayer{
					{
						ID:                        "L1",
						Name:                      "Layer 1",
						RotationTurnLengthSeconds: 604800,
						Users: []PDLayerUser{
							{User: PDUser{Email: "alice@example.com", Name: "Alice"}},
						},
					},
				},
			},
		})
	})
	detail, err := c.FetchScheduleDetail("S1")
	require.NoError(t, err)
	assert.Equal(t, "Primary Rota", detail.Name)
	assert.Len(t, detail.ScheduleLayers, 1)
	assert.Len(t, detail.ScheduleLayers[0].Users, 1)
}

func TestFetchEscalationPolicies_SinglePage(t *testing.T) {
	c, _ := makeServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(listPoliciesResponse{
			EscalationPolicies: []PDEscalationPolicy{{ID: "P1", Name: "Default"}},
			More:               false,
		})
	})
	policies, err := c.FetchEscalationPolicies()
	require.NoError(t, err)
	assert.Len(t, policies, 1)
}

func TestFetchEscalationPolicyDetail(t *testing.T) {
	c, _ := makeServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/escalation_policies/P1", r.URL.Path)
		json.NewEncoder(w).Encode(policyDetailResponse{
			EscalationPolicy: PDEscalationPolicyDetail{
				ID:   "P1",
				Name: "Default",
				EscalationRules: []PDEscalationRule{
					{
						EscalationDelayInMinutes: 5,
						Targets: []PDTarget{
							{Type: "schedule_reference", ID: "S1", Name: "Primary Rota"},
						},
					},
				},
			},
		})
	})
	detail, err := c.FetchEscalationPolicyDetail("P1")
	require.NoError(t, err)
	assert.Equal(t, "Default", detail.Name)
	assert.Len(t, detail.EscalationRules, 1)
	assert.Equal(t, 5, detail.EscalationRules[0].EscalationDelayInMinutes)
}

func TestGet_RetryOn429(t *testing.T) {
	calls := 0
	c, _ := makeServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"user":{}}`))
	})
	// Override backoff to 0 to avoid slow tests
	// (the real backoff is 1s; testing that it retries is enough)
	require.NoError(t, c.ValidateAPIKey())
	assert.Equal(t, 2, calls, "should have retried once after 429")
}
```

**Step 2: Run tests**

```bash
cd backend
go test ./internal/integrations/pagerduty/... -v -count=1
```

Expected: all tests PASS

**Step 3: Commit**

```bash
git add backend/internal/integrations/pagerduty/client_test.go
git commit -m "test(importer): PagerDuty client tests with httptest mocks"
```

---

## Task 6: Import report

**Files:**
- Create: `backend/internal/importer/report.go`

**Step 1: Create `report.go`**

Create `backend/internal/importer/report.go`:

```go
// Package importer converts PagerDuty data into OpenIncident entities and
// persists them to the database.
package importer

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// ImportReport is written to disk after the import completes.
// Matches the schema defined in docs/plans/2026-02-17-pagerduty-import-design.md.
type ImportReport struct {
	ImportedAt time.Time      `json:"imported_at"`
	Summary    ReportSummary  `json:"summary"`
	Warnings   []string       `json:"warnings"`
	Errors     []string       `json:"errors"`
}

// ReportSummary holds counts of imported and skipped entities.
type ReportSummary struct {
	SchedulesFound    int `json:"schedules_found"`
	SchedulesImported int `json:"schedules_imported"`
	SchedulesSkipped  int `json:"schedules_skipped"`
	LayersImported    int `json:"layers_imported"`
	LayersSkipped     int `json:"layers_skipped"`
	PoliciesFound     int `json:"policies_found"`
	PoliciesImported  int `json:"policies_imported"`
	PoliciesSkipped   int `json:"policies_skipped"`
	TiersImported     int `json:"tiers_imported"`
}

// WriteToFile serialises the report as JSON and writes it to path.
func (r *ImportReport) WriteToFile(path string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling report: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing report to %s: %w", path, err)
	}
	return nil
}

// PrintSummary writes a human-readable summary to stdout.
func (r *ImportReport) PrintSummary() {
	fmt.Println("─── PagerDuty Import Summary ────────────────────────")
	fmt.Printf("Schedules : %d found, %d imported, %d skipped\n",
		r.Summary.SchedulesFound, r.Summary.SchedulesImported, r.Summary.SchedulesSkipped)
	fmt.Printf("  Layers  : %d imported, %d skipped\n",
		r.Summary.LayersImported, r.Summary.LayersSkipped)
	fmt.Printf("Policies  : %d found, %d imported, %d skipped\n",
		r.Summary.PoliciesFound, r.Summary.PoliciesImported, r.Summary.PoliciesSkipped)
	fmt.Printf("  Tiers   : %d imported\n", r.Summary.TiersImported)
	if len(r.Warnings) > 0 {
		fmt.Printf("\nWarnings (%d):\n", len(r.Warnings))
		for _, w := range r.Warnings {
			fmt.Printf("  ⚠  %s\n", w)
		}
	}
	if len(r.Errors) > 0 {
		fmt.Printf("\nErrors (%d):\n", len(r.Errors))
		for _, e := range r.Errors {
			fmt.Printf("  ✗  %s\n", e)
		}
	}
	fmt.Println("─────────────────────────────────────────────────────")
}
```

**Step 2: Build check**

```bash
cd backend
go build ./internal/importer/...
```

**Step 3: Commit**

```bash
git add backend/internal/importer/report.go
git commit -m "feat(importer): add ImportReport struct and file/stdout writer"
```

---

## Task 7: Validator

**Files:**
- Create: `backend/internal/importer/validator.go`
- Create: `backend/internal/importer/validator_test.go`

**Step 1: Write the failing tests first**

Create `backend/internal/importer/validator_test.go`:

```go
package importer

import (
	"testing"

	"github.com/openincident/openincident/internal/integrations/pagerduty"
	"github.com/stretchr/testify/assert"
)

func TestValidateScheduleLayer_Custom_Skipped(t *testing.T) {
	layer := pagerduty.PDScheduleLayer{
		Name:                      "Weekend Layer",
		RotationTurnLengthSeconds: 0, // custom rotation has no uniform interval
	}
	// A layer with 0 RotationTurnLengthSeconds cannot be modelled as a uniform
	// repeating interval → mark as custom and return a warning.
	result := validateScheduleLayer("Platform On-Call", 0, layer)
	assert.False(t, result.ok, "custom rotation should be skipped")
	assert.Contains(t, result.warning, "rotation_turn_length_seconds=0")
}

func TestValidateScheduleLayer_Daily_OK(t *testing.T) {
	layer := pagerduty.PDScheduleLayer{
		Name:                      "Daily Layer",
		RotationTurnLengthSeconds: 86400,
	}
	result := validateScheduleLayer("My Schedule", 0, layer)
	assert.True(t, result.ok)
	assert.Empty(t, result.warning)
}

func TestValidateScheduleLayer_Weekly_OK(t *testing.T) {
	layer := pagerduty.PDScheduleLayer{
		Name:                      "Weekly Layer",
		RotationTurnLengthSeconds: 604800,
	}
	result := validateScheduleLayer("My Schedule", 0, layer)
	assert.True(t, result.ok)
	assert.Empty(t, result.warning)
}

func TestValidateScheduleLayer_NoUsers_Warning(t *testing.T) {
	layer := pagerduty.PDScheduleLayer{
		Name:                      "Empty Layer",
		RotationTurnLengthSeconds: 604800,
		Users:                     nil,
	}
	result := validateScheduleLayer("My Schedule", 0, layer)
	assert.True(t, result.ok, "empty layer still importable, just a warning")
	assert.Contains(t, result.warning, "no users")
}

func TestValidateEscalationRule_TeamTarget_Skipped(t *testing.T) {
	rule := pagerduty.PDEscalationRule{
		EscalationDelayInMinutes: 5,
		Targets: []pagerduty.PDTarget{
			{Type: "team_reference", ID: "T1", Name: "Infra Team"},
		},
	}
	warnings := validateEscalationRule("Infra Default", 0, rule)
	assert.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "team target")
}

func TestValidateEscalationRule_Mixed_PartialWarn(t *testing.T) {
	rule := pagerduty.PDEscalationRule{
		EscalationDelayInMinutes: 10,
		Targets: []pagerduty.PDTarget{
			{Type: "schedule_reference", ID: "S1", Name: "Primary"},
			{Type: "team_reference", ID: "T1", Name: "Backend Team"},
		},
	}
	// Only the team target generates a warning; schedule target is fine.
	warnings := validateEscalationRule("My Policy", 0, rule)
	assert.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "team target")
}
```

**Step 2: Run tests — expect FAIL**

```bash
cd backend
go test ./internal/importer/... -run TestValidate -v
```

Expected: compilation error or FAIL with "undefined: validateScheduleLayer"

**Step 3: Implement `validator.go`**

Create `backend/internal/importer/validator.go`:

```go
package importer

import (
	"fmt"

	"github.com/openincident/openincident/internal/integrations/pagerduty"
)

// layerValidationResult holds the outcome of validating a single schedule layer.
type layerValidationResult struct {
	ok      bool
	warning string
}

// validateScheduleLayer checks whether a PagerDuty schedule layer can be mapped
// to an OpenIncident ScheduleLayer. Returns ok=false for custom rotations
// (RotationTurnLengthSeconds == 0) which cannot be modelled as a uniform interval.
func validateScheduleLayer(scheduleName string, layerIdx int, layer pagerduty.PDScheduleLayer) layerValidationResult {
	if layer.RotationTurnLengthSeconds == 0 {
		return layerValidationResult{
			ok: false,
			warning: fmt.Sprintf(
				"Schedule %q layer %d (%q): rotation_turn_length_seconds=0 (custom rotation) — skipped. "+
					"Create manually in OpenIncident UI.",
				scheduleName, layerIdx, layer.Name,
			),
		}
	}
	var warning string
	if len(layer.Users) == 0 {
		warning = fmt.Sprintf(
			"Schedule %q layer %d (%q): no users — layer imported with empty participant list.",
			scheduleName, layerIdx, layer.Name,
		)
	}
	return layerValidationResult{ok: true, warning: warning}
}

// validateEscalationRule checks one escalation rule's targets and returns
// a slice of warning strings (one per unsupported team target).
// An empty slice means all targets are importable.
func validateEscalationRule(policyName string, ruleIdx int, rule pagerduty.PDEscalationRule) []string {
	var warnings []string
	for _, t := range rule.Targets {
		if t.Type == "team_reference" {
			warnings = append(warnings, fmt.Sprintf(
				"Policy %q tier %d: team target %q not supported in v0.5 — skipped. "+
					"Assign users or a schedule manually.",
				policyName, ruleIdx, t.Name,
			))
		}
	}
	return warnings
}
```

**Step 4: Run tests — expect PASS**

```bash
cd backend
go test ./internal/importer/... -run TestValidate -v
```

Expected: all PASS

**Step 5: Commit**

```bash
git add backend/internal/importer/validator.go \
        backend/internal/importer/validator_test.go
git commit -m "feat(importer): add schedule layer and escalation rule validator"
```

---

## Task 8: Schedule importer

**Files:**
- Create: `backend/internal/importer/schedule_importer.go`
- Create: `backend/internal/importer/schedule_importer_test.go`

**Step 1: Write failing tests**

Create `backend/internal/importer/schedule_importer_test.go`:

```go
package importer

import (
	"testing"
	"time"

	"github.com/openincident/openincident/internal/integrations/pagerduty"
	"github.com/openincident/openincident/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- minimal mock schedule repository ---

type mockScheduleRepo struct {
	schedules    []models.Schedule
	layers       []models.ScheduleLayer
	participants []models.ScheduleParticipant
}

func (m *mockScheduleRepo) Create(s *models.Schedule) error {
	m.schedules = append(m.schedules, *s)
	return nil
}
func (m *mockScheduleRepo) GetAll() ([]models.Schedule, error) {
	return m.schedules, nil
}
func (m *mockScheduleRepo) CreateLayer(l *models.ScheduleLayer) error {
	m.layers = append(m.layers, *l)
	return nil
}
func (m *mockScheduleRepo) CreateParticipantsBulk(p []models.ScheduleParticipant) error {
	m.participants = append(m.participants, p...)
	return nil
}

// scheduleRepoWriter is the minimal interface the importer needs.
type scheduleRepoWriter interface {
	Create(s *models.Schedule) error
	GetAll() ([]models.Schedule, error)
	CreateLayer(l *models.ScheduleLayer) error
	CreateParticipantsBulk(p []models.ScheduleParticipant) error
}

// --- tests ---

func TestImportSchedule_Basic(t *testing.T) {
	repo := &mockScheduleRepo{}
	emailToName := map[string]string{"alice@example.com": "Alice"}
	report := &ImportReport{}

	pdSchedule := pagerduty.PDScheduleDetail{
		ID:       "S1",
		Name:     "Primary On-Call",
		TimeZone: "UTC",
		ScheduleLayers: []pagerduty.PDScheduleLayer{
			{
				ID:                        "L1",
				Name:                      "Layer 1",
				RotationTurnLengthSeconds: 604800, // weekly
				RotationVirtualStart:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				Users: []pagerduty.PDLayerUser{
					{User: pagerduty.PDUser{Email: "alice@example.com", Name: "Alice"}},
				},
			},
		},
	}

	err := importSchedule(repo, pdSchedule, emailToName, false, report)
	require.NoError(t, err)

	assert.Equal(t, 1, report.Summary.SchedulesImported)
	assert.Len(t, repo.schedules, 1)
	assert.Equal(t, "Primary On-Call", repo.schedules[0].Name)
	assert.Equal(t, "UTC", repo.schedules[0].Timezone)
	assert.Len(t, repo.layers, 1)
	assert.Equal(t, 604800, repo.layers[0].ShiftDurationSeconds)
	assert.Equal(t, string(models.RotationTypeWeekly), string(repo.layers[0].RotationType))
	assert.Len(t, repo.participants, 1)
	assert.Equal(t, "Alice", repo.participants[0].UserName)
}

func TestImportSchedule_EmailFallback(t *testing.T) {
	repo := &mockScheduleRepo{}
	// emailToName doesn't contain the user → fall back to email
	emailToName := map[string]string{}
	report := &ImportReport{}

	pdSchedule := pagerduty.PDScheduleDetail{
		ID:   "S2",
		Name: "Backup Rota",
		ScheduleLayers: []pagerduty.PDScheduleLayer{
			{
				RotationTurnLengthSeconds: 86400,
				Users: []pagerduty.PDLayerUser{
					{User: pagerduty.PDUser{Email: "bob@example.com", Name: "Bob"}},
				},
			},
		},
	}

	err := importSchedule(repo, pdSchedule, emailToName, false, report)
	require.NoError(t, err)
	// User name falls back to email when no lookup entry
	assert.Equal(t, "bob@example.com", repo.participants[0].UserName)
}

func TestImportSchedule_CustomLayerSkipped(t *testing.T) {
	repo := &mockScheduleRepo{}
	report := &ImportReport{}

	pdSchedule := pagerduty.PDScheduleDetail{
		ID:   "S3",
		Name: "Complex Rota",
		ScheduleLayers: []pagerduty.PDScheduleLayer{
			{Name: "Custom Layer", RotationTurnLengthSeconds: 0}, // custom → skip
		},
	}

	err := importSchedule(repo, pdSchedule, nil, false, report)
	require.NoError(t, err)

	// Schedule is imported but layer is skipped
	assert.Equal(t, 1, report.Summary.SchedulesImported)
	assert.Equal(t, 1, report.Summary.LayersSkipped)
	assert.Equal(t, 0, report.Summary.LayersImported)
	assert.Len(t, report.Warnings, 1)
}

func TestImportSchedule_ConflictSkip(t *testing.T) {
	existing := models.Schedule{Name: "Existing Rota"}
	repo := &mockScheduleRepo{schedules: []models.Schedule{existing}}
	report := &ImportReport{}

	pdSchedule := pagerduty.PDScheduleDetail{ID: "S4", Name: "Existing Rota"}

	err := importSchedule(repo, pdSchedule, nil, false /* force=false */, report)
	require.NoError(t, err)

	assert.Equal(t, 0, report.Summary.SchedulesImported)
	assert.Equal(t, 1, report.Summary.SchedulesSkipped)
	assert.Len(t, report.Warnings, 1)
	assert.Contains(t, report.Warnings[0], "name conflict")
}

func TestImportSchedule_ForceOverwrite(t *testing.T) {
	existing := models.Schedule{Name: "Existing Rota"}
	repo := &mockScheduleRepo{schedules: []models.Schedule{existing}}
	report := &ImportReport{}

	pdSchedule := pagerduty.PDScheduleDetail{
		ID:   "S5",
		Name: "Existing Rota",
		ScheduleLayers: []pagerduty.PDScheduleLayer{
			{RotationTurnLengthSeconds: 604800},
		},
	}

	err := importSchedule(repo, pdSchedule, nil, true /* force=true */, report)
	require.NoError(t, err)
	// With force, it should import even though the name exists
	assert.Equal(t, 1, report.Summary.SchedulesImported)
}
```

**Step 2: Run failing tests**

```bash
cd backend
go test ./internal/importer/... -run TestImportSchedule -v
```

Expected: FAIL — "undefined: importSchedule"

**Step 3: Implement `schedule_importer.go`**

Create `backend/internal/importer/schedule_importer.go`:

```go
package importer

import (
	"fmt"

	"github.com/openincident/openincident/internal/integrations/pagerduty"
	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/repository"
)

// ImportSchedules imports all PagerDuty schedules into OpenIncident.
// emailToName is used to resolve user names; force overwrites name conflicts.
func ImportSchedules(
	repo repository.ScheduleRepository,
	details []pagerduty.PDScheduleDetail,
	emailToName map[string]string,
	force bool,
	report *ImportReport,
) error {
	report.Summary.SchedulesFound = len(details)
	for _, d := range details {
		if err := importSchedule(repo, d, emailToName, force, report); err != nil {
			return err
		}
	}
	return nil
}

// importSchedule handles one PD schedule.
func importSchedule(
	repo scheduleRepoWriter,
	d pagerduty.PDScheduleDetail,
	emailToName map[string]string,
	force bool,
	report *ImportReport,
) error {
	// Check for name conflict
	existing, err := repo.GetAll()
	if err != nil {
		return fmt.Errorf("checking existing schedules: %w", err)
	}
	for _, e := range existing {
		if e.Name == d.Name {
			if !force {
				report.Summary.SchedulesSkipped++
				report.Warnings = append(report.Warnings, fmt.Sprintf(
					"Schedule %q: name conflict — already exists. Use --force to overwrite.",
					d.Name,
				))
				return nil
			}
			break
		}
	}

	tz := d.TimeZone
	if tz == "" {
		tz = "UTC"
	}

	schedule := &models.Schedule{
		Name:        d.Name,
		Description: d.Description,
		Timezone:    tz,
	}
	if err := repo.Create(schedule); err != nil {
		return fmt.Errorf("creating schedule %q: %w", d.Name, err)
	}
	report.Summary.SchedulesImported++

	for i, pdLayer := range d.ScheduleLayers {
		validation := validateScheduleLayer(d.Name, i, pdLayer)
		if !validation.ok {
			report.Summary.LayersSkipped++
			report.Warnings = append(report.Warnings, validation.warning)
			continue
		}
		if validation.warning != "" {
			report.Warnings = append(report.Warnings, validation.warning)
		}

		rotationType := models.RotationTypeWeekly
		if pdLayer.RotationTurnLengthSeconds == 86400 {
			rotationType = models.RotationTypeDaily
		}

		layer := &models.ScheduleLayer{
			ScheduleID:           schedule.ID,
			Name:                 pdLayer.Name,
			OrderIndex:           i,
			RotationType:         rotationType,
			RotationStart:        pdLayer.RotationVirtualStart,
			ShiftDurationSeconds: pdLayer.RotationTurnLengthSeconds,
		}
		if err := repo.CreateLayer(layer); err != nil {
			return fmt.Errorf("creating layer %q in schedule %q: %w", pdLayer.Name, d.Name, err)
		}
		report.Summary.LayersImported++

		var participants []models.ScheduleParticipant
		for j, u := range pdLayer.Users {
			userName := resolveUserName(u.User, emailToName)
			participants = append(participants, models.ScheduleParticipant{
				LayerID:    layer.ID,
				UserName:   userName,
				OrderIndex: j,
			})
		}
		if len(participants) > 0 {
			if err := repo.CreateParticipantsBulk(participants); err != nil {
				return fmt.Errorf("adding participants to layer %q: %w", pdLayer.Name, err)
			}
		}
	}
	return nil
}

// resolveUserName returns the email → name mapping if available, otherwise
// falls back to the user's email address.
func resolveUserName(u pagerduty.PDUser, emailToName map[string]string) string {
	if name, ok := emailToName[u.Email]; ok {
		return name
	}
	if u.Email != "" {
		return u.Email
	}
	return u.Name
}
```

**Step 4: Run tests — expect PASS**

```bash
cd backend
go test ./internal/importer/... -run TestImportSchedule -v
```

Expected: all PASS

**Step 5: Commit**

```bash
git add backend/internal/importer/schedule_importer.go \
        backend/internal/importer/schedule_importer_test.go
git commit -m "feat(importer): implement schedule importer with conflict resolution"
```

---

## Task 9: Policy importer

**Files:**
- Create: `backend/internal/importer/policy_importer.go`
- Create: `backend/internal/importer/policy_importer_test.go`

**Step 1: Write failing tests**

Create `backend/internal/importer/policy_importer_test.go`:

```go
package importer

import (
	"testing"

	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/integrations/pagerduty"
	"github.com/openincident/openincident/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- minimal mock policy repository ---

type mockPolicyRepo struct {
	policies []models.EscalationPolicy
	tiers    []models.EscalationTier
}

func (m *mockPolicyRepo) CreatePolicy(p *models.EscalationPolicy) error {
	m.policies = append(m.policies, *p)
	return nil
}
func (m *mockPolicyRepo) GetAllPolicies() ([]models.EscalationPolicy, error) {
	return m.policies, nil
}
func (m *mockPolicyRepo) CreateTier(t *models.EscalationTier) error {
	m.tiers = append(m.tiers, *t)
	return nil
}

type policyRepoWriter interface {
	CreatePolicy(p *models.EscalationPolicy) error
	GetAllPolicies() ([]models.EscalationPolicy, error)
	CreateTier(t *models.EscalationTier) error
}

// --- tests ---

func TestImportPolicy_Basic(t *testing.T) {
	repo := &mockPolicyRepo{}
	// scheduleNameToID: "Primary Rota" was previously imported
	scheduleNameToID := map[string]uuid.UUID{
		"Primary Rota": uuid.MustParse("00000000-0000-0000-0000-000000000001"),
	}
	report := &ImportReport{}

	pdPolicy := pagerduty.PDEscalationPolicyDetail{
		ID:   "P1",
		Name: "Default Escalation",
		EscalationRules: []pagerduty.PDEscalationRule{
			{
				EscalationDelayInMinutes: 5,
				Targets: []pagerduty.PDTarget{
					{Type: "schedule_reference", ID: "S1", Name: "Primary Rota"},
				},
			},
			{
				EscalationDelayInMinutes: 10,
				Targets: []pagerduty.PDTarget{
					{Type: "user_reference", ID: "U1", Name: "alice"},
				},
			},
		},
	}

	err := importPolicy(repo, pdPolicy, scheduleNameToID, nil, false, report)
	require.NoError(t, err)

	assert.Equal(t, 1, report.Summary.PoliciesImported)
	assert.Equal(t, 2, report.Summary.TiersImported)
	require.Len(t, repo.policies, 1)
	require.Len(t, repo.tiers, 2)

	// Tier 0: schedule target
	tier0 := repo.tiers[0]
	assert.Equal(t, 0, tier0.TierIndex)
	assert.Equal(t, 300, tier0.TimeoutSeconds) // 5 min × 60
	assert.Equal(t, models.EscalationTargetTypeSchedule, tier0.TargetType)
	assert.Equal(t, scheduleNameToID["Primary Rota"], *tier0.ScheduleID)

	// Tier 1: user target
	tier1 := repo.tiers[1]
	assert.Equal(t, 1, tier1.TierIndex)
	assert.Equal(t, 600, tier1.TimeoutSeconds) // 10 min × 60
	assert.Equal(t, models.EscalationTargetTypeUsers, tier1.TargetType)
	assert.Equal(t, []string{"alice"}, tier1.UserNames)
}

func TestImportPolicy_TeamTargetSkipped(t *testing.T) {
	repo := &mockPolicyRepo{}
	report := &ImportReport{}

	pdPolicy := pagerduty.PDEscalationPolicyDetail{
		ID:   "P2",
		Name: "Team Policy",
		EscalationRules: []pagerduty.PDEscalationRule{
			{
				EscalationDelayInMinutes: 5,
				Targets: []pagerduty.PDTarget{
					{Type: "team_reference", ID: "T1", Name: "Backend Team"},
				},
			},
		},
	}

	err := importPolicy(repo, pdPolicy, nil, nil, false, report)
	require.NoError(t, err)
	// Policy imported but tier with only team target has a warning
	assert.Equal(t, 1, report.Summary.PoliciesImported)
	assert.Len(t, report.Warnings, 1)
	assert.Contains(t, report.Warnings[0], "team target")
}

func TestImportPolicy_ConflictSkip(t *testing.T) {
	existing := models.EscalationPolicy{Name: "Existing Policy"}
	repo := &mockPolicyRepo{policies: []models.EscalationPolicy{existing}}
	report := &ImportReport{}

	pdPolicy := pagerduty.PDEscalationPolicyDetail{ID: "P3", Name: "Existing Policy"}

	err := importPolicy(repo, pdPolicy, nil, nil, false /* force */, report)
	require.NoError(t, err)
	assert.Equal(t, 0, report.Summary.PoliciesImported)
	assert.Equal(t, 1, report.Summary.PoliciesSkipped)
}

func TestImportPolicy_BothTargetType(t *testing.T) {
	scheduleID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	repo := &mockPolicyRepo{}
	report := &ImportReport{}

	pdPolicy := pagerduty.PDEscalationPolicyDetail{
		ID:   "P4",
		Name: "Both Policy",
		EscalationRules: []pagerduty.PDEscalationRule{
			{
				EscalationDelayInMinutes: 5,
				Targets: []pagerduty.PDTarget{
					{Type: "schedule_reference", Name: "Primary"},
					{Type: "user_reference", Name: "alice"},
				},
			},
		},
	}

	err := importPolicy(repo, pdPolicy,
		map[string]uuid.UUID{"Primary": scheduleID},
		nil, false, report)
	require.NoError(t, err)

	require.Len(t, repo.tiers, 1)
	assert.Equal(t, models.EscalationTargetTypeBoth, repo.tiers[0].TargetType)
}
```

**Step 2: Run failing tests**

```bash
cd backend
go test ./internal/importer/... -run TestImportPolicy -v
```

Expected: FAIL — "undefined: importPolicy"

**Step 3: Implement `policy_importer.go`**

Create `backend/internal/importer/policy_importer.go`:

```go
package importer

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/integrations/pagerduty"
	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/repository"
)

// ImportPolicies imports all PagerDuty escalation policies into OpenIncident.
// scheduleNameToID maps imported schedule names to their new OI UUIDs (required
// for resolving schedule_reference targets).
// emailToName maps PD user email → display name for user_reference targets.
func ImportPolicies(
	repo repository.EscalationPolicyRepository,
	details []pagerduty.PDEscalationPolicyDetail,
	scheduleNameToID map[string]uuid.UUID,
	emailToName map[string]string,
	force bool,
	report *ImportReport,
) error {
	report.Summary.PoliciesFound = len(details)
	for _, d := range details {
		if err := importPolicy(repo, d, scheduleNameToID, emailToName, force, report); err != nil {
			return err
		}
	}
	return nil
}

func importPolicy(
	repo policyRepoWriter,
	d pagerduty.PDEscalationPolicyDetail,
	scheduleNameToID map[string]uuid.UUID,
	emailToName map[string]string,
	force bool,
	report *ImportReport,
) error {
	existing, err := repo.GetAllPolicies()
	if err != nil {
		return fmt.Errorf("checking existing policies: %w", err)
	}
	for _, e := range existing {
		if e.Name == d.Name {
			if !force {
				report.Summary.PoliciesSkipped++
				report.Warnings = append(report.Warnings, fmt.Sprintf(
					"Policy %q: name conflict — already exists. Use --force to overwrite.",
					d.Name,
				))
				return nil
			}
			break
		}
	}

	policy := &models.EscalationPolicy{
		Name:        d.Name,
		Description: d.Description,
		Enabled:     true,
	}
	if err := repo.CreatePolicy(policy); err != nil {
		return fmt.Errorf("creating policy %q: %w", d.Name, err)
	}
	report.Summary.PoliciesImported++

	for i, rule := range d.EscalationRules {
		// Emit warnings for team targets (not importable in v0.5)
		ruleWarnings := validateEscalationRule(d.Name, i, rule)
		report.Warnings = append(report.Warnings, ruleWarnings...)

		// Determine importable target type
		var schedID *uuid.UUID
		var userNames []string
		hasSchedule := false
		hasUser := false

		for _, target := range rule.Targets {
			switch target.Type {
			case "schedule_reference":
				hasSchedule = true
				if scheduleNameToID != nil {
					if id, ok := scheduleNameToID[target.Name]; ok {
						schedID = &id
					}
				}
			case "user_reference":
				hasUser = true
				name := resolveUserName(
					pagerduty.PDUser{Name: target.Name},
					emailToName,
				)
				userNames = append(userNames, name)
			}
			// team_reference: skip, already warned above
		}

		targetType := resolveTargetType(hasSchedule, hasUser)
		if targetType == "" {
			// Tier has only team targets — still create tier (empty) with a warning
			targetType = string(models.EscalationTargetTypeSchedule)
		}

		tier := &models.EscalationTier{
			PolicyID:       policy.ID,
			TierIndex:      i,
			TimeoutSeconds: rule.EscalationDelayInMinutes * 60,
			TargetType:     models.EscalationTargetType(targetType),
			ScheduleID:     schedID,
			UserNames:      userNames,
		}
		if err := repo.CreateTier(tier); err != nil {
			return fmt.Errorf("creating tier %d for policy %q: %w", i, d.Name, err)
		}
		report.Summary.TiersImported++
	}
	return nil
}

// resolveTargetType returns the EscalationTargetType string given which
// target kinds are present in a rule.
func resolveTargetType(hasSchedule, hasUser bool) string {
	switch {
	case hasSchedule && hasUser:
		return string(models.EscalationTargetTypeBoth)
	case hasSchedule:
		return string(models.EscalationTargetTypeSchedule)
	case hasUser:
		return string(models.EscalationTargetTypeUsers)
	default:
		return ""
	}
}
```

**Step 4: Run tests — expect PASS**

```bash
cd backend
go test ./internal/importer/... -v
```

Expected: all PASS (includes validator + schedule + policy tests)

**Step 5: Commit**

```bash
git add backend/internal/importer/policy_importer.go \
        backend/internal/importer/policy_importer_test.go
git commit -m "feat(importer): implement escalation policy importer"
```

---

## Task 10: Import CLI subcommand

**Files:**
- Create: `backend/cmd/openincident/commands/import_pagerduty.go`

This command orchestrates the full import flow: validate key → fetch users → fetch+import schedules → fetch+import policies → write report.

**Step 1: Create `import_pagerduty.go`**

Create `backend/cmd/openincident/commands/import_pagerduty.go`:

```go
package commands

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/openincident/openincident/internal/config"
	"github.com/openincident/openincident/internal/database"
	"github.com/openincident/openincident/internal/importer"
	"github.com/openincident/openincident/internal/integrations/pagerduty"
	"github.com/openincident/openincident/internal/repository"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

func newImportCmd() *cobra.Command {
	importCmd := &cobra.Command{
		Use:   "import",
		Short: "Import data from external services",
	}
	importCmd.AddCommand(newImportPagerDutyCmd())
	return importCmd
}

func newImportPagerDutyCmd() *cobra.Command {
	var (
		apiKey       string
		force        bool
		dryRun       bool
		outputReport string
	)

	cmd := &cobra.Command{
		Use:   "pagerduty",
		Short: "Import on-call schedules and escalation policies from PagerDuty",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImportPagerDuty(apiKey, force, dryRun, outputReport)
		},
	}

	cmd.Flags().StringVar(&apiKey, "api-key", "", "PagerDuty API key (required)")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing records with the same name")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview import without writing to the database")
	cmd.Flags().StringVar(&outputReport, "output-report", "pagerduty_import_report.json",
		"Path to write the JSON import report")
	_ = cmd.MarkFlagRequired("api-key")

	return cmd
}

func runImportPagerDuty(apiKey string, force, dryRun bool, reportPath string) error {
	if err := godotenv.Load(); err != nil {
		slog.Warn("no .env file found")
	}

	if dryRun {
		fmt.Println("Dry-run mode is planned for a future release.")
		fmt.Println("Run without --dry-run to perform the import.")
		return nil
	}

	setupLogging("info")

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	dbLogLevel := "warn"
	if err := database.Connect(database.Config{
		URL:      cfg.DatabaseURL,
		LogLevel: dbLogLevel,
	}); err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer database.Close()

	// Step 1: Validate API key
	fmt.Println("Validating PagerDuty API key...")
	pdClient := pagerduty.NewClient(apiKey)
	if err := pdClient.ValidateAPIKey(); err != nil {
		return fmt.Errorf("invalid API key: %w", err) // exit 1
	}
	fmt.Println("✓ API key valid")

	report := &importer.ImportReport{ImportedAt: time.Now().UTC()}

	// Step 2: Fetch users → email:name map
	fmt.Println("Fetching users...")
	emailToName, err := pdClient.FetchUsers()
	if err != nil {
		return fmt.Errorf("fetching users: %w", err)
	}
	fmt.Printf("✓ %d users fetched\n", len(emailToName))

	// Step 3: Fetch all schedules, import in one DB transaction
	fmt.Println("Fetching schedules...")
	pdSchedules, err := pdClient.FetchSchedules()
	if err != nil {
		return fmt.Errorf("fetching schedule list: %w", err)
	}

	scheduleDetails := make([]pagerduty.PDScheduleDetail, 0, len(pdSchedules))
	for _, s := range pdSchedules {
		detail, err := pdClient.FetchScheduleDetail(s.ID)
		if err != nil {
			return fmt.Errorf("fetching schedule %q: %w", s.Name, err)
		}
		scheduleDetails = append(scheduleDetails, *detail)
	}
	fmt.Printf("✓ %d schedules fetched\n", len(scheduleDetails))

	schedRepo := repository.NewScheduleRepository(database.DB)
	if err := importer.ImportSchedules(schedRepo, scheduleDetails, emailToName, force, report); err != nil {
		report.Errors = append(report.Errors, err.Error())
		_ = report.WriteToFile(reportPath)
		report.PrintSummary()
		return fmt.Errorf("importing schedules: %w", err) // exit 2
	}

	// Build scheduleNameToID for policy resolution
	scheduleNameToID := make(map[string]uuid.UUID)
	allSchedules, _ := schedRepo.GetAll()
	for _, s := range allSchedules {
		scheduleNameToID[s.Name] = s.ID
	}

	// Step 4: Fetch all escalation policies, import in one DB transaction
	fmt.Println("Fetching escalation policies...")
	pdPolicies, err := pdClient.FetchEscalationPolicies()
	if err != nil {
		return fmt.Errorf("fetching policy list: %w", err)
	}

	policyDetails := make([]pagerduty.PDEscalationPolicyDetail, 0, len(pdPolicies))
	for _, p := range pdPolicies {
		detail, err := pdClient.FetchEscalationPolicyDetail(p.ID)
		if err != nil {
			return fmt.Errorf("fetching policy %q: %w", p.Name, err)
		}
		policyDetails = append(policyDetails, *detail)
	}
	fmt.Printf("✓ %d policies fetched\n", len(policyDetails))

	policyRepo := repository.NewEscalationPolicyRepository(database.DB)
	if err := importer.ImportPolicies(policyRepo, policyDetails, scheduleNameToID, emailToName, force, report); err != nil {
		report.Errors = append(report.Errors, err.Error())
		_ = report.WriteToFile(reportPath)
		report.PrintSummary()
		return fmt.Errorf("importing policies: %w", err) // exit 2
	}

	// Step 5: Write report and print summary
	if err := report.WriteToFile(reportPath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to write report: %v\n", err)
	}
	report.PrintSummary()
	fmt.Printf("\nReport written to: %s\n", reportPath)

	return nil
}
```

**Step 2: Build the full binary**

```bash
cd backend
go build ./cmd/openincident/...
```

Expected: clean build

**Step 3: Smoke-test the help output**

```bash
./openincident import pagerduty --help
```

Expected:
```
Import on-call schedules and escalation policies from PagerDuty

Usage:
  openincident import pagerduty [flags]

Flags:
      --api-key string          PagerDuty API key (required)
      --dry-run                 Preview import without writing to the database
      --force                   Overwrite existing records with the same name
  -h, --help                    help for pagerduty
      --output-report string    Path to write the JSON import report (default "pagerduty_import_report.json")
```

**Step 4: Test dry-run output**

```bash
./openincident import pagerduty --api-key=fake --dry-run
```

Expected:
```
Dry-run mode is planned for a future release.
Run without --dry-run to perform the import.
```

**Step 5: Run all tests one final time**

```bash
cd backend
go test ./... -count=1
```

Expected: all PASS

**Step 6: Commit**

```bash
git add backend/cmd/openincident/commands/import_pagerduty.go
git commit -m "feat(epic-020): implement 'openincident import pagerduty' CLI subcommand"
```

---

## Task 11: User documentation

**Files:**
- Create: `docs/PAGERDUTY_IMPORT.md`

**Step 1: Create the doc**

Create `docs/PAGERDUTY_IMPORT.md`:

```markdown
# Importing from PagerDuty

The `openincident import pagerduty` command migrates your PagerDuty on-call
schedules and escalation policies to OpenIncident in a single operation.

## Prerequisites

1. A running OpenIncident instance with DATABASE_URL configured
2. A PagerDuty API key with **Read** access (account owner or Admin role)
   - Generate at: PagerDuty → My Profile → User Settings → API Access Keys

## Step-by-Step

### 1. Review what will be imported (dry-run stub)

Dry-run is planned for a future release. In the meantime, review the
[mapping rules](#what-gets-imported) below to understand what will be migrated.

### 2. Run the import

```bash
openincident import pagerduty --api-key=<your-pd-api-key>
```

The tool will:
1. Validate your API key
2. Fetch all users (for name resolution)
3. Import all schedules
4. Import all escalation policies
5. Write `pagerduty_import_report.json` with a full summary

### 3. Review the report

```bash
cat pagerduty_import_report.json
```

Check the `warnings` array for skipped items that need manual attention.

### 4. Handle conflicts (if re-running)

If records with the same name already exist, the import skips them by default.
To overwrite:

```bash
openincident import pagerduty --api-key=<key> --force
```

### 5. Verify in the UI

- Go to **On-Call** → confirm your schedules are present
- Go to **Escalation** → confirm your policies are present

## What Gets Imported

| PagerDuty Entity | OpenIncident Entity | Notes |
|-----------------|---------------------|-------|
| Schedule | Schedule + ScheduleLayers + ScheduleParticipants | Daily/weekly rotations only |
| Escalation Policy | EscalationPolicy + EscalationTiers | Schedule + user targets |

## What Does NOT Get Imported

| Item | Reason |
|------|--------|
| Custom rotation layers | OpenIncident uses a uniform shift interval; hand-off day/time rules can't be modelled |
| Team targets in policies | No Teams model in OpenIncident v0.5 |
| Incidents & alerts | Ephemeral — not migrated by design |
| PagerDuty services | No equivalent in OpenIncident v0.5 |
| Historical escalation logs | Not migrated |

## Troubleshooting

### "invalid PagerDuty API key"
Your key is incorrect or revoked. Generate a new one in PagerDuty user settings.

### "name conflict — already exists. Use --force to overwrite."
A schedule or policy with that name already exists. Either delete the existing
record in the UI or re-run with `--force`.

### Custom rotation layers
If you see: `rotation_turn_length_seconds=0 (custom rotation) — skipped`

PagerDuty custom rotations use hand-off day + time rules that OpenIncident
cannot model as a uniform repeating interval. Recreate these layers manually:
1. Go to **On-Call** → select the schedule → **Add Layer**
2. Set rotation type, participants, and shift duration manually

### Team targets skipped
If you see: `team target 'X' not supported — skipped`

OpenIncident v0.5 does not have a Teams model. Manually add the individual
users to the tier after import via the **Escalation** UI.

## Custom Report Path

```bash
openincident import pagerduty --api-key=<key> --output-report=/tmp/my-import.json
```
```

**Step 2: Commit**

```bash
git add docs/PAGERDUTY_IMPORT.md
git commit -m "docs(epic-020): add PagerDuty import user documentation"
```

---

## Task 12: Final check and push

**Step 1: Full test suite**

```bash
cd backend
go test ./... -count=1 -race
```

Expected: all PASS, no race conditions detected

**Step 2: Build binary**

```bash
cd backend
go build ./cmd/openincident/...
./openincident --help
./openincident serve --help
./openincident import --help
./openincident import pagerduty --help
```

**Step 3: Push to remote**

```bash
git push origin main
```

---

## Exit Code Behaviour Summary

| Scenario | Exit Code |
|----------|-----------|
| Import completed (warnings ok) | `0` |
| Invalid API key or DB unreachable | `1` (cobra returns error) |
| Schedule or policy import failed mid-way | `2` (partial; report written) |
| Dry-run | `0` (stub message, no-op) |

---

## Interfaces Required by Importer (reference)

The importer functions accept minimal interfaces so they can be mocked in tests:

```go
// scheduleRepoWriter — subset of repository.ScheduleRepository
type scheduleRepoWriter interface {
    Create(s *models.Schedule) error
    GetAll() ([]models.Schedule, error)
    CreateLayer(l *models.ScheduleLayer) error
    CreateParticipantsBulk(p []models.ScheduleParticipant) error
}

// policyRepoWriter — subset of repository.EscalationPolicyRepository
type policyRepoWriter interface {
    CreatePolicy(p *models.EscalationPolicy) error
    GetAllPolicies() ([]models.EscalationPolicy, error)
    CreateTier(t *models.EscalationTier) error
}
```

These private interfaces live inside the `importer` package — no new exported types needed.
