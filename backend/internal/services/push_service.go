package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/google/uuid"
	"google.golang.org/api/option"
)

// FCMSender is the minimal Firebase messaging interface.
// Production implementation: *messaging.Client. Tests: mock.
type FCMSender interface {
	Send(ctx context.Context, message *messaging.Message) (string, error)
}

// Compile-time check: *messaging.Client satisfies FCMSender.
var _ FCMSender = &messaging.Client{}

// PushNotification is a value type holding the content of a push notification.
type PushNotification struct {
	Title string
	Body  string
	// Data is passed as the FCM data payload. The mobile app uses it for deep-linking.
	// Example: {"incident_id": "...", "incident_number": "42", "event": "incident_created"}
	Data map[string]string
}

// PushNotifier is the interface used throughout the codebase for sending push notifications.
type PushNotifier interface {
	IsEnabled() bool
	// SendToUser sends a push notification to all registered devices for the user.
	// Partial failure (some tokens bad, some OK) is not an error — the method always returns nil
	// unless the entire operation is logically impossible.
	SendToUser(ctx context.Context, userID uuid.UUID, n PushNotification) error
}

// PushService sends Firebase Cloud Messaging push notifications.
// A nil *PushService is a valid no-op (push disabled).
type PushService struct {
	sender             FCMSender
	repo               repository.DeviceTokenRepository
	// checkNotRegistered determines whether an FCM error means the token is permanently stale.
	// Defaults to messaging.IsUnregistered. Injectable for unit tests.
	checkNotRegistered func(error) bool
}

// NewPushService initialises the Firebase Admin SDK using the service account at credPath.
// Returns (nil, nil) when credPath is empty — push is silently disabled.
// Returns an error if the file is unreadable, invalid JSON, or the Firebase SDK rejects it.
func NewPushService(credPath string, repo repository.DeviceTokenRepository) (*PushService, error) {
	if credPath == "" {
		return nil, nil
	}

	// Pre-validate: read and parse the credentials file as JSON so we give an immediate,
	// clear error rather than a deferred SDK error.
	data, err := os.ReadFile(credPath)
	if err != nil {
		return nil, fmt.Errorf("push: cannot read credential file: %w", err)
	}
	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("push: credential file is not valid JSON: %w", err)
	}

	opt := option.WithAuthCredentialsJSON(option.ServiceAccount, data)
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		return nil, fmt.Errorf("push: failed to init Firebase app: %w", err)
	}
	client, err := app.Messaging(context.Background())
	if err != nil {
		return nil, fmt.Errorf("push: failed to get messaging client: %w", err)
	}

	return &PushService{
		sender:             client,
		repo:               repo,
		checkNotRegistered: messaging.IsUnregistered,
	}, nil
}

// IsEnabled returns false for a nil *PushService (push disabled).
func (s *PushService) IsEnabled() bool { return s != nil }

// SendToUser sends a push notification to all registered devices for the given user.
// On FCM error the method continues to the next token, always returning nil.
// Stale tokens (IsRegistrationTokenNotRegistered) are deleted immediately.
// The token string is never logged.
func (s *PushService) SendToUser(ctx context.Context, userID uuid.UUID, n PushNotification) error {
	tokens, err := s.repo.GetByUserID(userID)
	if err != nil {
		return fmt.Errorf("push: failed to fetch tokens for user %s: %w", userID, err)
	}
	if len(tokens) == 0 {
		return nil
	}

	checkStale := s.checkNotRegistered
	if checkStale == nil {
		checkStale = messaging.IsUnregistered
	}

	for _, dt := range tokens {
		msg := &messaging.Message{
			// Fid (Firebase Installation ID) is a different identifier than an FCM
			// registration token — dt.Token isn't a drop-in replacement value, so
			// this isn't a one-line migration. Tracked in REG-156.
			Token: dt.Token, //nolint:staticcheck // SA1019: see REG-156
			Notification: &messaging.Notification{
				Title: n.Title,
				Body:  n.Body,
			},
			Data: n.Data,
			Android: &messaging.AndroidConfig{Priority: "high"},
			APNS: &messaging.APNSConfig{
				Payload: &messaging.APNSPayload{
					Aps: &messaging.Aps{Sound: "default"},
				},
			},
		}

		_, sendErr := s.sender.Send(ctx, msg)
		if sendErr == nil {
			// Successful delivery — bump last_seen_at (best-effort; ignore error).
			_ = s.repo.UpdateLastSeen(dt.Token)
			continue
		}

		if checkStale(sendErr) {
			// Token is permanently stale — remove it now.
			if delErr := s.repo.DeleteByToken(dt.Token); delErr != nil {
				slog.Warn("push: failed to delete stale token",
					"user_id", userID, "platform", dt.Platform, "err", delErr)
			} else {
				slog.Info("push: deleted stale FCM token",
					"user_id", userID, "platform", dt.Platform)
			}
			continue
		}

		// Transient error (network, quota) — log and move on.
		slog.Warn("push: FCM send failed (transient)",
			"user_id", userID, "platform", dt.Platform, "err", sendErr)
	}
	return nil
}

// SetPushService wires the optional push notification service into the incident service.
// The push parameter accepts PushNotifier (interface) so tests can inject spies directly.
// Called from routes.go after construction when GOOGLE_APPLICATION_CREDENTIALS is set.
func SetPushService(svc IncidentService, push PushNotifier) {
	if is, ok := svc.(*incidentService); ok {
		is.pushSvc = push
	}
}
