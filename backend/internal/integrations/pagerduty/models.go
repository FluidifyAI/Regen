// Package pagerduty provides a minimal HTTP client and data models for the
// PagerDuty REST API v2, scoped to the fields required by the import tool.
package pagerduty

import "time"

// PDUser is a PagerDuty user (used for email → name lookup).
type PDUser struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// PDSchedule is a PagerDuty on-call schedule (list view).
type PDSchedule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	TimeZone    string `json:"time_zone"`
}

// PDScheduleDetail is a full schedule including layers and users.
type PDScheduleDetail struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	TimeZone       string            `json:"time_zone"`
	ScheduleLayers []PDScheduleLayer `json:"schedule_layers"`
}

// PDScheduleLayer is one rotation layer within a schedule.
type PDScheduleLayer struct {
	ID                        string        `json:"id"`
	Name                      string        `json:"name"`
	Start                     time.Time     `json:"start"`
	RotationTurnLengthSeconds int           `json:"rotation_turn_length_seconds"`
	RotationVirtualStart      time.Time     `json:"rotation_virtual_start"`
	Users                     []PDLayerUser `json:"users"`
}

// PDLayerUser is a user entry within a schedule layer.
type PDLayerUser struct {
	User PDUser `json:"user"`
}

// PDEscalationPolicy is a PagerDuty escalation policy (list view).
type PDEscalationPolicy struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// PDEscalationPolicyDetail is a full policy including rules and targets.
type PDEscalationPolicyDetail struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	Description     string             `json:"description"`
	EscalationRules []PDEscalationRule `json:"escalation_rules"`
}

// PDEscalationRule is one tier in an escalation policy.
type PDEscalationRule struct {
	EscalationDelayInMinutes int        `json:"escalation_delay_in_minutes"`
	Targets                  []PDTarget `json:"targets"`
}

// PDTarget is a notification target within an escalation rule.
type PDTarget struct {
	Type string `json:"type"` // "schedule_reference", "user_reference", "team_reference"
	ID   string `json:"id"`
	Name string `json:"name"`
}

// listUsersResponse is the paginated response for GET /users.
type listUsersResponse struct {
	Users  []PDUser `json:"users"`
	More   bool     `json:"more"`
	Offset int      `json:"offset"`
	Limit  int      `json:"limit"`
}

// listSchedulesResponse is the paginated response for GET /schedules.
type listSchedulesResponse struct {
	Schedules []PDSchedule `json:"schedules"`
	More      bool         `json:"more"`
	Offset    int          `json:"offset"`
	Limit     int          `json:"limit"`
}

// scheduleDetailResponse wraps GET /schedules/:id.
type scheduleDetailResponse struct {
	Schedule PDScheduleDetail `json:"schedule"`
}

// listPoliciesResponse is the paginated response for GET /escalation_policies.
type listPoliciesResponse struct {
	EscalationPolicies []PDEscalationPolicy `json:"escalation_policies"`
	More               bool                 `json:"more"`
	Offset             int                  `json:"offset"`
	Limit              int                  `json:"limit"`
}

// policyDetailResponse wraps GET /escalation_policies/:id.
type policyDetailResponse struct {
	EscalationPolicy PDEscalationPolicyDetail `json:"escalation_policy"`
}
