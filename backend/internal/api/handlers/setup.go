package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/fluidify/regen/internal/coordinator"
	"github.com/fluidify/regen/internal/repository"
)

// SeedDemoData handles POST /api/v1/setup/demo-data.
// Admin-only. Creates a sample schedule, escalation policy, routing rule, and
// a pre-resolved incident so new installs feel populated out of the box.
// Returns 409 if demo data (or any real incident with number=1) already exists.
func SeedDemoData(
	scheduleRepo repository.ScheduleRepository,
	escalationRepo repository.EscalationPolicyRepository,
	routingRepo repository.RoutingRuleRepository,
	incidentRepo repository.IncidentRepository,
	timelineRepo repository.TimelineRepository,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		exists, err := coordinator.DemoDataExists(incidentRepo)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "failed to check existing data"}})
			return
		}
		if exists {
			c.JSON(http.StatusConflict, gin.H{"error": gin.H{"message": "data already exists — demo data can only be loaded on a fresh install"}})
			return
		}

		if err := coordinator.SeedDemoData(scheduleRepo, escalationRepo, routingRepo, incidentRepo, timelineRepo); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "failed to seed demo data: " + err.Error()}})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"ok": true, "message": "Sample data loaded. Explore your first incident, on-call schedule, and escalation policy."})
	}
}

// GetSetupStatus handles GET /api/v1/setup/status.
// Returns whether demo data can still be loaded (i.e. no incidents exist yet).
func GetSetupStatus(incidentRepo repository.IncidentRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		exists, err := coordinator.DemoDataExists(incidentRepo)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "failed to check status"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"demo_data_available": !exists})
	}
}
