package oncall

import (
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/models"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func emptyExisting() (map[string]bool, map[string]bool, map[string]bool) {
	return map[string]bool{}, map[string]bool{}, map[string]bool{}
}

func ptr[T any](v T) *T { return &v }

// ── Users ─────────────────────────────────────────────────────────────────────

func TestTransformAll_Users_Basic(t *testing.T) {
	users := []OnCallUser{
		{ID: "u1", Email: "alice@example.com", Name: "Alice", Role: "admin"},
		{ID: "u2", Email: "bob@example.com", Username: "bobsmith", Role: "user"},
	}
	emails, schedNames, policyNames := emptyExisting()
	p := TransformAll(users, nil, nil, nil, nil, nil, emails, schedNames, policyNames, "http://regen.example.com")

	if len(p.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(p.Users))
	}
	if len(p.Conflicts) != 0 {
		t.Errorf("expected no conflicts, got %d", len(p.Conflicts))
	}
	if len(p.Skipped) != 0 {
		t.Errorf("expected no skipped, got %d", len(p.Skipped))
	}

	alice := p.Users[0]
	if alice.Email != "alice@example.com" {
		t.Errorf("expected alice@example.com, got %s", alice.Email)
	}
	if alice.Role != models.UserRoleAdmin {
		t.Errorf("expected admin role for alice, got %s", alice.Role)
	}
	if alice.AuthSource != "local" {
		t.Errorf("expected auth_source=local, got %s", alice.AuthSource)
	}
	if !alice.Active {
		t.Error("expected user to be active")
	}

	// Bob has no Name — should fall back to Username
	bob := p.Users[1]
	if bob.Name != "bobsmith" {
		t.Errorf("expected name=bobsmith (from username), got %s", bob.Name)
	}
	if bob.Role != models.UserRoleMember {
		t.Errorf("expected member role for bob, got %s", bob.Role)
	}
}

func TestTransformAll_Users_EmailNormalised(t *testing.T) {
	users := []OnCallUser{
		{ID: "u1", Email: "  ALICE@Example.COM  ", Name: "Alice", Role: "user"},
	}
	emails, schedNames, policyNames := emptyExisting()
	p := TransformAll(users, nil, nil, nil, nil, nil, emails, schedNames, policyNames, "http://regen.example.com")

	if len(p.Users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(p.Users))
	}
	if p.Users[0].Email != "alice@example.com" {
		t.Errorf("expected lowercased trimmed email, got %q", p.Users[0].Email)
	}
}

func TestTransformAll_Users_NoEmail_Skipped(t *testing.T) {
	users := []OnCallUser{
		{ID: "u1", Email: "", Username: "ghost", Role: "user"},
	}
	emails, schedNames, policyNames := emptyExisting()
	p := TransformAll(users, nil, nil, nil, nil, nil, emails, schedNames, policyNames, "http://regen.example.com")

	if len(p.Users) != 0 {
		t.Errorf("expected 0 users, got %d", len(p.Users))
	}
	if len(p.Skipped) != 1 {
		t.Fatalf("expected 1 skipped item, got %d", len(p.Skipped))
	}
	if p.Skipped[0].Type != "user" {
		t.Errorf("expected type=user, got %s", p.Skipped[0].Type)
	}
}

func TestTransformAll_Users_ExistingEmail_Conflict(t *testing.T) {
	users := []OnCallUser{
		{ID: "u1", Email: "alice@example.com", Name: "Alice", Role: "user"},
	}
	existing := map[string]bool{"alice@example.com": true}
	p := TransformAll(users, nil, nil, nil, nil, nil, existing, map[string]bool{}, map[string]bool{}, "http://regen.example.com")

	if len(p.Users) != 0 {
		t.Errorf("expected 0 users (conflict), got %d", len(p.Users))
	}
	if len(p.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(p.Conflicts))
	}
	if p.Conflicts[0].Type != "user" {
		t.Errorf("expected type=user in conflict, got %s", p.Conflicts[0].Type)
	}
}

