package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/openincident/openincident/internal/api/handlers/dto"
	"github.com/openincident/openincident/internal/api/middleware"
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
		role := models.UserRoleMember
		if req.Role != "" {
			role = models.UserRole(req.Role)
		}
		user, setupToken, err := localAuth.CreateUser(req.Email, req.Name, req.Password, role)
		if err != nil {
			errMsg := err.Error()
			// Check for duplicate email (GORM/PostgreSQL unique constraint violation)
			if strings.Contains(errMsg, "duplicate") || strings.Contains(errMsg, "unique") || strings.Contains(errMsg, "already exists") {
				c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "duplicate_email", "message": "A user with this email already exists"}})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": errMsg}})
			return
		}
		if req.SlackUserID != "" {
			if err := localAuth.UpdateUserSlackID(user.ID, &req.SlackUserID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
				return
			}
			user.SlackUserID = &req.SlackUserID
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

		// Get the current user to preserve unset fields
		currentUser, err := localAuth.GetUser(id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": "user not found"}})
			return
		}

		name := currentUser.Name
		if req.Name != nil {
			name = *req.Name
		}
		role := currentUser.Role
		if req.Role != nil {
			role = models.UserRole(*req.Role)
		}
		var password string
		if req.Password != nil {
			password = *req.Password
		}

		if err := localAuth.UpdateUser(id, name, role, password); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}
		if req.SlackUserID != nil {
			var slackID *string
			if *req.SlackUserID != "" {
				slackID = req.SlackUserID
			}
			if err := localAuth.UpdateUserSlackID(id, slackID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
				return
			}
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

		// Guard: cannot deactivate yourself
		if caller := middleware.GetLocalUser(c); caller != nil && caller.ID == id {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "cannot deactivate your own account"}})
			return
		}

		// Guard: cannot remove the last admin — pre-fetch to get the target's role
		target, err := localAuth.GetUser(id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": "user not found"}})
			return
		}
		if target.Role == models.UserRoleAdmin {
			adminCount, err := localAuth.CountAdmins()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "failed to check admin count"}})
				return
			}
			if adminCount <= 1 {
				c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "cannot deactivate the last admin account"}})
				return
			}
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

// CreateFirstUser handles POST /api/v1/auth/bootstrap — creates the initial admin account.
// Returns 409 if any users already exist. This endpoint is unauthenticated on purpose.
func CreateFirstUser(localAuth services.LocalAuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if localAuth == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"message": "local auth not configured"}})
			return
		}
		// Only allowed when no users exist
		count, err := localAuth.CountUsers()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "failed to check user count"}})
			return
		}
		if count > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": gin.H{"message": "users already exist; use admin login to create additional users"}})
			return
		}
		var req dto.CreateUserRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}
		// Role is forced to admin regardless of what was requested — this is always the first admin.
		user, setupToken, err := localAuth.CreateUser(req.Email, req.Name, req.Password, models.UserRoleAdmin)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}
		c.JSON(http.StatusCreated, gin.H{
			"user":        dto.UserToResponse(*user),
			"setup_token": setupToken,
			"message":     "Initial admin account created. Use the setup_token to log in.",
		})
	}
}
