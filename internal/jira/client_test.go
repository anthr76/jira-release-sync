package jira

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetProjectVersions(t *testing.T) {
	versions := []Version{
		{ID: "1", Name: "1.0.0"},
		{ID: "2", Name: "1.1.0"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/rest/api/3/project/PRJ/versions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "user@test.com" || pass != "token123" {
			t.Error("bad auth")
		}
		json.NewEncoder(w).Encode(versions)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "user@test.com", "token123")
	result, err := client.GetProjectVersions(context.Background(), "PRJ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("got %d versions, want 2", len(result))
	}
	if result[0].Name != "1.0.0" {
		t.Errorf("version name = %q", result[0].Name)
	}
}

func TestCreateVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/rest/api/3/version" {
			t.Errorf("path = %s", r.URL.Path)
		}

		var req CreateVersionRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Name != "1.2.0" {
			t.Errorf("name = %q", req.Name)
		}
		if req.Project != "PRJ" {
			t.Errorf("project = %q", req.Project)
		}
		if req.Description != "changelog here" {
			t.Errorf("description = %q", req.Description)
		}
		if req.StartDate != "2025-01-15" {
			t.Errorf("startDate = %q, want %q", req.StartDate, "2025-01-15")
		}
		if req.ReleaseDate != "2025-02-20" {
			t.Errorf("releaseDate = %q, want %q", req.ReleaseDate, "2025-02-20")
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Version{ID: "10", Name: "1.2.0"})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "user", "token")
	v, err := client.CreateVersion(context.Background(), CreateVersionRequest{
		Name:        "1.2.0",
		Description: "changelog here",
		Project:     "PRJ",
		StartDate:   "2025-01-15",
		ReleaseDate: "2025-02-20",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.ID != "10" {
		t.Errorf("id = %q", v.ID)
	}
}

func TestAddFixVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/rest/api/3/issue/PRJ-123" {
			t.Errorf("path = %s", r.URL.Path)
		}

		body, _ := io.ReadAll(r.Body)
		var req IssueUpdateRequest
		json.Unmarshal(body, &req)

		if len(req.Update.FixVersions) != 1 {
			t.Fatalf("expected 1 fix version op, got %d", len(req.Update.FixVersions))
		}
		if req.Update.FixVersions[0].Add.Name != "1.2.0" {
			t.Errorf("version name = %q", req.Update.FixVersions[0].Add.Name)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "user", "token")
	err := client.AddFixVersion(context.Background(), "PRJ-123", "1.2.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddFixVersion_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"errorMessages":["forbidden"]}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "user", "token")
	err := client.AddFixVersion(context.Background(), "PRJ-999", "1.0.0")
	if err == nil {
		t.Fatal("expected error for 403")
	}
}

func TestGetProjectVersions_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "user", "token")
	_, err := client.GetProjectVersions(context.Background(), "PRJ")
	if err == nil {
		t.Fatal("expected error for 500")
	}
}
