package importer

import (
	"fmt"

	"github.com/FluidifyAI/Regen/backend/internal/integrations/opsgenie"
	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/google/uuid"
)

// ImportOpsgeniePolicies imports all Opsgenie escalation policies into Regen.
func ImportOpsgeniePolicies(
	repo policyRepoWriter,
	policies []opsgenie.OGEscalationPolicy,
	scheduleNameToID map[string]uuid.UUID,
	emailToName map[string]string,
	force bool,
	report *ImportReport,
) error {
	report.Summary.PoliciesFound = len(policies)
	for _, p := range policies {
		if err := importOpsgeniePolicy(repo, p, scheduleNameToID, emailToName, force, report); err != nil {
			return err
		}
	}
	return nil
}

func importOpsgeniePolicy(
	repo policyRepoWriter,
	p opsgenie.OGEscalationPolicy,
	scheduleNameToID map[string]uuid.UUID,
	emailToName map[string]string,
	force bool,
	report *ImportReport,
) error {
	existing, err := repo.GetAllPolicies()
	if err != nil {
		return fmt.Errorf("checking existing policies: %w", err)
	}
	for _, e := range existing {
		if e.Name == p.Name {
			if !force {
				report.Summary.PoliciesSkipped++
				report.Warnings = append(report.Warnings, fmt.Sprintf(
					"Policy %q: name conflict — already exists. Use --force to overwrite.",
					p.Name,
				))
				return nil
			}
			break
		}
	}

	policy := &models.EscalationPolicy{
		Name:        p.Name,
		Description: p.Description,
		Enabled:     true,
	}
	if err := repo.CreatePolicy(policy); err != nil {
		return fmt.Errorf("creating policy %q: %w", p.Name, err)
	}
	report.Summary.PoliciesImported++

	for i, rule := range p.Rules {
		var schedID *uuid.UUID
		var userNames []string
		hasSchedule := false
		hasUser := false

		for _, rec := range rule.Recipient {
			switch rec.Type {
			case "schedule":
				hasSchedule = true
				if scheduleNameToID != nil {
					if id, ok := scheduleNameToID[rec.Name]; ok {
						schedID = &id
					}
				}
			case "user":
				hasUser = true
				name := ogResolveUser(opsgenie.OGParticipant{Username: rec.Username, Name: rec.Name}, emailToName)
				userNames = append(userNames, name)
			case "team":
				report.Warnings = append(report.Warnings, fmt.Sprintf(
					"Policy %q tier %d: team target %q skipped (teams are not yet supported).",
					p.Name, i, rec.Name,
				))
			}
		}

		targetType := resolveTargetType(hasSchedule, hasUser)
		if targetType == "" {
			targetType = string(models.EscalationTargetSchedule)
		}

		tier := &models.EscalationTier{
			PolicyID:       policy.ID,
			TierIndex:      i,
			TimeoutSeconds: ogDelayToSeconds(rule.Delay),
			TargetType:     models.EscalationTargetType(targetType),
			ScheduleID:     schedID,
			UserNames:      models.JSONBArray(userNames),
		}
		if err := repo.CreateTier(tier); err != nil {
			return fmt.Errorf("creating tier %d for policy %q: %w", i, p.Name, err)
		}
		report.Summary.TiersImported++
	}
	return nil
}

// ogDelayToSeconds converts an OGDelay to a total number of seconds.
func ogDelayToSeconds(d opsgenie.OGDelay) int {
	switch d.TimeUnit {
	case "hours":
		return d.TimeAmount * 3600
	case "days":
		return d.TimeAmount * 86400
	default: // minutes
		return d.TimeAmount * 60
	}
}
