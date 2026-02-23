package jira

// Version represents a Jira project version.
type Version struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ProjectKey  string `json:"project,omitempty"`
	Released    bool   `json:"released,omitempty"`
	Archived    bool   `json:"archived,omitempty"`
}

// CreateVersionRequest is the payload for creating a Jira version.
type CreateVersionRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Project     string `json:"project"`
	Released    bool   `json:"released"`
	StartDate   string `json:"startDate,omitempty"`   // YYYY-MM-DD
	ReleaseDate string `json:"releaseDate,omitempty"` // YYYY-MM-DD
}

// IssueUpdateRequest is the payload for updating an issue's fix versions.
type IssueUpdateRequest struct {
	Update IssueUpdateFields `json:"update"`
}

// IssueUpdateFields contains the update operations.
type IssueUpdateFields struct {
	FixVersions []FixVersionOp `json:"fixVersions"`
}

// FixVersionOp represents an add operation for fix versions.
type FixVersionOp struct {
	Add *VersionRef `json:"add,omitempty"`
}

// VersionRef is a reference to a version by name.
type VersionRef struct {
	Name string `json:"name"`
}
