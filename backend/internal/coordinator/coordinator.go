package coordinator

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Agent is the interface all AI agents implement.
type Agent interface {
	Handle(ctx context.Context, incidentID uuid.UUID)
}

// resolvedPayload is the JSON published to "events:incident.resolved".
type resolvedPayload struct {
	IncidentID string `json:"incident_id"`
	AIEnabled  bool   `json:"ai_enabled"`
}

// AICoordinator subscribes to Redis and routes incident events to agents.
type AICoordinator struct {
	redis           *redis.Client
	postMortemAgent Agent
}

// New creates an AICoordinator wired to real Redis and the provided agents.
func New(redisClient *redis.Client, postMortemAgent Agent) *AICoordinator {
	return &AICoordinator{redis: redisClient, postMortemAgent: postMortemAgent}
}

// NewTestCoordinator creates an AICoordinator for unit tests (no Redis).
func NewTestCoordinator(postMortemAgent Agent) *AICoordinator {
	return &AICoordinator{postMortemAgent: postMortemAgent}
}

// Start subscribes to the event stream and blocks until ctx is cancelled.
// Run this in a goroutine: go coordinator.Start(appCtx)
func (c *AICoordinator) Start(ctx context.Context) {
	sub := c.redis.PSubscribe(ctx, "events:incident.*")
	defer sub.Close()
	slog.Info("AI coordinator started")

	for {
		select {
		case <-ctx.Done():
			slog.Info("AI coordinator stopped")
			return
		case msg, ok := <-sub.Channel():
			if !ok {
				return
			}
			go c.RoutePayload(msg.Channel, []byte(msg.Payload))
		}
	}
}

// RoutePayload parses a raw event payload and dispatches to the correct agent.
// Exported for testability (called from Start and unit tests).
func (c *AICoordinator) RoutePayload(channel string, payload []byte) {
	switch channel {
	case "events:incident.resolved":
		var p resolvedPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			slog.Error("coordinator: failed to parse resolved payload", "error", err)
			return
		}
		if !p.AIEnabled {
			slog.Info("coordinator: AI disabled for incident, skipping", "incident_id", p.IncidentID)
			return
		}
		id, err := uuid.Parse(p.IncidentID)
		if err != nil {
			slog.Error("coordinator: invalid incident_id", "incident_id", p.IncidentID)
			return
		}
		slog.Info("coordinator: routing to post-mortem agent", "incident_id", id)
		defer func() {
			if r := recover(); r != nil {
				slog.Error("coordinator: post-mortem agent panicked", "recover", r)
			}
		}()
		c.postMortemAgent.Handle(context.Background(), id)
	}
}
