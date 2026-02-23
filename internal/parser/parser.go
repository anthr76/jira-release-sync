package parser

import (
	"regexp"
	"strings"
)

// jiraPattern matches "jira: https://<domain>/browse/KEY-123" in PR bodies.
// It captures the issue key (e.g., "PRJ-456").
var jiraPattern = regexp.MustCompile(`(?i)jira:\s*https?://[^\s/]+/browse/([A-Z][A-Z0-9]+-\d+)`)

// ExtractJiraKeys parses a PR body and returns all unique Jira issue keys found.
func ExtractJiraKeys(body string) []string {
	matches := jiraPattern.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{})
	var keys []string
	for _, m := range matches {
		key := strings.ToUpper(m[1])
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
	}
	return keys
}
