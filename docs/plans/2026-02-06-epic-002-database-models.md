# Epic 002: Database & Models Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement complete PostgreSQL database layer with GORM, migrations, and core data models (Alert, Incident, TimelineEntry) with immutable timestamp constraints and repository pattern.

**Architecture:**
- GORM ORM with PostgreSQL driver for database operations
- golang-migrate for versioned schema migrations (up/down support)
- Repository pattern for data access layer with typed error handling
- Immutable audit trail via database triggers for timeline entries
- Server-generated timestamps enforced at database level

**Tech Stack:**
- Go 1.22
- GORM v2 (ORM)
- PostgreSQL (database)
- golang-migrate/migrate (migrations)
- google/uuid (UUID generation)
- Database triggers (immutability enforcement)

---

## Task 1: Add Database Dependencies

**Files:**
- Modify: `backend/go.mod`

**Step 1: Add GORM and PostgreSQL dependencies**

```bash
cd backend
go get -u gorm.io/gorm
go get -u gorm.io/driver/postgres
go get -u github.com/google/uuid
go get -u github.com/golang-migrate/migrate/v4
go get -u github.com/golang-migrate/migrate/v4/database/postgres
go get -u github.com/golang-migrate/migrate/v4/source/file
```

Expected: Dependencies added to go.mod and go.sum

**Step 2: Verify dependencies installed**

Run: `cd backend && go mod tidy && go mod verify`
Expected: "all modules verified"

**Step 3: Commit**

```bash
git add backend/go.mod backend/go.sum
git commit -m "feat(db): add GORM, PostgreSQL, and migration dependencies"
```

---

## Task 2: Database Configuration

**Files:**
- Create: `backend/internal/database/database.go`
- Modify: `backend/internal/config/config.go`

**Step 1: Update config to include database settings**

In `backend/internal/config/config.go`, add to the Config struct:

```go
type Config struct {
	Port        string `default:"8080"`
	Environment string `default:"development"`
	LogLevel    string `default:"info"`

	// Database
	DatabaseURL      string `default:"postgresql://openincident:secret@localhost:5432/openincident?sslmode=disable"`
	DBMaxOpenConns   int    `default:"25"`
	DBMaxIdleConns   int    `default:"5"`
	DBConnMaxLife    string `default:"5m"`

	// Redis
	RedisURL string `default:"redis://localhost:6379"`
}
```

**Step 2: Create database connection module**

Create `backend/internal/database/database.go`:

```go
package database

import (
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB holds the database connection
var DB *gorm.DB

// Config holds database configuration
type Config struct {
	URL          string
	MaxOpenConns int
	MaxIdleConns int
	ConnMaxLife  time.Duration
	LogLevel     logger.LogLevel
}

// Connect establishes database connection with retry logic
func Connect(cfg Config) (*gorm.DB, error) {
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(cfg.LogLevel),
	}

	var db *gorm.DB
	var err error

	// Retry logic on startup
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		db, err = gorm.Open(postgres.Open(cfg.URL), gormConfig)
		if err == nil {
			break
		}
		slog.Warn("failed to connect to database, retrying...",
			"attempt", i+1,
			"max_retries", maxRetries,
			"error", err,
		)
		time.Sleep(time.Second * time.Duration(i+1))
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database after %d retries: %w", maxRetries, err)
	}

	// Get underlying SQL DB for connection pool settings
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying SQL DB: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLife)

	// Ping to verify connection
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	slog.Info("database connection established",
		"max_open_conns", cfg.MaxOpenConns,
		"max_idle_conns", cfg.MaxIdleConns,
		"conn_max_life", cfg.ConnMaxLife,
	)

	DB = db
	return db, nil
}

// Close gracefully closes the database connection
func Close() error {
	if DB == nil {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	slog.Info("closing database connection")
	return sqlDB.Close()
}

// Health checks database connection health
func Health() error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	return sqlDB.Ping()
}
```

**Step 3: Test compilation**

Run: `cd backend && go build ./internal/database`
Expected: No errors

**Step 4: Commit**

```bash
git add backend/internal/config/config.go backend/internal/database/database.go
git commit -m "feat(db): add database connection with retry logic and connection pooling"
```

---

## Task 3: Migration System Setup

**Files:**
- Create: `backend/internal/database/migrate.go`
- Create: `backend/migrations/.gitkeep`

**Step 1: Create migrations directory**

```bash
mkdir -p backend/migrations
touch backend/migrations/.gitkeep
```

**Step 2: Create migration runner**

Create `backend/internal/database/migrate.go`:

```go
package database

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// RunMigrations applies all pending database migrations
func RunMigrations(db *gorm.DB, migrationsPath string) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get SQL DB: %w", err)
	}

	driver, err := postgres.WithInstance(sqlDB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsPath),
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	slog.Info("current migration state", "version", version, "dirty", dirty)

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	newVersion, _, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("failed to get new migration version: %w", err)
	}

	slog.Info("migrations applied successfully", "version", newVersion)
	return nil
}

// RollbackMigration rolls back the last migration
func RollbackMigration(db *gorm.DB, migrationsPath string) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get SQL DB: %w", err)
	}

	driver, err := postgres.WithInstance(sqlDB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsPath),
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}

	version, _, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	slog.Info("rolling back migration", "current_version", version)

	if err := m.Steps(-1); err != nil {
		return fmt.Errorf("failed to rollback migration: %w", err)
	}

	slog.Info("migration rolled back successfully")
	return nil
}
```

**Step 3: Test compilation**

Run: `cd backend && go build ./internal/database`
Expected: No errors

**Step 4: Commit**

```bash
git add backend/internal/database/migrate.go backend/migrations/.gitkeep
git commit -m "feat(db): add migration system with up/down support"
```

---

## Task 4: Alert Model and Migration

**Files:**
- Create: `backend/internal/models/alert.go`
- Create: `backend/migrations/000001_create_alerts_table.up.sql`
- Create: `backend/migrations/000001_create_alerts_table.down.sql`

**Step 1: Create Alert model**

Create `backend/internal/models/alert.go`:

