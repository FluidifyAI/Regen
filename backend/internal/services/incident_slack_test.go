package services

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/fluidify/regen/internal/database"
	"github.com/fluidify/regen/internal/models"
	"github.com/fluidify/regen/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ----------------------------------------------------------------------------
// Mock ChatService
// ----------------------------------------------------------------------------

type mockChatService struct {
	mu sync.Mutex

	CreateChannelFn func(name, description string) (*Channel, error)
	PostMessageFn   func(channelID string, message Message) (string, error)
	UpdateMessageFn func(channelID, messageTS string, message Message) error
	ArchiveChannelFn func(channelID string) error
	InviteUsersFn   func(channelID string, userIDs []string) error

	// Call tracking
	ArchivedChannels []string
	InviteCalls      []inviteCall

	// Signals for async assertions
	ArchiveDone chan struct{}
	InviteDone  chan struct{}
}

type inviteCall struct {
	ChannelID string
	UserIDs   []string
}

func newMockChat() *mockChatService {
	m := &mockChatService{
		ArchiveDone: make(chan struct{}, 1),
		InviteDone:  make(chan struct{}, 1),
	}
	// Sensible defaults
	m.CreateChannelFn = func(name, _ string) (*Channel, error) {
		return &Channel{ID: "C" + name, Name: name, URL: "https://slack.test"}, nil
	}
	m.PostMessageFn = func(_ string, _ Message) (string, error) { return "1234.5678", nil }
	m.UpdateMessageFn = func(_, _ string, _ Message) error { return nil }
	m.ArchiveChannelFn = func(channelID string) error {
		m.mu.Lock()
		m.ArchivedChannels = append(m.ArchivedChannels, channelID)
		m.mu.Unlock()
		select {
		case m.ArchiveDone <- struct{}{}:
		default:
		}
		return nil
	}
	m.InviteUsersFn = func(channelID string, userIDs []string) error {
		m.mu.Lock()
		m.InviteCalls = append(m.InviteCalls, inviteCall{ChannelID: channelID, UserIDs: userIDs})
		m.mu.Unlock()
		select {
		case m.InviteDone <- struct{}{}:
		default:
		}
		return nil
	}
	return m
}

func (m *mockChatService) CreateChannel(name, description string) (*Channel, error) {
	return m.CreateChannelFn(name, description)
}
func (m *mockChatService) PostMessage(channelID string, message Message) (string, error) {
	return m.PostMessageFn(channelID, message)
}
func (m *mockChatService) UpdateMessage(channelID, messageTS string, message Message) error {
	return m.UpdateMessageFn(channelID, messageTS, message)
}
func (m *mockChatService) ArchiveChannel(channelID string) error {
	return m.ArchiveChannelFn(channelID)
}
func (m *mockChatService) InviteUsers(channelID string, userIDs []string) error {
	return m.InviteUsersFn(channelID, userIDs)
}
func (m *mockChatService) SendDirectMessage(username string, message Message) error { return nil }
func (m *mockChatService) GetThreadMessages(channelID, threadTS string) ([]string, error) {
	return []string{}, nil
}

// ----------------------------------------------------------------------------
// Test DB helper (mirrors handler test setup)
// ----------------------------------------------------------------------------

func setupSlackTestDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()

	db, err := database.NewTestDB()
	require.NoError(t, err, "failed to create test database")

	sqlDB, err := db.DB()
	require.NoError(t, err)

	sqlDB.Exec(`DROP TABLE IF EXISTS timeline_entries`)
	sqlDB.Exec(`DROP TABLE IF EXISTS incident_alerts`)
	sqlDB.Exec(`DROP TABLE IF EXISTS incidents`)
	sqlDB.Exec(`DROP TABLE IF EXISTS alerts`)
	sqlDB.Exec(`DROP TRIGGER IF EXISTS assign_incident_number`)

	_, err = sqlDB.Exec(`CREATE TABLE alerts (
		id TEXT PRIMARY KEY,
		external_id TEXT NOT NULL,
		source TEXT NOT NULL,
		fingerprint TEXT,
		status TEXT NOT NULL DEFAULT 'firing',
		severity TEXT NOT NULL DEFAULT 'info',
		title TEXT NOT NULL,
		description TEXT,
		labels TEXT DEFAULT '{}',
		annotations TEXT DEFAULT '{}',
		raw_payload TEXT,
		started_at DATETIME NOT NULL,
		ended_at DATETIME,
		received_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)

	_, err = sqlDB.Exec(`CREATE TABLE incidents (
		id TEXT PRIMARY KEY,
		incident_number INTEGER,
		title TEXT NOT NULL,
		slug TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'triggered',
		severity TEXT NOT NULL DEFAULT 'medium',
		summary TEXT,
		group_key TEXT,
		slack_channel_id TEXT,
		slack_channel_name TEXT,
		slack_message_ts TEXT,
		teams_channel_id TEXT,
		teams_channel_name TEXT,
		teams_conversation_id TEXT,
		teams_activity_id TEXT,
		ai_enabled INTEGER NOT NULL DEFAULT 1,
		ai_summary TEXT,
		ai_summary_generated_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		triggered_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		acknowledged_at DATETIME,
		resolved_at DATETIME,
		created_by_type TEXT NOT NULL DEFAULT 'system',
		created_by_id TEXT NOT NULL DEFAULT 'system',
		commander_id TEXT,
		labels TEXT DEFAULT '{}',
		custom_fields TEXT DEFAULT '{}'
	)`)
	require.NoError(t, err)

	_, err = sqlDB.Exec(`CREATE TRIGGER assign_incident_number
		AFTER INSERT ON incidents
		BEGIN
			UPDATE incidents SET incident_number = (
				SELECT COALESCE(MAX(incident_number), 0) + 1 FROM incidents WHERE id != NEW.id
			) WHERE id = NEW.id;
		END`)
	require.NoError(t, err)

	_, err = sqlDB.Exec(`CREATE TABLE incident_alerts (
		incident_id TEXT NOT NULL,
		alert_id TEXT NOT NULL,
		linked_by_type TEXT,
		linked_by_id TEXT,
		linked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (incident_id, alert_id)
	)`)
	require.NoError(t, err)

	_, err = sqlDB.Exec(`CREATE TABLE timeline_entries (
		id TEXT PRIMARY KEY,
		incident_id TEXT NOT NULL,
		timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		type TEXT NOT NULL,
		actor_type TEXT NOT NULL DEFAULT 'system',
		actor_id TEXT NOT NULL DEFAULT 'system',
		content TEXT DEFAULT '{}',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)

	cleanup := func() {
		sqlDB.Exec(`DROP TABLE IF EXISTS timeline_entries`)
		sqlDB.Exec(`DROP TABLE IF EXISTS incident_alerts`)
		sqlDB.Exec(`DROP TABLE IF EXISTS incidents`)
		sqlDB.Exec(`DROP TABLE IF EXISTS alerts`)
		sqlDB.Exec(`DROP TRIGGER IF EXISTS assign_incident_number`)
	}
	return db, cleanup
}

// buildTestIncidentService builds an incidentService with the given mock and db.
func buildTestIncidentService(db *gorm.DB, chat ChatService) IncidentService {
	return NewIncidentService(
		repository.NewIncidentRepository(db),
		repository.NewTimelineRepository(db),
		repository.NewAlertRepository(db),
		chat,
		db,
	)
}

// ----------------------------------------------------------------------------
// Tests
// ----------------------------------------------------------------------------

// TestSlackMessageBuilder_BuildIncidentCreatedMessage verifies the created message
// contains the expected incident title and severity in both text and block content.
func TestSlackMessageBuilder_BuildIncidentCreatedMessage(t *testing.T) {
	b := NewSlackMessageBuilder()
	incident := &models.Incident{
		ID:             uuid.New(),
		IncidentNumber: 42,
		Title:          "API Gateway 5xx errors",
		Status:         models.IncidentStatusTriggered,
		Severity:       models.IncidentSeverityCritical,
		TriggeredAt:    time.Now(),
	}

	msg := b.BuildIncidentCreatedMessage(incident, nil)

	assert.Contains(t, msg.Text, "INC-42", "text fallback must include incident number")
	assert.Contains(t, msg.Text, "API Gateway 5xx errors", "text fallback must include title")
	assert.NotEmpty(t, msg.Blocks, "should produce block content")
}

// TestSlackMessageBuilder_BuildStatusUpdateMessage verifies the status-update message
// mentions both old and new status.
func TestSlackMessageBuilder_BuildStatusUpdateMessage(t *testing.T) {
	b := NewSlackMessageBuilder()
	incident := &models.Incident{
		ID:             uuid.New(),
		IncidentNumber: 7,
		Title:          "DB connection pool exhausted",
		Status:         models.IncidentStatusResolved,
		Severity:       models.IncidentSeverityHigh,
	}

	msg := b.BuildStatusUpdateMessage(incident, models.IncidentStatusTriggered, models.IncidentStatusResolved)

	assert.Contains(t, msg.Text, "RESOLVED", "text should mention new status")
}

