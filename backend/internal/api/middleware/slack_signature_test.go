package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSlackSignatureVerification(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Test signing secret
	testSecret := "test_signing_secret_12345"

	tests := []struct {
		name             string
		setupEnv         bool
		signingSecret    string
		body             string
		timestamp        string
		signature        string
		computeSignature bool // If true, compute valid signature; if false, use signature as-is
		expectedStatus   int
		description      string
	}{
		{
			name:             "valid signature",
			setupEnv:         true,
			signingSecret:    testSecret,
			body:             `{"type":"event_callback","event":{"type":"message"}}`,
			timestamp:        strconv.FormatInt(time.Now().Unix(), 10),
			signature:        "",
			computeSignature: true, // Compute valid signature
			expectedStatus:   http.StatusOK,
			description:      "Request with valid signature should pass",
		},
		{
			name:             "missing timestamp header",
			setupEnv:         true,
			signingSecret:    testSecret,
			body:             `{"type":"event_callback"}`,
			timestamp:        "", // Missing
			signature:        "v0=fakesignature",
			computeSignature: false,
			expectedStatus:   http.StatusUnauthorized,
			description:      "Request without timestamp should be rejected",
		},
		{
			name:             "missing signature header",
			setupEnv:         true,
			signingSecret:    testSecret,
			body:             `{"type":"event_callback"}`,
			timestamp:        strconv.FormatInt(time.Now().Unix(), 10),
			signature:        "",    // Don't send header
			computeSignature: false, // Don't compute
			expectedStatus:   http.StatusUnauthorized,
			description:      "Request without signature should be rejected",
		},
		{
			name:             "invalid signature",
			setupEnv:         true,
			signingSecret:    testSecret,
			body:             `{"type":"event_callback"}`,
			timestamp:        strconv.FormatInt(time.Now().Unix(), 10),
			signature:        "v0=invalid_signature_12345",
			computeSignature: false,
			expectedStatus:   http.StatusUnauthorized,
			description:      "Request with invalid signature should be rejected",
		},
		{
			name:             "timestamp too old (replay attack)",
			setupEnv:         true,
			signingSecret:    testSecret,
			body:             `{"type":"event_callback"}`,
			timestamp:        strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10), // 10 minutes ago
			signature:        "",
			computeSignature: true, // Compute signature with old timestamp
			expectedStatus:   http.StatusUnauthorized,
			description:      "Request older than 5 minutes should be rejected",
		},
		{
			name:             "timestamp in future",
			setupEnv:         true,
			signingSecret:    testSecret,
			body:             `{"type":"event_callback"}`,
			timestamp:        strconv.FormatInt(time.Now().Add(10*time.Minute).Unix(), 10), // 10 minutes in future
			signature:        "",
			computeSignature: true, // Compute signature
			expectedStatus:   http.StatusUnauthorized,
			description:      "Request with future timestamp should be rejected",
		},
		{
			name:             "no signing secret configured (dev mode)",
			setupEnv:         false, // Don't set SLACK_SIGNING_SECRET
			signingSecret:    "",
			body:             `{"type":"event_callback"}`,
			timestamp:        strconv.FormatInt(time.Now().Unix(), 10),
			signature:        "v0=anysignature",
			computeSignature: false,
			expectedStatus:   http.StatusOK,
			description:      "When no secret is configured, requests should pass (dev mode)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment
			if tt.setupEnv {
				os.Setenv("SLACK_SIGNING_SECRET", tt.signingSecret)
			} else {
				os.Unsetenv("SLACK_SIGNING_SECRET")
			}
			defer os.Unsetenv("SLACK_SIGNING_SECRET")

			// Compute signature if requested
			signature := tt.signature
			if tt.computeSignature && tt.timestamp != "" && tt.setupEnv {
				signature = computeTestSignature(tt.signingSecret, tt.timestamp, []byte(tt.body))
			}

			// Create test router
			router := gin.New()
			router.Use(SlackSignatureVerification())
			router.POST("/slack/events", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			})

			// Create request
			req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			if tt.timestamp != "" {
				req.Header.Set("X-Slack-Request-Timestamp", tt.timestamp)
			}
			if signature != "" {
				req.Header.Set("X-Slack-Signature", signature)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code, tt.description)
		})
	}
}

func TestComputeSlackSignature(t *testing.T) {
	// Test vector from Slack documentation
	// https://api.slack.com/authentication/verifying-requests-from-slack

	signingSecret := "8f742231b10e8888abcd99yyyzzz85a5"
	timestamp := "1531420618"
	body := []byte("token=xyzz0WbapA4vBCDEFasx0q6G&team_id=T1DC2JH3J&team_domain=testteamnow&channel_id=G8PSS9T3V&channel_name=foobar&user_id=U2CERLKJA&user_name=roadrunner&command=%2Fwebhook-collect&text=&response_url=https%3A%2F%2Fhooks.slack.com%2Fcommands%2FT1DC2JH3J%2F397700885554%2F96rGlfmibIGlgcZRskXaIFfN&trigger_id=398738663015.47445629121.803a0bc887a14d10d2c447fce8b6703c")

	expectedSignature := "v0=a2114d57b48eac39b9ad189dd8316235a7b4a8d21a10bd27519666489c69b503"

	actualSignature := computeSlackSignature(signingSecret, timestamp, body)

	assert.Equal(t, expectedSignature, actualSignature,
		"Signature should match Slack's documented test vector")
}

func TestParseSlackTimestamp(t *testing.T) {
	tests := []struct {
		name        string
		timestamp   string
		expectError bool
		description string
	}{
		{
			name:        "valid timestamp",
			timestamp:   "1531420618",
			expectError: false,
			description: "Valid Unix timestamp should parse",
		},
		{
			name:        "invalid format (not a number)",
			timestamp:   "not_a_number",
			expectError: true,
			description: "Non-numeric timestamp should fail",
		},
		{
			name:        "empty timestamp",
			timestamp:   "",
			expectError: true,
			description: "Empty timestamp should fail",
		},
		{
			name:        "negative timestamp",
			timestamp:   "-1",
			expectError: false, // Parses OK, but will fail age check in middleware
			description: "Negative timestamp (before 1970) should parse successfully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseSlackTimestamp(tt.timestamp)
			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

func TestSlackSignatureConstants(t *testing.T) {
	assert.Equal(t, 5*time.Minute, SlackMaxTimestampAge,
		"Slack max timestamp age should be 5 minutes")

	assert.Equal(t, "v0", SlackSignatureVersion,
		"Slack signature version should be v0")
}

// Helper function to compute test signatures
func computeTestSignature(secret, timestamp string, body []byte) string {
	baseString := fmt.Sprintf("v0:%s:%s", timestamp, string(body))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(baseString))
	return fmt.Sprintf("v0=%s", hex.EncodeToString(mac.Sum(nil)))
}
