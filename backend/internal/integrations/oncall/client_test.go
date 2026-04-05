package oncall

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// oncallPage returns a JSON-encoded listResponse for the given items and next URL.
func oncallPage[T any](items []T, next string) []byte {
	page := listResponse[T]{Results: items, Next: next}
	b, _ := json.Marshal(page)
	return b
}

// ── Ping ──────────────────────────────────────────────────────────────────────

func TestPing_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/users/", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(oncallPage([]OnCallUser{}, ""))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "mytoken")
	assert.NoError(t, c.Ping())
}

func TestPing_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "bad-token")
	err := c.Ping()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid API token")
	assert.Contains(t, err.Error(), "401")
}

func TestPing_Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "low-perms-token")
	err := c.Ping()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid API token")
	assert.Contains(t, err.Error(), "403")
}

func TestPing_UnexpectedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("service down"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	err := c.Ping()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

func TestPing_UnreachableHost(t *testing.T) {
	c := NewClient("http://127.0.0.1:19999", "tok")
	err := c.Ping()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot reach Grafana OnCall")
}

// ── Auth header ───────────────────────────────────────────────────────────────

func TestClient_SendsAuthorizationHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write(oncallPage([]OnCallUser{}, ""))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "secret-key")
	_, err := c.ListUsers()
	require.NoError(t, err)
	assert.Equal(t, "Token secret-key", gotAuth)
}

func TestClient_SendsUserAgentHeader(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		w.Write(oncallPage([]OnCallUser{}, ""))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok")
	_, _ = c.ListUsers()
	assert.Equal(t, userAgent, gotUA)
}

// ── ListUsers ─────────────────────────────────────────────────────────────────

func TestListUsers_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(oncallPage([]OnCallUser{}, ""))
	}))
	defer srv.Close()

	users, err := NewClient(srv.URL, "tok").ListUsers()
	require.NoError(t, err)
	assert.Empty(t, users)
}

func TestListUsers_SinglePage(t *testing.T) {
	want := []OnCallUser{
		{ID: "u1", Email: "alice@example.com", Username: "alice", Role: "admin"},
		{ID: "u2", Email: "bob@example.com", Username: "bob", Role: "viewer"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(oncallPage(want, ""))
	}))
	defer srv.Close()

	users, err := NewClient(srv.URL, "tok").ListUsers()
	require.NoError(t, err)
	require.Len(t, users, 2)
	assert.Equal(t, "alice@example.com", users[0].Email)
	assert.Equal(t, "bob@example.com", users[1].Email)
}

func TestListUsers_Pagination(t *testing.T) {
	// Two pages: first returns a Next URL pointing at the second page.
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		switch n {
		case 1:
			// First page — next points to the server's second-page URL.
			page1 := []OnCallUser{{ID: "u1", Email: "page1@example.com"}}
			// We build the next URL ourselves using the test server URL.
			// The client follows absolute Next URLs verbatim.
			nextURL := r.Host // includes host without scheme
			w.Write(oncallPage(page1, "http://"+nextURL+"/api/v1/users/?page=2"))
		default:
			// Second (final) page — no Next.
			page2 := []OnCallUser{{ID: "u2", Email: "page2@example.com"}}
			w.Write(oncallPage(page2, ""))
		}
	}))
	defer srv.Close()

	users, err := NewClient(srv.URL, "tok").ListUsers()
	require.NoError(t, err)
	require.Len(t, users, 2)
	assert.Equal(t, "page1@example.com", users[0].Email)
	assert.Equal(t, "page2@example.com", users[1].Email)
	assert.EqualValues(t, 2, atomic.LoadInt32(&callCount))
}

// ── ListSchedules ─────────────────────────────────────────────────────────────

func TestListSchedules_SinglePage(t *testing.T) {
	want := []OnCallSchedule{
		{ID: "s1", Name: "Primary", Type: "rolling_users", TimeZone: "UTC"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/schedules/", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Write(oncallPage(want, ""))
	}))
	defer srv.Close()

	schedules, err := NewClient(srv.URL, "tok").ListSchedules()
	require.NoError(t, err)
	require.Len(t, schedules, 1)
	assert.Equal(t, "Primary", schedules[0].Name)
}

