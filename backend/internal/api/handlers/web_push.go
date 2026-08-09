package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/FluidifyAI/Regen/backend/internal/api/middleware"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/gin-gonic/gin"
)

const (
	maxWebPushEndpointBytes = 2048
	maxWebPushKeyBytes      = 512
)

type registerWebPushRequest struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256DH string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

type unregisterWebPushRequest struct {
	Endpoint string `json:"endpoint"`
}

// GetVAPIDPublicKey handles GET /push/vapid-public-key.
// Returns the VAPID public key needed by PushManager.subscribe() in the browser.
// Returns 503 when Web Push is not configured (vapidPublicKey is empty).
func GetVAPIDPublicKey(vapidPublicKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if vapidPublicKey == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "web_push_not_configured"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"public_key": vapidPublicKey})
	}
}

// RegisterWebPushSubscription handles POST /push/web/register.
// Stores a Web Push subscription (endpoint + keys) for the authenticated user.
// Returns 204 on success, 422 on validation failure, 429 when the per-user cap is reached,
// 503 when Web Push is not configured.
func RegisterWebPushSubscription(repo repository.WebPushSubscriptionRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req registerWebPushRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid request body"})
			return
		}

		// Validate endpoint.
		if req.Endpoint == "" {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "endpoint is required"})
			return
		}
		if !strings.HasPrefix(req.Endpoint, "https://") {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "endpoint must use https scheme"})
			return
		}
		if len(req.Endpoint) > maxWebPushEndpointBytes {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": "endpoint exceeds maximum length of 2048 bytes",
			})
			return
		}

		// Validate keys.
		if req.Keys.P256DH == "" {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "keys.p256dh is required"})
			return
		}
		if len(req.Keys.P256DH) > maxWebPushKeyBytes {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "keys.p256dh exceeds maximum length"})
			return
		}
		if req.Keys.Auth == "" {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "keys.auth is required"})
			return
		}
		if len(req.Keys.Auth) > maxWebPushKeyBytes {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "keys.auth exceeds maximum length"})
			return
		}

		user := middleware.GetLocalUser(c)
		if user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		if err := repo.Upsert(user.ID, req.Endpoint, req.Keys.P256DH, req.Keys.Auth); err != nil {
			if errors.Is(err, repository.ErrTokenLimitExceeded) {
				c.JSON(http.StatusTooManyRequests, gin.H{"error": "token_limit_exceeded"})
				return
			}
			slog.Error("web push: failed to register subscription",
				"user_id", user.ID, "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		slog.Info("web push: subscription registered", "user_id", user.ID)
		c.Status(http.StatusNoContent)
	}
}

// UnregisterWebPushSubscription handles POST /push/web/unregister.
// Removes a Web Push subscription for the authenticated user. Idempotent — returns 204
// even if the subscription was not registered.
func UnregisterWebPushSubscription(repo repository.WebPushSubscriptionRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req unregisterWebPushRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid request body"})
			return
		}

		if req.Endpoint == "" {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "endpoint is required"})
			return
		}

		user := middleware.GetLocalUser(c)
		if user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		_, err := repo.DeleteByUserAndEndpoint(user.ID, req.Endpoint)
		if err != nil {
			slog.Error("web push: failed to unregister subscription",
				"user_id", user.ID, "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		slog.Info("web push: subscription unregistered", "user_id", user.ID)
		c.Status(http.StatusNoContent)
	}
}
