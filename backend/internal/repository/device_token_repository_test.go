package repository_test

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var deviceTokenTestDBCounter uint64

// setupDeviceTokenTestDB creates an isolated in-memory SQLite DB for device token tests.
func setupDeviceTokenTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	n := atomic.AddUint64(&deviceTokenTestDBCounter, 1)
	dsn := fmt.Sprintf("file:devicetokentestdb%d?mode=memory&cache=shared", n)

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("setupDeviceTokenTestDB: open sqlite: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("setupDeviceTokenTestDB: get sql.DB: %v", err)
	}

	// Pin to a single connection so PRAGMA foreign_keys = ON stays in effect.
	sqlDB.SetMaxOpenConns(1)
	if _, err := sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("setupDeviceTokenTestDB: enable FK pragma: %v", err)
	}

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id   TEXT PRIMARY KEY,
			email TEXT NOT NULL,
			name  TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS device_tokens (
			id           TEXT        PRIMARY KEY,
			user_id      TEXT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token        TEXT        NOT NULL,
			platform     TEXT        NOT NULL DEFAULT 'web',
			app_version  TEXT        NOT NULL DEFAULT '',
			created_at   DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_seen_at DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT uq_device_tokens_user_token UNIQUE (user_id, token)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_device_tokens_user_id   ON device_tokens (user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_device_tokens_last_seen ON device_tokens (last_seen_at)`,
	}

	for _, stmt := range stmts {
		if _, err := sqlDB.Exec(stmt); err != nil {
			t.Fatalf("setupDeviceTokenTestDB: exec DDL: %v\nSQL: %s", err, stmt)
		}
	}

	t.Cleanup(func() { sqlDB.Close() })

	return db
}

// makeTestUser inserts a user row and returns the model.
func makeTestUser(t *testing.T, db *gorm.DB) models.User {
	t.Helper()
	u := models.User{
		ID:    uuid.New(),
		Email: fmt.Sprintf("user-%s@test.example", uuid.New().String()[:8]),
		Name:  "Test User",
	}
	if err := db.Exec("INSERT INTO users (id, email, name) VALUES (?, ?, ?)", u.ID.String(), u.Email, u.Name).Error; err != nil {
		t.Fatalf("makeTestUser: insert: %v", err)
	}
	return u
}

// insertToken directly inserts a device_token row with explicit last_seen_at.
func insertToken(t *testing.T, db *gorm.DB, userID uuid.UUID, token, platform string, lastSeenAt time.Time) {
	t.Helper()
	id := uuid.New().String()
	if err := db.Exec(
		"INSERT INTO device_tokens (id, user_id, token, platform, app_version, created_at, last_seen_at) VALUES (?, ?, ?, ?, '', ?, ?)",
		id, userID.String(), token, platform, lastSeenAt, lastSeenAt,
	).Error; err != nil {
		t.Fatalf("insertToken: %v", err)
	}
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestDeviceTokenRepo_UpsertNew(t *testing.T) {
	db := setupDeviceTokenTestDB(t)
	u := makeTestUser(t, db)
	repo := repository.NewDeviceTokenRepository(db)

	err := repo.Upsert(u.ID, "tok-abc", "ios", "1.0.0")
	if err != nil {
		t.Fatalf("Upsert: unexpected error: %v", err)
	}

	tokens, err := repo.GetByUserID(u.ID)
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Platform != "ios" {
		t.Errorf("platform = %q, want %q", tokens[0].Platform, "ios")
	}
	// last_seen_at should be within the last minute
	if time.Since(tokens[0].LastSeenAt) > time.Minute {
		t.Errorf("last_seen_at is too old: %v", tokens[0].LastSeenAt)
	}
}

func TestDeviceTokenRepo_UpsertIdempotent(t *testing.T) {
	db := setupDeviceTokenTestDB(t)
	u := makeTestUser(t, db)
	repo := repository.NewDeviceTokenRepository(db)

	if err := repo.Upsert(u.ID, "tok-same", "android", "1.0.0"); err != nil {
		t.Fatalf("first Upsert: %v", err)
	}
	if err := repo.Upsert(u.ID, "tok-same", "ios", "2.0.0"); err != nil {
		t.Fatalf("second Upsert: %v", err)
	}

	tokens, err := repo.GetByUserID(u.ID)
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token (idempotent), got %d", len(tokens))
	}
	// Platform and app_version should be updated on conflict
	if tokens[0].Platform != "ios" {
		t.Errorf("platform after update = %q, want %q", tokens[0].Platform, "ios")
	}
	if tokens[0].AppVersion != "2.0.0" {
		t.Errorf("app_version after update = %q, want %q", tokens[0].AppVersion, "2.0.0")
	}
}

func TestDeviceTokenRepo_UpsertLimitExceeded(t *testing.T) {
	db := setupDeviceTokenTestDB(t)
	u := makeTestUser(t, db)
	repo := repository.NewDeviceTokenRepository(db)

	// Insert 20 distinct tokens
	for i := range 20 {
		token := fmt.Sprintf("tok-%02d", i)
		if err := repo.Upsert(u.ID, token, "web", ""); err != nil {
			t.Fatalf("Upsert token %d: %v", i, err)
		}
	}

	// 21st distinct token must return ErrTokenLimitExceeded
	err := repo.Upsert(u.ID, "tok-21st", "web", "")
	if err == nil {
		t.Fatal("expected ErrTokenLimitExceeded, got nil")
	}
	if !isErrTokenLimitExceeded(err) {
		t.Fatalf("expected ErrTokenLimitExceeded, got: %v", err)
	}
}

func isErrTokenLimitExceeded(err error) bool {
	return errors.Is(err, repository.ErrTokenLimitExceeded)
}

func TestDeviceTokenRepo_DeleteByUserAndToken(t *testing.T) {
	db := setupDeviceTokenTestDB(t)
	u := makeTestUser(t, db)
	repo := repository.NewDeviceTokenRepository(db)

	if err := repo.Upsert(u.ID, "tok-del", "ios", ""); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	deleted, err := repo.DeleteByUserAndToken(u.ID, "tok-del")
	if err != nil {
		t.Fatalf("DeleteByUserAndToken: %v", err)
	}
	if !deleted {
		t.Fatal("expected deleted=true, got false")
	}

	tokens, _ := repo.GetByUserID(u.ID)
	if len(tokens) != 0 {
		t.Fatalf("expected 0 tokens after delete, got %d", len(tokens))
	}
}

func TestDeviceTokenRepo_DeleteByUserAndToken_WrongUser(t *testing.T) {
	db := setupDeviceTokenTestDB(t)
	u1 := makeTestUser(t, db)
	u2 := makeTestUser(t, db)
	repo := repository.NewDeviceTokenRepository(db)

	if err := repo.Upsert(u1.ID, "tok-u1", "ios", ""); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	// u2 tries to delete u1's token — should return (false, nil)
	deleted, err := repo.DeleteByUserAndToken(u2.ID, "tok-u1")
	if err != nil {
		t.Fatalf("DeleteByUserAndToken: unexpected error: %v", err)
	}
	if deleted {
		t.Fatal("expected deleted=false for wrong user, got true")
	}

	// u1's token should still exist
	tokens, _ := repo.GetByUserID(u1.ID)
	if len(tokens) != 1 {
		t.Fatalf("u1's token should still exist, got %d tokens", len(tokens))
	}
}

func TestDeviceTokenRepo_DeleteByToken(t *testing.T) {
	db := setupDeviceTokenTestDB(t)
	u := makeTestUser(t, db)
	repo := repository.NewDeviceTokenRepository(db)

	if err := repo.Upsert(u.ID, "tok-stale", "android", ""); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	if err := repo.DeleteByToken("tok-stale"); err != nil {
		t.Fatalf("DeleteByToken: %v", err)
	}

	tokens, _ := repo.GetByUserID(u.ID)
	if len(tokens) != 0 {
		t.Fatalf("expected 0 tokens after DeleteByToken, got %d", len(tokens))
	}
}

func TestDeviceTokenRepo_DeleteStale(t *testing.T) {
	db := setupDeviceTokenTestDB(t)
	u := makeTestUser(t, db)
	repo := repository.NewDeviceTokenRepository(db)

	old := time.Now().Add(-100 * 24 * time.Hour)
	recent := time.Now()

	insertToken(t, db, u.ID, "tok-old", "web", old)
	insertToken(t, db, u.ID, "tok-new", "web", recent)

	cutoff := time.Now().Add(-90 * 24 * time.Hour)
	_, err := repo.DeleteStale(cutoff)
	if err != nil {
		t.Fatalf("DeleteStale: %v", err)
	}

	tokens, _ := repo.GetByUserID(u.ID)
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token remaining, got %d", len(tokens))
	}
	// Verify the remaining token was scanned correctly (Token field is readable from DB even though json:"-")
	if tokens[0].Token == "" {
		t.Error("expected non-empty token after DeleteStale, got empty string")
	}
	// Verify it's the recent one (by checking that old one is gone)
	// We can use CountByUserID as a proxy
	count, _ := repo.CountByUserID(u.ID)
	if count != 1 {
		t.Fatalf("expected count=1 after DeleteStale, got %d", count)
	}
}

func TestDeviceTokenRepo_DeleteStale_Count(t *testing.T) {
	db := setupDeviceTokenTestDB(t)
	u := makeTestUser(t, db)
	repo := repository.NewDeviceTokenRepository(db)

	old := time.Now().Add(-100 * 24 * time.Hour)

	insertToken(t, db, u.ID, "tok-old-1", "web", old)
	insertToken(t, db, u.ID, "tok-old-2", "ios", old)
	insertToken(t, db, u.ID, "tok-old-3", "android", old)

	cutoff := time.Now().Add(-90 * 24 * time.Hour)
	deleted, err := repo.DeleteStale(cutoff)
	if err != nil {
		t.Fatalf("DeleteStale: %v", err)
	}
	if deleted != 3 {
		t.Fatalf("expected deleted=3, got %d", deleted)
	}
}

func TestDeviceTokenRepo_CountByUserID(t *testing.T) {
	db := setupDeviceTokenTestDB(t)
	u := makeTestUser(t, db)
	repo := repository.NewDeviceTokenRepository(db)

	count, err := repo.CountByUserID(u.ID)
	if err != nil {
		t.Fatalf("CountByUserID: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}

	_ = repo.Upsert(u.ID, "tok-1", "ios", "")
	_ = repo.Upsert(u.ID, "tok-2", "android", "")

	count, err = repo.CountByUserID(u.ID)
	if err != nil {
		t.Fatalf("CountByUserID: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2, got %d", count)
	}
}

func TestDeviceTokenRepo_CascadeOnUserDelete(t *testing.T) {
	db := setupDeviceTokenTestDB(t)
	u := makeTestUser(t, db)
	repo := repository.NewDeviceTokenRepository(db)

	_ = repo.Upsert(u.ID, "tok-cascade-1", "ios", "")
	_ = repo.Upsert(u.ID, "tok-cascade-2", "android", "")

	// Delete the user row — cascade should remove device_tokens
	if err := db.Exec("DELETE FROM users WHERE id = ?", u.ID.String()).Error; err != nil {
		t.Fatalf("delete user: %v", err)
	}

	tokens, err := repo.GetByUserID(u.ID)
	if err != nil {
		t.Fatalf("GetByUserID after cascade: %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("expected 0 tokens after user cascade delete, got %d", len(tokens))
	}
}