```go
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Alert represents an alert from a monitoring system
type Alert struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ExternalID  string         `gorm:"type:varchar(255);not null;index:idx_alerts_source_external_id,unique" json:"external_id"`
	Source      string         `gorm:"type:varchar(50);not null;index:idx_alerts_source;index:idx_alerts_source_external_id,unique" json:"source"`
	Fingerprint string         `gorm:"type:varchar(255)" json:"fingerprint,omitempty"`
	Status      AlertStatus    `gorm:"type:varchar(20);not null;index:idx_alerts_status" json:"status"`
	Severity    AlertSeverity  `gorm:"type:varchar(20);not null;index:idx_alerts_severity" json:"severity"`
	Title       string         `gorm:"type:varchar(500);not null" json:"title"`
	Description string         `gorm:"type:text" json:"description,omitempty"`
	Labels      JSONB          `gorm:"type:jsonb;default:'{}'" json:"labels"`
	Annotations JSONB          `gorm:"type:jsonb;default:'{}'" json:"annotations"`
	RawPayload  JSONB          `gorm:"type:jsonb;not null" json:"raw_payload"`
	StartedAt   time.Time      `gorm:"not null" json:"started_at"`
	EndedAt     *time.Time     `json:"ended_at,omitempty"`
	ReceivedAt  time.Time      `gorm:"not null;default:now();index:idx_alerts_received_at" json:"received_at"`
	CreatedAt   time.Time      `gorm:"not null;default:now()" json:"created_at"`
}

// AlertStatus represents the status of an alert
type AlertStatus string

const (
	AlertStatusFiring   AlertStatus = "firing"
	AlertStatusResolved AlertStatus = "resolved"
)

// AlertSeverity represents the severity of an alert
type AlertSeverity string

const (
	AlertSeverityCritical AlertSeverity = "critical"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityInfo     AlertSeverity = "info"
)

// JSONB type for PostgreSQL JSONB fields
type JSONB map[string]interface{}

// BeforeCreate hook to set ReceivedAt if not already set (for immutability)
func (a *Alert) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	if a.ReceivedAt.IsZero() {
		a.ReceivedAt = time.Now()
	}
	return nil
}

// TableName specifies the table name for Alert
func (Alert) TableName() string {
	return "alerts"
}
```

**Step 2: Create up migration**

Create `backend/migrations/000001_create_alerts_table.up.sql`:

```sql
-- Create alerts table
CREATE TABLE IF NOT EXISTS alerts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id     VARCHAR(255) NOT NULL,
    source          VARCHAR(50) NOT NULL,
    fingerprint     VARCHAR(255),
    status          VARCHAR(20) NOT NULL CHECK (status IN ('firing', 'resolved')),
    severity        VARCHAR(20) NOT NULL CHECK (severity IN ('critical', 'warning', 'info')),
    title           VARCHAR(500) NOT NULL,
    description     TEXT,
    labels          JSONB DEFAULT '{}',
    annotations     JSONB DEFAULT '{}',
    raw_payload     JSONB NOT NULL,
    started_at      TIMESTAMPTZ NOT NULL,
    ended_at        TIMESTAMPTZ,
    received_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT alerts_source_external_id_unique UNIQUE (source, external_id)
);

-- Create indexes for common queries
CREATE INDEX IF NOT EXISTS idx_alerts_source ON alerts(source);
CREATE INDEX IF NOT EXISTS idx_alerts_status ON alerts(status);
CREATE INDEX IF NOT EXISTS idx_alerts_severity ON alerts(severity);
CREATE INDEX IF NOT EXISTS idx_alerts_received_at ON alerts(received_at);
CREATE INDEX IF NOT EXISTS idx_alerts_external_id ON alerts(external_id);

-- Ensure received_at is immutable (cannot be updated)
CREATE OR REPLACE FUNCTION prevent_received_at_update()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.received_at IS DISTINCT FROM NEW.received_at THEN
        RAISE EXCEPTION 'received_at is immutable and cannot be updated';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_prevent_received_at_update
    BEFORE UPDATE ON alerts
    FOR EACH ROW
    EXECUTE FUNCTION prevent_received_at_update();

-- Comment on table
COMMENT ON TABLE alerts IS 'Alerts received from monitoring systems (Prometheus, Grafana, etc.)';
COMMENT ON COLUMN alerts.received_at IS 'Server-generated timestamp, immutable';
COMMENT ON COLUMN alerts.external_id IS 'External identifier from source system (e.g., Prometheus fingerprint)';
```

**Step 3: Create down migration**

Create `backend/migrations/000001_create_alerts_table.down.sql`:

```sql
-- Drop trigger
DROP TRIGGER IF EXISTS trigger_prevent_received_at_update ON alerts;

-- Drop function
DROP FUNCTION IF EXISTS prevent_received_at_update();

-- Drop indexes
DROP INDEX IF EXISTS idx_alerts_external_id;
DROP INDEX IF EXISTS idx_alerts_received_at;
DROP INDEX IF EXISTS idx_alerts_severity;
DROP INDEX IF EXISTS idx_alerts_status;
DROP INDEX IF EXISTS idx_alerts_source;

-- Drop table
DROP TABLE IF EXISTS alerts;
```

**Step 4: Test compilation**

Run: `cd backend && go build ./internal/models`
Expected: No errors

**Step 5: Commit**

```bash
git add backend/internal/models/alert.go backend/migrations/000001_create_alerts_table.up.sql backend/migrations/000001_create_alerts_table.down.sql
git commit -m "feat(models): add Alert model and migration with immutable received_at"
```

---

## Task 5: Incident Model and Migration

**Files:**
- Create: `backend/internal/models/incident.go`
- Create: `backend/migrations/000002_create_incidents_table.up.sql`
- Create: `backend/migrations/000002_create_incidents_table.down.sql`

**Step 1: Create Incident model**

Create `backend/internal/models/incident.go`:

```go
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Incident represents an incident
type Incident struct {
	ID               uuid.UUID        `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	IncidentNumber   int              `gorm:"autoIncrement;unique;not null" json:"incident_number"`
	Title            string           `gorm:"type:varchar(500);not null" json:"title"`
	Slug             string           `gorm:"type:varchar(100);not null" json:"slug"`
	Status           IncidentStatus   `gorm:"type:varchar(20);not null;default:'triggered';index:idx_incidents_status" json:"status"`
	Severity         IncidentSeverity `gorm:"type:varchar(20);not null;default:'medium';index:idx_incidents_severity" json:"severity"`
	Summary          string           `gorm:"type:text" json:"summary,omitempty"`

	// Slack integration
	SlackChannelID   string           `gorm:"type:varchar(50)" json:"slack_channel_id,omitempty"`
	SlackChannelName string           `gorm:"type:varchar(100)" json:"slack_channel_name,omitempty"`

	// Timestamps (created_at and triggered_at are immutable)
	CreatedAt        time.Time        `gorm:"not null;default:now()" json:"created_at"`
	TriggeredAt      time.Time        `gorm:"not null;default:now();index:idx_incidents_triggered_at" json:"triggered_at"`
	AcknowledgedAt   *time.Time       `json:"acknowledged_at,omitempty"`
	ResolvedAt       *time.Time       `json:"resolved_at,omitempty"`

	// Ownership
	CreatedByType    string           `gorm:"type:varchar(20);not null" json:"created_by_type"`
	CreatedByID      string           `gorm:"type:varchar(100)" json:"created_by_id,omitempty"`
	CommanderID      *uuid.UUID       `gorm:"type:uuid" json:"commander_id,omitempty"`

	// Metadata
	Labels           JSONB            `gorm:"type:jsonb;default:'{}'" json:"labels"`
	CustomFields     JSONB            `gorm:"type:jsonb;default:'{}'" json:"custom_fields"`

	// Relationships (not in database, loaded via joins)
	Alerts           []Alert          `gorm:"many2many:incident_alerts;" json:"alerts,omitempty"`
	TimelineEntries  []TimelineEntry  `gorm:"foreignKey:IncidentID" json:"timeline_entries,omitempty"`
}

