package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ----------------------------------------------------------------------------
// Mocks
// ----------------------------------------------------------------------------

type mockIncidentRepoForSetup struct {
	GetByNumberFn func(number int) (*models.Incident, error)
}

func (m *mockIncidentRepoForSetup) GetByNumber(n int) (*models.Incident, error) {
	return m.GetByNumberFn(n)
}
func (m *mockIncidentRepoForSetup) Create(_ *models.Incident) error                    { return nil }
func (m *mockIncidentRepoForSetup) GetByID(_ uuid.UUID) (*models.Incident, error)      { return nil, nil }
func (m *mockIncidentRepoForSetup) GetBySlackChannelID(_ string) (*models.Incident, error) {
	return nil, nil
}
func (m *mockIncidentRepoForSetup) GetBySlackMessageTS(_ string) (*models.Incident, error) {
	return nil, nil
}
func (m *mockIncidentRepoForSetup) GetByTeamsChannelID(_ string) (*models.Incident, error) {
	return nil, nil
}
func (m *mockIncidentRepoForSetup) GetByTeamsConversationID(_ string) (*models.Incident, error) {
	return nil, nil
}
func (m *mockIncidentRepoForSetup) List(_ repository.IncidentFilters, _ repository.Pagination) ([]models.Incident, int64, error) {
	return nil, 0, nil
}
func (m *mockIncidentRepoForSetup) Update(_ *models.Incident) error                        { return nil }
func (m *mockIncidentRepoForSetup) UpdateStatus(_ uuid.UUID, _ models.IncidentStatus) error { return nil }
func (m *mockIncidentRepoForSetup) UpdateSlackChannel(_ uuid.UUID, _, _ string) error       { return nil }
func (m *mockIncidentRepoForSetup) UpdateSlackMessageTS(_ uuid.UUID, _ string) error        { return nil }
func (m *mockIncidentRepoForSetup) UpdateTeamsChannel(_ uuid.UUID, _, _ string) error       { return nil }
func (m *mockIncidentRepoForSetup) UpdateTeamsConversationID(_ uuid.UUID, _ string) error   { return nil }
func (m *mockIncidentRepoForSetup) UpdateTeamsActivityID(_ uuid.UUID, _ string) error       { return nil }
func (m *mockIncidentRepoForSetup) UpdateTeamsPostingIDs(_ uuid.UUID, _, _ string) error    { return nil }
func (m *mockIncidentRepoForSetup) LinkAlert(_ uuid.UUID, _ uuid.UUID, _, _ string) error   { return nil }
func (m *mockIncidentRepoForSetup) GetAlerts(_ uuid.UUID) ([]models.Alert, error)           { return nil, nil }
func (m *mockIncidentRepoForSetup) GetIncidentByAlertID(_ uuid.UUID) (*models.Incident, error) {
	return nil, nil
}
func (m *mockIncidentRepoForSetup) UpdateAISummary(_ uuid.UUID, _ string, _ time.Time) error {
	return nil
}

type mockSlackConfigRepoForSetup struct {
	GetFn func() (*models.SlackConfig, error)
}

func (m *mockSlackConfigRepoForSetup) Get() (*models.SlackConfig, error) { return m.GetFn() }
func (m *mockSlackConfigRepoForSetup) Save(_ *models.SlackConfig) error  { return nil }
func (m *mockSlackConfigRepoForSetup) Delete() error                     { return nil }

type mockScheduleRepoForSetup struct {
	GetAllFn func() ([]models.Schedule, error)
}

func (m *mockScheduleRepoForSetup) GetAll() ([]models.Schedule, error) { return m.GetAllFn() }
func (m *mockScheduleRepoForSetup) Create(_ *models.Schedule) error    { return nil }
func (m *mockScheduleRepoForSetup) GetByID(_ uuid.UUID) (*models.Schedule, error) {
	return nil, nil
}
func (m *mockScheduleRepoForSetup) Update(_ *models.Schedule) error       { return nil }
func (m *mockScheduleRepoForSetup) Delete(_ uuid.UUID) error              { return nil }
func (m *mockScheduleRepoForSetup) GetWithLayers(_ uuid.UUID) (*models.Schedule, error) {
	return nil, nil
}
func (m *mockScheduleRepoForSetup) CreateLayer(_ *models.ScheduleLayer) error { return nil }
func (m *mockScheduleRepoForSetup) DeleteLayer(_ uuid.UUID) error             { return nil }
func (m *mockScheduleRepoForSetup) UpdateLayer(_ *models.ScheduleLayer, _ *[]models.ScheduleParticipant) error {
	return nil
}
func (m *mockScheduleRepoForSetup) CreateParticipantsBulk(_ []models.ScheduleParticipant) error {
	return nil
}
func (m *mockScheduleRepoForSetup) CreateOverride(_ *models.ScheduleOverride) error { return nil }
func (m *mockScheduleRepoForSetup) DeleteOverride(_ uuid.UUID) error                { return nil }
func (m *mockScheduleRepoForSetup) GetActiveOverrides(_ uuid.UUID, _ time.Time) ([]models.ScheduleOverride, error) {
	return nil, nil
}
func (m *mockScheduleRepoForSetup) GetOverridesInWindow(_ uuid.UUID, _, _ time.Time) ([]models.ScheduleOverride, error) {
	return nil, nil
}
func (m *mockScheduleRepoForSetup) ListUpcomingOverrides(_ uuid.UUID) ([]models.ScheduleOverride, error) {
	return nil, nil
}
func (m *mockScheduleRepoForSetup) GetHolidayCountries(_ uuid.UUID) ([]string, error) {
	return nil, nil
}
func (m *mockScheduleRepoForSetup) SetHolidayCountries(_ uuid.UUID, _ []string) ([]string, []string, error) {
	return nil, nil, nil
}
func (m *mockScheduleRepoForSetup) UpsertHolidays(_ []models.ScheduleHoliday) error { return nil }
func (m *mockScheduleRepoForSetup) ListHolidays(_ uuid.UUID, _, _ time.Time) ([]models.ScheduleHoliday, error) {
	return nil, nil
}
func (m *mockScheduleRepoForSetup) DeleteHolidaysByCountry(_ uuid.UUID, _ string) error { return nil }
func (m *mockScheduleRepoForSetup) ListSchedulesWithHolidays() ([]models.Schedule, error) {
	return nil, nil
}
func (m *mockScheduleRepoForSetup) CreateUnavailability(_ *models.ScheduleUnavailability) error {
	return nil
}
func (m *mockScheduleRepoForSetup) DeleteUnavailability(_, _ uuid.UUID) error { return nil }
func (m *mockScheduleRepoForSetup) ListUnavailabilities(_ uuid.UUID) ([]models.ScheduleUnavailability, error) {
	return nil, nil
}
func (m *mockScheduleRepoForSetup) GetUnavailabilitiesInWindow(_ uuid.UUID, _, _ time.Time) ([]models.ScheduleUnavailability, error) {
	return nil, nil
}

