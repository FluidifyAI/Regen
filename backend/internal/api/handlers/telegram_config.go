package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/FluidifyAI/Regen/backend/internal/services"
)

type telegramConfigResponse struct {
	Configured  bool       `json:"configured"`
	ChatID      string     `json:"chat_id,omitempty"`
	ChatName    string     `json:"chat_name,omitempty"`
	HasToken    bool       `json:"has_token"`
	ConnectedAt *time.Time `json:"connected_at,omitempty"`
}

func toTelegramConfigResponse(cfg *models.TelegramConfig) telegramConfigResponse {
	if cfg == nil || cfg.BotToken == "" {
		return telegramConfigResponse{Configured: false}
	}
	return telegramConfigResponse{
		Configured:  true,
		ChatID:      cfg.ChatID,
		ChatName:    cfg.ChatName,
		HasToken:    true,
		ConnectedAt: &cfg.ConnectedAt,
	}
}

// GetTelegramConfig returns Telegram connection status (never returns the token).
func GetTelegramConfig(repo repository.TelegramConfigRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := repo.Get()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load telegram config"})
			return
		}
		c.JSON(http.StatusOK, toTelegramConfigResponse(cfg))
	}
}

// SaveTelegramConfig stores bot token and chat ID and hot-reloads the in-memory service.
func SaveTelegramConfig(repo repository.TelegramConfigRepository, incidentSvc services.IncidentService, appURL string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			BotToken string `json:"bot_token" binding:"required"`
			ChatID   string `json:"chat_id"   binding:"required"`
			ChatName string `json:"chat_name"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bot_token and chat_id are required"})
			return
		}

		var connectedBy *uuid.UUID
		if uid, ok := c.Get("user_id"); ok {
			if id, err := uuid.Parse(uid.(string)); err == nil {
				connectedBy = &id
			}
		}

		cfg := &models.TelegramConfig{
			BotToken:    req.BotToken,
			ChatID:      req.ChatID,
			ChatName:    req.ChatName,
			ConnectedAt: time.Now(),
			ConnectedBy: connectedBy,
		}
		if err := repo.Save(cfg); err != nil {
			slog.Error("failed to save telegram config", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save telegram config"})
			return
		}
		// Hot-reload: wire the new service into the incident service without restart
		if tgSvc := services.NewTelegramServiceFromConfig(cfg, appURL); tgSvc != nil {
			services.SetTelegramService(incidentSvc, tgSvc)
			slog.Info("telegram service hot-reloaded", "chat_id", cfg.ChatID)
		}
		c.JSON(http.StatusOK, toTelegramConfigResponse(cfg))
	}
}

// TestTelegramConfig validates the bot token and sends a test message.
func TestTelegramConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			BotToken string `json:"bot_token" binding:"required"`
			ChatID   string `json:"chat_id"   binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bot_token and chat_id are required"})
			return
		}

		botUsername, err := services.TestTelegramConnection(c.Request.Context(), req.BotToken, req.ChatID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"bot_username": botUsername, "message": "Test message sent successfully"})
	}
}

// FetchTelegramChatID calls getUpdates to discover the chat ID from recent messages.
func FetchTelegramChatID() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			BotToken string `json:"bot_token" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bot_token is required"})
			return
		}

		chatID, chatName, err := services.FetchTelegramChatID(c.Request.Context(), req.BotToken)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"chat_id": chatID, "chat_name": chatName})
	}
}

// DeleteTelegramConfig removes the Telegram integration and disables the in-memory service.
func DeleteTelegramConfig(repo repository.TelegramConfigRepository, incidentSvc services.IncidentService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := repo.Delete(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete telegram config"})
			return
		}
		services.SetTelegramService(incidentSvc, nil)
		slog.Info("telegram service disabled")
		c.JSON(http.StatusOK, gin.H{"message": "telegram integration removed"})
	}
}
