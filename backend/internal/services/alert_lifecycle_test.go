package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/models/webhooks"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Real PostgreSQL is required to exercise advisory locks, row locks and rollback.
// Each test applies the real migrations in a private schema in an explicit test DB.
func lifecycleDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("REGEN_LIFECYCLE_TEST_DSN")
	if dsn == "" {
		t.Skip("set REGEN_LIFECYCLE_TEST_DSN to an isolated PostgreSQL test database")
	}
	config := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	admin, err := gorm.Open(postgres.Open(dsn), config)
	require.NoError(t, err)
	schema := "lifecycle_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	separator := " "
	if strings.Contains(dsn, "://") {
		separator = "?"
		if strings.Contains(dsn, "?") {
			separator = "&"
		}
	}
	db, err := gorm.Open(postgres.Open(dsn+separator+"search_path="+schema), config)
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
		require.NoError(t, admin.Exec("DROP SCHEMA "+schema+" CASCADE").Error)
		sqlAdmin, err := admin.DB()
		require.NoError(t, err)
		require.NoError(t, sqlAdmin.Close())
	})
	paths, err := filepath.Glob("../../migrations/*.up.sql")
	require.NoError(t, err)
	require.NotEmpty(t, paths)
	for _, path := range paths {
		sql, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NoError(t, db.Exec(string(sql)).Error, path)
	}
	return db
}

func lifecycleService(db *gorm.DB) AlertService {
	return NewAlertService(repository.NewAlertRepository(db), buildTestIncidentService(db, nil))
}
func deliver(t *testing.T, svc AlertService, alert webhooks.NormalizedAlert, status string) *ProcessingResult {
	t.Helper()
	alert.Status = status
	if status == "resolved" {
		now := time.Now()
		alert.EndedAt = &now
	}
	result, err := svc.ProcessNormalizedAlerts(context.Background(), []webhooks.NormalizedAlert{alert})
	require.NoError(t, err)
	return result
}
func incidentFor(t *testing.T, db *gorm.DB, alert webhooks.NormalizedAlert) *models.Incident {
	t.Helper()
	a, err := repository.NewAlertRepository(db).GetByExternalID(context.Background(), alert.Source, alert.ExternalID)
	require.NoError(t, err)
	inc, err := repository.NewIncidentRepository(db).GetIncidentByAlertID(a.ID)
	require.NoError(t, err)
	return inc
}
func assertStatus(t *testing.T, db *gorm.DB, id uuid.UUID, status models.IncidentStatus) {
	t.Helper()
	inc, err := repository.NewIncidentRepository(db).GetByID(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, status, inc.Status)
}

func TestAlertLifecycleRecurrenceAndHistory(t *testing.T) {
	db := lifecycleDB(t)
	svc := lifecycleService(db)
	alert := makeNormalizedAlert("generic", "warning")
	require.Equal(t, 1, deliver(t, svc, alert, "firing").IncidentsCreated)
	first := incidentFor(t, db, alert)
	require.Equal(t, 0, deliver(t, svc, alert, "firing").IncidentsCreated)
	deliver(t, svc, alert, "resolved")
	assertStatus(t, db, first.ID, models.IncidentStatusResolved)
	var resolved models.Incident
	require.NoError(t, db.First(&resolved, "id = ?", first.ID).Error)
	deliver(t, svc, alert, "resolved")
	var repeated models.Incident
	require.NoError(t, db.First(&repeated, "id = ?", first.ID).Error)
	require.True(t, resolved.ResolvedAt.Equal(*repeated.ResolvedAt))
	alert.StartedAt = alert.StartedAt.Add(time.Hour)
	require.Equal(t, 1, deliver(t, svc, alert, "firing").IncidentsCreated)
	second := incidentFor(t, db, alert)
	require.NotEqual(t, first.ID, second.ID)
	assertStatus(t, db, first.ID, models.IncidentStatusResolved)
	assertStatus(t, db, second.ID, models.IncidentStatusTriggered)
	alerts, err := repository.NewIncidentRepository(db).GetAlerts(first.ID)
	require.NoError(t, err)
	require.Len(t, alerts, 1)
	require.WithinDuration(t, alert.StartedAt, alerts[0].StartedAt, time.Microsecond)
	require.Nil(t, alerts[0].EndedAt)
	var links, timeline int64
	require.NoError(t, db.Table("incident_alerts").Count(&links).Error)
	require.EqualValues(t, 2, links)
	require.NoError(t, db.Model(&models.TimelineEntry{}).Where("incident_id = ? AND type = ?", first.ID, "status_changed").Count(&timeline).Error)
	require.EqualValues(t, 1, timeline)
}

