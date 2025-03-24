package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
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
		c.logger.Error("Failed to fetch repository details",
			zap.Error(err),
			zap.String("endpoint", endpoint))
		return nil, err
	}
	if repo.MainBranch != nil {
		c.logger.Info("Repository main branch",
			zap.String("branch", repo.MainBranch.Name))
	} else {
		c.logger.Info("Repository has no main branch defined")
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
	c.logger.Info("Fetching workspace members",
		zap.String("workspace", workspace))

	var allUsers []data.User
	page := 1
	pageLen := 100
	hasMore := true

	for hasMore {
		endpoint := fmt.Sprintf("workspaces/%s/members?page=%d&pagelen=%d",
			workspace, page, pageLen)

		var response struct {
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

		err := c.makeRequest("GET", endpoint, &response)
		if err != nil {
			c.logger.Warn("Failed to fetch workspace members, using fallback user",
				zap.Error(err))
			return []data.User{
				{
					Type:      "user",
					URL:       fmt.Sprintf("https://bitbucket.org/%s", workspace),
					Login:     workspace,
					Name:      workspace,
					Company:   nil,
					Website:   nil,
					Location:  nil,
					Emails:    nil,
					CreatedAt: formatDateToZ(time.Now().Format(time.RFC3339)),
				},
			}, nil
		}

		for _, member := range response.Values {
			user := member.User

			profileURL := fmt.Sprintf("https://bitbucket.org/%s", user.DisplayName)

			newUser := data.User{
				Type:      "user",
				URL:       profileURL,
				Login:     user.DisplayName,
				Name:      user.DisplayName,
				Company:   nil,
				Website:   nil,
				Location:  nil,
				Emails:    nil,
				CreatedAt: formatDateToZ(time.Now().Format(time.RFC3339)),
			}

			allUsers = append(allUsers, newUser)
		}

		hasMore = response.Next != ""
		if hasMore {
			page++
		}
	}
	if len(allUsers) == 0 {
		c.logger.Warn("No workspace members found, using fallback user")
		allUsers = append(allUsers, data.User{
			Type:      "user",
			URL:       fmt.Sprintf("https://bitbucket.org/%s", workspace),
			Login:     workspace,
			Name:      workspace,
			Company:   nil,
			Website:   nil,
			Location:  nil,
			Emails:    nil,
			CreatedAt: formatDateToZ(time.Now().Format(time.RFC3339)),
		})
	}

	c.logger.Info("Fetched workspace members",
		zap.Int("count", len(allUsers)))

	return allUsers, nil
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

	baseEndpoint := fmt.Sprintf("repositories/%s/%s/pullrequests", workspace, repoSlug)

	params := url.Values{}
	params.Add("state", "all")
	params.Add("pagelen", "50")

	fullEndpoint := baseEndpoint
	if len(params) > 0 {
		fullEndpoint = baseEndpoint + "?" + params.Encode()
	}

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

		c.logger.Debug("Pull requests response",
			zap.Int("page", page),
			zap.Int("values_count", len(response.Values)),
			zap.String("next_url", response.Next))

		for _, pr := range response.Values {
			var mergedAt, closedAt *string
			if pr.State == "MERGED" {
				mergedStr := formatDateToZ(pr.UpdatedOn)
				mergedAt = &mergedStr
				closedStr := formatDateToZ(pr.UpdatedOn)
				closedAt = &closedStr
			} else if pr.State == "DECLINED" {
				closedStr := formatDateToZ(pr.UpdatedOn)
				closedAt = &closedStr
			}

			prURL := fmt.Sprintf("https://bitbucket.org/%s/%s/merge_requests/%d",
				workspace, repoSlug, pr.ID)

			userURL := fmt.Sprintf("https://bitbucket.org/%s", pr.Author.DisplayName)
			repoURL := fmt.Sprintf("https://bitbucket.org/%s/%s", workspace, repoSlug)
			prUser := fmt.Sprintf("https://bitbucket.org/%s", workspace)

			baseSHA := pr.Destination.Commit.Hash
			headSHA := pr.Source.Commit.Hash

			if len(baseSHA) < 40 {
				fullBaseSHA, err := c.GetFullCommitSHA(workspace, repoSlug, baseSHA)
				if err == nil {
					baseSHA = fullBaseSHA
				} else {
					c.logger.Warn("Failed to get full base commit SHA",
						zap.String("original", baseSHA),
						zap.Error(err))
				}
			}

			if len(headSHA) < 40 {
				fullHeadSHA, err := c.GetFullCommitSHA(workspace, repoSlug, headSHA)
				if err == nil {
					headSHA = fullHeadSHA
				} else {
					c.logger.Warn("Failed to get full head commit SHA",
						zap.String("original", headSHA),
						zap.Error(err))
				}
			}

			var body string
			if pr.Description != nil {
				body = *pr.Description
			}

			createdAt := formatDateToZ(pr.CreatedOn)
			newPR := data.PullRequest{
				Type:       "pull_request",
				URL:        prURL,
				User:       userURL,
				Repository: repoURL,
				Title:      pr.Title,
				Body:       body,
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
		if response.Next == "" {
			break
		}
		nextURL, err := url.Parse(response.Next)
		if err != nil {
			c.logger.Warn("Failed to parse next URL", zap.Error(err))
			break
		}

		fullEndpoint = strings.TrimPrefix(nextURL.Path, "/")
		if nextURL.RawQuery != "" {
			fullEndpoint += "?" + nextURL.RawQuery
		}

		baseURLStr, _ := url.Parse(c.baseURL)
		if baseURLStr != nil && baseURLStr.Path != "" && strings.HasPrefix(fullEndpoint, baseURLStr.Path) {
			fullEndpoint = strings.TrimPrefix(fullEndpoint, baseURLStr.Path)
			fullEndpoint = strings.TrimPrefix(fullEndpoint, "/")
		}
		page++

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

	var allRestrictions []data.BranchRestriction
	page := 1
	pageLen := 100
	hasMore := true

	for hasMore {
		endpoint := fmt.Sprintf("repositories/%s/%s/branch-restrictions?page=%d&pagelen=%d",
			workspace, repoSlug, page, pageLen)

		var response struct {
			Values []struct {
				ID         int    `json:"id"`
				Kind       string `json:"kind"`
				Pattern    string `json:"pattern"`
				Type       string `json:"type"`
				BranchType string `json:"branch_type"`
				Users      []struct {
					DisplayName string `json:"display_name"`
					UUID        string `json:"uuid"`
					Links       struct {
						Self struct {
							Href string `json:"href"`
						} `json:"self"`
					} `json:"links"`
				} `json:"users"`
				Groups []struct {
					Name  string `json:"name"`
					Links struct {
						Self struct {
							Href string `json:"href"`
						} `json:"self"`
					} `json:"links"`
				} `json:"groups"`
			} `json:"values"`
			Next string `json:"next"`
		}

		err := c.makeRequest("GET", endpoint, &response)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch branch restrictions: %w", err)
		}

		// Convert API response to our data model
		for _, restriction := range response.Values {
			branchName := restriction.Pattern
			// BitBucket uses ** for wildcard patterns like **master**
			// Remove wildcards to get the actual branch name
			branchName = strings.Replace(branchName, "**", "", -1)

			// Map restriction kinds to our model
			restrictionType := ""
			switch restriction.Kind {
			case "push":
				restrictionType = "push"
			case "force":
				restrictionType = "force_push"
			case "delete":
				restrictionType = "delete"
			case "require_approvals_to_merge":
				restrictionType = "require_reviews"
			case "require_default_reviewer_approvals_to_merge":
				restrictionType = "require_code_owner_review"
			}

			// Skip if we don't map this restriction type
			if restrictionType == "" {
				continue
			}

			// Add users who can bypass this restriction
			authorizedUsers := []string{}
			for _, user := range restriction.Users {
				authorizedUsers = append(authorizedUsers, user.Links.Self.Href)
			}

			allRestrictions = append(allRestrictions, data.BranchRestriction{
				ID:            restriction.ID,
				Type:          restrictionType,
				BranchPattern: branchName,
				Users:         authorizedUsers,
			})
		}

		// Check if there are more pages
		hasMore = response.Next != ""
		if hasMore {
			page++
		}
	}

	return allRestrictions, nil
}

func (c *Client) GetFullCommitSHA(workspace, repoSlug, commitHash string) (string, error) {
	if len(commitHash) == 40 {
		return commitHash, nil
	}

	endpoint := fmt.Sprintf("repositories/%s/%s/commit/%s", workspace, repoSlug, commitHash)

	var response struct {
		Hash string `json:"hash"`
	}

	err := c.makeRequest("GET", endpoint, &response)
	if err != nil {
		return commitHash, fmt.Errorf("failed to fetch full commit SHA: %w", err)
	}

	if len(response.Hash) == 40 {
		return response.Hash, nil
	}

	return commitHash, nil
}

func (c *Client) GetPullRequestComments(workspace, repoSlug string) ([]data.IssueComment, []data.PullRequestReviewComment, error) {
	c.logger.Info("Fetching pull request comments",
		zap.String("workspace", workspace),
		zap.String("repository", repoSlug))

	var regularComments []data.IssueComment
	var reviewComments []data.PullRequestReviewComment

	pullRequests, err := c.GetPullRequests(workspace, repoSlug)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch pull requests: %w", err)
	}

	prURLMap := make(map[int]string)
	prCommitMap := make(map[int]string)
	for _, pr := range pullRequests {
		parts := strings.Split(pr.URL, "/")
		if len(parts) > 0 {
			prID, err := strconv.Atoi(parts[len(parts)-1])
			if err == nil {
				prURLMap[prID] = pr.URL
				prCommitMap[prID] = pr.Head.Sha
			}
		}
	}
	for prID, prURL := range prURLMap {
		page := 1
		pageLen := 100
		hasMore := true

		for hasMore {
			endpoint := fmt.Sprintf("repositories/%s/%s/pullrequests/%d/comments?page=%d&pagelen=%d",
				workspace, repoSlug, prID, page, pageLen)

			var response struct {
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
						Links       struct {
							Self struct {
								Href string `json:"href"`
							} `json:"self"`
						} `json:"links"`
					} `json:"user"`
					CreatedOn string `json:"created_on"`
					UpdatedOn string `json:"updated_on"`
					Links     struct {
						Self struct {
							Href string `json:"href"`
						} `json:"self"`
					} `json:"links"`
					Inline *struct {
						From *int   `json:"from"`
						To   *int   `json:"to"`
						Path string `json:"path"`
					} `json:"inline"`
					ParentID int `json:"parent,omitempty"`
				} `json:"values"`
				Next string `json:"next"`
			}

			err := c.makeRequest("GET", endpoint, &response)
			if err != nil {
				c.logger.Warn("Failed to fetch PR comments",
					zap.Int("pr_id", prID),
					zap.Error(err))
				break
			}

			for _, comment := range response.Values {
				// Determine user URL
				var userHandle string
				if comment.User.Nickname != "" {
					userHandle = comment.User.Nickname
				} else if comment.User.AccountID != "" {
					userHandle = comment.User.AccountID
				} else if comment.User.DisplayName != "" {
					userHandle = strings.ToLower(strings.ReplaceAll(comment.User.DisplayName, " ", "-"))
				} else {
					userHandle = strings.Trim(comment.User.UUID, "{}")
				}
				userURL := fmt.Sprintf("https://bitbucket.org/%s", userHandle)

				// Format timestamps
				createdAt := formatDateToZ(comment.CreatedOn)
				updatedAt := formatDateToZ(comment.UpdatedOn)

				// Transform PR references in body
				transformedBody := c.transformCommentBody(comment.Content.Raw, workspace, repoSlug)

				// Check if this is an inline comment
				if comment.Inline != nil && comment.Inline.Path != "" {
					// This is an inline review comment
					lineNumber := 1
					if comment.Inline.To != nil {
						lineNumber = *comment.Inline.To
					} else if comment.Inline.From != nil {
						lineNumber = *comment.Inline.From
					}

					// Create review comment with correct format
					reviewComment := data.PullRequestReviewComment{
						Type:        "pull_request_review_comment",
						URL:         fmt.Sprintf("%s/diffs#note_%d", prURL, comment.ID),
						PullRequest: prURL,
						User:        userURL,
						CommitID:    prCommitMap[prID], // Use the PR's head commit SHA
						Path:        comment.Inline.Path,
						Position:    lineNumber,
						Body:        transformedBody,
						CreatedAt:   createdAt,
						UpdatedAt:   updatedAt,
					}

					reviewComments = append(reviewComments, reviewComment)
				} else {
					// This is a regular PR comment
					regularComment := data.IssueComment{
						Type:        "issue_comment",
						URL:         fmt.Sprintf("%s#note_%d", prURL, comment.ID),
						User:        userURL,
						CreatedAt:   createdAt,
						UpdatedAt:   updatedAt,
						IssueURL:    prURL,
						Body:        transformedBody,
						PullRequest: prURL,
					}

					regularComments = append(regularComments, regularComment)
				}
			}

			// Check for more pages
			hasMore = response.Next != ""
			if hasMore {
				page++
			}
		}
	}

	c.logger.Info("Fetched pull request comments",
		zap.Int("regular_comments", len(regularComments)),
		zap.Int("review_comments", len(reviewComments)),
		zap.String("repository", repoSlug))

	return regularComments, reviewComments, nil
}

func (c *Client) transformCommentBody(body, workspace, repoSlug string) string {
	if body == "" {
		return body
	}

	pattern := fmt.Sprintf("https://bitbucket.org/%s/%s/pull-requests/(\\d+)",
		regexp.QuoteMeta(workspace), regexp.QuoteMeta(repoSlug))
	replacement := fmt.Sprintf("https://bitbucket.org/%s/%s/merge_requests/$1",
		workspace, repoSlug)

	re := regexp.MustCompile(pattern)
	transformedBody := re.ReplaceAllString(body, replacement)

	prPattern := fmt.Sprintf(`\b#(\d+)\b`)

	prRe := regexp.MustCompile(prPattern)
	transformedBody = prRe.ReplaceAllStringFunc(transformedBody, func(match string) string {
		numStr := match[1:] // Remove the # prefix
		return fmt.Sprintf("[%s](%s)", match, fmt.Sprintf("https://bitbucket.org/%s/%s/merge_requests/%s",
			workspace, repoSlug, numStr))
	})

	return transformedBody
}
