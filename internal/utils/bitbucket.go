package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/katiem0/gh-bbc-exporter/internal/data"
	"go.uber.org/zap"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
	username   string
	appPass    string
	logger     *zap.Logger
}

func NewClient(baseURL, token, username, appPass string, logger *zap.Logger) *Client {
	baseURL = strings.TrimSuffix(baseURL, "/")

	if !strings.Contains(baseURL, "/2.0") && strings.Contains(baseURL, "api.bitbucket.org") {
		baseURL = baseURL + "/2.0"
	}

	logger.Info("Creating Bitbucket client", zap.String("baseURL", baseURL))
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{},
		token:      token,
		username:   username,
		appPass:    appPass,
		logger:     logger,
	}
}

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

func (c *Client) makeRequest(method, endpoint string, v interface{}) error {
	var fullURL string

	// Check if the endpoint already has the base URL
	if strings.HasPrefix(endpoint, c.baseURL) {
		fullURL = endpoint
	} else {
		// Handle endpoints with query parameters
		endpointPath := endpoint
		queryParams := ""

		if strings.Contains(endpoint, "?") {
			parts := strings.SplitN(endpoint, "?", 2)
			endpointPath = parts[0]
			queryParams = parts[1]
		}

		// Make sure there are no double slashes between baseURL and endpoint
		baseURL := strings.TrimSuffix(c.baseURL, "/")
		endpointPath = strings.TrimPrefix(endpointPath, "/")

		// Build the full URL
		if queryParams != "" {
			fullURL = fmt.Sprintf("%s/%s?%s", baseURL, endpointPath, queryParams)
		} else {
			fullURL = fmt.Sprintf("%s/%s", baseURL, endpointPath)
		}
	}

	c.logger.Debug("Making API request",
		zap.String("method", method),
		zap.String("url", fullURL))

	req, err := http.NewRequest(method, fullURL, nil)
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

	// Log the response status
	c.logger.Debug("API response",
		zap.Int("status", resp.StatusCode),
		zap.String("status_text", resp.Status))

	// Handle rate limiting
	if resp.StatusCode == 429 {
		// Read the response body to get any rate limit information
		bodyBytes, _ := io.ReadAll(resp.Body)
		rateLimitReset := resp.Header.Get("X-RateLimit-Reset")

		return fmt.Errorf("rate limit exceeded (429): reset at %s: %s",
			rateLimitReset, string(bodyBytes))
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s: %s",
			resp.StatusCode, resp.Status, string(bodyBytes))
	}

	return json.NewDecoder(resp.Body).Decode(v)
}

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

func (c *Client) GetIssues(workspace, repoSlug string) ([]data.Issue, error) {
	c.logger.Debug("Fetching issues",
		zap.String("workspace", workspace),
		zap.String("repository", repoSlug))

	// TODO: Implement real API call to fetch issues
	return []data.Issue{}, nil
}

