package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/openincident/openincident/internal/api/middleware"
	"github.com/openincident/openincident/internal/config"
)

// SetupRouter initializes the Gin router with middleware and routes
func SetupRouter(cfg *config.Config) *gin.Engine {
	// Set Gin mode based on environment
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	// Create router
	router := gin.New()

	// Add middleware
	router.Use(middleware.Recovery())
	router.Use(middleware.Logger())
	router.Use(middleware.CORS())

	// Health check endpoints
	router.GET("/health", healthHandler)
	router.GET("/ready", readyHandler(cfg))

	// API v1 routes group
	v1 := router.Group("/api/v1")
	{
		// Webhooks will be added in Epic 002
		webhooks := v1.Group("/webhooks")
		{
			webhooks.POST("/prometheus", func(c *gin.Context) {
				c.JSON(http.StatusNotImplemented, gin.H{"message": "Coming in Epic 002"})
			})
		}

		// Incidents will be added in Epic 003
		incidents := v1.Group("/incidents")
		{
			incidents.GET("", func(c *gin.Context) {
				c.JSON(http.StatusNotImplemented, gin.H{"message": "Coming in Epic 003"})
			})
		}

		// Alerts will be added in Epic 002
		alerts := v1.Group("/alerts")
		{
			alerts.GET("", func(c *gin.Context) {
				c.JSON(http.StatusNotImplemented, gin.H{"message": "Coming in Epic 002"})
			})
		}
	}

	return router
}

// healthHandler returns basic health status
func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

// readyHandler returns readiness status with dependency checks
func readyHandler(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Add actual database and redis health checks in Epic 002
		c.JSON(http.StatusOK, gin.H{
			"status":   "ready",
			"database": "ok",
			"redis":    "ok",
		})
	}
}
