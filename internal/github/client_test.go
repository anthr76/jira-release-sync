package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFindPreviousTag(t *testing.T) {
	tags := []Tag{
		{Name: "v1.3.0"},
		{Name: "v1.2.0"},
		{Name: "v1.1.0"},
		{Name: "v1.0.0"},
	}

	call := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing auth header")
		}
		if call == 0 {
			call++
			json.NewEncoder(w).Encode(tags)
		} else {
			json.NewEncoder(w).Encode([]Tag{})
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token")
	ctx := context.Background()

	prev, err := client.FindPreviousTag(ctx, "owner", "repo", "v1.2.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prev != "v1.1.0" {
		t.Errorf("got %q, want %q", prev, "v1.1.0")
	}
}

func TestFindPreviousTag_NotFound(t *testing.T) {
	call := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if call == 0 {
			call++
			json.NewEncoder(w).Encode([]Tag{{Name: "v1.0.0"}})
		} else {
			json.NewEncoder(w).Encode([]Tag{})
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "token")
	_, err := client.FindPreviousTag(context.Background(), "o", "r", "v2.0.0")
	if err == nil {
		t.Fatal("expected error for missing tag")
	}
}

func TestFindPreviousTag_NoPrevious(t *testing.T) {
	call := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if call == 0 {
			call++
			json.NewEncoder(w).Encode([]Tag{{Name: "v1.0.0"}})
		} else {
			json.NewEncoder(w).Encode([]Tag{})
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "token")
	_, err := client.FindPreviousTag(context.Background(), "o", "r", "v1.0.0")
	if err == nil {
		t.Fatal("expected error for no previous tag")
	}
	if !errors.Is(err, ErrNoPreviousTag) {
		t.Errorf("expected ErrNoPreviousTag, got: %v", err)
	}
}

func TestCompareCommits(t *testing.T) {
	resp := CompareResponse{
		Commits: []CommitEntry{
			{SHA: "abc123"},
			{SHA: "def456"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := "/repos/owner/repo/compare/v1.0.0...v1.1.0"
		if r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "token")
	commits, err := client.CompareCommits(context.Background(), "owner", "repo", "v1.0.0", "v1.1.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("got %d commits, want 2", len(commits))
	}
	if commits[0].SHA != "abc123" {
		t.Errorf("commit SHA = %q, want abc123", commits[0].SHA)
	}
}

func TestListPullRequestsForCommit(t *testing.T) {
	prs := []PullRequest{
		{Number: 42, Body: "jira: https://jira.example.com/browse/PRJ-100", State: "closed"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := "/repos/owner/repo/commits/abc123/pulls"
		if r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		json.NewEncoder(w).Encode(prs)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "token")
	result, err := client.ListPullRequestsForCommit(context.Background(), "owner", "repo", "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("got %d PRs, want 1", len(result))
	}
	if result[0].Number != 42 {
		t.Errorf("PR number = %d, want 42", result[0].Number)
	}
}

func TestGetReleaseByTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := "/repos/owner/repo/releases/tags/v1.0.0"
		if r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		json.NewEncoder(w).Encode(Release{TagName: "v1.0.0", PublishedAt: "2025-03-15T12:00:00Z"})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "token")
	release, err := client.GetReleaseByTag(context.Background(), "owner", "repo", "v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if release.TagName != "v1.0.0" {
		t.Errorf("tag = %q", release.TagName)
	}
	if release.PublishedAt != "2025-03-15T12:00:00Z" {
		t.Errorf("published_at = %q", release.PublishedAt)
	}
}

func TestGetReleaseByTag_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Not Found"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "token")
	_, err := client.GetReleaseByTag(context.Background(), "owner", "repo", "v99.0.0")
	if err == nil {
		t.Fatal("expected error for missing release")
	}
}

func TestGet_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Not Found"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "token")
	var out []Tag
	err := client.get(context.Background(), srv.URL+"/test", &out)
	if err == nil {
		t.Fatal("expected error for 404")
	}
}
