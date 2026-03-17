package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/fluidify/regen/internal/api/handlers"
	"github.com/fluidify/regen/internal/database"
	"github.com/fluidify/regen/internal/models"
	"github.com/fluidify/regen/internal/models/webhooks"
	"github.com/fluidify/regen/internal/repository"
	"github.com/fluidify/regen/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestPrometheusWebhookIntegration tests the full webhook flow from HTTP request to database
// This covers OI-058: Write integration tests for webhook flow
func TestPrometheusWebhookIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name                   string
		payload                *webhooks.AlertmanagerPayload
		expectedStatus         int
		expectedAlertsCreated  int
		expectedIncidents      int
		verifyAlertInDB        bool
		verifyIncidentInDB     bool
		description            string
	}{
		{
			name: "critical alert creates alert record and incident",
			payload: &webhooks.AlertmanagerPayload{
				Version:  "4",
				GroupKey: "test-group-1",
				Status:   "firing",
				Receiver: "test-receiver",
				Alerts: []webhooks.AlertmanagerAlert{
					{
						Status:       "firing",
						Labels:       map[string]string{"alertname": "HighCPU", "severity": "critical"},
						Annotations:  map[string]string{"summary": "CPU usage > 90%"},
						StartsAt:     time.Now().Add(-5 * time.Minute),
						EndsAt:       time.Time{},
						GeneratorURL: "http://prometheus:9090/graph",
						Fingerprint:  "abc123critical",
					},
				},
			},
			expectedStatus:        http.StatusOK,
			expectedAlertsCreated: 1,
			expectedIncidents:     1,
			verifyAlertInDB:       true,
			verifyIncidentInDB:    true,
			description:           "Critical severity alert should create both alert record and incident",
		},
		{
			name: "warning alert creates alert record and incident",
			payload: &webhooks.AlertmanagerPayload{
				Version:  "4",
				GroupKey: "test-group-2",
				Status:   "firing",
				Receiver: "test-receiver",
				Alerts: []webhooks.AlertmanagerAlert{
					{
						Status:       "firing",
						Labels:       map[string]string{"alertname": "HighMemory", "severity": "warning"},
						Annotations:  map[string]string{"summary": "Memory usage > 80%"},
						StartsAt:     time.Now().Add(-3 * time.Minute),
						EndsAt:       time.Time{},
						GeneratorURL: "http://prometheus:9090/graph",
						Fingerprint:  "def456warning",
					},
				},
			},
			expectedStatus:        http.StatusOK,
			expectedAlertsCreated: 1,
			expectedIncidents:     1,
			verifyAlertInDB:       true,
			verifyIncidentInDB:    true,
			description:           "Warning severity alert should create both alert record and incident",
		},
		{
			name: "info alert creates alert record but no incident",
			payload: &webhooks.AlertmanagerPayload{
				Version:  "4",
				GroupKey: "test-group-3",
				Status:   "firing",
				Receiver: "test-receiver",
				Alerts: []webhooks.AlertmanagerAlert{
					{
						Status:       "firing",
						Labels:       map[string]string{"alertname": "InfoAlert", "severity": "info"},
						Annotations:  map[string]string{"summary": "Informational message"},
						StartsAt:     time.Now().Add(-1 * time.Minute),
						EndsAt:       time.Time{},
						GeneratorURL: "http://prometheus:9090/graph",
						Fingerprint:  "ghi789info",
					},
				},
			},
			expectedStatus:        http.StatusOK,
			expectedAlertsCreated: 1,
			expectedIncidents:     0,
			verifyAlertInDB:       true,
			verifyIncidentInDB:    false,
			description:           "Info severity alert should create alert record but NOT create incident",
		},
		{
			name: "invalid payload returns 400",
			payload: &webhooks.AlertmanagerPayload{
				Version:  "4",
				GroupKey: "test-group-invalid",
				Status:   "firing",
				Receiver: "test-receiver",
				Alerts:   []webhooks.AlertmanagerAlert{}, // Empty alerts array - invalid
			},
			expectedStatus:        http.StatusBadRequest,
			expectedAlertsCreated: 0,
			expectedIncidents:     0,
			verifyAlertInDB:       false,
			verifyIncidentInDB:    false,
			description:           "Payload with no alerts should return 400 Bad Request",
		},
		{
			name: "multiple alerts in single webhook",
			payload: &webhooks.AlertmanagerPayload{
				Version:  "4",
				GroupKey: "test-group-multi",
				Status:   "firing",
				Receiver: "test-receiver",
				Alerts: []webhooks.AlertmanagerAlert{
					{
						Status:       "firing",
						Labels:       map[string]string{"alertname": "Alert1", "severity": "critical"},
						Annotations:  map[string]string{"summary": "Alert 1 summary"},
						StartsAt:     time.Now(),
						Fingerprint:  "multi-1-critical",
					},
					{
						Status:       "firing",
						Labels:       map[string]string{"alertname": "Alert2", "severity": "warning"},
						Annotations:  map[string]string{"summary": "Alert 2 summary"},
						StartsAt:     time.Now(),
						Fingerprint:  "multi-2-warning",
					},
					{
						Status:       "firing",
						Labels:       map[string]string{"alertname": "Alert3", "severity": "info"},
						Annotations:  map[string]string{"summary": "Alert 3 summary"},
						StartsAt:     time.Now(),
						Fingerprint:  "multi-3-info",
					},
				},
			},
			expectedStatus:        http.StatusOK,
			expectedAlertsCreated: 3,
			expectedIncidents:     2, // Only critical and warning create incidents
			verifyAlertInDB:       true,
			verifyIncidentInDB:    true,
			description:           "Webhook with multiple alerts should create correct number of incidents based on severity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup: Create test database and services
			db, cleanup := setupTestDB(t)
			defer cleanup()

			alertRepo := repository.NewAlertRepository(db)
			incidentRepo := repository.NewIncidentRepository(db)
			timelineRepo := repository.NewTimelineRepository(db)

			// Create incident service with all required dependencies (nil for optional ChatService)
			incidentSvc := services.NewIncidentService(incidentRepo, timelineRepo, alertRepo, nil, db)
			alertSvc := services.NewAlertService(alertRepo, incidentSvc)

			// Create test router
			router := gin.New()
			router.POST("/webhooks/prometheus", handlers.PrometheusWebhook(alertSvc))

			// Execute: Send webhook request
			payloadBytes, err := json.Marshal(tt.payload)
			require.NoError(t, err, "Failed to marshal test payload")

			req := httptest.NewRequest(http.MethodPost, "/webhooks/prometheus", bytes.NewReader(payloadBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Assert: Verify HTTP response
			assert.Equal(t, tt.expectedStatus, w.Code, tt.description)

			if tt.expectedStatus == http.StatusOK {
				// Verify response body
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err, "Failed to parse response body")

				received, ok := response["received"].(float64)
				assert.True(t, ok, "Response should contain 'received' count")
				assert.Equal(t, float64(len(tt.payload.Alerts)), received, "Received count should match alerts in payload")

				incidentsCreated, ok := response["incidents_created"].(float64)
				assert.True(t, ok, "Response should contain 'incidents_created' count")
				assert.Equal(t, float64(tt.expectedIncidents), incidentsCreated, "Incidents created should match expected")

				// Verify database state
				if tt.verifyAlertInDB {
					var alertCount int64
					db.Model(&models.Alert{}).Count(&alertCount)
					assert.Equal(t, int64(tt.expectedAlertsCreated), alertCount, "Alert count in database should match expected")

					// Verify alert details for single-alert tests
					if len(tt.payload.Alerts) == 1 {
						var alert models.Alert
						err := db.Where("fingerprint = ?", tt.payload.Alerts[0].Fingerprint).First(&alert).Error
						require.NoError(t, err, "Alert should exist in database")

						assert.Equal(t, "prometheus", alert.Source, "Alert source should be 'prometheus'")
						assert.Equal(t, models.AlertStatusFiring, alert.Status, "Alert status should be 'firing'")
						assert.NotEmpty(t, alert.Title, "Alert title should not be empty")
						assert.NotZero(t, alert.ReceivedAt, "ReceivedAt should be set")
					}
				}

				if tt.verifyIncidentInDB {
					var incidentCount int64
					db.Model(&models.Incident{}).Count(&incidentCount)
					assert.Equal(t, int64(tt.expectedIncidents), incidentCount, "Incident count in database should match expected")

					// Verify incident details for single-incident tests
					if tt.expectedIncidents == 1 && len(tt.payload.Alerts) == 1 {
						var incident models.Incident
						err := db.First(&incident).Error
						require.NoError(t, err, "Incident should exist in database")

						assert.NotZero(t, incident.IncidentNumber, "Incident number should be set")
						assert.Equal(t, models.IncidentStatusTriggered, incident.Status, "Incident status should be 'triggered'")
						assert.NotEmpty(t, incident.Title, "Incident title should not be empty")
						assert.NotZero(t, incident.CreatedAt, "CreatedAt should be set")
						assert.NotZero(t, incident.TriggeredAt, "TriggeredAt should be set")
						assert.Equal(t, "system", incident.CreatedByType, "CreatedByType should be 'system'")
					}
				}
			}
		})
	}
}

