package data

type BitbucketRepository struct {
	Name        string `json:"name"`
	UUID        string `json:"uuid"`
	FullName    string `json:"full_name"`
	Owner       Owner  `json:"owner"`
	Description string `json:"description"`
	CreatedOn   string `json:"created_on"`
	IsPrivate   bool   `json:"is_private"`
}

type Owner struct {
	Username string `json:"username"`
	UUID     string `json:"uuid"`
}

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

// TeamMember represents a team member
type TeamMember struct {
	User string `json:"user"`
	Role string `json:"role"`
}

type Repository struct {
	Type          string        `json:"type"`
	URL           string        `json:"url"`
	Owner         string        `json:"owner"`
	Name          string        `json:"name"`
	Description   string        `json:"description"`
	Private       bool          `json:"private"`
	HasIssues     bool          `json:"has_issues"`
	HasWiki       bool          `json:"has_wiki"`
	HasDownloads  bool          `json:"has_downloads"`
	Labels        []Label       `json:"labels"`
	Webhooks      []interface{} `json:"webhooks"`
	Collaborators []interface{} `json:"collaborators"`
	CreatedAt     string        `json:"created_at"`
	GitURL        string        `json:"git_url"`
	DefaultBranch string        `json:"default_branch"`
	WikiURL       string        `json:"wiki_url"`
	PublicKeys    []interface{} `json:"public_keys"`
}

type Label struct {
	Type      string `json:"type"`
	URL       string `json:"url"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	CreatedAt string `json:"created_at"`
}

type ProtectedBranch struct {
	Type                                 string   `json:"type"`
	Name                                 string   `json:"name"`
	URL                                  string   `json:"url"`
	CreatorURL                           string   `json:"creator_url"`
	RepositoryURL                        string   `json:"repository_url"`
	AdminEnforced                        bool     `json:"admin_enforced"`
	BlockDeletionsEnforcementLevel       int      `json:"block_deletions_enforcement_level"`
	BlockForcePushesEnforcementLevel     int      `json:"block_force_pushes_enforcement_level"`
	DismissStaleReviewsOnPush            bool     `json:"dismiss_stale_reviews_on_push"`
	PullRequestReviewsEnforcementLevel   string   `json:"pull_request_reviews_enforcement_level"`
	RequireCodeOwnerReview               bool     `json:"require_code_owner_review"`
	RequiredStatusChecksEnforcementLevel string   `json:"required_status_checks_enforcement_level"`
	StrictRequiredStatusChecksPolicy     bool     `json:"strict_required_status_checks_policy"`
	AuthorizedActorsOnly                 bool     `json:"authorized_actors_only"`
	AuthorizedUserURLs                   []string `json:"authorized_user_urls"`
	AuthorizedTeamURLs                   []string `json:"authorized_team_urls"`
	DismissalRestrictedUserURLs          []string `json:"dismissal_restricted_user_urls"`
	DismissalRestrictedTeamURLs          []string `json:"dismissal_restricted_team_urls"`
	RequiredStatusChecks                 []string `json:"required_status_checks"`
}

type PullRequest struct {
	Type           string   `json:"type"`
	URL            string   `json:"url"`
	User           string   `json:"user"`
	Repository     string   `json:"repository"`
	Title          string   `json:"title"`
	Body           string   `json:"body"`
	Base           PRBranch `json:"base"`
	Head           PRBranch `json:"head"`
	MergeCommitSha *string  `json:"merge_commit_sha"`
	Labels         []string `json:"labels"`
	MergedAt       *string  `json:"merged_at"`
	ClosedAt       *string  `json:"closed_at"`
	CreatedAt      string   `json:"created_at"`
	Assignee       *string  `json:"assignee"`
	Milestone      *string  `json:"milestone"`
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
	Type        string `json:"type"`
	URL         string `json:"url"`
	PullRequest string `json:"pull_request"`
	User        string `json:"user"`
	Body        string `json:"body"`
	Formatter   string `json:"formatter"`
	CreatedAt   string `json:"created_at"`
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

type BranchRestriction struct {
	BranchName string `json:"branch_name"`
	Kind       string `json:"kind"`
}

type CommitData struct {
	Type    string `json:"type"`
	Hash    string `json:"hash"`
	Date    string `json:"date"`
	Message string `json:"message"`
}

type BitbucketPRResponse struct {
	Size     int           `json:"size"`
	Page     int           `json:"page"`
	PageLen  int           `json:"pagelen"`
	Next     string        `json:"next"`
	Previous string        `json:"previous"`
	Values   []BitbucketPR `json:"values"`
}

type BitbucketPR struct {
	ID                int                 `json:"id"`
	Title             string              `json:"title"`
	Description       *string             `json:"description"`
	State             string              `json:"state"`
	CreatedOn         string              `json:"created_on"`
	UpdatedOn         string              `json:"updated_on"`
	CloseSourceBranch bool                `json:"close_source_branch"`
	Source            BitbucketPREndpoint `json:"source"`
	Destination       BitbucketPREndpoint `json:"destination"`
	MergeCommit       *BitbucketCommit    `json:"merge_commit"`
	Author            BitbucketUser       `json:"author"`
}

type BitbucketPREndpoint struct {
	Branch struct {
		Name string `json:"name"`
	} `json:"branch"`
	Commit struct {
		Hash string `json:"hash"`
	} `json:"commit"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

type BitbucketCommit struct {
	Hash string `json:"hash"`
}

type BitbucketUser struct {
	DisplayName string `json:"display_name"`
	UUID        string `json:"uuid"`
	Nickname    string `json:"nickname"`
	AccountID   string `json:"account_id"`
}
