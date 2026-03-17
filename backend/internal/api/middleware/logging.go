package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger returns a Gin middleware for structured JSON logging with slog
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Get request ID from context
		requestID, _ := c.Get("request_id")

		// Build log attributes
		attrs := []any{
			slog.String("service", "regen"),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", c.Writer.Status()),
			slog.Int64("duration_ms", latency.Milliseconds()),
			slog.String("client_ip", c.ClientIP()),
		}

		// Add request ID if present
		if requestID != nil {
			attrs = append(attrs, slog.String("request_id", requestID.(string)))
		}

		// Add user agent if present
		if userAgent := c.Request.UserAgent(); userAgent != "" {
			attrs = append(attrs, slog.String("user_agent", userAgent))
		}

		// Determine log level based on status code
		status := c.Writer.Status()
		if len(c.Errors) > 0 {
			// Log errors
			attrs = append(attrs, slog.String("error", c.Errors.String()))
			slog.Error("request completed with errors", attrs...)
		} else if status >= 500 {
			slog.Error("request failed", attrs...)
		} else if status >= 400 {
			slog.Warn("request rejected", attrs...)
		} else {
			slog.Info("request completed", attrs...)
		}
	}
}