func TestTransformAll_Users_DuplicateWithinBatch(t *testing.T) {
	// Two OnCall users with the same email — only the first should be imported.
	users := []OnCallUser{
		{ID: "u1", Email: "alice@example.com", Name: "Alice", Role: "user"},
		{ID: "u2", Email: "alice@example.com", Name: "Alice2", Role: "admin"},
	}
	emails, schedNames, policyNames := emptyExisting()
	p := TransformAll(users, nil, nil, nil, nil, nil, emails, schedNames, policyNames, "http://regen.example.com")

	if len(p.Users) != 1 {
		t.Errorf("expected 1 user (dedup), got %d", len(p.Users))
	}
	if len(p.Conflicts) != 1 {
		t.Errorf("expected 1 conflict for duplicate, got %d", len(p.Conflicts))
	}
}

func TestTransformAll_Users_SlackID(t *testing.T) {
	users := []OnCallUser{
		{ID: "u1", Email: "alice@example.com", Name: "Alice", Role: "user",
			Slack: &OnCallSlackRef{UserID: "U012AB3CD"}},
	}
	emails, schedNames, policyNames := emptyExisting()
	p := TransformAll(users, nil, nil, nil, nil, nil, emails, schedNames, policyNames, "http://regen.example.com")

	if len(p.Users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(p.Users))
	}
	if p.Users[0].SlackUserID == nil || *p.Users[0].SlackUserID != "U012AB3CD" {
		t.Errorf("expected slack_user_id=U012AB3CD, got %v", p.Users[0].SlackUserID)
	}
}

// ── Schedules ─────────────────────────────────────────────────────────────────

func TestTransformAll_Schedule_Basic(t *testing.T) {
	schedules := []OnCallSchedule{
		{
			ID:       "sched1",
			Name:     "Primary On-Call",
			Type:     "web",
			TimeZone: "America/New_York",
			Shifts:   []string{"shift1"},
		},
	}
	shifts := []OnCallShift{
		{
			ID:            "shift1",
			Name:          "Primary Layer",
			Type:          "rolling_users",
			Level:         0,
			Frequency:     "weekly",
			Duration:      604800, // 1 week
			RotationStart: "2026-01-01T00:00:00",
			RollingUsers:  [][]string{{"u1"}, {"u2"}},
		},
	}
	users := []OnCallUser{
		{ID: "u1", Email: "alice@example.com", Name: "Alice"},
		{ID: "u2", Email: "bob@example.com", Name: "Bob"},
	}
	emails, schedNames, policyNames := emptyExisting()
	p := TransformAll(users, schedules, shifts, nil, nil, nil, emails, schedNames, policyNames, "http://regen.example.com")

	if len(p.Schedules) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(p.Schedules))
	}
	s := p.Schedules[0]
	if s.Name != "Primary On-Call" {
		t.Errorf("expected name=Primary On-Call, got %s", s.Name)
	}
	if s.Timezone != "America/New_York" {
		t.Errorf("expected timezone=America/New_York, got %s", s.Timezone)
	}
	if len(s.Layers) != 1 {
		t.Fatalf("expected 1 layer, got %d", len(s.Layers))
	}
	layer := s.Layers[0]
	if layer.RotationType != models.RotationTypeWeekly {
		t.Errorf("expected weekly rotation, got %s", layer.RotationType)
	}
	if layer.ShiftDurationSeconds != 604800 {
		t.Errorf("expected duration=604800, got %d", layer.ShiftDurationSeconds)
	}
	// Participants should be resolved to names, not IDs
	if len(layer.Participants) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(layer.Participants))
	}
	if layer.Participants[0].UserName != "Alice" {
		t.Errorf("expected participant name=Alice (resolved), got %s", layer.Participants[0].UserName)
	}
	if layer.Participants[1].UserName != "Bob" {
		t.Errorf("expected participant name=Bob (resolved), got %s", layer.Participants[1].UserName)
	}
}

func TestTransformAll_Schedule_MissingTimezone_DefaultsUTC(t *testing.T) {
	schedules := []OnCallSchedule{
		{ID: "s1", Name: "No TZ", Type: "web", TimeZone: "", Shifts: nil},
	}
	emails, schedNames, policyNames := emptyExisting()
	p := TransformAll(nil, schedules, nil, nil, nil, nil, emails, schedNames, policyNames, "http://regen.example.com")

	if len(p.Schedules) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(p.Schedules))
	}
	if p.Schedules[0].Timezone != "UTC" {
		t.Errorf("expected timezone=UTC, got %s", p.Schedules[0].Timezone)
	}
}