// IncidentStatus represents the status of an incident
type IncidentStatus string

const (
	IncidentStatusTriggered    IncidentStatus = "triggered"
	IncidentStatusAcknowledged IncidentStatus = "acknowledged"
	IncidentStatusResolved     IncidentStatus = "resolved"
	IncidentStatusCanceled     IncidentStatus = "canceled"
)

// IncidentSeverity represents the severity of an incident
type IncidentSeverity string

const (
	IncidentSeverityCritical IncidentSeverity = "critical"
	IncidentSeverityHigh     IncidentSeverity = "high"
	IncidentSeverityMedium   IncidentSeverity = "medium"
	IncidentSeverityLow      IncidentSeverity = "low"
)

// BeforeCreate hook
func (i *Incident) BeforeCreate(tx *gorm.DB) error {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	if i.TriggeredAt.IsZero() {
		i.TriggeredAt = time.Now()
	}
	return nil
}

// TableName specifies the table name for Incident
func (Incident) TableName() string {
	return "incidents"
}
```

**Step 2: Create up migration**

Create `backend/migrations/000002_create_incidents_table.up.sql`:

```sql
-- Create incidents table
CREATE TABLE IF NOT EXISTS incidents (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_number     SERIAL UNIQUE NOT NULL,
    title               VARCHAR(500) NOT NULL,
    slug                VARCHAR(100) NOT NULL,
    status              VARCHAR(20) NOT NULL DEFAULT 'triggered' CHECK (status IN ('triggered', 'acknowledged', 'resolved', 'canceled')),
    severity            VARCHAR(20) NOT NULL DEFAULT 'medium' CHECK (severity IN ('critical', 'high', 'medium', 'low')),
    summary             TEXT,

    -- Slack
    slack_channel_id    VARCHAR(50),
    slack_channel_name  VARCHAR(100),

    -- Timestamps
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    triggered_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    acknowledged_at     TIMESTAMPTZ,
    resolved_at         TIMESTAMPTZ,

    -- Ownership
    created_by_type     VARCHAR(20) NOT NULL CHECK (created_by_type IN ('system', 'user')),
    created_by_id       VARCHAR(100),
    commander_id        UUID,

    -- Metadata
    labels              JSONB DEFAULT '{}',
    custom_fields       JSONB DEFAULT '{}'
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_incidents_status ON incidents(status);
CREATE INDEX IF NOT EXISTS idx_incidents_severity ON incidents(severity);
CREATE INDEX IF NOT EXISTS idx_incidents_triggered_at ON incidents(triggered_at);
CREATE INDEX IF NOT EXISTS idx_incidents_incident_number ON incidents(incident_number);

-- Ensure created_at and triggered_at are immutable
CREATE OR REPLACE FUNCTION prevent_incident_timestamp_update()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.created_at IS DISTINCT FROM NEW.created_at THEN
        RAISE EXCEPTION 'created_at is immutable and cannot be updated';
    END IF;
    IF OLD.triggered_at IS DISTINCT FROM NEW.triggered_at THEN
        RAISE EXCEPTION 'triggered_at is immutable and cannot be updated';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_prevent_incident_timestamp_update
    BEFORE UPDATE ON incidents
    FOR EACH ROW
    EXECUTE FUNCTION prevent_incident_timestamp_update();

-- Comments
COMMENT ON TABLE incidents IS 'Incidents tracked in the system';
COMMENT ON COLUMN incidents.incident_number IS 'Auto-incrementing human-readable incident number';
COMMENT ON COLUMN incidents.created_at IS 'Server-generated timestamp, immutable';
COMMENT ON COLUMN incidents.triggered_at IS 'Server-generated timestamp, immutable';
```

**Step 3: Create down migration**

Create `backend/migrations/000002_create_incidents_table.down.sql`:

```sql
-- Drop trigger
DROP TRIGGER IF EXISTS trigger_prevent_incident_timestamp_update ON incidents;

-- Drop function
DROP FUNCTION IF EXISTS prevent_incident_timestamp_update();

-- Drop indexes
DROP INDEX IF EXISTS idx_incidents_incident_number;
DROP INDEX IF EXISTS idx_incidents_triggered_at;
DROP INDEX IF EXISTS idx_incidents_severity;
DROP INDEX IF EXISTS idx_incidents_status;

-- Drop table
DROP TABLE IF EXISTS incidents;
```

**Step 4: Test compilation**

Run: `cd backend && go build ./internal/models`
Expected: No errors

**Step 5: Commit**

```bash
git add backend/internal/models/incident.go backend/migrations/000002_create_incidents_table.up.sql backend/migrations/000002_create_incidents_table.down.sql
git commit -m "feat(models): add Incident model with auto-incrementing number and immutable timestamps"
```

---

## Task 6: IncidentAlert Join Table Migration

**Files:**
- Create: `backend/migrations/000003_create_incident_alerts_table.up.sql`
- Create: `backend/migrations/000003_create_incident_alerts_table.down.sql`

**Step 1: Create up migration**

Create `backend/migrations/000003_create_incident_alerts_table.up.sql`:

```sql
-- Create incident_alerts join table
CREATE TABLE IF NOT EXISTS incident_alerts (
    incident_id     UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    alert_id        UUID NOT NULL REFERENCES alerts(id) ON DELETE CASCADE,
    linked_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    linked_by_type  VARCHAR(20) NOT NULL CHECK (linked_by_type IN ('system', 'user')),
    linked_by_id    VARCHAR(100),

    PRIMARY KEY (incident_id, alert_id)
);

-- Create indexes for common queries
CREATE INDEX IF NOT EXISTS idx_incident_alerts_incident_id ON incident_alerts(incident_id);
CREATE INDEX IF NOT EXISTS idx_incident_alerts_alert_id ON incident_alerts(alert_id);

-- Comments
COMMENT ON TABLE incident_alerts IS 'Many-to-many relationship between incidents and alerts';
```

**Step 2: Create down migration**

Create `backend/migrations/000003_create_incident_alerts_table.down.sql`:

```sql
-- Drop indexes
DROP INDEX IF EXISTS idx_incident_alerts_alert_id;
DROP INDEX IF EXISTS idx_incident_alerts_incident_id;

-- Drop table
DROP TABLE IF EXISTS incident_alerts;
```

**Step 3: Commit**

```bash
git add backend/migrations/000003_create_incident_alerts_table.up.sql backend/migrations/000003_create_incident_alerts_table.down.sql
git commit -m "feat(migrations): add incident_alerts join table for many-to-many relationship"
```

---

## Task 7: TimelineEntry Model and Migration

**Files:**
- Create: `backend/internal/models/timeline.go`
- Create: `backend/migrations/000004_create_timeline_entries_table.up.sql`
- Create: `backend/migrations/000004_create_timeline_entries_table.down.sql`

**Step 1: Create TimelineEntry model**

Create `backend/internal/models/timeline.go`:

```go
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TimelineEntry represents an immutable timeline entry for an incident
type TimelineEntry struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	IncidentID uuid.UUID `gorm:"type:uuid;not null;index:idx_timeline_incident_timestamp" json:"incident_id"`
	Timestamp  time.Time `gorm:"not null;default:now();index:idx_timeline_incident_timestamp" json:"timestamp"`
	Type       string    `gorm:"type:varchar(50);not null" json:"type"`
	ActorType  string    `gorm:"type:varchar(20);not null" json:"actor_type"`
	ActorID    string    `gorm:"type:varchar(100)" json:"actor_id,omitempty"`
	Content    JSONB     `gorm:"type:jsonb;not null" json:"content"`
	CreatedAt  time.Time `gorm:"not null;default:now()" json:"created_at"`

	// Relationship
	Incident   *Incident `gorm:"foreignKey:IncidentID" json:"incident,omitempty"`
}

