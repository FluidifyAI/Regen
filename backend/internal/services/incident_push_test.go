package services

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ─── spyPushNotifier ─────────────────────────────────────────────────────────

type pushCall struct {
	UserID uuid.UUID
	Notif  PushNotification
}

type spyPushNotifier struct {
	mu    sync.Mutex
	calls []pushCall
}

func (s *spyPushNotifier) IsEnabled() bool { return true }

func (s *spyPushNotifier) SendToUser(_ context.Context, userID uuid.UUID, n PushNotification) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, pushCall{UserID: userID, Notif: n})
	return nil
}

func (s *spyPushNotifier) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func (s *spyPushNotifier) firstCall() (pushCall, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.calls) == 0 {
		return pushCall{}, false
	}
	return s.calls[0], true
}

// Compile-time check.
var _ PushNotifier = (*spyPushNotifier)(nil)

// ─── makePushTestAlert creates a minimal alert without conflicting with escalation_engine_test ────

func makePushTestAlert() *models.Alert {
	return &models.Alert{
		ID:         uuid.New(),
		ExternalID: "push-test-alert-" + uuid.New().String()[:8],
		Source:     "prometheus",
		Status:     "firing",
		Severity:   models.AlertSeverityCritical,
		Title:      "Push Test Alert",
		Labels:     make(models.JSONB),
		StartedAt:  time.Now(),
		ReceivedAt: time.Now(),
	}
}

// ─── Fake on-call dependencies ───────────────────────────────────────────────

// pushTestOnCallDeps is a convenience struct holding all on-call fakes needed to
// make findOnCallUserID() return a specific UUID.
type pushTestOnCallDeps struct {
	userID    uuid.UUID
	userEmail string
}

// newPushTestOnCallDeps creates fakes that make findOnCallUserID() return the given UUID.
func newPushTestOnCallDeps(userID uuid.UUID) *pushTestOnCallDeps {
	return &pushTestOnCallDeps{
		userID:    userID,
		userEmail: "oncall-" + userID.String()[:8] + "@test.example",
	}
}

func (d *pushTestOnCallDeps) scheduleRepo() *minimalScheduleRepo {
	return &minimalScheduleRepo{
		schedules: []models.Schedule{
			{ID: uuid.New(), Name: "Test Schedule"},
		},
	}
}

func (d *pushTestOnCallDeps) evaluator() *minimalEvaluator {
	return &minimalEvaluator{email: d.userEmail}
}

func (d *pushTestOnCallDeps) userRepo() *minimalUserRepo {
	return &minimalUserRepo{
		byEmail: map[string]models.User{
			d.userEmail: {ID: d.userID, Email: d.userEmail, Name: "On-Call User"},
		},
	}
}

// ─── minimalScheduleRepo ─────────────────────────────────────────────────────

// minimalScheduleRepo implements repository.ScheduleRepository with all methods stubbed
// except GetAll, which returns a configurable list.
type minimalScheduleRepo struct {
	schedules []models.Schedule
}

