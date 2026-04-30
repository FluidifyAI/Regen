package middleware

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeaders returns a middleware that sets standard security headers on all responses.
//
// Headers set:
//   - X-Content-Type-Options: nosniff (prevents MIME type sniffing)
//   - X-Frame-Options: DENY (legacy clickjacking protection for older browsers)
//   - Content-Security-Policy: tightened policy — see inline comment
//   - Referrer-Policy: strict-origin-when-cross-origin
//   - Strict-Transport-Security: max-age=63072000; includeSubDomains (2 years; HTTPS only)
//   - X-Permitted-Cross-Domain-Policies: none (blocks Flash/PDF cross-domain policy files)
//
// X-XSS-Protection is intentionally omitted: it is deprecated, removed from modern
// browsers, and can introduce vulnerabilities in older IE/Edge versions.
func SecurityHeaders() gin.HandlerFunc {
	// object-src 'none'             — disables plugins (Flash, Java applets)
	// base-uri 'self'               — prevents <base> tag hijacking (relative URL manipulation)
	// frame-ancestors 'none'        — modern clickjacking protection (supersedes X-Frame-Options)
	// form-action 'self'            — forms may only submit to our own origin
	// style-src + 'unsafe-inline'   — React components use inline styles; required for SPA
	// img-src data:                 — base64-encoded data URIs used in UI components
	// img-src cdn.jsdelivr.net      — Simple Icons brand logos on the Integrations page
	// connect-src us.i.posthog.com    — PostHog analytics (write-only capture; OPE-79)
	// connect-src static.fluidify.ai — in-app announcements polling (OPE-79)
	const csp = "default-src 'self'; " +
		"script-src 'self'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: https://cdn.jsdelivr.net; " +
		"connect-src 'self' https://us.i.posthog.com https://static.fluidify.ai; " +
		"font-src 'self'; " +
		"object-src 'none'; " +
		"base-uri 'self'; " +
		"frame-ancestors 'none'; " +
		"form-action 'self'"

	return func(c *gin.Context) {
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		c.Writer.Header().Set("X-Frame-Options", "DENY")
		c.Writer.Header().Set("Content-Security-Policy", csp)
		c.Writer.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// HSTS: tell browsers to always use HTTPS for this domain for 2 years.
		// Safe to set here — HTTP clients ignore it; it only takes effect over HTTPS.
		c.Writer.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		c.Writer.Header().Set("X-Permitted-Cross-Domain-Policies", "none")
		c.Next()
	}
}
