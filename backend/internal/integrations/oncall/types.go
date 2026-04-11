// Package oncall provides a typed client and data transformation layer for the
// Grafana OnCall OSS HTTP API (v1). It is used exclusively by the one-time
// migration import flow; it does not maintain any persistent state.
package oncall

// ── Paginated list wrapper ────────────────────────────────────────────────────

// listResponse is the envelope returned by every paginated OnCall list endpoint.
// `Next` contains an absolute URL when more pages exist, or is empty/null.
type listResponse[T any] struct {
	Count    int    `json:"count"`
	Next     string `json:"next"`
	Previous string `json:"previous"`
	Results  []T    `json:"results"`
}

// ── Users ─────────────────────────────────────────────────────────────────────

// OnCallUser represents a user returned by GET /api/v1/users.
type OnCallUser struct {
	ID       string          `json:"pk"`
	Email    string          `json:"email"`
	Username string          `json:"username"`
	Name     string          `json:"name"` // may be empty; fall back to Username
	Role     string          `json:"role"` // "admin", "user", "viewer"
	Slack    *OnCallSlackRef `json:"slack"`
}

// OnCallSlackRef holds the Slack identity embedded in some OnCall objects.
type OnCallSlackRef struct {
	UserID string `json:"user_id"`
	Name   string `json:"name"`
}

// ── Teams ─────────────────────────────────────────────────────────────────────

// OnCallTeam represents a team returned by GET /api/v1/teams.
type OnCallTeam struct {
	ID   string `json:"pk"`
	Name string `json:"name"`
}

// ── Schedules ────────────────────────────────────────────────────────────────

// OnCallSchedule represents a schedule returned by GET /api/v1/schedules.
type OnCallSchedule struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Type     string   `json:"type"`      // "web", "ical", "calendar"
	TimeZone string   `json:"time_zone"` // IANA timezone
	Shifts   []string `json:"shifts"`    // IDs of on-call shifts
	Team     string   `json:"team"`      // team ID, may be empty
}

// ── Shifts ────────────────────────────────────────────────────────────────────

// OnCallShift represents a rotation shift returned by GET /api/v1/on_call_shifts.
// OnCall models every layer of a schedule as a "shift" object.
type OnCallShift struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Type          string     `json:"type"`           // "rolling_users", "recurrent_event", "override"
	TimeZone      string     `json:"time_zone"`      // may be empty — falls back to schedule timezone
	Level         int        `json:"level"`          // layer index (0 = primary)
	Start         string     `json:"start"`          // ISO 8601 datetime
	RotationStart string     `json:"rotation_start"` // ISO 8601 datetime; epoch of rotation
	Duration      int        `json:"duration"`       // seconds per shift slot
	Frequency     string     `json:"frequency"`      // "weekly", "daily", "hourly", "monthly"
	Interval      int        `json:"interval"`       // repeat every N frequency units
	ByDay         []string   `json:"by_day"`         // e.g. ["MO","WE","FR"]
	RollingUsers  [][]string `json:"rolling_users"`  // groups of user IDs cycling through
	Users         []string   `json:"users"`          // fixed user IDs for non-rolling shifts
	Team          string     `json:"team"`           // team ID, may be empty
}

// ── Escalation ────────────────────────────────────────────────────────────────

// OnCallEscalationChain represents a chain returned by GET /api/v1/escalation_chains.
type OnCallEscalationChain struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Team string `json:"team"` // team ID, may be empty
}

// OnCallEscalationStep represents a single step within a chain, returned by
// GET /api/v1/escalation_policies (the OnCall API calls these "policies").
type OnCallEscalationStep struct {
	ID              string   `json:"id"`
	Step            int      `json:"step"`                             // 0-based position within the chain
	EscalationChain string   `json:"escalation_chain"`                 // parent chain ID
	Type            string   `json:"type"`                             // see step type constants below
	Duration        *int     `json:"duration"`                         // seconds; only set for "wait" steps
	PersonsToNotify []string `json:"persons_to_notify"`                // user IDs
	PersonsNextTime []string `json:"persons_to_notify_next_each_time"` // user IDs
	Schedule        string   `json:"schedule"`                         // schedule ID; for notify_on_call_from_schedule
}

// OnCall escalation step type constants.
const (
	StepTypeWait                     = "wait"
	StepTypeNotifyPersons            = "notify_persons"
	StepTypeNotifyPersonNextEachTime = "notify_person_next_each_time"
	StepTypeNotifyOnCallFromSchedule = "notify_on_call_from_schedule"
	StepTypeNotifyWholedTeam         = "notify_whole_channel"
	StepTypeResolveIncident          = "resolve_incident"
)

// ── Integrations ──────────────────────────────────────────────────────────────

// OnCallIntegration represents a webhook integration returned by GET /api/v1/integrations.
type OnCallIntegration struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // "alertmanager", "grafana", "cloudwatch", "generic", etc.
	Link string `json:"link"` // existing OnCall inbound webhook URL
	Team string `json:"team"` // team ID, may be empty
}