func TestTransformAll_Schedule_ICalSkipped(t *testing.T) {
	schedules := []OnCallSchedule{
		{ID: "s1", Name: "iCal Schedule", Type: "ical", TimeZone: "UTC"},
	}
	emails, schedNames, policyNames := emptyExisting()
	p := TransformAll(nil, schedules, nil, nil, nil, nil, emails, schedNames, policyNames, "http://regen.example.com")

	if len(p.Schedules) != 0 {
		t.Errorf("expected 0 schedules (ical skipped), got %d", len(p.Schedules))
	}
	if len(p.Skipped) != 1 {
		t.Fatalf("expected 1 skipped item, got %d", len(p.Skipped))
	}
	if p.Skipped[0].Type != "schedule" {
		t.Errorf("expected type=schedule, got %s", p.Skipped[0].Type)
	}
}

func TestTransformAll_Schedule_ExistingName_Conflict(t *testing.T) {
	schedules := []OnCallSchedule{
		{ID: "s1", Name: "Primary", Type: "web", TimeZone: "UTC"},
	}
	existing := map[string]bool{"Primary": true}
	p := TransformAll(nil, schedules, nil, nil, nil, nil, map[string]bool{}, existing, map[string]bool{}, "http://regen.example.com")

	if len(p.Schedules) != 0 {
		t.Errorf("expected 0 schedules (conflict), got %d", len(p.Schedules))
	}
	if len(p.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(p.Conflicts))
	}
	if p.Conflicts[0].Type != "schedule" {
		t.Errorf("expected type=schedule in conflict, got %s", p.Conflicts[0].Type)
	}
}

func TestTransformAll_Schedule_OverrideShiftSkipped(t *testing.T) {
	schedules := []OnCallSchedule{
		{ID: "s1", Name: "With Override", Type: "web", TimeZone: "UTC", Shifts: []string{"shift1", "shift2"}},
	}
	shifts := []OnCallShift{
		{ID: "shift1", Name: "Regular", Type: "rolling_users", Level: 0, Frequency: "weekly", Duration: 604800, Users: []string{"u1"}},
		{ID: "shift2", Name: "Override", Type: "override", Level: 0, Users: []string{"u2"}},
	}
	users := []OnCallUser{{ID: "u1", Email: "alice@example.com", Name: "Alice"}}
	emails, schedNames, policyNames := emptyExisting()
	p := TransformAll(users, schedules, shifts, nil, nil, nil, emails, schedNames, policyNames, "http://regen.example.com")

	if len(p.Schedules) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(p.Schedules))
	}
	// Only the regular shift should become a layer; override is skipped
	if len(p.Schedules[0].Layers) != 1 {
		t.Errorf("expected 1 layer (override skipped), got %d", len(p.Schedules[0].Layers))
	}
}

func TestTransformAll_Schedule_UnresolvedUserID_FallsBackToID(t *testing.T) {
	// User u99 exists in OnCall but has no email — they were skipped.
	// The participant name should fall back to their raw ID.
	schedules := []OnCallSchedule{
		{ID: "s1", Name: "Sched", Type: "web", TimeZone: "UTC", Shifts: []string{"shift1"}},
	}
	shifts := []OnCallShift{
		{ID: "shift1", Name: "L1", Type: "rolling_users", Level: 0, Frequency: "weekly", Duration: 604800,
			RollingUsers: [][]string{{"u99"}}},
	}
	// u99 has no email so it won't appear in ocUsers — name index won't have it
	emails, schedNames, policyNames := emptyExisting()
	p := TransformAll(nil, schedules, shifts, nil, nil, nil, emails, schedNames, policyNames, "http://regen.example.com")

	participants := p.Schedules[0].Layers[0].Participants
	if len(participants) != 1 {
		t.Fatalf("expected 1 participant, got %d", len(participants))
	}
	if participants[0].UserName != "u99" {
		t.Errorf("expected fallback to raw ID 'u99', got %s", participants[0].UserName)
	}
}

