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
// teamsSvc is constructed once in serve.go and shared with the API router.
func StartAll(ctx context.Context, db *gorm.DB, cfg *config.Config, teamsSvc *services.TeamsService) {
	// Initialize dependencies for the shift notifier
	scheduleRepo := repository.NewScheduleRepository(db)
	scheduleEvaluator := services.NewScheduleEvaluator(scheduleRepo)

	// Slack is optional — workers run but DM sends become no-ops when unconfigured.
	var chatService services.ChatService
	if cfg.SlackBotToken != "" {
		var err error
		chatService, err = services.NewSlackService(cfg.SlackBotToken)
		if err != nil {
			slog.Warn("failed to initialize slack for workers (will skip notifications)",
				"error", err)
		}
	}

	// Build a multi-provider chat service that fans out to Slack and/or Teams.
	// The shift notifier and escalation worker use this for on-call DMs.
	activeChatServices := make([]services.ChatService, 0, 2)
	if chatService != nil {
		activeChatServices = append(activeChatServices, chatService)
	}
	if teamsSvc != nil {
		activeChatServices = append(activeChatServices, teamsSvc)
	}
	var workerChatService services.ChatService
	switch len(activeChatServices) {
	case 1:
		workerChatService = activeChatServices[0]
	case 2:
		workerChatService = services.NewMultiChatService(activeChatServices...)
	}

	// Start the shift notifier
	notifier := NewShiftNotifier(scheduleRepo, scheduleEvaluator, workerChatService)
	go notifier.Run(ctx)

	// Start the escalation worker.
	// Build the escalation engine here with the worker as its notifier so that
	// the engine can immediately send DMs when a tier fires.
	escalationPolicyRepo := repository.NewEscalationPolicyRepository(db)
	escalationWorker := NewEscalationWorker(workerChatService)
	escalationEngine := services.NewEscalationEngine(escalationPolicyRepo, scheduleEvaluator, escalationWorker)
	escalationWorker.SetEngine(escalationEngine)
	go escalationWorker.Run(ctx)

	slog.Info("background workers started")
}
