package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FluidifyAI/Regen/backend/enterprise"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestAnalyticsRoutes_NoopReturns402(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	hooks := enterprise.NewNoOp()
	grp := r.Group("/api/v1/analytics")
	hooks.Analytics.RegisterRoutes(grp, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/analytics/incidents", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusPaymentRequired, w.Code)
}
