package middleware

import (
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS returns a middleware that enforces an origin allowlist.
//
// Allowed origins are read from the CORS_ALLOWED_ORIGINS environment variable
// (comma-separated).
//
// In development (APP_ENV=development or unset) all http://localhost:* origins
// are automatically allowed — no configuration needed when running `npm run dev`
// on any port.
//
// In production (APP_ENV=production) only explicitly listed origins are
// permitted.  When the embedded frontend is used (same-origin deployment) the
// browser sends no Origin header at all, so CORS headers are never needed.
//
// Security properties:
//   - Only allowlisted origins receive Access-Control-Allow-Origin.
//   - Access-Control-Allow-Credentials: true is only sent for allowlisted origins.
//   - Requests from unknown origins are served without CORS headers, causing
//     the browser to block the cross-origin response.
func CORS() gin.HandlerFunc {
	isDev := os.Getenv("APP_ENV") != "production"
	allowed := parseAllowedOrigins(os.Getenv("CORS_ALLOWED_ORIGINS"))

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		if origin != "" && (isAllowed(origin, allowed) || (isDev && isLocalhost(origin))) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Vary", "Origin")
			c.Writer.Header().Set("Access-Control-Allow-Headers",
				"Content-Type, Authorization, X-Requested-With, Cache-Control, Accept")
			c.Writer.Header().Set("Access-Control-Allow-Methods",
				"GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}

		if c.Request.Method == http.MethodOptions {
			if origin != "" && (isAllowed(origin, allowed) || (isDev && isLocalhost(origin))) {
				c.AbortWithStatus(http.StatusNoContent)
			} else {
				c.AbortWithStatus(http.StatusForbidden)
			}
			return
		}

		c.Next()
	}
}

// parseAllowedOrigins splits the comma-separated origins string.
func parseAllowedOrigins(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func isAllowed(origin string, allowed []string) bool {
	for _, a := range allowed {
		if origin == a {
			return true
		}
	}
	return false
}

// isLocalhost reports whether the origin is an http://localhost:* URL.
// Used to auto-allow all local ports in development without any config.
func isLocalhost(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return u.Scheme == "http" && u.Hostname() == "localhost"
}
