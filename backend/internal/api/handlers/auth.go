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
