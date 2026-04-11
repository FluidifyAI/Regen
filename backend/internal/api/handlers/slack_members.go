package handlers

import (
	"net/http"

	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/gin-gonic/gin"
	slack "github.com/slack-go/slack"
)

// ListSlackMembers handles GET /api/v1/settings/slack/members.
// Returns all non-bot, non-deleted workspace members.
// Each member is annotated with already_imported=true if a user with that email exists in Fluidify Regen.
// Used by the "Import from Slack" modal in Settings → Users.
func ListSlackMembers(slackRepo repository.SlackConfigRepository, userRepo repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := slackRepo.Get()
		if err != nil || cfg == nil || cfg.BotToken == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Slack is not configured"})
			return
		}

		client := slack.New(cfg.BotToken)
		slackUsers, err := client.GetUsers()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch Slack members: " + err.Error()})
			return
		}

		// Build a set of emails that are already imported into Fluidify Regen.
		existing, _ := userRepo.ListAll()
		importedEmails := make(map[string]bool, len(existing))
		for _, u := range existing {
			importedEmails[u.Email] = true
		}

		type member struct {
			ID              string `json:"id"`
			Name            string `json:"name"`
			Email           string `json:"email"`
			Avatar          string `json:"avatar"`
			AlreadyImported bool   `json:"already_imported"`
		}

		members := make([]member, 0, len(slackUsers))
		for _, u := range slackUsers {
			if u.Deleted || u.IsBot || u.ID == "USLACKBOT" {
				continue
			}
			members = append(members, member{
				ID:              u.ID,
				Name:            u.RealName,
				Email:           u.Profile.Email,
				Avatar:          u.Profile.Image48,
				AlreadyImported: importedEmails[u.Profile.Email],
			})
		}

		c.JSON(http.StatusOK, gin.H{"members": members})
	}
}
