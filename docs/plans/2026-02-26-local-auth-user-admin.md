# Local Auth + User Admin Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add email/password authentication and a `/settings/users` admin panel so teams without SSO can have real user identities, invite colleagues, and manage accounts.

**Architecture:** Extend the existing `users` table with `password_hash` + `auth_source` columns and add a `local_sessions` table. `RequireAuth` middleware gains a local-session check that runs before the existing SAML check — SAML path is completely unchanged. Frontend gains a local login form, a Settings nav item, and a `/settings/users` admin page.

**Tech Stack:** Go + Gin + GORM (backend), `golang.org/x/crypto/bcrypt` (already in go.mod), React + TypeScript + TailwindCSS (frontend), PostgreSQL.

**Design doc:** `docs/plans/2026-02-26-local-auth-user-admin-design.md`

---

## Task 1: Migration — add local auth columns + local_sessions table

**Files:**
- Create: `backend/migrations/000024_add_local_auth.up.sql`
- Create: `backend/migrations/000024_add_local_auth.down.sql`

**Step 1: Write the up migration**

```sql
-- backend/migrations/000024_add_local_auth.up.sql

-- Allow saml_subject to be NULL for locally-created users.
-- SAML-provisioned users keep their existing saml_subject value.
ALTER TABLE users
    ALTER COLUMN saml_subject DROP NOT NULL;

-- Local auth columns
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS password_hash TEXT,
    ADD COLUMN IF NOT EXISTS auth_source   VARCHAR(20) NOT NULL DEFAULT 'saml'
        CHECK (auth_source IN ('saml', 'local'));

-- Local session store
CREATE TABLE IF NOT EXISTS local_sessions (
    token       TEXT        PRIMARY KEY,
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_local_sessions_user_id    ON local_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_local_sessions_expires_at ON local_sessions(expires_at);
```

**Step 2: Write the down migration**

```sql
-- backend/migrations/000024_add_local_auth.down.sql
DROP TABLE IF EXISTS local_sessions;
ALTER TABLE users DROP COLUMN IF EXISTS password_hash;
ALTER TABLE users DROP COLUMN IF EXISTS auth_source;
ALTER TABLE users ALTER COLUMN saml_subject SET NOT NULL;
```

**Step 3: Verify migration files exist**

```bash
ls backend/migrations/000024_add_local_auth.*
```
Expected: two files listed.

**Step 4: Commit**

```bash
git add backend/migrations/000024_add_local_auth.up.sql backend/migrations/000024_add_local_auth.down.sql
git commit -m "feat(auth): add local_sessions table and local auth columns to users"
```

---

## Task 2: Backend — update User model + UserRepository

**Files:**
- Modify: `backend/internal/models/user.go`
- Modify: `backend/internal/repository/user_repository.go`
- Create: `backend/internal/models/local_session.go`

**Context:** `models/user.go` currently has `SAMLSubject string ... not null;uniqueIndex`. We need to make it nullable and add two new fields. `user_repository.go` needs new methods for local auth.

**Step 1: Update `models/user.go`**

Replace the struct definition (keep `TableName` and `BeforeCreate` unchanged):

```go
// User represents a person authenticated via SAML SSO or local email/password.
type User struct {
	ID    uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Email string    `gorm:"type:varchar(255);not null;uniqueIndex"          json:"email"`
	Name  string    `gorm:"type:varchar(255);not null;default:''"           json:"name"`

	// SAMLSubject is the NameID from the SAML assertion.
	// NULL for locally-created users.
	SAMLSubject   *string `gorm:"type:varchar(500);uniqueIndex;column:saml_subject" json:"-"`
	SAMLIDPIssuer string  `gorm:"type:varchar(500);not null;default:'';column:saml_idp_issuer" json:"-"`

	// Local auth fields — only set for auth_source='local'.
	PasswordHash *string `gorm:"type:text;column:password_hash" json:"-"`
	AuthSource   string  `gorm:"type:varchar(20);not null;default:'saml';column:auth_source" json:"-"`

	Role        UserRole   `gorm:"type:varchar(50);not null;default:'member'" json:"role"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`

	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`
}
```

**Step 2: Create `models/local_session.go`**

```go
package models

import (
	"time"
	"github.com/google/uuid"
)

// LocalSession stores a session token for locally-authenticated users.
type LocalSession struct {
	Token     string    `gorm:"type:text;primaryKey"                           json:"-"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index"                       json:"-"`
	ExpiresAt time.Time `gorm:"not null"                                       json:"-"`
	CreatedAt time.Time `gorm:"not null;default:now()"                         json:"-"`
}