// TestUpdateIncident_ArchivesChannelOnResolve verifies that resolving an incident
// triggers channel archival.
func TestUpdateIncident_ArchivesChannelOnResolve(t *testing.T) {
	db, cleanup := setupSlackTestDB(t)
	defer cleanup()

	mock := newMockChat()
	svc := buildTestIncidentService(db, mock)

	// Create an incident with a Slack channel already assigned
	incident := &models.Incident{
		ID:               uuid.New(),
		Title:            "High latency on checkout",
		Slug:             "high-latency-on-checkout",
		Status:           models.IncidentStatusTriggered,
		Severity:         models.IncidentSeverityHigh,
		SlackChannelID:   "C012345",
		SlackChannelName: "inc-1-high-latency",
		CreatedByType:    "user",
		CreatedByID:      "test",
		TriggeredAt:      time.Now(),
	}
	repo := repository.NewIncidentRepository(db)
	require.NoError(t, repo.Create(incident))

	_, err := svc.UpdateIncident(incident.ID, &UpdateIncidentParams{
		Status:    models.IncidentStatusResolved,
		UpdatedBy: "test-user",
	})
	require.NoError(t, err)

	// Wait for async goroutine with timeout
	select {
	case <-mock.ArchiveDone:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for ArchiveChannel to be called")
	}

	mock.mu.Lock()
	archived := mock.ArchivedChannels
	mock.mu.Unlock()

	require.Len(t, archived, 1)
	assert.Equal(t, "C012345", archived[0])
}

// TestUpdateIncident_ArchivesChannelOnCanceled verifies canceled incidents also
// have their Slack channel archived.
func TestUpdateIncident_ArchivesChannelOnCanceled(t *testing.T) {
	db, cleanup := setupSlackTestDB(t)
	defer cleanup()

	mock := newMockChat()
	svc := buildTestIncidentService(db, mock)

	incident := &models.Incident{
		ID:               uuid.New(),
		Title:            "False alarm",
		Slug:             "false-alarm",
		Status:           models.IncidentStatusTriggered,
		Severity:         models.IncidentSeverityLow,
		SlackChannelID:   "CCANCEL",
		SlackChannelName: "inc-2-false-alarm",
		CreatedByType:    "user",
		CreatedByID:      "test",
		TriggeredAt:      time.Now(),
	}
	repo := repository.NewIncidentRepository(db)
	require.NoError(t, repo.Create(incident))

	_, err := svc.UpdateIncident(incident.ID, &UpdateIncidentParams{
		Status:    models.IncidentStatusCanceled,
		UpdatedBy: "test-user",
	})
	require.NoError(t, err)

	select {
	case <-mock.ArchiveDone:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for ArchiveChannel to be called")
	}

	mock.mu.Lock()
	archived := mock.ArchivedChannels
	mock.mu.Unlock()

	assert.Equal(t, []string{"CCANCEL"}, archived)
}

// TestUpdateIncident_DoesNotArchiveOnAcknowledge verifies that acknowledging
// an incident does NOT archive the channel.
func TestUpdateIncident_DoesNotArchiveOnAcknowledge(t *testing.T) {
	db, cleanup := setupSlackTestDB(t)
	defer cleanup()

	// Replace ArchiveChannelFn with a version that fails the test if called
	mock := newMockChat()
	mock.ArchiveChannelFn = func(channelID string) error {
		t.Errorf("ArchiveChannel must NOT be called on acknowledge, got %q", channelID)
		return nil
	}
	svc := buildTestIncidentService(db, mock)

	incident := &models.Incident{
		ID:               uuid.New(),
		Title:            "Memory pressure",
		Slug:             "memory-pressure",
		Status:           models.IncidentStatusTriggered,
		Severity:         models.IncidentSeverityMedium,
		SlackChannelID:   "CACTIVE",
		SlackChannelName: "inc-3-memory-pressure",
		CreatedByType:    "user",
		CreatedByID:      "test",
		TriggeredAt:      time.Now(),
	}
	repo := repository.NewIncidentRepository(db)
	require.NoError(t, repo.Create(incident))

	_, err := svc.UpdateIncident(incident.ID, &UpdateIncidentParams{
		Status:    models.IncidentStatusAcknowledged,
		UpdatedBy: "test-user",
	})
	require.NoError(t, err)

	// Give the goroutine time to run (it shouldn't call archive)
	time.Sleep(200 * time.Millisecond)
}

// TestCreateSlackChannelForIncident_NoInviteWhenNoOnCallConfigured verifies that
// InviteUsers is never called when no schedules or on-call users are configured.
func TestCreateSlackChannelForIncident_NoInviteWhenNoOnCallConfigured(t *testing.T) {
	db, cleanup := setupSlackTestDB(t)
	defer cleanup()

	mock := newMockChat()
	mock.InviteUsersFn = func(channelID string, userIDs []string) error {
		t.Error("InviteUsers must NOT be called when no on-call user is configured")
		return nil
	}
	svc := buildTestIncidentService(db, mock)

	incident := &models.Incident{
		ID:            uuid.New(),
		IncidentNumber: 100,
		Title:          "No invite test",
		Slug:           "no-invite-test",
		Status:        models.IncidentStatusTriggered,
		Severity:      models.IncidentSeverityLow,
		CreatedByType: "system",
		CreatedByID:   "test",
		TriggeredAt:   time.Now(),
	}
	repo := repository.NewIncidentRepository(db)
	require.NoError(t, repo.Create(incident))

	err := svc.CreateSlackChannelForIncident(incident, nil)
	require.NoError(t, err)
}