// TimelineEntryType constants
const (
	TimelineTypeIncidentCreated   = "incident_created"
	TimelineTypeStatusChanged     = "status_changed"
	TimelineTypeSeverityChanged   = "severity_changed"
	TimelineTypeAlertLinked       = "alert_linked"
	TimelineTypeMessage           = "message"
	TimelineTypeResponderAdded    = "responder_added"
	TimelineTypeEscalated         = "escalated"
	TimelineTypeSummaryGenerated  = "summary_generated"
	TimelineTypePostmortemCreated = "postmortem_created"
)

// BeforeCreate hook
func (t *TimelineEntry) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	if t.Timestamp.IsZero() {
		t.Timestamp = time.Now()
	}
	return nil
}

// TableName specifies the table name for TimelineEntry
func (TimelineEntry) TableName() string {
	return "timeline_entries"
}
```

**Step 2: Create up migration with immutability triggers**

Create `backend/migrations/000004_create_timeline_entries_table.up.sql`:

```sql
-- Create timeline_entries table
CREATE TABLE IF NOT EXISTS timeline_entries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id     UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    type            VARCHAR(50) NOT NULL,
    actor_type      VARCHAR(20) NOT NULL CHECK (actor_type IN ('user', 'system', 'slack_bot')),
    actor_id        VARCHAR(100),
    content         JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_timeline_incident_id ON timeline_entries(incident_id);
CREATE INDEX IF NOT EXISTS idx_timeline_incident_timestamp ON timeline_entries(incident_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_timeline_type ON timeline_entries(type);

-- Prevent UPDATE operations on timeline_entries (immutable audit log)
CREATE OR REPLACE FUNCTION prevent_timeline_update()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'timeline_entries are immutable and cannot be updated';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_prevent_timeline_update
    BEFORE UPDATE ON timeline_entries
    FOR EACH ROW
    EXECUTE FUNCTION prevent_timeline_update();

-- Prevent DELETE operations on timeline_entries (immutable audit log)
CREATE OR REPLACE FUNCTION prevent_timeline_delete()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'timeline_entries are immutable and cannot be deleted';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_prevent_timeline_delete
    BEFORE DELETE ON timeline_entries
    FOR EACH ROW
    EXECUTE FUNCTION prevent_timeline_delete();

-- Ensure timestamp and created_at are immutable (redundant since UPDATE is prevented, but explicit)
CREATE OR REPLACE FUNCTION prevent_timeline_timestamp_update()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.timestamp IS DISTINCT FROM NEW.timestamp THEN
        RAISE EXCEPTION 'timestamp is immutable and cannot be updated';
    END IF;
    IF OLD.created_at IS DISTINCT FROM NEW.created_at THEN
        RAISE EXCEPTION 'created_at is immutable and cannot be updated';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Comments
COMMENT ON TABLE timeline_entries IS 'Immutable audit log of incident timeline events - cannot be updated or deleted';
COMMENT ON COLUMN timeline_entries.timestamp IS 'Server-generated timestamp, immutable';
COMMENT ON COLUMN timeline_entries.created_at IS 'Server-generated timestamp, immutable';
```

**Step 3: Create down migration**

Create `backend/migrations/000004_create_timeline_entries_table.down.sql`:

```sql
-- Drop triggers
DROP TRIGGER IF EXISTS trigger_prevent_timeline_delete ON timeline_entries;
DROP TRIGGER IF EXISTS trigger_prevent_timeline_update ON timeline_entries;

-- Drop functions
DROP FUNCTION IF EXISTS prevent_timeline_timestamp_update();
DROP FUNCTION IF EXISTS prevent_timeline_delete();
DROP FUNCTION IF EXISTS prevent_timeline_update();

-- Drop indexes
DROP INDEX IF EXISTS idx_timeline_type;
DROP INDEX IF EXISTS idx_timeline_incident_timestamp;
DROP INDEX IF EXISTS idx_timeline_incident_id;

-- Drop table
DROP TABLE IF EXISTS timeline_entries;
```

**Step 4: Test compilation**

Run: `cd backend && go build ./internal/models`
Expected: No errors

**Step 5: Commit**

```bash
git add backend/internal/models/timeline.go backend/migrations/000004_create_timeline_entries_table.up.sql backend/migrations/000004_create_timeline_entries_table.down.sql
git commit -m "feat(models): add TimelineEntry model with database triggers preventing updates/deletes"
```

---

## Task 8: Repository Error Types

**Files:**
- Create: `backend/internal/repository/errors.go`

**Step 1: Create typed errors for repository layer**

Create `backend/internal/repository/errors.go`:

```go
package repository

import (
	"errors"
	"fmt"
)

// Common repository errors
var (
	ErrNotFound      = errors.New("resource not found")
	ErrAlreadyExists = errors.New("resource already exists")
	ErrInvalidInput  = errors.New("invalid input")
	ErrDatabase      = errors.New("database error")
)

// NotFoundError represents a resource not found error
type NotFoundError struct {
	Resource string
	ID       interface{}
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found: %v", e.Resource, e.ID)
}

func (e *NotFoundError) Is(target error) bool {
	return target == ErrNotFound
}

// AlreadyExistsError represents a duplicate resource error
type AlreadyExistsError struct {
	Resource string
	Field    string
	Value    interface{}
}

func (e *AlreadyExistsError) Error() string {
	return fmt.Sprintf("%s already exists with %s: %v", e.Resource, e.Field, e.Value)
}

func (e *AlreadyExistsError) Is(target error) bool {
	return target == ErrAlreadyExists
}

// ValidationError represents invalid input error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

func (e *ValidationError) Is(target error) bool {
	return target == ErrInvalidInput
}

// DatabaseError wraps database errors
type DatabaseError struct {
	Op  string
	Err error
}

func (e *DatabaseError) Error() string {
	return fmt.Sprintf("database error during %s: %v", e.Op, e.Err)
}

func (e *DatabaseError) Is(target error) bool {
	return target == ErrDatabase
}

func (e *DatabaseError) Unwrap() error {
	return e.Err
}
```

**Step 2: Test compilation**

Run: `cd backend && go build ./internal/repository`
Expected: No errors

**Step 3: Commit**

```bash
git add backend/internal/repository/errors.go
git commit -m "feat(repository): add typed error handling for repository layer"
```

---

## Task 9: Alert Repository

**Files:**
- Create: `backend/internal/repository/alert_repository.go`

**Step 1: Create Alert repository interface and implementation**

Create `backend/internal/repository/alert_repository.go`:

```go
package repository

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/models"
	"gorm.io/gorm"
)

