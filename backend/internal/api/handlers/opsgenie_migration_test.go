package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/integrations/opsgenie"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Opsgenie mock server ──────────────────────────────────────────────────────

type ogMockConfig struct {
	rejectAuth bool
	schedules  []map[string]interface{}
	rotations  []map[string]interface{}
	policies   []map[string]interface{}
}

func newOGMockServer(t *testing.T, cfg ogMockConfig) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/v2/users/me", func(w http.ResponseWriter, r *http.Request) {
		if cfg.rejectAuth {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"data": map[string]string{"id": "u1"}})
	})

	mux.HandleFunc("/v2/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"data": []interface{}{}})
	})

	mux.HandleFunc("/v2/schedules", func(w http.ResponseWriter, r *http.Request) {
		schedules := cfg.schedules
		if schedules == nil {
			schedules = []map[string]interface{}{}
		}
		// Only respond to the list call (no path suffix)
		if r.URL.Path != "/v2/schedules" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"data": schedules})
	})

	// Rotation sub-resource: /v2/schedules/{id}/rotations
	mux.HandleFunc("/v2/schedules/", func(w http.ResponseWriter, r *http.Request) {
		rotations := cfg.rotations
		if rotations == nil {
			rotations = []map[string]interface{}{}
		}
		if !strings.HasSuffix(r.URL.Path, "/rotations") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"data": rotations})
	})

	mux.HandleFunc("/v1/escalations", func(w http.ResponseWriter, r *http.Request) {
		policies := cfg.policies
		if policies == nil {
			policies = []map[string]interface{}{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"data": policies})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// overrideOGFactory replaces the package-level ogClientFactory for the duration of the test.
func overrideOGFactory(t *testing.T, baseURL string) {
	t.Helper()
	original := ogClientFactory
	ogClientFactory = func(apiKey, region string) *opsgenie.Client {
		return opsgenie.NewClientWithBaseURL(apiKey, region, baseURL)
	}
	t.Cleanup(func() { ogClientFactory = original })
}

func buildOGRouter(scheduleRepo repository.ScheduleRepository, escalationRepo repository.EscalationPolicyRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/migrations/opsgenie/preview", PreviewOpsgenieMigration(scheduleRepo, escalationRepo))
	r.POST("/api/v1/migrations/opsgenie/import", ImportOpsgenieMigration(scheduleRepo, escalationRepo))
	return r
}

// ── Preview tests ─────────────────────────────────────────────────────────────

func TestPreviewOpsgenieMigration_MissingBody(t *testing.T) {
	db := setupMigrationTestDB(t)
	r := buildOGRouter(repository.NewScheduleRepository(db), repository.NewEscalationPolicyRepository(db))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/migrations/opsgenie/preview", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPreviewOpsgenieMigration_InvalidAPIKey(t *testing.T) {
	srv := newOGMockServer(t, ogMockConfig{rejectAuth: true})
	overrideOGFactory(t, srv.URL)

	db := setupMigrationTestDB(t)
	r := buildOGRouter(repository.NewScheduleRepository(db), repository.NewEscalationPolicyRepository(db))

	w := postJSON(r, "/api/v1/migrations/opsgenie/preview", map[string]string{"api_key": "bad", "region": "us"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPreviewOpsgenieMigration_EmptyAccount(t *testing.T) {
	srv := newOGMockServer(t, ogMockConfig{})
	overrideOGFactory(t, srv.URL)

	db := setupMigrationTestDB(t)
	r := buildOGRouter(repository.NewScheduleRepository(db), repository.NewEscalationPolicyRepository(db))

	w := postJSON(r, "/api/v1/migrations/opsgenie/preview", map[string]string{"api_key": "tok", "region": "us"})
	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Len(t, body["schedules"].([]interface{}), 0)
	assert.Len(t, body["policies"].([]interface{}), 0)
}

func TestPreviewOpsgenieMigration_WithSchedule(t *testing.T) {
	srv := newOGMockServer(t, ogMockConfig{
		schedules: []map[string]interface{}{
			{"id": "s1", "name": "Primary On-Call", "timezone": "America/New_York", "enabled": true},
		},
		rotations: []map[string]interface{}{
			{
				"id": "r1", "name": "Weekly", "type": "weekly", "length": 1,
				"participants": []map[string]interface{}{
					{"type": "user", "id": "u1", "username": "alice@example.com", "name": "Alice"},
				},
			},
		},
	})
	overrideOGFactory(t, srv.URL)

	db := setupMigrationTestDB(t)
	r := buildOGRouter(repository.NewScheduleRepository(db), repository.NewEscalationPolicyRepository(db))

	w := postJSON(r, "/api/v1/migrations/opsgenie/preview", map[string]string{"api_key": "tok", "region": "us"})
	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	schedules := body["schedules"].([]interface{})
	require.Len(t, schedules, 1)
	s := schedules[0].(map[string]interface{})
	assert.Equal(t, "Primary On-Call", s["name"])
	assert.EqualValues(t, 1, s["rotation_count"])
	assert.EqualValues(t, 1, s["user_count"])
}

// ── Import tests ──────────────────────────────────────────────────────────────

func TestImportOpsgenieMigration_MissingBody(t *testing.T) {
	db := setupMigrationTestDB(t)
	r := buildOGRouter(repository.NewScheduleRepository(db), repository.NewEscalationPolicyRepository(db))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/migrations/opsgenie/import", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestImportOpsgenieMigration_EmptyAccount(t *testing.T) {
	srv := newOGMockServer(t, ogMockConfig{})
	overrideOGFactory(t, srv.URL)

	db := setupMigrationTestDB(t)
	r := buildOGRouter(repository.NewScheduleRepository(db), repository.NewEscalationPolicyRepository(db))

	w := postJSON(r, "/api/v1/migrations/opsgenie/import", map[string]string{"api_key": "tok", "region": "eu"})
	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	summary := body["summary"].(map[string]interface{})
	assert.EqualValues(t, 0, summary["schedules_imported"])
	assert.EqualValues(t, 0, summary["policies_imported"])
}

func TestImportOpsgenieMigration_CreatesSchedule(t *testing.T) {
	srv := newOGMockServer(t, ogMockConfig{
		schedules: []map[string]interface{}{
			{"id": "s1", "name": "Primary On-Call", "timezone": "UTC", "enabled": true},
		},
		rotations: []map[string]interface{}{},
	})
	overrideOGFactory(t, srv.URL)

	db := setupMigrationTestDB(t)
	scheduleRepo := repository.NewScheduleRepository(db)
	r := buildOGRouter(scheduleRepo, repository.NewEscalationPolicyRepository(db))

	w := postJSON(r, "/api/v1/migrations/opsgenie/import", map[string]string{"api_key": "tok", "region": "us"})
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	summary := body["summary"].(map[string]interface{})
	assert.EqualValues(t, 1, summary["schedules_imported"])

	schedules, err := scheduleRepo.GetAll()
	require.NoError(t, err)
	require.Len(t, schedules, 1)
	assert.Equal(t, "Primary On-Call", schedules[0].Name)
}

func TestImportOpsgenieMigration_InvalidAPIKey(t *testing.T) {
	srv := newOGMockServer(t, ogMockConfig{rejectAuth: true})
	overrideOGFactory(t, srv.URL)

	db := setupMigrationTestDB(t)
	r := buildOGRouter(repository.NewScheduleRepository(db), repository.NewEscalationPolicyRepository(db))

	w := postJSON(r, "/api/v1/migrations/opsgenie/import", map[string]string{"api_key": "bad", "region": "us"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
