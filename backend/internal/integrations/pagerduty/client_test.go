package pagerduty

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeTestClient creates an httptest server and a Client pre-configured to use it.
func makeTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewClient("test-key")
	c.baseURL = srv.URL
	return c
}

func TestValidateAPIKey_Success(t *testing.T) {
	c := makeTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/users/me", r.URL.Path)
		assert.Equal(t, "Token token=test-key", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"user":{"id":"U1","email":"a@b.com","name":"Alice"}}`))
	})
	require.NoError(t, c.ValidateAPIKey())
}

func TestValidateAPIKey_Invalid(t *testing.T) {
	c := makeTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	err := c.ValidateAPIKey()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid PagerDuty API key")
}

func TestFetchUsers_SinglePage(t *testing.T) {
	c := makeTestClient(t, func(w http.ResponseWriter, r *http.Request) {
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
	c := makeTestClient(t, func(w http.ResponseWriter, r *http.Request) {
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
	assert.Len(t, result, 2)
	assert.Equal(t, 2, calls, "should have made 2 paginated requests")
}

func TestFetchSchedules_SinglePage(t *testing.T) {
	c := makeTestClient(t, func(w http.ResponseWriter, r *http.Request) {
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
	c := makeTestClient(t, func(w http.ResponseWriter, r *http.Request) {
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
	c := makeTestClient(t, func(w http.ResponseWriter, r *http.Request) {
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
	c := makeTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/escalation_policies/P1", r.URL.Path)
		assert.Equal(t, "targets", r.URL.Query().Get("include[]"))
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
	c := makeTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"user":{}}`))
	})
	// Override backoff time via a direct call — ValidateAPIKey exercises the retry path
	c.httpClient.Timeout = 5 * time.Second // keep test fast
	require.NoError(t, c.ValidateAPIKey())
	assert.Equal(t, 2, calls, "should have retried once after 429")
}