// AlertRepository defines the interface for alert data access
type AlertRepository interface {
	Create(alert *models.Alert) error
	GetByID(id uuid.UUID) (*models.Alert, error)
	GetByExternalID(source, externalID string) (*models.Alert, error)
	List(filters AlertFilters, pagination Pagination) ([]models.Alert, int64, error)
	Update(alert *models.Alert) error
}

// AlertFilters holds filter options for listing alerts
type AlertFilters struct {
	Source   string
	Status   models.AlertStatus
	Severity models.AlertSeverity
}

// Pagination holds pagination parameters
type Pagination struct {
	Page     int
	PageSize int
}

// alertRepository implements AlertRepository
type alertRepository struct {
	db *gorm.DB
}

// NewAlertRepository creates a new alert repository
func NewAlertRepository(db *gorm.DB) AlertRepository {
	return &alertRepository{db: db}
}

// Create inserts a new alert
func (r *alertRepository) Create(alert *models.Alert) error {
	if err := r.db.Create(alert).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return &AlreadyExistsError{
				Resource: "alert",
				Field:    "source,external_id",
				Value:    fmt.Sprintf("%s,%s", alert.Source, alert.ExternalID),
			}
		}
		return &DatabaseError{Op: "create alert", Err: err}
	}
	return nil
}

// GetByID retrieves an alert by UUID
func (r *alertRepository) GetByID(id uuid.UUID) (*models.Alert, error) {
	var alert models.Alert
	if err := r.db.Where("id = ?", id).First(&alert).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &NotFoundError{Resource: "alert", ID: id}
		}
		return nil, &DatabaseError{Op: "get alert by id", Err: err}
	}
	return &alert, nil
}

// GetByExternalID retrieves an alert by source and external ID
func (r *alertRepository) GetByExternalID(source, externalID string) (*models.Alert, error) {
	var alert models.Alert
	if err := r.db.Where("source = ? AND external_id = ?", source, externalID).First(&alert).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &NotFoundError{
				Resource: "alert",
				ID:       fmt.Sprintf("source=%s,external_id=%s", source, externalID),
			}
		}
		return nil, &DatabaseError{Op: "get alert by external id", Err: err}
	}
	return &alert, nil
}

// List retrieves alerts with filtering and pagination
func (r *alertRepository) List(filters AlertFilters, pagination Pagination) ([]models.Alert, int64, error) {
	var alerts []models.Alert
	var total int64

	query := r.db.Model(&models.Alert{})

	// Apply filters
	if filters.Source != "" {
		query = query.Where("source = ?", filters.Source)
	}
	if filters.Status != "" {
		query = query.Where("status = ?", filters.Status)
	}
	if filters.Severity != "" {
		query = query.Where("severity = ?", filters.Severity)
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, &DatabaseError{Op: "count alerts", Err: err}
	}

	// Apply pagination
	offset := (pagination.Page - 1) * pagination.PageSize
	query = query.Offset(offset).Limit(pagination.PageSize)

	// Order by received_at descending
	query = query.Order("received_at DESC")

	if err := query.Find(&alerts).Error; err != nil {
		return nil, 0, &DatabaseError{Op: "list alerts", Err: err}
	}

	return alerts, total, nil
}

// Update updates mutable fields of an alert
func (r *alertRepository) Update(alert *models.Alert) error {
	// Only allow updating specific mutable fields
	updates := map[string]interface{}{
		"status":      alert.Status,
		"title":       alert.Title,
		"description": alert.Description,
		"ended_at":    alert.EndedAt,
	}

	if err := r.db.Model(alert).Updates(updates).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &NotFoundError{Resource: "alert", ID: alert.ID}
		}
		return &DatabaseError{Op: "update alert", Err: err}
	}

	return nil
}
```

**Step 2: Test compilation**

Run: `cd backend && go build ./internal/repository`
Expected: No errors

**Step 3: Commit**

```bash
git add backend/internal/repository/alert_repository.go
git commit -m "feat(repository): implement Alert repository with CRUD operations"
```

---

## Task 10: Incident Repository

**Files:**
- Create: `backend/internal/repository/incident_repository.go`

**Step 1: Create Incident repository interface and implementation**

Create `backend/internal/repository/incident_repository.go`:

```go
package repository

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/models"
	"gorm.io/gorm"
)

// IncidentRepository defines the interface for incident data access
type IncidentRepository interface {
	Create(incident *models.Incident) error
	GetByID(id uuid.UUID) (*models.Incident, error)
	GetByNumber(number int) (*models.Incident, error)
	List(filters IncidentFilters, pagination Pagination) ([]models.Incident, int64, error)
	Update(incident *models.Incident) error
	UpdateStatus(id uuid.UUID, status models.IncidentStatus) error
	LinkAlert(incidentID, alertID uuid.UUID, linkedByType, linkedByID string) error
	GetAlerts(incidentID uuid.UUID) ([]models.Alert, error)
}

// IncidentFilters holds filter options for listing incidents
type IncidentFilters struct {
	Status    models.IncidentStatus
	Severity  models.IncidentSeverity
	StartDate *time.Time
	EndDate   *time.Time
}

// incidentRepository implements IncidentRepository
type incidentRepository struct {
	db *gorm.DB
}

// NewIncidentRepository creates a new incident repository
func NewIncidentRepository(db *gorm.DB) IncidentRepository {
	return &incidentRepository{db: db}
}

// Create inserts a new incident and returns it with the generated incident_number
func (r *incidentRepository) Create(incident *models.Incident) error {
	if err := r.db.Create(incident).Error; err != nil {
		return &DatabaseError{Op: "create incident", Err: err}
	}
	return nil
}

// GetByID retrieves an incident by UUID
func (r *incidentRepository) GetByID(id uuid.UUID) (*models.Incident, error) {
	var incident models.Incident
	if err := r.db.Where("id = ?", id).First(&incident).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &NotFoundError{Resource: "incident", ID: id}
		}
		return nil, &DatabaseError{Op: "get incident by id", Err: err}
	}
	return &incident, nil
}

// GetByNumber retrieves an incident by incident_number
func (r *incidentRepository) GetByNumber(number int) (*models.Incident, error) {
	var incident models.Incident
	if err := r.db.Where("incident_number = ?", number).First(&incident).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &NotFoundError{Resource: "incident", ID: number}
		}
		return nil, &DatabaseError{Op: "get incident by number", Err: err}
	}
	return &incident, nil
}