func (c *Client) GetPullRequests(workspace, repoSlug string) ([]data.PullRequest, error) {
	c.logger.Info("Fetching pull requests",
		zap.String("workspace", workspace),
		zap.String("repository", repoSlug))

	var allPRs []data.PullRequest

	// Basic API endpoint
	baseEndpoint := fmt.Sprintf("repositories/%s/%s/pullrequests", workspace, repoSlug)

	// Add query parameters
	params := url.Values{}
	params.Add("state", "all")  // Get all PRs (open, closed, merged)
	params.Add("pagelen", "50") // Number of results per page

	// Build full endpoint with parameters
	fullEndpoint := baseEndpoint
	if len(params) > 0 {
		fullEndpoint = baseEndpoint + "?" + params.Encode()
	}

	// Add debug logging
	c.logger.Debug("Initial API request",
		zap.String("endpoint", fullEndpoint),
		zap.String("full_url", c.baseURL+"/"+fullEndpoint))

	page := 1
	for {
		var response data.BitbucketPRResponse

		var err error
		for retries := 0; retries < 3; retries++ {
			// Try to use our standard makeRequest first
			err = c.makeRequest("GET", fullEndpoint, &response)

			if err != nil {
				c.logger.Warn("API request error",
					zap.String("endpoint", fullEndpoint),
					zap.Int("retry", retries),
					zap.Error(err))

				// Sleep before retry
				time.Sleep(time.Duration(500*(retries+1)) * time.Millisecond)
				continue
			}
			break
		}

		if err != nil {
			return nil, fmt.Errorf("failed to fetch pull requests: %w", err)
		}

		// Log response details
		c.logger.Debug("Pull requests response",
			zap.Int("page", page),
			zap.Int("values_count", len(response.Values)),
			zap.String("next_url", response.Next))

		// Process pull requests from this page
		for _, pr := range response.Values {
			var mergedAt, closedAt *string
			if pr.State == "MERGED" {
				// Format dates in ISO 8601 Z format
				mergedStr := formatDateToZ(pr.UpdatedOn)
				mergedAt = &mergedStr
				closedStr := formatDateToZ(pr.UpdatedOn)
				closedAt = &closedStr
			} else if pr.State == "DECLINED" {
				closedStr := formatDateToZ(pr.UpdatedOn)
				closedAt = &closedStr
			}

			prURL := fmt.Sprintf("https://bitbucket.org/%s/%s/pull-requests/%d",
				workspace, repoSlug, pr.ID)

			// userHandle := pr.Author.Nickname
			// if userHandle == "" {
			// 	if pr.Author.DisplayName != "" {
			// 		userHandle = strings.ToLower(strings.ReplaceAll(pr.Author.DisplayName, " ", ""))
			// 	} else {
			// 		userHandle = pr.Author.UUID
			// 	}
			// }
			userURL := fmt.Sprintf("https://bitbucket.org/%s", pr.Author.UUID)
			repoURL := fmt.Sprintf("https://bitbucket.org/%s/%s", workspace, repoSlug)
			prUser := fmt.Sprintf("https://bitbucket.org/%s", workspace)
			baseSHA, _ := c.GetCommitSha(workspace, repoSlug, pr.Destination.Commit.Hash)
			headSHA, _ := c.GetCommitSha(workspace, repoSlug, pr.Source.Commit.Hash)

			var mergeCommitSha *string
			if pr.MergeCommit != nil && pr.MergeCommit.Hash != "" {
				hash := pr.MergeCommit.Hash
				mergeCommitSha = &hash
			}

			var body string
			if pr.Description != nil {
				body = *pr.Description
			}
			createdAt := formatDateToZ(pr.CreatedOn)

			// Create the pull request with format matching the expected structure
			newPR := data.PullRequest{
				Type:           "pull_request",
				URL:            prURL,
				User:           userURL,
				Repository:     repoURL,
				Title:          pr.Title,
				Body:           body,
				MergeCommitSha: mergeCommitSha,
				Base: data.PRBranch{
					Ref:  pr.Destination.Branch.Name,
					Sha:  baseSHA,
					User: prUser,
					Repo: repoURL,
				},
				Head: data.PRBranch{
					Ref:  pr.Source.Branch.Name,
					Sha:  headSHA,
					User: prUser,
					Repo: repoURL,
				},
				Labels:    []string{},
				MergedAt:  mergedAt,
				ClosedAt:  closedAt,
				CreatedAt: createdAt,
				Assignee:  nil,
				Milestone: nil,
			}

			allPRs = append(allPRs, newPR)
		}

		// Check if there are more pages
		if response.Next == "" {
			break
		}

		// For next page, use the next URL from the response
		// We need to extract just the path part
		nextURL, err := url.Parse(response.Next)
		if err != nil {
			c.logger.Warn("Failed to parse next URL", zap.Error(err))
			break
		}

		// Extract just the path and query string
		fullEndpoint = strings.TrimPrefix(nextURL.Path, "/")
		if nextURL.RawQuery != "" {
			fullEndpoint += "?" + nextURL.RawQuery
		}

		// If endpoint contains the base URL, strip it out
		baseURLStr, _ := url.Parse(c.baseURL)
		if baseURLStr != nil && baseURLStr.Path != "" && strings.HasPrefix(fullEndpoint, baseURLStr.Path) {
			fullEndpoint = strings.TrimPrefix(fullEndpoint, baseURLStr.Path)
			fullEndpoint = strings.TrimPrefix(fullEndpoint, "/")
		}

		page++

		// Add a small delay between pages
		time.Sleep(200 * time.Millisecond)
	}

	c.logger.Info("Fetched all pull requests", zap.Int("total", len(allPRs)))
	return allPRs, nil
}

func (c *Client) GetComments(workspace, repoSlug string) ([]data.IssueComment, error) {
	c.logger.Debug("Fetching comments",
		zap.String("workspace", workspace),
		zap.String("repository", repoSlug))

	// TODO: Implement real API call to fetch comments
	return []data.IssueComment{}, nil
}

func (c *Client) GetBranches(workspace, repoSlug string) ([]data.Branch, error) {
	c.logger.Debug("Fetching branches",
		zap.String("workspace", workspace),
		zap.String("repository", repoSlug))

	// TODO: Implement real API call to fetch branches
	return []data.Branch{}, nil
}

func (c *Client) GetBranchRestrictions(workspace, repoSlug string) ([]data.BranchRestriction, error) {
	c.logger.Debug("Fetching branch restrictions",
		zap.String("workspace", workspace),
		zap.String("repository", repoSlug))

	// TODO: Implement real API call to fetch branch restrictions
	return []data.BranchRestriction{}, nil
}

func (c *Client) GetCommitSha(workspace, repoSlug string, commit string) (string, error) {
	endpoint := fmt.Sprintf("/repositories/%s/%s/commit/%s", workspace, repoSlug, commit)

	c.logger.Debug("Fetching commit SHA",
		zap.String("workspace", workspace),
		zap.String("repository", repoSlug),
		zap.String("commit", commit))

	var response data.CommitData

	err := c.makeRequest("GET", endpoint, &response)
	if err != nil {
		return "", err
	}

	return response.Hash, nil
}
