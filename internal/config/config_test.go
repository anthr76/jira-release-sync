package config

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestParseRepository(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{name: "valid", input: "coreweave/bmaas-api", wantOwner: "coreweave", wantRepo: "bmaas-api"},
		{name: "empty", input: "", wantErr: true},
		{name: "no slash", input: "bmaas-api", wantErr: true},
		{name: "empty owner", input: "/repo", wantErr: true},
		{name: "empty repo", input: "owner/", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := ParseRepository(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if owner != tt.wantOwner {
				t.Errorf("owner = %q, want %q", owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Errorf("repo = %q, want %q", repo, tt.wantRepo)
			}
		})
	}
}

func TestExtractVersion(t *testing.T) {
	tests := []struct {
		name      string
		tag       string
		tagFormat *regexp.Regexp
		want      string
	}{
		{name: "strip v prefix", tag: "v1.2.3", tagFormat: nil, want: "1.2.3"},
		{name: "no v prefix", tag: "1.2.3", tagFormat: nil, want: "1.2.3"},
		{name: "with capture group", tag: "release-1.2.3", tagFormat: regexp.MustCompile(`release-(.+)`), want: "1.2.3"},
		{name: "complex capture", tag: "myapp/v2.0.0", tagFormat: regexp.MustCompile(`myapp/v(.+)`), want: "2.0.0"},
		{name: "no match falls through", tag: "v1.2.3", tagFormat: regexp.MustCompile(`release-(.+)`), want: "1.2.3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractVersion(tt.tag, tt.tagFormat)
			if got != tt.want {
				t.Errorf("ExtractVersion(%q) = %q, want %q", tt.tag, got, tt.want)
			}
		})
	}
}

func TestFormatReleaseName(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		version string
		want    string
	}{
		{name: "default", format: "{version}", version: "1.2.3", want: "1.2.3"},
		{name: "prefix", format: "v{version}", version: "1.2.3", want: "v1.2.3"},
		{name: "full name", format: "Release {version}", version: "2.0.0", want: "Release 2.0.0"},
		{name: "no placeholder", format: "static-name", version: "1.0.0", want: "static-name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatReleaseName(tt.format, tt.version)
			if got != tt.want {
				t.Errorf("FormatReleaseName(%q, %q) = %q, want %q", tt.format, tt.version, got, tt.want)
			}
		})
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	for _, key := range []string{
		"GITHUB_TOKEN", "GITHUB_REPOSITORY", "RELEASE_TAG", "RELEASE_BODY",
		"INPUT_JIRA_SERVER", "INPUT_JIRA_PROJECT", "INPUT_JIRA_USER", "INPUT_JIRA_TOKEN",
		"INPUT_TAG_FORMAT", "INPUT_RELEASE_NAME_FORMAT", "GITHUB_API_URL",
	} {
		t.Setenv(key, "")
	}

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing required vars")
	}
}

func TestLoad_Valid(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_test")
	t.Setenv("GITHUB_REPOSITORY", "myorg/myrepo")
	t.Setenv("GITHUB_API_URL", "")
	t.Setenv("RELEASE_TAG", "v1.5.0")
	t.Setenv("RELEASE_BODY", "## Changes\n- feat: stuff")
	t.Setenv("INPUT_JIRA_SERVER", "https://jira.example.com")
	t.Setenv("INPUT_JIRA_PROJECT", "PRJ")
	t.Setenv("INPUT_JIRA_USER", "user@example.com")
	t.Setenv("INPUT_JIRA_TOKEN", "jira-token")
	t.Setenv("INPUT_TAG_FORMAT", "")
	t.Setenv("INPUT_RELEASE_NAME_FORMAT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RepoOwner != "myorg" {
		t.Errorf("RepoOwner = %q, want %q", cfg.RepoOwner, "myorg")
	}
	if cfg.Version != "1.5.0" {
		t.Errorf("Version = %q, want %q", cfg.Version, "1.5.0")
	}
	if cfg.GitHubAPIURL != "https://api.github.com" {
		t.Errorf("GitHubAPIURL = %q, want default", cfg.GitHubAPIURL)
	}
	if cfg.ReleaseNameFormat != "{version}" {
		t.Errorf("ReleaseNameFormat = %q, want default", cfg.ReleaseNameFormat)
	}
}

func TestLoad_WithTagFormat(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_test")
	t.Setenv("GITHUB_REPOSITORY", "myorg/myrepo")
	t.Setenv("GITHUB_API_URL", "")
	t.Setenv("RELEASE_TAG", "myapp/v2.0.0")
	t.Setenv("RELEASE_BODY", "changelog")
	t.Setenv("INPUT_JIRA_SERVER", "https://jira.example.com")
	t.Setenv("INPUT_JIRA_PROJECT", "PRJ")
	t.Setenv("INPUT_JIRA_USER", "user@example.com")
	t.Setenv("INPUT_JIRA_TOKEN", "jira-token")
	t.Setenv("INPUT_TAG_FORMAT", `myapp/v(.+)`)
	t.Setenv("INPUT_RELEASE_NAME_FORMAT", "Release {version}")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Version != "2.0.0" {
		t.Errorf("Version = %q, want %q", cfg.Version, "2.0.0")
	}
	if cfg.ReleaseNameFormat != "Release {version}" {
		t.Errorf("ReleaseNameFormat = %q", cfg.ReleaseNameFormat)
	}
}

func TestLoad_EventPayload(t *testing.T) {
	eventJSON := `{
		"release": {
			"tag_name": "v3.0.0",
			"body": "## What's Changed\n\n### Features\n- feat: new thing\n\n### Bug Fixes\n- fix: old thing"
		}
	}`
	eventFile := filepath.Join(t.TempDir(), "event.json")
	os.WriteFile(eventFile, []byte(eventJSON), 0644)

	t.Setenv("GITHUB_EVENT_PATH", eventFile)
	t.Setenv("GITHUB_TOKEN", "ghp_test")
	t.Setenv("GITHUB_REPOSITORY", "myorg/myrepo")
	t.Setenv("GITHUB_API_URL", "")
	t.Setenv("RELEASE_TAG", "should-be-overridden")
	t.Setenv("RELEASE_BODY", "should-be-overridden")
	t.Setenv("INPUT_JIRA_SERVER", "https://jira.example.com")
	t.Setenv("INPUT_JIRA_PROJECT", "PRJ")
	t.Setenv("INPUT_JIRA_USER", "user@example.com")
	t.Setenv("INPUT_JIRA_TOKEN", "jira-token")
	t.Setenv("INPUT_TAG_FORMAT", "")
	t.Setenv("INPUT_RELEASE_NAME_FORMAT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ReleaseTag != "v3.0.0" {
		t.Errorf("ReleaseTag = %q, want %q (should come from event payload)", cfg.ReleaseTag, "v3.0.0")
	}
	if cfg.Version != "3.0.0" {
		t.Errorf("Version = %q, want %q", cfg.Version, "3.0.0")
	}
	if cfg.ReleaseBody == "" {
		t.Fatal("ReleaseBody is empty, should have multiline content from event payload")
	}
	if cfg.ReleaseBody == "should-be-overridden" {
		t.Error("ReleaseBody should come from event payload, not env var")
	}
}
