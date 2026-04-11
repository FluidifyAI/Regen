package services_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/FluidifyAI/Regen/backend/internal/services"
	"golang.org/x/crypto/bcrypt"
)

// ── In-memory stub implementations ──────────────────────────────────────────

type stubUserRepo struct {
	users map[string]*models.User // keyed by email
}

func newStubUserRepo() *stubUserRepo {
	return &stubUserRepo{users: map[string]*models.User{}}
}

func (r *stubUserRepo) GetBySubject(subject string) (*models.User, error) {
	for _, u := range r.users {
		if u.SAMLSubject != nil && *u.SAMLSubject == subject {
			return u, nil
		}
	}
	return nil, &repository.NotFoundError{Resource: "user", ID: subject}
}

func (r *stubUserRepo) GetByEmail(email string) (*models.User, error) {
	if u, ok := r.users[email]; ok {
		return u, nil
	}
	return nil, &repository.NotFoundError{Resource: "user", ID: email}
}

func (r *stubUserRepo) Upsert(_ context.Context, user *models.User) error {
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	r.users[user.Email] = user
	return nil
}

func (r *stubUserRepo) UpdateLastLogin(id uuid.UUID, at time.Time) error {
	for _, u := range r.users {
		if u.ID == id {
			u.LastLoginAt = &at
			return nil
		}
	}
	return &repository.NotFoundError{Resource: "user", ID: id.String()}
}

func (r *stubUserRepo) CreateLocal(user *models.User) error {
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	if _, exists := r.users[user.Email]; exists {
		return &repository.AlreadyExistsError{Resource: "user", Field: "email", Value: user.Email}
	}
	r.users[user.Email] = user
	return nil
}

func (r *stubUserRepo) ListAll() ([]models.User, error) {
	out := make([]models.User, 0, len(r.users))
	for _, u := range r.users {
		out = append(out, *u)
	}
	return out, nil
}

func (r *stubUserRepo) GetByID(id uuid.UUID) (*models.User, error) {
	for _, u := range r.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, &repository.NotFoundError{Resource: "user", ID: id.String()}
}

func (r *stubUserRepo) Update(user *models.User) error {
	existing, err := r.GetByID(user.ID)
	if err != nil {
		return err
	}
	existing.Name = user.Name
	existing.Role = user.Role
	existing.PasswordHash = user.PasswordHash
	existing.UpdatedAt = user.UpdatedAt
	return nil
}

func (r *stubUserRepo) Count() (int64, error) {
	var n int64
	for _, u := range r.users {
		if u.AuthSource != "deactivated" {
			n++
		}
	}
	return n, nil
}

func (r *stubUserRepo) CountByRole(role models.UserRole) (int64, error) {
	var n int64
	for _, u := range r.users {
		if u.AuthSource != "deactivated" && u.Role == role {
			n++
		}
	}
	return n, nil
}

func (r *stubUserRepo) Deactivate(id uuid.UUID) error {
	for _, u := range r.users {
		if u.ID == id {
			u.AuthSource = "deactivated"
			u.PasswordHash = nil
			return nil
		}
	}
	return &repository.NotFoundError{Resource: "user", ID: id.String()}
}

func (r *stubUserRepo) RestoreAgent(id uuid.UUID) error {
	for _, u := range r.users {
		if u.ID == id {
			u.AuthSource = "ai"
			u.Active = true
			return nil
		}
	}
	return &repository.NotFoundError{Resource: "user", ID: id.String()}
}

func (r *stubUserRepo) CreateAgent(user *models.User) error {
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	r.users[user.Email] = user
	return nil
}

func (r *stubUserRepo) SetActive(id uuid.UUID, active bool) error {
	for _, u := range r.users {
		if u.ID == id {
			u.Active = active
			return nil
		}
	}
	return &repository.NotFoundError{Resource: "user", ID: id.String()}
}

func (r *stubUserRepo) ListAgents() ([]models.User, error) {
	var out []models.User
	for _, u := range r.users {
		if u.AuthSource == "ai" {
			out = append(out, *u)
		}
	}
	return out, nil
}

