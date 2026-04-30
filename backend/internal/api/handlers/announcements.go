package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetAnnouncements handles GET /api/v1/announcements.
// It serves the cached announcement JSON fetched by the telemetry worker from
// api.fluidify.ai/regen/announcements. Returns an empty list when nothing is cached.
func GetAnnouncements(getCached func() []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json", getCached())
	}
}
