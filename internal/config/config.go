package config

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Config holds all configuration parsed from environment variables.
type Config struct {
	GitHubToken      string
	GitHubRepository string
	GitHubAPIURL     string

	ReleaseTag  string
	ReleaseBody string

	JiraServer  string
	JiraProject string
	JiraUser    string
	JiraToken   string

	TagFormat         *regexp.Regexp
	ReleaseNameFormat string

	RepoOwner string
	RepoName  string
	Version   string
}

// Load reads environment variables and returns a validated Config.
func Load() (*Config, error) {
	eventTag, eventBody := loadEventPayload()

	c := &Config{
		GitHubToken:       os.Getenv("GITHUB_TOKEN"),
		GitHubRepository:  os.Getenv("GITHUB_REPOSITORY"),
		GitHubAPIURL:      os.Getenv("GITHUB_API_URL"),
		ReleaseTag:        firstNonEmpty(eventTag, os.Getenv("RELEASE_TAG")),
		ReleaseBody:       firstNonEmpty(eventBody, os.Getenv("RELEASE_BODY")),
		JiraServer:        os.Getenv("INPUT_JIRA_SERVER"),
		JiraProject:       os.Getenv("INPUT_JIRA_PROJECT"),
		JiraUser:          os.Getenv("INPUT_JIRA_USER"),
		JiraToken:         os.Getenv("INPUT_JIRA_TOKEN"),
		ReleaseNameFormat: os.Getenv("INPUT_RELEASE_NAME_FORMAT"),
	}

	if c.GitHubAPIURL == "" {
		c.GitHubAPIURL = "https://api.github.com"
	}
	if c.ReleaseNameFormat == "" {
		c.ReleaseNameFormat = "{version}"
	}

	for _, check := range []struct {
		val, name string
	}{
		{c.GitHubToken, "GITHUB_TOKEN"},
		{c.GitHubRepository, "GITHUB_REPOSITORY"},
		{c.ReleaseTag, "RELEASE_TAG"},
		{c.JiraServer, "INPUT_JIRA_SERVER"},
		{c.JiraProject, "INPUT_JIRA_PROJECT"},
		{c.JiraUser, "INPUT_JIRA_USER"},
		{c.JiraToken, "INPUT_JIRA_TOKEN"},
	} {
		if check.val == "" {
			return nil, fmt.Errorf("required environment variable %s is not set", check.name)
		}
	}

	owner, repo, err := ParseRepository(c.GitHubRepository)
	if err != nil {
		return nil, err
	}
	c.RepoOwner = owner
	c.RepoName = repo

	tagFormat := os.Getenv("INPUT_TAG_FORMAT")
	if tagFormat != "" {
		re, err := regexp.Compile(tagFormat)
		if err != nil {
			return nil, fmt.Errorf("invalid tag_format regex %q: %w", tagFormat, err)
		}
		c.TagFormat = re
	}

	c.Version = ExtractVersion(c.ReleaseTag, c.TagFormat)

	return c, nil
}

// ParseRepository splits "owner/repo" into its parts.
func ParseRepository(repo string) (owner, name string, err error) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid GITHUB_REPOSITORY format %q, expected owner/repo", repo)
	}
	return parts[0], parts[1], nil
}

// ExtractVersion extracts the version string from a tag.
// If tagFormat is provided and has a capturing group, it uses the first capture.
// Otherwise, it strips a leading "v" prefix.
func ExtractVersion(tag string, tagFormat *regexp.Regexp) string {
	if tagFormat != nil {
		matches := tagFormat.FindStringSubmatch(tag)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	return strings.TrimPrefix(tag, "v")
}

// FormatReleaseName replaces {version} in the format string.
func FormatReleaseName(format, version string) string {
	return strings.ReplaceAll(format, "{version}", version)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// ghEvent is the subset of the GitHub Actions event payload we care about.
type ghEvent struct {
	Release struct {
		TagName string `json:"tag_name"`
		Body    string `json:"body"`
	} `json:"release"`
}

// loadEventPayload reads release data from the GITHUB_EVENT_PATH JSON file.
// This is more reliable than env vars for multiline fields like the release body.
func loadEventPayload() (tag, body string) {
	path := os.Getenv("GITHUB_EVENT_PATH")
	if path == "" {
		return "", ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	var event ghEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return "", ""
	}
	return event.Release.TagName, event.Release.Body
}
