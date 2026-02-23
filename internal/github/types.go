package github

// Tag represents a GitHub tag from the tags API.
type Tag struct {
	Name string `json:"name"`
}

// CompareResponse is the response from the compare API.
type CompareResponse struct {
	Commits []CommitEntry `json:"commits"`
}

// CommitEntry is a single commit in a compare response.
type CommitEntry struct {
	SHA string `json:"sha"`
}

// PullRequest represents a PR associated with a commit.
type PullRequest struct {
	Number int    `json:"number"`
	Body   string `json:"body"`
	Merged bool   `json:"merged_at"`
	State  string `json:"state"`
}

// Release represents a GitHub release.
type Release struct {
	TagName     string `json:"tag_name"`
	PublishedAt string `json:"published_at"`
}