// List retrieves incidents with filtering and pagination
func (r *incidentRepository) List(filters IncidentFilters, pagination Pagination) ([]models.Incident, int64, error) {
	var incidents []models.Incident
	var total int64

	query := r.db.Model(&models.Incident{})

	// Apply filters
	if filters.Status != "" {
		query = query.Where("status = ?", filters.Status)
	}
	if filters.Severity != "" {
		query = query.Where("severity = ?", filters.Severity)
	}
	if filters.StartDate != nil {
		query = query.Where("triggered_at >= ?", filters.StartDate)
	}
	if filters.EndDate != nil {
		query = query.Where("triggered_at <= ?", filters.EndDate)
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, &DatabaseError{Op: "count incidents", Err: err}
	}

	// Apply pagination
	offset := (pagination.Page - 1) * pagination.PageSize
	query = query.Offset(offset).Limit(pagination.PageSize)

	// Order by triggered_at descending
	query = query.Order("triggered_at DESC")

	if err := query.Find(&incidents).Error; err != nil {
		return nil, 0, &DatabaseError{Op: "list incidents", Err: err}
	}

	return incidents, total, nil
}

// Update updates mutable fields of an incident
func (r *incidentRepository) Update(incident *models.Incident) error {
	// Only update mutable fields
	updates := map[string]interface{}{
		"title":              incident.Title,
		"slug":               incident.Slug,
		"status":             incident.Status,
		"severity":           incident.Severity,
		"summary":            incident.Summary,
		"slack_channel_id":   incident.SlackChannelID,
		"slack_channel_name": incident.SlackChannelName,
		"acknowledged_at":    incident.AcknowledgedAt,
		"resolved_at":        incident.ResolvedAt,
		"commander_id":       incident.CommanderID,
		"labels":             incident.Labels,
		"custom_fields":      incident.CustomFields,
	}

	if err := r.db.Model(incident).Updates(updates).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &NotFoundError{Resource: "incident", ID: incident.ID}
		}
		return &DatabaseError{Op: "update incident", Err: err}
	}

	return nil
}

// UpdateStatus updates the incident status and sets appropriate timestamps
func (r *incidentRepository) UpdateStatus(id uuid.UUID, status models.IncidentStatus) error {
	updates := map[string]interface{}{
		"status": status,
	}

	// Set timestamps based on status transition
	now := time.Now()
	switch status {
	case models.IncidentStatusAcknowledged:
		updates["acknowledged_at"] = now
	case models.IncidentStatusResolved, models.IncidentStatusCanceled:
		updates["resolved_at"] = now
	}

	if err := r.db.Model(&models.Incident{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &NotFoundError{Resource: "incident", ID: id}
		}
		return &DatabaseError{Op: "update incident status", Err: err}
	}

	return nil
}

// LinkAlert creates an incident_alert association
func (r *incidentRepository) LinkAlert(incidentID, alertID uuid.UUID, linkedByType, linkedByID string) error {
	link := map[string]interface{}{
		"incident_id":    incidentID,
		"alert_id":       alertID,
		"linked_by_type": linkedByType,
		"linked_by_id":   linkedByID,
	}

	if err := r.db.Table("incident_alerts").Create(link).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return &AlreadyExistsError{
				Resource: "incident_alert",
				Field:    "incident_id,alert_id",
				Value:    "",
			}
		}
		return &DatabaseError{Op: "link alert to incident", Err: err}
	}

	return nil
}

// GetAlerts retrieves all alerts linked to an incident
func (r *incidentRepository) GetAlerts(incidentID uuid.UUID) ([]models.Alert, error) {
	var alerts []models.Alert

	if err := r.db.
		Joins("JOIN incident_alerts ON incident_alerts.alert_id = alerts.id").
		Where("incident_alerts.incident_id = ?", incidentID).
		Order("alerts.received_at ASC").
		Find(&alerts).Error; err != nil {
		return nil, &DatabaseError{Op: "get incident alerts", Err: err}
	}

	return alerts, nil
}
```

**Step 2: Test compilation**

Run: `cd backend && go build ./internal/repository`
Expected: No errors

**Step 3: Commit**

```bash
git add backend/internal/repository/incident_repository.go
git commit -m "feat(repository): implement Incident repository with status management and alert linking"
```

---

## Task 11: TimelineEntry Repository

**Files:**
- Create: `backend/internal/repository/timeline_repository.go`

**Step 1: Create TimelineEntry repository (append-only)**

Create `backend/internal/repository/timeline_repository.go`:

```go
package repository

import (
	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/models"
	"gorm.io/gorm"
)

// TimelineRepository defines the interface for timeline entry data access
type TimelineRepository interface {
	Create(entry *models.TimelineEntry) error
	CreateBulk(entries []models.TimelineEntry) error
	GetByIncidentID(incidentID uuid.UUID, pagination Pagination) ([]models.TimelineEntry, int64, error)
}

// timelineRepository implements TimelineRepository
type timelineRepository struct {
	db *gorm.DB
}

// NewTimelineRepository creates a new timeline repository
func NewTimelineRepository(db *gorm.DB) TimelineRepository {
	return &timelineRepository{db: db}
}

// Create inserts a new timeline entry (append-only)
func (r *timelineRepository) Create(entry *models.TimelineEntry) error {
	if err := r.db.Create(entry).Error; err != nil {
		return &DatabaseError{Op: "create timeline entry", Err: err}
	}
	return nil
}

// CreateBulk inserts multiple timeline entries (bulk import)
func (r *timelineRepository) CreateBulk(entries []models.TimelineEntry) error {
	if len(entries) == 0 {
		return nil
	}

	if err := r.db.CreateInBatches(entries, 100).Error; err != nil {
		return &DatabaseError{Op: "bulk create timeline entries", Err: err}
	}

	return nil
}

// GetByIncidentID retrieves timeline entries for an incident, ordered by timestamp
func (r *timelineRepository) GetByIncidentID(incidentID uuid.UUID, pagination Pagination) ([]models.TimelineEntry, int64, error) {
	var entries []models.TimelineEntry
	var total int64

	query := r.db.Model(&models.TimelineEntry{}).Where("incident_id = ?", incidentID)

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, &DatabaseError{Op: "count timeline entries", Err: err}
	}

	// Apply pagination
	offset := (pagination.Page - 1) * pagination.PageSize
	query = query.Offset(offset).Limit(pagination.PageSize)

	// Order by timestamp ascending (chronological)
	query = query.Order("timestamp ASC")

	if err := query.Find(&entries).Error; err != nil {
		return nil, 0, &DatabaseError{Op: "get timeline entries", Err: err}
	}

	return entries, total, nil
}
```

**Step 2: Test compilation**

Run: `cd backend && go build ./internal/repository`
Expected: No errors

**Step 3: Commit**

```bash
git add backend/internal/repository/timeline_repository.go
git commit -m "feat(repository): implement TimelineEntry append-only repository"
```

---

## Task 12: Integration with Main Application

**Files:**
- Modify: `backend/cmd/openincident/main.go`

**Step 1: Read current main.go**

Run: `cd backend && cat cmd/openincident/main.go`

**Step 2: Update main.go to initialize database and run migrations**

Update `backend/cmd/openincident/main.go`:

```go
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/openincident/openincident/internal/api"
	"github.com/openincident/openincident/internal/config"
	"github.com/openincident/openincident/internal/database"
	"gorm.io/gorm/logger"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		slog.Warn("no .env file found")
	}

	// Load configuration
	cfg := config.Load()

	// Setup structured logging
	setupLogging(cfg.LogLevel)

	slog.Info("starting OpenIncident",
		"version", "0.1.0",
		"environment", cfg.Environment,
		"port", cfg.Port,
	)

	// Connect to database
	dbLogLevel := logger.Info
	if cfg.Environment == "production" {
		dbLogLevel = logger.Warn
	}

	dbConfig := database.Config{
		URL:          cfg.DatabaseURL,
		MaxOpenConns: cfg.DBMaxOpenConns,
		MaxIdleConns: cfg.DBMaxIdleConns,
		ConnMaxLife:  parseDuration(cfg.DBConnMaxLife, 5*time.Minute),
		LogLevel:     dbLogLevel,
	}

	db, err := database.Connect(dbConfig)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	// Run migrations
	slog.Info("running database migrations...")
	if err := database.RunMigrations(db, "./migrations"); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// Setup Gin
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Setup routes
	api.SetupRoutes(router, db)

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		slog.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	slog.Info("server exited")
}

