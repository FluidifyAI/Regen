package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/FluidifyAI/Regen/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// settingsRepoStub implements only what GetSystemSettings reads; the embedded nil
// interface panics if the handler touches anything else.
type settingsRepoStub struct {
	repository.SystemSettingsRepository
	values map[string]string
}

func (s settingsRepoStub) GetString(key string) (string, error) { return s.values[key], nil }
func (s settingsRepoStub) GetTelemetryOptOut() (bool, error)    { return false, nil }

type aiStub struct{ services.AIService }

func (aiStub) IsEnabled() bool { return false }

func getInstanceTimezone(t *testing.T, saved string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	repo := settingsRepoStub{values: map[string]string{}}
	if saved != "" {
		repo.values[repository.KeyTimezone] = saved
	}
	r.GET("/settings/system", GetSystemSettings(repo, aiStub{}, false))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/settings/system", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Timezone string `json:"timezone"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body.Timezone
}

func TestGetSystemSettings_TimezoneDefaultsToContainerTZ(t *testing.T) {
	t.Setenv("TZ", "Australia/Perth")
	assert.Equal(t, "Australia/Perth", getInstanceTimezone(t, ""))
}

func TestGetSystemSettings_SavedTimezoneWinsOverTZ(t *testing.T) {
	t.Setenv("TZ", "Australia/Perth")
	assert.Equal(t, "Europe/Paris", getInstanceTimezone(t, "Europe/Paris"))
}

func TestDefaultInstanceTimezone(t *testing.T) {
	cases := []struct{ name, tz, want string }{
		{"valid zone", "Asia/Kolkata", "Asia/Kolkata"},
		{"leading colon (POSIX style)", ":Australia/Perth", "Australia/Perth"},
		{"unset", "", "UTC"},
		{"garbage", "Not/AZone", "UTC"},
		{"Go's special Local name", "Local", "UTC"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TZ", tc.tz)
			assert.Equal(t, tc.want, defaultInstanceTimezone())
		})
	}
}
