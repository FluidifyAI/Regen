package importer

import (
	"fmt"

	"github.com/FluidifyAI/Regen/backend/internal/integrations/opsgenie"
	"github.com/FluidifyAI/Regen/backend/internal/models"
)

// ImportOpsgenieSchedules imports all Opsgenie schedule details into Regen.
func ImportOpsgenieSchedules(
	repo scheduleRepoWriter,
	details []opsgenie.OGScheduleDetail,
	emailToName map[string]string,
	force bool,
	report *ImportReport,
) error {
	report.Summary.SchedulesFound = len(details)
	for _, d := range details {
		if err := importOpsgenieSchedule(repo, d, emailToName, force, report); err != nil {
			return err
		}
	}
	return nil
}

func importOpsgenieSchedule(
	repo scheduleRepoWriter,
	d opsgenie.OGScheduleDetail,
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

	tz := d.Timezone
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

	for i, rot := range d.Rotations {
		rotationType, shiftSeconds := ogRotationToRegen(rot)

		layer := &models.ScheduleLayer{
			ScheduleID:           schedule.ID,
			Name:                 rot.Name,
			OrderIndex:           i,
			RotationType:         rotationType,
			RotationStart:        rot.StartDate,
			ShiftDurationSeconds: shiftSeconds,
		}
		if err := repo.CreateLayer(layer); err != nil {
			return fmt.Errorf("creating layer %q in schedule %q: %w", rot.Name, d.Name, err)
		}
		report.Summary.LayersImported++

		var participants []models.ScheduleParticipant
		for j, p := range rot.Participants {
			if p.Type != "user" {
				continue
			}
			participants = append(participants, models.ScheduleParticipant{
				LayerID:    layer.ID,
				UserName:   ogResolveUser(p, emailToName),
				OrderIndex: j,
			})
		}
		if len(participants) > 0 {
			if err := repo.CreateParticipantsBulk(participants); err != nil {
				return fmt.Errorf("adding participants to layer %q: %w", rot.Name, err)
			}
		}
	}
	return nil
}

// ogRotationToRegen converts an Opsgenie rotation to a Regen RotationType and shift duration in seconds.
// weekly/length=1 → weekly, daily/length=1 → daily, everything else → custom.
func ogRotationToRegen(rot opsgenie.OGRotation) (models.RotationType, int) {
	length := rot.Length
	if length <= 0 {
		length = 1
	}
	switch rot.Type {
	case "weekly":
		if length == 1 {
			return models.RotationTypeWeekly, 604800
		}
		return models.RotationTypeCustom, length * 604800
	case "daily":
		if length == 1 {
			return models.RotationTypeDaily, 86400
		}
		return models.RotationTypeCustom, length * 86400
	case "hourly":
		return models.RotationTypeCustom, length * 3600
	default: // "none" or unknown
		return models.RotationTypeCustom, 86400
	}
}

// ogResolveUser returns the best display name for an OG participant.
func ogResolveUser(p opsgenie.OGParticipant, emailToName map[string]string) string {
	if p.Username != "" {
		if name, ok := emailToName[p.Username]; ok {
			return name
		}
		return p.Username
	}
	return p.Name
}
