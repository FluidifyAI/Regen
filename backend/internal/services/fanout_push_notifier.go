package services

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
)

// fanoutPushNotifier delegates SendToUser to all enabled sub-notifiers.
type fanoutPushNotifier struct {
	notifiers []PushNotifier
}

// NewFanoutPushNotifier builds a PushNotifier that fans out to all provided notifiers that
// are non-nil and report IsEnabled()==true. Returns nil when no active notifiers remain,
// so callers can pass the result directly to SetPushService (which handles nil gracefully).
func NewFanoutPushNotifier(notifiers ...PushNotifier) PushNotifier {
	active := make([]PushNotifier, 0, len(notifiers))
	for _, n := range notifiers {
		if n != nil && n.IsEnabled() {
			active = append(active, n)
		}
	}
	if len(active) == 0 {
		return nil
	}
	return &fanoutPushNotifier{notifiers: active}
}

func (f *fanoutPushNotifier) IsEnabled() bool { return len(f.notifiers) > 0 }

// SendToUser calls every active sub-notifier. Errors are logged but never propagated;
// all sub-notifiers are always attempted regardless of earlier failures.
func (f *fanoutPushNotifier) SendToUser(ctx context.Context, userID uuid.UUID, n PushNotification) error {
	for _, notifier := range f.notifiers {
		if err := notifier.SendToUser(ctx, userID, n); err != nil {
			slog.Warn("fanout push: sub-notifier error", "user_id", userID, "err", err)
		}
	}
	return nil
}
