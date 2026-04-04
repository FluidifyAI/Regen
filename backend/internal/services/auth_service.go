package services

import (
	"context"
	"fmt"
	"time"

	"github.com/fluidify/regen/internal/models"
	"github.com/fluidify/regen/internal/repository"
)

// AuthService handles user provisioning from SAML assertions.
type AuthService interface {
	UpsertFromSAML(ctx context.Context, subject, issuer, email, name string) error
}

type authService struct {
	userRepo     repository.UserRepository
	ossUserLimit int
}

func NewAuthService(userRepo repository.UserRepository, ossUserLimit int) AuthService {
	return &authService{userRepo: userRepo, ossUserLimit: ossUserLimit}
}

// UpsertFromSAML creates or updates a user from a SAML assertion.
// Called on every successful SSO login — safe to call repeatedly.
// Setting AuthSource="saml" on conflict ensures that a local account adopted
// via SSO can no longer authenticate with a local password (prevents dual-path).
// Returns ErrUserLimitReached if this would be a new user and the OSS limit is hit.
func (s *authService) UpsertFromSAML(ctx context.Context, subject, issuer, email, name string) error {
	// Check if this is an existing user (by email or subject) — existing users
	// are always allowed to log in regardless of the limit.
	existing, _ := s.userRepo.GetByEmail(email)
	if existing == nil {
		existing, _ = s.userRepo.GetBySubject(subject)
	}
	if existing == nil {
		// New user — check limit before provisioning
		count, err := s.userRepo.Count()
		if err != nil {
			return fmt.Errorf("failed to check user count: %w", err)
		}
		if count >= int64(s.ossUserLimit) {
			return fmt.Errorf("user_limit_reached: your team has reached the %d-user community edition limit — upgrade to Fluidify Pro for unlimited users", s.ossUserLimit)
		}
	}

	now := time.Now()
	user := &models.User{
		SAMLSubject:   &subject,
		SAMLIDPIssuer: issuer,
		Email:         email,
		Name:          name,
		AuthSource:    "saml",
		LastLoginAt:   &now,
	}
	return s.userRepo.Upsert(ctx, user)
}
