package services

import (
	"context"
	"errors"
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/models/webhooks"
	"github.com/google/uuid"
)

type recordingChat struct {
	ids    []string
	emails []string
	names  []string
	texts  []string
	err    error
}

func (m *recordingChat) CreateChannel(string, string) (*Channel, error) { return nil, nil }
func (m *recordingChat) PostMessage(string, Message) (string, error)    { return "", nil }
func (m *recordingChat) UpdateMessage(string, string, Message) error    { return nil }
func (m *recordingChat) ArchiveChannel(string) error                    { return nil }
func (m *recordingChat) InviteUsers(string, []string) error             { return nil }
func (m *recordingChat) GetThreadMessages(string, string) ([]string, error) {
	return nil, nil
}
func (m *recordingChat) SendDirectMessage(name string, message Message) error {
	m.names = append(m.names, name)
	m.texts = append(m.texts, message.Text)
	return m.err
}
func (m *recordingChat) SendDirectMessageByUserID(id string, message Message) error {
	m.ids = append(m.ids, id)
	m.texts = append(m.texts, message.Text)
	return m.err
}
func (m *recordingChat) SendDirectMessageByEmail(email string, message Message) error {
	m.emails = append(m.emails, email)
	m.texts = append(m.texts, message.Text)
	return m.err
}

func (m *recordingChat) total() int { return len(m.ids) + len(m.emails) + len(m.names) }

func labelDMRule(actions models.JSONB) []models.RoutingRule {
	return []models.RoutingRule{makeRoutingRule(10, models.JSONB{}, actions)}
}

func labelDMUsers(users ...models.User) *minimalUserRepo {
	repo := &minimalUserRepo{byEmail: map[string]models.User{}}
	for _, u := range users {
		if u.ID == uuid.Nil {
			u.ID = uuid.New()
		}
		repo.byEmail[u.Email] = u
	}
	return repo
}

func labelDMService(chat *recordingChat, users *minimalUserRepo, actions models.JSONB) AlertService {
	svc := buildAlertServiceWithEscalation(labelDMRule(actions), &mockEscalationEngine{})
	svc.SetChatService(chat, users)
	return svc
}

func labelDMAlert(owner string) webhooks.NormalizedAlert {
	alert := makeNormalizedAlert("generic", "warning")
	alert.Labels = map[string]string{"owner": owner}
	return alert
}

func TestAlertService_DMsEmailFromLabelWhenSuppressed(t *testing.T) {
	chat := &recordingChat{}
	svc := labelDMService(chat, labelDMUsers(models.User{Email: "dev@example.com", Active: true}), models.JSONB{
		"suppress":           true,
		"notify_email_label": "owner",
	})

	alert := labelDMAlert("dev@example.com")
	alert.Title = "CI failed"
	alert.Description = "main"

	result, err := svc.ProcessNormalizedAlerts(context.Background(), []webhooks.NormalizedAlert{alert})
	if err != nil {
		t.Fatalf("ProcessNormalizedAlerts: %v", err)
	}
	if result.IncidentsCreated != 0 {
		t.Errorf("suppressed alert created %d incidents", result.IncidentsCreated)
	}
	if len(chat.emails) != 1 || chat.emails[0] != "dev@example.com" {
		t.Fatalf("emails = %v, want [dev@example.com]", chat.emails)
	}
	if len(chat.ids)+len(chat.names) != 0 {
		t.Errorf("ids = %v names = %v, want none", chat.ids, chat.names)
	}
	if chat.texts[0] != "CI failed\nmain" {
		t.Errorf("message = %q", chat.texts[0])
	}
}

func TestAlertService_UsesStoredSlackIDWithoutLookup(t *testing.T) {
	for _, slackID := range []string{"U0AJLLY3678", "W012AB3CDEF"} {
		chat := &recordingChat{}
		id := slackID
		svc := labelDMService(chat, labelDMUsers(models.User{Email: "dev@example.com", Active: true, SlackUserID: &id}), models.JSONB{"notify_email_label": "owner"})

		if _, err := svc.ProcessNormalizedAlerts(context.Background(), []webhooks.NormalizedAlert{labelDMAlert("dev@example.com")}); err != nil {
			t.Fatalf("ProcessNormalizedAlerts: %v", err)
		}
		if len(chat.ids) != 1 || chat.ids[0] != slackID {
			t.Fatalf("ids = %v, want [%s]", chat.ids, slackID)
		}
		if len(chat.emails)+len(chat.names) != 0 {
			t.Errorf("emails = %v names = %v, want none", chat.emails, chat.names)
		}
	}
}

