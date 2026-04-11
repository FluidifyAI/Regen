package importer

import (
	"fmt"

	"github.com/FluidifyAI/Regen/backend/internal/integrations/pagerduty"
)

// layerValidationResult holds the outcome of validating a single schedule layer.
type layerValidationResult struct {
	ok      bool
	warning string
}

// validateScheduleLayer checks whether a PagerDuty schedule layer can be mapped
// to a Fluidify Regen ScheduleLayer. Returns ok=false for custom rotations
// (RotationTurnLengthSeconds == 0) which cannot be modelled as a uniform interval.
func validateScheduleLayer(scheduleName string, layerIdx int, layer pagerduty.PDScheduleLayer) layerValidationResult {
	if layer.RotationTurnLengthSeconds == 0 {
		return layerValidationResult{
			ok: false,
			warning: fmt.Sprintf(
				"Schedule %q layer %d (%q): rotation_turn_length_seconds=0 (custom rotation) — skipped. "+
					"Create manually in Fluidify Regen UI.",
				scheduleName, layerIdx, layer.Name,
			),
		}
	}
	var warning string
	if len(layer.Users) == 0 {
		warning = fmt.Sprintf(
			"Schedule %q layer %d (%q): no users — layer imported with empty participant list.",
			scheduleName, layerIdx, layer.Name,
		)
	}
	return layerValidationResult{ok: true, warning: warning}
}

// validateEscalationRule checks one escalation rule's targets and returns
// a slice of warning strings (one per unsupported team target).
// An empty slice means all targets are importable.
func validateEscalationRule(policyName string, ruleIdx int, rule pagerduty.PDEscalationRule) []string {
	var warnings []string
	for _, t := range rule.Targets {
		if t.Type == "team_reference" {
			warnings = append(warnings, fmt.Sprintf(
				"Policy %q tier %d: team target %q not supported in v0.5 — skipped. "+
					"Assign users or a schedule manually.",
				policyName, ruleIdx, t.Name,
			))
		}
	}
	return warnings
}