func TestAlertLifecycleAllLinkedAlertsAndManualIncidents(t *testing.T) {
	db := lifecycleDB(t)
	svc := lifecycleService(db)
	a := makeNormalizedAlert("generic", "warning")
	b := makeNormalizedAlert("generic", "warning")
	deliver(t, svc, a, "firing")
	deliver(t, svc, b, "firing")
	first := incidentFor(t, db, a)
	other := incidentFor(t, db, b)
	br, err := repository.NewAlertRepository(db).GetByExternalID(context.Background(), b.Source, b.ExternalID)
	require.NoError(t, err)
	require.NoError(t, repository.NewIncidentRepository(db).LinkAlert(context.Background(), first.ID, br.ID, "system", "alertmanager"))
	manual := &models.Incident{Title: "Manual", Slug: "manual", Status: models.IncidentStatusAcknowledged, Severity: models.IncidentSeverityHigh, CreatedByType: "user", CreatedByID: "operator"}
	require.NoError(t, db.Create(manual).Error)
	require.NoError(t, repository.NewIncidentRepository(db).LinkAlert(context.Background(), manual.ID, br.ID, "user", "operator"))
	deliver(t, svc, a, "resolved")
	assertStatus(t, db, first.ID, models.IncidentStatusTriggered)
	deliver(t, svc, b, "resolved")
	assertStatus(t, db, first.ID, models.IncidentStatusResolved)
	assertStatus(t, db, other.ID, models.IncidentStatusResolved)
	assertStatus(t, db, manual.ID, models.IncidentStatusAcknowledged)
	deliver(t, svc, b, "firing")
	assertStatus(t, db, manual.ID, models.IncidentStatusAcknowledged)
	var count int64
	require.NoError(t, db.Model(&models.Incident{}).Count(&count).Error)
	require.EqualValues(t, 3, count)
}

func TestAlertLifecycleConcurrentRetries(t *testing.T) {
	db := lifecycleDB(t)
	svc := lifecycleService(db)
	alert := makeNormalizedAlert("generic", "warning")
	for _, status := range []string{"firing", "resolved", "firing", "resolved"} {
		alert.Status = status
		var wg sync.WaitGroup
		errors := make(chan error, 12)
		for i := 0; i < 12; i++ {
			wg.Add(1)
			go func(a webhooks.NormalizedAlert) {
				defer wg.Done()
				_, err := svc.ProcessNormalizedAlerts(context.Background(), []webhooks.NormalizedAlert{a})
				errors <- err
			}(alert)
		}
		wg.Wait()
		close(errors)
		for err := range errors {
			require.NoError(t, err)
		}
	}
	var alerts, incidents, links int64
	require.NoError(t, db.Model(&models.Alert{}).Count(&alerts).Error)
	require.EqualValues(t, 1, alerts)
	require.NoError(t, db.Model(&models.Incident{}).Count(&incidents).Error)
	require.EqualValues(t, 2, incidents)
	require.NoError(t, db.Table("incident_alerts").Count(&links).Error)
	require.EqualValues(t, 2, links)
}