func TestAlertService_MatchesLabelEmailCaseInsensitively(t *testing.T) {
	chat := &recordingChat{}
	svc := labelDMService(chat, labelDMUsers(models.User{Email: "dev@example.com", Active: true}), models.JSONB{"notify_email_label": "owner"})

	if _, err := svc.ProcessNormalizedAlerts(context.Background(), []webhooks.NormalizedAlert{labelDMAlert(" Dev@Example.com ")}); err != nil {
		t.Fatalf("ProcessNormalizedAlerts: %v", err)
	}
	if len(chat.emails) != 1 || chat.emails[0] != "dev@example.com" {
		t.Fatalf("emails = %v, want [dev@example.com]", chat.emails)
	}
}

func TestAlertService_SkipsDMWhenNoRegenUser(t *testing.T) {
	chat := &recordingChat{}
	svc := labelDMService(chat, labelDMUsers(), models.JSONB{"notify_email_label": "owner"})

	if _, err := svc.ProcessNormalizedAlerts(context.Background(), []webhooks.NormalizedAlert{labelDMAlert("stranger@example.com")}); err != nil {
		t.Fatalf("ProcessNormalizedAlerts: %v", err)
	}
	if chat.total() != 0 {
		t.Fatalf("DM sent to unknown address: ids=%v emails=%v names=%v", chat.ids, chat.emails, chat.names)
	}
}

func TestAlertService_SkipsDMWhenUserInactive(t *testing.T) {
	chat := &recordingChat{}
	svc := labelDMService(chat, labelDMUsers(models.User{Email: "dev@example.com", Active: false}), models.JSONB{"notify_email_label": "owner"})

	if _, err := svc.ProcessNormalizedAlerts(context.Background(), []webhooks.NormalizedAlert{labelDMAlert("dev@example.com")}); err != nil {
		t.Fatalf("ProcessNormalizedAlerts: %v", err)
	}
	if chat.total() != 0 {
		t.Fatalf("DM sent to inactive user: ids=%v emails=%v names=%v", chat.ids, chat.emails, chat.names)
	}
}

func TestAlertService_SkipsDMWhenLabelIsNotAnEmail(t *testing.T) {
	chat := &recordingChat{}
	svc := labelDMService(chat, labelDMUsers(models.User{Email: "not-an-email", Active: true}), models.JSONB{"notify_email_label": "owner"})

	if _, err := svc.ProcessNormalizedAlerts(context.Background(), []webhooks.NormalizedAlert{labelDMAlert("not-an-email")}); err != nil {
		t.Fatalf("ProcessNormalizedAlerts: %v", err)
	}
	if chat.total() != 0 {
		t.Fatalf("DM sent for non-email label: ids=%v emails=%v names=%v", chat.ids, chat.emails, chat.names)
	}
}

func TestAlertService_SlackErrorDoesNotFailProcessing(t *testing.T) {
	chat := &recordingChat{err: errors.New("slack down")}
	svc := labelDMService(chat, labelDMUsers(models.User{Email: "dev@example.com", Active: true}), models.JSONB{"notify_email_label": "owner"})

	if _, err := svc.ProcessNormalizedAlerts(context.Background(), []webhooks.NormalizedAlert{labelDMAlert("dev@example.com")}); err != nil {
		t.Fatalf("ProcessNormalizedAlerts: %v", err)
	}
	if len(chat.emails) != 1 {
		t.Fatalf("expected one DM attempt, got %d", len(chat.emails))
	}
}

func TestAlertService_RepeatAlertDoesNotDMAgain(t *testing.T) {
	chat := &recordingChat{}
	alertRepo := &mockAlertRepository{}
	svc := NewAlertService(alertRepo, &mockIncidentService{shouldCreate: true})
	svc.SetRoutingEngine(NewRoutingEngine(&mockRoutingRuleRepo{rules: labelDMRule(models.JSONB{"notify_email_label": "owner"})}))
	svc.SetChatService(chat, labelDMUsers(models.User{Email: "dev@example.com", Active: true}))

	alert := labelDMAlert("dev@example.com")
	if _, err := svc.ProcessNormalizedAlerts(context.Background(), []webhooks.NormalizedAlert{alert}); err != nil {
		t.Fatalf("first process: %v", err)
	}
	alertRepo.existingAlert = &models.Alert{Source: alert.Source, ExternalID: alert.ExternalID}
	if _, err := svc.ProcessNormalizedAlerts(context.Background(), []webhooks.NormalizedAlert{alert}); err != nil {
		t.Fatalf("second process: %v", err)
	}
	if len(chat.emails) != 1 {
		t.Fatalf("DM count = %d, want 1", len(chat.emails))
	}
}

func TestEmailFromLabel(t *testing.T) {
	labels := models.JSONB{"owner": "  Dev@Example.com  ", "name": "dev"}
	got, ok := emailFromLabel(labels, "owner")
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