func TestTransformAll_Schedule_DailyRotation(t *testing.T) {
	schedules := []OnCallSchedule{
		{ID: "s1", Name: "Daily", Type: "web", TimeZone: "UTC", Shifts: []string{"shift1"}},
	}
	shifts := []OnCallShift{
		{ID: "shift1", Name: "Daily Layer", Type: "rolling_users", Level: 0,
			Frequency: "daily", Duration: 86400, Users: []string{"u1"}},
	}
	users := []OnCallUser{{ID: "u1", Email: "alice@example.com", Name: "Alice"}}
	emails, schedNames, policyNames := emptyExisting()
	p := TransformAll(users, schedules, shifts, nil, nil, nil, emails, schedNames, policyNames, "http://regen.example.com")

	if p.Schedules[0].Layers[0].RotationType != models.RotationTypeDaily {
		t.Errorf("expected daily rotation, got %s", p.Schedules[0].Layers[0].RotationType)
	}
}

func TestTransformAll_Schedule_ZeroDuration_DefaultsOneWeek(t *testing.T) {
	schedules := []OnCallSchedule{
		{ID: "s1", Name: "Sched", Type: "web", TimeZone: "UTC", Shifts: []string{"shift1"}},
	}
	shifts := []OnCallShift{
		{ID: "shift1", Name: "L1", Type: "rolling_users", Level: 0, Frequency: "weekly",
			Duration: 0, Users: []string{"u1"}},
	}
	users := []OnCallUser{{ID: "u1", Email: "alice@example.com", Name: "Alice"}}
	emails, schedNames, policyNames := emptyExisting()
	p := TransformAll(users, schedules, shifts, nil, nil, nil, emails, schedNames, policyNames, "http://regen.example.com")

	if p.Schedules[0].Layers[0].ShiftDurationSeconds != 7*24*3600 {
		t.Errorf("expected default 1 week duration, got %d", p.Schedules[0].Layers[0].ShiftDurationSeconds)
	}
}

func TestTransformAll_Schedule_MultiLayerOrdering(t *testing.T) {
	// Two shifts with levels 1 and 0 — should be sorted so level 0 comes first.
	schedules := []OnCallSchedule{
		{ID: "s1", Name: "Multi", Type: "web", TimeZone: "UTC", Shifts: []string{"shift_secondary", "shift_primary"}},
	}
	shifts := []OnCallShift{
		{ID: "shift_secondary", Name: "Secondary", Type: "rolling_users", Level: 1, Frequency: "weekly", Duration: 604800, Users: []string{"u2"}},
		{ID: "shift_primary", Name: "Primary", Type: "rolling_users", Level: 0, Frequency: "weekly", Duration: 604800, Users: []string{"u1"}},
	}
	users := []OnCallUser{
		{ID: "u1", Email: "alice@example.com", Name: "Alice"},
		{ID: "u2", Email: "bob@example.com", Name: "Bob"},
	}
	emails, schedNames, policyNames := emptyExisting()
	p := TransformAll(users, schedules, shifts, nil, nil, nil, emails, schedNames, policyNames, "http://regen.example.com")

	layers := p.Schedules[0].Layers
	if len(layers) != 2 {
		t.Fatalf("expected 2 layers, got %d", len(layers))
	}
	if layers[0].Name != "Primary" {
		t.Errorf("expected Primary layer first (level 0), got %s", layers[0].Name)
	}
	if layers[1].Name != "Secondary" {
		t.Errorf("expected Secondary layer second (level 1), got %s", layers[1].Name)
	}
	if layers[0].OrderIndex != 0 || layers[1].OrderIndex != 1 {
		t.Errorf("layer order indices wrong: %d, %d", layers[0].OrderIndex, layers[1].OrderIndex)
	}
}

// ── Escalation policies ───────────────────────────────────────────────────────

