package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── stubs ─────────────────────────────────────────────────────────────────────

type stubNeuriRepo struct {
	created  []*models.NeuriResult
	listResp []models.NeuriResult
	listErr  error
	createErr error
}

func (s *stubNeuriRepo) Create(r *models.NeuriResult) error {
	if s.createErr != nil {
		return s.createErr
	}
	r.ID = uuid.New()
	s.created = append(s.created, r)
	return nil
}

func (s *stubNeuriRepo) ListByIncidentID(_ uuid.UUID) ([]models.NeuriResult, error) {
	return s.listResp, s.listErr
}

type stubIncidentRepoForNeuri struct {
	incident *models.Incident
	err      error
}

func (s *stubIncidentRepoForNeuri) GetByID(_ uuid.UUID) (*models.Incident, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.incident, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// fullIncidentRepoForNeuri adapts stubIncidentRepoForNeuri to the full
// repository.IncidentRepository interface. Embedding the interface satisfies
// the compiler; only GetByID is called in these tests.
type fullIncidentRepoForNeuri struct {
	repository.IncidentRepository
	stub *stubIncidentRepoForNeuri
}

func (f *fullIncidentRepoForNeuri) GetByID(id uuid.UUID) (*models.Incident, error) {
	return f.stub.GetByID(id)
}

func neuriRouter(incRepo repository.IncidentRepository, neuriRepo repository.NeuriResultRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/neuri/result", ReceiveNeuriResult(incRepo, neuriRepo))
	r.GET("/api/v1/neuri/result", ListNeuriResults(neuriRepo))
	return r
}

func postJSONNeuri(r *gin.Engine, path string, body interface{}) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func validPayload(incidentID string) map[string]interface{} {
	return map[string]interface{}{
		"incident_id":          incidentID,
		"investigation_run_id": uuid.New().String(),
		"top_hypothesis":       "CODE_CHANGE",
		"confidence":           0.85,
		"summary":              "A deploy correlates with the spike.",
		"ranked_hypotheses":    []interface{}{},
	}
}

// ── ReceiveNeuriResult tests ──────────────────────────────────────────────────

func TestReceiveNeuriResult_HappyPath(t *testing.T) {
	incidentID := uuid.New()
	incRepo := &fullIncidentRepoForNeuri{stub: &stubIncidentRepoForNeuri{
		incident: &models.Incident{ID: incidentID},
	}}
	neuriRepo := &stubNeuriRepo{}
	r := neuriRouter(incRepo, neuriRepo)

	w := postJSONNeuri(r, "/api/v1/neuri/result", validPayload(incidentID.String()))

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, incidentID.String(), resp["incident_id"])
	assert.NotEmpty(t, resp["id"])
	assert.Len(t, neuriRepo.created, 1)
	assert.Equal(t, "CODE_CHANGE", neuriRepo.created[0].TopHypothesis)
	assert.InDelta(t, 0.85, neuriRepo.created[0].Confidence, 0.001)
}

func TestReceiveNeuriResult_MalformedJSON(t *testing.T) {
	r := neuriRouter(&fullIncidentRepoForNeuri{stub: &stubIncidentRepoForNeuri{}}, &stubNeuriRepo{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/neuri/result", bytes.NewBufferString("{invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestReceiveNeuriResult_MissingRequiredFields(t *testing.T) {
	r := neuriRouter(&fullIncidentRepoForNeuri{stub: &stubIncidentRepoForNeuri{}}, &stubNeuriRepo{})
	// confidence and summary omitted
	w := postJSONNeuri(r, "/api/v1/neuri/result", map[string]interface{}{
		"incident_id":          uuid.New().String(),
		"investigation_run_id": uuid.New().String(),
		"top_hypothesis":       "CODE_CHANGE",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestReceiveNeuriResult_InvalidIncidentUUID(t *testing.T) {
	r := neuriRouter(&fullIncidentRepoForNeuri{stub: &stubIncidentRepoForNeuri{}}, &stubNeuriRepo{})
	payload := validPayload("not-a-uuid")
	w := postJSONNeuri(r, "/api/v1/neuri/result", payload)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestReceiveNeuriResult_IncidentNotFound(t *testing.T) {
	incRepo := &fullIncidentRepoForNeuri{stub: &stubIncidentRepoForNeuri{
		err: &repository.NotFoundError{Resource: "incident", ID: "some-id"},
	}}
	r := neuriRouter(incRepo, &stubNeuriRepo{})
	w := postJSONNeuri(r, "/api/v1/neuri/result", validPayload(uuid.New().String()))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ── ListNeuriResults tests ────────────────────────────────────────────────────

func TestListNeuriResults_HappyPath(t *testing.T) {
	incidentID := uuid.New()
	neuriRepo := &stubNeuriRepo{
		listResp: []models.NeuriResult{
			{
				ID:            uuid.New(),
				IncidentID:    incidentID,
				TopHypothesis: "CODE_CHANGE",
				Confidence:    0.85,
				Summary:       "Deploy correlates.",
				RankedHypotheses: models.RawJSON("[]"),
			},
		},
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/neuri/result", ListNeuriResults(neuriRepo))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/neuri/result?incident_id="+incidentID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	results := resp["results"].([]interface{})
	assert.Len(t, results, 1)
}

func TestListNeuriResults_MissingIncidentID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/neuri/result", ListNeuriResults(&stubNeuriRepo{}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/neuri/result", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListNeuriResults_InvalidUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/neuri/result", ListNeuriResults(&stubNeuriRepo{}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/neuri/result?incident_id=not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
