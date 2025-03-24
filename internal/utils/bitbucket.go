package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

			profileURL := fmt.Sprintf("https://bitbucket.org/%s", strings.Trim(user.UUID, "{}"))

			newUser := data.User{
				Type:      "user",
				URL:       profileURL,
				Login:     strings.Trim(user.UUID, "{}"),
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

func (c *Client) GetPullRequests(workspace, repoSlug string) ([]data.PullRequest, error) {
	c.logger.Info("Fetching pull requests",
		zap.String("workspace", workspace),
		zap.String("repository", repoSlug))

	var pullRequests []data.PullRequest
	page := 1
	pageLen := 50
	hasMore := true

	// Add a commit SHA cache to avoid redundant API calls
	commitSHACache := make(map[string]string)

	// Helper function to get full SHA with caching
	getFullSHA := func(shortSHA string) string {
		if len(shortSHA) == 40 {
			return shortSHA
		}

		// Check cache first
		if fullSHA, exists := commitSHACache[shortSHA]; exists {
			return fullSHA
		}

		// Make API call if not in cache
		fullSHA, err := c.GetFullCommitSHA(workspace, repoSlug, shortSHA)
		if err == nil && len(fullSHA) == 40 {
			// Cache the result
			commitSHACache[shortSHA] = fullSHA
			return fullSHA
		}

		c.logger.Warn("Failed to get full commit SHA",
			zap.String("original", shortSHA),
			zap.Error(err))

		return shortSHA
	}

	for hasMore {
		endpoint := fmt.Sprintf("repositories/%s/%s/pullrequests?page=%d&pagelen=%d&state=ALL",
			workspace, repoSlug, page, pageLen)

		var response data.BitbucketPRResponse
		var err error

		// Retry logic for API requests
		maxRetries := 3
		for retries := 0; retries < maxRetries; retries++ {
			err = c.makeRequest("GET", endpoint, &response)
			if err != nil {
				c.logger.Warn("API request error",
					zap.String("endpoint", endpoint),
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

			// Use GitHub-style URLs for compatibility with GHES format
			prURL := fmt.Sprintf("https://bitbucket.org/%s/%s/pull/%d",
				workspace, repoSlug, pr.ID)

			userURL := fmt.Sprintf("https://bitbucket.org/%s",
				strings.Trim(pr.Author.UUID, "{}"))
			repoURL := fmt.Sprintf("https://bitbucket.org/%s/%s", workspace, repoSlug)
			prUser := fmt.Sprintf("https://bitbucket.org/%s", workspace)

			baseSHA := pr.Destination.Commit.Hash
			headSHA := pr.Source.Commit.Hash

			// Get full SHA values
			baseSHA = getFullSHA(baseSHA)
			headSHA = getFullSHA(headSHA)

			// Extract description or use empty string if nil
			description := ""
			if pr.Description != nil {
				description = *pr.Description
			}

			// Format merge commit SHA if available
			var mergeCommitSha *string
			if pr.MergeCommit != nil && pr.State == "MERGED" {
				fullMergeSHA := getFullSHA(pr.MergeCommit.Hash)
				mergeCommitSha = &fullMergeSHA
			}

			// Create empty labels for PR
			labels := []string{}

			// Create the Pull Request with GitHub-compatible structure
			pullRequest := data.PullRequest{
				Type:       "pull_request",
				URL:        prURL,
				User:       userURL,
				Repository: repoURL,
				Title:      pr.Title,
				Body:       description,
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
				Labels:               labels,
				MergedAt:             mergedAt,
				ClosedAt:             closedAt,
				CreatedAt:            formatDateToZ(pr.CreatedOn),
				Assignee:             nil,
				Assignees:            []string{},
				Milestone:            nil,
				Reactions:            []string{},
				ReviewRequests:       []string{},
				CloseIssueReferences: []string{},
				WorkInProgress:       false,
				MergeCommitSha:       mergeCommitSha,
			}

			pullRequests = append(pullRequests, pullRequest)
		}

		hasMore = response.Next != ""
		if hasMore {
			page++
		}
	}

	return pullRequests, nil
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

	for prID, _ := range prURLMap {
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

			err := c.makeRequest("GET", endpoint, &response)
			if err != nil {
				c.logger.Warn("Failed to fetch PR comments",
					zap.Int("pr_id", prID),
					zap.Error(err))
				break
			}

			for _, comment := range response.Values {
				// Format GitHub-style user URL - use UUID as username
				userURL := fmt.Sprintf("https://bitbucket.org/%s", strings.Trim(comment.User.UUID, "{}"))

				// Format timestamps
				createdAt := formatDateToZ(comment.CreatedOn)
				updatedAt := formatDateToZ(comment.UpdatedOn)

				// Transform PR references in body
				transformedBody := c.transformCommentBody(comment.Content.Raw, workspace, repoSlug)

				// Extract PR number from URL for reference generation
				prNumber := fmt.Sprintf("%d", prID)

				// Check if this is an inline comment
				if comment.Inline != nil && comment.Inline.Path != "" {
					// This is an inline review comment
					lineNumber := 1
					if comment.Inline.To != nil {
						lineNumber = *comment.Inline.To
					} else if comment.Inline.From != nil {
						lineNumber = *comment.Inline.From
					}

					// Generate unique IDs for comment and thread
					commentId := fmt.Sprintf("%d", comment.ID)
					reviewId := fmt.Sprintf("%d", comment.ID) // Use same ID for review
					threadId := fmt.Sprintf("%d", comment.ID) // Use same ID for thread

					// Generate GitHub-style URLs
					commentURL := fmt.Sprintf("https://bitbucket.org/%s/%s/pull/%s/files#r%s",
						workspace, repoSlug, prNumber, commentId)
					reviewURL := fmt.Sprintf("https://bitbucket.org/%s/%s/pull/%s/files#pullrequestreview-%s",
						workspace, repoSlug, prNumber, reviewId)
					threadURL := fmt.Sprintf("https://bitbucket.org/%s/%s/pull/%s/files#pullrequestreviewthread-%s",
						workspace, repoSlug, prNumber, threadId)
					prFullURL := fmt.Sprintf("https://bitbucket.org/%s/%s/pull/%s",
						workspace, repoSlug, prNumber)

					// Get full commit SHA
					commitSHA := prCommitMap[prID]
					if len(commitSHA) < 40 {
						fullCommitSHA, err := c.GetFullCommitSHA(workspace, repoSlug, commitSHA)
						if err == nil {
							commitSHA = fullCommitSHA
						}
					}

					// Create diff hunk
					diffHunk := fmt.Sprintf("@@ -0,0 +1,%d @@\n+%s", lineNumber, transformedBody)

					// Create review comment with correct format
					reviewComment := data.PullRequestReviewComment{
						Type:                    "pull_request_review_comment",
						URL:                     commentURL,
						PullRequest:             prFullURL,
						PullRequestReview:       reviewURL,
						PullRequestReviewThread: threadURL,
						User:                    userURL,
						CommitID:                commitSHA,
						OriginalCommitId:        commitSHA,
						Path:                    comment.Inline.Path,
						Position:                lineNumber,
						OriginalPosition:        lineNumber,
						Body:                    transformedBody,
						CreatedAt:               createdAt,
						UpdatedAt:               updatedAt,
						Formatter:               "markdown",
						DiffHunk:                diffHunk,
						State:                   1, // Active state
						InReplyTo:               nil,
						Reactions:               []string{},
						SubjectType:             "line",
					}

					reviewComments = append(reviewComments, reviewComment)
				} else {

					regularComment := data.IssueComment{
						Type:        "issue_comment",
						URL:         fmt.Sprintf("https://bitbucket.org/%s/%s/pull/%s#issuecomment-%d", workspace, repoSlug, prNumber, comment.ID),
						User:        userURL,
						Body:        transformedBody,
						CreatedAt:   createdAt,
						Formatter:   "markdown",
						Reactions:   []string{},
						PullRequest: fmt.Sprintf("https://bitbucket.org/%s/%s/pull/%s", workspace, repoSlug, prNumber),
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

func (e *Exporter) createReviewThreads(comments []data.PullRequestReviewComment, workspace, repoSlug string) []map[string]interface{} {
	var threads []map[string]interface{}

	for _, comment := range comments {

		thread := map[string]interface{}{
			"type":                  "pull_request_review_thread",
			"url":                   comment.PullRequestReviewThread,
			"pull_request":          comment.PullRequest,
			"pull_request_review":   comment.PullRequestReview,
			"diff_hunk":             comment.DiffHunk,
			"path":                  comment.Path,
			"position":              comment.Position,
			"original_position":     comment.OriginalPosition,
			"commit_id":             comment.CommitID,
			"original_commit_id":    comment.OriginalCommitId,
			"start_position_offset": nil,
			"blob_position":         comment.Position - 1,
			"start_line":            nil,
			"line":                  comment.Position,
			"start_side":            nil,
			"side":                  "right",
			"original_start_line":   nil,
			"original_line":         comment.OriginalPosition,
			"created_at":            comment.CreatedAt,
			"resolved_at":           nil,
			"resolver":              nil,
			"subject_type":          comment.SubjectType,
			"outdated":              false,
		}

		threads = append(threads, thread)
	}

	return threads
}

func (e *Exporter) createReviews(comments []data.PullRequestReviewComment, workspace, repoSlug string) []map[string]interface{} {
	// Group comments by PR review URL
	commentsByReview := make(map[string][]data.PullRequestReviewComment)

	for _, comment := range comments {
		key := comment.PullRequestReview
		commentsByReview[key] = append(commentsByReview[key], comment)
	}

	var reviews []map[string]interface{}

	// Iterate through the map of reviews and their comments
	for reviewURL, reviewComments := range commentsByReview {
		if len(reviewComments) == 0 {
			continue
		}

		comment := reviewComments[0]

		review := map[string]interface{}{
			"type":         "pull_request_review",
			"url":          reviewURL,
			"pull_request": comment.PullRequest,
			"user":         comment.User,
			"body":         nil,
			"head_sha":     comment.CommitID,
			"formatter":    "markdown",
			"state":        comment.State,
			"reactions":    []interface{}{},
			"created_at":   comment.CreatedAt,
			"submitted_at": comment.CreatedAt,
		}

		reviews = append(reviews, review)
	}

	return reviews
}
