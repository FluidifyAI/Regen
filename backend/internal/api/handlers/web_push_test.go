package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/api/handlers"
	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// --- full stub implementing repository.WebPushSubscriptionRepository ---

type stubWebPushRepo struct {
	upsertErr error
}

func (s *stubWebPushRepo) Upsert(_ uuid.UUID, _, _, _ string) error { return s.upsertErr }
func (s *stubWebPushRepo) GetByUserID(_ uuid.UUID) ([]models.WebPushSubscription, error) {
	return nil, nil
}
func (s *stubWebPushRepo) DeleteByUserAndEndpoint(_ uuid.UUID, _ string) (bool, error) {
	return true, nil
}
func (s *stubWebPushRepo) DeleteByEndpoint(_ string) error           { return nil }
func (s *stubWebPushRepo) UpdateLastSeen(_ string) error             { return nil }
func (s *stubWebPushRepo) DeleteStale(_ time.Time) (int64, error)    { return 0, nil }
func (s *stubWebPushRepo) CountByUserID(_ uuid.UUID) (int64, error)  { return 0, nil }

// injectWebPushUser sets the local_user context key so handlers can call GetLocalUser.
func injectWebPushUser(c *gin.Context) {
	c.Set("local_user", &models.User{ID: uuid.New(), Email: "test@example.com"})
}

// --- GetVAPIDPublicKey ---

func TestGetVAPIDPublicKey_ReturnsKey(t *testing.T) {
	r := gin.New()
	r.GET("/vapid", injectWebPushUser, handlers.GetVAPIDPublicKey("test-public-key"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/vapid", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["public_key"] != "test-public-key" {
		t.Errorf("expected public_key=test-public-key, got %q", resp["public_key"])
	}
}

func TestGetVAPIDPublicKey_NotConfigured_Returns503(t *testing.T) {
	r := gin.New()
	r.GET("/vapid", injectWebPushUser, handlers.GetVAPIDPublicKey(""))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/vapid", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when not configured, got %d", w.Code)
	}
}

// --- RegisterWebPushSubscription ---

func registerWebPushRouter(repo repository.WebPushSubscriptionRepository) *gin.Engine {
	r := gin.New()
	r.POST("/register", injectWebPushUser, handlers.RegisterWebPushSubscription(repo))
	return r
}

func TestRegisterWebPushSubscription_Valid_Returns204(t *testing.T) {
	r := registerWebPushRouter(&stubWebPushRepo{})
	body := `{"endpoint":"https://push.example.com/ep","keys":{"p256dh":"abc","auth":"def"}}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRegisterWebPushSubscription_MissingEndpoint_Returns422(t *testing.T) {
	r := registerWebPushRouter(&stubWebPushRepo{})
	body := `{"keys":{"p256dh":"abc","auth":"def"}}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for missing endpoint, got %d", w.Code)
	}
}

func TestRegisterWebPushSubscription_HTTPEndpoint_Returns422(t *testing.T) {
	r := registerWebPushRouter(&stubWebPushRepo{})
	body := `{"endpoint":"http://insecure.example.com/ep","keys":{"p256dh":"abc","auth":"def"}}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for http:// endpoint, got %d", w.Code)
	}
}

func TestRegisterWebPushSubscription_EndpointTooLong_Returns422(t *testing.T) {
	r := registerWebPushRouter(&stubWebPushRepo{})
	longEP := "https://push.example.com/" + strings.Repeat("a", 2048)
	body, _ := json.Marshal(map[string]any{
		"endpoint": longEP,
		"keys":     map[string]string{"p256dh": "abc", "auth": "def"},
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for overlong endpoint, got %d", w.Code)
	}
}

func TestRegisterWebPushSubscription_MissingP256DH_Returns422(t *testing.T) {
	r := registerWebPushRouter(&stubWebPushRepo{})
	body := `{"endpoint":"https://push.example.com/ep","keys":{"auth":"def"}}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for missing p256dh, got %d", w.Code)
	}
}

func TestRegisterWebPushSubscription_MissingAuth_Returns422(t *testing.T) {
	r := registerWebPushRouter(&stubWebPushRepo{})
	body := `{"endpoint":"https://push.example.com/ep","keys":{"p256dh":"abc"}}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for missing auth, got %d", w.Code)
	}
}

func TestRegisterWebPushSubscription_CapExceeded_Returns429(t *testing.T) {
	r := registerWebPushRouter(&stubWebPushRepo{upsertErr: repository.ErrTokenLimitExceeded})
	body := `{"endpoint":"https://push.example.com/ep","keys":{"p256dh":"abc","auth":"def"}}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 when cap exceeded, got %d", w.Code)
	}
}

// --- UnregisterWebPushSubscription ---

func unregisterWebPushRouter(repo repository.WebPushSubscriptionRepository) *gin.Engine {
	r := gin.New()
	r.POST("/unregister", injectWebPushUser, handlers.UnregisterWebPushSubscription(repo))
	return r
}

func TestUnregisterWebPushSubscription_Valid_Returns204(t *testing.T) {
	r := unregisterWebPushRouter(&stubWebPushRepo{})
	body := `{"endpoint":"https://push.example.com/ep"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/unregister", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUnregisterWebPushSubscription_MissingEndpoint_Returns422(t *testing.T) {
	r := unregisterWebPushRouter(&stubWebPushRepo{})
	body := `{}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/unregister", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for missing endpoint, got %d", w.Code)
	}
}

func TestUnregisterWebPushSubscription_NotFound_Returns204(t *testing.T) {
	r := unregisterWebPushRouter(&stubWebPushRepo{})
	body := `{"endpoint":"https://push.example.com/nonexistent"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/unregister", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204 (idempotent), got %d", w.Code)
	}
}
