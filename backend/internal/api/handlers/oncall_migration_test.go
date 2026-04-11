package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/config"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/FluidifyAI/Regen/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var migrationTestDBCounter uint64

// ── Test DB setup ─────────────────────────────────────────────────────────────

// setupMigrationTestDB creates an isolated in-memory SQLite database with all
// tables required by the migration handler: users, local_sessions, schedules,
// schedule_layers, schedule_participants, escalation_policies, escalation_tiers.
func setupMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	n := atomic.AddUint64(&migrationTestDBCounter, 1)
	dsn := fmt.Sprintf("file:migtest%d?mode=memory&cache=shared", n)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })

	stmts := []string{
		`CREATE TABLE users (
			id               TEXT PRIMARY KEY,
			email            TEXT NOT NULL UNIQUE,
			name             TEXT NOT NULL DEFAULT '',
			saml_subject     TEXT UNIQUE,
			saml_idp_issuer  TEXT NOT NULL DEFAULT '',
			password_hash    TEXT,
			auth_source      TEXT NOT NULL DEFAULT 'local',
			agent_type       TEXT,
			active           INTEGER NOT NULL DEFAULT 1,
			slack_user_id    TEXT,
			teams_user_id    TEXT,
			role             TEXT NOT NULL DEFAULT 'member',
			last_login_at    DATETIME,
			created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE local_sessions (
			token       TEXT PRIMARY KEY,
			user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at  DATETIME NOT NULL,
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE schedules (
			id                   TEXT PRIMARY KEY,
			name                 TEXT NOT NULL,
			description          TEXT NOT NULL DEFAULT '',
			timezone             TEXT NOT NULL DEFAULT 'UTC',
			notification_channel TEXT NOT NULL DEFAULT '',
			created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE schedule_layers (
			id                     TEXT PRIMARY KEY,
			schedule_id            TEXT NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
			name                   TEXT NOT NULL,
			order_index            INTEGER NOT NULL DEFAULT 0,
			rotation_type          TEXT NOT NULL DEFAULT 'weekly',
			rotation_start         DATETIME NOT NULL,
			shift_duration_seconds INTEGER NOT NULL DEFAULT 604800,
			created_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE schedule_participants (
			id          TEXT PRIMARY KEY,
			layer_id    TEXT NOT NULL REFERENCES schedule_layers(id) ON DELETE CASCADE,
			user_name   TEXT NOT NULL,
			order_index INTEGER NOT NULL DEFAULT 0,
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE escalation_policies (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			enabled     INTEGER NOT NULL DEFAULT 1,
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE escalation_tiers (
			id               TEXT PRIMARY KEY,
			policy_id        TEXT NOT NULL REFERENCES escalation_policies(id) ON DELETE CASCADE,
			tier_index       INTEGER NOT NULL,
			timeout_seconds  INTEGER NOT NULL DEFAULT 300,
			target_type      TEXT NOT NULL DEFAULT 'users',
			schedule_id      TEXT REFERENCES schedules(id) ON DELETE SET NULL,
			user_names       TEXT NOT NULL DEFAULT '[]',
			created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, stmt := range stmts {
		_, err := sqlDB.Exec(stmt)
		require.NoError(t, err, "creating table: %s", stmt[:40])
	}

	return db
}

// testCfg returns a minimal dev config (not production so backendBaseURL returns localhost:8080).
func testCfg() *config.Config {
	return &config.Config{
		Environment:  "development",
		OSSUserLimit: 100,
	}
}

// ── Mock OnCall server ────────────────────────────────────────────────────────

type oncallMockResponses struct {
	usersBody        []byte
	schedulesBody    []byte
	shiftsBody       []byte
	chainsBody       []byte
	stepsBody        []byte
	integrationsBody []byte
}

// newOnCallMockServer starts an httptest.Server that serves minimal valid OnCall
// API responses. The caller can override individual endpoints via the returned
// oncallMockResponses (ignored after server start — use newOnCallMockServerFn for
// dynamic control). Call srv.Close() when done.
func newOnCallMockServer(t *testing.T, r oncallMockResponses) *httptest.Server {
	t.Helper()
	if r.usersBody == nil {
		r.usersBody = emptyPage()
	}
	if r.schedulesBody == nil {
		r.schedulesBody = emptyPage()
	}
	if r.shiftsBody == nil {
		r.shiftsBody = emptyPage()
	}
	if r.chainsBody == nil {
		r.chainsBody = emptyPage()
	}
	if r.stepsBody == nil {
		r.stepsBody = emptyPage()
	}
	if r.integrationsBody == nil {
		r.integrationsBody = emptyPage()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/users/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(r.usersBody)
	})
	mux.HandleFunc("/api/v1/schedules/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(r.schedulesBody)
	})
	mux.HandleFunc("/api/v1/on_call_shifts/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(r.shiftsBody)
	})
	mux.HandleFunc("/api/v1/escalation_chains/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(r.chainsBody)
	})
	mux.HandleFunc("/api/v1/escalation_policies/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(r.stepsBody)
	})
	mux.HandleFunc("/api/v1/integrations/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(r.integrationsBody)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// emptyPage returns a JSON-encoded empty OnCall list page.
func emptyPage() []byte {
	b, _ := json.Marshal(map[string]interface{}{
		"results":  []interface{}{},
		"next":     nil,
		"previous": nil,
		"count":    0,
	})
	return b
}

// oncallUserPage returns a JSON-encoded page with the given users and no next cursor.
func oncallUserPage(users []map[string]interface{}) []byte {
	b, _ := json.Marshal(map[string]interface{}{
		"results": users,
		"next":    nil,
	})
	return b
}

// oncallIntegrationPage returns a JSON-encoded page with the given integrations.
func oncallIntegrationPage(integrations []map[string]interface{}) []byte {
	b, _ := json.Marshal(map[string]interface{}{
		"results": integrations,
		"next":    nil,
	})
	return b
}

// ── Handler helper ────────────────────────────────────────────────────────────

func buildMigrationRouter(db *gorm.DB, cfg *config.Config) *gin.Engine {
	gin.SetMode(gin.TestMode)

	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewLocalSessionRepository(db)
	localAuth := services.NewLocalAuthService(userRepo, sessionRepo)
	scheduleRepo := repository.NewScheduleRepository(db)
	escalationRepo := repository.NewEscalationPolicyRepository(db)

	r := gin.New()
	r.POST("/api/v1/migrations/oncall/preview",
		PreviewOnCallMigration(localAuth, scheduleRepo, escalationRepo, cfg))
	r.POST("/api/v1/migrations/oncall/import",
		ImportOnCallMigration(localAuth, scheduleRepo, escalationRepo, cfg))
	return r
}

func postJSON(router *gin.Engine, path string, body interface{}) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// ── Preview endpoint ──────────────────────────────────────────────────────────

func TestPreviewOnCallMigration_MissingBody(t *testing.T) {
	db := setupMigrationTestDB(t)
	r := buildMigrationRouter(db, testCfg())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/migrations/oncall/preview", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPreviewOnCallMigration_MissingToken(t *testing.T) {
	db := setupMigrationTestDB(t)
	r := buildMigrationRouter(db, testCfg())

	w := postJSON(r, "/api/v1/migrations/oncall/preview", map[string]string{
		"oncall_url": "http://example.com",
		// api_token missing
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPreviewOnCallMigration_InvalidToken_401(t *testing.T) {
	// OnCall server returns 401 on Ping.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)

	db := setupMigrationTestDB(t)
	r := buildMigrationRouter(db, testCfg())

	w := postJSON(r, "/api/v1/migrations/oncall/preview", map[string]string{
		"oncall_url": srv.URL,
		"api_token":  "wrong",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	errObj := resp["error"].(map[string]interface{})
	assert.Contains(t, errObj["message"], "invalid API token")
}

func TestPreviewOnCallMigration_EmptyOnCall(t *testing.T) {
	// OnCall has no data — preview should return empty counts, no error.
	srv := newOnCallMockServer(t, oncallMockResponses{})

	db := setupMigrationTestDB(t)
	r := buildMigrationRouter(db, testCfg())

	w := postJSON(r, "/api/v1/migrations/oncall/preview", map[string]string{
		"oncall_url": srv.URL,
		"api_token":  "tok",
	})
	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.EqualValues(t, 0, body["users"].(map[string]interface{})["count"])
	assert.EqualValues(t, 0, body["schedules"].(map[string]interface{})["count"])
	assert.EqualValues(t, 0, body["escalation_policies"].(map[string]interface{})["count"])
}

func TestPreviewOnCallMigration_WithUsers(t *testing.T) {
	usersBody := oncallUserPage([]map[string]interface{}{
		{"id": "u1", "email": "alice@example.com", "username": "alice", "name": "Alice", "role": "admin"},
		{"id": "u2", "email": "bob@example.com", "username": "bob", "name": "Bob", "role": "viewer"},
	})
	srv := newOnCallMockServer(t, oncallMockResponses{usersBody: usersBody})

	db := setupMigrationTestDB(t)
	r := buildMigrationRouter(db, testCfg())

	w := postJSON(r, "/api/v1/migrations/oncall/preview", map[string]string{
		"oncall_url": srv.URL,
		"api_token":  "tok",
	})
	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.EqualValues(t, 2, body["users"].(map[string]interface{})["count"])
}

func TestPreviewOnCallMigration_ExistingUserAppearsAsConflict(t *testing.T) {
	usersBody := oncallUserPage([]map[string]interface{}{
		{"id": "u1", "email": "alice@example.com", "username": "alice", "name": "Alice", "role": "admin"},
	})
	srv := newOnCallMockServer(t, oncallMockResponses{usersBody: usersBody})

	db := setupMigrationTestDB(t)

	// Pre-insert alice so she already exists in Regen.
	sqlDB, _ := db.DB()
	_, err := sqlDB.Exec(`INSERT INTO users (id, email, name, auth_source, role) VALUES ('00000000-0000-0000-0000-000000000001', 'alice@example.com', 'Alice', 'local', 'admin')`)
	require.NoError(t, err)

	r := buildMigrationRouter(db, testCfg())
	w := postJSON(r, "/api/v1/migrations/oncall/preview", map[string]string{
		"oncall_url": srv.URL,
		"api_token":  "tok",
	})
	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	// User should be in conflicts, not in users.
	assert.EqualValues(t, 0, body["users"].(map[string]interface{})["count"])
	conflicts := body["conflicts"].([]interface{})
	require.Len(t, conflicts, 1)
	assert.Equal(t, "user", conflicts[0].(map[string]interface{})["type"])
}

func TestPreviewOnCallMigration_WithIntegrations(t *testing.T) {
	intBody := oncallIntegrationPage([]map[string]interface{}{
		{"id": "i1", "name": "AM", "type": "alertmanager", "link": "http://old/am"},
		{"id": "i2", "name": "GF", "type": "grafana", "link": "http://old/gf"},
	})
	srv := newOnCallMockServer(t, oncallMockResponses{integrationsBody: intBody})

	db := setupMigrationTestDB(t)
	r := buildMigrationRouter(db, testCfg())

	w := postJSON(r, "/api/v1/migrations/oncall/preview", map[string]string{
		"oncall_url": srv.URL,
		"api_token":  "tok",
	})
	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	webhooks := body["webhooks"].(map[string]interface{})
	assert.EqualValues(t, 2, webhooks["count"])

	items := webhooks["items"].([]interface{})
	firstWebhook := items[0].(map[string]interface{})
	// alertmanager → prometheus
	assert.Equal(t, "prometheus", firstWebhook["regen_source"])
	// New URL should point to our backend
	assert.Contains(t, firstWebhook["new_url"], "/api/v1/webhooks/prometheus")
}

// ── Import endpoint ───────────────────────────────────────────────────────────

func TestImportOnCallMigration_EmptyOnCall(t *testing.T) {
	srv := newOnCallMockServer(t, oncallMockResponses{})

	db := setupMigrationTestDB(t)
	r := buildMigrationRouter(db, testCfg())

	w := postJSON(r, "/api/v1/migrations/oncall/import", map[string]string{
		"oncall_url": srv.URL,
		"api_token":  "tok",
	})
	assert.Equal(t, http.StatusOK, w.Code)

	var result importResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, 0, result.Imported.Users)
	assert.Equal(t, 0, result.Imported.Schedules)
	assert.Equal(t, 0, result.Imported.EscalationPolicies)
}

func TestImportOnCallMigration_CreatesUsers(t *testing.T) {
	usersBody := oncallUserPage([]map[string]interface{}{
		{"id": "u1", "email": "alice@example.com", "username": "alice", "name": "Alice", "role": "admin"},
		{"id": "u2", "email": "bob@example.com", "username": "bob", "name": "Bob", "role": "viewer"},
	})
	srv := newOnCallMockServer(t, oncallMockResponses{usersBody: usersBody})

	db := setupMigrationTestDB(t)
	r := buildMigrationRouter(db, testCfg())

	w := postJSON(r, "/api/v1/migrations/oncall/import", map[string]string{
		"oncall_url": srv.URL,
		"api_token":  "tok",
	})
	assert.Equal(t, http.StatusOK, w.Code)

	var result importResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, 2, result.Imported.Users)
	assert.Len(t, result.SetupTokens, 2)

	// Verify users actually exist in DB.
	sqlDB, _ := db.DB()
	var count int
	sqlDB.QueryRow("SELECT COUNT(*) FROM users WHERE email IN ('alice@example.com', 'bob@example.com')").Scan(&count)
	assert.Equal(t, 2, count)
}

func TestImportOnCallMigration_UserLimitExceeded(t *testing.T) {
	usersBody := oncallUserPage([]map[string]interface{}{
		{"id": "u1", "email": "alice@example.com", "username": "alice", "name": "Alice", "role": "admin"},
		{"id": "u2", "email": "bob@example.com", "username": "bob", "name": "Bob", "role": "viewer"},
	})
	srv := newOnCallMockServer(t, oncallMockResponses{usersBody: usersBody})

	db := setupMigrationTestDB(t)

	// Set limit to 1 — importing 2 users should be rejected.
	cfg := &config.Config{Environment: "development", OSSUserLimit: 1}
	r := buildMigrationRouter(db, cfg)

	w := postJSON(r, "/api/v1/migrations/oncall/import", map[string]string{
		"oncall_url": srv.URL,
		"api_token":  "tok",
	})
	assert.Equal(t, http.StatusForbidden, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	errObj := resp["error"].(map[string]interface{})
	assert.Equal(t, "user_limit_exceeded", errObj["code"])
}

func TestImportOnCallMigration_ExistingUserSkipped(t *testing.T) {
	usersBody := oncallUserPage([]map[string]interface{}{
		{"id": "u1", "email": "alice@example.com", "username": "alice", "name": "Alice", "role": "admin"},
		{"id": "u2", "email": "bob@example.com", "username": "bob", "name": "Bob", "role": "viewer"},
	})
	srv := newOnCallMockServer(t, oncallMockResponses{usersBody: usersBody})

	db := setupMigrationTestDB(t)
	sqlDB, _ := db.DB()
	// Pre-insert alice.
	_, err := sqlDB.Exec(`INSERT INTO users (id, email, name, auth_source, role) VALUES ('00000000-0000-0000-0000-000000000002', 'alice@example.com', 'Alice', 'local', 'admin')`)
	require.NoError(t, err)

	r := buildMigrationRouter(db, testCfg())
	w := postJSON(r, "/api/v1/migrations/oncall/import", map[string]string{
		"oncall_url": srv.URL,
		"api_token":  "tok",
	})
	assert.Equal(t, http.StatusOK, w.Code)

	var result importResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	// Only bob imported; alice is a conflict.
	assert.Equal(t, 1, result.Imported.Users)
	assert.Len(t, result.Conflicts, 1)
	assert.Equal(t, "user", result.Conflicts[0].Type)
}

func TestImportOnCallMigration_WebhookMappingsReturned(t *testing.T) {
	intBody := oncallIntegrationPage([]map[string]interface{}{
		{"id": "i1", "name": "AM", "type": "alertmanager", "link": "http://old/am"},
	})
	srv := newOnCallMockServer(t, oncallMockResponses{integrationsBody: intBody})

	db := setupMigrationTestDB(t)
	r := buildMigrationRouter(db, testCfg())

	w := postJSON(r, "/api/v1/migrations/oncall/import", map[string]string{
		"oncall_url": srv.URL,
		"api_token":  "tok",
	})
	assert.Equal(t, http.StatusOK, w.Code)

	var result importResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	require.Len(t, result.Webhooks, 1)
	assert.Equal(t, "prometheus", result.Webhooks[0].RegenSource)
	assert.Equal(t, "http://old/am", result.Webhooks[0].OldURL)
	assert.Contains(t, result.Webhooks[0].NewURL, "/api/v1/webhooks/prometheus")
}

func TestImportOnCallMigration_MissingBody(t *testing.T) {
	db := setupMigrationTestDB(t)
	r := buildMigrationRouter(db, testCfg())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/migrations/oncall/import", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