func TestTransformAll_EscalationPolicy_Basic(t *testing.T) {
	chains := []OnCallEscalationChain{
		{ID: "chain1", Name: "SEV1 On-Call"},
	}
	steps := []OnCallEscalationStep{
		{ID: "step1", Step: 0, EscalationChain: "chain1", Type: StepTypeNotifyPersons,
			PersonsToNotify: []string{"u1"}, Duration: ptr(300)},
	}
	users := []OnCallUser{{ID: "u1", Email: "alice@example.com", Name: "Alice"}}
	emails, schedNames, policyNames := emptyExisting()
	p := TransformAll(users, nil, nil, chains, steps, nil, emails, schedNames, policyNames, "http://regen.example.com")

	if len(p.EscalationPolicies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(p.EscalationPolicies))
	}
	policy := p.EscalationPolicies[0]
	if policy.Name != "SEV1 On-Call" {
		t.Errorf("expected name=SEV1 On-Call, got %s", policy.Name)
	}
	if !policy.Enabled {
		t.Error("expected policy to be enabled")
	}
	if len(policy.Tiers) != 1 {
		t.Fatalf("expected 1 tier, got %d", len(policy.Tiers))
	}
	tier := policy.Tiers[0]
	if tier.TargetType != models.EscalationTargetUsers {
		t.Errorf("expected target_type=users, got %s", tier.TargetType)
	}
	if len(tier.UserNames) != 1 || tier.UserNames[0] != "Alice" {
		t.Errorf("expected user_names=[Alice] (resolved), got %v", tier.UserNames)
	}
	if tier.TimeoutSeconds != 300 {
		t.Errorf("expected timeout=300, got %d", tier.TimeoutSeconds)
	}
}

func TestTransformAll_EscalationPolicy_WaitStepSkipped(t *testing.T) {
	chains := []OnCallEscalationChain{{ID: "chain1", Name: "Policy"}}
	fiveMin := 300
	steps := []OnCallEscalationStep{
		{ID: "s1", Step: 0, EscalationChain: "chain1", Type: StepTypeWait, Duration: &fiveMin},
		{ID: "s2", Step: 1, EscalationChain: "chain1", Type: StepTypeNotifyPersons, PersonsToNotify: []string{"u1"}},
	}
	users := []OnCallUser{{ID: "u1", Email: "alice@example.com", Name: "Alice"}}
	emails, schedNames, policyNames := emptyExisting()
	p := TransformAll(users, nil, nil, chains, steps, nil, emails, schedNames, policyNames, "http://regen.example.com")

	if len(p.EscalationPolicies[0].Tiers) != 1 {
		t.Errorf("expected 1 tier (wait skipped), got %d", len(p.EscalationPolicies[0].Tiers))
	}
}

func TestTransformAll_EscalationPolicy_UnsupportedStepsSkipped(t *testing.T) {
	chains := []OnCallEscalationChain{{ID: "chain1", Name: "Policy"}}
	steps := []OnCallEscalationStep{
		{ID: "s1", Step: 0, EscalationChain: "chain1", Type: StepTypeResolveIncident},
		{ID: "s2", Step: 1, EscalationChain: "chain1", Type: StepTypeNotifyWholedTeam},
		{ID: "s3", Step: 2, EscalationChain: "chain1", Type: StepTypeNotifyPersons, PersonsToNotify: []string{"u1"}},
	}
	users := []OnCallUser{{ID: "u1", Email: "alice@example.com", Name: "Alice"}}
	emails, schedNames, policyNames := emptyExisting()
	p := TransformAll(users, nil, nil, chains, steps, nil, emails, schedNames, policyNames, "http://regen.example.com")

	// Only the notify_persons step should produce a tier
	if len(p.EscalationPolicies[0].Tiers) != 1 {
		t.Errorf("expected 1 tier (unsupported steps skipped), got %d", len(p.EscalationPolicies[0].Tiers))
	}
}

func TestTransformAll_EscalationPolicy_NotifyOnCallFromSchedule(t *testing.T) {
	schedules := []OnCallSchedule{
		{ID: "sched1", Name: "Primary", Type: "web", TimeZone: "UTC", Shifts: nil},
	}
	chains := []OnCallEscalationChain{{ID: "chain1", Name: "Policy"}}
	steps := []OnCallEscalationStep{
		{ID: "s1", Step: 0, EscalationChain: "chain1", Type: StepTypeNotifyOnCallFromSchedule, Schedule: "sched1"},
	}
	emails, schedNames, policyNames := emptyExisting()
	p := TransformAll(nil, schedules, nil, chains, steps, nil, emails, schedNames, policyNames, "http://regen.example.com")

	if len(p.EscalationPolicies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(p.EscalationPolicies))
	}
	tier := p.EscalationPolicies[0].Tiers[0]
	if tier.TargetType != models.EscalationTargetSchedule {
		t.Errorf("expected target_type=schedule, got %s", tier.TargetType)
	}
	if tier.ScheduleID == nil {
		t.Error("expected schedule_id to be set")
	}
	// ScheduleID should match the imported schedule's Regen UUID
	if *tier.ScheduleID != p.Schedules[0].ID {
		t.Errorf("schedule_id mismatch: tier has %s, schedule has %s", *tier.ScheduleID, p.Schedules[0].ID)
	}
}

