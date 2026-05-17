package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/integrations/pagerduty"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── PagerDuty mock server ─────────────────────────────────────────────────────

type pdMockConfig struct {
	rejectAuth    bool
	usersBody     []byte
	schedules     []map[string]interface{}
	scheduleDetail map[string]interface{}
	policies      []map[string]interface{}
	policyDetail  map[string]interface{}
}

func newPDMockServer(t *testing.T, cfg pdMockConfig) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/users/me", func(w http.ResponseWriter, r *http.Request) {
		if cfg.rejectAuth {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"user": map[string]string{"id": "U1"}})
	})

	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		users := cfg.usersBody
		if users == nil {
			users, _ = json.Marshal(map[string]interface{}{"users": []interface{}{}, "more": false})
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(users)
	})

	// List endpoint: /schedules (exact)
	mux.HandleFunc("/schedules", func(w http.ResponseWriter, r *http.Request) {
		schedules := cfg.schedules
		if schedules == nil {
			schedules = []map[string]interface{}{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"schedules": schedules, "more": false})
	})
	// Detail endpoint: /schedules/<id>
	mux.HandleFunc("/schedules/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/schedules/")
		detail := cfg.scheduleDetail
		if detail == nil {
			detail = map[string]interface{}{
				"schedule": map[string]interface{}{
					"id": id, "name": "Schedule " + id,
					"time_zone": "UTC", "schedule_layers": []interface{}{},
				},
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(detail)
	})

	// List endpoint: /escalation_policies (exact)
	mux.HandleFunc("/escalation_policies", func(w http.ResponseWriter, r *http.Request) {
		policies := cfg.policies
		if policies == nil {
			policies = []map[string]interface{}{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"escalation_policies": policies, "more": false})
	})
	// Detail endpoint: /escalation_policies/<id>
	mux.HandleFunc("/escalation_policies/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/escalation_policies/")
		detail := cfg.policyDetail
		if detail == nil {
			detail = map[string]interface{}{
				"escalation_policy": map[string]interface{}{
					"id": id, "name": "Policy " + id, "escalation_rules": []interface{}{},
				},
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(detail)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// overridePDFactory replaces the package-level pdClientFactory with one that
// points the real PagerDuty client at the given base URL, and restores on cleanup.
func overridePDFactory(t *testing.T, baseURL string) {
	t.Helper()
	original := pdClientFactory
	pdClientFactory = func(apiKey string) *pagerduty.Client {
		return pagerduty.NewClientWithBaseURL(apiKey, baseURL)
	}
	t.Cleanup(func() { pdClientFactory = original })
}

// ── Router builder ────────────────────────────────────────────────────────────

func buildPDMigrationRouter(db interface{ DB() interface{ Exec(string, ...interface{}) (interface{}, error) } }) *gin.Engine {
	return nil // placeholder — real builder below
}

func buildPDRouter(scheduleRepo repository.ScheduleRepository, escalationRepo repository.EscalationPolicyRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/migrations/pagerduty/preview", PreviewPagerDutyMigration(scheduleRepo, escalationRepo))
	r.POST("/api/v1/migrations/pagerduty/import", ImportPagerDutyMigration(scheduleRepo, escalationRepo))
	return r
}

// ── Preview tests ─────────────────────────────────────────────────────────────

func TestPreviewPagerDutyMigration_MissingBody(t *testing.T) {
	db := setupMigrationTestDB(t)
	scheduleRepo := repository.NewScheduleRepository(db)
	escalationRepo := repository.NewEscalationPolicyRepository(db)
	r := buildPDRouter(scheduleRepo, escalationRepo)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/migrations/pagerduty/preview", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPreviewPagerDutyMigration_InvalidAPIKey(t *testing.T) {
	srv := newPDMockServer(t, pdMockConfig{rejectAuth: true})
	overridePDFactory(t, srv.URL)

	db := setupMigrationTestDB(t)
	r := buildPDRouter(repository.NewScheduleRepository(db), repository.NewEscalationPolicyRepository(db))

	w := postJSON(r, "/api/v1/migrations/pagerduty/preview", map[string]string{"api_key": "bad"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPreviewPagerDutyMigration_EmptyAccount(t *testing.T) {
	srv := newPDMockServer(t, pdMockConfig{})
	overridePDFactory(t, srv.URL)

	db := setupMigrationTestDB(t)
	r := buildPDRouter(repository.NewScheduleRepository(db), repository.NewEscalationPolicyRepository(db))

	w := postJSON(r, "/api/v1/migrations/pagerduty/preview", map[string]string{"api_key": "tok"})
	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.EqualValues(t, 0, len(body["schedules"].([]interface{})))
	assert.EqualValues(t, 0, len(body["policies"].([]interface{})))
}

func TestPreviewPagerDutyMigration_WithSchedule(t *testing.T) {
	srv := newPDMockServer(t, pdMockConfig{
		schedules: []map[string]interface{}{
			{"id": "S1", "name": "Primary On-Call", "time_zone": "America/New_York"},
		},
		scheduleDetail: map[string]interface{}{
			"schedule": map[string]interface{}{
				"id": "S1", "name": "Primary On-Call", "time_zone": "America/New_York",
				"schedule_layers": []map[string]interface{}{
					{
						"id": "L1", "name": "Layer 1",
						"rotation_turn_length_seconds": 604800,
						"rotation_virtual_start":       "2024-01-01T00:00:00Z",
						"users": []map[string]interface{}{
							{"user": map[string]string{"id": "U1", "name": "Alice", "email": "alice@example.com"}},
						},
					},
				},
			},
		},
	})
	overridePDFactory(t, srv.URL)

	db := setupMigrationTestDB(t)
	r := buildPDRouter(repository.NewScheduleRepository(db), repository.NewEscalationPolicyRepository(db))

	w := postJSON(r, "/api/v1/migrations/pagerduty/preview", map[string]string{"api_key": "tok"})
	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	schedules := body["schedules"].([]interface{})
	require.Len(t, schedules, 1)
	s := schedules[0].(map[string]interface{})
	assert.Equal(t, "Primary On-Call", s["name"])
	assert.EqualValues(t, 1, s["layer_count"])
	assert.EqualValues(t, 1, s["user_count"])
}

// ── Import tests ──────────────────────────────────────────────────────────────

func TestImportPagerDutyMigration_MissingBody(t *testing.T) {
	db := setupMigrationTestDB(t)
	r := buildPDRouter(repository.NewScheduleRepository(db), repository.NewEscalationPolicyRepository(db))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/migrations/pagerduty/import", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestImportPagerDutyMigration_EmptyAccount(t *testing.T) {
	srv := newPDMockServer(t, pdMockConfig{})
	overridePDFactory(t, srv.URL)

	db := setupMigrationTestDB(t)
	r := buildPDRouter(repository.NewScheduleRepository(db), repository.NewEscalationPolicyRepository(db))

	w := postJSON(r, "/api/v1/migrations/pagerduty/import", map[string]string{"api_key": "tok"})
	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	summary := body["summary"].(map[string]interface{})
	assert.EqualValues(t, 0, summary["schedules_imported"])
	assert.EqualValues(t, 0, summary["policies_imported"])
}

func TestImportPagerDutyMigration_CreatesSchedule(t *testing.T) {
	srv := newPDMockServer(t, pdMockConfig{
		schedules: []map[string]interface{}{
			{"id": "S1", "name": "Primary On-Call", "time_zone": "UTC"},
		},
		scheduleDetail: map[string]interface{}{
			"schedule": map[string]interface{}{
				"id": "S1", "name": "Primary On-Call", "time_zone": "UTC",
				"schedule_layers": []interface{}{},
			},
		},
	})
	overridePDFactory(t, srv.URL)

	db := setupMigrationTestDB(t)
	scheduleRepo := repository.NewScheduleRepository(db)
	r := buildPDRouter(scheduleRepo, repository.NewEscalationPolicyRepository(db))

	w := postJSON(r, "/api/v1/migrations/pagerduty/import", map[string]string{"api_key": "tok"})
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	summary := body["summary"].(map[string]interface{})
	assert.EqualValues(t, 1, summary["schedules_imported"])

	// Verify schedule exists in DB.
	schedules, err := scheduleRepo.GetAll()
	require.NoError(t, err)
	require.Len(t, schedules, 1)
	assert.Equal(t, "Primary On-Call", schedules[0].Name)
}

func TestImportPagerDutyMigration_InvalidAPIKey(t *testing.T) {
	srv := newPDMockServer(t, pdMockConfig{rejectAuth: true})
	overridePDFactory(t, srv.URL)

	db := setupMigrationTestDB(t)
	r := buildPDRouter(repository.NewScheduleRepository(db), repository.NewEscalationPolicyRepository(db))

	w := postJSON(r, "/api/v1/migrations/pagerduty/import", map[string]string{"api_key": "bad"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
