package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/api/handlers"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupGroupingRulesTestDB creates an in-memory SQLite database for testing
func setupGroupingRulesTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// Create table manually (AutoMigrate generates PostgreSQL-specific DDL that SQLite rejects)
	sqlDB, err := db.DB()
	assert.NoError(t, err)
	_, err = sqlDB.Exec(`CREATE TABLE grouping_rules (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT DEFAULT '',
		enabled INTEGER NOT NULL DEFAULT 1,
		priority INTEGER NOT NULL,
		match_labels TEXT NOT NULL DEFAULT '{}',
		time_window_seconds INTEGER NOT NULL DEFAULT 300,
		cross_source_labels TEXT DEFAULT '[]',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	assert.NoError(t, err)

	return db
}

// setupGroupingRulesRouter creates a test router with grouping rules routes
func setupGroupingRulesRouter(t *testing.T, db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	ruleRepo := repository.NewGroupingRuleRepository(db)

	// Register routes
	v1 := router.Group("/api/v1")
	{
		v1.GET("/grouping-rules", handlers.ListGroupingRules(ruleRepo))
		v1.GET("/grouping-rules/:id", handlers.GetGroupingRule(ruleRepo))
		onRuleMutate := func() {} // no-op for tests
		v1.POST("/grouping-rules", handlers.CreateGroupingRule(ruleRepo, onRuleMutate))
		v1.PUT("/grouping-rules/:id", handlers.UpdateGroupingRule(ruleRepo, onRuleMutate))
		v1.DELETE("/grouping-rules/:id", handlers.DeleteGroupingRule(ruleRepo, onRuleMutate))
	}

	return router
}

func TestCreateGroupingRule(t *testing.T) {
	db := setupGroupingRulesTestDB(t)
	router := setupGroupingRulesRouter(t, db)

	// Test case: Create valid grouping rule
	payload := map[string]interface{}{
		"name":        "Test Rule",
		"description": "Test description",
		"enabled":     true,
		"priority":    50,
		"match_labels": map[string]interface{}{
			"severity": "critical",
		},
		"cross_source_labels": []string{"service", "env"},
		"time_window_seconds": 300,
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/grouping-rules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Test Rule", response["name"])
	assert.Equal(t, float64(50), response["priority"])
	assert.NotNil(t, response["id"])
}

func TestCreateGroupingRule_ValidationError(t *testing.T) {
	db := setupGroupingRulesTestDB(t)
	router := setupGroupingRulesRouter(t, db)

	// Test case: Missing required fields
	payload := map[string]interface{}{
		"name": "Test Rule",
		// Missing priority and match_labels
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/grouping-rules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateGroupingRule_PriorityConflict(t *testing.T) {
	db := setupGroupingRulesTestDB(t)
	router := setupGroupingRulesRouter(t, db)

	// Create first rule with priority 50
	payload1 := map[string]interface{}{
		"name":     "Rule 1",
		"priority": 50,
		"match_labels": map[string]interface{}{
			"severity": "critical",
		},
		"time_window_seconds": 300,
	}

	body1, _ := json.Marshal(payload1)
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/grouping-rules", bytes.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusCreated, w1.Code)

	// Try to create second rule with same priority
	payload2 := map[string]interface{}{
		"name":     "Rule 2",
		"priority": 50, // Conflict!
		"match_labels": map[string]interface{}{
			"service": "*",
		},
		"time_window_seconds": 600,
	}

	body2, _ := json.Marshal(payload2)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/grouping-rules", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusConflict, w2.Code)

	var response map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &response)
	errMap := response["error"].(map[string]interface{})
	assert.Contains(t, errMap["message"], "priority already in use")
}

func TestListGroupingRules(t *testing.T) {
	db := setupGroupingRulesTestDB(t)
	router := setupGroupingRulesRouter(t, db)

	// Create a few test rules
	createRule(t, router, "Rule 1", 10, true)
	createRule(t, router, "Rule 2", 20, false)
	createRule(t, router, "Rule 3", 30, true)

	// Test: Get all rules
	req := httptest.NewRequest(http.MethodGet, "/api/v1/grouping-rules", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	data := response["data"].([]interface{})
	assert.Equal(t, 3, len(data))
}

func TestListGroupingRules_EnabledFilter(t *testing.T) {
	db := setupGroupingRulesTestDB(t)
	router := setupGroupingRulesRouter(t, db)

	// Create rules
	createRule(t, router, "Enabled Rule", 10, true)
	createRule(t, router, "Disabled Rule", 20, false)

	// Test: Get enabled rules only
	req := httptest.NewRequest(http.MethodGet, "/api/v1/grouping-rules?enabled=true", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	data := response["data"].([]interface{})
	assert.Equal(t, 1, len(data))
	assert.Equal(t, "Enabled Rule", data[0].(map[string]interface{})["name"])
}

func TestGetGroupingRule(t *testing.T) {
	db := setupGroupingRulesTestDB(t)
	router := setupGroupingRulesRouter(t, db)

	// Create a test rule
	ruleID := createRule(t, router, "Test Rule", 50, true)

	// Test: Get rule by ID
	req := httptest.NewRequest(http.MethodGet, "/api/v1/grouping-rules/"+ruleID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Test Rule", response["name"])
	assert.Equal(t, float64(50), response["priority"])
}

func TestGetGroupingRule_NotFound(t *testing.T) {
	db := setupGroupingRulesTestDB(t)
	router := setupGroupingRulesRouter(t, db)

	// Test: Get non-existent rule
	fakeID := uuid.New().String()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/grouping-rules/"+fakeID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateGroupingRule(t *testing.T) {
	db := setupGroupingRulesTestDB(t)
	router := setupGroupingRulesRouter(t, db)

	// Create a test rule
	ruleID := createRule(t, router, "Original Name", 50, true)

	// Test: Update rule
	payload := map[string]interface{}{
		"name":     "Updated Name",
		"priority": 60,
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/grouping-rules/"+ruleID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Updated Name", response["name"])
	assert.Equal(t, float64(60), response["priority"])
}

func TestUpdateGroupingRule_PriorityConflict(t *testing.T) {
	db := setupGroupingRulesTestDB(t)
	router := setupGroupingRulesRouter(t, db)

	// Create two rules
	rule1ID := createRule(t, router, "Rule 1", 10, true)
	createRule(t, router, "Rule 2", 20, true)

	// Try to update Rule 1 to have Rule 2's priority
	payload := map[string]interface{}{
		"priority": 20, // Conflict with Rule 2
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/grouping-rules/"+rule1ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestDeleteGroupingRule(t *testing.T) {
	db := setupGroupingRulesTestDB(t)
	router := setupGroupingRulesRouter(t, db)

	// Create a test rule
	ruleID := createRule(t, router, "Test Rule", 50, true)

	// Test: Delete rule
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/grouping-rules/"+ruleID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)

	// Verify rule is deleted
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/grouping-rules/"+ruleID, nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusNotFound, w2.Code)
}

func TestDeleteGroupingRule_NotFound(t *testing.T) {
	db := setupGroupingRulesTestDB(t)
	router := setupGroupingRulesRouter(t, db)

	// Test: Delete non-existent rule
	fakeID := uuid.New().String()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/grouping-rules/"+fakeID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// Helper function to create a test grouping rule
func createRule(t *testing.T, router *gin.Engine, name string, priority int, enabled bool) string {
	payload := map[string]interface{}{
		"name":     name,
		"enabled":  enabled,
		"priority": priority,
		"match_labels": map[string]interface{}{
			"alertname": "*",
		},
		"time_window_seconds": 300,
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/grouping-rules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	return response["id"].(string)
}
