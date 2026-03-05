package database

import (
	"fmt"
	"sync/atomic"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewTestDB creates an in-memory SQLite database for testing.
// Callers that need a users table should prefer SetupTestDB(t) which also
// creates the table and registers cleanup. This function is kept for
// compatibility with test helpers that manage their own schema.
func NewTestDB() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}

// testDBCounter is used to create unique in-memory database names per test.
var testDBCounter uint64

// createUsersTable creates the users table with a SQLite-compatible DDL.
// GORM AutoMigrate cannot be used here because the User model uses
// PostgreSQL-specific expressions (gen_random_uuid()) that SQLite rejects.
const createUsersTableSQL = `
CREATE TABLE IF NOT EXISTS users (
	id          TEXT PRIMARY KEY,
	email       TEXT NOT NULL UNIQUE,
	name        TEXT NOT NULL DEFAULT '',
	saml_subject      TEXT UNIQUE,
	saml_idp_issuer   TEXT NOT NULL DEFAULT '',
	password_hash     TEXT,
	auth_source       TEXT NOT NULL DEFAULT 'saml',
	agent_type        TEXT,
	active            INTEGER NOT NULL DEFAULT 1,
	slack_user_id     TEXT,
	role              TEXT NOT NULL DEFAULT 'member',
	last_login_at     DATETIME,
	created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
)`

// SetupTestDB creates an isolated in-memory SQLite database for a single test,
// creates the users table, and registers a cleanup function that closes the
// connection when the test finishes.
func SetupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	// Each test gets its own named in-memory database so parallel tests don't share state.
	n := atomic.AddUint64(&testDBCounter, 1)
	dsn := fmt.Sprintf("file:testdb%d?mode=memory&cache=shared", n)

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("SetupTestDB: open sqlite: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("SetupTestDB: get sql.DB: %v", err)
	}
	if _, err := sqlDB.Exec(createUsersTableSQL); err != nil {
		t.Fatalf("SetupTestDB: create users table: %v", err)
	}

	t.Cleanup(func() {
		sqlDB.Close()
	})

	return db
}
