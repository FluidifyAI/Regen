package handlers

import (
	"net/http"
	"strings"

	"github.com/FluidifyAI/Regen/backend/internal/config"
	"github.com/FluidifyAI/Regen/backend/internal/integrations/oncall"
	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/FluidifyAI/Regen/backend/internal/services"
	"github.com/gin-gonic/gin"
)

// oncallMigrationRequest is the body for both preview and import endpoints.
type oncallMigrationRequest struct {
	OnCallURL string `json:"oncall_url" binding:"required"`
	APIToken  string `json:"api_token"  binding:"required"`
}

// PreviewOnCallMigration handles POST /api/v1/migrations/oncall/preview.
// It fetches all data from the remote Grafana OnCall instance, runs the
// transformation, and returns what would be created — without writing to the DB.
func PreviewOnCallMigration(
	localAuth services.LocalAuthService,
	scheduleRepo repository.ScheduleRepository,
	escalationRepo repository.EscalationPolicyRepository,
	cfg *config.Config,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req oncallMigrationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}

		client := oncall.NewClient(req.OnCallURL, req.APIToken)
		if err := client.Ping(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}

		preview, err := fetchAndTransform(client, localAuth, scheduleRepo, escalationRepo, cfg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}

		c.JSON(http.StatusOK, previewResponse(preview))
	}
}

// ImportOnCallMigration handles POST /api/v1/migrations/oncall/import.
// It fetches, transforms, and persists all importable data, returning a summary.
func ImportOnCallMigration(
	localAuth services.LocalAuthService,
	scheduleRepo repository.ScheduleRepository,
	escalationRepo repository.EscalationPolicyRepository,
	cfg *config.Config,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req oncallMigrationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}

		client := oncall.NewClient(req.OnCallURL, req.APIToken)
		if err := client.Ping(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}

		preview, err := fetchAndTransform(client, localAuth, scheduleRepo, escalationRepo, cfg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}

		// Check user limit before writing anything.
		currentCount, err := localAuth.CountUsers()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "failed to check user count"}})
			return
		}
		newUserCount := int64(len(preview.Users))
		if currentCount+newUserCount > int64(cfg.OSSUserLimit) {
			c.JSON(http.StatusForbidden, gin.H{"error": gin.H{
				"code":        "user_limit_exceeded",
				"message":     "Importing these users would exceed the community edition limit",
				"limit":       cfg.OSSUserLimit,
				"current":     currentCount,
				"to_import":   newUserCount,
				"would_reach": currentCount + newUserCount,
			}})
			return
		}

		result, err := writeImport(preview, localAuth, scheduleRepo, escalationRepo)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
			return
		}

		c.JSON(http.StatusOK, result)
	}
}

// ── fetch + transform ─────────────────────────────────────────────────────────

func fetchAndTransform(
	client *oncall.Client,
	localAuth services.LocalAuthService,
	scheduleRepo repository.ScheduleRepository,
	escalationRepo repository.EscalationPolicyRepository,
	cfg *config.Config,
) (oncall.ImportPreview, error) {
	// Fetch all data from OnCall in parallel-ish sequential calls.
	users, err := client.ListUsers()
	if err != nil {
		return oncall.ImportPreview{}, err
	}
	schedules, err := client.ListSchedules()
	if err != nil {
		return oncall.ImportPreview{}, err
	}
	shifts, err := client.ListShifts()
	if err != nil {
		return oncall.ImportPreview{}, err
	}
	chains, err := client.ListEscalationChains()
	if err != nil {
		return oncall.ImportPreview{}, err
	}
	steps, err := client.ListEscalationSteps()
	if err != nil {
		return oncall.ImportPreview{}, err
	}
	integrations, err := client.ListIntegrations()
	if err != nil {
		return oncall.ImportPreview{}, err
	}

	// Build conflict-detection sets from existing Regen data.
	existingEmails, existingScheduleNames, existingPolicyNames, err :=
		buildExistingSets(localAuth, scheduleRepo, escalationRepo)
	if err != nil {
		return oncall.ImportPreview{}, err
	}

	baseURL := backendBaseURL(cfg)
	return oncall.TransformAll(
		users, schedules, shifts, chains, steps, integrations,
		existingEmails, existingScheduleNames, existingPolicyNames,
		baseURL,
	), nil
}

func buildExistingSets(
	localAuth services.LocalAuthService,
	scheduleRepo repository.ScheduleRepository,
	escalationRepo repository.EscalationPolicyRepository,
) (emails, scheduleNames, policyNames map[string]bool, err error) {
	emails = make(map[string]bool)
	scheduleNames = make(map[string]bool)
	policyNames = make(map[string]bool)

	existingUsers, err := localAuth.ListUsers()
	if err != nil {
		return nil, nil, nil, err
	}
	for _, u := range existingUsers {
		emails[strings.ToLower(u.Email)] = true
	}

	existingSchedules, err := scheduleRepo.GetAll()
	if err != nil {
		return nil, nil, nil, err
	}
	for _, s := range existingSchedules {
		scheduleNames[s.Name] = true
	}

	existingPolicies, err := escalationRepo.GetAllPolicies()
	if err != nil {
		return nil, nil, nil, err
	}
	for _, p := range existingPolicies {
		policyNames[p.Name] = true
	}

	return emails, scheduleNames, policyNames, nil
}

// backendBaseURL derives the backend base URL used to construct new webhook URLs.
// We use FrontendURL as the public base since in production both are served from
// the same origin. In dev mode the backend port (8080) is used.
func backendBaseURL(cfg *config.Config) string {
	if cfg.IsProduction() {
		return strings.TrimRight(cfg.FrontendURL, "/")
	}
	return "http://localhost:8080"
}