func setupLogging(level string) {
	var logLevel slog.Level

	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})

	slog.SetDefault(slog.New(handler))
}

func parseDuration(s string, defaultDuration time.Duration) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return defaultDuration
	}
	return d
}
```

**Step 3: Update routes to accept database connection**

Update `backend/internal/api/routes.go` to accept the database:

```go
package api

import (
	"github.com/gin-gonic/gin"
	"github.com/openincident/openincident/internal/api/handlers"
	"github.com/openincident/openincident/internal/api/middleware"
	"gorm.io/gorm"
)

// SetupRoutes configures all application routes
func SetupRoutes(router *gin.Engine, db *gorm.DB) {
	// Middleware
	router.Use(middleware.CORS())
	router.Use(middleware.Recovery())
	router.Use(middleware.Logging())

	// Health check endpoints
	router.GET("/health", handlers.Health(db))
	router.GET("/ready", handlers.Ready(db))

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Webhooks (to be implemented in future tasks)
		webhooks := v1.Group("/webhooks")
		{
			webhooks.POST("/prometheus", func(c *gin.Context) {
				c.JSON(501, gin.H{"error": "not implemented"})
			})
			webhooks.POST("/grafana", func(c *gin.Context) {
				c.JSON(501, gin.H{"error": "not implemented"})
			})
		}

		// Incidents (to be implemented)
		v1.GET("/incidents", func(c *gin.Context) {
			c.JSON(501, gin.H{"error": "not implemented"})
		})

		// Alerts (to be implemented)
		v1.GET("/alerts", func(c *gin.Context) {
			c.JSON(501, gin.H{"error": "not implemented"})
		})
	}
}
```

**Step 4: Update health check handlers to use database**

Update `backend/internal/api/handlers/health.go` (create if doesn't exist):

```go
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/openincident/openincident/internal/database"
	"gorm.io/gorm"
)

// Health returns a simple health check
func Health(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	}
}

// Ready checks if the application is ready to serve requests
func Ready(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := "ready"
		dbStatus := "ok"

		// Check database health
		if err := database.Health(); err != nil {
			dbStatus = "error"
			status = "not ready"
		}

		code := http.StatusOK
		if status != "ready" {
			code = http.StatusServiceUnavailable
		}

		c.JSON(code, gin.H{
			"status":   status,
			"database": dbStatus,
		})
	}
}
```

**Step 5: Test compilation**

Run: `cd backend && go build ./cmd/openincident`
Expected: Binary created successfully

**Step 6: Commit**

```bash
git add backend/cmd/openincident/main.go backend/internal/api/routes.go backend/internal/api/handlers/
git commit -m "feat(app): integrate database connection and migrations into main application"
```

---

## Task 13: Update Environment Variables

**Files:**
- Modify: `.env.example`

**Step 1: Update .env.example with database configuration**

Update `.env.example`:

```env
# Application
APP_ENV=development
LOG_LEVEL=info
PORT=8080

# Database
DATABASE_URL=postgresql://openincident:secret@localhost:5432/openincident?sslmode=disable
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=5
DB_CONN_MAX_LIFE=5m

# Redis
REDIS_URL=redis://localhost:6379

# Slack (for future use)
SLACK_BOT_TOKEN=
SLACK_SIGNING_SECRET=
SLACK_APP_TOKEN=

# AI (optional - for future use)
OPENAI_API_KEY=
```

**Step 2: Commit**

```bash
git add .env.example
git commit -m "docs: update environment variables for database configuration"
```

---

## Task 14: Testing and Verification

**Files:**
- Run commands to verify the implementation

**Step 1: Start database and test migrations**

```bash
# Start PostgreSQL and Redis
make dev

# Wait for services to be ready (check with make health or docker-compose ps)
sleep 5
```

**Step 2: Verify migrations run successfully**

```bash
# Check migration logs
docker-compose logs api | grep migration
```

Expected: Log messages showing migrations applied successfully

**Step 3: Verify database schema**

```bash
# Connect to database and check tables
docker-compose exec db psql -U openincident -d openincident -c "\dt"
```

Expected output should show:
- alerts
- incidents
- incident_alerts
- timeline_entries
- schema_migrations

**Step 4: Verify constraints and triggers**

```bash
# Check alerts table structure
docker-compose exec db psql -U openincident -d openincident -c "\d alerts"

# Check incidents table structure
docker-compose exec db psql -U openincident -d openincident -c "\d incidents"

# Check timeline_entries table structure
docker-compose exec db psql -U openincident -d openincident -c "\d timeline_entries"

# List all triggers
docker-compose exec db psql -U openincident -d openincident -c "SELECT trigger_name, event_object_table FROM information_schema.triggers WHERE trigger_schema = 'public';"
```

Expected: Triggers for immutability should be listed

**Step 5: Test health endpoints**

```bash
# Test health endpoint
curl http://localhost:8080/health

# Test ready endpoint
curl http://localhost:8080/ready
```

Expected: Both should return 200 OK with JSON responses

**Step 6: Test immutability constraints (optional manual test)**

```bash
# Insert a test alert
docker-compose exec db psql -U openincident -d openincident -c "
INSERT INTO alerts (external_id, source, status, severity, title, raw_payload, started_at)
VALUES ('test-001', 'prometheus', 'firing', 'critical', 'Test Alert', '{}'::jsonb, NOW());
"

# Try to update received_at (should fail)
docker-compose exec db psql -U openincident -d openincident -c "
UPDATE alerts SET received_at = NOW() WHERE external_id = 'test-001';
"
```

Expected: Second command should fail with error "received_at is immutable and cannot be updated"

**Step 7: Clean up test data**

```bash
docker-compose exec db psql -U openincident -d openincident -c "DELETE FROM alerts WHERE external_id = 'test-001';"
```

---

## Task 15: Documentation Update

**Files:**
- Create: `docs/database-setup.md`

**Step 1: Create database setup documentation**

Create `docs/database-setup.md`:

```markdown
# Database Setup

