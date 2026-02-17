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

	// ChatService is optional — if Slack is not configured, the workers run
	// but all DM sends become graceful no-ops.
	var chatService services.ChatService
	if cfg.SlackBotToken != "" {
		var err error
		chatService, err = services.NewSlackService(cfg.SlackBotToken)
		if err != nil {
			slog.Warn("failed to initialize slack for workers (will skip notifications)",
				"error", err)
		}
	}

	// Start the shift notifier
	notifier := NewShiftNotifier(scheduleRepo, scheduleEvaluator, chatService)
	go notifier.Run(ctx)

	// Start the escalation worker.
	// Build the escalation engine here with the worker as its notifier so that
	// the engine can immediately send DMs when a tier fires.
	escalationPolicyRepo := repository.NewEscalationPolicyRepository(db)
	escalationWorker := NewEscalationWorker(chatService)
	escalationEngine := services.NewEscalationEngine(escalationPolicyRepo, scheduleEvaluator, escalationWorker)
	escalationWorker.SetEngine(escalationEngine)
	go escalationWorker.Run(ctx)

	slog.Info("background workers started")
}
