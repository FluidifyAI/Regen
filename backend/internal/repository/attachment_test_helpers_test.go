package repository_test

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var attachmentTestDBCounter uint64

// setupTestDB creates an isolated in-memory SQLite database for attachment tests.
// It creates the incidents, incident_attachments, and incident_attachment_data tables.
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	n := atomic.AddUint64(&attachmentTestDBCounter, 1)
	dsn := fmt.Sprintf("file:attachtestdb%d?mode=memory&cache=shared", n)

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("setupTestDB: open sqlite: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("setupTestDB: get sql.DB: %v", err)
	}

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS incidents (
			id TEXT PRIMARY KEY,
			incident_number INTEGER UNIQUE,
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
			created_by_type TEXT NOT NULL,
			created_by_id TEXT NOT NULL DEFAULT '',
			commander_id TEXT,
			labels TEXT DEFAULT '{}',
			custom_fields TEXT DEFAULT '{}'
		)`,
		`CREATE TRIGGER IF NOT EXISTS assign_incident_number_att
		 AFTER INSERT ON incidents
		 WHEN NEW.incident_number IS NULL
		 BEGIN
		   UPDATE incidents
		   SET incident_number = (SELECT COALESCE(MAX(incident_number), 0) + 1 FROM incidents WHERE id != NEW.id)
		   WHERE id = NEW.id;
		 END`,
		`CREATE TABLE IF NOT EXISTS incident_attachments (
			id TEXT PRIMARY KEY,
			incident_id TEXT NOT NULL REFERENCES incidents(id),
			file_name TEXT NOT NULL,
			file_size INTEGER NOT NULL,
			mime_type TEXT NOT NULL,
			uploaded_by TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS incident_attachment_data (
			attachment_id TEXT PRIMARY KEY REFERENCES incident_attachments(id),
			data BLOB NOT NULL
		)`,
	}

	for _, stmt := range stmts {
		if _, err := sqlDB.Exec(stmt); err != nil {
			t.Fatalf("setupTestDB: exec DDL: %v\nSQL: %s", err, stmt)
		}
	}

	t.Cleanup(func() { sqlDB.Close() })

	return db
}

// makeTestIncident returns a minimal Incident ready for insertion.
func makeTestIncident() *models.Incident {
	return &models.Incident{
		ID:            uuid.New(),
		Title:         "Test Incident",
		Slug:          fmt.Sprintf("test-incident-%s", uuid.New().String()[:8]),
		Status:        models.IncidentStatusTriggered,
		Severity:      models.IncidentSeverityMedium,
		CreatedByType: "user",
		CreatedByID:   "test-user",
		TriggeredAt:   time.Now(),
	}
}
