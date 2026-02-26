package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/api/handlers/dto"
	"github.com/openincident/openincident/internal/models"
	"github.com/openincident/openincident/internal/services"
)

// ListUsers handles GET /api/v1/settings/users
func ListUsers(localAuth services.LocalAuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		users, err := localAuth.ListUsers()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "failed to list users"}})
			return
		}
		resp := make([]dto.UserResponse, len(users))
		for i, u := range users {
			resp[i] = dto.UserToResponse(u)
		}
		c.JSON(http.StatusOK, gin.H{"users": resp})
	}
}

// CreateUser handles POST /api/v1/settings/users
func CreateUser(localAuth services.LocalAuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dto.CreateUserRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}
		user, setupToken, err := localAuth.CreateUser(req.Email, req.Name, req.Password, models.UserRole(req.Role))
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}
		c.JSON(http.StatusCreated, gin.H{
			"user":        dto.UserToResponse(*user),
			"setup_token": setupToken,
		})
	}
}

// UpdateUser handles PATCH /api/v1/settings/users/:id
func UpdateUser(localAuth services.LocalAuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid user id"}})
			return
		}
		var req dto.UpdateUserRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}
		if err := localAuth.UpdateUser(id, req.Name, models.UserRole(req.Role), req.Password); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// DeactivateUser handles DELETE /api/v1/settings/users/:id
func DeactivateUser(localAuth services.LocalAuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid user id"}})
			return
		}
		if err := localAuth.DeactivateUser(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// ResetUserPassword handles POST /api/v1/settings/users/:id/reset-password
func ResetUserPassword(localAuth services.LocalAuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid user id"}})
			return
		}
		token, err := localAuth.ResetPassword(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"setup_token": token})
	}
}
