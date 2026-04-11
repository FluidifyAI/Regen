package handlers

import (
	"net/http"

	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/gin-gonic/gin"
)

// ListUsersForAssignment returns minimal user info for all active users.
// Available to all authenticated users (not just admins) for commander assignment.
func ListUsersForAssignment(userRepo repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		users, err := userRepo.ListAll()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list users"})
			return
		}
		type userEntry struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Email string `json:"email"`
			Role  string `json:"role"`
		}
		out := make([]userEntry, 0, len(users))
		for _, u := range users {
			if !u.Active {
				continue
			}
			out = append(out, userEntry{
				ID:    u.ID.String(),
				Name:  u.Name,
				Email: u.Email,
				Role:  string(u.Role),
			})
		}
		c.JSON(http.StatusOK, gin.H{"users": out})
	}
}
