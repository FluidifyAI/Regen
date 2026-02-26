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
// No-op in SAML/open mode.
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
