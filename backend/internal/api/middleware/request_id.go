package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

// RequestID adds a unique request ID to each request for tracing.
// Supports X-Request-ID header from load balancers/proxies.
//
// When a valid trace already exists in the request context (otelgin's
// middleware runs before this one), request_id is set to the trace ID instead
// — reconciling the two correlation systems so logs, headers, and traces all
// agree on one identifier. This takes priority over any incoming
// X-Request-ID header: once a real trace exists, it is the source of truth.
// With tracing disabled (the default), behavior is unchanged from before.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")

		if sc := trace.SpanContextFromContext(c.Request.Context()); sc.IsValid() {
			requestID = sc.TraceID().String()
		} else if requestID == "" {
			requestID = uuid.New().String()
		}

		// Store in context for handlers and logging
		c.Set("request_id", requestID)

		// Include in response headers
		c.Header("X-Request-ID", requestID)

		c.Next()
	}
}
