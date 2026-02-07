package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestBodySizeLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		maxSize        int64
		bodySize       int
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "small body within limit",
			maxSize:        1024, // 1KB
			bodySize:       500,  // 500 bytes
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "body just under limit",
			maxSize:        1024,      // 1KB total
			bodySize:       1024 - 15, // Account for JSON wrapping: {"data":""} = ~11 bytes + safety margin
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "body exceeds limit",
			maxSize:        1024, // 1KB
			bodySize:       2048, // 2KB
			expectedStatus: http.StatusRequestEntityTooLarge,
			expectError:    true,
		},
		{
			name:           "webhook size limit (1MB)",
			maxSize:        WebhookMaxBodySize,
			bodySize:       500 * 1024, // 500KB
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "webhook exceeds 1MB limit",
			maxSize:        WebhookMaxBodySize,
			bodySize:       2 * 1024 * 1024, // 2MB
			expectedStatus: http.StatusRequestEntityTooLarge,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create router with body size limit
			router := gin.New()
			router.Use(BodySizeLimit(tt.maxSize))

			// Add test endpoint that tries to read the body
			router.POST("/test", func(c *gin.Context) {
				var data map[string]interface{}
				if err := c.ShouldBindJSON(&data); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, gin.H{"received": true})
			})

			// Create a body of the specified size
			body := bytes.Repeat([]byte("a"), tt.bodySize)
			jsonBody := []byte(`{"data":"` + string(body) + `"}`)

			// Make request
			req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Verify response
			if tt.expectError {
				assert.Equal(t, tt.expectedStatus, w.Code,
					"Expected status %d for body size %d with limit %d",
					tt.expectedStatus, tt.bodySize, tt.maxSize)
			} else {
				// For successful requests, we expect either 200 or 400 (if JSON is malformed)
				// but not 413 (Request Entity Too Large)
				assert.NotEqual(t, http.StatusRequestEntityTooLarge, w.Code,
					"Should not get 413 for body size %d with limit %d",
					tt.bodySize, tt.maxSize)
			}
		})
	}
}

func TestBodySizeLimitConstants(t *testing.T) {
	// Verify the constants are set correctly
	assert.Equal(t, int64(10*1024*1024), DefaultMaxBodySize,
		"Default max body size should be 10MB")

	assert.Equal(t, int64(1*1024*1024), WebhookMaxBodySize,
		"Webhook max body size should be 1MB")
}