func TestAlertLifecycleRollback(t *testing.T) {
	db := lifecycleDB(t)
	svc := lifecycleService(db)
	alert := makeNormalizedAlert("generic", "warning")
	require.NoError(t, db.Exec("ALTER TABLE timeline_entries ADD CONSTRAINT reject_test_entries CHECK (type = 'impossible')").Error)
	_, err := svc.ProcessNormalizedAlerts(context.Background(), []webhooks.NormalizedAlert{alert})
	require.Error(t, err)
	for _, table := range []string{"alerts", "incidents", "incident_alerts", "timeline_entries"} {
		var count int64
		require.NoError(t, db.Table(table).Count(&count).Error)
		require.Zero(t, count, fmt.Sprintf("partial write in %s", table))
	}
	require.NoError(t, db.Exec("ALTER TABLE timeline_entries DROP CONSTRAINT reject_test_entries").Error)
	deliver(t, svc, alert, "firing")
	incident := incidentFor(t, db, alert)
	require.NoError(t, db.Exec("ALTER TABLE timeline_entries ADD CONSTRAINT reject_resolution CHECK (type <> 'status_changed')").Error)
	alert.Status = "resolved"
	_, err = svc.ProcessNormalizedAlerts(context.Background(), []webhooks.NormalizedAlert{alert})
	require.Error(t, err)
	assertStatus(t, db, incident.ID, models.IncidentStatusTriggered)
	a, err := repository.NewAlertRepository(db).GetByExternalID(context.Background(), alert.Source, alert.ExternalID)
	require.NoError(t, err)
	require.Equal(t, models.AlertStatusFiring, a.Status)
}

func TestAlertLifecycleConcurrentGrouping(t *testing.T) {
	db := lifecycleDB(t)
	require.NoError(t, db.Exec("DELETE FROM grouping_rules").Error)
	rule := &models.GroupingRule{ID: uuid.New(), Name: "same service", Enabled: true, Priority: 1, MatchLabels: models.JSONB{}, CrossSourceLabels: models.JSONBArray{"service"}, TimeWindowSeconds: 300}
	require.NoError(t, db.Create(rule).Error)
	svc := lifecycleService(db)
	svc.SetGroupingEngine(NewGroupingEngine(repository.NewGroupingRuleRepository(db), repository.NewIncidentRepository(db), db))
	alerts := make([]webhooks.NormalizedAlert, 8)
	var wg sync.WaitGroup
	errs := make(chan error, len(alerts))
	for i := range alerts {
		alerts[i] = makeNormalizedAlert("generic", "warning")
		alerts[i].Labels = map[string]string{"service": "bridge"}
		wg.Add(1)
		go func(a webhooks.NormalizedAlert) {
			defer wg.Done()
			_, err := svc.ProcessNormalizedAlerts(context.Background(), []webhooks.NormalizedAlert{a})
			errs <- err
		}(alerts[i])
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var count int64
	require.NoError(t, db.Model(&models.Incident{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
	incident := incidentFor(t, db, alerts[0])
	for i, alert := range alerts {
		deliver(t, svc, alert, "resolved")
		if i < len(alerts)-1 {
			assertStatus(t, db, incident.ID, models.IncidentStatusTriggered)
		}
	}
	assertStatus(t, db, incident.ID, models.IncidentStatusResolved)
	deliver(t, svc, alerts[0], "firing")
	require.NotEqual(t, incident.ID, incidentFor(t, db, alerts[0]).ID)
}

func TestAlertLifecycleClosedManualAndResolvedFirst(t *testing.T) {
	db := lifecycleDB(t)
	svc := lifecycleService(db)
	alert := makeNormalizedAlert("generic", "warning")
	require.Zero(t, deliver(t, svc, alert, "resolved").IncidentsCreated)
	require.Equal(t, 1, deliver(t, svc, alert, "firing").IncidentsCreated)
	first := incidentFor(t, db, alert)
	require.NoError(t, repository.NewIncidentRepository(db).UpdateStatus(first.ID, models.IncidentStatusCanceled))
	require.Equal(t, 1, deliver(t, svc, alert, "firing").IncidentsCreated)
	require.NotEqual(t, first.ID, incidentFor(t, db, alert).ID)
	assertStatus(t, db, first.ID, models.IncidentStatusCanceled)
}