func (LocalSession) TableName() string { return "local_sessions" }
```

**Step 3: Add new methods to `UserRepository` interface** (add to the interface block in `user_repository.go`)

```go
// CreateLocal creates a new locally-authenticated user with a bcrypt password hash.
CreateLocal(user *models.User) error
// ListAll returns all users ordered by created_at ASC.
ListAll() ([]models.User, error)
// GetByID retrieves a user by primary key.
GetByID(id uuid.UUID) (*models.User, error)
// Update saves changed fields (name, role, password_hash) for a user.
Update(user *models.User) error
// Deactivate soft-deletes a user by blanking their password and marking
// auth_source='deactivated'. We don't hard-delete to preserve timeline entries.
Deactivate(id uuid.UUID) error
```

**Step 4: Implement the new methods** (add after existing implementations in `user_repository.go`)

```go
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
		Updates(user)
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
```

**Step 5: Build to verify no compile errors**

```bash
cd backend && go build ./...
```
Expected: no output (clean build).

**Step 6: Commit**

```bash
git add backend/internal/models/ backend/internal/repository/user_repository.go
git commit -m "feat(auth): extend User model and UserRepository for local auth"
```

---

## Task 3: Backend — LocalSessionRepository

**Files:**
- Create: `backend/internal/repository/local_session_repository.go`

**Step 1: Create the file**

```go
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
		return nil, &NotFoundError{Resource: "local_session", ID: token[:8] + "..."}
	}
	return &s, err
}

func (r *localSessionRepository) DeleteByToken(token string) error {
	return r.db.Delete(&models.LocalSession{}, "token = ?", token).Error
}

func (r *localSessionRepository) DeleteExpired() error {
	return r.db.Delete(&models.LocalSession{}, "expires_at <= NOW()").Error
}
```

**Step 2: Build**

```bash
cd backend && go build ./...
```

**Step 3: Commit**

```bash
git add backend/internal/repository/local_session_repository.go
git commit -m "feat(auth): add LocalSessionRepository"
```

---

## Task 4: Backend — extend RequireAuth middleware + LocalAuthService

**Files:**
- Create: `backend/internal/services/local_auth_service.go`
- Modify: `backend/internal/api/middleware/auth.go`

**Context:** `RequireAuth` is currently a thin wrapper around SAML. We need it to check a local session cookie first, attach the user to context, and fall through to SAML if not found.

**Step 1: Create `local_auth_service.go`**

```go
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
	// Lazy cleanup of expired sessions for this user
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
// Returns the user and a one-time setup token (empty string if password was provided).
func (s *localAuthService) CreateUser(email, name, password string, role models.UserRole) (*models.User, string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, "", fmt.Errorf("failed to hash password: %w", err)
	}
	hashStr := string(hash)
	authSource := "local"
	user := &models.User{
		Email:        email,
		Name:         name,
		Role:         role,
		PasswordHash: &hashStr,
		AuthSource:   authSource,
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
	// Used for bootstrap detection (zero users → allow first-user creation unauthenticated)
	var count int64
	// Call ListAll and count — or add a dedicated Count method to repo if perf matters
	users, err := s.users.ListAll()
	if err != nil {
		return 0, err
	}
	return int64(len(users)), nil
}
```

**Step 2: Extend `middleware/auth.go`**

Replace the entire file:

```go
package middleware

import (
	"net/http"

	"github.com/crewjam/saml/samlsp"
	"github.com/gin-gonic/gin"
	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/services"
)

const (
	contextKeySAMLSession  = "saml_session"
	contextKeyLocalUser    = "local_user"
	localSessionCookieName = "oi_session"
)

// RequireAuth enforces authentication on a route group.
//
// Priority order:
//  1. Local session cookie (oi_session) — checked first
//  2. SAML session — checked if no local session
//  3. Neither configured → open mode pass-through
func RequireAuth(samlMiddleware *samlsp.Middleware, localAuth ...services.LocalAuthService) gin.HandlerFunc {
	var la services.LocalAuthService
	if len(localAuth) > 0 {
		la = localAuth[0]
	}

	return func(c *gin.Context) {
		// 1. Check local session cookie
		if la != nil {
			if cookie, err := c.Cookie(localSessionCookieName); err == nil && cookie != "" {
				if user, err := la.GetSessionUser(cookie); err == nil {
					c.Set(contextKeyLocalUser, user)
					c.Next()
					return
				}
				// Invalid/expired token — clear the cookie
				clearSessionCookie(c)
			}
		}

		// 2. Check SAML session
		if samlMiddleware != nil {
			session, err := samlMiddleware.Session.GetSession(c.Request)
			if err != nil || session == nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": gin.H{
						"code":    "unauthorized",
						"message": "Authentication required",
					},
				})
				return
			}
			c.Set(contextKeySAMLSession, session)
			c.Next()
			return
		}

		// 3. Open mode — no auth configured
		c.Next()
	}
}

// GetSAMLSession retrieves the SAML session from the Gin context.
func GetSAMLSession(c *gin.Context) samlsp.Session {
	if val, exists := c.Get(contextKeySAMLSession); exists {
		if s, ok := val.(samlsp.Session); ok {
			return s
		}
	}
	return nil
}

