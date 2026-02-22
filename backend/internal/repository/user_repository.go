package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserRepository defines the data access interface for users.
type UserRepository interface {
	GetBySubject(subject string) (*models.User, error)
	GetByEmail(email string) (*models.User, error)
	Upsert(ctx context.Context, user *models.User) error
	UpdateLastLogin(id uuid.UUID, at time.Time) error
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) GetBySubject(subject string) (*models.User, error) {
	var user models.User
	err := r.db.Where("saml_subject = ?", subject).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		return nil, &NotFoundError{Resource: "user", ID: subject}
	}
	return &user, err
}

func (r *userRepository) GetByEmail(email string) (*models.User, error) {
	var user models.User
	err := r.db.Where("email = ?", email).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		return nil, &NotFoundError{Resource: "user", ID: email}
	}
	return &user, err
}

// Upsert inserts the user or updates email, name, and saml_idp_issuer if the
// saml_subject already exists. Role is never overwritten by the IdP.
func (r *userRepository) Upsert(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "saml_subject"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"email", "name", "saml_idp_issuer", "last_login_at", "updated_at",
		}),
	}).Create(user).Error
}

func (r *userRepository) UpdateLastLogin(id uuid.UUID, at time.Time) error {
	return r.db.Model(&models.User{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"last_login_at": at,
			"updated_at":    at,
		}).Error
}
