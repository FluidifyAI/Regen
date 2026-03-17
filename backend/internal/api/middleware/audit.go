package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/fluidify/regen/internal/enterprise"
)

// AuditLog returns a Gin middleware that calls exporter.Export after each
// request completes. In the OSS build the exporter is the no-op stub, so
// this middleware adds zero overhead beyond the function call.
//
// The middleware captures the HTTP layer only (method, path, status, actor).
// Fine-grained domain events (e.g. "incident.resolved") are emitted by the
// enterprise service layer implementations in the enterprise repo.
func AuditLog(exporter enterprise.AuditExporter) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next() // run the handler first so we have the status code

		// Skip health/metrics endpoints — not meaningful for audit trails.
		path := c.Request.URL.Path
		if path == "/health" || path == "/ready" || path == "/metrics" {
			return
		}

		actorID := "anonymous"
		if id, ok := c.Get("user_id"); ok {
			if s, ok := id.(string); ok && s != "" {
				actorID = s
			}
		}

		exporter.Export(c.Request.Context(), enterprise.AuditEvent{
			Timestamp:    time.Now().UTC(),
			ActorID:      actorID,
			ActorType:    "user",
			Action:       c.Request.Method + " " + c.FullPath(),
			ResourceType: "http",
			IPAddress:    c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			StatusCode:   c.Writer.Status(),
		})
	}
}
