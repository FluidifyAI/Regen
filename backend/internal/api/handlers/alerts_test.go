package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/fluidify/regen/internal/models"
	"github.com/fluidify/regen/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ----------------------------------------------------------------------------
// Mock EscalationEngine (only AcknowledgeAlert used by handler)
// ----------------------------------------------------------------------------

type mockEscalationEngineForHandler struct {
	AcknowledgeAlertFn func(alertID uuid.UUID, by string, via models.AcknowledgmentVia) error
}

func (m *mockEscalationEngineForHandler) TriggerEscalation(alert *models.Alert) error { return nil }
func (m *mockEscalationEngineForHandler) TriggerIncidentEscalation(_ uuid.UUID, _ uuid.UUID) error { return nil }
func (m *mockEscalationEngineForHandler) EvaluateEscalations() error                  { return nil }
func (m *mockEscalationEngineForHandler) AcknowledgeAlert(alertID uuid.UUID, by string, via models.AcknowledgmentVia) error {
	return m.AcknowledgeAlertFn(alertID, by, via)
}
func (m *mockEscalationEngineForHandler) MarkAlertCompleted(alertID uuid.UUID) error { return nil }

// ----------------------------------------------------------------------------
// Helper: set up DB with alerts table + escalation columns
// ----------------------------------------------------------------------------

func setupAlertsHandlerDB(t *testing.T) (repository.AlertRepository, func()) {
	t.Helper()
	db, cleanup := setupIncidentTestDB(t)

	// Add escalation columns introduced in migration 000015
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.Exec(`ALTER TABLE alerts ADD COLUMN escalation_policy_id TEXT`)
	sqlDB.Exec(`ALTER TABLE alerts ADD COLUMN acknowledgment_status TEXT NOT NULL DEFAULT 'pending'`)

	return repository.NewAlertRepository(db), cleanup
}

// ----------------------------------------------------------------------------
// Tests
// ----------------------------------------------------------------------------

func TestAcknowledgeAlert_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	alertRepo, cleanup := setupAlertsHandlerDB(t)
	defer cleanup()

	alert := &models.Alert{
		ID:          uuid.New(),
		ExternalID:  "ext-1",
		Source:      "prometheus",
		Fingerprint: "fp1",
		Status:      models.AlertStatusFiring,
		Severity:    models.AlertSeverityCritical,
		Title:       "High CPU",
		StartedAt:   time.Now(),
		ReceivedAt:  time.Now(),
	}
	require.NoError(t, alertRepo.Create(alert))

	acknowledged := false
	engine := &mockEscalationEngineForHandler{
		AcknowledgeAlertFn: func(alertID uuid.UUID, by string, via models.AcknowledgmentVia) error {
			assert.Equal(t, alert.ID, alertID)
			assert.Equal(t, "user-alice", by)
			assert.Equal(t, models.AcknowledgmentViaAPI, via)
			acknowledged = true
			return nil
		},
	}

	body, _ := json.Marshal(map[string]string{
		"user_name":        "user-alice",
		"acknowledged_via": "api",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: alert.ID.String()}}

	AcknowledgeAlert(alertRepo, engine, nil, nil)(c)

	// Handler returns 204 No Content — the acknowledged_at timestamp is written
	// inside the repository transaction; we don't fabricate it in the response.
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, acknowledged)
	assert.Empty(t, w.Body.Bytes())
}

func TestAcknowledgeAlert_AlertNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	alertRepo, cleanup := setupAlertsHandlerDB(t)
	defer cleanup()

	engine := &mockEscalationEngineForHandler{
		AcknowledgeAlertFn: func(alertID uuid.UUID, by string, via models.AcknowledgmentVia) error {
			return nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	missingID := uuid.New()
	c.Request = httptest.NewRequest(http.MethodPost, "/",
		bytes.NewReader([]byte(`{"user_name":"alice","acknowledged_via":"api"}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: missingID.String()}}

	AcknowledgeAlert(alertRepo, engine, nil, nil)(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAcknowledgeAlert_InvalidUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	alertRepo, cleanup := setupAlertsHandlerDB(t)
	defer cleanup()

	engine := &mockEscalationEngineForHandler{
		AcknowledgeAlertFn: func(alertID uuid.UUID, by string, via models.AcknowledgmentVia) error {
			return nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/",
		bytes.NewReader([]byte(`{"user_name":"alice","acknowledged_via":"api"}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "not-a-uuid"}}

	AcknowledgeAlert(alertRepo, engine, nil, nil)(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAcknowledgeAlert_MissingUserName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	alertRepo, cleanup := setupAlertsHandlerDB(t)
	defer cleanup()

	engine := &mockEscalationEngineForHandler{
		AcknowledgeAlertFn: func(alertID uuid.UUID, by string, via models.AcknowledgmentVia) error {
			return nil
		},
	}

	alert := &models.Alert{
		ID:          uuid.New(),
		ExternalID:  "ext-2",
		Source:      "prometheus",
		Fingerprint: "fp2",
		Status:      models.AlertStatusFiring,
		Severity:    models.AlertSeverityWarning,
		Title:       "Disk full",
		StartedAt:   time.Now(),
		ReceivedAt:  time.Now(),
	}
	require.NoError(t, alertRepo.Create(alert))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/",
		bytes.NewReader([]byte(`{"acknowledged_via":"api"}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: alert.ID.String()}}

	AcknowledgeAlert(alertRepo, engine, nil, nil)(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAcknowledgeAlert_DefaultViaIsAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	alertRepo, cleanup := setupAlertsHandlerDB(t)
	defer cleanup()

	var capturedVia models.AcknowledgmentVia
	engine := &mockEscalationEngineForHandler{
		AcknowledgeAlertFn: func(alertID uuid.UUID, by string, via models.AcknowledgmentVia) error {
			capturedVia = via
			return nil
		},
	}

	alert := &models.Alert{
		ID:          uuid.New(),
		ExternalID:  "ext-3",
		Source:      "grafana",
		Fingerprint: "fp3",
		Status:      models.AlertStatusFiring,
		Severity:    models.AlertSeverityCritical,
		Title:       "Service down",
		StartedAt:   time.Now(),
		ReceivedAt:  time.Now(),
	}
	require.NoError(t, alertRepo.Create(alert))

	// Omit acknowledged_via — should default to "api"
	body := []byte(`{"user_name":"bob"}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: alert.ID.String()}}

	AcknowledgeAlert(alertRepo, engine, nil, nil)(c)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, models.AcknowledgmentViaAPI, capturedVia)
}
