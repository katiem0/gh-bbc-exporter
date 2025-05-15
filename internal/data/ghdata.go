package data

type MigrationArchiveSchema struct {
	Version string `json:"version"`
}

type URLs struct {
	User                 string           `json:"user"`
	Organization         string           `json:"organization"`
	Team                 string           `json:"team"`
	Repository           string           `json:"repository"`
	ProtectedBranch      string           `json:"protected_branch"`
	Milestone            string           `json:"milestone"`
	Issue                string           `json:"issue"`
	PullRequest          string           `json:"pull_request"`
	PullRequestReviewCmt string           `json:"pull_request_review_comment"`
	CommitComment        string           `json:"commit_comment"`
	IssueComment         IssueCommentURLs `json:"issue_comment"`
	Release              string           `json:"release"`
	Label                string           `json:"label"`
}

type IssueCommentURLs struct {
	Issue       string `json:"issue"`
	PullRequest string `json:"pull_request"`
}

type User struct {
	Type      string  `json:"type"`
	URL       string  `json:"url"`
	Login     string  `json:"login"`
	Name      string  `json:"name"`
	Company   *string `json:"company"`
	Website   *string `json:"website"`
	Location  *string `json:"location"`
	Emails    []Email `json:"emails"`
	CreatedAt string  `json:"created_at"`
}

type Email struct {
	Address string `json:"address"`
	Primary bool   `json:"primary"`
}

type Organization struct {
	Type        string   `json:"type"`
	URL         string   `json:"url"`
	Login       string   `json:"login"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Website     *string  `json:"website"`
	Location    *string  `json:"location"`
	Email       *string  `json:"email"`
	Members     []Member `json:"members"`
}

type Member struct {
	User  string `json:"user"`
	Role  string `json:"role"`
	State string `json:"state"`
}

type Team struct {
	Type         string       `json:"type"`
	URL          string       `json:"url"`
	Organization string       `json:"organization"`
	Name         string       `json:"name"`
	Description  *string      `json:"description"`
	Permissions  []Permission `json:"permissions"`
	Members      []TeamMember `json:"members"`
	CreatedAt    string       `json:"created_at"`
}

type Permission struct {
	Repository string `json:"repository"`
	Access     string `json:"access"`
}

type TeamMember struct {
	User string `json:"user"`
	Role string `json:"role"`
}

type Repository struct {
	Type                   string                 `json:"type"`
	URL                    string                 `json:"url"`
	Owner                  string                 `json:"owner"`
	Name                   string                 `json:"name"`
	Slug                   string                 `json:"slug"`
	Description            string                 `json:"description"`
	Private                bool                   `json:"private"`
	HasIssues              bool                   `json:"has_issues"`
	HasWiki                bool                   `json:"has_wiki"`
	HasDownloads           bool                   `json:"has_downloads"`
	Labels                 []Label                `json:"labels"`
	Webhooks               []interface{}          `json:"webhooks"`
	Collaborators          []interface{}          `json:"collaborators"`
	CreatedAt              string                 `json:"created_at"`
	GitURL                 string                 `json:"git_url"`
	DefaultBranch          string                 `json:"default_branch"`
	WikiURL                string                 `json:"wiki_url"`
	PublicKeys             []interface{}          `json:"public_keys"`
	RepositoryTopics       []interface{}          `json:"repository_topics,omitempty"`
	SecurityAndAnalysis    map[string]interface{} `json:"security_and_analysis,omitempty"`
	Autolinks              []interface{}          `json:"autolinks"`
	GeneralSettings        map[string]interface{} `json:"general_settings"`
	ActionsGeneralSettings map[string]interface{} `json:"actions_general_settings"`
	Website                *string                `json:"website"`
	Page                   *string                `json:"page"`
	IsArchived             bool                   `json:"is_archived"`
}

type Label struct {
	Type        string `json:"type,omitempty"`
	URL         string `json:"url"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"created_at"`
}

type PullRequest struct {
	Type                 string   `json:"type"`
	URL                  string   `json:"url"`
	User                 string   `json:"user"`
	Repository           string   `json:"repository"`
	Title                string   `json:"title"`
	Body                 string   `json:"body"`
	Base                 PRBranch `json:"base"`
	Head                 PRBranch `json:"head"`
	Labels               []string `json:"labels"`
	MergedAt             *string  `json:"merged_at"`
	ClosedAt             *string  `json:"closed_at"`
	CreatedAt            string   `json:"created_at"`
	Assignee             *string  `json:"assignee"`
	Assignees            []string `json:"assignees"`
	Milestone            *string  `json:"milestone"`
	Reactions            []string `json:"reactions"`
	ReviewRequests       []string `json:"review_requests"`
	CloseIssueReferences []string `json:"close_issue_references"`
	WorkInProgress       bool     `json:"work_in_progress"`
	MergeCommitSha       *string  `json:"merge_commit_sha"`
}

type PRBranch struct {
	Ref  string `json:"ref"`
	Sha  string `json:"sha"`
	User string `json:"user"`
	Repo string `json:"repo"`
}

type Issue struct {
	Type       string   `json:"type"`
	URL        string   `json:"url"`
	Repository string   `json:"repository"`
	User       string   `json:"user"`
	Title      string   `json:"title"`
	Body       *string  `json:"body"`
	Assignee   *string  `json:"assignee"`
	Milestone  *string  `json:"milestone"`
	Labels     []string `json:"labels"`
	ClosedAt   *string  `json:"closed_at"`
	CreatedAt  string   `json:"created_at"`
}

type IssueComment struct {
	Type        string   `json:"type"`
	URL         string   `json:"url"`
	User        string   `json:"user"`
	CreatedAt   string   `json:"created_at"`
	Formatter   string   `json:"formatter"`
	Reactions   []string `json:"reactions"`
	Body        string   `json:"body"`
	PullRequest string   `json:"pull_request"`
}

type PullRequestReviewComment struct {
	Type                    string   `json:"type"`
	URL                     string   `json:"url"`
	PullRequest             string   `json:"pull_request"`
	PullRequestReview       string   `json:"pull_request_review"`
	PullRequestReviewThread string   `json:"pull_request_review_thread"`
	Formatter               string   `json:"formatter"`
	DiffHunk                string   `json:"diff_hunk"`
	OriginalPosition        int      `json:"original_position"`
	OriginalCommitId        string   `json:"original_commit_id"`
	State                   int      `json:"state"`
	InReplyTo               *string  `json:"in_reply_to"`
	Reactions               []string `json:"reactions"`
	SubjectType             string   `json:"subject_type"`
	User                    string   `json:"user"`
	CommitID                string   `json:"commit_id"`
	Path                    string   `json:"path"`
	Position                int      `json:"position"`
	Body                    string   `json:"body"`
	CreatedAt               string   `json:"created_at"`
	UpdatedAt               string   `json:"updated_at"`
}

type MigrationArchive struct {
	RepositoryName string `json:"repository_name"`
	Owner          string `json:"owner"`
	Description    string `json:"description"`
	CreatedAt      string `json:"created_at"`
	Private        bool   `json:"private"`
}

type Branch struct {
	Name   string `json:"name"`
	IsMain bool   `json:"is_main"`
}

type CommitData struct {
	Type    string `json:"type"`
	Hash    string `json:"hash"`
	Date    string `json:"date"`
	Message string `json:"message"`
}
