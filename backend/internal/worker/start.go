package worker

import (
	"context"
	"log/slog"

	"github.com/openincident/openincident/internal/config"
	"github.com/openincident/openincident/internal/repository"
	"github.com/openincident/openincident/internal/services"
	"gorm.io/gorm"
)

// StartAll starts all background workers.
// Should be called from main.go with the application lifecycle context.
func StartAll(ctx context.Context, db *gorm.DB, cfg *config.Config) {
	// Initialize dependencies for the shift notifier
	scheduleRepo := repository.NewScheduleRepository(db)
	scheduleEvaluator := services.NewScheduleEvaluator(scheduleRepo)

	// ChatService is optional — if Slack is not configured, the worker runs
	// but all DM sends become graceful no-ops.
	var chatService services.ChatService
	if cfg.SlackBotToken != "" {
		var err error
		chatService, err = services.NewSlackService(cfg.SlackBotToken)
		if err != nil {
			slog.Warn("failed to initialize slack for shift notifier (will skip notifications)",
				"error", err)
		}
	}

	// Start the shift notifier
	notifier := NewShiftNotifier(scheduleRepo, scheduleEvaluator, chatService)
	go notifier.Run(ctx)

	slog.Info("background workers started")
}