// GetLocalUser retrieves the locally-authenticated user from context.
func GetLocalUser(c *gin.Context) *models.User {
	if val, exists := c.Get(contextKeyLocalUser); exists {
		if u, ok := val.(*models.User); ok {
			return u
		}
	}
	return nil
}

// RequireAdmin aborts with 403 if the local user is not an admin.
// No-op in SAML/open mode (RBAC is handled by the SAML IdP or not enforced).
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetLocalUser(c)
		if user != nil && user.Role != models.UserRoleAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": gin.H{"code": "forbidden", "message": "Admin access required"},
			})
			return
		}
		c.Next()
	}
}

func clearSessionCookie(c *gin.Context) {
	c.SetCookie(localSessionCookieName, "", -1, "/", "", false, true)
}
```

**Step 3: Build**

```bash
cd backend && go build ./...
```

Expected: compile errors in `routes.go` because `RequireAuth` signature changed. Fix in next task.

**Step 4: Commit**

```bash
git add backend/internal/services/local_auth_service.go backend/internal/api/middleware/auth.go
git commit -m "feat(auth): add LocalAuthService and extend RequireAuth to check local sessions"
```

---

## Task 5: Backend — auth handlers (login, logout, /auth/me update) + wire routes

**Files:**
- Modify: `backend/internal/api/handlers/auth.go`
- Modify: `backend/internal/api/routes.go`
- Modify: `backend/cmd/openincident/commands/serve.go` (inject LocalAuthService)

**Step 1: Replace `handlers/auth.go`**

```go
package handlers

import (
	"net/http"

	"github.com/crewjam/saml/samlsp"
	"github.com/gin-gonic/gin"
	"github.com/openincident/openincident/internal/api/middleware"
	"github.com/openincident/openincident/internal/services"
)

// Logout clears both local and SAML sessions.
func Logout(samlMiddleware *samlsp.Middleware, localAuth services.LocalAuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Clear local session
		if localAuth != nil {
			if cookie, err := c.Cookie("oi_session"); err == nil {
				_ = localAuth.Logout(cookie)
			}
			c.SetCookie("oi_session", "", -1, "/", "", false, true)
		}
		// Clear SAML session
		if samlMiddleware != nil {
			_ = samlMiddleware.Session.DeleteSession(c.Writer, c.Request)
		}
		c.Redirect(http.StatusFound, "/")
	}
}

