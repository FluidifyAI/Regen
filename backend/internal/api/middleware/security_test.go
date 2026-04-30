package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSecurityHeaders(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create a test router with the security headers middleware
	router := gin.New()
	router.Use(SecurityHeaders())

	// Add a simple test endpoint
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})

	// Create a test request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	// Perform the request
	router.ServeHTTP(w, req)

	// Assert all security headers are set correctly
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"),
		"X-Content-Type-Options should be set to nosniff")

	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"),
		"X-Frame-Options should be set to DENY")

	assert.Empty(t, w.Header().Get("X-XSS-Protection"),
		"X-XSS-Protection should not be set (deprecated header)")

	assert.Equal(t, "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https://cdn.jsdelivr.net; connect-src 'self' https://us.i.posthog.com https://static.fluidify.ai; font-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'", w.Header().Get("Content-Security-Policy"),
		"Content-Security-Policy should match the hardened policy")

	assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"),
		"Referrer-Policy should be set to strict-origin-when-cross-origin")

	// Assert the request was successful
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSecurityHeadersDoesNotBlockRequests(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create a test router with the security headers middleware
	router := gin.New()
	router.Use(SecurityHeaders())

	// Add a test endpoint that returns data
	router.GET("/data", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": "test data"})
	})

	// Create a test request
	req := httptest.NewRequest(http.MethodGet, "/data", nil)
	w := httptest.NewRecorder()

	// Perform the request
	router.ServeHTTP(w, req)

	// Assert the request completes successfully
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "test data",
		"Response body should contain the expected data")
}
