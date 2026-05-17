package opsgenie_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/integrations/opsgenie"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMockServer builds a test HTTP server with realistic Opsgenie responses.
func newMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/v2/users/me", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GenieKey test-key", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"id":"u1","username":"admin@example.com","fullName":"Admin"}}`))
	})

	mux.HandleFunc("/v2/users", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "u1", "username": "alice@example.com", "fullName": "Alice"},
				{"id": "u2", "username": "bob@example.com", "fullName": "Bob"},
			},
		})
	})

	mux.HandleFunc("/v2/schedules", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "s1", "name": "Primary On-Call", "timezone": "America/New_York", "enabled": true},
			},
		})
	})

	mux.HandleFunc("/v2/schedules/s1/rotations", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id": "r1", "name": "Weekly Rotation", "type": "weekly", "length": 1,
					"participants": []map[string]any{
						{"type": "user", "id": "u1", "username": "alice@example.com", "name": "Alice"},
					},
				},
			},
		})
	})

	mux.HandleFunc("/v1/escalations", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id": "e1", "name": "Default Escalation",
					"rules": []map[string]any{
						{
							"condition": "if-not-acked",
							"notifyType": "default",
							"delay":     map[string]any{"timeAmount": 5, "timeUnit": "minutes"},
							"recipient": []map[string]any{
								{"type": "schedule", "id": "s1", "name": "Primary On-Call"},
							},
						},
					},
				},
			},
		})
	})

	return httptest.NewServer(mux)
}

func TestValidateAPIKey_valid(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()
	c := opsgenie.NewClientWithBaseURL("test-key", "us", srv.URL)
	require.NoError(t, c.ValidateAPIKey())
}

func TestValidateAPIKey_invalid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	c := opsgenie.NewClientWithBaseURL("bad-key", "us", srv.URL)
	require.Error(t, c.ValidateAPIKey())
}

func TestFetchUsers(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()
	c := opsgenie.NewClientWithBaseURL("test-key", "us", srv.URL)
	users, err := c.FetchUsers()
	require.NoError(t, err)
	assert.Len(t, users, 2)
	assert.Equal(t, "Alice", users["alice@example.com"])
	assert.Equal(t, "Bob", users["bob@example.com"])
}

func TestFetchSchedules(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()
	c := opsgenie.NewClientWithBaseURL("test-key", "us", srv.URL)
	schedules, err := c.FetchSchedules()
	require.NoError(t, err)
	require.Len(t, schedules, 1)
	assert.Equal(t, "Primary On-Call", schedules[0].Name)
}

func TestFetchRotations(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()
	c := opsgenie.NewClientWithBaseURL("test-key", "us", srv.URL)
	rotations, err := c.FetchRotations("s1")
	require.NoError(t, err)
	require.Len(t, rotations, 1)
	assert.Equal(t, "Weekly Rotation", rotations[0].Name)
	assert.Equal(t, "weekly", rotations[0].Type)
}

func TestFetchEscalationPolicies(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()
	c := opsgenie.NewClientWithBaseURL("test-key", "us", srv.URL)
	policies, err := c.FetchEscalationPolicies()
	require.NoError(t, err)
	require.Len(t, policies, 1)
	assert.Equal(t, "Default Escalation", policies[0].Name)
	require.Len(t, policies[0].Rules, 1)
	assert.Equal(t, 5, policies[0].Rules[0].Delay.TimeAmount)
	assert.Equal(t, "minutes", policies[0].Rules[0].Delay.TimeUnit)
}
