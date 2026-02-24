package middleware

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeaders returns a middleware that sets standard security headers on all responses.
//
// Headers set:
//   - X-Content-Type-Options: nosniff (prevents MIME type sniffing)
//   - X-Frame-Options: DENY (prevents clickjacking)
//   - X-XSS-Protection: 1; mode=block (XSS protection for older browsers)
//   - Content-Security-Policy: default-src 'self'
//   - Referrer-Policy: strict-origin-when-cross-origin
//   - Strict-Transport-Security: max-age=63072000; includeSubDomains (2 years; HTTPS only)
//   - X-Permitted-Cross-Domain-Policies: none (blocks Flash/PDF cross-domain policy files)
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		c.Writer.Header().Set("X-Frame-Options", "DENY")
		c.Writer.Header().Set("X-XSS-Protection", "1; mode=block")
		c.Writer.Header().Set("Content-Security-Policy", "default-src 'self'")
		c.Writer.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// HSTS: tell browsers to always use HTTPS for this domain for 2 years.
		// Safe to set here — HTTP clients ignore it; it only takes effect over HTTPS.
		c.Writer.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		c.Writer.Header().Set("X-Permitted-Cross-Domain-Policies", "none")
		c.Next()
	}
}