func TestTransformAll_EscalationPolicy_NotifyOnCallFromSchedule_ScheduleNotImported(t *testing.T) {
	// Chain references a schedule that was skipped (conflict) — tier should be dropped.
	chains := []OnCallEscalationChain{{ID: "chain1", Name: "Policy"}}
	steps := []OnCallEscalationStep{
		{ID: "s1", Step: 0, EscalationChain: "chain1", Type: StepTypeNotifyOnCallFromSchedule, Schedule: "sched_missing"},
	}
	emails, schedNames, policyNames := emptyExisting()
	p := TransformAll(nil, nil, nil, chains, steps, nil, emails, schedNames, policyNames, "http://regen.example.com")

	if len(p.EscalationPolicies[0].Tiers) != 0 {
		t.Errorf("expected 0 tiers (schedule not imported), got %d", len(p.EscalationPolicies[0].Tiers))
	}
}

func TestTransformAll_EscalationPolicy_MultipleStepsOrdered(t *testing.T) {
	chains := []OnCallEscalationChain{{ID: "chain1", Name: "Multi"}}
	steps := []OnCallEscalationStep{
		// Deliberately out of order to test sorting
		{ID: "s2", Step: 1, EscalationChain: "chain1", Type: StepTypeNotifyPersons, PersonsToNotify: []string{"u2"}, Duration: ptr(600)},
		{ID: "s1", Step: 0, EscalationChain: "chain1", Type: StepTypeNotifyPersons, PersonsToNotify: []string{"u1"}, Duration: ptr(300)},
	}
	users := []OnCallUser{
		{ID: "u1", Email: "alice@example.com", Name: "Alice"},
		{ID: "u2", Email: "bob@example.com", Name: "Bob"},
	}
	emails, schedNames, policyNames := emptyExisting()
	p := TransformAll(users, nil, nil, chains, steps, nil, emails, schedNames, policyNames, "http://regen.example.com")

	tiers := p.EscalationPolicies[0].Tiers
	if len(tiers) != 2 {
		t.Fatalf("expected 2 tiers, got %d", len(tiers))
	}
	if tiers[0].TimeoutSeconds != 300 {
		t.Errorf("expected first tier timeout=300, got %d", tiers[0].TimeoutSeconds)
	}
	if tiers[1].TimeoutSeconds != 600 {
		t.Errorf("expected second tier timeout=600, got %d", tiers[1].TimeoutSeconds)
	}
	if tiers[0].UserNames[0] != "Alice" || tiers[1].UserNames[0] != "Bob" {
		t.Errorf("tier order wrong: first=%v second=%v", tiers[0].UserNames, tiers[1].UserNames)
	}
}

func TestTransformAll_EscalationPolicy_DefaultTimeout(t *testing.T) {
	chains := []OnCallEscalationChain{{ID: "chain1", Name: "Policy"}}
	steps := []OnCallEscalationStep{
		// Duration is nil — should default to 300s
		{ID: "s1", Step: 0, EscalationChain: "chain1", Type: StepTypeNotifyPersons, PersonsToNotify: []string{"u1"}},
	}
	users := []OnCallUser{{ID: "u1", Email: "alice@example.com", Name: "Alice"}}
	emails, schedNames, policyNames := emptyExisting()
	p := TransformAll(users, nil, nil, chains, steps, nil, emails, schedNames, policyNames, "http://regen.example.com")

	if p.EscalationPolicies[0].Tiers[0].TimeoutSeconds != 300 {
		t.Errorf("expected default timeout=300, got %d", p.EscalationPolicies[0].Tiers[0].TimeoutSeconds)
	}
}

