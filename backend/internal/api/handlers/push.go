package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/FluidifyAI/Regen/backend/internal/api/middleware"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/gin-gonic/gin"
)

// validPlatforms is the set of accepted platform identifiers.
var validPlatforms = map[string]bool{
	"ios":     true,
	"android": true,
	"web":     true,
}

// maxTokenBytes is the maximum accepted byte length for a device token.
const maxTokenBytes = 4096

type registerTokenRequest struct {
	Token      string `json:"token"`
	Platform   string `json:"platform"`
	AppVersion string `json:"app_version"`
}

type unregisterTokenRequest struct {
	Token string `json:"token"`
}

// RegisterDeviceToken handles POST /push/register.
// Registers an FCM device token for the authenticated user.
// Returns 204 on success, 422 on validation failure, 429 when the token cap is reached.
func RegisterDeviceToken(repo repository.DeviceTokenRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req registerTokenRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid request body"})
			return
		}

		// Validate token
		if req.Token == "" {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "token is required"})
			return
		}
		if len(req.Token) > maxTokenBytes {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": "token exceeds maximum length of 4096 bytes",
			})
			return
		}

		// Validate platform
		if req.Platform == "" {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "platform is required"})
			return
		}
		if !validPlatforms[req.Platform] {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": "platform must be one of: ios, android, web",
			})
			return
		}

		user := middleware.GetLocalUser(c)
		if user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		if err := repo.Upsert(user.ID, req.Token, req.Platform, req.AppVersion); err != nil {
			if errors.Is(err, repository.ErrTokenLimitExceeded) {
				c.JSON(http.StatusTooManyRequests, gin.H{"error": "token_limit_exceeded"})
				return
			}
			slog.Error("push: failed to register device token",
				"user_id", user.ID, "platform", req.Platform, "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		slog.Info("push: device token registered", "user_id", user.ID, "platform", req.Platform)
		c.Status(http.StatusNoContent)
	}
}

// UnregisterDeviceToken handles POST /push/unregister.
// Removes an FCM device token for the authenticated user. Idempotent — returns 204 even if the
// token was not registered.
func UnregisterDeviceToken(repo repository.DeviceTokenRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req unregisterTokenRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid request body"})
			return
		}

		if req.Token == "" {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "token is required"})
			return
		}

		user := middleware.GetLocalUser(c)
		if user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		_, err := repo.DeleteByUserAndToken(user.ID, req.Token)
		if err != nil {
			slog.Error("push: failed to unregister device token",
				"user_id", user.ID, "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		slog.Info("push: device token unregistered", "user_id", user.ID)
		c.Status(http.StatusNoContent)
	}
}
