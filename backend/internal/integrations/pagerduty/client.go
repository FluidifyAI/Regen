package pagerduty

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	baseURL    = "https://api.pagerduty.com"
	pageLimit  = 100
	maxRetries = 3
)

// Client is a minimal PagerDuty API v2 HTTP client.
type Client struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string // overridable for testing
}

// NewClient creates a PagerDuty client with the given API key.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
	}
}

// ValidateAPIKey checks the API key by calling GET /users/me.
// Returns an error if the key is invalid or the API is unreachable.
func (c *Client) ValidateAPIKey() error {
	resp, err := c.getRaw("/users/me", nil)
	if err != nil {
		return fmt.Errorf("API key validation failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid PagerDuty API key")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API key validation returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// FetchUsers returns all PagerDuty users, paginating automatically.
// Returns a map of email → display name for user resolution.
func (c *Client) FetchUsers() (map[string]string, error) {
	emailToName := make(map[string]string)
	offset := 0
	for {
		params := map[string]string{
			"limit":  fmt.Sprintf("%d", pageLimit),
			"offset": fmt.Sprintf("%d", offset),
		}
		resp, err := c.get("/users", params)
		if err != nil {
			return nil, fmt.Errorf("fetching users (offset=%d): %w", offset, err)
		}
		var result listUsersResponse
		if err := decodeJSON(resp, &result); err != nil {
			return nil, err
		}
		for _, u := range result.Users {
			emailToName[u.Email] = u.Name
		}
		if !result.More {
			break
		}
		offset += pageLimit
	}
	return emailToName, nil
}

// FetchSchedules returns all PagerDuty schedules (list view, no layer detail).
func (c *Client) FetchSchedules() ([]PDSchedule, error) {
	var all []PDSchedule
	offset := 0
	for {
		params := map[string]string{
			"limit":  fmt.Sprintf("%d", pageLimit),
			"offset": fmt.Sprintf("%d", offset),
		}
		resp, err := c.get("/schedules", params)
		if err != nil {
			return nil, fmt.Errorf("fetching schedules (offset=%d): %w", offset, err)
		}
		var result listSchedulesResponse
		if err := decodeJSON(resp, &result); err != nil {
			return nil, err
		}
		all = append(all, result.Schedules...)
		if !result.More {
			break
		}
		offset += pageLimit
	}
	return all, nil
}

// FetchScheduleDetail returns a full schedule including layers and users.
func (c *Client) FetchScheduleDetail(id string) (*PDScheduleDetail, error) {
	params := map[string]string{"include[]": "users"}
	resp, err := c.get("/schedules/"+id, params)
	if err != nil {
		return nil, fmt.Errorf("fetching schedule %s: %w", id, err)
	}
	var result scheduleDetailResponse
	if err := decodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.Schedule, nil
}

// FetchEscalationPolicies returns all PagerDuty escalation policies (list view).
func (c *Client) FetchEscalationPolicies() ([]PDEscalationPolicy, error) {
	var all []PDEscalationPolicy
	offset := 0
	for {
		params := map[string]string{
			"limit":  fmt.Sprintf("%d", pageLimit),
			"offset": fmt.Sprintf("%d", offset),
		}
		resp, err := c.get("/escalation_policies", params)
		if err != nil {
			return nil, fmt.Errorf("fetching policies (offset=%d): %w", offset, err)
		}
		var result listPoliciesResponse
		if err := decodeJSON(resp, &result); err != nil {
			return nil, err
		}
		all = append(all, result.EscalationPolicies...)
		if !result.More {
			break
		}
		offset += pageLimit
	}
	return all, nil
}

// FetchEscalationPolicyDetail returns a full policy including rules and targets.
func (c *Client) FetchEscalationPolicyDetail(id string) (*PDEscalationPolicyDetail, error) {
	params := map[string]string{"include[]": "targets"}
	resp, err := c.get("/escalation_policies/"+id, params)
	if err != nil {
		return nil, fmt.Errorf("fetching policy %s: %w", id, err)
	}
	var result policyDetailResponse
	if err := decodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return &result.EscalationPolicy, nil
}

// getRaw executes a GET request and returns the response even for 4xx status
// codes (caller is responsible for checking the status). Used for ValidateAPIKey
// where we need to distinguish 401 from other errors.
func (c *Client) getRaw(path string, params map[string]string) (*http.Response, error) {
	url := c.baseURL + path
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token token="+c.apiKey)
	req.Header.Set("Accept", "application/vnd.pagerduty+json;version=2")
	if len(params) > 0 {
		q := req.URL.Query()
		for k, v := range params {
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}
	backoff := time.Second
	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			time.Sleep(backoff)
			backoff *= 2
			continue
		}
		return resp, nil
	}
	return nil, fmt.Errorf("PagerDuty API %s: exceeded %d retries", path, maxRetries)
}

// get executes a GET request against the PD API with retry on 429 (rate limit).
func (c *Client) get(path string, params map[string]string) (*http.Response, error) {
	url := c.baseURL + path
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token token="+c.apiKey)
	req.Header.Set("Accept", "application/vnd.pagerduty+json;version=2")

	if len(params) > 0 {
		q := req.URL.Query()
		for k, v := range params {
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}

	backoff := time.Second
	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			time.Sleep(backoff)
			backoff *= 2
			continue
		}
		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("PagerDuty API %s returned HTTP %d: %s", path, resp.StatusCode, string(body))
		}
		return resp, nil
	}
	return nil, fmt.Errorf("PagerDuty API %s: exceeded %d retries", path, maxRetries)
}

func decodeJSON(resp *http.Response, dst any) error {
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decoding PagerDuty response: %w", err)
	}
	return nil
}
