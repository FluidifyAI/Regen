package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/fluidify/regen/internal/models"
)

type UserResponse struct {
	ID          uuid.UUID  `json:"id"`
	Email       string     `json:"email"`
	Name        string     `json:"name"`
	Role        string     `json:"role"`
	AuthSource  string     `json:"auth_source"`
	SlackUserID *string    `json:"slack_user_id,omitempty"`
	TeamsUserID *string    `json:"teams_user_id,omitempty"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

func UserToResponse(u models.User) UserResponse {
	return UserResponse{
		ID:          u.ID,
		Email:       u.Email,
		Name:        u.Name,
		Role:        string(u.Role),
		AuthSource:  u.AuthSource,
		SlackUserID: u.SlackUserID,
		TeamsUserID: u.TeamsUserID,
		LastLoginAt: u.LastLoginAt,
		CreatedAt:   u.CreatedAt,
	}
}
