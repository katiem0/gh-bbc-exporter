package data

type GHESMigrationPullRequest struct {
	Number             string   `json:"number"`
	ProfileName        string   `json:"profile_name"`
	BaseRefName        string   `json:"base_ref_name"`
	HeadRefName        string   `json:"head_ref_name"`
	BaseRefCommitSha   string   `json:"base_ref_commit_sha"`
	HeadRefCommitSha   string   `json:"head_ref_commit_sha"`
	IsDraft            bool     `json:"is_draft"`
	SourceURL          string   `json:"source_url"`
	AuthorLogin        string   `json:"author_login"`
	Title              string   `json:"title"`
	Body               string   `json:"body"`
	CreatedAt          string   `json:"created_at"`
	MergedAt           string   `json:"merged_at,omitempty"`
	ClosedAt           string   `json:"closed_at,omitempty"`
	IsSquashMerge      bool     `json:"is_squash_merge"`
	Labels             []string `json:"labels"`
	MilestoneID        string   `json:"milestone_id,omitempty"`
	ResourceIdentifier string   `json:"resource_identifier"`
	MergeCommitSha     string   `json:"merge_commit_sha,omitempty"`
	Assignees          []string `json:"assignees"`
}
