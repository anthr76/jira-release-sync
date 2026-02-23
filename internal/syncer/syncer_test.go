package syncer

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/coreweave/jira-release-sync/internal/config"
	"github.com/coreweave/jira-release-sync/internal/github"
	"github.com/coreweave/jira-release-sync/internal/jira"
)

func TestRun_FullWorkflow(t *testing.T) {
	ghMux := http.NewServeMux()

	ghMux.HandleFunc("GET /repos/owner/repo/tags", func(w http.ResponseWriter, r *http.Request) {
		tags := []github.Tag{
			{Name: "v1.1.0"},
			{Name: "v1.0.0"},
		}
		json.NewEncoder(w).Encode(tags)
	})

	ghMux.HandleFunc("GET /repos/owner/repo/compare/v1.0.0...v1.1.0", func(w http.ResponseWriter, r *http.Request) {
		resp := github.CompareResponse{
			Commits: []github.CommitEntry{
				{SHA: "aaa"},
				{SHA: "bbb"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	ghMux.HandleFunc("GET /repos/owner/repo/commits/aaa/pulls", func(w http.ResponseWriter, r *http.Request) {
		prs := []github.PullRequest{
			{Number: 1, Body: "feat: something\n\njira: https://jira.example.com/browse/PRJ-100"},
		}
		json.NewEncoder(w).Encode(prs)
	})

	ghMux.HandleFunc("GET /repos/owner/repo/commits/bbb/pulls", func(w http.ResponseWriter, r *http.Request) {
		prs := []github.PullRequest{
			{Number: 2, Body: "fix: bug\n\njira: https://jira.example.com/browse/PRJ-200"},
		}
		json.NewEncoder(w).Encode(prs)
	})

	ghMux.HandleFunc("GET /repos/owner/repo/releases/tags/v1.0.0", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(github.Release{TagName: "v1.0.0", PublishedAt: "2025-01-15T10:00:00Z"})
	})

	ghMux.HandleFunc("GET /repos/owner/repo/releases/tags/v1.1.0", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(github.Release{TagName: "v1.1.0", PublishedAt: "2025-02-20T14:30:00Z"})
	})

	ghSrv := httptest.NewServer(ghMux)
	defer ghSrv.Close()

	jiraMux := http.NewServeMux()

	jiraMux.HandleFunc("GET /rest/api/3/project/PRJ/versions", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]jira.Version{}) // empty, will create
	})

	jiraMux.HandleFunc("POST /rest/api/3/version", func(w http.ResponseWriter, r *http.Request) {
		var req jira.CreateVersionRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Name != "1.1.0" {
			t.Errorf("version name = %q, want %q", req.Name, "1.1.0")
		}
		if req.Project != "PRJ" {
			t.Errorf("project = %q", req.Project)
		}
		if !strings.Contains(req.Description, "changelog") {
			t.Errorf("description missing changelog content: %q", req.Description)
		}
		if req.StartDate != "2025-01-15" {
			t.Errorf("startDate = %q, want %q", req.StartDate, "2025-01-15")
		}
		if req.ReleaseDate != "2025-02-20" {
			t.Errorf("releaseDate = %q, want %q", req.ReleaseDate, "2025-02-20")
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(jira.Version{ID: "42", Name: "1.1.0"})
	})

	updatedIssues := make(map[string]bool)
	jiraMux.HandleFunc("PUT /rest/api/3/issue/", func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/rest/api/3/issue/")
		updatedIssues[key] = true
		w.WriteHeader(http.StatusNoContent)
	})

	jiraSrv := httptest.NewServer(jiraMux)
	defer jiraSrv.Close()

	cfg := &config.Config{
		GitHubToken:       "token",
		GitHubRepository:  "owner/repo",
		GitHubAPIURL:      ghSrv.URL,
		ReleaseTag:        "v1.1.0",
		ReleaseBody:       "## changelog\n- feat: something",
		JiraServer:        jiraSrv.URL,
		JiraProject:       "PRJ",
		JiraUser:          "user",
		JiraToken:         "token",
		ReleaseNameFormat: "{version}",
		RepoOwner:         "owner",
		RepoName:          "repo",
		Version:           "1.1.0",
	}

	ghClient := github.NewClient(ghSrv.URL, "token")
	jiraClient := jira.NewClient(jiraSrv.URL, "user", "token")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	s := New(cfg, ghClient, jiraClient, logger)
	if err := s.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !updatedIssues["PRJ-100"] {
		t.Error("PRJ-100 was not updated")
	}
	if !updatedIssues["PRJ-200"] {
		t.Error("PRJ-200 was not updated")
	}
}

