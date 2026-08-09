package repository

import (
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WebPushSubscriptionRepository defines the persistence operations for Web Push subscriptions.
type WebPushSubscriptionRepository interface {
	// Upsert inserts or updates a Web Push subscription row.
	// On conflict (user_id, endpoint): updates last_seen_at, p256dh, auth.
	// Returns ErrTokenLimitExceeded when the user already has MaxTokensPerUser subscriptions.
	Upsert(userID uuid.UUID, endpoint, p256dh, auth string) error

	// GetByUserID returns all subscriptions for a user. Returns empty slice, not error, if none.
	GetByUserID(userID uuid.UUID) ([]models.WebPushSubscription, error)

	// DeleteByUserAndEndpoint deletes a specific subscription owned by a user.
	// Returns (false, nil) when the row does not exist (idempotent).
	DeleteByUserAndEndpoint(userID uuid.UUID, endpoint string) (bool, error)

	// DeleteByEndpoint deletes a subscription regardless of owner — used by WebPushService
	// when the push service returns HTTP 410 Gone (subscription permanently expired).
	DeleteByEndpoint(endpoint string) error

	// UpdateLastSeen bumps last_seen_at for an endpoint to NOW().
	// Best-effort — called after a successful Web Push send. Not a hard failure.
	UpdateLastSeen(endpoint string) error

	// DeleteStale deletes subscriptions with last_seen_at older than the given cutoff.
	// Returns the number of rows deleted.
	DeleteStale(before time.Time) (int64, error)

	// CountByUserID returns the number of distinct subscriptions for a user.
	CountByUserID(userID uuid.UUID) (int64, error)
}

type webPushSubscriptionRepository struct {
	db *gorm.DB
}

// NewWebPushSubscriptionRepository constructs a WebPushSubscriptionRepository backed by GORM.
func NewWebPushSubscriptionRepository(db *gorm.DB) WebPushSubscriptionRepository {
	return &webPushSubscriptionRepository{db: db}
}

func (r *webPushSubscriptionRepository) Upsert(userID uuid.UUID, endpoint, p256dh, auth string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Lock the user row for the duration of the transaction to serialize concurrent
		// Upsert calls for the same user. Without this, two goroutines can both pass
		// the 20-subscription count check before either inserts, bypassing the cap.
		// FOR UPDATE is PostgreSQL-only; SQLite (used in tests) does not support it.
		if tx.Name() == "postgres" {
			if err := tx.Exec("SELECT id FROM users WHERE id = ? FOR UPDATE", userID).Error; err != nil {
				return &DatabaseError{Op: "upsert web_push_subscription: lock user row", Err: err}
			}
		}

		// Check whether this exact (user_id, endpoint) pair already exists.
		var existing int64
		if err := tx.Model(&models.WebPushSubscription{}).
			Where("user_id = ? AND endpoint = ?", userID, endpoint).
			Count(&existing).Error; err != nil {
			return &DatabaseError{Op: "upsert web_push_subscription: check existing", Err: err}
		}

		// If the endpoint is new, enforce the per-user cap.
		if existing == 0 {
			var count int64
			if err := tx.Model(&models.WebPushSubscription{}).
				Where("user_id = ?", userID).
				Count(&count).Error; err != nil {
				return &DatabaseError{Op: "upsert web_push_subscription: count", Err: err}
			}
			if count >= MaxTokensPerUser {
				return ErrTokenLimitExceeded
			}
		}

		// Upsert: insert or update on conflict (user_id, endpoint).
		now := time.Now()
		err := tx.Exec(
			`INSERT INTO web_push_subscriptions (id, user_id, endpoint, p256dh, auth, created_at, last_seen_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT (user_id, endpoint)
			 DO UPDATE SET last_seen_at = excluded.last_seen_at,
			               p256dh      = excluded.p256dh,
			               auth        = excluded.auth`,
			uuid.New(), userID, endpoint, p256dh, auth, now, now,
		).Error
		if err != nil {
			return &DatabaseError{Op: "upsert web_push_subscription", Err: err}
		}
		return nil
	})
}

func (r *webPushSubscriptionRepository) GetByUserID(userID uuid.UUID) ([]models.WebPushSubscription, error) {
	var subs []models.WebPushSubscription
	if err := r.db.Where("user_id = ?", userID).Find(&subs).Error; err != nil {
		return nil, &DatabaseError{Op: "get web_push_subscriptions by user", Err: err}
	}
	return subs, nil
}

func (r *webPushSubscriptionRepository) DeleteByUserAndEndpoint(userID uuid.UUID, endpoint string) (bool, error) {
	result := r.db.Where("user_id = ? AND endpoint = ?", userID, endpoint).Delete(&models.WebPushSubscription{})
	if result.Error != nil {
		return false, &DatabaseError{Op: "delete web_push_subscription by user+endpoint", Err: result.Error}
	}
	return result.RowsAffected > 0, nil
}

func (r *webPushSubscriptionRepository) DeleteByEndpoint(endpoint string) error {
	if err := r.db.Where("endpoint = ?", endpoint).Delete(&models.WebPushSubscription{}).Error; err != nil {
		return &DatabaseError{Op: "delete web_push_subscription by endpoint", Err: err}
	}
	return nil
}

func (r *webPushSubscriptionRepository) UpdateLastSeen(endpoint string) error {
	if err := r.db.Model(&models.WebPushSubscription{}).
		Where("endpoint = ?", endpoint).
		Update("last_seen_at", time.Now()).Error; err != nil {
		return &DatabaseError{Op: "update web_push_subscription last_seen_at", Err: err}
	}
	return nil
}

func (r *webPushSubscriptionRepository) DeleteStale(before time.Time) (int64, error) {
	result := r.db.Where("last_seen_at < ?", before).Delete(&models.WebPushSubscription{})
	if result.Error != nil {
		return 0, &DatabaseError{Op: "delete stale web_push_subscriptions", Err: result.Error}
	}
	return result.RowsAffected, nil
}

func (r *webPushSubscriptionRepository) CountByUserID(userID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.Model(&models.WebPushSubscription{}).
		Where("user_id = ?", userID).
		Count(&count).Error; err != nil {
		return 0, &DatabaseError{Op: "count web_push_subscriptions by user", Err: err}
	}
	return count, nil
}
