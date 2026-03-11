package syncer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/coreweave/jira-release-sync/internal/config"
	"github.com/coreweave/jira-release-sync/internal/github"
	"github.com/coreweave/jira-release-sync/internal/jira"
	"github.com/coreweave/jira-release-sync/internal/parser"
)

// Syncer orchestrates the release-to-Jira sync workflow.
type Syncer struct {
	cfg    *config.Config
	gh     *github.Client
	jiraC  *jira.Client
	logger *slog.Logger
}

// New creates a new Syncer.
func New(cfg *config.Config, gh *github.Client, jiraC *jira.Client, logger *slog.Logger) *Syncer {
	return &Syncer{
		cfg:    cfg,
		gh:     gh,
		jiraC:  jiraC,
		logger: logger,
	}
}

// Run executes the full sync workflow.
func (s *Syncer) Run(ctx context.Context) error {
	releaseName := config.FormatReleaseName(s.cfg.ReleaseNameFormat, s.cfg.Version)
	s.logger.Info("starting release sync",
		"tag", s.cfg.ReleaseTag,
		"version", s.cfg.Version,
		"releaseName", releaseName,
	)

	prevTag, err := s.gh.FindPreviousTag(ctx, s.cfg.RepoOwner, s.cfg.RepoName, s.cfg.ReleaseTag)
	if err != nil && !errors.Is(err, github.ErrNoPreviousTag) {
		return fmt.Errorf("finding previous tag: %w", err)
	}

	var jiraKeys []string

	if errors.Is(err, github.ErrNoPreviousTag) {
		s.logger.Info("first release, no previous tag to compare against")
	} else {
		s.logger.Info("found previous tag", "previous", prevTag, "current", s.cfg.ReleaseTag)

		commits, err := s.gh.CompareCommits(ctx, s.cfg.RepoOwner, s.cfg.RepoName, prevTag, s.cfg.ReleaseTag)
		if err != nil {
			return fmt.Errorf("comparing commits: %w", err)
		}
		s.logger.Info("found commits between tags", "count", len(commits))

		if len(commits) > 0 {
			jiraKeys, err = s.collectJiraKeys(ctx, commits)
			if err != nil {
				return err
			}
		}
	}

	if len(jiraKeys) > 0 {
		s.logger.Info("collected Jira issue keys", "keys", jiraKeys)
	}

	startDate, releaseDate, description := s.resolveVersionDates(ctx, prevTag)

	versionName, err := s.ensureJiraVersion(ctx, releaseName, startDate, releaseDate, description)
	if err != nil {
		return fmt.Errorf("ensuring Jira version: %w", err)
	}

	if len(jiraKeys) == 0 {
		s.logger.Info("no Jira issue keys to update")
		return nil
	}
	return s.updateIssues(ctx, jiraKeys, versionName)
}

func (s *Syncer) collectJiraKeys(ctx context.Context, commits []github.CommitEntry) ([]string, error) {
	seenPRs := make(map[int]struct{})
	seenKeys := make(map[string]struct{})
	var keys []string

	for _, commit := range commits {
		prs, err := s.gh.ListPullRequestsForCommit(ctx, s.cfg.RepoOwner, s.cfg.RepoName, commit.SHA)
		if err != nil {
			s.logger.Warn("failed to list PRs for commit, skipping", "sha", commit.SHA, "error", err)
			continue
		}

		for _, pr := range prs {
			if _, seen := seenPRs[pr.Number]; seen {
				continue
			}
			seenPRs[pr.Number] = struct{}{}

			extracted := parser.ExtractJiraKeys(pr.Body)
			for _, key := range extracted {
				if _, seen := seenKeys[key]; !seen {
					seenKeys[key] = struct{}{}
					keys = append(keys, key)
				}
			}
		}
	}

	return keys, nil
}

// resolveVersionDates returns the start date (previous release's published_at),
// release date (current release's published_at), and a description built from
// the current release body. Any value may be empty if the release cannot be fetched.
func (s *Syncer) resolveVersionDates(ctx context.Context, prevTag string) (startDate, releaseDate, description string) {
	if prevTag != "" {
		prevRelease, err := s.gh.GetReleaseByTag(ctx, s.cfg.RepoOwner, s.cfg.RepoName, prevTag)
		if err != nil {
			s.logger.Warn("could not fetch previous release for start date", "tag", prevTag, "error", err)
		} else if d := toDateString(prevRelease.PublishedAt); d != "" {
			startDate = d
			s.logger.Info("resolved version start date from previous release", "startDate", startDate)
		}
	}

	curRelease, err := s.gh.GetReleaseByTag(ctx, s.cfg.RepoOwner, s.cfg.RepoName, s.cfg.ReleaseTag)
	if err != nil {
		s.logger.Warn("could not fetch current release for release date", "tag", s.cfg.ReleaseTag, "error", err)
	} else {
		if d := toDateString(curRelease.PublishedAt); d != "" {
			releaseDate = d
			s.logger.Info("resolved version release date from current release", "releaseDate", releaseDate)
		}
		if curRelease.HTMLURL != "" {
			description = curRelease.HTMLURL
		}
	}

	return startDate, releaseDate, description
}

// toDateString parses an ISO 8601 timestamp and returns the YYYY-MM-DD portion.
func toDateString(iso8601 string) string {
	if iso8601 == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, iso8601)
	if err != nil {
		return ""
	}
	return t.Format("2006-01-02")
}

func (s *Syncer) ensureJiraVersion(ctx context.Context, releaseName, startDate, releaseDate, description string) (string, error) {
	versions, err := s.jiraC.GetProjectVersions(ctx, s.cfg.JiraProject)
	if err != nil {
		return "", fmt.Errorf("getting project versions: %w", err)
	}

	for _, v := range versions {
		if v.Name == releaseName {
			s.logger.Info("Jira version already exists", "name", releaseName, "id", v.ID)
			if description != "" && v.Description != description {
				if err := s.jiraC.UpdateVersion(ctx, v.ID, jira.UpdateVersionRequest{
					Description: description,
				}); err != nil {
					s.logger.Warn("failed to update version description", "id", v.ID, "error", err)
				} else {
					s.logger.Info("updated version description", "id", v.ID)
				}
			}
			return releaseName, nil
		}
	}

	const maxDesc = 16384
	if len(description) > maxDesc {
		description = description[:maxDesc]
	}

	created, err := s.jiraC.CreateVersion(ctx, jira.CreateVersionRequest{
		Name:        releaseName,
		Description: description,
		Project:     s.cfg.JiraProject,
		Released:    true,
		StartDate:   startDate,
		ReleaseDate: releaseDate,
	})
	if err != nil {
		return "", fmt.Errorf("creating version: %w", err)
	}
	s.logger.Info("created Jira version", "name", created.Name, "id", created.ID)
	return created.Name, nil
}

func (s *Syncer) updateIssues(ctx context.Context, keys []string, versionName string) error {
	var failures int
	for _, key := range keys {
		if err := s.jiraC.AddFixVersion(ctx, key, versionName); err != nil {
			s.logger.Error("failed to add fix version to issue", "issue", key, "error", err)
			failures++
			continue
		}
		s.logger.Info("added fix version to issue", "issue", key, "version", versionName)
	}

	if failures > 0 {
		return fmt.Errorf("failed to update %d of %d issues", failures, len(keys))
	}
	return nil
}