// Login handles POST /api/v1/auth/login — email/password authentication.
func Login(localAuth services.LocalAuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Email    string `json:"email"    binding:"required,email"`
			Password string `json:"password" binding:"required,min=1"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": err.Error()}})
			return
		}

		session, err := localAuth.Login(req.Email, req.Password)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "unauthorized", "message": err.Error()}})
			return
		}

		// Set session cookie: 7-day, HttpOnly, SameSite=Lax
		c.SetCookie("oi_session", session.Token, 7*24*3600, "/", "", false, true)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// GetCurrentUser handles GET /api/v1/auth/me.
func GetCurrentUser(localAuth services.LocalAuthService, samlConfigured bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Local session user
		if user := middleware.GetLocalUser(c); user != nil {
			c.JSON(http.StatusOK, gin.H{
				"authenticated": true,
				"id":            user.ID,
				"email":         user.Email,
				"name":          user.Name,
				"role":          user.Role,
				"ssoEnabled":    samlConfigured,
			})
			return
		}

		// SAML session user
		session := middleware.GetSAMLSession(c)
		if session != nil {
			claims, ok := session.(samlsp.JWTSessionClaims)
			if !ok {
				c.JSON(http.StatusOK, gin.H{"authenticated": true, "ssoEnabled": true})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"authenticated": true,
				"email":         claims.Attributes.Get("email"),
				"name":          claims.Attributes.Get("displayName"),
				"ssoEnabled":    true,
			})
			return
		}

		// Open mode / unauthenticated
		c.JSON(http.StatusOK, gin.H{
			"authenticated": false,
			"mode":          "open",
			"ssoEnabled":    samlConfigured,
			"message":       "No active session",
		})
	}
}
```

**Step 2: Update `routes.go`** — find the `RequireAuth` and `Logout` call sites and update signatures

Look for these lines in `routes.go`:

```go
// OLD:
router.GET("/auth/logout", handlers.Logout(samlMiddleware))
v1.GET("/auth/me", middleware.RequireAuth(samlMiddleware), handlers.GetCurrentUser())
```

Replace with:

```go
// NEW:
router.POST("/api/v1/auth/login", handlers.Login(localAuthSvc))
router.GET("/auth/logout", handlers.Logout(samlMiddleware, localAuthSvc))
v1.GET("/auth/me", middleware.RequireAuth(samlMiddleware, localAuthSvc), handlers.GetCurrentUser(localAuthSvc, samlMiddleware != nil))
```

Also update all other `middleware.RequireAuth(samlMiddleware)` calls in routes.go to pass `localAuthSvc` as the second argument:

```go
// Find every occurrence of:
middleware.RequireAuth(samlMiddleware)
// Replace with:
middleware.RequireAuth(samlMiddleware, localAuthSvc)
```

**Step 3: Wire `localAuthSvc` in `serve.go`**

In `serve.go`, after `userRepo` is created, add:

```go
sessionRepo := repository.NewLocalSessionRepository(database.DB)
localAuthSvc := services.NewLocalAuthService(userRepo, sessionRepo)
```

Then pass `localAuthSvc` to `routes.Setup(...)`. Check the `routes.Setup` function signature and update it to accept `services.LocalAuthService`.

**Step 4: Build**

```bash
cd backend && go build ./...
```
Expected: clean build.

**Step 5: Commit**

```bash
git add backend/internal/api/handlers/auth.go backend/internal/api/routes.go backend/cmd/openincident/commands/serve.go
git commit -m "feat(auth): add login/logout endpoints and update /auth/me with ssoEnabled field"
```

---

## Task 6: Backend — /settings/users CRUD handlers

**Files:**
- Create: `backend/internal/api/handlers/settings_users.go`
- Create: `backend/internal/api/handlers/dto/settings_users_request.go`
- Create: `backend/internal/api/handlers/dto/settings_users_response.go`
- Modify: `backend/internal/api/routes.go`

**Step 1: Create DTOs**

```go
// backend/internal/api/handlers/dto/settings_users_request.go
package dto

type CreateUserRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Name     string `json:"name"     binding:"required,min=1"`
	Role     string `json:"role"     binding:"required,oneof=admin member viewer"`
	Password string `json:"password" binding:"required,min=8"`
}

type UpdateUserRequest struct {
	Name     string `json:"name"`
	Role     string `json:"role"     binding:"omitempty,oneof=admin member viewer"`
	Password string `json:"password" binding:"omitempty,min=8"`
}
```

```go
// backend/internal/api/handlers/dto/settings_users_response.go
package dto

import (
	"time"
	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/models"
)

type UserResponse struct {
	ID          uuid.UUID  `json:"id"`
	Email       string     `json:"email"`
	Name        string     `json:"name"`
	Role        string     `json:"role"`
	AuthSource  string     `json:"auth_source"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

func UserToResponse(u models.User) UserResponse {
	return UserResponse{
		ID:          u.ID,
		Email:       u.Email,
		Name:        u.Name,
		Role:        string(u.Role),
		AuthSource:  u.AuthSource,
		LastLoginAt: u.LastLoginAt,
		CreatedAt:   u.CreatedAt,
	}
}
```

**Step 2: Create `settings_users.go`**

```go
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/api/handlers/dto"
	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/services"
)

// ListUsers handles GET /api/v1/settings/users
func ListUsers(localAuth services.LocalAuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		users, err := localAuth.ListUsers()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "failed to list users"}})
			return
		}
		resp := make([]dto.UserResponse, len(users))
		for i, u := range users {
			resp[i] = dto.UserToResponse(u)
		}
		c.JSON(http.StatusOK, gin.H{"users": resp})
	}
}

// CreateUser handles POST /api/v1/settings/users
func CreateUser(localAuth services.LocalAuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dto.CreateUserRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}
		user, setupToken, err := localAuth.CreateUser(req.Email, req.Name, req.Password, models.UserRole(req.Role))
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}
		c.JSON(http.StatusCreated, gin.H{
			"user":        dto.UserToResponse(*user),
			"setup_token": setupToken, // one-time login link token; empty if unused
		})
	}
}

// UpdateUser handles PATCH /api/v1/settings/users/:id
func UpdateUser(localAuth services.LocalAuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid user id"}})
			return
		}
		var req dto.UpdateUserRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}
		if err := localAuth.UpdateUser(id, req.Name, models.UserRole(req.Role), req.Password); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// DeactivateUser handles DELETE /api/v1/settings/users/:id
func DeactivateUser(localAuth services.LocalAuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid user id"}})
			return
		}
		if err := localAuth.DeactivateUser(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// ResetUserPassword handles POST /api/v1/settings/users/:id/reset-password
func ResetUserPassword(localAuth services.LocalAuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid user id"}})
			return
		}
		token, err := localAuth.ResetPassword(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"setup_token": token})
	}
}
```

**Step 3: Register settings routes in `routes.go`**

Find the protected `v1` group and add after existing routes:

```go
// Settings — admin only
settingsGroup := v1.Group("/settings", middleware.RequireAdmin())
{
    settingsGroup.GET("/users", handlers.ListUsers(localAuthSvc))
    settingsGroup.POST("/users", handlers.CreateUser(localAuthSvc))
    settingsGroup.PATCH("/users/:id", handlers.UpdateUser(localAuthSvc))
    settingsGroup.DELETE("/users/:id", handlers.DeactivateUser(localAuthSvc))
    settingsGroup.POST("/users/:id/reset-password", handlers.ResetUserPassword(localAuthSvc))
}
```

**Step 4: Build**

```bash
cd backend && go build ./...
```

**Step 5: Commit**

```bash
git add backend/internal/api/handlers/settings_users.go \
        backend/internal/api/handlers/dto/settings_users_request.go \
        backend/internal/api/handlers/dto/settings_users_response.go \
        backend/internal/api/routes.go
