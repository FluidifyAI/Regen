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
	GetUser(id uuid.UUID) (*models.User, error)
	ListUsers() ([]models.User, error)
	CountUsers() (int64, error)
	CountAdmins() (int64, error)
	// GetUserByEmail returns the user with the given email, regardless of auth source.
	// Used by middleware to resolve a SAML JWT session to a DB user record.
	GetUserByEmail(email string) (*models.User, error)
}

// dummyHash is computed once at startup and used in Login to ensure constant-time
// responses regardless of whether the email exists in the database.
var dummyHash []byte

func init() {
	h, _ := bcrypt.GenerateFromPassword([]byte("dummy-constant-time"), 12)
	dummyHash = h
}

type localAuthService struct {
	users    repository.UserRepository
	sessions repository.LocalSessionRepository
}

func NewLocalAuthService(users repository.UserRepository, sessions repository.LocalSessionRepository) LocalAuthService {
	return &localAuthService{users: users, sessions: sessions}
}

func (s *localAuthService) Login(email, password string) (*models.LocalSession, error) {
	user, lookupErr := s.users.GetByEmail(email)

	// Always run a bcrypt comparison to prevent timing-based user enumeration.
	// If the user wasn't found or isn't a local user, compare against a dummy hash
	// (guaranteed to fail) but take roughly the same time.
	hashToCompare := dummyHash
	if lookupErr == nil && user.AuthSource == "local" && user.PasswordHash != nil {
		hashToCompare = []byte(*user.PasswordHash)
	}
	compareErr := bcrypt.CompareHashAndPassword(hashToCompare, []byte(password))

	if lookupErr != nil || user.AuthSource != "local" || user.PasswordHash == nil || compareErr != nil {
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
	if len(password) < 8 {
		return nil, "", fmt.Errorf("password must be at least 8 characters")
	}
	// bcrypt silently truncates input at 72 bytes — enforce the limit explicitly
	// so two different passwords cannot produce the same hash.
	if len(password) > 72 {
		return nil, "", fmt.Errorf("password must be at most 72 characters")
	}
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

// UpdateUser updates a user's name, role, and optionally their password.
// The handler pre-fetches the user to resolve any unset fields; this method
// does not re-fetch from the database to avoid a redundant round-trip.
func (s *localAuthService) UpdateUser(id uuid.UUID, name string, role models.UserRole, newPassword string) error {
	if newPassword != "" && len(newPassword) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if newPassword != "" && len(newPassword) > 72 {
		return fmt.Errorf("password must be at most 72 characters")
	}

	user := &models.User{ID: id}
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

// ResetPassword issues a one-time setup session for the given user.
// Only local accounts are eligible — SAML users have no password to reset.
func (s *localAuthService) ResetPassword(id uuid.UUID) (string, error) {
	user, err := s.users.GetByID(id)
	if err != nil {
		return "", err
	}
	if user.AuthSource != "local" {
		return "", fmt.Errorf("password reset is only available for local accounts")
	}
	sess, err := s.sessions.Create(id)
	if err != nil {
		return "", err
	}
	return sess.Token, nil
}

func (s *localAuthService) DeactivateUser(id uuid.UUID) error {
	if err := s.users.Deactivate(id); err != nil {
		return err
	}
	// Invalidate all active sessions immediately so the user cannot continue
	// using a live cookie after their account is deactivated.
	_ = s.sessions.DeleteByUserID(id)
	return nil
}

func (s *localAuthService) GetUser(id uuid.UUID) (*models.User, error) {
	return s.users.GetByID(id)
}

func (s *localAuthService) ListUsers() ([]models.User, error) {
	return s.users.ListAll()
}

func (s *localAuthService) CountUsers() (int64, error) {
	return s.users.Count()
}

// CountAdmins returns the number of active admin accounts.
func (s *localAuthService) CountAdmins() (int64, error) {
	return s.users.CountByRole(models.UserRoleAdmin)
}

func (s *localAuthService) GetUserByEmail(email string) (*models.User, error) {
	return s.users.GetByEmail(email)
}
