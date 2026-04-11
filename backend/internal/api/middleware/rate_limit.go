package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/FluidifyAI/Regen/backend/internal/redis"
)

// Fixed-window rate limiter backed by Redis.
//
// Algorithm: atomic Lua script — INCR + conditional EXPIRE in one round-trip.
// Key format: rl:{tier}:{ip}:{window_index}
//   where window_index = unix_seconds / windowSecs
//
// Fails open: if Redis is unavailable the request is allowed through and a
// warning is logged. This keeps the API alive during a Redis blip.
//
// Three pre-configured tiers (tuned for self-hosted single-tenant use):
//   RateLimitWebhooks() — 300 req/min  (alertmanager burst tolerance)
//   RateLimitAPI()      — 120 req/min  (normal UI/API usage)
//   RateLimitAuth()     —  10 req/min  (brute-force protection on login)

// luaIncr atomically increments a counter and sets its TTL on first touch.
// Returns the new count. The key expires automatically after windowSecs.
var luaIncr = `
local current = redis.call('INCR', KEYS[1])
if current == 1 then
    redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return current
`

// rateLimitWithIdentity returns a Gin middleware that limits to limit requests
// per windowSecs for the identity returned by identityFn.
func rateLimitWithIdentity(tier string, limit int, windowSecs int, identityFn func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if redis.Client == nil {
			// Redis not initialised — fail open.
			c.Next()
			return
		}

		identity := identityFn(c)
		windowIdx := time.Now().Unix() / int64(windowSecs)
		key := fmt.Sprintf("rl:%s:%s:%d", tier, identity, windowIdx)

		ctx, cancel := context.WithTimeout(c.Request.Context(), 200*time.Millisecond)
		defer cancel()

		res, err := redis.Client.Eval(ctx, luaIncr, []string{key}, windowSecs).Int64()
		if err != nil {
			slog.Warn("rate limiter redis error — failing open",
				"tier", tier, "identity", identity, "error", err)
			c.Next()
			return
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(max(0, limit-int(res))))
		c.Header("X-RateLimit-Reset", strconv.FormatInt((windowIdx+1)*int64(windowSecs), 10))

		if int(res) > limit {
			c.Header("Retry-After", strconv.Itoa(windowSecs))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"tier":        tier,
				"retry_after": windowSecs,
			})
			return
		}

		c.Next()
	}
}

// RateLimit returns a Gin middleware that allows at most limit requests per
// windowSecs seconds per client IP. tier is used in the Redis key and log fields.
func RateLimit(tier string, limit int, windowSecs int) gin.HandlerFunc {
	return rateLimitWithIdentity(tier, limit, windowSecs, func(c *gin.Context) string {
		return c.ClientIP()
	})
}

// RateLimitWebhooks allows 300 requests per minute per IP.
// Sized for Prometheus Alertmanager bursts (many alerts firing simultaneously).
func RateLimitWebhooks() gin.HandlerFunc {
	return RateLimit("webhook", 300, 60)
}

// RateLimitAPI limits authenticated requests by user-ID (600 req/min) and
// falls back to IP-based limiting (120 req/min) for unauthenticated requests.
//
// The higher authenticated limit prevents false positives when the UI fires
// many parallel requests (e.g. Gantt calendar loading one timeline per
// schedule). IP-based limits are insufficient for multi-user teams sharing an
// office NAT, and unnecessary once the user is already authenticated.
func RateLimitAPI() gin.HandlerFunc {
	return func(c *gin.Context) {
		if user := GetLocalUser(c); user != nil {
			// Authenticated: key by user ID, higher limit.
			rateLimitWithIdentity("api_user", 600, 60, func(*gin.Context) string {
				return user.ID.String()
			})(c)
			return
		}
		// Unauthenticated: fall back to IP-based limit.
		RateLimit("api", 120, 60)(c)
	}
}

// RateLimitAuth allows 10 requests per minute per IP.
// Applied to login endpoints to prevent brute-force attacks.
func RateLimitAuth() gin.HandlerFunc {
	return RateLimit("auth", 10, 60)
}

