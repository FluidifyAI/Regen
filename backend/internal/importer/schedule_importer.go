package importer

import (
	"fmt"

	"github.com/fluidify/regen/internal/integrations/pagerduty"
	"github.com/fluidify/regen/internal/models"
)

// scheduleRepoWriter is the minimal subset of repository.ScheduleRepository
// that the schedule importer needs. Using a local interface allows easy mocking
// in tests without importing the full repository package.
type scheduleRepoWriter interface {
	Create(s *models.Schedule) error
	GetAll() ([]models.Schedule, error)
	CreateLayer(l *models.ScheduleLayer) error
	CreateParticipantsBulk(p []models.ScheduleParticipant) error
}

// ImportSchedules imports all PagerDuty schedule details into Fluidify Regen.
// emailToName is used to resolve user names; force overwrites name conflicts.
func ImportSchedules(
	repo scheduleRepoWriter,
	details []pagerduty.PDScheduleDetail,
	emailToName map[string]string,
	force bool,
	report *ImportReport,
) error {
	report.Summary.SchedulesFound = len(details)
	for _, d := range details {
		if err := importSchedule(repo, d, emailToName, force, report); err != nil {
			return err
		}
	}
	return nil
}

// importSchedule handles one PD schedule.
func importSchedule(
	repo scheduleRepoWriter,
	d pagerduty.PDScheduleDetail,
	emailToName map[string]string,
	force bool,
	report *ImportReport,
) error {
	existing, err := repo.GetAll()
	if err != nil {
		return fmt.Errorf("checking existing schedules: %w", err)
	}
	for _, e := range existing {
		if e.Name == d.Name {
			if !force {
				report.Summary.SchedulesSkipped++
				report.Warnings = append(report.Warnings, fmt.Sprintf(
					"Schedule %q: name conflict — already exists. Use --force to overwrite.",
					d.Name,
				))
				return nil
			}
			break
		}
	}

	tz := d.TimeZone
	if tz == "" {
		tz = "UTC"
	}

	schedule := &models.Schedule{
		Name:        d.Name,
		Description: d.Description,
		Timezone:    tz,
	}
	if err := repo.Create(schedule); err != nil {
		return fmt.Errorf("creating schedule %q: %w", d.Name, err)
	}
	report.Summary.SchedulesImported++

	for i, pdLayer := range d.ScheduleLayers {
		validation := validateScheduleLayer(d.Name, i, pdLayer)
		if !validation.ok {
			report.Summary.LayersSkipped++
			report.Warnings = append(report.Warnings, validation.warning)
			continue
		}
		if validation.warning != "" {
			report.Warnings = append(report.Warnings, validation.warning)
		}

		rotationType := models.RotationTypeWeekly
		if pdLayer.RotationTurnLengthSeconds == 86400 {
			rotationType = models.RotationTypeDaily
		}

		layer := &models.ScheduleLayer{
			ScheduleID:           schedule.ID,
			Name:                 pdLayer.Name,
			OrderIndex:           i,
			RotationType:         rotationType,
			RotationStart:        pdLayer.RotationVirtualStart,
			ShiftDurationSeconds: pdLayer.RotationTurnLengthSeconds,
		}
		if err := repo.CreateLayer(layer); err != nil {
			return fmt.Errorf("creating layer %q in schedule %q: %w", pdLayer.Name, d.Name, err)
		}
		report.Summary.LayersImported++

		var participants []models.ScheduleParticipant
		for j, u := range pdLayer.Users {
			participants = append(participants, models.ScheduleParticipant{
				LayerID:    layer.ID,
				UserName:   resolveUserName(u.User, emailToName),
				OrderIndex: j,
			})
		}
		if len(participants) > 0 {
			if err := repo.CreateParticipantsBulk(participants); err != nil {
				return fmt.Errorf("adding participants to layer %q: %w", pdLayer.Name, err)
			}
		}
	}
	return nil
}

// resolveUserName returns the email → name mapping if available, otherwise
// falls back to the user's email address, then the display name.
func resolveUserName(u pagerduty.PDUser, emailToName map[string]string) string {
	if name, ok := emailToName[u.Email]; ok {
		return name
	}
	if u.Email != "" {
		return u.Email
	}
	return u.Name
}