func (r *stubUserRepo) GetBySlackUserID(slackUserID string) (*models.User, error) {
	for _, u := range r.users {
		if u.SlackUserID != nil && *u.SlackUserID == slackUserID {
			return u, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

func (r *stubUserRepo) GetByTeamsUserID(teamsUserID string) (*models.User, error) {
	for _, u := range r.users {
		if u.TeamsUserID != nil && *u.TeamsUserID == teamsUserID {
			return u, nil
		}
	}
	return nil, &repository.NotFoundError{Resource: "user", ID: teamsUserID}
}

// ── stubSessionRepo ──────────────────────────────────────────────────────────

type stubSessionRepo struct {
	sessions map[string]*models.LocalSession
}

func newStubSessionRepo() *stubSessionRepo {
	return &stubSessionRepo{sessions: map[string]*models.LocalSession{}}
}

func (r *stubSessionRepo) Create(userID uuid.UUID) (*models.LocalSession, error) {
	s := &models.LocalSession{
		Token:     uuid.NewString(),
		UserID:    userID,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		CreatedAt: time.Now(),
	}
	r.sessions[s.Token] = s
	return s, nil
}

func (r *stubSessionRepo) GetByToken(token string) (*models.LocalSession, error) {
	if s, ok := r.sessions[token]; ok {
		if time.Now().Before(s.ExpiresAt) {
			return s, nil
		}
		delete(r.sessions, token)
	}
	return nil, &repository.NotFoundError{Resource: "local_session", ID: token}
}

func (r *stubSessionRepo) DeleteByToken(token string) error {
	delete(r.sessions, token)
	return nil
}

func (r *stubSessionRepo) DeleteExpired() error {
	now := time.Now()
	for token, s := range r.sessions {
		if !now.Before(s.ExpiresAt) {
			delete(r.sessions, token)
		}
	}
	return nil
}

func (r *stubSessionRepo) DeleteByUserID(userID uuid.UUID) error {
	for token, s := range r.sessions {
		if s.UserID == userID {
			delete(r.sessions, token)
		}
	}
	return nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func newSvc() services.LocalAuthService {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), 12)
	h := string(hash)

	userRepo := newStubUserRepo()
	userRepo.users["alice@test.com"] = &models.User{
		ID:           uuid.New(),
		Email:        "alice@test.com",
		Name:         "Alice",
		PasswordHash: &h,
		AuthSource:   "local",
		Role:         models.UserRoleMember,
	}

	sessionRepo := newStubSessionRepo()
	return services.NewLocalAuthService(userRepo, sessionRepo)
}

// ── Tests ────────────────────────────────────────────────────────────────────

func TestLogin_ValidCredentials(t *testing.T) {
	svc := newSvc()
	sess, err := svc.Login("alice@test.com", "password123")
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if sess.Token == "" {
		t.Error("expected non-empty token")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	svc := newSvc()
	_, err := svc.Login("alice@test.com", "wrongpassword")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
	if err.Error() != "invalid email or password" {
		t.Errorf("unexpected error message: %q", err.Error())
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	svc := newSvc()
	_, err := svc.Login("nobody@test.com", "password123")
	if err == nil {
		t.Fatal("expected error for unknown email")
	}
	// Must not reveal whether the email exists (prevent user enumeration)
	if err.Error() != "invalid email or password" {
		t.Errorf("unexpected error message (user enumeration risk): %q", err.Error())
	}
}

func TestCreateUser_ThenLogin(t *testing.T) {
	svc := newSvc()
	_, _, err := svc.CreateUser("bob@test.com", "Bob", "mypassword!", models.UserRoleMember)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	sess, err := svc.Login("bob@test.com", "mypassword!")
	if err != nil {
		t.Fatalf("Login after CreateUser failed: %v", err)
	}
	if sess.Token == "" {
		t.Error("expected non-empty session token")
	}
}

func TestLogout_InvalidatesSession(t *testing.T) {
	svc := newSvc()
	sess, _ := svc.Login("alice@test.com", "password123")
	if err := svc.Logout(sess.Token); err != nil {
		t.Fatalf("Logout failed: %v", err)
	}
	_, err := svc.GetSessionUser(sess.Token)
	if err == nil {
		t.Fatal("expected error after logout — session should be invalidated")
	}
}
