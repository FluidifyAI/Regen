package importer

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/FluidifyAI/Regen/backend/internal/integrations/pagerduty"
	"github.com/FluidifyAI/Regen/backend/internal/models"
)

// policyRepoWriter is the minimal subset of repository.EscalationPolicyRepository
// that the policy importer needs.
type policyRepoWriter interface {
	CreatePolicy(p *models.EscalationPolicy) error
	GetAllPolicies() ([]models.EscalationPolicy, error)
	CreateTier(t *models.EscalationTier) error
}

// ImportPolicies imports all PagerDuty escalation policy details into Fluidify Regen.
// scheduleNameToID maps imported schedule names to their new OI UUIDs.
// emailToName maps PD user email → display name for user_reference targets.
func ImportPolicies(
	repo policyRepoWriter,
	details []pagerduty.PDEscalationPolicyDetail,
	scheduleNameToID map[string]uuid.UUID,
	emailToName map[string]string,
	force bool,
	report *ImportReport,
) error {
	report.Summary.PoliciesFound = len(details)
	for _, d := range details {
		if err := importPolicy(repo, d, scheduleNameToID, emailToName, force, report); err != nil {
			return err
		}
	}
	return nil
}

func importPolicy(
	repo policyRepoWriter,
	d pagerduty.PDEscalationPolicyDetail,
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
		if e.Name == d.Name {
			if !force {
				report.Summary.PoliciesSkipped++
				report.Warnings = append(report.Warnings, fmt.Sprintf(
					"Policy %q: name conflict — already exists. Use --force to overwrite.",
					d.Name,
				))
				return nil
			}
			break
		}
	}

	policy := &models.EscalationPolicy{
		Name:        d.Name,
		Description: d.Description,
		Enabled:     true,
	}
	if err := repo.CreatePolicy(policy); err != nil {
		return fmt.Errorf("creating policy %q: %w", d.Name, err)
	}
	report.Summary.PoliciesImported++

	for i, rule := range d.EscalationRules {
		ruleWarnings := validateEscalationRule(d.Name, i, rule)
		report.Warnings = append(report.Warnings, ruleWarnings...)

		var schedID *uuid.UUID
		var userNames []string
		hasSchedule := false
		hasUser := false

		for _, target := range rule.Targets {
			switch target.Type {
			case "schedule_reference":
				hasSchedule = true
				if scheduleNameToID != nil {
					if id, ok := scheduleNameToID[target.Name]; ok {
						schedID = &id
					}
				}
			case "user_reference":
				hasUser = true
				name := resolveUserName(pagerduty.PDUser{Name: target.Name}, emailToName)
				userNames = append(userNames, name)
			// team_reference: skip (warned above)
			}
		}

		targetType := resolveTargetType(hasSchedule, hasUser)
		if targetType == "" {
			targetType = string(models.EscalationTargetSchedule)
		}

		tier := &models.EscalationTier{
			PolicyID:       policy.ID,
			TierIndex:      i,
			TimeoutSeconds: rule.EscalationDelayInMinutes * 60,
			TargetType:     models.EscalationTargetType(targetType),
			ScheduleID:     schedID,
			UserNames:      models.JSONBArray(userNames),
		}
		if err := repo.CreateTier(tier); err != nil {
			return fmt.Errorf("creating tier %d for policy %q: %w", i, d.Name, err)
		}
		report.Summary.TiersImported++
	}
	return nil
}

// resolveTargetType returns the EscalationTargetType string given which
// target kinds are present in a rule.
func resolveTargetType(hasSchedule, hasUser bool) string {
	switch {
	case hasSchedule && hasUser:
		return string(models.EscalationTargetBoth)
	case hasSchedule:
		return string(models.EscalationTargetSchedule)
	case hasUser:
		return string(models.EscalationTargetUsers)
	default:
		return ""
	}
}
