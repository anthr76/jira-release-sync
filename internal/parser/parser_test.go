package parser

import (
	"reflect"
	"testing"
)

func TestExtractJiraKeys(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "single key",
			body: "Some PR description\n\njira: https://jira.example.com/browse/PRJ-123",
			want: []string{"PRJ-123"},
		},
		{
			name: "multiple keys",
			body: "Fix things\n\njira: https://jira.example.com/browse/PRJ-123\njira: https://jira.example.com/browse/PRJ-456",
			want: []string{"PRJ-123", "PRJ-456"},
		},
		{
			name: "duplicate keys deduplicated",
			body: "jira: https://jira.example.com/browse/PRJ-123\njira: https://jira.example.com/browse/PRJ-123",
			want: []string{"PRJ-123"},
		},
		{
			name: "no keys",
			body: "Just a normal PR body with no Jira references",
			want: nil,
		},
		{
			name: "mixed content",
			body: "## Description\nFixed the bug\n\n## References\njira: https://company.atlassian.net/browse/TEAM-42\n\nSome other text",
			want: []string{"TEAM-42"},
		},
		{
			name: "case insensitive jira prefix",
			body: "Jira: https://jira.example.com/browse/FOO-1\nJIRA: https://jira.example.com/browse/FOO-2",
			want: []string{"FOO-1", "FOO-2"},
		},
		{
			name: "http without tls",
			body: "jira: http://jira.internal/browse/INT-99",
			want: []string{"INT-99"},
		},
		{
			name: "empty body",
			body: "",
			want: nil,
		},
		{
			name: "key with large number",
			body: "jira: https://jira.example.com/browse/BIG-99999",
			want: []string{"BIG-99999"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractJiraKeys(tt.body)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExtractJiraKeys() = %v, want %v", got, tt.want)
			}
		})
	}
}
