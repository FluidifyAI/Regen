package repository

import (
	"errors"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrTokenLimitExceeded is returned when a user already has MaxTokensPerUser distinct tokens.
var ErrTokenLimitExceeded = errors.New("device token limit reached (max 20 per user)")

// MaxTokensPerUser is the maximum number of distinct device tokens allowed per user.
const MaxTokensPerUser = 20

// DeviceTokenRepository defines the persistence operations for device tokens.
type DeviceTokenRepository interface {
	// Upsert inserts or updates a device token row.
	// On conflict (user_id, token): updates last_seen_at, platform, app_version.
	// Returns ErrTokenLimitExceeded when the user already has MaxTokensPerUser distinct tokens.
	Upsert(userID uuid.UUID, token, platform, appVersion string) error

	// GetByUserID returns all tokens for a user. Returns empty slice, not error, if none.
	GetByUserID(userID uuid.UUID) ([]models.DeviceToken, error)

	// DeleteByUserAndToken deletes a specific token owned by a user.
	// Returns (false, nil) when the row does not exist (idempotent).
	DeleteByUserAndToken(userID uuid.UUID, token string) (bool, error)

	// DeleteByToken deletes a token regardless of owner — used by PushService when
	// FCM reports the token as no longer registered.
	DeleteByToken(token string) error

	// UpdateLastSeen bumps last_seen_at for a token to NOW().
	// Best-effort — called after a successful FCM send. Not a hard failure.
	UpdateLastSeen(token string) error

	// DeleteStale deletes tokens with last_seen_at older than the given cutoff.
	// Returns the number of rows deleted.
	DeleteStale(before time.Time) (int64, error)

	// CountByUserID returns the number of distinct tokens for a user.
	CountByUserID(userID uuid.UUID) (int64, error)
}

type deviceTokenRepository struct {
	db *gorm.DB
}

// NewDeviceTokenRepository constructs a DeviceTokenRepository backed by GORM.
func NewDeviceTokenRepository(db *gorm.DB) DeviceTokenRepository {
	return &deviceTokenRepository{db: db}
}

func (r *deviceTokenRepository) Upsert(userID uuid.UUID, token, platform, appVersion string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Lock the user row for the duration of the transaction to serialize concurrent
		// Upsert calls for the same user. Without this, two goroutines can both pass
		// the 20-token count check before either inserts, bypassing the cap.
		// FOR UPDATE is PostgreSQL-only; SQLite (used in tests) does not support it.
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Exec("SELECT id FROM users WHERE id = ? FOR UPDATE", userID).Error; err != nil {
				return &DatabaseError{Op: "upsert device_token: lock user row", Err: err}
			}
		}

		// Check whether this exact (user_id, token) pair already exists.
		var existing int64
		if err := tx.Model(&models.DeviceToken{}).
			Where("user_id = ? AND token = ?", userID, token).
			Count(&existing).Error; err != nil {
			return &DatabaseError{Op: "upsert device_token: check existing", Err: err}
		}

		// If the token is new, enforce the per-user cap.
		if existing == 0 {
			var count int64
			if err := tx.Model(&models.DeviceToken{}).
				Where("user_id = ?", userID).
				Count(&count).Error; err != nil {
				return &DatabaseError{Op: "upsert device_token: count", Err: err}
			}
			if count >= MaxTokensPerUser {
				return ErrTokenLimitExceeded
			}
		}

		// Upsert: insert or update on conflict (user_id, token).
		// SQLite uses ON CONFLICT DO UPDATE; PostgreSQL uses the same syntax.
		now := time.Now()
		err := tx.Exec(
			`INSERT INTO device_tokens (id, user_id, token, platform, app_version, created_at, last_seen_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT (user_id, token)
			 DO UPDATE SET last_seen_at = excluded.last_seen_at,
			               platform    = excluded.platform,
			               app_version = excluded.app_version`,
			uuid.New(), userID, token, platform, appVersion, now, now,
		).Error
		if err != nil {
			return &DatabaseError{Op: "upsert device_token", Err: err}
		}
		return nil
	})
}

func (r *deviceTokenRepository) GetByUserID(userID uuid.UUID) ([]models.DeviceToken, error) {
	var tokens []models.DeviceToken
	if err := r.db.Where("user_id = ?", userID).Find(&tokens).Error; err != nil {
		return nil, &DatabaseError{Op: "get device_tokens by user", Err: err}
	}
	return tokens, nil
}

func (r *deviceTokenRepository) DeleteByUserAndToken(userID uuid.UUID, token string) (bool, error) {
	result := r.db.Where("user_id = ? AND token = ?", userID, token).Delete(&models.DeviceToken{})
	if result.Error != nil {
		return false, &DatabaseError{Op: "delete device_token by user+token", Err: result.Error}
	}
	return result.RowsAffected > 0, nil
}

func (r *deviceTokenRepository) DeleteByToken(token string) error {
	if err := r.db.Where("token = ?", token).Delete(&models.DeviceToken{}).Error; err != nil {
		return &DatabaseError{Op: "delete device_token by token", Err: err}
	}
	return nil
}

func (r *deviceTokenRepository) UpdateLastSeen(token string) error {
	if err := r.db.Model(&models.DeviceToken{}).
		Where("token = ?", token).
		Update("last_seen_at", time.Now()).Error; err != nil {
		return &DatabaseError{Op: "update device_token last_seen_at", Err: err}
	}
	return nil
}

func (r *deviceTokenRepository) DeleteStale(before time.Time) (int64, error) {
	result := r.db.Where("last_seen_at < ?", before).Delete(&models.DeviceToken{})
	if result.Error != nil {
		return 0, &DatabaseError{Op: "delete stale device_tokens", Err: result.Error}
	}
	return result.RowsAffected, nil
}

func (r *deviceTokenRepository) CountByUserID(userID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.Model(&models.DeviceToken{}).
		Where("user_id = ?", userID).
		Count(&count).Error; err != nil {
		return 0, &DatabaseError{Op: "count device_tokens by user", Err: err}
	}
	return count, nil
}