git commit -m "feat(auth): add /settings/users CRUD endpoints"
```

---

## Task 7: Backend — tests for LocalAuthService

**Files:**
- Create: `backend/internal/services/local_auth_service_test.go`

**Step 1: Write tests**

```go
package services_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/services"
	"golang.org/x/crypto/bcrypt"
)

// ── minimal in-memory stubs ──────────────────────────────────────────────────

type stubUserRepo struct {
	users map[string]*models.User // keyed by email
}

func (r *stubUserRepo) GetByEmail(email string) (*models.User, error) {
	if u, ok := r.users[email]; ok {
		return u, nil
	}
	return nil, &stubNotFound{}
}
func (r *stubUserRepo) GetByID(id uuid.UUID) (*models.User, error) {
	for _, u := range r.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, &stubNotFound{}
}
func (r *stubUserRepo) CreateLocal(u *models.User) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	r.users[u.Email] = u
	return nil
}
func (r *stubUserRepo) Update(u *models.User) error           { r.users[u.Email] = u; return nil }
func (r *stubUserRepo) Deactivate(id uuid.UUID) error         { return nil }
func (r *stubUserRepo) ListAll() ([]models.User, error)       { return nil, nil }
func (r *stubUserRepo) GetBySubject(s string) (*models.User, error) { return nil, &stubNotFound{} }
func (r *stubUserRepo) Upsert(_ interface{}, u *models.User) error  { return nil }
func (r *stubUserRepo) UpdateLastLogin(id uuid.UUID, _ interface{}) error { return nil }

type stubSessionRepo struct {
	sessions map[string]*models.LocalSession
}

func (r *stubSessionRepo) Create(userID uuid.UUID) (*models.LocalSession, error) {
	s := &models.LocalSession{Token: uuid.NewString(), UserID: userID}
	r.sessions[s.Token] = s
	return s, nil
}
func (r *stubSessionRepo) GetByToken(token string) (*models.LocalSession, error) {
	if s, ok := r.sessions[token]; ok {
		return s, nil
	}
	return nil, &stubNotFound{}
}
func (r *stubSessionRepo) DeleteByToken(token string) error { delete(r.sessions, token); return nil }
func (r *stubSessionRepo) DeleteExpired() error             { return nil }

type stubNotFound struct{}
func (e *stubNotFound) Error() string { return "not found" }

func newSvc() services.LocalAuthService {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), 12)
	h := string(hash)
	src := "local"
	users := &stubUserRepo{users: map[string]*models.User{
		"alice@test.com": {
			ID: uuid.New(), Email: "alice@test.com", Name: "Alice",
			PasswordHash: &h, AuthSource: src, Role: models.UserRoleMember,
		},
	}}
	sessions := &stubSessionRepo{sessions: map[string]*models.LocalSession{}}
	return services.NewLocalAuthService(users, sessions)
}

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
	_, err := svc.Login("alice@test.com", "wrong")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	svc := newSvc()
	_, err := svc.Login("nobody@test.com", "password123")
	if err == nil {
		t.Fatal("expected error for unknown email")
	}
	// Must not reveal whether the email exists
	if err.Error() != "invalid email or password" {
		t.Errorf("unexpected error message: %q", err.Error())
	}
}

func TestCreateUser_ThenLogin(t *testing.T) {
	svc := newSvc()
	_, _, err := svc.CreateUser("bob@test.com", "Bob", "mypassword!", models.UserRoleMember)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	_, err = svc.Login("bob@test.com", "mypassword!")
	if err != nil {
		t.Fatalf("Login after CreateUser failed: %v", err)
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
		t.Fatal("expected error after logout")
	}
}
```

**Step 2: Run tests**

```bash
cd backend && go test ./internal/services/... -run TestLogin -v
```
Expected: all 4 tests PASS.

**Step 3: Commit**

```bash
git add backend/internal/services/local_auth_service_test.go
git commit -m "test(auth): add LocalAuthService unit tests"
```

---

## Task 8: Frontend — update AuthContext + api/auth.ts

**Files:**
- Modify: `frontend/src/api/auth.ts`
- Modify: `frontend/src/contexts/AuthContext.tsx`

**Step 1: Update `api/auth.ts`**

```typescript
import { apiClient } from './client'

