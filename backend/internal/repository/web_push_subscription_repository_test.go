package repository_test

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var webPushTestDBCounter uint64

// setupWebPushTestDB creates an isolated in-memory SQLite DB for web push subscription tests.
func setupWebPushTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	n := atomic.AddUint64(&webPushTestDBCounter, 1)
	dsn := fmt.Sprintf("file:webpushtestdb%d?mode=memory&cache=shared", n)

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("setupWebPushTestDB: open sqlite: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("setupWebPushTestDB: get sql.DB: %v", err)
	}

	// Pin to a single connection so PRAGMA foreign_keys = ON stays in effect.
	sqlDB.SetMaxOpenConns(1)
	if _, err := sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("setupWebPushTestDB: enable FK pragma: %v", err)
	}

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id    TEXT PRIMARY KEY,
			email TEXT NOT NULL,
			name  TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS web_push_subscriptions (
			id           TEXT     PRIMARY KEY,
			user_id      TEXT     NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			endpoint     TEXT     NOT NULL,
			p256dh       TEXT     NOT NULL,
			auth         TEXT     NOT NULL,
			created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT uq_web_push_user_endpoint UNIQUE (user_id, endpoint)
		)`,
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			t.Fatalf("setupWebPushTestDB: exec schema: %v", err)
		}
	}

	t.Cleanup(func() { sqlDB.Close() })
	return db
}

// insertWebPushUser creates a user row for FK purposes and returns its ID.
func insertWebPushUser(t *testing.T, db *gorm.DB) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if err := db.Exec(
		"INSERT INTO users (id, email, name) VALUES (?, ?, ?)",
		id.String(), "test@example.com", "Test User",
	).Error; err != nil {
		t.Fatalf("insertWebPushUser: %v", err)
	}
	return id
}

func TestWebPushSubscriptionRepository_Upsert_Insert(t *testing.T) {
	db := setupWebPushTestDB(t)
	repo := repository.NewWebPushSubscriptionRepository(db)
	userID := insertWebPushUser(t, db)

	if err := repo.Upsert(userID, "https://example.com/ep1", "p256dh-aaa", "auth-aaa"); err != nil {
		t.Fatalf("Upsert: unexpected error: %v", err)
	}

	count, err := repo.CountByUserID(userID)
	if err != nil {
		t.Fatalf("CountByUserID: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count=1, got %d", count)
	}
}

func TestWebPushSubscriptionRepository_Upsert_UpdatesKeysOnConflict(t *testing.T) {
	db := setupWebPushTestDB(t)
	repo := repository.NewWebPushSubscriptionRepository(db)
	userID := insertWebPushUser(t, db)

	ep := "https://example.com/ep1"
	if err := repo.Upsert(userID, ep, "p256dh-aaa", "auth-aaa"); err != nil {
		t.Fatalf("first Upsert: %v", err)
	}
	if err := repo.Upsert(userID, ep, "p256dh-bbb", "auth-bbb"); err != nil {
		t.Fatalf("second Upsert: %v", err)
	}

	// Still only one row.
	count, _ := repo.CountByUserID(userID)
	if count != 1 {
		t.Errorf("expected count=1 after duplicate upsert, got %d", count)
	}

	// p256dh and auth must be updated to the latest values.
	subs, err := repo.GetByUserID(userID)
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(subs))
	}
	if subs[0].P256DH != "p256dh-bbb" {
		t.Errorf("expected P256DH=p256dh-bbb, got %q", subs[0].P256DH)
	}
	if subs[0].Auth != "auth-bbb" {
		t.Errorf("expected Auth=auth-bbb, got %q", subs[0].Auth)
	}
}

func TestWebPushSubscriptionRepository_Upsert_CapEnforced(t *testing.T) {
	db := setupWebPushTestDB(t)
	repo := repository.NewWebPushSubscriptionRepository(db)
	userID := insertWebPushUser(t, db)

	// Insert exactly MaxTokensPerUser subscriptions.
	for i := range repository.MaxTokensPerUser {
		ep := fmt.Sprintf("https://example.com/ep%d", i)
		if err := repo.Upsert(userID, ep, "p256dh", "auth"); err != nil {
			t.Fatalf("Upsert %d: %v", i, err)
		}
	}

	// The next new endpoint must fail with ErrTokenLimitExceeded.
	err := repo.Upsert(userID, "https://example.com/ep-over", "p256dh", "auth")
	if !errors.Is(err, repository.ErrTokenLimitExceeded) {
		t.Errorf("expected ErrTokenLimitExceeded, got %v", err)
	}
}

func TestWebPushSubscriptionRepository_Upsert_ExistingEndpointNotCountedAgainstCap(t *testing.T) {
	db := setupWebPushTestDB(t)
	repo := repository.NewWebPushSubscriptionRepository(db)
	userID := insertWebPushUser(t, db)

	// Fill to the cap.
	for i := range repository.MaxTokensPerUser {
		ep := fmt.Sprintf("https://example.com/ep%d", i)
		if err := repo.Upsert(userID, ep, "p256dh", "auth"); err != nil {
			t.Fatalf("Upsert %d: %v", i, err)
		}
	}

	// Re-upserting an existing endpoint must succeed.
	ep0 := "https://example.com/ep0"
	if err := repo.Upsert(userID, ep0, "p256dh-new", "auth-new"); err != nil {
		t.Errorf("re-upsert of existing endpoint at cap should succeed, got: %v", err)
	}
}

func TestWebPushSubscriptionRepository_GetByUserID_Empty(t *testing.T) {
	db := setupWebPushTestDB(t)
	repo := repository.NewWebPushSubscriptionRepository(db)
	userID := insertWebPushUser(t, db)

	subs, err := repo.GetByUserID(userID)
	if err != nil {
		t.Fatalf("GetByUserID (empty): %v", err)
	}
	if len(subs) != 0 {
		t.Errorf("expected empty slice, got %d", len(subs))
	}
}

func TestWebPushSubscriptionRepository_GetByUserID_DoesNotReturnOtherUsers(t *testing.T) {
	db := setupWebPushTestDB(t)
	repo := repository.NewWebPushSubscriptionRepository(db)

	userA := insertWebPushUser(t, db)
	userB := uuid.New()
	db.Exec("INSERT INTO users (id, email, name) VALUES (?, ?, ?)", userB.String(), "b@example.com", "User B")

	repo.Upsert(userA, "https://example.com/epA", "p256dh", "auth")
	repo.Upsert(userB, "https://example.com/epB", "p256dh", "auth")

	subs, _ := repo.GetByUserID(userA)
	if len(subs) != 1 {
		t.Errorf("expected 1 subscription for userA, got %d", len(subs))
	}
}

func TestWebPushSubscriptionRepository_DeleteByUserAndEndpoint_Deleted(t *testing.T) {
	db := setupWebPushTestDB(t)
	repo := repository.NewWebPushSubscriptionRepository(db)
	userID := insertWebPushUser(t, db)

	ep := "https://example.com/ep1"
	repo.Upsert(userID, ep, "p256dh", "auth")

	deleted, err := repo.DeleteByUserAndEndpoint(userID, ep)
	if err != nil {
		t.Fatalf("DeleteByUserAndEndpoint: %v", err)
	}
	if !deleted {
		t.Error("expected deleted=true")
	}

	count, _ := repo.CountByUserID(userID)
	if count != 0 {
		t.Errorf("expected count=0 after delete, got %d", count)
	}
}

func TestWebPushSubscriptionRepository_DeleteByUserAndEndpoint_Idempotent(t *testing.T) {
	db := setupWebPushTestDB(t)
	repo := repository.NewWebPushSubscriptionRepository(db)
	userID := insertWebPushUser(t, db)

	deleted, err := repo.DeleteByUserAndEndpoint(userID, "https://example.com/nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted {
		t.Error("expected deleted=false for non-existent endpoint")
	}
}

func TestWebPushSubscriptionRepository_DeleteByEndpoint(t *testing.T) {
	db := setupWebPushTestDB(t)
	repo := repository.NewWebPushSubscriptionRepository(db)
	userID := insertWebPushUser(t, db)

	ep := "https://example.com/ep1"
	repo.Upsert(userID, ep, "p256dh", "auth")

	if err := repo.DeleteByEndpoint(ep); err != nil {
		t.Fatalf("DeleteByEndpoint: %v", err)
	}

	count, _ := repo.CountByUserID(userID)
	if count != 0 {
		t.Errorf("expected count=0 after DeleteByEndpoint, got %d", count)
	}
}

func TestWebPushSubscriptionRepository_UpdateLastSeen(t *testing.T) {
	db := setupWebPushTestDB(t)
	repo := repository.NewWebPushSubscriptionRepository(db)
	userID := insertWebPushUser(t, db)

	ep := "https://example.com/ep1"
	repo.Upsert(userID, ep, "p256dh", "auth")

	if err := repo.UpdateLastSeen(ep); err != nil {
		t.Fatalf("UpdateLastSeen: %v", err)
	}
}

func TestWebPushSubscriptionRepository_DeleteStale(t *testing.T) {
	db := setupWebPushTestDB(t)
	repo := repository.NewWebPushSubscriptionRepository(db)
	userID := insertWebPushUser(t, db)

	// Insert a subscription and manually backdate last_seen_at.
	ep := "https://example.com/ep1"
	repo.Upsert(userID, ep, "p256dh", "auth")
	db.Exec("UPDATE web_push_subscriptions SET last_seen_at = ? WHERE endpoint = ?",
		time.Now().Add(-48*time.Hour), ep)

	// Insert a fresh subscription.
	repo.Upsert(userID, "https://example.com/ep2", "p256dh", "auth")

	deleted, err := repo.DeleteStale(time.Now().Add(-24 * time.Hour))
	if err != nil {
		t.Fatalf("DeleteStale: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	count, _ := repo.CountByUserID(userID)
	if count != 1 {
		t.Errorf("expected 1 remaining subscription, got %d", count)
	}
}

func TestWebPushSubscriptionRepository_CountByUserID(t *testing.T) {
	db := setupWebPushTestDB(t)
	repo := repository.NewWebPushSubscriptionRepository(db)
	userID := insertWebPushUser(t, db)

	for i := range 3 {
		ep := fmt.Sprintf("https://example.com/ep%d", i)
		repo.Upsert(userID, ep, "p256dh", "auth")
	}

	count, err := repo.CountByUserID(userID)
	if err != nil {
		t.Fatalf("CountByUserID: %v", err)
	}
	if count != 3 {
		t.Errorf("expected count=3, got %d", count)
	}
}