func TestRun_ExistingVersion(t *testing.T) {
	ghMux := http.NewServeMux()

	ghMux.HandleFunc("GET /repos/owner/repo/tags", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]github.Tag{{Name: "v2.0.0"}, {Name: "v1.0.0"}})
	})

	ghMux.HandleFunc("GET /repos/owner/repo/compare/v1.0.0...v2.0.0", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(github.CompareResponse{
			Commits: []github.CommitEntry{{SHA: "ccc"}},
		})
	})

	ghMux.HandleFunc("GET /repos/owner/repo/commits/ccc/pulls", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]github.PullRequest{
			{Number: 10, Body: "jira: https://jira.test/browse/FOO-1"},
		})
	})

	ghMux.HandleFunc("GET /repos/owner/repo/releases/tags/v1.0.0", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(github.Release{TagName: "v1.0.0", PublishedAt: "2025-01-01T00:00:00Z"})
	})

	ghMux.HandleFunc("GET /repos/owner/repo/releases/tags/v2.0.0", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(github.Release{TagName: "v2.0.0", PublishedAt: "2025-06-01T00:00:00Z"})
	})

	ghSrv := httptest.NewServer(ghMux)
	defer ghSrv.Close()

	jiraMux := http.NewServeMux()

	jiraMux.HandleFunc("GET /rest/api/3/project/FOO/versions", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]jira.Version{{ID: "99", Name: "2.0.0"}})
	})

	jiraMux.HandleFunc("PUT /rest/api/3/issue/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	jiraSrv := httptest.NewServer(jiraMux)
	defer jiraSrv.Close()

	cfg := &config.Config{
		ReleaseTag:        "v2.0.0",
		ReleaseBody:       "body",
		JiraProject:       "FOO",
		ReleaseNameFormat: "{version}",
		RepoOwner:         "owner",
		RepoName:          "repo",
		Version:           "2.0.0",
	}

	s := New(cfg, github.NewClient(ghSrv.URL, "t"), jira.NewClient(jiraSrv.URL, "u", "t"),
		slog.New(slog.NewTextHandler(os.Stderr, nil)))

	if err := s.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_NoCommits(t *testing.T) {
	ghMux := http.NewServeMux()

	ghMux.HandleFunc("GET /repos/o/r/tags", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]github.Tag{{Name: "v1.1.0"}, {Name: "v1.0.0"}})
	})

	ghMux.HandleFunc("GET /repos/o/r/compare/v1.0.0...v1.1.0", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(github.CompareResponse{Commits: nil})
	})

	ghMux.HandleFunc("GET /repos/o/r/releases/tags/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	ghSrv := httptest.NewServer(ghMux)
	defer ghSrv.Close()

	jiraMux := http.NewServeMux()
	jiraMux.HandleFunc("GET /rest/api/3/project/NC/versions", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]jira.Version{})
	})
	jiraMux.HandleFunc("POST /rest/api/3/version", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(jira.Version{ID: "1", Name: "1.1.0"})
	})
	jiraSrv := httptest.NewServer(jiraMux)
	defer jiraSrv.Close()

	cfg := &config.Config{
		ReleaseTag:        "v1.1.0",
		ReleaseNameFormat: "{version}",
		JiraProject:       "NC",
		RepoOwner:         "o",
		RepoName:          "r",
		Version:           "1.1.0",
	}

	s := New(cfg, github.NewClient(ghSrv.URL, "t"), jira.NewClient(jiraSrv.URL, "u", "t"),
		slog.New(slog.NewTextHandler(os.Stderr, nil)))

	if err := s.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_NoJiraKeys(t *testing.T) {
	ghMux := http.NewServeMux()

	ghMux.HandleFunc("GET /repos/o/r/tags", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]github.Tag{{Name: "v1.1.0"}, {Name: "v1.0.0"}})
	})

	ghMux.HandleFunc("GET /repos/o/r/compare/v1.0.0...v1.1.0", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(github.CompareResponse{
			Commits: []github.CommitEntry{{SHA: "ddd"}},
		})
	})

	ghMux.HandleFunc("GET /repos/o/r/commits/ddd/pulls", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]github.PullRequest{
			{Number: 5, Body: "no jira links here"},
		})
	})

	ghMux.HandleFunc("GET /repos/o/r/releases/tags/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	ghSrv := httptest.NewServer(ghMux)
	defer ghSrv.Close()

	jiraMux := http.NewServeMux()
	jiraMux.HandleFunc("GET /rest/api/3/project/NK/versions", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]jira.Version{})
	})
	jiraMux.HandleFunc("POST /rest/api/3/version", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(jira.Version{ID: "1", Name: "1.1.0"})
	})
	jiraSrv := httptest.NewServer(jiraMux)
	defer jiraSrv.Close()

	cfg := &config.Config{
		ReleaseTag:        "v1.1.0",
		ReleaseNameFormat: "{version}",
		JiraProject:       "NK",
		RepoOwner:         "o",
		RepoName:          "r",
		Version:           "1.1.0",
	}

	s := New(cfg, github.NewClient(ghSrv.URL, "t"), jira.NewClient(jiraSrv.URL, "u", "t"),
		slog.New(slog.NewTextHandler(os.Stderr, nil)))

	if err := s.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_PartialFailure(t *testing.T) {
	ghMux := http.NewServeMux()

	ghMux.HandleFunc("GET /repos/o/r/tags", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]github.Tag{{Name: "v1.0.0"}, {Name: "v0.9.0"}})
	})

	ghMux.HandleFunc("GET /repos/o/r/compare/v0.9.0...v1.0.0", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(github.CompareResponse{
			Commits: []github.CommitEntry{{SHA: "eee"}},
		})
	})

	ghMux.HandleFunc("GET /repos/o/r/commits/eee/pulls", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]github.PullRequest{
			{Number: 1, Body: "jira: https://j.co/browse/XX-1\njira: https://j.co/browse/XX-2"},
		})
	})

	ghMux.HandleFunc("GET /repos/o/r/releases/tags/v0.9.0", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(github.Release{TagName: "v0.9.0", PublishedAt: "2025-03-01T00:00:00Z"})
	})

	ghMux.HandleFunc("GET /repos/o/r/releases/tags/v1.0.0", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(github.Release{TagName: "v1.0.0", PublishedAt: "2025-04-01T00:00:00Z"})
	})

	ghSrv := httptest.NewServer(ghMux)
	defer ghSrv.Close()

	jiraMux := http.NewServeMux()

	jiraMux.HandleFunc("GET /rest/api/3/project/XX/versions", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]jira.Version{})
	})

	jiraMux.HandleFunc("POST /rest/api/3/version", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(jira.Version{ID: "1", Name: "1.0.0"})
	})

	jiraMux.HandleFunc("PUT /rest/api/3/issue/", func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/rest/api/3/issue/")
		if key == "XX-1" {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error":"forbidden"}`))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	jiraSrv := httptest.NewServer(jiraMux)
	defer jiraSrv.Close()

	cfg := &config.Config{
		ReleaseTag:        "v1.0.0",
		ReleaseBody:       "body",
		JiraProject:       "XX",
		ReleaseNameFormat: "{version}",
		RepoOwner:         "o",
		RepoName:          "r",
		Version:           "1.0.0",
	}

	s := New(cfg, github.NewClient(ghSrv.URL, "t"), jira.NewClient(jiraSrv.URL, "u", "t"),
		slog.New(slog.NewTextHandler(os.Stderr, nil)))

	err := s.Run(context.Background())
	if err == nil {
		t.Fatal("expected error for partial failure")
	}
	if !strings.Contains(err.Error(), "failed to update 1 of 2") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRun_DeduplicatesPRsAcrossCommits(t *testing.T) {
	ghMux := http.NewServeMux()

	ghMux.HandleFunc("GET /repos/o/r/tags", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]github.Tag{{Name: "v1.0.0"}, {Name: "v0.9.0"}})
	})

	ghMux.HandleFunc("GET /repos/o/r/compare/v0.9.0...v1.0.0", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(github.CompareResponse{
			Commits: []github.CommitEntry{{SHA: "aaa"}, {SHA: "bbb"}},
		})
	})

	prHandler := func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]github.PullRequest{
			{Number: 10, Body: "jira: https://j.co/browse/DUP-1"},
		})
	}
	ghMux.HandleFunc("GET /repos/o/r/commits/aaa/pulls", prHandler)
	ghMux.HandleFunc("GET /repos/o/r/commits/bbb/pulls", prHandler)

	ghMux.HandleFunc("GET /repos/o/r/releases/tags/v0.9.0", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(github.Release{TagName: "v0.9.0", PublishedAt: "2025-01-01T00:00:00Z"})
	})

	ghMux.HandleFunc("GET /repos/o/r/releases/tags/v1.0.0", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(github.Release{TagName: "v1.0.0", PublishedAt: "2025-02-01T00:00:00Z"})
	})

	ghSrv := httptest.NewServer(ghMux)
	defer ghSrv.Close()

	jiraMux := http.NewServeMux()
	jiraMux.HandleFunc("GET /rest/api/3/project/DUP/versions", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]jira.Version{})
	})
	jiraMux.HandleFunc("POST /rest/api/3/version", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(jira.Version{ID: "1", Name: "1.0.0"})
	})

	updateCount := 0
	jiraMux.HandleFunc("PUT /rest/api/3/issue/", func(w http.ResponseWriter, r *http.Request) {
		updateCount++
		w.WriteHeader(http.StatusNoContent)
	})

	jiraSrv := httptest.NewServer(jiraMux)
	defer jiraSrv.Close()

	cfg := &config.Config{
		ReleaseTag:        "v1.0.0",
		ReleaseBody:       "body",
		JiraProject:       "DUP",
		ReleaseNameFormat: "{version}",
		RepoOwner:         "o",
		RepoName:          "r",
		Version:           "1.0.0",
	}

	s := New(cfg, github.NewClient(ghSrv.URL, "t"), jira.NewClient(jiraSrv.URL, "u", "t"),
		slog.New(slog.NewTextHandler(os.Stderr, nil)))

	if err := s.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updateCount != 1 {
		t.Errorf("expected 1 issue update (deduplicated), got %d", updateCount)
	}
}

