package services

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/models"
)

func TestBuildEscalationDMMessage_NilAlertDoesNotPanic(t *testing.T) {
	msg := NewSlackMessageBuilder().BuildEscalationDMMessage(nil, 1)

	if !strings.Contains(msg.Text, "Tier 2") {
		t.Errorf("text %q should mention Tier 2", msg.Text)
	}
	raw, _ := json.Marshal(msg.Blocks)
	if strings.Contains(string(raw), "acknowledge_") {
		t.Error("incident-sourced page must not carry an alert Acknowledge button")
	}
}

func TestBuildEscalationDMMessage_AlertHasAcknowledgeButton(t *testing.T) {
	alert := &models.Alert{Title: "disk full", Severity: models.AlertSeverityCritical, Source: "prometheus"}
	msg := NewSlackMessageBuilder().BuildEscalationDMMessage(alert, 0)

	raw, _ := json.Marshal(msg.Blocks)
	if !strings.Contains(string(raw), "acknowledge_") {
		t.Error("alert page should keep its Acknowledge button")
	}
}
