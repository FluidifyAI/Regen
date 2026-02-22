package services

import (
	"context"
	"time"

	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/repository"
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
func (s *authService) UpsertFromSAML(ctx context.Context, subject, issuer, email, name string) error {
	now := time.Now()
	user := &models.User{
		SAMLSubject:   subject,
		SAMLIDPIssuer: issuer,
		Email:         email,
		Name:          name,
		LastLoginAt:   &now,
	}
	return s.userRepo.Upsert(ctx, user)
}
