package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/config"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// posthogAPIKey is a write-only capture key — safe to embed in open-source code.
// It can only ingest events; it cannot read data or access the PostHog dashboard.
const posthogAPIKey = "phc_PLACEHOLDER"

// TelemetryWorker runs two background tasks:
//  1. Daily heartbeat to PostHog with anonymous aggregate stats
//  2. Every-6h poll of api.fluidify.ai/regen/announcements, cached for the API
type TelemetryWorker struct {
	db   *gorm.DB
	cfg  *config.Config
	repo repository.SystemSettingsRepository

	announcementsMu sync.RWMutex
	announcements   []byte

	httpClient *http.Client
}

func NewTelemetryWorker(db *gorm.DB, cfg *config.Config, repo repository.SystemSettingsRepository) *TelemetryWorker {
	return &TelemetryWorker{
		db:         db,
		cfg:        cfg,
		repo:       repo,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetCachedAnnouncements returns the last successfully fetched announcements JSON.
// Falls back to an empty list when nothing has been fetched yet.
func (tw *TelemetryWorker) GetCachedAnnouncements() []byte {
	tw.announcementsMu.RLock()
	defer tw.announcementsMu.RUnlock()
	if tw.announcements == nil {
		return []byte(`{"announcements":[]}`)
	}
	return tw.announcements
}

func (tw *TelemetryWorker) Run(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("telemetry worker panicked", "panic", r)
		}
	}()

	instanceID := tw.ensureInstanceID()
	slog.Info("telemetry worker started", "instance_id", instanceID, "disabled", tw.cfg.TelemetryDisabled)

	tw.sendHeartbeat(instanceID)
	tw.fetchAnnouncements()

	heartbeatTicker := time.NewTicker(24 * time.Hour)
	announcementsTicker := time.NewTicker(6 * time.Hour)
	defer heartbeatTicker.Stop()
	defer announcementsTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeatTicker.C:
			tw.sendHeartbeat(instanceID)
		case <-announcementsTicker.C:
			tw.fetchAnnouncements()
		}
	}
}

func (tw *TelemetryWorker) ensureInstanceID() string {
	id, _ := tw.repo.GetInstanceID()
	if id == "" {
		id = uuid.New().String()
		if err := tw.repo.SetInstanceID(id); err != nil {
			slog.Warn("telemetry: failed to persist instance ID", "error", err)
		}
	}
	return id
}

func (tw *TelemetryWorker) isOptedOut() bool {
	if tw.cfg.TelemetryDisabled {
		return true
	}
	optOut, _ := tw.repo.GetTelemetryOptOut()
	return optOut
}

func (tw *TelemetryWorker) sendHeartbeat(instanceID string) {
	defer func() { recover() }() //nolint:errcheck

	if tw.isOptedOut() {
		return
	}

	var incidentCount, userCount int64
	tw.db.Table("incidents").Where("created_at > NOW() - INTERVAL '30 days'").Count(&incidentCount)
	tw.db.Table("users").Where("auth_source != 'deactivated' AND auth_source != 'ai'").Count(&userCount)

	batch := map[string]any{
		"api_key": posthogAPIKey,
		"batch": []map[string]any{
			{
				"event":       "regen_heartbeat",
				"distinct_id": instanceID,
				"timestamp":   time.Now().UTC().Format(time.RFC3339),
				"properties": map[string]any{
					"version":        "1.0.0",
					"incident_count": incidentCount,
					"user_count":     userCount,
					"slack_enabled":  tw.cfg.SlackBotToken != "",
					"teams_enabled":  tw.cfg.TeamsAppID != "",
					"ai_enabled":     tw.cfg.OpenAIAPIKey != "",
					"saml_enabled":   tw.cfg.SAMLIDPMetadataURL != "",
				},
			},
		},
	}

	payload, err := json.Marshal(batch)
	if err != nil {
		return
	}

	req, err := http.NewRequest(http.MethodPost, "https://us.i.posthog.com/batch/", bytes.NewReader(payload))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := tw.httpClient.Do(req)
	if err != nil {
		slog.Warn("telemetry: heartbeat send failed", "error", err)
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) //nolint:errcheck
}

func (tw *TelemetryWorker) fetchAnnouncements() {
	defer func() { recover() }() //nolint:errcheck

	req, err := http.NewRequest(http.MethodGet, "https://api.fluidify.ai/regen/announcements", nil)
	if err != nil {
		return
	}

	resp, err := tw.httpClient.Do(req)
	if err != nil {
		slog.Warn("telemetry: announcement fetch failed", "error", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		slog.Warn("telemetry: failed to read announcements", "error", err)
		return
	}

	if !json.Valid(body) {
		slog.Warn("telemetry: invalid JSON in announcements response")
		return
	}

	tw.announcementsMu.Lock()
	tw.announcements = body
	tw.announcementsMu.Unlock()
}
