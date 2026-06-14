package enterprise_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FluidifyAI/Regen/backend/enterprise"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestNoopAnalytics_Returns402(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hooks := enterprise.NewNoOp()

	r := gin.New()
	grp := r.Group("/analytics")
	hooks.Analytics.RegisterRoutes(grp, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/analytics/incidents", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusPaymentRequired, w.Code)
}

func TestNoopAnalytics_HooksFieldExists(t *testing.T) {
	hooks := enterprise.NewNoOp()
	assert.NotNil(t, hooks.Analytics)
}
