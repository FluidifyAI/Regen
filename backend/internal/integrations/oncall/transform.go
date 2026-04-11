package oncall

import (
	"sort"
	"strings"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/google/uuid"
)

// ── Output types ──────────────────────────────────────────────────────────────

// ImportPreview is the complete dry-run result: what would be created plus any
// conflicts or items that will be skipped.
type ImportPreview struct {
	Users              []models.User             `json:"users"`
	Schedules          []models.Schedule         `json:"schedules"`
	EscalationPolicies []models.EscalationPolicy `json:"escalation_policies"`
	Webhooks           []WebhookMapping          `json:"webhooks"`
	Conflicts          []ConflictItem            `json:"conflicts"`
	Skipped            []SkippedItem             `json:"skipped"`
}

// WebhookMapping describes an OnCall integration and the corresponding Regen
// webhook URL the user should point their monitoring tool at.
type WebhookMapping struct {
	Name        string `json:"name"`
	OnCallType  string `json:"oncall_type"`  // original OnCall integration type
	OldURL      string `json:"old_url"`      // previous OnCall inbound URL
	NewURL      string `json:"new_url"`      // new Regen webhook URL
	RegenSource string `json:"regen_source"` // "prometheus","grafana","cloudwatch","generic"
}

// ConflictItem describes an entity that already exists in Regen and will be
// skipped rather than overwritten during import.
type ConflictItem struct {
	Type   string `json:"type"` // "user", "schedule", "escalation_policy"
	Name   string `json:"name"`
	Reason string `json:"reason"` // human-readable reason
}

// SkippedItem describes an entity from OnCall that was intentionally not
// imported (e.g. unsupported type or empty data).
type SkippedItem struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// ── Transform functions ───────────────────────────────────────────────────────

// TransformAll runs the full transformation pipeline on raw OnCall API data.
// existingEmails, existingScheduleNames, and existingPolicyNames are sets of
// names/emails already present in Regen (used for conflict detection).
// baseURL is the Regen instance base URL used to generate new webhook URLs.
func TransformAll(
	ocUsers []OnCallUser,
	ocSchedules []OnCallSchedule,
	ocShifts []OnCallShift,
	ocChains []OnCallEscalationChain,
	ocSteps []OnCallEscalationStep,
	ocIntegrations []OnCallIntegration,
	existingEmails map[string]bool,
	existingScheduleNames map[string]bool,
	existingPolicyNames map[string]bool,
	baseURL string,
) ImportPreview {
	preview := ImportPreview{}

	// Build an OnCall user ID → display name map for resolving references in
	// shifts and escalation steps. This covers ALL OnCall users (including those
	// that will be skipped due to conflicts) so shift participants are resolved
	// even when the user already exists in Regen.
	ocUserNameByID := buildOncallUserNameIndex(ocUsers)

	// 1. Users
	for _, u := range ocUsers {
		email := strings.ToLower(strings.TrimSpace(u.Email))
		if email == "" {
			preview.Skipped = append(preview.Skipped, SkippedItem{
				Type:   "user",
				Name:   u.Username,
				Reason: "no email address",
			})
			continue
		}
		if existingEmails[email] {
			preview.Conflicts = append(preview.Conflicts, ConflictItem{
				Type:   "user",
				Name:   email,
				Reason: "user with this email already exists in Regen",
			})
			continue
		}
		regenUser := buildUser(u)
		preview.Users = append(preview.Users, regenUser)
		existingEmails[email] = true // prevent duplicates within the import batch
	}

	// 2. Schedules
	shiftByID := indexShiftsByID(ocShifts)
	for _, s := range ocSchedules {
		// skip ical schedules — can't reconstruct layers from an external URL
		if s.Type == "ical" {
			preview.Skipped = append(preview.Skipped, SkippedItem{
				Type:   "schedule",
				Name:   s.Name,
				Reason: "iCal schedule — layers cannot be auto-imported; recreate manually",
			})
			continue
		}
		if existingScheduleNames[s.Name] {
			preview.Conflicts = append(preview.Conflicts, ConflictItem{
				Type:   "schedule",
				Name:   s.Name,
				Reason: "schedule with this name already exists in Regen",
			})
			continue
		}
		regenSched := buildSchedule(s, shiftByID, ocUserNameByID)
		preview.Schedules = append(preview.Schedules, regenSched)
		existingScheduleNames[s.Name] = true
	}

	// 3. Escalation policies
	// Build map from OnCall schedule ID → Regen schedule UUID for schedule references.
	ocScheduleIDToRegenID := buildOnCallScheduleIDMap(ocSchedules, preview.Schedules)

	stepsByChain := groupStepsByChain(ocSteps)
	for _, chain := range ocChains {
		if existingPolicyNames[chain.Name] {
			preview.Conflicts = append(preview.Conflicts, ConflictItem{
				Type:   "escalation_policy",
				Name:   chain.Name,
				Reason: "escalation policy with this name already exists in Regen",
			})
			continue
		}
		steps := stepsByChain[chain.ID]
		regenPolicy := buildEscalationPolicy(chain, steps, ocUserNameByID, ocScheduleIDToRegenID)
		preview.EscalationPolicies = append(preview.EscalationPolicies, regenPolicy)
		existingPolicyNames[chain.Name] = true
	}

	// 4. Webhook mappings
	for _, integ := range ocIntegrations {
		source := mapIntegrationType(integ.Type)
		mapping := WebhookMapping{
			Name:        integ.Name,
			OnCallType:  integ.Type,
			OldURL:      integ.Link,
			NewURL:      strings.TrimRight(baseURL, "/") + "/api/v1/webhooks/" + source,
			RegenSource: source,
		}
		preview.Webhooks = append(preview.Webhooks, mapping)
	}

	return preview
}

