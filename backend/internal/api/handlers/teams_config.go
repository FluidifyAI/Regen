package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/FluidifyAI/Regen/backend/internal/services"
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

// DownloadTeamsAppPackage generates and streams a sideloadable Teams app zip.
// The package contains manifest.json (with the stored bot App ID) and two icons.
func DownloadTeamsAppPackage(repo repository.TeamsConfigRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := repo.Get()
		if err != nil || cfg == nil || cfg.AppID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "teams is not configured — save your credentials first"})
			return
		}

		manifest, err := teamsManifestJSON(cfg.AppID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate manifest"})
			return
		}
		colorIcon, err := teamsColorIcon()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate color icon"})
			return
		}
		outlineIcon, err := teamsOutlineIcon()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate outline icon"})
			return
		}

		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		for _, f := range []struct {
			name string
			data []byte
		}{
			{"manifest.json", manifest},
			{"color.png", colorIcon},
			{"outline.png", outlineIcon},
		} {
			w, err := zw.Create(f.name)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create zip"})
				return
			}
			if _, err = w.Write(f.data); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write zip"})
				return
			}
		}
		if err := zw.Close(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to finalise zip"})
			return
		}

		c.Header("Content-Disposition", "attachment; filename=\"fluidify-regen-teams-app.zip\"")
		c.Header("Content-Type", "application/zip")
		c.Data(http.StatusOK, "application/zip", buf.Bytes())
	}
}

// teamsManifestJSON builds the Teams manifest.json for the given bot App ID.
func teamsManifestJSON(appID string) ([]byte, error) {
	type command struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	type commandList struct {
		Scopes   []string  `json:"scopes"`
		Commands []command `json:"commands"`
	}
	type bot struct {
		BotID              string        `json:"botId"`
		Scopes             []string      `json:"scopes"`
		SupportsFiles      bool          `json:"supportsFiles"`
		IsNotificationOnly bool          `json:"isNotificationOnly"`
		CommandLists       []commandList `json:"commandLists"`
	}
	manifest := map[string]any{
		"$schema":         "https://developer.microsoft.com/en-us/json-schemas/teams/v1.17/MicrosoftTeams.schema.json",
		"manifestVersion": "1.17",
		"version":         "1.0.0",
		"id":              fmt.Sprintf("%s-teams-app", appID),
		"packageName":     "com.fluidify.regen",
		"developer": map[string]string{
			"name":           "Fluidify",
			"websiteUrl":     "https://fluidify.ai",
			"privacyUrl":     "https://fluidify.ai/privacy",
			"termsOfUseUrl":  "https://fluidify.ai/terms",
		},
		"icons": map[string]string{
			"color":   "color.png",
			"outline": "outline.png",
		},
		"name": map[string]string{
			"short": "Fluidify Regen",
			"full":  "Fluidify Regen — Incident Management",
		},
		"description": map[string]string{
			"short": "Declare, track and resolve incidents from Teams",
			"full":  "Fluidify Regen brings your full incident lifecycle into Microsoft Teams. Create incidents, acknowledge alerts, assign commanders, and resolve — all without leaving Teams. Open source. Self-hosted. No per-seat fees.",
		},
		"accentColor": "#1800AD",
		"bots": []bot{
			{
				BotID:              appID,
				Scopes:             []string{"team", "personal"},
				SupportsFiles:      false,
				IsNotificationOnly: false,
				CommandLists: []commandList{
					{
						Scopes: []string{"team", "personal"},
						Commands: []command{
							{Title: "new", Description: "Create a new incident: new <title>"},
							{Title: "ack", Description: "Acknowledge the active incident in this channel"},
							{Title: "resolve", Description: "Resolve the active incident in this channel"},
							{Title: "status", Description: "Show current incident status"},
							{Title: "help", Description: "List all available commands"},
						},
					},
				},
			},
		},
		"permissions":  []string{"identity", "messageTeamMembers"},
		"validDomains": []string{},
	}
	return json.MarshalIndent(manifest, "", "  ")
}

// teamsColorIcon generates a 192×192 PNG: brand purple (#1800AD) with white "FR".
func teamsColorIcon() ([]byte, error) {
	const size = 192
	purple := color.NRGBA{R: 24, G: 0, B: 173, A: 255}
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}

	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			img.SetNRGBA(x, y, purple)
		}
	}

	// 5×7 bitmap glyphs
	glyphs := map[rune][7][5]bool{
		'F': {
			{true, true, true, true, true},
			{true, false, false, false, false},
			{true, false, false, false, false},
			{true, true, true, true, false},
			{true, false, false, false, false},
			{true, false, false, false, false},
			{true, false, false, false, false},
		},
		'R': {
			{true, true, true, true, false},
			{true, false, false, false, true},
			{true, false, false, false, true},
			{true, true, true, true, false},
			{true, false, true, false, false},
			{true, false, false, true, false},
			{true, false, false, false, true},
		},
	}

	const scale = 8
	totalW := (5 + 1 + 5) * scale
	totalH := 7 * scale
	ox := (size - totalW) / 2
	oy := (size - totalH) / 2

	drawGlyph := func(g [7][5]bool, startX int) {
		for gy, row := range g {
			for gx, on := range row {
				if on {
					for dy := range scale {
						for dx := range scale {
							img.SetNRGBA(startX+gx*scale+dx, oy+gy*scale+dy, white)
						}
					}
				}
			}
		}
	}
	drawGlyph(glyphs['F'], ox)
	drawGlyph(glyphs['R'], ox+(5+1)*scale)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// teamsOutlineIcon generates a 32×32 PNG: white circle on transparent background.
func teamsOutlineIcon() ([]byte, error) {
	const size = 32
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	cx, cy, r := float64(size)/2, float64(size)/2, 13.0
	for y := range size {
		for x := range size {
			dist := math.Sqrt(math.Pow(float64(x)+0.5-cx, 2) + math.Pow(float64(y)+0.5-cy, 2))
			if dist <= r {
				img.SetNRGBA(x, y, white)
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
