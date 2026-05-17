package handlers

import (
	"net/http"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/importer"
	"github.com/FluidifyAI/Regen/backend/internal/integrations/opsgenie"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ogClientFactory is overridable in tests to point at a mock HTTP server.
var ogClientFactory = func(apiKey, region string) *opsgenie.Client {
	return opsgenie.NewClient(apiKey, region)
}

type ogMigrationRequest struct {
	APIKey string `json:"api_key" binding:"required"`
	Region string `json:"region"` // "us" (default) or "eu"
	Force  bool   `json:"force"`
}

// ── Preview response types ────────────────────────────────────────────────────

type ogPreviewSchedule struct {
	Name          string `json:"name"`
	Timezone      string `json:"timezone"`
	RotationCount int    `json:"rotation_count"`
	UserCount     int    `json:"user_count"`
}

type ogPreviewPolicy struct {
	Name      string `json:"name"`
	RuleCount int    `json:"rule_count"`
}

type ogPreviewResponseBody struct {
	Schedules []ogPreviewSchedule `json:"schedules"`
	Policies  []ogPreviewPolicy   `json:"policies"`
	Warnings  []string            `json:"warnings"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// PreviewOpsgenieMigration handles POST /api/v1/migrations/opsgenie/preview.
func PreviewOpsgenieMigration(
	scheduleRepo repository.ScheduleRepository,
	escalationRepo repository.EscalationPolicyRepository,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ogMigrationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}
		if req.Region == "" {
			req.Region = "us"
		}

		client := ogClientFactory(req.APIKey, req.Region)
		if err := client.ValidateAPIKey(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}

		_, scheduleDetails, policies, err := fetchOGData(client)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}

		c.JSON(http.StatusOK, buildOGPreview(scheduleDetails, policies))
	}
}

// ImportOpsgenieMigration handles POST /api/v1/migrations/opsgenie/import.
func ImportOpsgenieMigration(
	scheduleRepo repository.ScheduleRepository,
	escalationRepo repository.EscalationPolicyRepository,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ogMigrationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}
		if req.Region == "" {
			req.Region = "us"
		}

		client := ogClientFactory(req.APIKey, req.Region)
		if err := client.ValidateAPIKey(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}

		emailToName, scheduleDetails, policies, err := fetchOGData(client)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}

		report := &importer.ImportReport{ImportedAt: time.Now()}

		if err := importer.ImportOpsgenieSchedules(scheduleRepo, scheduleDetails, emailToName, req.Force, report); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}

		scheduleNameToID, err := ogBuildScheduleNameMap(scheduleRepo)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}

		if err := importer.ImportOpsgeniePolicies(escalationRepo, policies, scheduleNameToID, emailToName, req.Force, report); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}

		c.JSON(http.StatusOK, report)
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func fetchOGData(client *opsgenie.Client) (
	emailToName map[string]string,
	scheduleDetails []opsgenie.OGScheduleDetail,
	policies []opsgenie.OGEscalationPolicy,
	err error,
) {
	emailToName, err = client.FetchUsers()
	if err != nil {
		return
	}

	schedules, err := client.FetchSchedules()
	if err != nil {
		return
	}
	for _, s := range schedules {
		rotations, fetchErr := client.FetchRotations(s.ID)
		if fetchErr != nil {
			continue
		}
		scheduleDetails = append(scheduleDetails, opsgenie.OGScheduleDetail{
			ID:          s.ID,
			Name:        s.Name,
			Description: s.Description,
			Timezone:    s.Timezone,
			Rotations:   rotations,
		})
	}

	policies, err = client.FetchEscalationPolicies()
	return
}

func buildOGPreview(
	scheduleDetails []opsgenie.OGScheduleDetail,
	policies []opsgenie.OGEscalationPolicy,
) ogPreviewResponseBody {
	schedules := make([]ogPreviewSchedule, 0, len(scheduleDetails))
	for _, d := range scheduleDetails {
		userCount := 0
		for _, r := range d.Rotations {
			for _, p := range r.Participants {
				if p.Type == "user" {
					userCount++
				}
			}
		}
		schedules = append(schedules, ogPreviewSchedule{
			Name:          d.Name,
			Timezone:      d.Timezone,
			RotationCount: len(d.Rotations),
			UserCount:     userCount,
		})
	}

	policyPreviews := make([]ogPreviewPolicy, 0, len(policies))
	for _, p := range policies {
		policyPreviews = append(policyPreviews, ogPreviewPolicy{
			Name:      p.Name,
			RuleCount: len(p.Rules),
		})
	}

	return ogPreviewResponseBody{
		Schedules: schedules,
		Policies:  policyPreviews,
		Warnings:  []string{"Opsgenie teams are not imported — configure team routing manually in Regen."},
	}
}

func ogBuildScheduleNameMap(repo repository.ScheduleRepository) (map[string]uuid.UUID, error) {
	schedules, err := repo.GetAll()
	if err != nil {
		return nil, err
	}
	m := make(map[string]uuid.UUID, len(schedules))
	for _, s := range schedules {
		m[s.Name] = s.ID
	}
	return m, nil
}
