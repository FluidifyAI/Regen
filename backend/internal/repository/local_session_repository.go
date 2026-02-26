package repository

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/models"
	"gorm.io/gorm"
)

const sessionTTL = 7 * 24 * time.Hour

// LocalSessionRepository manages local session tokens.
type LocalSessionRepository interface {
	Create(userID uuid.UUID) (*models.LocalSession, error)
	GetByToken(token string) (*models.LocalSession, error)
	DeleteByToken(token string) error
	DeleteExpired() error
}

type localSessionRepository struct {
	db *gorm.DB
}

func NewLocalSessionRepository(db *gorm.DB) LocalSessionRepository {
	return &localSessionRepository{db: db}
}

func (r *localSessionRepository) Create(userID uuid.UUID) (*models.LocalSession, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	s := &models.LocalSession{
		Token:     hex.EncodeToString(raw),
		UserID:    userID,
		ExpiresAt: time.Now().Add(sessionTTL),
	}
	return s, r.db.Create(s).Error
}

func (r *localSessionRepository) GetByToken(token string) (*models.LocalSession, error) {
	var s models.LocalSession
	err := r.db.Where("token = ? AND expires_at > NOW()", token).First(&s).Error
	if err == gorm.ErrRecordNotFound {
		preview := token
		if len(token) > 8 {
			preview = token[:8] + "..."
		}
		return nil, &NotFoundError{Resource: "local_session", ID: preview}
	}
	return &s, err
}

func (r *localSessionRepository) DeleteByToken(token string) error {
	return r.db.Delete(&models.LocalSession{}, "token = ?", token).Error
}

func (r *localSessionRepository) DeleteExpired() error {
	return r.db.Delete(&models.LocalSession{}, "expires_at <= NOW()").Error
}
