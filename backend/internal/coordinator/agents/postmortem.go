package agents

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/repository"
	"github.com/openincident/openincident/internal/services"
	"gorm.io/gorm"
)

const minIncidentDuration = 5 * time.Minute
const defaultWaitDuration = 60 * time.Second

// PostMortemAgentDeps holds all dependencies for the Post-Mortem Agent.
// WaitDuration is configurable so tests can set it to 0 (skip the wait).
// Production callers pass defaultWaitDuration explicitly.
type PostMortemAgentDeps struct {
	AgentUserID  uuid.UUID
	AISvc        interface{ IsEnabled() bool }
	IncidentRepo interface{ GetByID(uuid.UUID) (*models.Incident, error) }
	PostMortemSvc interface {
		GetPostMortem(uuid.UUID) (*models.PostMortem, error)
		GeneratePostMortem(*models.Incident, *uuid.UUID, string) (*models.PostMortem, error)
	}
	TimelineRepo repository.TimelineRepository // nil in tests
	SlackSvc     services.ChatService          // nil if not configured
	TeamsSvc     interface {                   // nil if not configured
		PostToConversation(conversationID string, msg services.Message) (string, error)
	}
	MultiChat   services.ChatService     // for DM to commander; nil if not configured
	UserRepo    repository.UserRepository // for commander lookup; nil in tests
	FrontendURL string
	WaitDuration time.Duration
}

// PostMortemAgent auto-drafts a post-mortem when an incident resolves.
type PostMortemAgent struct {
	deps PostMortemAgentDeps
}

// NewPostMortemAgent creates a PostMortemAgent.
// WaitDuration == 0 means skip the wait entirely (test/no-wait mode).
// Production code passes defaultWaitDuration explicitly.
func NewPostMortemAgent(deps PostMortemAgentDeps) *PostMortemAgent {
	return &PostMortemAgent{deps: deps}
}

// Handle is called by the AICoordinator when an incident resolves.
func (a *PostMortemAgent) Handle(ctx context.Context, incidentID uuid.UUID) {
	// Step 1: wait for timeline to settle (skip if WaitDuration == 0)
	if a.deps.WaitDuration > 0 {
		select {
		case <-ctx.Done():
			return
		case <-time.After(a.deps.WaitDuration):
		}
	}

	// Step 2: precondition — AI configured
	if !a.deps.AISvc.IsEnabled() {
		slog.Info("post-mortem agent: AI not configured, skipping", "incident_id", incidentID)
		return
	}

	// Step 3: fetch incident
	incident, err := a.deps.IncidentRepo.GetByID(incidentID)
	if err != nil {
		slog.Error("post-mortem agent: failed to fetch incident", "incident_id", incidentID, "error", err)
		return
	}

	// Step 4: precondition — incident duration >= 5 minutes
	if incident.ResolvedAt != nil {
		duration := incident.ResolvedAt.Sub(incident.TriggeredAt)
		if duration < minIncidentDuration {
			slog.Info("post-mortem agent: incident too short, skipping",
				"incident_id", incidentID, "duration", duration)
			return
		}
	}

	// Step 5: precondition — no existing post-mortem
	_, err = a.deps.PostMortemSvc.GetPostMortem(incidentID)
	if err == nil {
		slog.Info("post-mortem agent: post-mortem already exists, skipping", "incident_id", incidentID)
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		slog.Error("post-mortem agent: failed to check existing post-mortem", "error", err)
		return
	}

	// Step 6: generate
	pm, err := a.deps.PostMortemSvc.GeneratePostMortem(incident, nil, a.deps.AgentUserID.String())
	if err != nil {
		slog.Error("post-mortem agent: generation failed", "incident_id", incidentID, "error", err)
		return
	}
	slog.Info("post-mortem agent: draft created", "incident_id", incidentID, "postmortem_id", pm.ID)

	// Step 7: write timeline entry
	a.writeTimelineEntry(incident, pm)

	// Step 8: notify
	a.notify(incident, pm)
}

func (a *PostMortemAgent) writeTimelineEntry(incident *models.Incident, pm *models.PostMortem) {
	if a.deps.TimelineRepo == nil {
		return
	}
	entry := &models.TimelineEntry{
		IncidentID: incident.ID,
		Timestamp:  time.Now().UTC(),
		Type:       "postmortem_drafted",
		ActorType:  "ai_agent",
		ActorID:    a.deps.AgentUserID.String(),
		Content: models.JSONB{
			"postmortem_id": pm.ID.String(),
			"agent":         "postmortem",
		},
	}
	if err := a.deps.TimelineRepo.Create(entry); err != nil {
		slog.Warn("post-mortem agent: failed to write timeline entry", "error", err)
	}
}

func (a *PostMortemAgent) notify(incident *models.Incident, pm *models.PostMortem) {
	link := a.deps.FrontendURL + "/incidents/" + incident.ID.String() + "/postmortem"
	channelMsg := services.Message{
		Text: "Post-Mortem Agent\n\n" +
			fmt.Sprintf("I've drafted a post-mortem for INC-%d - %s\n\n", incident.IncidentNumber, incident.Title) +
			"Severity: " + string(incident.Severity) + "\n" +
			"Review and edit: " + link,
	}

	// Post to Slack incident channel
	if a.deps.SlackSvc != nil && incident.SlackChannelID != "" {
		if _, err := a.deps.SlackSvc.PostMessage(incident.SlackChannelID, channelMsg); err != nil {
			slog.Warn("post-mortem agent: slack channel post failed", "error", err)
		}
	}

	// Post to Teams incident conversation
	if a.deps.TeamsSvc != nil && incident.TeamsConversationID != nil {
		if _, err := a.deps.TeamsSvc.PostToConversation(*incident.TeamsConversationID, channelMsg); err != nil {
			slog.Warn("post-mortem agent: teams channel post failed", "error", err)
		}
	}

	// DM the incident commander
	if a.deps.MultiChat != nil && a.deps.UserRepo != nil && incident.CommanderID != nil {
		commander, err := a.deps.UserRepo.GetByID(*incident.CommanderID)
		if err != nil {
			slog.Warn("post-mortem agent: commander not found for DM", "commander_id", incident.CommanderID)
			return
		}
		dmMsg := services.Message{
			Text: "Post-Mortem Agent\n\n" +
				fmt.Sprintf("As incident commander for INC-%d, I've drafted a post-mortem for your review.\n\n", incident.IncidentNumber) +
				"Review: " + link,
		}
		if err := a.deps.MultiChat.SendDirectMessage(commander.Email, dmMsg); err != nil {
			slog.Warn("post-mortem agent: DM to commander failed", "error", err)
		}
	}
}
