package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/api/handlers"
	"github.com/FluidifyAI/Regen/backend/internal/database"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupCustomFieldsRouter(t *testing.T) (*gin.Engine, repository.CustomFieldRepository) {
	db := database.SetupTestDB(t)
	repo := repository.NewCustomFieldRepository(db)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.GET("/custom-fields", handlers.ListCustomFields(repo))
	v1.POST("/custom-fields", handlers.CreateCustomField(repo))
	v1.PUT("/custom-fields/:id", handlers.UpdateCustomField(repo))
	v1.DELETE("/custom-fields/:id", handlers.DeleteCustomField(repo))
	v1.PATCH("/custom-fields/reorder", handlers.ReorderCustomFields(repo))

	return r, repo
}

func TestCustomFields_List_Empty(t *testing.T) {
	r, _ := setupCustomFieldsRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/custom-fields", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp []interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Empty(t, resp)
}

func TestCustomFields_Create_Valid(t *testing.T) {
	r, _ := setupCustomFieldsRouter(t)

	body := `{"name":"Affected Service","key":"affected_service","field_type":"string"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/custom-fields", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "affected_service", resp["key"])
	assert.NotEmpty(t, resp["id"])
}

func TestCustomFields_Create_InvalidKeyFormat(t *testing.T) {
	r, _ := setupCustomFieldsRouter(t)

	body := `{"name":"Bad Key","key":"Bad-Key","field_type":"string"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/custom-fields", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCustomFields_Create_InvalidType(t *testing.T) {
	r, _ := setupCustomFieldsRouter(t)

	body := `{"name":"Field","key":"my_field","field_type":"boolean"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/custom-fields", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCustomFields_Create_DropdownWithoutOptions(t *testing.T) {
	r, _ := setupCustomFieldsRouter(t)

	body := `{"name":"Priority","key":"priority","field_type":"dropdown","options":[]}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/custom-fields", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCustomFields_Create_DropdownWithOptions(t *testing.T) {
	r, _ := setupCustomFieldsRouter(t)

	body := `{"name":"Priority","key":"priority","field_type":"dropdown","options":[{"label":"High","value":"high"}]}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/custom-fields", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCustomFields_Create_DuplicateKey(t *testing.T) {
	r, _ := setupCustomFieldsRouter(t)

	body := `{"name":"Service","key":"service","field_type":"string"}`
	for range 2 {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/custom-fields", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
	}

	// Second create must return conflict
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/custom-fields", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestCustomFields_Update(t *testing.T) {
	r, repo := setupCustomFieldsRouter(t)

	// Create first
	body := `{"name":"Team","key":"team","field_type":"string"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/custom-fields", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	fields, _ := repo.List()
	require.Len(t, fields, 1)
	id := fields[0].ID.String()

	// Update
	update := `{"name":"Team (updated)","key":"team","field_type":"string"}`
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodPut, "/api/v1/custom-fields/"+id, bytes.NewBufferString(update))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp))
	assert.Equal(t, "Team (updated)", resp["name"])
}

func TestCustomFields_Delete_NoUsage(t *testing.T) {
	r, repo := setupCustomFieldsRouter(t)

	body := `{"name":"Region","key":"region","field_type":"string"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/custom-fields", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	fields, _ := repo.List()
	id := fields[0].ID.String()

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodDelete, "/api/v1/custom-fields/"+id, nil)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusNoContent, w2.Code)
}

func TestCustomFields_Reorder(t *testing.T) {
	r, repo := setupCustomFieldsRouter(t)

	// Create two fields
	for _, b := range []string{
		`{"name":"Alpha","key":"alpha","field_type":"string"}`,
		`{"name":"Beta","key":"beta","field_type":"string"}`,
	} {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/custom-fields", bytes.NewBufferString(b))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusCreated, w.Code)
	}

	fields, _ := repo.List()
	require.Len(t, fields, 2)

	reorderBody, _ := json.Marshal([]map[string]interface{}{
		{"id": fields[0].ID.String(), "order": 10},
		{"id": fields[1].ID.String(), "order": 1},
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPatch, "/api/v1/custom-fields/reorder", bytes.NewBuffer(reorderBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
