// Package opsgenie provides a minimal HTTP client and data models for the
// Opsgenie REST API v2, scoped to the fields required by the import tool.
package opsgenie

import "time"

// OGUser is an Opsgenie user.
type OGUser struct {
	ID       string `json:"id"`
	Username string `json:"username"` // email address
	FullName string `json:"fullName"`
}

// OGSchedule is an Opsgenie on-call schedule (list view).
type OGSchedule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Timezone    string `json:"timezone"`
	Enabled     bool   `json:"enabled"`
}

// OGRotation is a rotation within a schedule.
type OGRotation struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	StartDate    time.Time      `json:"startDate"`
	Type         string         `json:"type"`   // weekly, daily, hourly, none
	Length       int            `json:"length"` // multiplier (e.g. 2 = biweekly)
	Participants []OGParticipant `json:"participants"`
}

// OGParticipant is a user/team within a rotation.
type OGParticipant struct {
	Type     string `json:"type"`     // user, team, escalation
	ID       string `json:"id"`
	Username string `json:"username"` // email for user type
	Name     string `json:"name"`
}

// OGScheduleDetail is a schedule together with its fetched rotations.
type OGScheduleDetail struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Timezone    string       `json:"timezone"`
	Rotations   []OGRotation `json:"rotations"`
}

// OGEscalationPolicy is an Opsgenie escalation policy (rules are inline in list response).
type OGEscalationPolicy struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Rules       []OGRule `json:"rules"`
}

// OGRule is one tier in an escalation policy.
type OGRule struct {
	Condition  string        `json:"condition"`  // if-not-acked, if-not-closed
	NotifyType string        `json:"notifyType"`
	Delay      OGDelay       `json:"delay"`
	Recipient  []OGRecipient `json:"recipient"`
}

// OGDelay encodes the escalation timeout.
type OGDelay struct {
	TimeAmount int    `json:"timeAmount"`
	TimeUnit   string `json:"timeUnit"` // minutes, hours, days
}

// OGRecipient is a notification target in an escalation rule.
type OGRecipient struct {
	Type     string `json:"type"` // user, team, schedule, escalation
	ID       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"` // email for user type
}

// ── paginated list wrappers ───────────────────────────────────────────────────

type listUsersResponse struct {
	Data []OGUser `json:"data"`
}

type listSchedulesResponse struct {
	Data []OGSchedule `json:"data"`
}

type listRotationsResponse struct {
	Data []OGRotation `json:"data"`
}

type listEscalationsResponse struct {
	Data []OGEscalationPolicy `json:"data"`
}
