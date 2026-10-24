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

func TestSendDirectMessage_UsesBotFrameworkWithResolvedTeamsID(t *testing.T) {
	var calls []recordedCall
	svc := newDMTestService(t, http.StatusOK, &calls)
	svc.SetUserResolver(func(identifier string) (string, bool) {
		if identifier == "alice@example.com" {
			return "alice-teams-id", true
		}
		return "", false
	})

	if err := svc.SendDirectMessage("alice@example.com", Message{Text: "you are being paged"}); err != nil {
		t.Fatalf("SendDirectMessage: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("want 2 Bot Framework calls, got %d", len(calls))
	}
	members := calls[0].body["members"].([]interface{})
	if id := members[0].(map[string]interface{})["id"]; id != "29:alice-teams-id" {
		t.Errorf("member id = %v", id)
	}
	if calls[1].body["text"] != "you are being paged" {
		t.Errorf("activity text = %v", calls[1].body["text"])
	}
	if _, hasCard := calls[1].body["attachments"]; hasCard {
		t.Error("Slack blocks must not be sent as an Adaptive Card")
	}
}

func TestSendDirectMessage_UnknownUserFailsWithoutCallingTeams(t *testing.T) {
	var calls []recordedCall
	svc := newDMTestService(t, http.StatusOK, &calls)
	svc.SetUserResolver(func(string) (string, bool) { return "", false })

	if err := svc.SendDirectMessage("nobody@example.com", Message{Text: "hi"}); err == nil {
		t.Fatal("a user with no teams_user_id must produce an error, not a silent success")
	}
	if len(calls) != 0 {
		t.Errorf("no Teams call expected, got %d", len(calls))
	}
}

func TestSendDirectMessage_WithoutResolverFails(t *testing.T) {
	var calls []recordedCall
	svc := newDMTestService(t, http.StatusOK, &calls)
	if err := svc.SendDirectMessage("alice@example.com", Message{Text: "hi"}); err == nil {
		t.Fatal("expected an error when no resolver is configured")
	}
}
