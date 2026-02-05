package middleware

import (
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
)

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp  string `json:"timestamp"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	Status     int    `json:"status"`
	Latency    string `json:"latency"`
	ClientIP   string `json:"client_ip"`
	UserAgent  string `json:"user_agent,omitempty"`
	Error      string `json:"error,omitempty"`
}

// Logger returns a Gin middleware for structured JSON logging
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Build log entry
		entry := LogEntry{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Method:    c.Request.Method,
			Path:      c.Request.URL.Path,
			Status:    c.Writer.Status(),
			Latency:   latency.String(),
			ClientIP:  c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
		}

		// Add error if present
		if len(c.Errors) > 0 {
			entry.Error = c.Errors.String()
		}

		// Marshal to JSON and print
		if logJSON, err := json.Marshal(entry); err == nil {
			gin.DefaultWriter.Write(logJSON)
			gin.DefaultWriter.Write([]byte("\n"))
		}
	}
}
