package repository

import (
	"context"
	"fmt"
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
	// Count returns the total number of active (non-deactivated) users.
	Count() (int64, error)
	// CountByRole returns the number of active users with the given role.
	CountByRole(role models.UserRole) (int64, error)
	// Deactivate soft-deletes a user by setting auth_source='deactivated'.
	Deactivate(id uuid.UUID) error
	// CreateAgent inserts an AI agent user. No password hash is set.
	CreateAgent(user *models.User) error
	// SetActive enables or disables a user (used for agent on/off toggle).
	SetActive(id uuid.UUID, active bool) error
	// ListAgents returns all users with auth_source='ai', including inactive ones.
	// The coordinator must additionally check the Active flag before dispatching work.
	ListAgents() ([]models.User, error)
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

// Upsert inserts the user or, if a user with the same email already exists
// (local account or previous SAML login), updates the SAML identity fields.
// Conflicting on email — not saml_subject — lets SAML "adopt" an existing
// local account on first SSO login. Role is never overwritten by the IdP.
func (r *userRepository) Upsert(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "email"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"saml_subject", "name", "saml_idp_issuer", "auth_source", "last_login_at", "updated_at",
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

func (r *userRepository) CreateAgent(user *models.User) error {
	if user.PasswordHash != nil {
		return fmt.Errorf("repository: CreateAgent: password_hash must be nil for AI agent accounts")
	}
	return r.db.Create(user).Error
}

func (r *userRepository) SetActive(id uuid.UUID, active bool) error {
	result := r.db.Model(&models.User{}).
		Where("id = ?", id).
		Update("active", active)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return &NotFoundError{Resource: "user", ID: id.String()}
	}
	return nil
}

func (r *userRepository) ListAgents() ([]models.User, error) {
	var agents []models.User
	err := r.db.Where("auth_source = ?", "ai").Order("name").Find(&agents).Error
	return agents, err
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

func (r *userRepository) Count() (int64, error) {
	var n int64
	err := r.db.Model(&models.User{}).
		Where("auth_source != 'deactivated'").
		Count(&n).Error
	return n, err
}

func (r *userRepository) CountByRole(role models.UserRole) (int64, error) {
	var n int64
	err := r.db.Model(&models.User{}).
		Where("role = ? AND auth_source != 'deactivated'", role).
		Count(&n).Error
	return n, err
}