func TestTransformAll_EscalationPolicy_ExistingName_Conflict(t *testing.T) {
	chains := []OnCallEscalationChain{{ID: "chain1", Name: "Existing Policy"}}
	existing := map[string]bool{"Existing Policy": true}
	p := TransformAll(nil, nil, nil, chains, nil, nil, map[string]bool{}, map[string]bool{}, existing, "http://regen.example.com")

	if len(p.EscalationPolicies) != 0 {
		t.Errorf("expected 0 policies (conflict), got %d", len(p.EscalationPolicies))
	}
	if len(p.Conflicts) != 1 || p.Conflicts[0].Type != "escalation_policy" {
		t.Errorf("expected 1 escalation_policy conflict, got %v", p.Conflicts)
	}
}

func TestTransformAll_EscalationPolicy_EmptyPersonsList_TierDropped(t *testing.T) {
	chains := []OnCallEscalationChain{{ID: "chain1", Name: "Policy"}}
	steps := []OnCallEscalationStep{
		{ID: "s1", Step: 0, EscalationChain: "chain1", Type: StepTypeNotifyPersons,
			PersonsToNotify: []string{}, PersonsNextTime: []string{}},
	}
	emails, schedNames, policyNames := emptyExisting()
	p := TransformAll(nil, nil, nil, chains, steps, nil, emails, schedNames, policyNames, "http://regen.example.com")

	if len(p.EscalationPolicies[0].Tiers) != 0 {
		t.Errorf("expected 0 tiers (empty persons list), got %d", len(p.EscalationPolicies[0].Tiers))
	}
}

// ── Webhooks ──────────────────────────────────────────────────────────────────

func TestTransformAll_Webhooks_TypeMapping(t *testing.T) {
	integrations := []OnCallIntegration{
		{ID: "i1", Name: "Prometheus", Type: "alertmanager", Link: "https://oncall.example.com/old/prom"},
		{ID: "i2", Name: "Grafana", Type: "grafana", Link: "https://oncall.example.com/old/grafana"},
		{ID: "i3", Name: "CloudWatch", Type: "cloudwatch", Link: "https://oncall.example.com/old/cw"},
		{ID: "i4", Name: "Custom", Type: "webhook", Link: "https://oncall.example.com/old/custom"},
	}
	emails, schedNames, policyNames := emptyExisting()
	p := TransformAll(nil, nil, nil, nil, nil, integrations, emails, schedNames, policyNames, "https://regen.example.com")

	if len(p.Webhooks) != 4 {
		t.Fatalf("expected 4 webhook mappings, got %d", len(p.Webhooks))
	}

	cases := []struct {
		name         string
		regenSource  string
		newURLSuffix string
	}{
		{"Prometheus", "prometheus", "/api/v1/webhooks/prometheus"},
		{"Grafana", "grafana", "/api/v1/webhooks/grafana"},
		{"CloudWatch", "cloudwatch", "/api/v1/webhooks/cloudwatch"},
		{"Custom", "generic", "/api/v1/webhooks/generic"},
	}

	for i, tc := range cases {
		w := p.Webhooks[i]
		if w.RegenSource != tc.regenSource {
			t.Errorf("[%s] expected regen_source=%s, got %s", tc.name, tc.regenSource, w.RegenSource)
		}
		if w.NewURL != "https://regen.example.com"+tc.newURLSuffix {
			t.Errorf("[%s] expected new_url=https://regen.example.com%s, got %s", tc.name, tc.newURLSuffix, w.NewURL)
		}
		if w.OldURL == "" {
			t.Errorf("[%s] expected old_url to be set", tc.name)
		}
	}
}

func TestTransformAll_Webhooks_BaseURLTrailingSlash(t *testing.T) {
	integrations := []OnCallIntegration{
		{ID: "i1", Name: "Prom", Type: "alertmanager", Link: "https://oncall.example.com/old"},
	}
	emails, schedNames, policyNames := emptyExisting()
	p := TransformAll(nil, nil, nil, nil, nil, integrations, emails, schedNames, policyNames, "https://regen.example.com/")

	if p.Webhooks[0].NewURL != "https://regen.example.com/api/v1/webhooks/prometheus" {
		t.Errorf("unexpected new URL with trailing slash: %s", p.Webhooks[0].NewURL)
	}
}

// ── Empty input ───────────────────────────────────────────────────────────────

