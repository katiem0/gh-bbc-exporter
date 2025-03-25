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
	Author            BitbucketPRUser     `json:"author"`
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

type BitbucketPRUser struct {
	DisplayName string `json:"display_name"`
	UUID        string `json:"uuid"`
	Nickname    string `json:"nickname"`
	AccountID   string `json:"account_id"`
}

type BitbucketUserResponse struct {
	Values []struct {
		User struct {
			AccountID   string `json:"account_id"`
			DisplayName string `json:"display_name"`
			Nickname    string `json:"nickname"`
			UUID        string `json:"uuid"`
			Links       struct {
				Self struct {
					Href string `json:"href"`
				} `json:"self"`
				HTML struct {
					Href string `json:"href"`
				} `json:"html"`
			} `json:"links"`
		} `json:"user"`
		Workspace struct {
			Slug string `json:"slug"`
			Name string `json:"name"`
		} `json:"workspace"`
	} `json:"values"`
	Next string `json:"next"`
}

type BitBucketCommentResponse struct {
	Values []struct {
		ID      int `json:"id"`
		Content struct {
			Raw string `json:"raw"`
		} `json:"content"`
		User struct {
			DisplayName string `json:"display_name"`
			UUID        string `json:"uuid"`
			Nickname    string `json:"nickname"`
			AccountID   string `json:"account_id"`
		} `json:"user"`
		CreatedOn string `json:"created_on"`
		UpdatedOn string `json:"updated_on"`
		Inline    *struct {
			From *int   `json:"from"`
			To   *int   `json:"to"`
			Path string `json:"path"`
		} `json:"inline"`
		ParentID int `json:"parent,omitempty"`
	} `json:"values"`
	Next string `json:"next"`
}