// ── write import ──────────────────────────────────────────────────────────────

type importResult struct {
	Imported struct {
		Users              int `json:"users"`
		Schedules          int `json:"schedules"`
		EscalationPolicies int `json:"escalation_policies"`
	} `json:"imported"`
	Skipped     []oncall.SkippedItem    `json:"skipped"`
	Conflicts   []oncall.ConflictItem   `json:"conflicts"`
	Webhooks    []oncall.WebhookMapping `json:"webhooks"`
	SetupTokens []userSetupToken        `json:"setup_tokens"`
}

type userSetupToken struct {
	Email      string `json:"email"`
	SetupToken string `json:"setup_token"`
}

func writeImport(
	preview oncall.ImportPreview,
	localAuth services.LocalAuthService,
	scheduleRepo repository.ScheduleRepository,
	escalationRepo repository.EscalationPolicyRepository,
) (importResult, error) {
	result := importResult{
		Skipped:   preview.Skipped,
		Conflicts: preview.Conflicts,
		Webhooks:  preview.Webhooks,
	}

	// ── Users ──────────────────────────────────────────────────────────────────
	// User names and participant references are already resolved in the transform
	// layer, so we simply create each user and collect setup tokens.
	for _, u := range preview.Users {
		_, setupToken, err := localAuth.CreateUser(u.Email, u.Name, generateTempPassword(), u.Role)
		if err != nil {
			if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "already exists") {
				result.Conflicts = append(result.Conflicts, oncall.ConflictItem{
					Type:   "user",
					Name:   u.Email,
					Reason: "already exists (concurrent import or race)",
				})
				continue
			}
			return importResult{}, err
		}
		if u.SlackUserID != nil {
			if created, err := localAuth.GetUserByEmail(u.Email); err == nil {
				_ = localAuth.UpdateUserSlackID(created.ID, u.SlackUserID)
			}
		}
		result.SetupTokens = append(result.SetupTokens, userSetupToken{
			Email:      u.Email,
			SetupToken: setupToken,
		})
		result.Imported.Users++
	}

	// ── Schedules ──────────────────────────────────────────────────────────────
	for i := range preview.Schedules {
		sched := &preview.Schedules[i]
		if err := scheduleRepo.Create(sched); err != nil {
			if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
				result.Conflicts = append(result.Conflicts, oncall.ConflictItem{
					Type:   "schedule",
					Name:   sched.Name,
					Reason: "already exists (concurrent import or race)",
				})
				continue
			}
			return importResult{}, err
		}
		for j := range sched.Layers {
			layer := &sched.Layers[j]
			if err := scheduleRepo.CreateLayer(layer); err != nil {
				return importResult{}, err
			}
			if len(layer.Participants) > 0 {
				if err := scheduleRepo.CreateParticipantsBulk(layer.Participants); err != nil {
					return importResult{}, err
				}
			}
		}
		result.Imported.Schedules++
	}

	// ── Escalation Policies ────────────────────────────────────────────────────
	for i := range preview.EscalationPolicies {
		policy := &preview.EscalationPolicies[i]
		if err := escalationRepo.CreatePolicy(policy); err != nil {
			if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
				result.Conflicts = append(result.Conflicts, oncall.ConflictItem{
					Type:   "escalation_policy",
					Name:   policy.Name,
					Reason: "already exists (concurrent import or race)",
				})
				continue
			}
			return importResult{}, err
		}
		for _, tier := range policy.Tiers {
			if err := escalationRepo.CreateTier(&tier); err != nil {
				return importResult{}, err
			}
		}
		result.Imported.EscalationPolicies++
	}

	return result, nil
}

// ── Response shaping ──────────────────────────────────────────────────────────

type previewEntitySummary[T any] struct {
	Count int `json:"count"`
	Items []T `json:"items"`
}

type previewResponseBody struct {
	Users              previewEntitySummary[models.User]             `json:"users"`
	Schedules          previewEntitySummary[models.Schedule]         `json:"schedules"`
	EscalationPolicies previewEntitySummary[models.EscalationPolicy] `json:"escalation_policies"`
	Webhooks           previewEntitySummary[oncall.WebhookMapping]   `json:"webhooks"`
	Conflicts          []oncall.ConflictItem                         `json:"conflicts"`
	Skipped            []oncall.SkippedItem                          `json:"skipped"`
}

func previewResponse(p oncall.ImportPreview) previewResponseBody {
	return previewResponseBody{
		Users:              previewEntitySummary[models.User]{Count: len(p.Users), Items: p.Users},
		Schedules:          previewEntitySummary[models.Schedule]{Count: len(p.Schedules), Items: p.Schedules},
		EscalationPolicies: previewEntitySummary[models.EscalationPolicy]{Count: len(p.EscalationPolicies), Items: p.EscalationPolicies},
		Webhooks:           previewEntitySummary[oncall.WebhookMapping]{Count: len(p.Webhooks), Items: p.Webhooks},
		Conflicts:          p.Conflicts,
		Skipped:            p.Skipped,
	}
}

// ── Utilities ─────────────────────────────────────────────────────────────────

// generateTempPassword creates a random password that satisfies the 8-char minimum.
// Users will replace it via their setup token.
func generateTempPassword() string {
	return "Regen!" + generateToken(12)
}

func generateToken(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[i%len(chars)]
	}
	return string(b)
}