// ── Internal builders ─────────────────────────────────────────────────────────

func buildUser(u OnCallUser) models.User {
	name := u.Name
	if name == "" {
		name = u.Username
	}
	role := models.UserRoleMember
	if u.Role == "admin" {
		role = models.UserRoleAdmin
	}
	email := strings.ToLower(strings.TrimSpace(u.Email))

	user := models.User{
		ID:         uuid.New(),
		Email:      email,
		Name:       name,
		Role:       role,
		AuthSource: "local",
		Active:     true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if u.Slack != nil && u.Slack.UserID != "" {
		user.SlackUserID = &u.Slack.UserID
	}
	return user
}

func buildSchedule(
	s OnCallSchedule,
	shiftByID map[string]OnCallShift,
	ocUserNameByID map[string]string,
) models.Schedule {
	tz := s.TimeZone
	if tz == "" {
		tz = "UTC"
	}

	schedID := uuid.New()
	sched := models.Schedule{
		ID:        schedID,
		Name:      s.Name,
		Timezone:  tz,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Map each shift ID → layer, ordered by shift level
	type layerEntry struct {
		level int
		layer models.ScheduleLayer
	}
	var layers []layerEntry

	for i, shiftID := range s.Shifts {
		shift, ok := shiftByID[shiftID]
		if !ok {
			continue
		}
		// Only import rolling_users and recurrent_event shifts; skip overrides.
		if shift.Type == "override" {
			continue
		}

		layer := buildLayer(schedID, shift, i, ocUserNameByID)
		layers = append(layers, layerEntry{level: shift.Level, layer: layer})
	}

	// Sort by original level so primary layer is first.
	sort.Slice(layers, func(i, j int) bool {
		return layers[i].level < layers[j].level
	})
	for i := range layers {
		layers[i].layer.OrderIndex = i
		sched.Layers = append(sched.Layers, layers[i].layer)
	}

	return sched
}

func buildLayer(
	schedID uuid.UUID,
	shift OnCallShift,
	orderIndex int,
	ocUserNameByID map[string]string,
) models.ScheduleLayer {
	name := shift.Name
	if name == "" {
		name = "Layer"
	}

	rotType := mapRotationType(shift.Frequency)

	rotStart := time.Now()
	if shift.RotationStart != "" {
		if t, err := time.Parse("2006-01-02T15:04:05", shift.RotationStart); err == nil {
			rotStart = t
		}
	} else if shift.Start != "" {
		if t, err := time.Parse("2006-01-02T15:04:05", shift.Start); err == nil {
			rotStart = t
		}
	}

	duration := shift.Duration
	if duration == 0 {
		// Default: one week in seconds
		duration = 7 * 24 * 3600
	}

	layer := models.ScheduleLayer{
		ID:                   uuid.New(),
		ScheduleID:           schedID,
		Name:                 name,
		OrderIndex:           orderIndex,
		RotationType:         rotType,
		RotationStart:        rotStart,
		ShiftDurationSeconds: duration,
		CreatedAt:            time.Now(),
	}

	// Build participant list from rolling_users (preferred) or users.
	participants := buildParticipants(layer.ID, shift, ocUserNameByID)
	layer.Participants = participants

	return layer
}

func buildParticipants(
	layerID uuid.UUID,
	shift OnCallShift,
	ocUserNameByID map[string]string,
) []models.ScheduleParticipant {
	var participants []models.ScheduleParticipant
	order := 0

	addUser := func(uid string) {
		name := uid // fall back to OnCall user ID if not resolvable
		if resolved, ok := ocUserNameByID[uid]; ok && resolved != "" {
			name = resolved
		}
		participants = append(participants, models.ScheduleParticipant{
			ID:         uuid.New(),
			LayerID:    layerID,
			UserName:   name,
			OrderIndex: order,
			CreatedAt:  time.Now(),
		})
		order++
	}

	if len(shift.RollingUsers) > 0 {
		// Rolling users: flatten the groups in rotation order.
		for _, group := range shift.RollingUsers {
			for _, uid := range group {
				addUser(uid)
			}
		}
	} else {
		for _, uid := range shift.Users {
			addUser(uid)
		}
	}
	return participants
}

func buildEscalationPolicy(
	chain OnCallEscalationChain,
	steps []OnCallEscalationStep,
	ocUserNameByID map[string]string,
	ocScheduleIDToRegenID map[string]uuid.UUID,
) models.EscalationPolicy {
	policy := models.EscalationPolicy{
		ID:          uuid.New(),
		Name:        chain.Name,
		Description: "Imported from Grafana OnCall",
		Enabled:     true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Sort steps by step index before building tiers.
	sort.Slice(steps, func(i, j int) bool {
		return steps[i].Step < steps[j].Step
	})

	tierIndex := 0
	for _, step := range steps {
		tier := buildTier(policy.ID, step, tierIndex, ocUserNameByID, ocScheduleIDToRegenID)
		if tier == nil {
			continue // skip unsupported step types
		}
		policy.Tiers = append(policy.Tiers, *tier)
		tierIndex++
	}

	return policy
}

func buildTier(
	policyID uuid.UUID,
	step OnCallEscalationStep,
	tierIndex int,
	ocUserNameByID map[string]string,
	ocScheduleIDToRegenID map[string]uuid.UUID,
) *models.EscalationTier {
	timeout := 300 // default 5 min
	if step.Duration != nil && *step.Duration > 0 {
		timeout = *step.Duration
	}

	tier := &models.EscalationTier{
		ID:             uuid.New(),
		PolicyID:       policyID,
		TierIndex:      tierIndex,
		TimeoutSeconds: timeout,
		CreatedAt:      time.Now(),
	}

	switch step.Type {
	case StepTypeWait:
		// wait steps don't notify anyone — skip; the timeout is absorbed into
		// the next notify step's TimeoutSeconds if we wanted to be precise,
		// but for a clean import we simply skip pure wait steps.
		return nil

	case StepTypeNotifyPersons, StepTypeNotifyPersonNextEachTime:
		persons := step.PersonsToNotify
		if len(persons) == 0 {
			persons = step.PersonsNextTime
		}
		if len(persons) == 0 {
			return nil
		}
		// Resolve OnCall user IDs to display names.
		userNames := make([]string, 0, len(persons))
		for _, uid := range persons {
			name := uid
			if resolved, ok := ocUserNameByID[uid]; ok && resolved != "" {
				name = resolved
			}
			userNames = append(userNames, name)
		}
		tier.TargetType = models.EscalationTargetUsers
		tier.UserNames = models.JSONBArray(userNames)

	case StepTypeNotifyOnCallFromSchedule:
		if step.Schedule == "" {
			return nil
		}
		if regenID, ok := ocScheduleIDToRegenID[step.Schedule]; ok {
			tier.TargetType = models.EscalationTargetSchedule
			tier.ScheduleID = &regenID
		} else {
			// Schedule wasn't imported (conflict/skip) — use users fallback.
			return nil
		}

	default:
		// resolve_incident, notify_whole_channel, etc. — not mappable.
		return nil
	}

	return tier
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func indexShiftsByID(shifts []OnCallShift) map[string]OnCallShift {
	m := make(map[string]OnCallShift, len(shifts))
	for _, s := range shifts {
		m[s.ID] = s
	}
	return m
}

func groupStepsByChain(steps []OnCallEscalationStep) map[string][]OnCallEscalationStep {
	m := make(map[string][]OnCallEscalationStep)
	for _, s := range steps {
		m[s.EscalationChain] = append(m[s.EscalationChain], s)
	}
	return m
}

func buildOnCallScheduleIDMap(
	ocSchedules []OnCallSchedule,
	regenSchedules []models.Schedule,
) map[string]uuid.UUID {
	// Build name → regen ID index
	nameToID := make(map[string]uuid.UUID, len(regenSchedules))
	for _, s := range regenSchedules {
		nameToID[s.Name] = s.ID
	}
	// Map OnCall schedule ID → regen ID via name matching
	m := make(map[string]uuid.UUID)
	for _, oc := range ocSchedules {
		if id, ok := nameToID[oc.Name]; ok {
			m[oc.ID] = id
		}
	}
	return m
}

func mapRotationType(frequency string) models.RotationType {
	switch strings.ToLower(frequency) {
	case "daily":
		return models.RotationTypeDaily
	case "weekly":
		return models.RotationTypeWeekly
	default:
		return models.RotationTypeWeekly // safe default
	}
}

// buildOncallUserNameIndex builds a map from OnCall user ID (pk) → display name.
// Falls back to email if name and username are both empty.
func buildOncallUserNameIndex(users []OnCallUser) map[string]string {
	m := make(map[string]string, len(users))
	for _, u := range users {
		name := u.Name
		if name == "" {
			name = u.Username
		}
		if name == "" {
			name = u.Email
		}
		m[u.ID] = name
	}
	return m
}

// mapIntegrationType maps an OnCall integration type to the closest Regen
// webhook source name.
func mapIntegrationType(oncallType string) string {
	switch strings.ToLower(oncallType) {
	case "alertmanager":
		return "prometheus"
	case "grafana":
		return "grafana"
	case "cloudwatch":
		return "cloudwatch"
	default:
		return "generic"
	}
}