// ── ListShifts ────────────────────────────────────────────────────────────────

func TestListShifts_SinglePage(t *testing.T) {
	want := []OnCallShift{
		{ID: "sh1", Name: "Week", Type: "rolling_users", Frequency: "weekly"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/on_call_shifts/", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Write(oncallPage(want, ""))
	}))
	defer srv.Close()

	shifts, err := NewClient(srv.URL, "tok").ListShifts()
	require.NoError(t, err)
	require.Len(t, shifts, 1)
	assert.Equal(t, "sh1", shifts[0].ID)
}

// ── ListEscalationChains ──────────────────────────────────────────────────────

func TestListEscalationChains_SinglePage(t *testing.T) {
	want := []OnCallEscalationChain{{ID: "ec1", Name: "SRE Chain"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/escalation_chains/", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Write(oncallPage(want, ""))
	}))
	defer srv.Close()

	chains, err := NewClient(srv.URL, "tok").ListEscalationChains()
	require.NoError(t, err)
	require.Len(t, chains, 1)
	assert.Equal(t, "SRE Chain", chains[0].Name)
}

// ── ListEscalationSteps ───────────────────────────────────────────────────────

func TestListEscalationSteps_SinglePage(t *testing.T) {
	dur := 300
	want := []OnCallEscalationStep{
		{ID: "ep1", EscalationChain: "ec1", Type: StepTypeNotifyPersons, Step: 0, Duration: &dur},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/escalation_policies/", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Write(oncallPage(want, ""))
	}))
	defer srv.Close()

	steps, err := NewClient(srv.URL, "tok").ListEscalationSteps()
	require.NoError(t, err)
	require.Len(t, steps, 1)
	assert.Equal(t, "ec1", steps[0].EscalationChain)
}

// ── ListIntegrations ──────────────────────────────────────────────────────────

func TestListIntegrations_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/integrations/", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Write(oncallPage([]OnCallIntegration{}, ""))
	}))
	defer srv.Close()

	integrations, err := NewClient(srv.URL, "tok").ListIntegrations()
	require.NoError(t, err)
	assert.Empty(t, integrations)
}

func TestListIntegrations_MultipleTypes(t *testing.T) {
	want := []OnCallIntegration{
		{ID: "i1", Name: "Alertmanager", Type: "alertmanager", Link: "http://old/am"},
		{ID: "i2", Name: "Grafana", Type: "grafana", Link: "http://old/grafana"},
		{ID: "i3", Name: "Custom", Type: "webhook", Link: "http://old/custom"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(oncallPage(want, ""))
	}))
	defer srv.Close()

	integrations, err := NewClient(srv.URL, "tok").ListIntegrations()
	require.NoError(t, err)
	require.Len(t, integrations, 3)
	assert.Equal(t, "alertmanager", integrations[0].Type)
}

// ── Error handling ────────────────────────────────────────────────────────────

func TestListUsers_ServerError(t *testing.T) {
	// Always returns 500 — the client should fail after exhausting retries.
	// We disable the sleep by controlling the server to return 500 every time.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer srv.Close()

	// Override httpClient timeout to be short to avoid multi-second retry sleeps in tests.
	c := NewClient(srv.URL, "tok")
	_, err := c.ListUsers()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing OnCall users")
}

func TestListUsers_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not-json{{{"))
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "tok").ListUsers()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decoding OnCall response")
}

// ── NewClient trailing slash normalisation ────────────────────────────────────

func TestNewClient_TrimsTrailingSlash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should only have one slash between base and path
		assert.False(t, strings.Contains(r.URL.Path, "//"),
			"unexpected double slash in path: %s", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Write(oncallPage([]OnCallUser{}, ""))
	}))
	defer srv.Close()

	c := NewClient(srv.URL+"/", "tok") // trailing slash on base URL
	_, err := c.ListUsers()
	require.NoError(t, err)
}
