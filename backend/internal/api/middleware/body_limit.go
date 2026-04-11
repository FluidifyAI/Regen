package middleware

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	// DefaultMaxBodySize is 10MB for general API requests
	DefaultMaxBodySize int64 = 10 * 1024 * 1024

	// WebhookMaxBodySize is 1MB for webhook endpoints
	// Webhooks should be small - large payloads indicate a problem
	WebhookMaxBodySize int64 = 1 * 1024 * 1024
)

// BodySizeLimit returns a middleware that limits the request body size
//
// This helps prevent:
// - Denial of service attacks via large payloads
// - Memory exhaustion from unbounded inputs
// - Accidental large payloads indicating misconfiguration
//
// Usage:
//
//	webhooks.Use(middleware.BodySizeLimit(WebhookMaxBodySize))
func BodySizeLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check Content-Length header if present
		if c.Request.ContentLength > maxBytes {
			slog.Warn("request body too large",
				"content_length", c.Request.ContentLength,
				"max_bytes", maxBytes,
				"path", c.Request.URL.Path,
				"client_ip", c.ClientIP(),
				"request_id", c.GetString("request_id"),
			)

			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": "request body too large",
				"details": map[string]interface{}{
					"max_size_bytes": maxBytes,
					"max_size_mb":    float64(maxBytes) / (1024 * 1024),
				},
			})
			return
		}

		// Also set MaxBytesReader as a safety net for cases where Content-Length
		// is not set or is incorrect
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)

		c.Next()
	}
}
