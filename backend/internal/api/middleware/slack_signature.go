package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// SlackMaxTimestampAge is the maximum age of a request timestamp (5 minutes)
	// This prevents replay attacks
	SlackMaxTimestampAge = 5 * time.Minute

	// SlackSignatureVersion is the version prefix for Slack signatures
	SlackSignatureVersion = "v0"
)

// SlackSignatureVerification returns a middleware that verifies Slack request signatures.
// getSecret is called on every request so the signing secret is always current — no restart
// needed after Slack config is saved via the UI.
//
// This implements Slack's signature verification algorithm:
// https://api.slack.com/authentication/verifying-requests-from-slack
func SlackSignatureVerification(getSecret func() string) gin.HandlerFunc {
	return func(c *gin.Context) {
		signingSecret := getSecret()
		if signingSecret == "" {
			slog.Warn("Slack signing secret not configured - signature verification disabled")
			c.Next()
			return
		}
		// Get the timestamp and signature headers
		timestamp := c.GetHeader("X-Slack-Request-Timestamp")
		signature := c.GetHeader("X-Slack-Signature")

		// Both headers are required
		if timestamp == "" || signature == "" {
			slog.Warn("slack request missing signature headers",
				"path", c.Request.URL.Path,
				"has_timestamp", timestamp != "",
				"has_signature", signature != "",
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing signature headers",
			})
			return
		}

		// Parse and validate timestamp
		requestTime, err := parseSlackTimestamp(timestamp)
		if err != nil {
			slog.Warn("invalid slack timestamp",
				"timestamp", timestamp,
				"error", err,
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid timestamp",
			})
			return
		}

		// Check timestamp age (prevent replay attacks)
		age := time.Since(requestTime)
		if age > SlackMaxTimestampAge {
			slog.Warn("slack request timestamp too old",
				"timestamp", timestamp,
				"age_seconds", age.Seconds(),
				"max_age_seconds", SlackMaxTimestampAge.Seconds(),
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "timestamp too old",
				"details": "request must be sent within 5 minutes",
			})
			return
		}

		// Future timestamps are also invalid
		if age < 0 {
			slog.Warn("slack request timestamp in the future",
				"timestamp", timestamp,
				"offset_seconds", math.Abs(age.Seconds()),
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid timestamp",
			})
			return
		}

		// Read the request body
		// We need to read it to compute the signature, then restore it for handlers
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			slog.Error("failed to read request body for signature verification",
				"error", err,
			)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "internal server error",
			})
			return
		}

		// Restore the body for downstream handlers
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// Compute the expected signature
		expectedSig := computeSlackSignature(signingSecret, timestamp, bodyBytes)

		// Compare signatures using constant-time comparison
		if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
			slog.Warn("slack signature verification failed",
				"path", c.Request.URL.Path,
				"client_ip", c.ClientIP(),
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid signature",
			})
			return
		}

		// Store the buffered body so handlers can read it without a second io.ReadAll.
		c.Set("slack_raw_body", bodyBytes)

		// Signature is valid, proceed
		c.Next()
	}
}

// parseSlackTimestamp parses a Slack timestamp header value (Unix timestamp as string)
func parseSlackTimestamp(timestamp string) (time.Time, error) {
	unixTime, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timestamp format: %w", err)
	}
	return time.Unix(unixTime, 0), nil
}

// computeSlackSignature computes the HMAC-SHA256 signature for a Slack request
//
// Algorithm (per Slack docs):
// 1. Concatenate version, timestamp, and body with colons: "v0:timestamp:body"
// 2. Compute HMAC-SHA256 using signing secret as key
// 3. Format as "v0=<hex_encoded_hash>"
func computeSlackSignature(signingSecret, timestamp string, body []byte) string {
	// Build the base string: v0:timestamp:body
	baseString := fmt.Sprintf("%s:%s:%s", SlackSignatureVersion, timestamp, string(body))

	// Compute HMAC-SHA256
	mac := hmac.New(sha256.New, []byte(signingSecret))
	mac.Write([]byte(baseString))
	hash := mac.Sum(nil)

	// Format as v0=<hex>
	return fmt.Sprintf("%s=%s", SlackSignatureVersion, hex.EncodeToString(hash))
}

// SlackBodyFromContext retrieves the raw request body buffered by SlackSignatureVerification.
// Falls back to reading from c.Request.Body when the key is absent (e.g. in unit tests).
func SlackBodyFromContext(c *gin.Context) ([]byte, error) {
	if raw, ok := c.Get("slack_raw_body"); ok {
		if b, ok := raw.([]byte); ok {
			return b, nil
		}
	}
	return io.ReadAll(c.Request.Body)
}
