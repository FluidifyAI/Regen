package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/google/uuid"
)

// webPushSender is the minimal repository surface WebPushService needs.
// Using a narrow interface keeps the service testable without a real database.
type webPushSender interface {
	GetByUserID(userID uuid.UUID) ([]models.WebPushSubscription, error)
	UpdateLastSeen(endpoint string) error
	DeleteByEndpoint(endpoint string) error
}

// WebPushService sends Web Push notifications using VAPID signing.
// A nil *WebPushService is a valid no-op (web push disabled).
type WebPushService struct {
	vapidPublicKey  string
	vapidPrivateKey string
	vapidSubscriber string
	repo            webPushSender
	httpClient      *http.Client
}

// NewWebPushService constructs a WebPushService from VAPID key pair and repository.
//
// Returns (nil, nil) when both keys are empty — web push is silently disabled,
// consistent with how Firebase credentials are handled.
// Returns an error when exactly one key is provided (misconfiguration).
// subscriber is a mailto: or https: URI included in the VAPID JWT.
func NewWebPushService(vapidPublicKey, vapidPrivateKey, subscriber string, repo webPushSender) (*WebPushService, error) {
	pubEmpty := vapidPublicKey == ""
	privEmpty := vapidPrivateKey == ""

	if pubEmpty && privEmpty {
		return nil, nil
	}
	if pubEmpty != privEmpty {
		return nil, fmt.Errorf("web push: VAPID_PUBLIC_KEY and VAPID_PRIVATE_KEY must both be set or both be empty")
	}

	sub := subscriber
	if sub == "" {
		sub = "mailto:admin@regen.local"
	}

	return &WebPushService{
		vapidPublicKey:  vapidPublicKey,
		vapidPrivateKey: vapidPrivateKey,
		vapidSubscriber: sub,
		repo:            repo,
		httpClient:      &http.Client{},
	}, nil
}

// IsEnabled returns false for a nil *WebPushService (web push disabled).
func (s *WebPushService) IsEnabled() bool { return s != nil }

// SetHTTPClient replaces the default HTTP client. Used in tests to inject a mock transport.
func (s *WebPushService) SetHTTPClient(c *http.Client) { s.httpClient = c }

// SendToUser sends a web push notification to all registered subscriptions for the given user.
// HTTP 410 Gone causes the subscription to be deleted permanently.
// All other HTTP errors are logged and skipped — they are not propagated.
// The endpoint string is never included in log output.
func (s *WebPushService) SendToUser(ctx context.Context, userID uuid.UUID, n PushNotification) error {
	subs, err := s.repo.GetByUserID(userID)
	if err != nil {
		return fmt.Errorf("web push: failed to fetch subscriptions for user %s: %w", userID, err)
	}
	if len(subs) == 0 {
		return nil
	}

	payload, err := json.Marshal(map[string]any{
		"title": n.Title,
		"body":  n.Body,
		"data":  n.Data,
	})
	if err != nil {
		return fmt.Errorf("web push: failed to marshal payload: %w", err)
	}

	for _, sub := range subs {
		resp, sendErr := webpush.SendNotificationWithContext(ctx, payload,
			&webpush.Subscription{
				Endpoint: sub.Endpoint,
				Keys: webpush.Keys{
					P256dh: sub.P256DH,
					Auth:   sub.Auth,
				},
			},
			&webpush.Options{
				HTTPClient:      s.httpClient,
				VAPIDPublicKey:  s.vapidPublicKey,
				VAPIDPrivateKey: s.vapidPrivateKey,
				Subscriber:      s.vapidSubscriber,
				TTL:             86400,
			},
		)

		if sendErr != nil {
			slog.Warn("web push: send failed (network)", "user_id", userID, "err", sendErr)
			continue
		}
		resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusCreated, http.StatusOK:
			_ = s.repo.UpdateLastSeen(sub.Endpoint)
		case http.StatusGone:
			if delErr := s.repo.DeleteByEndpoint(sub.Endpoint); delErr != nil {
				slog.Warn("web push: failed to delete expired subscription",
					"user_id", userID, "err", delErr)
			} else {
				slog.Info("web push: deleted expired subscription", "user_id", userID)
			}
		default:
			slog.Warn("web push: send failed (http)", "user_id", userID, "status", resp.StatusCode)
		}
	}
	return nil
}
