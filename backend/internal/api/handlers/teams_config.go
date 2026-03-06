package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/repository"
	"github.com/openincident/openincident/internal/services"
)

type teamsConfigResponse struct {
	Configured  bool       `json:"configured"`
	TeamID      string     `json:"team_id,omitempty"`
	TeamName    string     `json:"team_name,omitempty"`
	TenantID    string     `json:"tenant_id,omitempty"`
	AppID       string     `json:"app_id,omitempty"`
	ServiceURL  string     `json:"service_url,omitempty"`
	HasPassword bool       `json:"has_password"`
	ConnectedAt *time.Time `json:"connected_at,omitempty"`
}

func toTeamsConfigResponse(cfg *models.TeamsConfig) teamsConfigResponse {
	if cfg == nil || cfg.AppID == "" {
		return teamsConfigResponse{Configured: false}
	}
	return teamsConfigResponse{
		Configured:  true,
		TeamID:      cfg.TeamID,
		TeamName:    cfg.TeamName,
		TenantID:    cfg.TenantID,
		AppID:       cfg.AppID,
		ServiceURL:  cfg.ServiceURL,
		HasPassword: cfg.AppPassword != "",
		ConnectedAt: &cfg.ConnectedAt,
	}
}

// GetTeamsConfig returns Teams connection status (no secret values).
func GetTeamsConfig(repo repository.TeamsConfigRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := repo.Get()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load teams config"})
			return
		}
		c.JSON(http.StatusOK, toTeamsConfigResponse(cfg))
	}
}

// SaveTeamsConfig stores Teams credentials.
func SaveTeamsConfig(repo repository.TeamsConfigRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			AppID      string `json:"app_id"      binding:"required"`
			AppPassword string `json:"app_password" binding:"required"`
			TenantID   string `json:"tenant_id"   binding:"required"`
			TeamID     string `json:"team_id"     binding:"required"`
			BotUserID  string `json:"bot_user_id"`
			ServiceURL string `json:"service_url"`
			TeamName   string `json:"team_name"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "app_id, app_password, tenant_id, and team_id are required"})
			return
		}
		if req.ServiceURL == "" {
			req.ServiceURL = "https://smba.trafficmanager.net/amer/"
		}

		var connectedBy *uuid.UUID
		if uid, ok := c.Get("user_id"); ok {
			if id, err := uuid.Parse(uid.(string)); err == nil {
				connectedBy = &id
			}
		}

		cfg := &models.TeamsConfig{
			AppID:       req.AppID,
			AppPassword: req.AppPassword,
			TenantID:    req.TenantID,
			TeamID:      req.TeamID,
			BotUserID:   req.BotUserID,
			ServiceURL:  req.ServiceURL,
			TeamName:    req.TeamName,
			ConnectedAt: time.Now(),
			ConnectedBy: connectedBy,
		}
		if err := repo.Save(cfg); err != nil {
			slog.Error("failed to save teams config", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save teams config"})
			return
		}
		c.JSON(http.StatusOK, toTeamsConfigResponse(cfg))
	}
}

// TestTeamsConfig validates Teams credentials against the Graph API.
func TestTeamsConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			AppID      string `json:"app_id"      binding:"required"`
			AppPassword string `json:"app_password" binding:"required"`
			TenantID   string `json:"tenant_id"   binding:"required"`
			TeamID     string `json:"team_id"     binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "app_id, app_password, tenant_id, and team_id are required"})
			return
		}

		teamName, err := services.TestTeamsCredentials(c.Request.Context(), req.AppID, req.AppPassword, req.TenantID, req.TeamID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "teams auth failed: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"team_id":   req.TeamID,
			"team_name": teamName,
		})
	}
}

// DeleteTeamsConfig removes the Teams integration.
func DeleteTeamsConfig(repo repository.TeamsConfigRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := repo.Delete(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete teams config"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "teams integration removed"})
	}
}