// TestDuplicateAlertUpdate tests that duplicate alerts update existing records
// This covers the deduplication requirement from OI-058
func TestDuplicateAlertUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup
	db, cleanup := setupTestDB(t)
	defer cleanup()

	alertRepo := repository.NewAlertRepository(db)
	incidentRepo := repository.NewIncidentRepository(db)
	timelineRepo := repository.NewTimelineRepository(db)

	incidentSvc := services.NewIncidentService(incidentRepo, timelineRepo, alertRepo, nil, db)
	alertSvc := services.NewAlertService(alertRepo, incidentSvc)

	router := gin.New()
	router.POST("/webhooks/prometheus", handlers.PrometheusWebhook(alertSvc))

	fingerprint := "duplicate-test-123"

	// Step 1: Send initial alert (firing)
	initialPayload := &webhooks.AlertmanagerPayload{
		Version:  "4",
		GroupKey: "test-group",
		Status:   "firing",
		Receiver: "test-receiver",
		Alerts: []webhooks.AlertmanagerAlert{
			{
				Status:       "firing",
				Labels:       map[string]string{"alertname": "DuplicateTest", "severity": "critical"},
				Annotations:  map[string]string{"summary": "Initial summary"},
				StartsAt:     time.Now(),
				Fingerprint:  fingerprint,
			},
		},
	}

	payloadBytes, _ := json.Marshal(initialPayload)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/prometheus", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Initial alert should succeed")

	// Verify initial state
	var alertCount int64
	db.Model(&models.Alert{}).Count(&alertCount)
	assert.Equal(t, int64(1), alertCount, "Should have 1 alert after initial webhook")

	var incidentCount int64
	db.Model(&models.Incident{}).Count(&incidentCount)
	assert.Equal(t, int64(1), incidentCount, "Should have 1 incident after initial webhook")

	// Step 2: Send duplicate alert (resolved)
	updatedPayload := &webhooks.AlertmanagerPayload{
		Version:  "4",
		GroupKey: "test-group",
		Status:   "resolved",
		Receiver: "test-receiver",
		Alerts: []webhooks.AlertmanagerAlert{
			{
				Status:       "resolved",
				Labels:       map[string]string{"alertname": "DuplicateTest", "severity": "critical"},
				Annotations:  map[string]string{"summary": "Updated summary"}, // Changed
				StartsAt:     time.Now(),
				EndsAt:       time.Now(),
				Fingerprint:  fingerprint, // Same fingerprint
			},
		},
	}

	payloadBytes, _ = json.Marshal(updatedPayload)
	req = httptest.NewRequest(http.MethodPost, "/webhooks/prometheus", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Duplicate alert should succeed")

	// Verify response indicates update, not creation
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	incidentsCreated := response["incidents_created"].(float64)
	assert.Equal(t, float64(0), incidentsCreated, "No new incidents should be created for duplicate alert")

	// Step 3: Verify database state
	db.Model(&models.Alert{}).Count(&alertCount)
	assert.Equal(t, int64(1), alertCount, "Should still have only 1 alert (updated, not duplicated)")

	db.Model(&models.Incident{}).Count(&incidentCount)
	assert.Equal(t, int64(1), incidentCount, "Should still have only 1 incident (not duplicated)")

	// Verify alert was updated
	var alert models.Alert
	err := db.Where("fingerprint = ?", fingerprint).First(&alert).Error
	require.NoError(t, err, "Alert should exist")

	assert.Equal(t, models.AlertStatusResolved, alert.Status, "Alert status should be updated to 'resolved'")
	assert.Equal(t, "Updated summary", alert.Description, "Alert description should be updated")
	assert.NotNil(t, alert.EndedAt, "Alert EndedAt should be set after resolution")
}

