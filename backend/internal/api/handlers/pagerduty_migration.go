package handlers

import (
	"net/http"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/importer"
	"github.com/FluidifyAI/Regen/backend/internal/integrations/pagerduty"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// pdClientFactory is a package-level variable so tests can override it to point
// the real PagerDuty client at a mock HTTP server.
var pdClientFactory = func(apiKey, baseURL string) *pagerduty.Client {
	return pagerduty.NewClientWithBaseURL(apiKey, baseURL)
}

// pdBaseURL maps a region string ("us" or "eu") to the PagerDuty API base URL.
// Defaults to the US endpoint for empty or unrecognised values.
func pdBaseURL(region string) string {
	if region == "eu" {
		return "https://api.eu.pagerduty.com"
	}
	return "https://api.pagerduty.com"
}

type pdMigrationRequest struct {
	APIKey string `json:"api_key" binding:"required"`
	Region string `json:"region"`
	Force  bool   `json:"force"`
}

// ── Preview response types ────────────────────────────────────────────────────

type pdPreviewSchedule struct {
	Name       string `json:"name"`
	Timezone   string `json:"timezone"`
	LayerCount int    `json:"layer_count"`
	UserCount  int    `json:"user_count"`
}

type pdPreviewPolicy struct {
	Name      string `json:"name"`
	TierCount int    `json:"tier_count"`
}

type pdPreviewResponseBody struct {
	Schedules []pdPreviewSchedule `json:"schedules"`
	Policies  []pdPreviewPolicy   `json:"policies"`
	Warnings  []string            `json:"warnings"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// PreviewPagerDutyMigration handles POST /api/v1/migrations/pagerduty/preview.
// Validates the API key, fetches PD data, and returns a summary without writing to the DB.
func PreviewPagerDutyMigration(
	scheduleRepo repository.ScheduleRepository,
	escalationRepo repository.EscalationPolicyRepository,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req pdMigrationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}

		client := pdClientFactory(req.APIKey, pdBaseURL(req.Region))
		if err := client.ValidateAPIKey(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}

		_, scheduleDetails, policyDetails, err := fetchPDData(client)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}

		resp := buildPDPreview(scheduleDetails, policyDetails)
		c.JSON(http.StatusOK, resp)
	}
}

// ImportPagerDutyMigration handles POST /api/v1/migrations/pagerduty/import.
// Validates the API key, fetches PD data, and persists schedules and escalation policies.
func ImportPagerDutyMigration(
	scheduleRepo repository.ScheduleRepository,
	escalationRepo repository.EscalationPolicyRepository,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req pdMigrationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}

		client := pdClientFactory(req.APIKey, pdBaseURL(req.Region))
		if err := client.ValidateAPIKey(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}

		emailToName, scheduleDetails, policyDetails, err := fetchPDData(client)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}

		report := &importer.ImportReport{ImportedAt: time.Now()}

		if err := importer.ImportSchedules(scheduleRepo, scheduleDetails, emailToName, req.Force, report); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}

		scheduleNameToID, err := pdBuildScheduleNameMap(scheduleRepo)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}

		if err := importer.ImportPolicies(escalationRepo, policyDetails, scheduleNameToID, emailToName, req.Force, report); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}

		c.JSON(http.StatusOK, report)
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func fetchPDData(client *pagerduty.Client) (
	emailToName map[string]string,
	scheduleDetails []pagerduty.PDScheduleDetail,
	policyDetails []pagerduty.PDEscalationPolicyDetail,
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
		detail, fetchErr := client.FetchScheduleDetail(s.ID)
		if fetchErr != nil {
			continue
		}
		scheduleDetails = append(scheduleDetails, *detail)
	}

	policies, err := client.FetchEscalationPolicies()
	if err != nil {
		return
	}
	for _, p := range policies {
		detail, fetchErr := client.FetchEscalationPolicyDetail(p.ID)
		if fetchErr != nil {
			continue
		}
		policyDetails = append(policyDetails, *detail)
	}

	return
}

func buildPDPreview(
	scheduleDetails []pagerduty.PDScheduleDetail,
	policyDetails []pagerduty.PDEscalationPolicyDetail,
) pdPreviewResponseBody {
	schedules := make([]pdPreviewSchedule, 0, len(scheduleDetails))
	for _, d := range scheduleDetails {
		userCount := 0
		for _, l := range d.ScheduleLayers {
			userCount += len(l.Users)
		}
		schedules = append(schedules, pdPreviewSchedule{
			Name:       d.Name,
			Timezone:   d.TimeZone,
			LayerCount: len(d.ScheduleLayers),
			UserCount:  userCount,
		})
	}

	policies := make([]pdPreviewPolicy, 0, len(policyDetails))
	for _, d := range policyDetails {
		policies = append(policies, pdPreviewPolicy{
			Name:      d.Name,
			TierCount: len(d.EscalationRules),
		})
	}

	return pdPreviewResponseBody{
		Schedules: schedules,
		Policies:  policies,
		Warnings:  []string{"PagerDuty services are not imported — configure alert routing rules manually in Regen."},
	}
}

func pdBuildScheduleNameMap(repo repository.ScheduleRepository) (map[string]uuid.UUID, error) {
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
