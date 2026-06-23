package opsgenie

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	baseURLUS  = "https://api.opsgenie.com"
	baseURLEU  = "https://api.eu.opsgenie.com"
	pageLimit  = 100
	maxRetries = 3
)

// Client is a minimal Opsgenie API HTTP client.
type Client struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
}

// NewClient creates an Opsgenie client for the given region ("us" or "eu").
func NewClient(apiKey, region string) *Client {
	base := baseURLUS
	if region == "eu" {
		base = baseURLEU
	}
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    base,
	}
}

// NewClientWithBaseURL creates a client with a custom base URL. Used in tests.
func NewClientWithBaseURL(apiKey, region, base string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    base,
	}
}

// ValidateAPIKey checks the API key by calling GET /v2/users/me.
func (c *Client) ValidateAPIKey() error {
	resp, err := c.getRaw("/v2/users/me", nil)
	if err != nil {
		return fmt.Errorf("API key validation failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("invalid Opsgenie API key")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API key validation returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// FetchUsers returns a map of email → display name for all Opsgenie users.
func (c *Client) FetchUsers() (map[string]string, error) {
	emailToName := make(map[string]string)
	params := map[string]string{"limit": fmt.Sprintf("%d", pageLimit)}
	resp, err := c.get("/v2/users", params)
	if err != nil {
		return nil, fmt.Errorf("fetching users: %w", err)
	}
	var result listUsersResponse
	if err := decodeJSON(resp, &result); err != nil {
		return nil, err
	}
	for _, u := range result.Data {
		name := u.FullName
		if name == "" {
			name = u.Username
		}
		emailToName[u.Username] = name
	}
	return emailToName, nil
}

// FetchSchedules returns all Opsgenie schedules (list view, no rotations).
func (c *Client) FetchSchedules() ([]OGSchedule, error) {
	resp, err := c.get("/v2/schedules", nil)
	if err != nil {
		return nil, fmt.Errorf("fetching schedules: %w", err)
	}
	var result listSchedulesResponse
	if err := decodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return result.Data, nil
}

// FetchRotations returns all rotations for the schedule identified by scheduleID.
func (c *Client) FetchRotations(scheduleID string) ([]OGRotation, error) {
	path := "/v2/schedules/" + scheduleID + "/rotations"
	params := map[string]string{"identifierType": "id"}
	resp, err := c.get(path, params)
	if err != nil {
		return nil, fmt.Errorf("fetching rotations for schedule %s: %w", scheduleID, err)
	}
	var result listRotationsResponse
	if err := decodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return result.Data, nil
}

// FetchEscalationPolicies returns all Opsgenie escalation policies with rules inline.
func (c *Client) FetchEscalationPolicies() ([]OGEscalationPolicy, error) {
	resp, err := c.get("/v1/escalations", nil)
	if err != nil {
		return nil, fmt.Errorf("fetching escalation policies: %w", err)
	}
	var result listEscalationsResponse
	if err := decodeJSON(resp, &result); err != nil {
		return nil, err
	}
	return result.Data, nil
}

// getRaw executes a GET and returns the raw response (caller checks status).
func (c *Client) getRaw(path string, params map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "GenieKey "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
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
	return nil, fmt.Errorf("opsgenie API %s: exceeded %d retries", path, maxRetries)
}

// get executes a GET and returns the response, failing on 4xx/5xx.
func (c *Client) get(path string, params map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "GenieKey "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
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
			return nil, fmt.Errorf("opsgenie API %s returned HTTP %d: %s", path, resp.StatusCode, string(body))
		}
		return resp, nil
	}
	return nil, fmt.Errorf("opsgenie API %s: exceeded %d retries", path, maxRetries)
}

func decodeJSON(resp *http.Response, dst any) error {
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decoding Opsgenie response: %w", err)
	}
	return nil
}
