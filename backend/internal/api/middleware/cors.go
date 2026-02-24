package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS returns a middleware that enforces an origin allowlist.
//
// Allowed origins are read from the CORS_ALLOWED_ORIGINS environment variable
// (comma-separated). The default is "http://localhost:3000" for local development.
//
// Security properties:
//   - Only allowlisted origins receive Access-Control-Allow-Origin.
//   - Access-Control-Allow-Credentials: true is only sent for allowlisted origins.
//   - Requests from unknown origins are served without CORS headers, causing
//     the browser to block the cross-origin response.
//   - Reflecting the request Origin header unconditionally (the previous
//     behaviour) is a critical vulnerability when combined with credentials.
func CORS() gin.HandlerFunc {
	allowed := parseAllowedOrigins(os.Getenv("CORS_ALLOWED_ORIGINS"))

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		if origin != "" && isAllowed(origin, allowed) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Vary", "Origin")
			c.Writer.Header().Set("Access-Control-Allow-Headers",
				"Content-Type, Authorization, X-Requested-With, Cache-Control, Accept")
			c.Writer.Header().Set("Access-Control-Allow-Methods",
				"GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}

		if c.Request.Method == http.MethodOptions {
			if origin != "" && isAllowed(origin, allowed) {
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
// Falls back to localhost:3000 for development when the env var is unset.
func parseAllowedOrigins(raw string) []string {
	if raw == "" {
		return []string{"http://localhost:3000"}
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
