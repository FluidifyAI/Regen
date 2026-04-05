package oncall

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	maxResponseBytes = 4 * 1024 * 1024 // 4 MB guard per response
	requestTimeout   = 30 * time.Second
	userAgent        = "fluidify-regen-migrator/1.0"
	maxRetries       = 2
)

// Client is a minimal, typed HTTP client for the Grafana OnCall OSS API v1.
// Construct one with NewClient; it is safe for concurrent use after construction.
type Client struct {
	baseURL    string // e.g. "https://grafana.myco.com"
	apiToken   string
	httpClient *http.Client
}

// NewClient creates a new OnCall API client.
// baseURL must be the root of the Grafana/OnCall instance (no trailing slash required).
// apiToken is the Grafana OnCall API key prefixed with "Token ".
func NewClient(baseURL, apiToken string) *Client {
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		apiToken: apiToken,
		httpClient: &http.Client{
			Timeout: requestTimeout,
		},
	}
}

// Ping validates the URL and API token by calling GET /api/v1/users with a
// page size of 1. Returns a descriptive error on auth failure or unreachable host.
func (c *Client) Ping() error {
	resp, err := c.getRaw("/api/v1/users/", map[string]string{"perpage": "1"})
	if err != nil {
		return fmt.Errorf("cannot reach Grafana OnCall at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("invalid API token (HTTP %d) — generate one in Grafana OnCall → API Tokens", resp.StatusCode)
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("unexpected response from Grafana OnCall (HTTP %d): %s", resp.StatusCode, string(body))
	}
}

// ListUsers returns all users from GET /api/v1/users, paginating automatically.
func (c *Client) ListUsers() ([]OnCallUser, error) {
	var all []OnCallUser
	nextURL := c.baseURL + "/api/v1/users/"
	for nextURL != "" {
		var page listResponse[OnCallUser]
		if err := c.getPage(nextURL, &page); err != nil {
			return nil, fmt.Errorf("listing OnCall users: %w", err)
		}
		all = append(all, page.Results...)
		nextURL = page.Next
	}
	return all, nil
}

// ListTeams returns all teams from GET /api/v1/teams.
func (c *Client) ListTeams() ([]OnCallTeam, error) {
	var all []OnCallTeam
	nextURL := c.baseURL + "/api/v1/teams/"
	for nextURL != "" {
		var page listResponse[OnCallTeam]
		if err := c.getPage(nextURL, &page); err != nil {
			return nil, fmt.Errorf("listing OnCall teams: %w", err)
		}
		all = append(all, page.Results...)
		nextURL = page.Next
	}
	return all, nil
}

// ListSchedules returns all schedules from GET /api/v1/schedules.
func (c *Client) ListSchedules() ([]OnCallSchedule, error) {
	var all []OnCallSchedule
	nextURL := c.baseURL + "/api/v1/schedules/"
	for nextURL != "" {
		var page listResponse[OnCallSchedule]
		if err := c.getPage(nextURL, &page); err != nil {
			return nil, fmt.Errorf("listing OnCall schedules: %w", err)
		}
		all = append(all, page.Results...)
		nextURL = page.Next
	}
	return all, nil
}

// ListShifts returns all on-call shifts from GET /api/v1/on_call_shifts.
func (c *Client) ListShifts() ([]OnCallShift, error) {
	var all []OnCallShift
	nextURL := c.baseURL + "/api/v1/on_call_shifts/"
	for nextURL != "" {
		var page listResponse[OnCallShift]
		if err := c.getPage(nextURL, &page); err != nil {
			return nil, fmt.Errorf("listing OnCall shifts: %w", err)
		}
		all = append(all, page.Results...)
		nextURL = page.Next
	}
	return all, nil
}

// ListEscalationChains returns all escalation chains from GET /api/v1/escalation_chains.
func (c *Client) ListEscalationChains() ([]OnCallEscalationChain, error) {
	var all []OnCallEscalationChain
	nextURL := c.baseURL + "/api/v1/escalation_chains/"
	for nextURL != "" {
		var page listResponse[OnCallEscalationChain]
		if err := c.getPage(nextURL, &page); err != nil {
			return nil, fmt.Errorf("listing OnCall escalation chains: %w", err)
		}
		all = append(all, page.Results...)
		nextURL = page.Next
	}
	return all, nil
}

// ListEscalationSteps returns all escalation policy steps from GET /api/v1/escalation_policies.
// Steps are returned flat and must be grouped by EscalationChain.ID in the caller.
func (c *Client) ListEscalationSteps() ([]OnCallEscalationStep, error) {
	var all []OnCallEscalationStep
	nextURL := c.baseURL + "/api/v1/escalation_policies/"
	for nextURL != "" {
		var page listResponse[OnCallEscalationStep]
		if err := c.getPage(nextURL, &page); err != nil {
			return nil, fmt.Errorf("listing OnCall escalation steps: %w", err)
		}
		all = append(all, page.Results...)
		nextURL = page.Next
	}
	return all, nil
}

// ListIntegrations returns all integrations from GET /api/v1/integrations.
func (c *Client) ListIntegrations() ([]OnCallIntegration, error) {
	var all []OnCallIntegration
	nextURL := c.baseURL + "/api/v1/integrations/"
	for nextURL != "" {
		var page listResponse[OnCallIntegration]
		if err := c.getPage(nextURL, &page); err != nil {
			return nil, fmt.Errorf("listing OnCall integrations: %w", err)
		}
		all = append(all, page.Results...)
		nextURL = page.Next
	}
	return all, nil
}

// ── internal helpers ──────────────────────────────────────────────────────────

// getPage fetches a single page from an absolute URL and JSON-decodes it into dst.
// It preserves the absolute Next URL returned by OnCall for cursor-based pagination.
func (c *Client) getPage(absoluteURL string, dst any) error {
	req, err := http.NewRequest(http.MethodGet, absoluteURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Token "+c.apiToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	httpResp, err := c.doWithRetry(req)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(httpResp.Body, 512))
		return fmt.Errorf("OnCall API %s returned HTTP %d: %s", absoluteURL, httpResp.StatusCode, string(body))
	}
	if err := json.NewDecoder(io.LimitReader(httpResp.Body, maxResponseBytes)).Decode(dst); err != nil {
		return fmt.Errorf("decoding OnCall response from %s: %w", absoluteURL, err)
	}
	return nil
}

// getRaw issues a GET request to a path relative to baseURL, returning the raw response.
// path must start with "/". params are appended as query string.
// The caller is responsible for closing the response body.
func (c *Client) getRaw(path string, params map[string]string) (*http.Response, error) {
	rawURL := c.baseURL + path
	if len(params) > 0 {
		u, err := url.Parse(rawURL)
		if err != nil {
			return nil, err
		}
		q := u.Query()
		for k, v := range params {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
		rawURL = u.String()
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+c.apiToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	return c.httpClient.Do(req)
}

// doWithRetry executes req, retrying once on 5xx responses.
func (c *Client) doWithRetry(req *http.Request) (*http.Response, error) {
	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < 500 || attempt == maxRetries {
			return resp, nil
		}
		resp.Body.Close()
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}
	return nil, fmt.Errorf("exceeded retry limit")
}