func (r *minimalScheduleRepo) Create(_ *models.Schedule) error { return nil }
func (r *minimalScheduleRepo) GetByID(_ uuid.UUID) (*models.Schedule, error) {
	if len(r.schedules) > 0 {
		s := r.schedules[0]
		return &s, nil
	}
	return nil, nil
}
func (r *minimalScheduleRepo) GetAll() ([]models.Schedule, error) {
	return r.schedules, nil
}
func (r *minimalScheduleRepo) Update(_ *models.Schedule) error                     { return nil }
func (r *minimalScheduleRepo) Delete(_ uuid.UUID) error                            { return nil }
func (r *minimalScheduleRepo) GetWithLayers(_ uuid.UUID) (*models.Schedule, error) { return nil, nil }
func (r *minimalScheduleRepo) CreateLayer(_ *models.ScheduleLayer) error           { return nil }
func (r *minimalScheduleRepo) DeleteLayer(_ uuid.UUID) error                       { return nil }
func (r *minimalScheduleRepo) UpdateLayer(_ *models.ScheduleLayer, _ *[]models.ScheduleParticipant) error {
	return nil
}
func (r *minimalScheduleRepo) CreateParticipantsBulk(_ []models.ScheduleParticipant) error {
	return nil
}
func (r *minimalScheduleRepo) CreateOverride(_ *models.ScheduleOverride) error { return nil }
func (r *minimalScheduleRepo) DeleteOverride(_ uuid.UUID) error                { return nil }
func (r *minimalScheduleRepo) GetActiveOverrides(_ uuid.UUID, _ time.Time) ([]models.ScheduleOverride, error) {
	return nil, nil
}
func (r *minimalScheduleRepo) GetOverridesInWindow(_ uuid.UUID, _, _ time.Time) ([]models.ScheduleOverride, error) {
	return nil, nil
}
func (r *minimalScheduleRepo) ListUpcomingOverrides(_ uuid.UUID) ([]models.ScheduleOverride, error) {
	return nil, nil
}
func (r *minimalScheduleRepo) GetHolidayCountries(_ uuid.UUID) ([]string, error) { return nil, nil }
func (r *minimalScheduleRepo) SetHolidayCountries(_ uuid.UUID, _ []string) ([]string, []string, error) {
	return nil, nil, nil
}
func (r *minimalScheduleRepo) UpsertHolidays(_ []models.ScheduleHoliday) error { return nil }
func (r *minimalScheduleRepo) ListHolidays(_ uuid.UUID, _, _ time.Time) ([]models.ScheduleHoliday, error) {
	return nil, nil
}
func (r *minimalScheduleRepo) DeleteHolidaysByCountry(_ uuid.UUID, _ string) error { return nil }
func (r *minimalScheduleRepo) ListSchedulesWithHolidays() ([]models.Schedule, error) {
	return nil, nil
}
func (r *minimalScheduleRepo) CreateUnavailability(_ *models.ScheduleUnavailability) error {
	return nil
}
func (r *minimalScheduleRepo) DeleteUnavailability(_ uuid.UUID, _ uuid.UUID) error { return nil }
func (r *minimalScheduleRepo) ListUnavailabilities(_ uuid.UUID) ([]models.ScheduleUnavailability, error) {
	return nil, nil
}
func (r *minimalScheduleRepo) GetUnavailabilitiesInWindow(_ uuid.UUID, _, _ time.Time) ([]models.ScheduleUnavailability, error) {
	return nil, nil
}

// Compile-time check.
var _ repository.ScheduleRepository = (*minimalScheduleRepo)(nil)

// ─── minimalEvaluator ────────────────────────────────────────────────────────

type minimalEvaluator struct {
	email string
}

func (e *minimalEvaluator) WhoIsOnCall(_ uuid.UUID, _ time.Time) (string, error) {
	return e.email, nil
}
func (e *minimalEvaluator) GetTimeline(_ uuid.UUID, _, _ time.Time) ([]TimelineSegment, error) {
	return nil, nil
}
func (e *minimalEvaluator) GetLayerTimelines(_ uuid.UUID, _, _ time.Time) (map[uuid.UUID][]TimelineSegment, []TimelineSegment, error) {
	return nil, nil, nil
}

// Compile-time check.
var _ ScheduleEvaluator = (*minimalEvaluator)(nil)

// ─── minimalUserRepo ─────────────────────────────────────────────────────────

type minimalUserRepo struct {
	byEmail map[string]models.User
}

func (r *minimalUserRepo) GetBySubject(_ string) (*models.User, error) { return nil, nil }
func (r *minimalUserRepo) GetByEmail(email string) (*models.User, error) {
	if u, ok := r.byEmail[email]; ok {
		return &u, nil
	}
	return nil, &repository.NotFoundError{Resource: "user", ID: email}
}
func (r *minimalUserRepo) Upsert(_ context.Context, _ *models.User) error { return nil }
func (r *minimalUserRepo) UpdateLastLogin(_ uuid.UUID, _ time.Time) error { return nil }
func (r *minimalUserRepo) CreateLocal(_ *models.User) error               { return nil }
func (r *minimalUserRepo) ListAll() ([]models.User, error) {
	users := make([]models.User, 0, len(r.byEmail))
	for _, u := range r.byEmail {
		users = append(users, u)
	}
	return users, nil
}
func (r *minimalUserRepo) GetByID(id uuid.UUID) (*models.User, error) {
	// Search by ID across all known users
	for _, u := range r.byEmail {
		if u.ID == id {
			uc := u
			return &uc, nil
		}
	}
	// Return a minimal non-nil user to satisfy resolveCommanderName without panic
	u := &models.User{ID: id, Name: "Unknown", Email: ""}
	return u, nil
}
func (r *minimalUserRepo) Update(_ *models.User) error                     { return nil }
func (r *minimalUserRepo) Count() (int64, error)                           { return 0, nil }
func (r *minimalUserRepo) CountByRole(_ models.UserRole) (int64, error)    { return 0, nil }
func (r *minimalUserRepo) Deactivate(_ uuid.UUID) error                    { return nil }
func (r *minimalUserRepo) CreateAgent(_ *models.User) error                { return nil }
func (r *minimalUserRepo) SetActive(_ uuid.UUID, _ bool) error             { return nil }
func (r *minimalUserRepo) ListAgents() ([]models.User, error)              { return nil, nil }
func (r *minimalUserRepo) GetBySlackUserID(_ string) (*models.User, error) { return nil, nil }
func (r *minimalUserRepo) GetByTeamsUserID(_ string) (*models.User, error) { return nil, nil }
func (r *minimalUserRepo) RestoreAgent(_ uuid.UUID) error                  { return nil }