func TestRun_FirstRelease(t *testing.T) {
	ghMux := http.NewServeMux()

	call := 0
	ghMux.HandleFunc("GET /repos/o/r/tags", func(w http.ResponseWriter, r *http.Request) {
		if call == 0 {
			call++
			json.NewEncoder(w).Encode([]github.Tag{{Name: "v1.0.0"}})
		} else {
			json.NewEncoder(w).Encode([]github.Tag{})
		}
	})

	ghMux.HandleFunc("GET /repos/o/r/releases/tags/v1.0.0", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(github.Release{TagName: "v1.0.0", PublishedAt: "2025-05-01T10:00:00Z"})
	})

	ghSrv := httptest.NewServer(ghMux)
	defer ghSrv.Close()

	jiraMux := http.NewServeMux()

	jiraMux.HandleFunc("GET /rest/api/3/project/FIRST/versions", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]jira.Version{})
	})

	var createdVersion jira.CreateVersionRequest
	jiraMux.HandleFunc("POST /rest/api/3/version", func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&createdVersion)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(jira.Version{ID: "1", Name: "1.0.0"})
	})

	jiraSrv := httptest.NewServer(jiraMux)
	defer jiraSrv.Close()

	cfg := &config.Config{
		ReleaseTag:        "v1.0.0",
		ReleaseBody:       "## First release\n- initial",
		JiraProject:       "FIRST",
		ReleaseNameFormat: "{version}",
		RepoOwner:         "o",
		RepoName:          "r",
		Version:           "1.0.0",
	}

	s := New(cfg, github.NewClient(ghSrv.URL, "t"), jira.NewClient(jiraSrv.URL, "u", "t"),
		slog.New(slog.NewTextHandler(os.Stderr, nil)))

	if err := s.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if createdVersion.Name != "1.0.0" {
		t.Errorf("version name = %q, want %q", createdVersion.Name, "1.0.0")
	}
	if createdVersion.ReleaseDate != "2025-05-01" {
		t.Errorf("releaseDate = %q, want %q", createdVersion.ReleaseDate, "2025-05-01")
	}
	if createdVersion.StartDate != "" {
		t.Errorf("startDate = %q, want empty", createdVersion.StartDate)
	}
}
