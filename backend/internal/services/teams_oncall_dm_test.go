package services

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/models"
)

func TestTeamsMemberID(t *testing.T) {
	cases := map[string]string{
		"abc123":    "29:abc123",
		"29:abc123": "29:abc123",
		"  abc123 ": "29:abc123",
	}
	for in, want := range cases {
		if got := teamsMemberID(in); got != want {
			t.Errorf("teamsMemberID(%q) = %q, want %q", in, got, want)
		}
	}
}

type recordedCall struct {
	path string
	body map[string]interface{}
}

func newDMTestService(t *testing.T, activityStatus int, calls *[]recordedCall) *TeamsService {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]interface{}
		_ = json.Unmarshal(raw, &body)
		*calls = append(*calls, recordedCall{path: r.URL.Path, body: body})
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/activities") {
			w.WriteHeader(activityStatus)
			_, _ = w.Write([]byte(`{"id":"act-1"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"conv-1"}`))
	}))
	t.Cleanup(srv.Close)
	return &TeamsService{
		ctx: t.Context(), tenantID: "tenant", botAppID: "app",
		serviceURL: srv.URL + "/", botfwClient: srv.Client(),
	}
}

func TestSendOnCallDM_PostsCardAsSeparateActivity(t *testing.T) {
	var calls []recordedCall
	svc := newDMTestService(t, http.StatusOK, &calls)

	if err := svc.SendOnCallDM("abc123", &models.Incident{Title: "db down"}, "http://app"); err != nil {
		t.Fatalf("SendOnCallDM: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("want 2 calls, got %d", len(calls))
	}
	if calls[0].path != "/v3/conversations" {
		t.Errorf("first call path = %q", calls[0].path)
	}
	if _, inline := calls[0].body["activity"]; inline {
		t.Error("conversation create must not carry an inline activity")
	}
	members := calls[0].body["members"].([]interface{})
	if id := members[0].(map[string]interface{})["id"]; id != "29:abc123" {
		t.Errorf("member id = %v, want 29:abc123", id)
	}
	if calls[1].path != "/v3/conversations/conv-1/activities" {
		t.Errorf("second call path = %q", calls[1].path)
	}
	if calls[1].body["type"] != "message" || calls[1].body["attachments"] == nil {
		t.Errorf("activity body missing card: %v", calls[1].body)
	}
}

func TestSendOnCallDM_ReportsActivityFailure(t *testing.T) {
	var calls []recordedCall
	svc := newDMTestService(t, http.StatusForbidden, &calls)

	if err := svc.SendOnCallDM("abc123", &models.Incident{Title: "db down"}, "http://app"); err == nil {
		t.Fatal("a failed activity post must not be reported as success")
	}
}
