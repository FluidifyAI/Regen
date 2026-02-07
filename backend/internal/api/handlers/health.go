package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/openincident/openincident/internal/database"
	"github.com/openincident/openincident/internal/redis"
	"gorm.io/gorm"
)

// Health returns a simple health check
func Health(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	}
}

// Ready checks if the application is ready to serve requests
func Ready(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := "ready"
		dbStatus := "ok"
		redisStatus := "ok"

		// Check database health
		if err := database.Health(); err != nil {
			dbStatus = "error"
			status = "not ready"
		}

		// Check Redis health
		if err := redis.Health(); err != nil {
			redisStatus = "error"
			status = "not ready"
		}

		code := http.StatusOK
		if status != "ready" {
			code = http.StatusServiceUnavailable
		}

		c.JSON(code, gin.H{
			"status":   status,
			"database": dbStatus,
			"redis":    redisStatus,
		})
	}
}
