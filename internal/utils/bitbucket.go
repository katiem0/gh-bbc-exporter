package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/katiem0/gh-bbc-exporter/internal/data"
	"go.uber.org/zap"
)

type Client struct {
	baseURL        string
	httpClient     *http.Client
	accessToken    string // Workspace Access Token
	apiToken       string // API Token replacing AppPass after Sept 2025
	email          string // Will replace username after Sept 2025
	username       string // Will be removed after Sept 2025 with appPass
	appPass        string // To be deprecated Sept 2025
	logger         *zap.Logger
	commitSHACache map[string]string
	exportDir      string
}

func NewClient(baseURL, accessToken, apiToken, email, username, appPass string, logger *zap.Logger, exportDir string) *Client {
	baseURL = strings.TrimSuffix(baseURL, "/")

	if !strings.Contains(baseURL, "/2.0") && strings.Contains(baseURL, "api.bitbucket.org") {
		baseURL = baseURL + "/2.0"
	}

	if accessToken != "" {
		logger.Debug("Creating Bitbucket client with workspace access token authentication",
			zap.String("baseURL", baseURL))
	} else if apiToken != "" {
		if email != "" {
			logger.Debug("Creating Bitbucket client with API token authentication",
				zap.String("baseURL", baseURL),
				zap.String("email", email))
		} else {
			logger.Debug("Creating Bitbucket client with API token authentication using x-bitbucket-api-token-auth",
				zap.String("baseURL", baseURL))
		}
	} else if username != "" && appPass != "" {
		logger.Debug("Creating Bitbucket client with basic authentication",
			zap.String("baseURL", baseURL),
			zap.String("username", username))
	} else {
		logger.Warn("Creating Bitbucket client without authentication credentials")
	}

	logger.Debug("Creating Bitbucket client", zap.String("baseURL", baseURL))
	return &Client{
		baseURL:        baseURL,
		httpClient:     &http.Client{},
		accessToken:    accessToken,
		apiToken:       apiToken,
		email:          email,
		username:       username,
		appPass:        appPass,
		logger:         logger,
		commitSHACache: make(map[string]string), // Initialize commit cache
		exportDir:      exportDir,
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
		c.logger.Debug("Repository main branch",
			zap.String("branch", repo.MainBranch.Name))
	} else {
		c.logger.Debug("Repository has no main branch defined")
	}

	return &repo, nil
}

func (c *Client) makeRequest(method, endpoint string, v interface{}) error {
	var fullURL string
	maxRetries := 5
	baseDelay := 1 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		if strings.HasPrefix(endpoint, c.baseURL) {
			fullURL = endpoint
		} else {
			endpointPath := endpoint
			queryParams := ""

			if strings.Contains(endpoint, "?") {
				parts := strings.SplitN(endpoint, "?", 2)
				endpointPath = parts[0]
				queryParams = parts[1]
			}

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

		if c.accessToken != "" {
			c.logger.Debug("Using workspace access token authentication")
			req.Header.Set("Authorization", "Bearer "+c.accessToken)
		} else if c.apiToken != "" {
			c.logger.Debug("Using API token authentication")
			// For API tokens, we use Basic auth with either the email or x-bitbucket-api-token-auth
			if c.email != "" {
				c.logger.Debug("Using Atlassian account email with API token")
				req.SetBasicAuth(c.email, c.apiToken)
			} else {
				c.logger.Debug("Using x-bitbucket-api-token-auth with API token")
				req.SetBasicAuth("x-bitbucket-api-token-auth", c.apiToken)
			}
		} else if c.username != "" && c.appPass != "" {
			c.logger.Debug("Using basic authentication")
			req.SetBasicAuth(c.username, c.appPass)
		} else {
			c.logger.Warn("No authentication credentials provided")
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer func() {
			err := resp.Body.Close()
			if err != nil {
				c.logger.Warn("Error closing response body", zap.Error(err))
			}
		}()
		remaining := resp.Header.Get("X-RateLimit-Remaining")
		limit := resp.Header.Get("X-RateLimit-Limit")
		c.logger.Debug("Rate limit status",
			zap.String("remaining", remaining),
			zap.String("limit", limit))

		// Log the response status
		c.logger.Debug("API response",
			zap.Int("status", resp.StatusCode),
			zap.String("status_text", resp.Status))

		if resp.StatusCode == 429 {
			delay := baseDelay * time.Duration(1<<attempt) // Exponential backoff
			if delay > 5*time.Minute {
				delay = 5 * time.Minute // Max delay
			}

			c.logger.Warn("Rate limit hit - waiting before retrying",
				zap.Duration("delay", delay),
				zap.Int("attempt", attempt+1),
				zap.Int("max_retries", maxRetries))

			time.Sleep(delay)
			continue // Retry the request
		}

		// If the request was successful, break out of the retry loop
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return json.NewDecoder(resp.Body).Decode(v)
		}

		// Handle other errors
		bodyBytes, _ := io.ReadAll(resp.Body)
		err = fmt.Errorf("API request failed with status %d: %s: %s",
			resp.StatusCode, resp.Status, string(bodyBytes))
		c.logger.Error("API request failed", zap.Error(err))
		return err
	}

	return fmt.Errorf("API request failed after %d retries", maxRetries)
}

