package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/katiem0/gh-bbc-exporter/internal/data"
	"go.uber.org/zap"
)

// Client represents a BitBucket API client
type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
	username   string
	appPass    string
	logger     *zap.Logger
}

// NewClient creates a new BitBucket API client
func NewClient(baseURL, token, username, appPass string, logger *zap.Logger) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{},
		token:      token,
		username:   username,
		appPass:    appPass,
		logger:     logger,
	}
}

// GetRepository retrieves repository information
func (c *Client) GetRepository(workspace, repoSlug string) (*data.BitbucketRepository, error) {
	endpoint := fmt.Sprintf("/repositories/%s/%s", workspace, repoSlug)

	c.logger.Debug("Fetching repository",
		zap.String("workspace", workspace),
		zap.String("repository", repoSlug))

	var repo data.BitbucketRepository
	err := c.makeRequest("GET", endpoint, &repo)
	if err != nil {
		return nil, err
	}

	return &repo, nil
}

// makeRequest performs an HTTP request against the BitBucket API
func (c *Client) makeRequest(method, endpoint string, v interface{}) error {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return err
	}
	u.Path = path.Join(u.Path, endpoint)

	req, err := http.NewRequest(method, u.String(), nil)
	if err != nil {
		return err
	}

	// Set authentication headers
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	} else if c.username != "" && c.appPass != "" {
		req.SetBasicAuth(c.username, c.appPass)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status: %s", resp.Status)
	}

	return json.NewDecoder(resp.Body).Decode(v)
}

// GetUsers retrieves users for a repository
func (c *Client) GetUsers(workspace, repoSlug string) ([]data.User, error) {
	c.logger.Debug("Fetching users",
		zap.String("workspace", workspace),
		zap.String("repository", repoSlug))

	// TODO: Implement real API call to fetch users
	// For now, return a basic workspace user
	return []data.User{
		{
			Type:      "user",
			URL:       fmt.Sprintf("https://bitbucket.org/%s", workspace),
			Login:     workspace,
			Name:      workspace,
			Company:   nil,
			Website:   nil,
			Location:  nil,
			Emails:    []data.Email{},
			CreatedAt: time.Now().Format("2006-01-02T15:04:05.000Z"),
		},
	}, nil
}

// GetIssues retrieves issues for a repository
func (c *Client) GetIssues(workspace, repoSlug string) ([]data.Issue, error) {
	c.logger.Debug("Fetching issues",
		zap.String("workspace", workspace),
		zap.String("repository", repoSlug))

	// TODO: Implement real API call to fetch issues
	return []data.Issue{}, nil
}

// GetPullRequests retrieves pull requests for a repository
func (c *Client) GetPullRequests(workspace, repoSlug string) ([]data.PullRequest, error) {
	c.logger.Debug("Fetching pull requests",
		zap.String("workspace", workspace),
		zap.String("repository", repoSlug))

	// TODO: Implement real API call to fetch PRs
	return []data.PullRequest{}, nil
}

// GetComments retrieves comments for a repository
func (c *Client) GetComments(workspace, repoSlug string) ([]data.IssueComment, error) {
	c.logger.Debug("Fetching comments",
		zap.String("workspace", workspace),
		zap.String("repository", repoSlug))

	// TODO: Implement real API call to fetch comments
	return []data.IssueComment{}, nil
}

// GetBranches retrieves branches for a repository
func (c *Client) GetBranches(workspace, repoSlug string) ([]data.Branch, error) {
	c.logger.Debug("Fetching branches",
		zap.String("workspace", workspace),
		zap.String("repository", repoSlug))

	// TODO: Implement real API call to fetch branches
	return []data.Branch{}, nil
}

// GetBranchRestrictions retrieves branch restrictions for a repository
func (c *Client) GetBranchRestrictions(workspace, repoSlug string) ([]data.BranchRestriction, error) {
	c.logger.Debug("Fetching branch restrictions",
		zap.String("workspace", workspace),
		zap.String("repository", repoSlug))

	// TODO: Implement real API call to fetch branch restrictions
	return []data.BranchRestriction{}, nil
}