export interface CurrentUser {
  authenticated: boolean
  mode?: 'open'
  message?: string
  email?: string
  name?: string
  id?: string
  role?: 'admin' | 'member' | 'viewer'
  ssoEnabled?: boolean
}

export interface LoginRequest {
  email: string
  password: string
}

export async function getCurrentUser(): Promise<CurrentUser> {
  return apiClient.get<CurrentUser>('/api/v1/auth/me')
}

export async function login(req: LoginRequest): Promise<void> {
  await apiClient.post('/api/v1/auth/login', req)
}
```

**Step 2: Update `AuthContext.tsx`** — add `ssoEnabled` to state

```typescript
interface AuthState {
  user: CurrentUser | null
  loading: boolean
  authenticated: boolean
  openMode: boolean
  ssoEnabled: boolean   // ← new
}

// in AuthProvider value:
const value: AuthState = {
  user,
  loading,
  authenticated: user?.authenticated === true,
  openMode: user?.mode === 'open',
  ssoEnabled: user?.ssoEnabled === true,
}
```

**Step 3: Update `hooks/useAuth.ts`** if it re-exports AuthContext (check the file; update exported type if needed).

**Step 4: Commit**

```bash
git add frontend/src/api/auth.ts frontend/src/contexts/AuthContext.tsx frontend/src/hooks/useAuth.ts
git commit -m "feat(auth): add login API + ssoEnabled to AuthContext"
```

---

## Task 9: Frontend — update LoginPage with local login form

**Files:**
- Modify: `frontend/src/pages/LoginPage.tsx`
- Modify: `frontend/src/components/auth/AuthGate.tsx`

**Step 1: Replace `LoginPage.tsx`**

Keep the existing card/background styling. Add a local email+password form above the SSO button. Show SSO button only when `ssoEnabled=true`.

```tsx
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { login } from '../api/auth'
import { useAuth } from '../hooks/useAuth'

export function LoginPage() {
  const { ssoEnabled } = useAuth()
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      await login({ email, password })
      navigate('/', { replace: true })
      window.location.reload() // refresh AuthContext
    } catch {
      setError('Invalid email or password')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div
      className="min-h-screen bg-[#0F172A] flex items-center justify-center p-4"
      style={{
        backgroundImage: `radial-gradient(circle, #1E293B 1px, transparent 1px)`,
        backgroundSize: '28px 28px',
      }}
    >
      <div className="relative w-full max-w-sm">
        <div
          className="rounded-2xl border border-[#1E293B] bg-[#0F172A] p-10"
          style={{ boxShadow: '0 0 0 1px #1E293B, 0 24px 48px rgba(0,0,0,0.4)' }}
        >
          {/* Logo */}
          <div className="flex flex-col items-center mb-8">
            <div className="w-14 h-14 rounded-xl bg-[#1E3A5F] flex items-center justify-center border border-[#2563EB] border-opacity-30 mb-4">
              <ShieldIcon className="w-7 h-7 text-[#2563EB]" />
            </div>
            <h1 className="text-[#F1F5F9] text-xl font-semibold tracking-tight">OpenIncident</h1>
            <p className="text-[#475569] text-sm mt-1">Incident management, self-hosted</p>
          </div>

          <div className="border-t border-[#1E293B] mb-8" />

          {/* Local login form */}
          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="block text-[#94A3B8] text-xs font-medium mb-1.5">Email</label>
              <input
                type="email"
                value={email}
                onChange={e => setEmail(e.target.value)}
                required
                className="w-full h-10 rounded-lg bg-[#1E293B] border border-[#334155] text-[#F1F5F9] text-sm px-3
                           focus:outline-none focus:border-[#2563EB] placeholder-[#475569]"
                placeholder="you@company.com"
              />
            </div>
            <div>
              <label className="block text-[#94A3B8] text-xs font-medium mb-1.5">Password</label>
              <input
                type="password"
                value={password}
                onChange={e => setPassword(e.target.value)}
                required
                className="w-full h-10 rounded-lg bg-[#1E293B] border border-[#334155] text-[#F1F5F9] text-sm px-3
                           focus:outline-none focus:border-[#2563EB]"
                placeholder="••••••••"
              />
            </div>

            {error && (
              <p className="text-red-400 text-xs text-center">{error}</p>
            )}

            <button
              type="submit"
              disabled={loading}
              className="w-full h-11 rounded-lg bg-[#2563EB] hover:bg-[#1D4ED8] text-white text-sm font-medium
                         transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {loading ? 'Signing in…' : 'Sign in'}
            </button>
          </form>

          {/* SSO option — only when configured */}
          {ssoEnabled && (
            <>
              <div className="flex items-center gap-3 my-6">
                <div className="flex-1 border-t border-[#1E293B]" />
                <span className="text-[#334155] text-xs">or</span>
                <div className="flex-1 border-t border-[#1E293B]" />
              </div>
              <a
                href="/saml/login"
                className="flex items-center justify-center gap-2.5 w-full h-11 rounded-lg border border-[#334155]
                           text-[#94A3B8] hover:text-[#F1F5F9] hover:border-[#475569] text-sm font-medium transition-colors"
              >
                <KeyIcon className="w-4 h-4" />
                Sign in with SSO
              </a>
            </>
          )}
        </div>
      </div>
    </div>
  )
}

function ShieldIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.75}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
    </svg>
  )
}

function KeyIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round"
        d="M15.75 5.25a3 3 0 013 3m3 0a6 6 0 01-7.029 5.912c-.563-.097-1.159.026-1.563.43L10.5 17.25H8.25v2.25H6v2.25H2.25v-2.818c0-.597.237-1.17.659-1.591l6.499-6.499c.404-.404.527-1 .43-1.563A6 6 0 1121.75 8.25z" />
    </svg>
  )
}
```

**Step 2: Update `AuthGate.tsx`** — show LoginPage in local-auth-required scenarios

Current logic: `if (openMode || authenticated) render children`. Now:
- `openMode` AND no local session → still render children (backward compat for truly open deploys)
- If SAML configured OR local users exist → show LoginPage when not authenticated

For now, simplest change: the `openMode` pass-through stays as-is. Once a local session exists the user will be `authenticated=true`. The only change is to also show LoginPage when `!authenticated && !openMode`:

```tsx
// AuthGate — no change needed to logic; LoginPage now has local form
// The existing behaviour is correct: show LoginPage when SAML configured + not authenticated.
// Local users will hit /login directly or be redirected by the sidebar "Sign in" link.
```

No change required to `AuthGate.tsx` — the login form on `LoginPage` sets the cookie which makes the next `/auth/me` call return `authenticated: true`.

**Step 3: Commit**

```bash
git add frontend/src/pages/LoginPage.tsx frontend/src/api/auth.ts frontend/src/contexts/AuthContext.tsx
git commit -m "feat(auth): add local login form to LoginPage; SSO button shown only when configured"
```

---

## Task 10: Frontend — sidebar fix + Settings nav item

**Files:**
- Modify: `frontend/src/components/layout/Sidebar.tsx`

**Step 1: Fix the user display area**

In `Sidebar.tsx`, find the `userDisplayName` helper. It already handles the fallback chain correctly — the bug is that in open mode, `user` has no `name` or `email`. Add an open-mode case:

```tsx
function userDisplayName(user: CurrentUser | null): string {
  if (!user) return '...'
  if (user.name) return user.name
  if (user.email) return user.email.split('@')[0] ?? user.email
  if (user.mode === 'open') return 'Open Mode'  // ← add this
  return 'You'
}
```

**Step 2: Add Settings nav item to `navSections`**

In the `navSections` array, add a second section after `organization`:

```tsx
{
  id: 'settings',
  label: 'Settings',
  collapsible: false,
  items: [
    {
      id: 'settings-users',
      label: 'Users',
      icon: Users,        // import Users from 'lucide-react'
      href: '/settings/users',
      matchPaths: ['/settings/users'],
    },
  ],
},
```

Add `Users` to the lucide-react import at the top of the file.

**Step 3: Fix the bottom bar for open-mode unauthenticated state**

In the bottom bar JSX, add a "Sign in" link when user is in open mode with no identity:

```tsx
{/* In the expanded bottom bar, after the user avatar/name block: */}
{user?.mode === 'open' && !user?.authenticated && (
  <a
    href="/login"
    className="text-xs text-brand-primary hover:underline ml-auto"
  >
    Sign in
  </a>
)}
```

**Step 4: Commit**

```bash
git add frontend/src/components/layout/Sidebar.tsx
git commit -m "fix(sidebar): show real name for local users; add Settings nav; fix open-mode display"
```

---

## Task 11: Frontend — /settings/users admin page

**Files:**
- Create: `frontend/src/api/settings.ts`
- Create: `frontend/src/pages/SettingsUsersPage.tsx`
- Modify: `frontend/src/App.tsx`

**Step 1: Create `api/settings.ts`**

```typescript
import { apiClient } from './client'

export interface UserRecord {
  id: string
  email: string
  name: string
  role: 'admin' | 'member' | 'viewer'
  auth_source: 'saml' | 'local' | 'deactivated'
  last_login_at?: string
  created_at: string
}

export interface CreateUserPayload {
  email: string
  name: string
  role: string
  password: string
}

export async function listUsers(): Promise<UserRecord[]> {
  const res = await apiClient.get<{ users: UserRecord[] }>('/api/v1/settings/users')
  return res.users
}

export async function createUser(payload: CreateUserPayload): Promise<{ user: UserRecord; setup_token: string }> {
  return apiClient.post('/api/v1/settings/users', payload)
}

export async function updateUser(id: string, payload: Partial<CreateUserPayload>): Promise<void> {
  await apiClient.patch(`/api/v1/settings/users/${id}`, payload)
}

