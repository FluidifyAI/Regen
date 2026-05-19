package handlers

import (
	"net/http"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/gin-gonic/gin"
)

// StatusIncident is the public-facing incident shape for the status page.
type StatusIncident struct {
	IncidentNumber int        `json:"incident_number"`
	Title          string     `json:"title"`
	Severity       string     `json:"severity"`
	Status         string     `json:"status"`
	TriggeredAt    time.Time  `json:"triggered_at"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	DurationSecs   *int64     `json:"duration_seconds,omitempty"`
}

// StatusPageResponse is the full status page payload.
type StatusPageResponse struct {
	OrgName          string           `json:"org_name"`
	GeneratedAt      time.Time        `json:"generated_at"`
	ActiveIncidents  []StatusIncident `json:"active_incidents"`
	RecentlyResolved []StatusIncident `json:"recently_resolved"`
}

// GetStatusPage returns an unauthenticated status page payload.
// Active = triggered or acknowledged. Recently resolved = resolved/canceled in last 7 days.
func GetStatusPage(incidentRepo repository.IncidentRepository, settingsRepo repository.SystemSettingsRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		orgName, _ := settingsRepo.GetString(repository.KeyInstanceName)
		if orgName == "" {
			orgName = "Regen"
		}

		active, err := fetchActive(incidentRepo)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch active incidents"})
			return
		}

		recent, err := fetchRecentlyResolved(incidentRepo)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch resolved incidents"})
			return
		}

		c.JSON(http.StatusOK, StatusPageResponse{
			OrgName:          orgName,
			GeneratedAt:      time.Now().UTC(),
			ActiveIncidents:  active,
			RecentlyResolved: recent,
		})
	}
}

func fetchActive(repo repository.IncidentRepository) ([]StatusIncident, error) {
	triggered, _, err := repo.List(
		repository.IncidentFilters{Status: models.IncidentStatusTriggered},
		repository.Pagination{Page: 1, PageSize: 100},
	)
	if err != nil {
		return nil, err
	}

	acknowledged, _, err := repo.List(
		repository.IncidentFilters{Status: models.IncidentStatusAcknowledged},
		repository.Pagination{Page: 1, PageSize: 100},
	)
	if err != nil {
		return nil, err
	}

	all := append(triggered, acknowledged...)
	return toStatusIncidents(all), nil
}

func fetchRecentlyResolved(repo repository.IncidentRepository) ([]StatusIncident, error) {
	since := time.Now().UTC().AddDate(0, 0, -7)
	resolved, _, err := repo.List(
		repository.IncidentFilters{
			Status:        models.IncidentStatusResolved,
			ResolvedSince: &since,
		},
		repository.Pagination{Page: 1, PageSize: 100},
	)
	if err != nil {
		return nil, err
	}

	canceled, _, err := repo.List(
		repository.IncidentFilters{
			Status:        models.IncidentStatusCanceled,
			ResolvedSince: &since,
		},
		repository.Pagination{Page: 1, PageSize: 100},
	)
	if err != nil {
		return nil, err
	}

	all := append(resolved, canceled...)
	return toStatusIncidents(all), nil
}

func toStatusIncidents(incidents []models.Incident) []StatusIncident {
	out := make([]StatusIncident, 0, len(incidents))
	for _, inc := range incidents {
		si := StatusIncident{
			IncidentNumber: inc.IncidentNumber,
			Title:          inc.Title,
			Severity:       string(inc.Severity),
			Status:         string(inc.Status),
			TriggeredAt:    inc.TriggeredAt,
			ResolvedAt:     inc.ResolvedAt,
		}
		if inc.ResolvedAt != nil {
			secs := int64(inc.ResolvedAt.Sub(inc.TriggeredAt).Seconds())
			si.DurationSecs = &secs
		}
		out = append(out, si)
	}
	return out
}
