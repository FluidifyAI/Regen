package services

import (
	"context"
	"errors"
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/models/webhooks"
)

type recordingChat struct {
	recipients []string
	texts      []string
	err        error
}

func (m *recordingChat) CreateChannel(string, string) (*Channel, error) { return nil, nil }
func (m *recordingChat) PostMessage(string, Message) (string, error)    { return "", nil }
func (m *recordingChat) UpdateMessage(string, string, Message) error    { return nil }
func (m *recordingChat) ArchiveChannel(string) error                    { return nil }
func (m *recordingChat) InviteUsers(string, []string) error             { return nil }
func (m *recordingChat) GetThreadMessages(string, string) ([]string, error) {
	return nil, nil
}
func (m *recordingChat) SendDirectMessage(username string, message Message) error {
	m.recipients = append(m.recipients, username)
	m.texts = append(m.texts, message.Text)
	return m.err
}

func TestAlertService_DMsEmailFromLabelWhenSuppressed(t *testing.T) {
	rules := []models.RoutingRule{
		makeRoutingRule(10, models.JSONB{}, models.JSONB{
			"suppress":           true,
			"notify_email_label": "email",
		}),
	}
	chat := &recordingChat{}
	svc := buildAlertServiceWithEscalation(rules, &mockEscalationEngine{})
	svc.SetChatService(chat)

	alert := makeNormalizedAlert("generic", "warning")
	alert.Title = "CI failed"
	alert.Description = "main"
	alert.Labels = map[string]string{"email": "dev@example.com"}

	result, err := svc.ProcessNormalizedAlerts(context.Background(), []webhooks.NormalizedAlert{alert})
	if err != nil {
		t.Fatalf("ProcessNormalizedAlerts: %v", err)
	}
	if result.IncidentsCreated != 0 {
		t.Errorf("suppressed alert created %d incidents", result.IncidentsCreated)
	}
	if len(chat.recipients) != 1 || chat.recipients[0] != "dev@example.com" {
		t.Fatalf("recipients = %v, want [dev@example.com]", chat.recipients)
	}
	if chat.texts[0] != "CI failed\nmain" {
		t.Errorf("message = %q", chat.texts[0])
	}
}

func TestAlertService_SkipsDMWhenLabelIsNotAnEmail(t *testing.T) {
	rules := []models.RoutingRule{
		makeRoutingRule(10, models.JSONB{}, models.JSONB{"notify_email_label": "email"}),
	}
	chat := &recordingChat{}
	svc := buildAlertServiceWithEscalation(rules, &mockEscalationEngine{})
	svc.SetChatService(chat)

	alert := makeNormalizedAlert("generic", "warning")
	alert.Labels = map[string]string{"email": "not-an-email"}

	if _, err := svc.ProcessNormalizedAlerts(context.Background(), []webhooks.NormalizedAlert{alert}); err != nil {
		t.Fatalf("ProcessNormalizedAlerts: %v", err)
	}
	if len(chat.recipients) != 0 {
		t.Fatalf("recipients = %v, want none", chat.recipients)
	}
}

func TestAlertService_SlackErrorDoesNotFailProcessing(t *testing.T) {
	rules := []models.RoutingRule{
		makeRoutingRule(10, models.JSONB{}, models.JSONB{"notify_email_label": "email"}),
	}
	chat := &recordingChat{err: errors.New("slack down")}
	svc := buildAlertServiceWithEscalation(rules, &mockEscalationEngine{})
	svc.SetChatService(chat)

	alert := makeNormalizedAlert("generic", "warning")
	alert.Labels = map[string]string{"email": "dev@example.com"}

	if _, err := svc.ProcessNormalizedAlerts(context.Background(), []webhooks.NormalizedAlert{alert}); err != nil {
		t.Fatalf("ProcessNormalizedAlerts: %v", err)
	}
	if len(chat.recipients) != 1 {
		t.Fatalf("expected one DM attempt, got %d", len(chat.recipients))
	}
}

func TestAlertService_RepeatAlertDoesNotDMAgain(t *testing.T) {
	rules := []models.RoutingRule{
		makeRoutingRule(10, models.JSONB{}, models.JSONB{"notify_email_label": "email"}),
	}
	chat := &recordingChat{}
	alertRepo := &mockAlertRepository{}
	svc := NewAlertService(alertRepo, &mockIncidentService{shouldCreate: true})
	svc.SetRoutingEngine(NewRoutingEngine(&mockRoutingRuleRepo{rules: rules}))
	svc.SetChatService(chat)

	alert := makeNormalizedAlert("generic", "warning")
	alert.Labels = map[string]string{"email": "dev@example.com"}

	if _, err := svc.ProcessNormalizedAlerts(context.Background(), []webhooks.NormalizedAlert{alert}); err != nil {
		t.Fatalf("first process: %v", err)
	}
	alertRepo.existingAlert = &models.Alert{Source: alert.Source, ExternalID: alert.ExternalID}
	if _, err := svc.ProcessNormalizedAlerts(context.Background(), []webhooks.NormalizedAlert{alert}); err != nil {
		t.Fatalf("second process: %v", err)
	}
	if len(chat.recipients) != 1 {
		t.Fatalf("DM count = %d, want 1", len(chat.recipients))
	}
}

func TestEmailFromLabel(t *testing.T) {
	labels := models.JSONB{"email": "  dev@example.com  ", "name": "dev"}
	got, ok := emailFromLabel(labels, "email")
	if !ok || got != "dev@example.com" {
		t.Fatalf("email = %q ok=%v", got, ok)
	}
	if _, ok := emailFromLabel(labels, "name"); ok {
		t.Fatal("display name should not be treated as an email")
	}
	if _, ok := emailFromLabel(labels, "missing"); ok {
		t.Fatal("missing label should not match")
	}
}