export async function deactivateUser(id: string): Promise<void> {
  await apiClient.delete(`/api/v1/settings/users/${id}`)
}

export async function resetUserPassword(id: string): Promise<{ setup_token: string }> {
  return apiClient.post(`/api/v1/settings/users/${id}/reset-password`, {})
}
```

**Step 2: Create `SettingsUsersPage.tsx`**

Build a page with:
- User table (name, email, role badge, auth source, last login, actions)
- "Invite user" button → modal with name/email/role/password fields
- Edit button per row → edit modal
- Deactivate button per row (with confirmation)
- Copy setup link button after creating/resetting (shows token-based login URL)

The page is ~200 lines. Match the existing dark navy design system (`bg-surface`, `text-text-primary`, etc.) used across the app. See `IncidentsListPage.tsx` for patterns on table layout and modal usage.

Key pieces:

```tsx
// Role badge
function RoleBadge({ role }: { role: string }) {
  const colors: Record<string, string> = {
    admin: 'bg-purple-500/20 text-purple-300 border-purple-500/30',
    member: 'bg-blue-500/20 text-blue-300 border-blue-500/30',
    viewer: 'bg-gray-500/20 text-gray-300 border-gray-500/30',
  }
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium border ${colors[role] ?? colors.member}`}>
      {role}
    </span>
  )
}

// Setup link copy area (shown after invite or password reset)
function SetupLinkBox({ token }: { token: string }) {
  const url = `${window.location.origin}/login?setup=${token}`
  return (
    <div className="mt-4 p-3 bg-[#1E293B] rounded-lg border border-[#334155]">
      <p className="text-xs text-[#94A3B8] mb-2">Share this one-time login link with the user:</p>
      <div className="flex gap-2">
        <input readOnly value={url} className="flex-1 text-xs bg-[#0F172A] border border-[#334155] rounded px-2 py-1 text-[#F1F5F9]" />
        <button onClick={() => navigator.clipboard.writeText(url)}
          className="px-3 py-1 text-xs bg-[#2563EB] text-white rounded hover:bg-[#1D4ED8]">
          Copy
        </button>
      </div>
    </div>
  )
}
```

**Step 3: Add route to `App.tsx`**

```tsx
import { SettingsUsersPage } from './pages/SettingsUsersPage'

// Inside <Route element={<AppLayout />}>:
<Route path="/settings/users" element={<SettingsUsersPage />} />
```

**Step 4: Commit**

```bash
git add frontend/src/api/settings.ts frontend/src/pages/SettingsUsersPage.tsx frontend/src/App.tsx
git commit -m "feat(settings): add /settings/users admin page with invite, edit, deactivate"
```

---

## Task 12: End-to-end smoke test + final build check

**Step 1: Run all backend tests**

```bash
cd backend && go test ./... 2>&1 | grep -E "FAIL|ok|---"
```
Expected: no FAIL lines (the pre-existing `teams_conversation_id` SQLite failures in integration tests are known and unrelated).

**Step 2: Run frontend type check**

```bash
cd frontend && npm run build 2>&1 | tail -20
```
Expected: clean build, no TypeScript errors.

**Step 3: Manual smoke test checklist**

Start the app locally:
```bash
make dev
```

- [ ] Visit `http://localhost:3000` → sidebar shows `Open Mode` with Sign in link (no "You"/"?")
- [ ] Visit `/login` → email + password form visible; no SSO button (SAML not configured)
- [ ] Cannot access `/settings/users` without logging in → redirects to login (or 401 from API)
- [ ] `POST /api/v1/settings/users` with `{ email, name, password, role: "admin" }` → 201 + setup_token
- [ ] Use setup_token to login → sidebar shows real name and email
- [ ] Visit `/settings/users` → user table visible
- [ ] Invite another user → copy setup link → open in incognito → login → works

**Step 4: Commit docs update**

Update `CLAUDE.md` to note local auth is in progress (done after implementation lands).

---

## Notes for the implementer

- **`go.mod`**: `golang.org/x/crypto` is already present — bcrypt is available as `golang.org/x/crypto/bcrypt`.
- **`routes.Setup` signature**: Check the actual function signature in `routes.go` before wiring `localAuthSvc`. It currently takes `samlMiddleware *samlsp.Middleware` and other services — add `localAuth services.LocalAuthService` as a parameter.
- **SAML `Upsert` call**: `user_repository.go:Upsert` currently takes `context.Context` as first arg — update `stubUserRepo.Upsert` signature in tests to match.
- **Cookie security**: In `serve.go`, check if `APP_ENV=production` and set `Secure=true` on the `oi_session` cookie in that case. For now, `Secure=false` to work on HTTP localhost.
- **Bootstrap**: The design calls for a first-run bootstrap where the first `POST /settings/users` is allowed unauthenticated. This is a nice-to-have — for now, document that operators should create the first admin via a direct API call or a seed script.
