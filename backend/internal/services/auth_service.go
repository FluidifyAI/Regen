package services

import (
	"context"
	"time"

	"github.com/fluidify/regen/internal/models"
	"github.com/fluidify/regen/internal/repository"
)

// AuthService handles user provisioning from SAML assertions.
type AuthService interface {
	UpsertFromSAML(ctx context.Context, subject, issuer, email, name string) error
}

type authService struct {
	userRepo repository.UserRepository
}

func NewAuthService(userRepo repository.UserRepository) AuthService {
	return &authService{userRepo: userRepo}
}

// UpsertFromSAML creates or updates a user from a SAML assertion.
// Called on every successful SSO login — safe to call repeatedly.
// Setting AuthSource="saml" on conflict ensures that a local account adopted
// via SSO can no longer authenticate with a local password (prevents dual-path).
func (s *authService) UpsertFromSAML(ctx context.Context, subject, issuer, email, name string) error {
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