// ----------------------------------------------------------------------------
// Helper
// ----------------------------------------------------------------------------

func setupStatusRouter(
	incidentRepo repository.IncidentRepository,
	slackRepo repository.SlackConfigRepository,
	scheduleRepo repository.ScheduleRepository,
) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/setup/status", GetSetupStatus(incidentRepo, slackRepo, scheduleRepo))
	return r
}

func getSetupStatus(t *testing.T, r *gin.Engine) map[string]interface{} {
	t.Helper()
	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/api/v1/setup/status", nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body
}

// noIncident returns a GetByNumber mock that simulates a fresh install (no demo data).
func noIncident() func(int) (*models.Incident, error) {
	return func(n int) (*models.Incident, error) {
		return nil, &repository.NotFoundError{Resource: "incident", ID: n}
	}
}

// ----------------------------------------------------------------------------
// Tests
// ----------------------------------------------------------------------------

func TestGetSetupStatus_SlackConnected(t *testing.T) {
	r := setupStatusRouter(
		&mockIncidentRepoForSetup{GetByNumberFn: noIncident()},
		&mockSlackConfigRepoForSetup{GetFn: func() (*models.SlackConfig, error) {
			return &models.SlackConfig{BotToken: "xoxb-test"}, nil
		}},
		&mockScheduleRepoForSetup{GetAllFn: func() ([]models.Schedule, error) {
			return nil, nil
		}},
	)

	body := getSetupStatus(t, r)
	assert.Equal(t, true, body["slack_connected"], "expect slack_connected true when config row exists")
	assert.Equal(t, false, body["has_schedule"])
}

func TestGetSetupStatus_NoSlack(t *testing.T) {
	r := setupStatusRouter(
		&mockIncidentRepoForSetup{GetByNumberFn: noIncident()},
		&mockSlackConfigRepoForSetup{GetFn: func() (*models.SlackConfig, error) {
			return nil, nil // not configured
		}},
		&mockScheduleRepoForSetup{GetAllFn: func() ([]models.Schedule, error) {
			return nil, nil
		}},
	)

	body := getSetupStatus(t, r)
	assert.Equal(t, false, body["slack_connected"])
}

func TestGetSetupStatus_HasSchedule(t *testing.T) {
	r := setupStatusRouter(
		&mockIncidentRepoForSetup{GetByNumberFn: noIncident()},
		&mockSlackConfigRepoForSetup{GetFn: func() (*models.SlackConfig, error) { return nil, nil }},
		&mockScheduleRepoForSetup{GetAllFn: func() ([]models.Schedule, error) {
			return []models.Schedule{{ID: uuid.New(), Name: "Primary"}}, nil
		}},
	)

	body := getSetupStatus(t, r)
	assert.Equal(t, true, body["has_schedule"], "expect has_schedule true when schedules exist")
	assert.Equal(t, false, body["slack_connected"])
}

func TestGetSetupStatus_NoSchedule(t *testing.T) {
	r := setupStatusRouter(
		&mockIncidentRepoForSetup{GetByNumberFn: noIncident()},
		&mockSlackConfigRepoForSetup{GetFn: func() (*models.SlackConfig, error) { return nil, nil }},
		&mockScheduleRepoForSetup{GetAllFn: func() ([]models.Schedule, error) {
			return []models.Schedule{}, nil
		}},
	)

	body := getSetupStatus(t, r)
	assert.Equal(t, false, body["has_schedule"])
}

func TestGetSetupStatus_FreshInstall(t *testing.T) {
	r := setupStatusRouter(
		&mockIncidentRepoForSetup{GetByNumberFn: noIncident()},
		&mockSlackConfigRepoForSetup{GetFn: func() (*models.SlackConfig, error) { return nil, nil }},
		&mockScheduleRepoForSetup{GetAllFn: func() ([]models.Schedule, error) { return nil, nil }},
	)

	body := getSetupStatus(t, r)
	assert.Equal(t, false, body["slack_connected"], "fresh install: no slack")
	assert.Equal(t, false, body["has_schedule"], "fresh install: no schedules")
	assert.Equal(t, true, body["demo_data_available"], "fresh install: demo data can be loaded")
}