func (c *Client) GetUsers(workspace, repoSlug string) ([]data.User, error) {
	c.logger.Info("Fetching workspace members")

	var allUsers []data.User
	page := 1
	pageLen := 100
	hasMore := true

	for hasMore {
		endpoint := fmt.Sprintf("workspaces/%s/members?page=%d&pagelen=%d",
			workspace, page, pageLen)

		var response data.BitbucketUserResponse

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

	c.logger.Debug("Fetched workspace members",
		zap.Int("count", len(allUsers)))

	return allUsers, nil
}

func (c *Client) GetPullRequests(workspace, repoSlug string, openPRsOnly bool, prsFromDate string) ([]data.PullRequest, error) {
	c.logger.Info("Fetching pull requests",
		zap.String("workspace", workspace),
		zap.String("repository", repoSlug),
		zap.Bool("open_prs_only", openPRsOnly))

	var pullRequests []data.PullRequest
	page := 1
	pageLen := 50
	hasMore := true

	var fromDate time.Time
	var fromDateProvided bool

	if prsFromDate != "" {
		parsedDate, err := time.Parse("2006-01-02", prsFromDate)
		if err != nil {
			c.logger.Error("Failed to parse from date",
				zap.String("date", prsFromDate),
				zap.Error(err))
			return nil, fmt.Errorf("invalid date format for prsFromDate: %w", err)
		}

		// Set time to beginning of day in UTC to ensure consistent comparisons
		fromDate = time.Date(
			parsedDate.Year(),
			parsedDate.Month(),
			parsedDate.Day(),
			0, 0, 0, 0,
			time.UTC)

		fromDateProvided = true
		c.logger.Info("Filtering PRs by creation date (created_on)",
			zap.String("from_date", prsFromDate),
			zap.Time("parsed_date", fromDate),
			zap.String("field_used", "created_on"))
	}

	for hasMore {
		baseURL, parseErr := url.Parse(fmt.Sprintf("repositories/%s/%s/pullrequests", workspace, repoSlug))
		if parseErr != nil {
			c.logger.Error("failed to parse base URL", zap.Error(parseErr))
			return nil, parseErr
		}

		queryParams := url.Values{}
		queryParams.Set("page", strconv.Itoa(page))
		queryParams.Set("pagelen", strconv.Itoa(pageLen))

		if openPRsOnly {
			queryParams.Set("state", "OPEN")
		} else {
			queryParams.Set("state", "ALL")
		}

		baseURL.RawQuery = queryParams.Encode()
		endpoint := baseURL.String()

		c.logger.Debug("Fetching pull requests with endpoint",
			zap.String("endpoint", endpoint),
			zap.Bool("open_prs_only", openPRsOnly))

		var response data.BitbucketPRResponse
		var err error

		maxRetries := 3
		for retries := 0; retries < maxRetries; retries++ {
			err = c.makeRequest("GET", endpoint, &response)
			if err != nil {
				c.logger.Warn("API request error",
					zap.String("endpoint", endpoint),
					zap.Int("retry", retries),
					zap.Error(err))
				time.Sleep(time.Duration(500*(retries+1)) * time.Millisecond)
				continue
			}
			break
		}

		if err != nil {
			c.logger.Error("failed to fetch pull requests", zap.Error(err))
			return nil, err
		}

		c.logger.Debug("Pull requests response",
			zap.Int("page", page),
			zap.Int("values_count", len(response.Values)),
			zap.String("next_url", response.Next))

		for _, pr := range response.Values {
			hexPattern := regexp.MustCompile(`^[0-9a-f]{40}$`)

			if hexPattern.MatchString(pr.Source.Branch.Name) {
				c.logger.Warn("Skipping PR with ambiguous source branch name",
					zap.Int("pr_id", pr.ID),
					zap.String("pr_title", pr.Title),
					zap.String("branch_name", pr.Source.Branch.Name),
					zap.String("reason", "Source branch name is exactly 40 hex characters and could be mistaken for a commit SHA"))
				continue
			}

			if hexPattern.MatchString(pr.Destination.Branch.Name) {
				c.logger.Warn("Skipping PR with ambiguous destination branch name",
					zap.Int("pr_id", pr.ID),
					zap.String("pr_title", pr.Title),
					zap.String("branch_name", pr.Destination.Branch.Name),
					zap.String("reason", "Destination branch name is exactly 40 hex characters and could be mistaken for a commit SHA"))
				continue
			}

			if fromDateProvided {
				prCreatedAt, err := time.Parse(time.RFC3339, pr.CreatedOn)
				if err != nil {
					c.logger.Warn("Could not parse PR creation date",
						zap.String("date", pr.CreatedOn),
						zap.Int("pr_id", pr.ID),
						zap.Error(err))
					continue
				}

				if prCreatedAt.Before(fromDate) {
					c.logger.Debug("Skipping PR: created before filter date",
						zap.Int("pr_id", pr.ID),
						zap.String("pr_title", pr.Title),
						zap.Time("pr_created_at", prCreatedAt),
						zap.Time("filter_date", fromDate))
					continue
				}

				c.logger.Debug("Including PR: creation date meets filter criteria",
					zap.Int("pr_id", pr.ID),
					zap.String("pr_title", pr.Title),
					zap.Time("pr_created_at", prCreatedAt))
			}

			var mergedAt, closedAt *string
			switch pr.State {
			case "MERGED":
				mergedStr := formatDateToZ(pr.UpdatedOn)
				mergedAt = &mergedStr
				closedStr := formatDateToZ(pr.UpdatedOn)
				closedAt = &closedStr
			case "DECLINED":
				closedStr := formatDateToZ(pr.UpdatedOn)
				closedAt = &closedStr
			}

			prURL := formatURL("pr", workspace, repoSlug, pr.ID)
			userURL := formatURL("user", workspace, "", strings.Trim(pr.Author.UUID, "{}"))
			repoURL := formatURL("repository", workspace, repoSlug)
			prUser := formatURL("user", workspace, "")

			// Resolve commit SHAs
			baseSHA, _ := c.GetFullCommitSHA(workspace, repoSlug, pr.Destination.Commit.Hash)
			headSHA, _ := c.GetFullCommitSHA(workspace, repoSlug, pr.Source.Commit.Hash)

			c.logger.Debug("Getting full commit SHA",
				zap.String("base_sha", baseSHA),
				zap.String("head_sha", headSHA))

			description := ""
			if pr.Description != nil {
				description = *pr.Description
			}

			// Format merge commit SHA if available
			var mergeCommitSHA *string
			if pr.MergeCommit != nil && pr.State == "MERGED" {
				fullMergeSHA, _ := c.GetFullCommitSHA(workspace, repoSlug, pr.MergeCommit.Hash)
				mergeCommitSHA = &fullMergeSHA
			}

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
					SHA:  baseSHA,
					User: prUser,
					Repo: repoURL,
				},
				Head: data.PRBranch{
					Ref:  pr.Source.Branch.Name,
					SHA:  headSHA,
					User: prUser,
					Repo: repoURL,
				},
				Labels:               []string{},
				MergedAt:             mergedAt,
				ClosedAt:             closedAt,
				CreatedAt:            formatDateToZ(pr.CreatedOn),
				Assignee:             nil,
				Assignees:            []string{},
				Milestone:            nil,
				Reactions:            []string{},
				ReviewRequests:       []string{},
				CloseIssueReferences: []string{},
				WorkInProgress:       pr.Draft,
				MergeCommitSHA:       mergeCommitSHA,
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

	if fullSHA, exists := c.commitSHACache[commitHash]; exists {
		return fullSHA, nil
	}

	repoPath := filepath.Join(c.exportDir, "repositories", workspace, repoSlug+".git")
	if _, err := os.Stat(repoPath); err == nil {
		// Repository exists locally
		fullSHA, err := GetFullCommitSHAFromLocalRepo(repoPath, commitHash)
		if err == nil {
			// Cache the result
			c.commitSHACache[commitHash] = fullSHA
			return fullSHA, nil
		}
		// If local lookup fails, log and fall back to API
		c.logger.Debug("Failed to get full SHA from local repo, falling back to API",
			zap.String("shortSHA", commitHash),
			zap.Error(err))
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
		c.commitSHACache[commitHash] = response.Hash
		return response.Hash, nil
	}

	return commitHash, nil
}

func (c *Client) GetPullRequestComments(workspace, repoSlug string, pullRequests []data.PullRequest) ([]data.IssueComment, []data.PullRequestReviewComment, error) {
	c.logger.Info("Fetching pull request comments")

	var regularComments []data.IssueComment
	var reviewComments []data.PullRequestReviewComment

	prURLMap := make(map[int]string)
	prCommitMap := make(map[int]string)

	for _, pr := range pullRequests {
		parts := strings.Split(pr.URL, "/")
		if len(parts) > 0 {
			prID, err := strconv.Atoi(parts[len(parts)-1])
			if err == nil {
				prURLMap[prID] = pr.URL
				prCommitMap[prID] = pr.Head.SHA
			}
		}
	}

	resolvedSHAs := make(map[int]bool)
	for prID := range prURLMap {
		page := 1
		pageLen := 100
		hasMore := true

		for hasMore {
			baseEndpoint := fmt.Sprintf("repositories/%s/%s/pullrequests/%d/comments",
				workspace, repoSlug, prID)

			// Build query parameters properly
			params := url.Values{}
			params.Add("q", "deleted=false")
			params.Add("page", strconv.Itoa(page))
			params.Add("pagelen", strconv.Itoa(pageLen))

			// Combine endpoint with encoded query parameters
			endpoint := baseEndpoint + "?" + params.Encode()

			var response data.BitbucketCommentResponse

			err := c.makeRequest("GET", endpoint, &response)
			if err != nil {
				c.logger.Warn("Failed to fetch PR comments",
					zap.Int("pr_id", prID),
					zap.Error(err))
				break
			}

			for _, comment := range response.Values {
				createdAt := formatDateToZ(comment.CreatedOn)
				updatedAt := formatDateToZ(comment.UpdatedOn)
				transformedBody := c.transformCommentBody(comment.Content.Raw, workspace, repoSlug)
				prNumber := fmt.Sprintf("%d", prID)

				// Check if this is an inline comment
				if comment.Inline != nil && comment.Inline.Path != "" {
					// This is an inline review comment
					if !resolvedSHAs[prID] {
						shortSHA := prCommitMap[prID]
						fullSHA, err := c.GetFullCommitSHA(workspace, repoSlug, shortSHA)
						if err != nil {
							c.logger.Warn("Failed to resolve full SHA for PR",
								zap.Int("pr_id", prID),
								zap.String("short_sha", shortSHA),
								zap.Error(err))
							// Use the short SHA as fallback
							fullSHA = shortSHA
						}
						prCommitMap[prID] = fullSHA
						resolvedSHAs[prID] = true

						c.logger.Debug("Resolved SHA for PR with inline comment",
							zap.Int("pr_id", prID),
							zap.String("full_sha", fullSHA))
					}

					lineNumber := 1
					if comment.Inline.To != nil {
						lineNumber = *comment.Inline.To
					} else if comment.Inline.From != nil {
						lineNumber = *comment.Inline.From
					}

					// Create a unique thread identifier based on the file path and line number
					// rather than the comment ID
					threadKey := fmt.Sprintf("%s-%s-%d", workspace, comment.Inline.Path, lineNumber)
					threadId := fmt.Sprintf("thread-%s", HashString(threadKey))

					// Generate stable comment ID
					commentId := fmt.Sprintf("%d", comment.ID)

					// Handle parent-child relationship
					var inReplyTo *string
					var reviewId string

					if comment.Parent != nil {
						// This is a reply - use parent's ID for the review ID
						parentId := fmt.Sprintf("%d", comment.Parent.ID)
						inReplyTo = &parentId
						reviewId = fmt.Sprintf("review-%d", comment.Parent.ID)
					} else {
						// This is a top-level comment - use its own ID for the review ID
						reviewId = fmt.Sprintf("review-%d", comment.ID)
					}

					commentURL := formatURL("pr_review_comment", workspace, repoSlug, prNumber, commentId)
					reviewURL := formatURL("pr_review", workspace, repoSlug, prNumber, reviewId)
					threadURL := formatURL("pr_review_thread", workspace, repoSlug, prNumber, threadId)
					prFullURL := formatURL("pr", workspace, repoSlug, prNumber)
					userURL := formatURL("user", workspace, "", strings.Trim(comment.User.UUID, "{}"))
					commitSHA := prCommitMap[prID]

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
						State:                   1,
						InReplyTo:               inReplyTo,
						Reactions:               []string{},
						SubjectType:             "line",
					}

					reviewComments = append(reviewComments, reviewComment)
				} else {
					commentURL := formatURL("issue_comment", workspace, repoSlug, prNumber, comment.ID)
					prURL := formatURL("pr", workspace, repoSlug, prNumber)
					userURL := formatURL("user", workspace, "", strings.Trim(comment.User.UUID, "{}"))

					regularComment := data.IssueComment{
						Type:        "issue_comment",
						URL:         commentURL,
						User:        userURL,
						Body:        transformedBody,
						CreatedAt:   createdAt,
						Formatter:   "markdown",
						Reactions:   []string{},
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

	c.logger.Debug("Fetched pull request comments",
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
	replacement := fmt.Sprintf("https://bitbucket.org/%s/%s/pull/$1",
		workspace, repoSlug)

	re := regexp.MustCompile(pattern)
	transformedBody := re.ReplaceAllString(body, replacement)

	prPattern := `\b#(\d+)\b`

	prRe := regexp.MustCompile(prPattern)
	transformedBody = prRe.ReplaceAllStringFunc(transformedBody, func(match string) string {
		numStr := match[1:] // Remove the # prefix
		return fmt.Sprintf("[%s](%s)", match, fmt.Sprintf("https://bitbucket.org/%s/%s/pull/%s",
			workspace, repoSlug, numStr))
	})

	return transformedBody
}