// Compile-time check.
var _ repository.UserRepository = (*minimalUserRepo)(nil)

// ─── Tests ────────────────────────────────────────────────────────────────────

// setupPushTestDB extends setupSlackTestDB with alert columns added in later migrations.
func setupPushTestDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()
	db, cleanup := setupSlackTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// Add escalation columns introduced in migration 000015
	sqlDB.Exec(`ALTER TABLE alerts ADD COLUMN escalation_policy_id TEXT`)
	sqlDB.Exec(`ALTER TABLE alerts ADD COLUMN acknowledgment_status TEXT NOT NULL DEFAULT 'pending'`)
	return db, cleanup
}

func TestCreateIncidentFromAlert_PushSentToOnCallUser(t *testing.T) {
	db, cleanup := setupPushTestDB(t)
	defer cleanup()

	spy := &spyPushNotifier{}
	svc := buildTestIncidentService(db, nil)

	// Inject the spy directly (same package)
	is := svc.(*incidentService)
	is.pushSvc = spy

	onCallUserID := uuid.New()
	deps := newPushTestOnCallDeps(onCallUserID)
	SetCommanderDeps(svc, deps.userRepo(), deps.scheduleRepo(), deps.evaluator())

	alert := makePushTestAlert()
	require.NoError(t, repository.NewAlertRepository(db).Create(context.Background(), alert))

	_, err := svc.CreateIncidentFromAlert(context.Background(), alert, false)
	require.NoError(t, err)

	// Push is goroutine-based; wait for it.
	time.Sleep(150 * time.Millisecond)

	assert.Equal(t, 1, spy.callCount(), "SendToUser should be called once for incident_created")
	if call, ok := spy.firstCall(); ok {
		assert.Equal(t, onCallUserID, call.UserID)
		assert.Equal(t, "incident_created", call.Notif.Data["event"])
	}
}

func TestCreateIncidentFromAlert_PushNotCalledWhenNilPushSvc(t *testing.T) {
	db, cleanup := setupPushTestDB(t)
	defer cleanup()

	svc := buildTestIncidentService(db, nil)
	// pushSvc stays nil — no spy injected

	onCallUserID := uuid.New()
	deps := newPushTestOnCallDeps(onCallUserID)
	SetCommanderDeps(svc, deps.userRepo(), deps.scheduleRepo(), deps.evaluator())

	alert := makePushTestAlert()
	require.NoError(t, repository.NewAlertRepository(db).Create(context.Background(), alert))

	_, err := svc.CreateIncidentFromAlert(context.Background(), alert, false)
	require.NoError(t, err)

	// Allow goroutine time to run
	time.Sleep(150 * time.Millisecond)
	// No assertion needed — test verifies no panic when pushSvc is nil
}

func TestCreateIncidentFromAlertWithGrouping_PushSentToOnCallUser(t *testing.T) {
	db, cleanup := setupPushTestDB(t)
	defer cleanup()

	// CreateIncidentFromAlertWithGrouping uses pg_advisory_xact_lock which is PostgreSQL-only.
	// Skip gracefully when running against SQLite.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	if _, lockErr := sqlDB.Exec("SELECT pg_advisory_xact_lock(1)"); lockErr != nil {
		t.Skip("skipping grouping test: requires PostgreSQL advisory lock support")
	}

	spy := &spyPushNotifier{}
	svc := buildTestIncidentService(db, nil)

	is := svc.(*incidentService)
	is.pushSvc = spy

	onCallUserID := uuid.New()
	deps := newPushTestOnCallDeps(onCallUserID)
	SetCommanderDeps(svc, deps.userRepo(), deps.scheduleRepo(), deps.evaluator())

	alert := makePushTestAlert()
	require.NoError(t, repository.NewAlertRepository(db).Create(context.Background(), alert))

	groupKey := "test-group-" + uuid.New().String()[:8]
	_, err = svc.CreateIncidentFromAlertWithGrouping(context.Background(), alert, groupKey, false)
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)

	assert.Equal(t, 1, spy.callCount(), "SendToUser should be called once for new grouped incident")
	if call, ok := spy.firstCall(); ok {
		assert.Equal(t, onCallUserID, call.UserID)
		assert.Equal(t, "incident_created", call.Notif.Data["event"])
	}
}

