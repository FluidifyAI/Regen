package services

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/google/uuid"
)

func TestTeamsEscalationCard_AlertDetailsAndLink(t *testing.T) {
	id := uuid.New()
	alert := &models.Alert{ID: id, Title: "EscalationCheck", Severity: models.AlertSeverityCritical, Source: "prometheus"}

	raw, _ := json.Marshal(TeamsEscalationCard(alert, 1, "https://regen.example.com"))
	got := string(raw)

	for _, want := range []string{
		"Tier 2", "EscalationCheck", "critical", "prometheus", "Attention",
		"View Alert", "https://regen.example.com/alerts/" + id.String(),
	} {
		if !strings.Contains(got, want) {
			t.Errorf("card missing %q:\n%s", want, got)
		}
	}
}

func TestTeamsEscalationCard_NilAlertDoesNotPanic(t *testing.T) {
	raw, _ := json.Marshal(TeamsEscalationCard(nil, 0, "https://regen.example.com"))
	got := string(raw)
	if !strings.Contains(got, "Tier 1") || !strings.Contains(got, "https://regen.example.com/incidents") {
		t.Errorf("incident-sourced card incomplete:\n%s", got)
	}
}

func TestSendDirectMessage_SendsTeamsCardWhenSet(t *testing.T) {
	var calls []recordedCall
	svc := newDMTestService(t, http.StatusOK, &calls)
	svc.SetUserResolver(func(string) (string, bool) { return "alice-teams-id", true })

	msg := Message{Text: "fallback", TeamsCard: map[string]interface{}{"type": "AdaptiveCard"}}
	if err := svc.SendDirectMessage("alice@example.com", msg); err != nil {
		t.Fatal(err)
	}
	activity := calls[1].body
	atts, ok := activity["attachments"].([]interface{})
	if !ok || len(atts) != 1 {
		t.Fatalf("expected one card attachment, got %v", activity)
	}
	if ct := atts[0].(map[string]interface{})["contentType"]; ct != "application/vnd.microsoft.card.adaptive" {
		t.Errorf("contentType = %v", ct)
	}
	if _, hasText := activity["text"]; hasText {
		t.Error("card activity should not also carry text")
	}
}
