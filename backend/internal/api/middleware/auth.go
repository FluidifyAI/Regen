package middleware

import (
	"net/http"

	"github.com/crewjam/saml/samlsp"
	"github.com/gin-gonic/gin"
)

const contextKeySAMLSession = "saml_session"

// RequireAuth enforces SAML session authentication on a Gin route group.
// When samlMiddleware is nil (SAML not configured), this is a no-op —
// all requests pass through, preserving backwards compatibility.
func RequireAuth(samlMiddleware *samlsp.Middleware) gin.HandlerFunc {
	if samlMiddleware == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		session, err := samlMiddleware.Session.GetSession(c.Request)
		if err != nil || session == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "unauthorized",
					"message": "Authentication required. Please log in via /auth/saml/login",
				},
			})
			return
		}
		c.Set(contextKeySAMLSession, session)
		c.Next()
	}
}

// GetSAMLSession retrieves the SAML session from the Gin context.
// Returns nil when SAML is not configured or the request is not authenticated.
func GetSAMLSession(c *gin.Context) samlsp.Session {
	if val, exists := c.Get(contextKeySAMLSession); exists {
		if s, ok := val.(samlsp.Session); ok {
			return s
		}
	}
	return nil
}
