package services_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/services"
	"github.com/google/uuid"
)

// --- transport helpers ---

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func okTransport() http.RoundTripper {
	return roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(bytes.NewReader(nil)),
		}, nil
	})
}

func goneTransport() http.RoundTripper {
	return roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusGone,
			Body:       io.NopCloser(bytes.NewReader(nil)),
		}, nil
	})
}

func badTransport() http.RoundTripper {
	return roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       io.NopCloser(bytes.NewReader(nil)),
		}, nil
	})
}

// --- fake repository ---

type fakeWebPushSender struct {
	subs             []models.WebPushSubscription
	updateLastSeenEP string
	deletedEP        string
}

func (f *fakeWebPushSender) GetByUserID(userID uuid.UUID) ([]models.WebPushSubscription, error) {
	var out []models.WebPushSubscription
	for _, s := range f.subs {
		if s.UserID == userID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeWebPushSender) UpdateLastSeen(endpoint string) error {
	f.updateLastSeenEP = endpoint
	return nil
}

func (f *fakeWebPushSender) DeleteByEndpoint(endpoint string) error {
	f.deletedEP = endpoint
	return nil
}

// --- test VAPID keys (NOT production keys — safe to embed in tests) ---
const (
	testVAPIDPublic  = "BEl62iUYgUivxIkv69yViEuiBIa-Ib9-SkvMeAtA3LFgDzkrxZJjSgSnfckjBJuBkr3qBUYIHBQFLXYp5Nksh8U"
	testVAPIDPrivate = "UUxI4O8-HoLU33muFischU5shJ-sSymeJEqktzl-KPY"
)

// --- tests ---

func TestNewWebPushService_BothKeysEmpty_ReturnsNil(t *testing.T) {
	svc, err := services.NewWebPushService("", "", "", nil)
	if err != nil {
		t.Fatalf("expected nil error when both keys empty, got: %v", err)
	}
	if svc != nil {
		t.Error("expected nil service when both keys empty")
	}
}

func TestNewWebPushService_OnlyPublicKey_ReturnsError(t *testing.T) {
	_, err := services.NewWebPushService(testVAPIDPublic, "", "", nil)
	if err == nil {
		t.Fatal("expected error when only public key provided")
	}
}

func TestNewWebPushService_OnlyPrivateKey_ReturnsError(t *testing.T) {
	_, err := services.NewWebPushService("", testVAPIDPrivate, "", nil)
	if err == nil {
		t.Fatal("expected error when only private key provided")
	}
}

func TestNewWebPushService_BothKeysProvided_IsEnabled(t *testing.T) {
	svc, err := services.NewWebPushService(testVAPIDPublic, testVAPIDPrivate, "mailto:t@t.com", &fakeWebPushSender{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if !svc.IsEnabled() {
		t.Error("expected IsEnabled()=true")
	}
}

func TestWebPushService_IsEnabled_NilService(t *testing.T) {
	var svc *services.WebPushService
	if svc.IsEnabled() {
		t.Error("nil WebPushService should not be enabled")
	}
}

func TestWebPushService_SendToUser_NoSubscriptions_ReturnsNil(t *testing.T) {
	svc, _ := services.NewWebPushService(testVAPIDPublic, testVAPIDPrivate, "mailto:t@t.com", &fakeWebPushSender{})

	err := svc.SendToUser(context.Background(), uuid.New(), services.PushNotification{
		Title: "Test", Body: "Body",
	})
	if err != nil {
		t.Errorf("expected nil error with no subscriptions, got: %v", err)
	}
}

func TestWebPushService_SendToUser_SuccessUpdatesLastSeen(t *testing.T) {
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	ep := "https://push.example.com/sub/1"
	repo := &fakeWebPushSender{
		subs: []models.WebPushSubscription{{
			UserID:   userID,
			Endpoint: ep,
			P256DH:   "BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4",
			Auth:     "BTBZMqHH6r4Tts7J_aSIgg",
		}},
	}
	svc, _ := services.NewWebPushService(testVAPIDPublic, testVAPIDPrivate, "mailto:t@t.com", repo)
	svc.SetHTTPClient(&http.Client{Transport: okTransport()})

	err := svc.SendToUser(context.Background(), userID, services.PushNotification{
		Title: "Incident", Body: "P1 fired",
		Data: map[string]string{"incident_id": "abc"},
	})
	if err != nil {
		t.Fatalf("SendToUser: %v", err)
	}
	if repo.updateLastSeenEP != ep {
		t.Errorf("expected UpdateLastSeen(%q), got %q", ep, repo.updateLastSeenEP)
	}
}

func TestWebPushService_SendToUser_HTTP410_DeletesSubscription(t *testing.T) {
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	ep := "https://push.example.com/sub/stale"
	repo := &fakeWebPushSender{
		subs: []models.WebPushSubscription{{
			UserID:   userID,
			Endpoint: ep,
			P256DH:   "BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4",
			Auth:     "BTBZMqHH6r4Tts7J_aSIgg",
		}},
	}
	svc, _ := services.NewWebPushService(testVAPIDPublic, testVAPIDPrivate, "mailto:t@t.com", repo)
	svc.SetHTTPClient(&http.Client{Transport: goneTransport()})

	err := svc.SendToUser(context.Background(), userID, services.PushNotification{Title: "T", Body: "B"})
	if err != nil {
		t.Fatalf("SendToUser should not propagate error on 410: %v", err)
	}
	if repo.deletedEP != ep {
		t.Errorf("expected DeleteByEndpoint(%q), got %q", ep, repo.deletedEP)
	}
	if repo.updateLastSeenEP != "" {
		t.Errorf("UpdateLastSeen should not be called on 410, got %q", repo.updateLastSeenEP)
	}
}

func TestWebPushService_SendToUser_TransientError_ContinuesWithoutError(t *testing.T) {
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	repo := &fakeWebPushSender{
		subs: []models.WebPushSubscription{{
			UserID:   userID,
			Endpoint: "https://push.example.com/sub/rate-limited",
			P256DH:   "BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4",
			Auth:     "BTBZMqHH6r4Tts7J_aSIgg",
		}},
	}
	svc, _ := services.NewWebPushService(testVAPIDPublic, testVAPIDPrivate, "mailto:t@t.com", repo)
	svc.SetHTTPClient(&http.Client{Transport: badTransport()})

	err := svc.SendToUser(context.Background(), userID, services.PushNotification{Title: "T", Body: "B"})
	if err != nil {
		t.Fatalf("transient errors should not propagate: %v", err)
	}
	if repo.updateLastSeenEP != "" || repo.deletedEP != "" {
		t.Error("neither UpdateLastSeen nor DeleteByEndpoint should be called on transient error")
	}
}
