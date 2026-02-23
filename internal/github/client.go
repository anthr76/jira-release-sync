package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// ErrNoPreviousTag is returned when the current tag is the first/only tag.
var ErrNoPreviousTag = errors.New("no previous tag found")

// Client is a GitHub REST API client.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient creates a new GitHub API client.
func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL:    baseURL,
		token:      token,
		httpClient: &http.Client{},
	}
}

// FindPreviousTag paginates through tags to find the tag immediately before currentTag.
func (c *Client) FindPreviousTag(ctx context.Context, owner, repo, currentTag string) (string, error) {
	page := 1
	found := false

	for {
		url := fmt.Sprintf("%s/repos/%s/%s/tags?per_page=100&page=%d", c.baseURL, owner, repo, page)

		var tags []Tag
		if err := c.get(ctx, url, &tags); err != nil {
			return "", fmt.Errorf("listing tags: %w", err)
		}
		if len(tags) == 0 {
			break
		}

		for _, tag := range tags {
			if found {
				return tag.Name, nil
			}
			if tag.Name == currentTag {
				found = true
			}
		}

		page++
	}

	if found {
		return "", ErrNoPreviousTag
	}
	return "", fmt.Errorf("tag %s not found", currentTag)
}

// CompareCommits returns the commits between base and head.
func (c *Client) CompareCommits(ctx context.Context, owner, repo, base, head string) ([]CommitEntry, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/compare/%s...%s", c.baseURL, owner, repo, base, head)

	var resp CompareResponse
	if err := c.get(ctx, url, &resp); err != nil {
		return nil, fmt.Errorf("comparing commits: %w", err)
	}
	return resp.Commits, nil
}

// ListPullRequestsForCommit returns PRs associated with a commit SHA.
func (c *Client) ListPullRequestsForCommit(ctx context.Context, owner, repo, sha string) ([]PullRequest, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/commits/%s/pulls", c.baseURL, owner, repo, sha)

	var prs []PullRequest
	if err := c.get(ctx, url, &prs); err != nil {
		return nil, fmt.Errorf("listing PRs for commit %s: %w", sha, err)
	}
	return prs, nil
}

// GetReleaseByTag fetches a release by its tag name.
func (c *Client) GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s", c.baseURL, owner, repo, tag)

	var release Release
	if err := c.get(ctx, url, &release); err != nil {
		return nil, fmt.Errorf("getting release for tag %s: %w", tag, err)
	}
	return &release, nil
}

func (c *Client) get(ctx context.Context, url string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API %s returned %d: %s", url, resp.StatusCode, string(body))
	}

	return json.NewDecoder(resp.Body).Decode(out)
}
