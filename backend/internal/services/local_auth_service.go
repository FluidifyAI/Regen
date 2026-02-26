package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

// LocalAuthService handles local email/password authentication.
type LocalAuthService interface {
	Login(email, password string) (*models.LocalSession, error)
	Logout(token string) error
	GetSessionUser(token string) (*models.User, error)
	CreateUser(email, name, password string, role models.UserRole) (*models.User, string, error)
	UpdateUser(id uuid.UUID, name string, role models.UserRole, newPassword string) error
	ResetPassword(id uuid.UUID) (string, error)
	DeactivateUser(id uuid.UUID) error
	ListUsers() ([]models.User, error)
	CountUsers() (int64, error)
}

type localAuthService struct {
	users    repository.UserRepository
	sessions repository.LocalSessionRepository
}

func NewLocalAuthService(users repository.UserRepository, sessions repository.LocalSessionRepository) LocalAuthService {
	return &localAuthService{users: users, sessions: sessions}
}

func (s *localAuthService) Login(email, password string) (*models.LocalSession, error) {
	user, err := s.users.GetByEmail(email)
	if err != nil {
		// Return generic error to avoid user enumeration
		return nil, errors.New("invalid email or password")
	}
	if user.AuthSource != "local" || user.PasswordHash == nil {
		return nil, errors.New("invalid email or password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("invalid email or password")
	}
	// Lazy cleanup of expired sessions
	_ = s.sessions.DeleteExpired()

	session, err := s.sessions.Create(user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	now := time.Now()
	_ = s.users.UpdateLastLogin(user.ID, now)
	return session, nil
}

func (s *localAuthService) Logout(token string) error {
	return s.sessions.DeleteByToken(token)
}

func (s *localAuthService) GetSessionUser(token string) (*models.User, error) {
	sess, err := s.sessions.GetByToken(token)
	if err != nil {
		return nil, err
	}
	return s.users.GetByID(sess.UserID)
}

// CreateUser creates a new local user with a bcrypt password hash.
// Returns the user and a one-time setup token.
func (s *localAuthService) CreateUser(email, name, password string, role models.UserRole) (*models.User, string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, "", fmt.Errorf("failed to hash password: %w", err)
	}
	hashStr := string(hash)
	user := &models.User{
		Email:        email,
		Name:         name,
		Role:         role,
		PasswordHash: &hashStr,
		AuthSource:   "local",
	}
	if err := s.users.CreateLocal(user); err != nil {
		return nil, "", err
	}
	// Generate a one-time setup session so the inviter can share a direct login link
	sess, err := s.sessions.Create(user.ID)
	if err != nil {
		return user, "", nil // non-fatal
	}
	return user, sess.Token, nil
}

func (s *localAuthService) UpdateUser(id uuid.UUID, name string, role models.UserRole, newPassword string) error {
	user, err := s.users.GetByID(id)
	if err != nil {
		return err
	}
	user.Name = name
	user.Role = role
	if newPassword != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
		if err != nil {
			return err
		}
		hashStr := string(hash)
		user.PasswordHash = &hashStr
	}
	user.UpdatedAt = time.Now()
	return s.users.Update(user)
}

func (s *localAuthService) ResetPassword(id uuid.UUID) (string, error) {
	sess, err := s.sessions.Create(id)
	if err != nil {
		return "", err
	}
	return sess.Token, nil
}

func (s *localAuthService) DeactivateUser(id uuid.UUID) error {
	return s.users.Deactivate(id)
}

func (s *localAuthService) ListUsers() ([]models.User, error) {
	return s.users.ListAll()
}

func (s *localAuthService) CountUsers() (int64, error) {
	users, err := s.users.ListAll()
	if err != nil {
		return 0, err
	}
	return int64(len(users)), nil
}
