package handlers

import (
	"net/http"

	"github.com/crewjam/saml/samlsp"
	"github.com/gin-gonic/gin"
	"github.com/openincident/openincident/internal/api/middleware"
)

// Logout deletes the SAML session cookie and redirects to the root.
func Logout(samlMiddleware *samlsp.Middleware) gin.HandlerFunc {
	return func(c *gin.Context) {
		if samlMiddleware != nil {
			_ = samlMiddleware.Session.DeleteSession(c.Writer, c.Request)
		}
		c.Redirect(http.StatusFound, "/")
	}
}

// GetCurrentUser handles GET /api/v1/auth/me.
// Returns basic identity info from the SAML session.
// When SAML is disabled, returns a placeholder indicating open-access mode.
func GetCurrentUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := middleware.GetSAMLSession(c)
		if session == nil {
			// SAML not configured — open access mode
			c.JSON(http.StatusOK, gin.H{
				"authenticated": false,
				"mode":          "open",
				"message":       "SSO not configured — all requests are permitted",
			})
			return
		}

		claims, ok := session.(samlsp.JWTSessionClaims)
		if !ok {
			c.JSON(http.StatusOK, gin.H{"authenticated": true})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"authenticated": true,
			"email":         claims.Attributes.Get("email"),
			"name":          claims.Attributes.Get("displayName"),
		})
	}
}
