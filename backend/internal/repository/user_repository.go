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
	// CreateLocal creates a new locally-authenticated user with a bcrypt password hash.
	CreateLocal(user *models.User) error
	// ListAll returns all users ordered by created_at ASC.
	ListAll() ([]models.User, error)
	// GetByID retrieves a user by primary key.
	GetByID(id uuid.UUID) (*models.User, error)
	// Update saves changed fields (name, role, password_hash) for a user.
	Update(user *models.User) error
	// Deactivate soft-deletes a user by setting auth_source='deactivated'.
	Deactivate(id uuid.UUID) error
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

func (r *userRepository) CreateLocal(user *models.User) error {
	return r.db.Create(user).Error
}

func (r *userRepository) ListAll() ([]models.User, error) {
	var users []models.User
	err := r.db.Order("created_at ASC").Find(&users).Error
	return users, err
}

func (r *userRepository) GetByID(id uuid.UUID) (*models.User, error) {
	var user models.User
	err := r.db.First(&user, "id = ?", id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, &NotFoundError{Resource: "user", ID: id.String()}
	}
	return &user, err
}

func (r *userRepository) Update(user *models.User) error {
	result := r.db.Model(user).
		Select("name", "role", "password_hash", "updated_at").
		Updates(map[string]any{
			"name":          user.Name,
			"role":          user.Role,
			"password_hash": user.PasswordHash,
			"updated_at":    user.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return &NotFoundError{Resource: "user", ID: user.ID.String()}
	}
	return nil
}

func (r *userRepository) Deactivate(id uuid.UUID) error {
	result := r.db.Model(&models.User{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"auth_source":   "deactivated",
			"password_hash": nil,
			"updated_at":    time.Now(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return &NotFoundError{Resource: "user", ID: id.String()}
	}
	return nil
}
