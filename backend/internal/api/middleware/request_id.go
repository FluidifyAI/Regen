package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestID adds a unique request ID to each request for tracing
// Supports X-Request-ID header from load balancers/proxies
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if request ID exists in header (from load balancer)
		requestID := c.GetHeader("X-Request-ID")

		// Generate new ID if not provided
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Store in context for handlers and logging
		c.Set("request_id", requestID)

		// Include in response headers
		c.Header("X-Request-ID", requestID)

		c.Next()
	}
}
