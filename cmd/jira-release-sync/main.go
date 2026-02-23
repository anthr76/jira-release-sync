package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/coreweave/jira-release-sync/internal/config"
	"github.com/coreweave/jira-release-sync/internal/github"
	"github.com/coreweave/jira-release-sync/internal/jira"
	"github.com/coreweave/jira-release-sync/internal/syncer"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	ghClient := github.NewClient(cfg.GitHubAPIURL, cfg.GitHubToken)
	jiraClient := jira.NewClient(cfg.JiraServer, cfg.JiraUser, cfg.JiraToken)

	s := syncer.New(cfg, ghClient, jiraClient, logger)
	if err := s.Run(context.Background()); err != nil {
		logger.Error("sync failed", "error", err)
		os.Exit(1)
	}

	logger.Info("release sync completed successfully")
}
