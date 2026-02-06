package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/openincident/openincident/internal/api/handlers"
	"github.com/openincident/openincident/internal/api/middleware"
	"gorm.io/gorm"
)

// SetupRoutes configures all application routes
func SetupRoutes(router *gin.Engine, db *gorm.DB) {
	// Middleware
	router.Use(middleware.CORS())
	router.Use(middleware.Recovery())
	router.Use(middleware.Logger())

	// Health check endpoints
	router.GET("/health", handlers.Health(db))
	router.GET("/ready", handlers.Ready(db))

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Webhooks (to be implemented in future tasks)
		webhooks := v1.Group("/webhooks")
		{
			webhooks.POST("/prometheus", func(c *gin.Context) {
				c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
			})
			webhooks.POST("/grafana", func(c *gin.Context) {
				c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
			})
		}

		// Incidents (to be implemented)
		v1.GET("/incidents", func(c *gin.Context) {
			c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
		})

		// Alerts (to be implemented)
		v1.GET("/alerts", func(c *gin.Context) {
			c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
		})
	}
}