func TestTransformAll_AllEmpty(t *testing.T) {
	emails, schedNames, policyNames := emptyExisting()
	p := TransformAll(nil, nil, nil, nil, nil, nil, emails, schedNames, policyNames, "http://regen.example.com")

	if len(p.Users) != 0 || len(p.Schedules) != 0 || len(p.EscalationPolicies) != 0 || len(p.Webhooks) != 0 {
		t.Error("expected all empty slices for empty input")
	}
	if len(p.Conflicts) != 0 || len(p.Skipped) != 0 {
		t.Error("expected no conflicts or skipped for empty input")
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func TestMapIntegrationType(t *testing.T) {
	cases := []struct{ in, want string }{
		{"alertmanager", "prometheus"},
		{"ALERTMANAGER", "prometheus"},
		{"grafana", "grafana"},
		{"Grafana", "grafana"},
		{"cloudwatch", "cloudwatch"},
		{"webhook", "generic"},
		{"opsgenie", "generic"},
		{"", "generic"},
	}
	for _, tc := range cases {
		got := mapIntegrationType(tc.in)
		if got != tc.want {
			t.Errorf("mapIntegrationType(%q): want %q, got %q", tc.in, tc.want, got)
		}
	}
}

func TestMapRotationType(t *testing.T) {
	cases := []struct {
		in   string
		want models.RotationType
	}{
		{"weekly", models.RotationTypeWeekly},
		{"WEEKLY", models.RotationTypeWeekly},
		{"daily", models.RotationTypeDaily},
		{"DAILY", models.RotationTypeDaily},
		{"hourly", models.RotationTypeWeekly}, // unknown → default weekly
		{"", models.RotationTypeWeekly},
	}
	for _, tc := range cases {
		got := mapRotationType(tc.in)
		if got != tc.want {
			t.Errorf("mapRotationType(%q): want %q, got %q", tc.in, tc.want, got)
		}
	}
}

func TestBuildOncallUserNameIndex(t *testing.T) {
	users := []OnCallUser{
		{ID: "u1", Name: "Alice", Username: "alice", Email: "alice@example.com"},
		{ID: "u2", Name: "", Username: "bob", Email: "bob@example.com"},
		{ID: "u3", Name: "", Username: "", Email: "charlie@example.com"},
	}
	m := buildOncallUserNameIndex(users)

	if m["u1"] != "Alice" {
		t.Errorf("expected Alice for u1, got %s", m["u1"])
	}
	if m["u2"] != "bob" {
		t.Errorf("expected bob (username fallback) for u2, got %s", m["u2"])
	}
	if m["u3"] != "charlie@example.com" {
		t.Errorf("expected email fallback for u3, got %s", m["u3"])
	}
}

// ── IDs are unique ────────────────────────────────────────────────────────────

func TestTransformAll_UniqueIDs(t *testing.T) {
	// Every imported entity should get a unique UUID — never zero.
	users := []OnCallUser{
		{ID: "u1", Email: "alice@example.com", Name: "Alice", Role: "user"},
		{ID: "u2", Email: "bob@example.com", Name: "Bob", Role: "user"},
	}
	schedules := []OnCallSchedule{
		{ID: "s1", Name: "Sched A", Type: "web", TimeZone: "UTC", Shifts: []string{"shift1"}},
	}
	shifts := []OnCallShift{
		{ID: "shift1", Name: "Layer", Type: "rolling_users", Level: 0, Frequency: "weekly",
			Duration: 604800, Users: []string{"u1"}},
	}
	emails, schedNames, policyNames := emptyExisting()
	p := TransformAll(users, schedules, shifts, nil, nil, nil, emails, schedNames, policyNames, "http://regen.example.com")

	seen := map[string]bool{}
	for _, u := range p.Users {
		id := u.ID.String()
		if seen[id] {
			t.Errorf("duplicate UUID for user: %s", id)
		}
		seen[id] = true
	}
	for _, s := range p.Schedules {
		id := s.ID.String()
		if seen[id] {
			t.Errorf("duplicate UUID for schedule: %s", id)
		}
		seen[id] = true
		for _, l := range s.Layers {
			lid := l.ID.String()
			if seen[lid] {
				t.Errorf("duplicate UUID for layer: %s", lid)
			}
			seen[lid] = true
		}
	}
}