## Overview

OpenIncident uses PostgreSQL as its primary database with GORM as the ORM and golang-migrate for schema migrations.

## Schema

### Tables

1. **alerts** - Alerts from monitoring systems
2. **incidents** - Incident records
3. **incident_alerts** - Many-to-many join table linking incidents and alerts
4. **timeline_entries** - Immutable audit log of incident timeline events

### Immutability Constraints

#### Server-Generated Timestamps
The following fields are server-generated and cannot be modified after creation:
- `alerts.received_at`
- `incidents.created_at`
- `incidents.triggered_at`
- `timeline_entries.timestamp`
- `timeline_entries.created_at`

Database triggers enforce immutability.

#### Timeline Entries
Timeline entries are **completely immutable** - they cannot be updated or deleted. This is enforced at the database level via triggers that raise exceptions on UPDATE or DELETE operations.

## Migrations

### Running Migrations

```bash
# Apply all pending migrations
make migrate

# Or manually
docker-compose exec api go run cmd/openincident/main.go migrate
```

### Creating New Migrations

Migrations are numbered sequentially in the `backend/migrations/` directory:

```
backend/migrations/
├── 000001_create_alerts_table.up.sql
├── 000001_create_alerts_table.down.sql
├── 000002_create_incidents_table.up.sql
├── 000002_create_incidents_table.down.sql
...
```

**Format:**
- Up migration: `NNNNNN_description.up.sql`
- Down migration: `NNNNNN_description.down.sql`

**Example:**

```sql
-- 000005_add_users_table.up.sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

```sql
-- 000005_add_users_table.down.sql
DROP TABLE IF EXISTS users;
```

### Rolling Back Migrations

```bash
# Rollback last migration
make migrate-down

# Or manually using database.RollbackMigration()
```

## Connection Configuration

Database connection is configured via environment variables:

```env
DATABASE_URL=postgresql://user:password@host:port/dbname?sslmode=disable
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=5
DB_CONN_MAX_LIFE=5m
```

**Connection Pool Settings:**
- `DB_MAX_OPEN_CONNS` - Maximum number of open connections
- `DB_MAX_IDLE_CONNS` - Maximum number of idle connections
- `DB_CONN_MAX_LIFE` - Maximum lifetime of a connection

## Repository Pattern

Data access is abstracted through repository interfaces:

```go
// Example usage
alertRepo := repository.NewAlertRepository(db)
alert, err := alertRepo.GetByID(alertID)
```

### Available Repositories

1. **AlertRepository** - CRUD operations for alerts
2. **IncidentRepository** - CRUD operations for incidents, status management, alert linking
3. **TimelineRepository** - Append-only operations for timeline entries

### Error Handling

Repositories return typed errors:

```go
alert, err := alertRepo.GetByID(id)
if err != nil {
    if errors.Is(err, repository.ErrNotFound) {
        // Handle not found
    }
    // Handle other errors
}
```

**Error Types:**
- `ErrNotFound` - Resource not found
- `ErrAlreadyExists` - Duplicate resource
- `ErrInvalidInput` - Validation error
- `ErrDatabase` - Database operation error

## Testing

### Manual Testing

```bash
# Connect to database
docker-compose exec db psql -U openincident -d openincident

# List tables
\dt

# Describe table
\d alerts

# View triggers
SELECT trigger_name, event_object_table
FROM information_schema.triggers
WHERE trigger_schema = 'public';
```

### Integration Tests

Integration tests should use a test database:

```go
func setupTestDB(t *testing.T) *gorm.DB {
    db, err := database.Connect(database.Config{
        URL: "postgresql://openincident:secret@localhost:5432/openincident_test",
    })
    require.NoError(t, err)

    // Run migrations
    err = database.RunMigrations(db, "../../migrations")
    require.NoError(t, err)

    return db
}
```

## Troubleshooting

### Migration Errors

**Dirty Migration State:**
```bash
# Check migration version
docker-compose exec db psql -U openincident -d openincident -c "SELECT * FROM schema_migrations;"

# Manually fix dirty state (use with caution)
docker-compose exec db psql -U openincident -d openincident -c "UPDATE schema_migrations SET dirty = false;"
```

### Connection Issues

**Check database is running:**
```bash
docker-compose ps db
```

**Check connection:**
```bash
docker-compose exec db psql -U openincident -d openincident -c "SELECT 1;"
```

**View logs:**
```bash
docker-compose logs db
```

## Production Considerations

1. **SSL/TLS:** Use `sslmode=require` in production
2. **Connection Pooling:** Tune based on workload
3. **Monitoring:** Monitor connection pool metrics
4. **Backups:** Implement regular database backups
5. **Migrations:** Test migrations on staging before production
6. **Performance:** Add indexes based on query patterns
```

**Step 2: Commit**

```bash
git add docs/database-setup.md
git commit -m "docs: add comprehensive database setup documentation"
```

---

## Task 16: Final Verification and Cleanup

**Step 1: Run full test suite**

```bash
make test
```

Expected: All tests pass (currently should be minimal/none)

**Step 2: Run linters**

```bash
make lint
```

Expected: No linting errors

**Step 3: Format code**

```bash
make fmt
```

Expected: All code formatted

**Step 4: Stop services**

```bash
make down
```

**Step 5: Final commit**

```bash
git add .
git commit -m "feat(epic-002): complete database and models implementation

Implemented:
- GORM database connection with PostgreSQL
- golang-migrate migration system
- Alert model with immutable received_at
- Incident model with auto-incrementing number
- IncidentAlert join table
- TimelineEntry model with complete immutability (no updates/deletes)
- Repository pattern with typed errors
- Alert, Incident, and Timeline repositories
- Database triggers enforcing immutability
- Integration with main application
- Comprehensive documentation

All acceptance criteria for Epic-002 tasks OI-006 through OI-014 completed."
```

---

## Summary

This plan implements **Epic 002: Database & Models** completely, covering all 9 tasks (OI-006 through OI-014):

✅ **OI-006:** GORM with PostgreSQL connection, pooling, structured logging, graceful shutdown, retry logic
✅ **OI-007:** golang-migrate system with up/down migrations, numbered files, status checking
✅ **OI-008:** Alert model with all fields, immutable received_at, unique constraints, indexes
✅ **OI-009:** Incident model with SERIAL incident_number, immutable timestamps, enum constraints
✅ **OI-010:** IncidentAlert join table with composite primary key, foreign keys, CASCADE delete
✅ **OI-011:** TimelineEntry model with database triggers preventing UPDATE and DELETE
✅ **OI-012:** Alert repository with Create, GetByID, GetByExternalID, List, Update
✅ **OI-013:** Incident repository with Create, GetByID, GetByNumber, List, Update, LinkAlert, GetAlerts
✅ **OI-014:** TimelineEntry repository (append-only) with Create, GetByIncidentID, bulk create

**Key Features:**
- Database immutability enforced at PostgreSQL trigger level
- Repository pattern with typed error handling
- Connection retry logic and graceful shutdown
- Comprehensive migration system
- Full integration with application startup
