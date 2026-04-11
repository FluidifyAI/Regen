package middleware

import (
	"net/http"
	"os"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/services"
	"github.com/crewjam/saml/samlsp"
	"github.com/gin-gonic/gin"
)

// IsSecure returns true if the Secure flag should be set on cookies.
// Always secure except in local development to allow plain HTTP.
// Single definition used by both middleware and handler packages.
func IsSecure() bool {
	return os.Getenv("APP_ENV") != "development"
}

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

		// 2. Check SAML session and resolve DB user so RequireAdmin works uniformly.
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
			// Resolve DB user from SAML session email so downstream middleware
			// (e.g. RequireAdmin) can operate identically for both auth paths.
			resolveAndStoreSAMLUser(c, session, la)
			c.Next()
			return
		}

		// 3. Open mode — no auth configured
		c.Next()
	}
}

// InjectSAMLSession reads the SAML session cookie (if present) and sets it in
// the Gin context without aborting when no session exists. Use on routes that
// must be accessible without auth but should still identify SAML users
// (e.g. GET /auth/me). When la is provided, also resolves the DB user into
// contextKeyLocalUser so handlers see a unified identity regardless of auth path.
func InjectSAMLSession(samlMiddleware *samlsp.Middleware, la services.LocalAuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if samlMiddleware != nil {
			if session, err := samlMiddleware.Session.GetSession(c.Request); err == nil && session != nil {
				c.Set(contextKeySAMLSession, session)
				resolveAndStoreSAMLUser(c, session, la)
			}
		}
		c.Next()
	}
}

// resolveAndStoreSAMLUser extracts the email from a SAML JWT session, looks up
// the corresponding DB user, and stores it under contextKeyLocalUser. This
// allows RequireAdmin and GetLocalUser to work identically for both local and
// SAML auth paths. If resolution fails (e.g. user deactivated), the context key
// is left unset — callers that require authentication must check separately.
func resolveAndStoreSAMLUser(c *gin.Context, session samlsp.Session, la services.LocalAuthService) {
	if la == nil {
		return
	}
	sa, ok := session.(samlsp.JWTSessionClaims)
	if !ok {
		return
	}
	// Try common email attribute names — matches the samlAttr() extraction in auth/saml.go.
	email := sa.Attributes.Get("email")
	if email == "" {
		email = sa.Attributes.Get("http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress")
	}
	if email == "" {
		email = sa.Attributes.Get("http://schemas.xmlsoap.org/ws/2005/05/identity/claims/email")
	}
	if email == "" {
		return
	}
	user, err := la.GetUserByEmail(email)
	if err != nil || user == nil || user.AuthSource == "deactivated" {
		return
	}
	c.Set(contextKeyLocalUser, user)
}

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
// No-op in SAML/open mode.
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetLocalUser(c)
		if user == nil || user.Role != models.UserRoleAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": gin.H{"code": "forbidden", "message": "Admin access required"},
			})
			return
		}
		c.Next()
	}
}

func clearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     localSessionCookieName,
		Value:    "",
		MaxAge:   -1,
		Path:     "/",
		HttpOnly: true,
		Secure:   IsSecure(),
		SameSite: http.SameSiteStrictMode,
	})
}
