package data

type CmdFlags struct {
	BitbucketToken   string
	BitbucketUser    string
	BitbucketAppPass string
	BitbucketAPIURL  string
	Repository       string
	Workspace        string
	OutputDir        string
	Debug            bool
}

type BitbucketRepository struct {
	Name        string `json:"name"`
	UUID        string `json:"uuid"`
	FullName    string `json:"full_name"`
	Owner       Owner  `json:"owner"`
	Description string `json:"description"`
	CreatedOn   string `json:"created_on"`
	IsPrivate   bool   `json:"is_private"`
	MainBranch  *struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"mainbranch"`
}

type Owner struct {
	Username string `json:"username"`
	UUID     string `json:"uuid"`
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
