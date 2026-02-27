package handlers

import (
	"net/http"
	"os"

	"github.com/crewjam/saml/samlsp"
	"github.com/gin-gonic/gin"
	"github.com/openincident/openincident/internal/api/middleware"
	"github.com/openincident/openincident/internal/services"
)

// setSessionCookie writes the oi_session cookie with SameSite=Strict.
// Gin's c.SetCookie does not support SameSite selection, so we write directly.
func setSessionCookie(c *gin.Context, token string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "oi_session",
		Value:    token,
		MaxAge:   maxAge,
		Path:     "/",
		HttpOnly: true,
		Secure:   os.Getenv("APP_ENV") != "development",
		SameSite: http.SameSiteStrictMode,
	})
}

// Logout handles GET /auth/logout — browser redirect flow (kept for backwards compatibility).
func Logout(samlMiddleware *samlsp.Middleware, localAuth services.LocalAuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		clearSession(c, samlMiddleware, localAuth)
		c.Redirect(http.StatusFound, "/login")
	}
}

// APILogout handles POST /api/v1/auth/logout — called from the SPA via fetch.
// Returns JSON so the frontend can handle navigation itself.
func APILogout(samlMiddleware *samlsp.Middleware, localAuth services.LocalAuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		clearSession(c, samlMiddleware, localAuth)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// clearSession invalidates the current session for both local and SAML auth.
func clearSession(c *gin.Context, samlMiddleware *samlsp.Middleware, localAuth services.LocalAuthService) {
	if localAuth != nil {
		if cookie, err := c.Cookie("oi_session"); err == nil {
			_ = localAuth.Logout(cookie)
		}
		setSessionCookie(c, "", -1)
	}
	if samlMiddleware != nil {
		_ = samlMiddleware.Session.DeleteSession(c.Writer, c.Request)
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

		// Set session cookie: 7-day, HttpOnly, SameSite=Strict
		setSessionCookie(c, session.Token, 7*24*3600)
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

		// Determine true open mode: only when no users exist AND SAML is not configured.
		// Once a local user has been created, login is always required.
		isOpenMode := !samlConfigured
		if isOpenMode && localAuth != nil {
			count, err := localAuth.CountUsers()
			if err == nil && count > 0 {
				isOpenMode = false
			}
		}

		if isOpenMode {
			c.JSON(http.StatusOK, gin.H{
				"authenticated": false,
				"mode":          "open",
				"ssoEnabled":    false,
				"message":       "No auth configured — all requests permitted",
			})
		} else {
			c.JSON(http.StatusOK, gin.H{
				"authenticated": false,
				"ssoEnabled":    samlConfigured,
			})
		}
	}
}