// TestInvalidPayloadValidation tests various invalid payload scenarios
// This covers the validation requirement from OI-058
func TestInvalidPayloadValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		payloadJSON    string
		expectedStatus int
		description    string
	}{
		{
			name:           "malformed JSON",
			payloadJSON:    `{"version": "4", "alerts": [}`, // Invalid JSON
			expectedStatus: http.StatusBadRequest,
			description:    "Malformed JSON should return 400",
		},
		{
			name: "missing required fields",
			payloadJSON: `{
				"version": "4",
				"status": "firing"
			}`, // Missing alerts array
			expectedStatus: http.StatusBadRequest,
			description:    "Missing required alerts field should return 400",
		},
		{
			name: "invalid alert status",
			payloadJSON: `{
				"version": "4",
				"status": "invalid_status",
				"alerts": [{
					"status": "invalid",
					"labels": {"alertname": "Test"},
					"startsAt": "2024-01-01T00:00:00Z",
					"fingerprint": "test123"
				}]
			}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Invalid status value should return 400",
		},
		{
			name: "too many alerts",
			payloadJSON: func() string {
				// Create payload with 101 alerts (exceeds max of 100)
				payload := map[string]interface{}{
					"version":  "4",
					"status":   "firing",
					"receiver": "test",
					"alerts":   make([]map[string]interface{}, 101),
				}
				for i := 0; i < 101; i++ {
					payload["alerts"].([]map[string]interface{})[i] = map[string]interface{}{
						"status":      "firing",
						"labels":      map[string]string{"alertname": "Test"},
						"startsAt":    time.Now().Format(time.RFC3339),
						"fingerprint": "test" + string(rune(i)),
					}
				}
				bytes, _ := json.Marshal(payload)
				return string(bytes)
			}(),
			expectedStatus: http.StatusBadRequest,
			description:    "More than 100 alerts should return 400",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			db, cleanup := setupTestDB(t)
			defer cleanup()

			alertRepo := repository.NewAlertRepository(db)
			incidentRepo := repository.NewIncidentRepository(db)
			timelineRepo := repository.NewTimelineRepository(db)

			incidentSvc := services.NewIncidentService(incidentRepo, timelineRepo, alertRepo, nil, db)
			alertSvc := services.NewAlertService(alertRepo, incidentSvc)

			router := gin.New()
			router.POST("/webhooks/prometheus", handlers.PrometheusWebhook(alertSvc))

			// Execute
			req := httptest.NewRequest(http.MethodPost, "/webhooks/prometheus", bytes.NewReader([]byte(tt.payloadJSON)))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Assert
			assert.Equal(t, tt.expectedStatus, w.Code, tt.description)

			// Verify no data was created in database
			var alertCount int64
			db.Model(&models.Alert{}).Count(&alertCount)
			assert.Equal(t, int64(0), alertCount, "No alerts should be created for invalid payload")

			var incidentCount int64
			db.Model(&models.Incident{}).Count(&incidentCount)
			assert.Equal(t, int64(0), incidentCount, "No incidents should be created for invalid payload")
		})
	}
}

// setupTestDB creates an in-memory SQLite database for testing
// Returns the database connection and a cleanup function
func setupTestDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()

	// Create in-memory SQLite database
	db, err := database.NewTestDB()
	require.NoError(t, err, "Failed to create test database")

	// Get underlying SQL DB to ensure tables are created properly
	sqlDB, err := db.DB()
	require.NoError(t, err, "Failed to get SQL DB")

	// For SQLite testing, we need to create simplified schemas without PostgreSQL-specific features
	// This creates tables compatible with our models but using SQLite syntax

	// Drop tables if they exist (for test isolation with shared cache)
	sqlDB.Exec(`DROP TABLE IF EXISTS timeline_entries`)
	sqlDB.Exec(`DROP TABLE IF EXISTS incident_alerts`)
	sqlDB.Exec(`DROP TABLE IF EXISTS incidents`)
	sqlDB.Exec(`DROP TABLE IF EXISTS alerts`)
	sqlDB.Exec(`DROP TRIGGER IF EXISTS assign_incident_number`)

	// Create alerts table
	_, err = sqlDB.Exec(`CREATE TABLE alerts (
		id TEXT PRIMARY KEY,
		external_id TEXT NOT NULL,
		source TEXT NOT NULL,
		fingerprint TEXT,
		status TEXT NOT NULL DEFAULT 'firing',
		severity TEXT NOT NULL DEFAULT 'info',
		title TEXT NOT NULL,
		description TEXT,
		labels TEXT DEFAULT '{}',
		annotations TEXT DEFAULT '{}',
		raw_payload TEXT,
		started_at DATETIME NOT NULL,
		ended_at DATETIME,
		received_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		escalation_policy_id TEXT,
		acknowledgment_status TEXT NOT NULL DEFAULT 'pending'
	)`)
	require.NoError(t, err, "Failed to create alerts table")

	_, err = sqlDB.Exec(`CREATE INDEX idx_alerts_fingerprint ON alerts(fingerprint)`)
	require.NoError(t, err, "Failed to create alerts index")

	// Create incidents table
	_, err = sqlDB.Exec(`CREATE TABLE incidents (
		id TEXT PRIMARY KEY,
		incident_number INTEGER UNIQUE,
		title TEXT NOT NULL,
		slug TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'triggered',
		severity TEXT NOT NULL DEFAULT 'medium',
		summary TEXT,
		group_key TEXT,
		slack_channel_id TEXT,
		slack_channel_name TEXT,
		slack_message_ts TEXT,
		teams_channel_id TEXT,
		teams_channel_name TEXT,
		teams_conversation_id TEXT,
		teams_activity_id TEXT,
		ai_enabled INTEGER NOT NULL DEFAULT 1,
		ai_summary TEXT,
		ai_summary_generated_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		triggered_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		acknowledged_at DATETIME,
		resolved_at DATETIME,
		created_by_type TEXT NOT NULL,
		created_by_id TEXT NOT NULL,
		commander_id TEXT,
		labels TEXT DEFAULT '{}',
		custom_fields TEXT DEFAULT '{}'
	)`)
	require.NoError(t, err, "Failed to create incidents table")

	// Create trigger to auto-assign incident_number
	_, err = sqlDB.Exec(`
		CREATE TRIGGER assign_incident_number
		AFTER INSERT ON incidents
		WHEN NEW.incident_number IS NULL
		BEGIN
			UPDATE incidents
			SET incident_number = (SELECT COALESCE(MAX(incident_number), 0) + 1 FROM incidents WHERE id != NEW.id)
			WHERE id = NEW.id;
		END;
	`)
	require.NoError(t, err, "Failed to create incident_number trigger")

	// Create incident_alerts join table for many-to-many relationship
	_, err = sqlDB.Exec(`CREATE TABLE incident_alerts (
		incident_id TEXT NOT NULL,
		alert_id TEXT NOT NULL,
		linked_by_type TEXT NOT NULL,
		linked_by_id TEXT NOT NULL,
		PRIMARY KEY (incident_id, alert_id),
		FOREIGN KEY (incident_id) REFERENCES incidents(id),
		FOREIGN KEY (alert_id) REFERENCES alerts(id)
	)`)
	require.NoError(t, err, "Failed to create incident_alerts join table")

	_, err = sqlDB.Exec(`CREATE INDEX idx_incidents_number ON incidents(incident_number)`)
	require.NoError(t, err, "Failed to create incidents number index")

	_, err = sqlDB.Exec(`CREATE INDEX idx_incidents_status ON incidents(status)`)
	require.NoError(t, err, "Failed to create incidents status index")

	_, err = sqlDB.Exec(`CREATE INDEX idx_incidents_severity ON incidents(severity)`)
	require.NoError(t, err, "Failed to create incidents severity index")

	// Create timeline_entries table
	_, err = sqlDB.Exec(`CREATE TABLE timeline_entries (
		id TEXT PRIMARY KEY,
		incident_id TEXT NOT NULL,
		timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		type TEXT NOT NULL,
		actor_type TEXT NOT NULL,
		actor_id TEXT,
		content TEXT NOT NULL DEFAULT '{}',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (incident_id) REFERENCES incidents(id)
	)`)
	require.NoError(t, err, "Failed to create timeline_entries table")

	_, err = sqlDB.Exec(`CREATE INDEX idx_timeline_incident ON timeline_entries(incident_id)`)
	require.NoError(t, err, "Failed to create timeline index")

	cleanup := func() {
		// Clean up all tables (order matters for foreign keys)
		db.Exec("DELETE FROM timeline_entries")
		db.Exec("DELETE FROM incident_alerts")
		db.Exec("DELETE FROM incidents")
		db.Exec("DELETE FROM alerts")
	}

	return db, cleanup
}
