package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── fakePushTokenRepo ────────────────────────────────────────────────────────

type fakePushTokenRepo struct {
	upsertErr   error
	deleteErr   error
	returnFalse bool // DeleteByUserAndToken returns (false, nil)

	upsertCalled int
	deleteCalled int
}

func (f *fakePushTokenRepo) Upsert(_ uuid.UUID, _, _, _ string) error {
	f.upsertCalled++
	return f.upsertErr
}
func (f *fakePushTokenRepo) GetByUserID(_ uuid.UUID) ([]models.DeviceToken, error) {
	return nil, nil
}
func (f *fakePushTokenRepo) DeleteByUserAndToken(_ uuid.UUID, _ string) (bool, error) {
	f.deleteCalled++
	return !f.returnFalse, f.deleteErr
}
func (f *fakePushTokenRepo) DeleteByToken(_ string) error                     { return nil }
func (f *fakePushTokenRepo) UpdateLastSeen(_ string) error                    { return nil }
func (f *fakePushTokenRepo) DeleteStale(_ time.Time) (int64, error)           { return 0, nil }
func (f *fakePushTokenRepo) CountByUserID(_ uuid.UUID) (int64, error)         { return 0, nil }

// Compile-time check.
var _ repository.DeviceTokenRepository = (*fakePushTokenRepo)(nil)

// ─── injectUser sets the local_user context key so handlers can call GetLocalUser ───

func injectUser(c *gin.Context, user models.User) {
	c.Set("local_user", &user)
}

// ─── Router helpers ───────────────────────────────────────────────────────────

func setupPushRouter(repo repository.DeviceTokenRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Route with user injected via middleware stub
	r.POST("/push/register", func(c *gin.Context) {
		injectUser(c, models.User{ID: uuid.New(), Email: "test@example.com"})
		RegisterDeviceToken(repo)(c)
	})
	r.POST("/push/unregister", func(c *gin.Context) {
		injectUser(c, models.User{ID: uuid.New(), Email: "test@example.com"})
		UnregisterDeviceToken(repo)(c)
	})
	return r
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestRegisterDeviceToken_ValidBody_Returns204(t *testing.T) {
	repo := &fakePushTokenRepo{}
	r := setupPushRouter(repo)

	body := `{"token":"fcm-abc-123","platform":"ios","app_version":"1.0.0"}`
	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/push/register", bytes.NewBufferString(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, 1, repo.upsertCalled)
}

func TestRegisterDeviceToken_EmptyToken_Returns422(t *testing.T) {
	repo := &fakePushTokenRepo{}
	r := setupPushRouter(repo)

	body := `{"token":"","platform":"ios"}`
	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/push/register", bytes.NewBufferString(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Equal(t, 0, repo.upsertCalled)
}

func TestRegisterDeviceToken_UnknownPlatform_Returns422(t *testing.T) {
	repo := &fakePushTokenRepo{}
	r := setupPushRouter(repo)

	body := `{"token":"tok-abc","platform":"windows"}`
	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/push/register", bytes.NewBufferString(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Equal(t, 0, repo.upsertCalled)
}

func TestRegisterDeviceToken_TokenTooLong_Returns422(t *testing.T) {
	repo := &fakePushTokenRepo{}
	r := setupPushRouter(repo)

	// Token of 4097 bytes
	longToken := strings.Repeat("x", 4097)
	body, _ := json.Marshal(map[string]string{"token": longToken, "platform": "android"})
	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/push/register", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Equal(t, 0, repo.upsertCalled)
}

func TestRegisterDeviceToken_LimitExceeded_Returns429(t *testing.T) {
	repo := &fakePushTokenRepo{upsertErr: repository.ErrTokenLimitExceeded}
	r := setupPushRouter(repo)

	body := `{"token":"tok-abc","platform":"ios"}`
	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/push/register", bytes.NewBufferString(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "token_limit_exceeded", resp["error"])
}

func TestUnregisterDeviceToken_ValidToken_Returns204(t *testing.T) {
	repo := &fakePushTokenRepo{}
	r := setupPushRouter(repo)

	body := `{"token":"fcm-tok-to-remove"}`
	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/push/unregister", bytes.NewBufferString(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, 1, repo.deleteCalled)
}

func TestUnregisterDeviceToken_TokenNotFound_Returns204(t *testing.T) {
	repo := &fakePushTokenRepo{returnFalse: true}
	r := setupPushRouter(repo)

	body := `{"token":"fcm-tok-not-found"}`
	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/push/unregister", bytes.NewBufferString(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	r.ServeHTTP(w, req)

	// Idempotent — 204 even if row didn't exist
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, 1, repo.deleteCalled)
}

func TestUnregisterDeviceToken_EmptyToken_Returns422(t *testing.T) {
	repo := &fakePushTokenRepo{}
	r := setupPushRouter(repo)

	body := `{"token":""}`
	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/push/unregister", bytes.NewBufferString(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Equal(t, 0, repo.deleteCalled)
}
