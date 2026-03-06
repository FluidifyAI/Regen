package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/openincident/openincident/internal/repository"
	"github.com/openincident/openincident/internal/services"
)

// ListTeamsMembers handles GET /api/v1/settings/teams/members.
// Returns all AAD members of the configured team.
// Each member is annotated with already_imported=true if a user with that email
// already exists in OpenIncident.
func ListTeamsMembers(teamsConfigRepo repository.TeamsConfigRepository, userRepo repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := teamsConfigRepo.Get()
		if err != nil || cfg == nil || cfg.AppID == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Teams is not configured"})
			return
		}

		members, err := services.ListTeamMembers(c.Request.Context(), cfg.AppID, cfg.AppPassword, cfg.TenantID, cfg.TeamID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch Teams members: " + err.Error()})
			return
		}

		// Build a set of emails already imported into OpenIncident.
		existing, _ := userRepo.ListAll()
		importedEmails := make(map[string]string, len(existing)) // email → user id
		for _, u := range existing {
			importedEmails[u.Email] = u.ID.String()
		}

		type member struct {
			ID                 string `json:"id"`
			Name               string `json:"name"`
			Email              string `json:"email"`
			AlreadyImported    bool   `json:"already_imported"`
			OpenIncidentUserID string `json:"openincident_user_id,omitempty"`
		}

		result := make([]member, 0, len(members))
		for _, m := range members {
			uid, imported := importedEmails[m.Email]
			result = append(result, member{
				ID:                 m.UserID,
				Name:               m.DisplayName,
				Email:              m.Email,
				AlreadyImported:    imported,
				OpenIncidentUserID: uid,
			})
		}

		c.JSON(http.StatusOK, gin.H{"members": result})
	}
}
