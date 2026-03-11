package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client is a Jira REST API v3 client.
type Client struct {
	baseURL    string
	user       string
	token      string
	httpClient *http.Client
}

// NewClient creates a new Jira API client.
func NewClient(baseURL, user, token string) *Client {
	return &Client{
		baseURL:    baseURL,
		user:       user,
		token:      token,
		httpClient: &http.Client{},
	}
}

// GetProjectVersions returns all versions for a project.
func (c *Client) GetProjectVersions(ctx context.Context, projectKey string) ([]Version, error) {
	url := fmt.Sprintf("%s/rest/api/3/project/%s/versions", c.baseURL, projectKey)

	req, err := c.newRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	var versions []Version
	if err := c.do(req, &versions); err != nil {
		return nil, fmt.Errorf("getting project versions: %w", err)
	}
	return versions, nil
}

// CreateVersion creates a new version in Jira.
func (c *Client) CreateVersion(ctx context.Context, v CreateVersionRequest) (*Version, error) {
	url := fmt.Sprintf("%s/rest/api/3/version", c.baseURL)

	body, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	req, err := c.newRequest(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	var created Version
	if err := c.do(req, &created); err != nil {
		return nil, fmt.Errorf("creating version: %w", err)
	}
	return &created, nil
}

// UpdateVersion updates an existing version by ID.
func (c *Client) UpdateVersion(ctx context.Context, versionID string, v UpdateVersionRequest) error {
	url := fmt.Sprintf("%s/rest/api/3/version/%s", c.baseURL, versionID)

	body, err := json.Marshal(v)
	if err != nil {
		return err
	}

	req, err := c.newRequest(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	if err := c.do(req, nil); err != nil {
		return fmt.Errorf("updating version %s: %w", versionID, err)
	}
	return nil
}

// AddFixVersion adds a version to an issue's Fix Versions field.
func (c *Client) AddFixVersion(ctx context.Context, issueKey, versionName string) error {
	url := fmt.Sprintf("%s/rest/api/3/issue/%s", c.baseURL, issueKey)

	payload := IssueUpdateRequest{
		Update: IssueUpdateFields{
			FixVersions: []FixVersionOp{
				{Add: &VersionRef{Name: versionName}},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := c.newRequest(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("updating issue %s: %w", issueKey, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("updating issue %s fix versions: status %d: %s", issueKey, resp.StatusCode, string(respBody))
	}
	return nil
}

func (c *Client) newRequest(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.user, c.token)
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func (c *Client) do(req *http.Request, out interface{}) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Jira API %s returned %d: %s", req.URL.Path, resp.StatusCode, string(body))
	}

	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
