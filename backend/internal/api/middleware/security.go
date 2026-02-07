package middleware

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeaders returns a middleware that sets standard security headers on all responses
//
// Headers set:
// - X-Content-Type-Options: nosniff (prevents MIME type sniffing)
// - X-Frame-Options: DENY (prevents clickjacking attacks)
// - X-XSS-Protection: 1; mode=block (enables XSS protection in older browsers)
// - Content-Security-Policy: default-src 'self' (restricts resource loading to same origin)
// - Referrer-Policy: strict-origin-when-cross-origin (controls referrer information)
//
// These headers provide basic protection against common web vulnerabilities
// and are appropriate for v0.1 of the application.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent MIME type sniffing
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking by disallowing the page to be rendered in a frame
		c.Writer.Header().Set("X-Frame-Options", "DENY")

		// Enable XSS protection in older browsers
		c.Writer.Header().Set("X-XSS-Protection", "1; mode=block")

		// Content Security Policy - only allow resources from same origin
		c.Writer.Header().Set("Content-Security-Policy", "default-src 'self'")

		// Control referrer information sent to other sites
		c.Writer.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		c.Next()
	}
}
