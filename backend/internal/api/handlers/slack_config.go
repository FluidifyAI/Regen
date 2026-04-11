package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/slack-go/slack"
)

type slackConfigResponse struct {
	Configured    bool       `json:"configured"`
	WorkspaceID   string     `json:"workspace_id,omitempty"`
	WorkspaceName string     `json:"workspace_name,omitempty"`
	BotUserID     string     `json:"bot_user_id,omitempty"`
	HasBotToken   bool       `json:"has_bot_token"`
	HasAppToken   bool       `json:"has_app_token"`
	HasOAuthConfig bool      `json:"has_oauth_config"`
	ConnectedAt   *time.Time `json:"connected_at,omitempty"`
}

func toSlackConfigResponse(cfg *models.SlackConfig) slackConfigResponse {
	if cfg == nil || cfg.BotToken == "" {
		return slackConfigResponse{Configured: false}
	}
	return slackConfigResponse{
		Configured:     true,
		WorkspaceID:    cfg.WorkspaceID,
		WorkspaceName:  cfg.WorkspaceName,
		BotUserID:      cfg.BotUserID,
		HasBotToken:    cfg.BotToken != "",
		HasAppToken:    cfg.AppToken != "",
		HasOAuthConfig: cfg.OAuthClientID != "" && cfg.OAuthClientSecret != "",
		ConnectedAt:    &cfg.ConnectedAt,
	}
}

// GetSlackConfig returns Slack connection status (no token values).
func GetSlackConfig(repo repository.SlackConfigRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := repo.Get()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load slack config"})
			return
		}
		c.JSON(http.StatusOK, toSlackConfigResponse(cfg))
	}
}

// SaveSlackConfig stores Slack tokens.
func SaveSlackConfig(repo repository.SlackConfigRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			BotToken          string `json:"bot_token"`
			SigningSecret     string `json:"signing_secret"`
			AppToken          string `json:"app_token"`
			WorkspaceID       string `json:"workspace_id"`
			WorkspaceName     string `json:"workspace_name"`
			BotUserID         string `json:"bot_user_id"`
			OAuthClientID     string `json:"oauth_client_id"`
			OAuthClientSecret string `json:"oauth_client_secret"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		if req.BotToken == "" || req.SigningSecret == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bot_token and signing_secret are required"})
			return
		}

		var connectedBy *uuid.UUID
		if uid, ok := c.Get("user_id"); ok {
			if id, err := uuid.Parse(uid.(string)); err == nil {
				connectedBy = &id
			}
		}

		cfg := &models.SlackConfig{
			BotToken:          req.BotToken,
			SigningSecret:     req.SigningSecret,
			AppToken:          req.AppToken,
			WorkspaceID:       req.WorkspaceID,
			WorkspaceName:     req.WorkspaceName,
			BotUserID:         req.BotUserID,
			OAuthClientID:     req.OAuthClientID,
			OAuthClientSecret: req.OAuthClientSecret,
			ConnectedAt:       time.Now(),
			ConnectedBy:       connectedBy,
		}
		if err := repo.Save(cfg); err != nil {
			slog.Error("failed to save slack config", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save slack config"})
			return
		}
		c.JSON(http.StatusOK, toSlackConfigResponse(cfg))
	}
}

// TestSlackConfig calls auth.test on the Slack API and returns workspace info.
func TestSlackConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			BotToken string `json:"bot_token" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bot_token is required"})
			return
		}

		client := slack.New(req.BotToken)
		resp, err := client.AuthTest()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "slack auth failed: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"workspace_id":   resp.TeamID,
			"workspace_name": resp.Team,
			"bot_user_id":    resp.UserID,
			"bot_username":   resp.User,
		})
	}
}

// DeleteSlackConfig removes the Slack integration.
func DeleteSlackConfig(repo repository.SlackConfigRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := repo.Delete(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete slack config"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "slack integration removed"})
	}
}

// GetSlackOAuthConfig returns whether Slack OAuth login is enabled (public endpoint — no auth required).
func GetSlackOAuthConfig(repo repository.SlackConfigRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := repo.Get()
		if err != nil || cfg == nil {
			c.JSON(http.StatusOK, gin.H{"enabled": false})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"enabled":   cfg.OAuthClientID != "" && cfg.OAuthClientSecret != "",
			"client_id": cfg.OAuthClientID,
		})
	}
}