func TestUpdateIncident_ResolvedStatus_PushSentToCommander(t *testing.T) {
	db, cleanup := setupSlackTestDB(t)
	defer cleanup()

	spy := &spyPushNotifier{}
	svc := buildTestIncidentService(db, nil)

	is := svc.(*incidentService)
	is.pushSvc = spy

	commanderID := uuid.New()
	incident := &models.Incident{
		ID:            uuid.New(),
		Title:         "Test Incident",
		Slug:          "test-incident-push",
		Status:        models.IncidentStatusTriggered,
		Severity:      models.IncidentSeverityHigh,
		CommanderID:   &commanderID,
		CreatedByType: "user",
		CreatedByID:   "test",
		TriggeredAt:   time.Now(),
	}
	repo := repository.NewIncidentRepository(db)
	require.NoError(t, repo.Create(context.Background(), incident))

	_, err := svc.UpdateIncident(incident.ID, &UpdateIncidentParams{
		Status:    models.IncidentStatusResolved,
		UpdatedBy: "test-user",
	})
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)

	assert.Equal(t, 1, spy.callCount(), "SendToUser should be called for resolved incident with commander")
	if call, ok := spy.firstCall(); ok {
		assert.Equal(t, commanderID, call.UserID)
		assert.Equal(t, "incident_resolved", call.Notif.Data["event"])
	}
}

func TestUpdateIncident_ResolvedStatus_NoPushWhenNoCommander(t *testing.T) {
	db, cleanup := setupSlackTestDB(t)
	defer cleanup()

	spy := &spyPushNotifier{}
	svc := buildTestIncidentService(db, nil)

	is := svc.(*incidentService)
	is.pushSvc = spy

	incident := &models.Incident{
		ID:            uuid.New(),
		Title:         "No Commander Incident",
		Slug:          "no-commander-push",
		Status:        models.IncidentStatusTriggered,
		Severity:      models.IncidentSeverityHigh,
		CommanderID:   nil,
		CreatedByType: "user",
		CreatedByID:   "test",
		TriggeredAt:   time.Now(),
	}
	repo := repository.NewIncidentRepository(db)
	require.NoError(t, repo.Create(context.Background(), incident))

	_, err := svc.UpdateIncident(incident.ID, &UpdateIncidentParams{
		Status:    models.IncidentStatusResolved,
		UpdatedBy: "test-user",
	})
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)

	assert.Equal(t, 0, spy.callCount(), "SendToUser must NOT be called when there is no commander")
}

func TestResolveIncident_PushSentToCommander(t *testing.T) {
	db, cleanup := setupSlackTestDB(t)
	defer cleanup()

	spy := &spyPushNotifier{}
	svc := buildTestIncidentService(db, nil)

	is := svc.(*incidentService)
	is.pushSvc = spy

	commanderID := uuid.New()
	incident := &models.Incident{
		ID:            uuid.New(),
		Title:         "Resolve Test",
		Slug:          "resolve-test-push",
		Status:        models.IncidentStatusTriggered,
		Severity:      models.IncidentSeverityMedium,
		CommanderID:   &commanderID,
		CreatedByType: "user",
		CreatedByID:   "test",
		TriggeredAt:   time.Now(),
	}
	repo := repository.NewIncidentRepository(db)
	require.NoError(t, repo.Create(context.Background(), incident))

	err := svc.ResolveIncident(incident.ID, "user", "test-user")
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)

	assert.Equal(t, 1, spy.callCount(), "SendToUser should be called via ResolveIncident")
	if call, ok := spy.firstCall(); ok {
		assert.Equal(t, commanderID, call.UserID)
		assert.Equal(t, "incident_resolved", call.Notif.Data["event"])
	}
}

func TestResolveIncident_NoPushWhenPushSvcNil(t *testing.T) {
	db, cleanup := setupSlackTestDB(t)
	defer cleanup()

	svc := buildTestIncidentService(db, nil)
	// pushSvc remains nil

	commanderID := uuid.New()
	incident := &models.Incident{
		ID:            uuid.New(),
		Title:         "No Push Resolve Test",
		Slug:          "no-push-resolve-push",
		Status:        models.IncidentStatusTriggered,
		Severity:      models.IncidentSeverityMedium,
		CommanderID:   &commanderID,
		CreatedByType: "user",
		CreatedByID:   "test",
		TriggeredAt:   time.Now(),
	}
	repo := repository.NewIncidentRepository(db)
	require.NoError(t, repo.Create(context.Background(), incident))

	// Should not panic when pushSvc is nil
	err := svc.ResolveIncident(incident.ID, "user", "test-user")
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)
}
