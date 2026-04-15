package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/katiem0/gh-bbc-exporter/internal/data"
	"go.uber.org/zap"
)

type Client struct {
	baseURL          string
	httpClient       *http.Client
	accessToken      string // Workspace Access Token
	apiToken         string // API Token replacing AppPass after Sept 2025
	email            string // Will replace username after Sept 2025
	username         string // Will be removed after Sept 2025 with appPass
	appPass          string // To be deprecated Sept 2025
	logger           *zap.Logger
	commitSHACache      map[string]string
	prSourceCommitCache map[int]string // prID -> original source branch tip SHA
	exportDir           string
	skipCommitLookup    bool
}

func NewClient(baseURL, accessToken, apiToken, email, username, appPass string, logger *zap.Logger, exportDir string, skipCommitLookup bool) *Client {
	baseURL = strings.TrimSuffix(baseURL, "/")

	if !strings.Contains(baseURL, "/2.0") && strings.Contains(baseURL, "api.bitbucket.org") {
		baseURL = baseURL + "/2.0"
	}

	var authMethod string
	if accessToken != "" {
		authMethod = "workspace access token"
	} else if apiToken != "" {
		if email != "" {
			authMethod = "API token with email"
		} else {
			authMethod = "API token with x-bitbucket-api-token-auth"
		}
	} else if username != "" && appPass != "" {
		authMethod = "username and app password"
	} else {
		authMethod = "none"
	}

	logger.Debug("Creating Bitbucket client",
		zap.String("baseURL", baseURL),
		zap.String("authMethod", authMethod))

	return &Client{
		baseURL:          baseURL,
		httpClient:       &http.Client{},
		accessToken:      accessToken,
		apiToken:         apiToken,
		email:            email,
		username:         username,
		appPass:          appPass,
		logger:           logger,
		commitSHACache:      make(map[string]string),
		prSourceCommitCache: make(map[int]string),
		exportDir:           exportDir,
		skipCommitLookup:    skipCommitLookup,
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

func (c *Client) setAuth(req *http.Request) {
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	} else if c.apiToken != "" {
		if c.email != "" {
			req.SetBasicAuth(c.email, c.apiToken)
		} else {
			req.SetBasicAuth("x-bitbucket-api-token-auth", c.apiToken)
		}
	} else if c.username != "" && c.appPass != "" {
		req.SetBasicAuth(c.username, c.appPass)
	}
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

		if attempt == 0 {
			c.logger.Debug("Making API request",
				zap.String("method", method),
				zap.String("url", fullURL))
		}

		req, err := http.NewRequest(method, fullURL, nil)
		if err != nil {
			return err
		}

		c.setAuth(req)
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
		if remaining != "" && limit != "" {
			remainingInt, _ := strconv.Atoi(remaining)
			limitInt, _ := strconv.Atoi(limit)
			if limitInt > 0 && float64(remainingInt)/float64(limitInt) < 0.1 {
				c.logger.Warn("Low API rate limit remaining",
					zap.String("remaining", remaining),
					zap.String("limit", limit))
			}
		}

		// Only log non-successful responses
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			c.logger.Debug("API response",
				zap.Int("status", resp.StatusCode),
				zap.String("status_text", resp.Status))
		}
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
		c.logger.Info("Filtering PRs by creation date",
			zap.String("from_date", prsFromDate))
	}

	skippedAmbiguous := 0
	skippedByDate := 0

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

		for _, pr := range response.Values {

			if hexPatternRegex.MatchString(pr.Source.Branch.Name) {
				skippedAmbiguous++
				c.logger.Debug("Skipping PR with ambiguous source branch",
					zap.Int("pr_id", pr.ID),
					zap.String("branch_name", pr.Source.Branch.Name))
				continue
			}

			if hexPatternRegex.MatchString(pr.Destination.Branch.Name) {
				skippedAmbiguous++
				c.logger.Debug("Skipping PR with ambiguous destination branch",
					zap.Int("pr_id", pr.ID),
					zap.String("branch_name", pr.Destination.Branch.Name))
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
					skippedByDate++
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

			// For squash-merged PRs with deleted branches, the head SHA may not exist
			// as a git object in the pack. Fall back to the merge commit SHA so GEI
			// has a valid commit to show in the Commits tab rather than nothing.
			if pr.State == "MERGED" && mergeCommitSHA != nil {
				repoPath := filepath.Join(c.exportDir, "repositories", workspace, repoSlug+".git")
				if _, statErr := os.Stat(repoPath); statErr != nil {
					c.logger.Debug("repo path not found, skipping head SHA check",
						zap.String("repo_path", repoPath),
						zap.Error(statErr))
				} else {
					cmd := exec.Command("git", "cat-file", "-e", headSHA)
					cmd.Dir = repoPath
					if err := cmd.Run(); err != nil {
						c.logger.Debug("head SHA not in pack, falling back to merge commit SHA for PR",
							zap.Int("pr_id", pr.ID),
							zap.String("head_sha", headSHA),
							zap.String("merge_commit_sha", *mergeCommitSHA))
						headSHA = *mergeCommitSHA
						// Update the source commit cache so review comments use the same
						// valid SHA — the original branch tip is not in the pack.
						if c.prSourceCommitCache != nil {
							c.prSourceCommitCache[pr.ID] = headSHA
						}
					}
				}
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

	c.logger.Info("Pull requests fetched",
		zap.Int("total", len(pullRequests)),
		zap.Int("skipped_ambiguous", skippedAmbiguous),
		zap.Int("skipped_by_date", skippedByDate))

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
			c.logger.Debug("Resolved full SHA from local repository",
				zap.String("shortSHA", commitHash),
				zap.String("fullSHA", fullSHA))
			return fullSHA, nil
		}
		// If local lookup fails, log and fall back to API
		c.logger.Debug("Failed to get full SHA from local repo, falling back to API",
			zap.String("shortSHA", commitHash),
			zap.Error(err))
	}

	if c.skipCommitLookup {
		c.logger.Warn("Cannot resolve full commit SHA - API lookup disabled,",
			zap.String("workspace", workspace),
			zap.String("repo", repoSlug),
			zap.String("sha", commitHash),
			zap.Int("sha_length", len(commitHash)),
			zap.String("impact", "This may cause failures if full SHA required"))
		c.commitSHACache[commitHash] = commitHash
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
		c.commitSHACache[commitHash] = response.Hash
		return response.Hash, nil
	}

	return commitHash, nil
}

// GetPullRequestDiff fetches the unified diff for a pull request as raw text.
func (c *Client) GetPullRequestDiff(workspace, repoSlug string, prID int) (string, error) {
	fullURL := fmt.Sprintf("%s/repositories/%s/%s/pullrequests/%d/diff",
		c.baseURL, workspace, repoSlug, prID)

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return "", err
	}

	c.setAuth(req)
	req.Header.Set("Accept", "text/plain")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("diff request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// DiffHunkResult holds the extracted diff hunk and GitHub position for an inline comment.
type DiffHunkResult struct {
	Hunk     string
	Position int
}

// parseDiffHunk finds the unified diff hunk for a given file path and new-file line number,
// returning the hunk text and the GitHub position (1-based count from the @@ header line).
// Returns nil if the file or line cannot be found in the diff.
func parseDiffHunk(diff, filePath string, targetLine int) *DiffHunkResult {
	lines := strings.Split(diff, "\n")

	inTargetFile := false
	var hunkLines []string
	hunkStarted := false
	newFileLineNum := 0
	position := 0
	targetPosition := -1

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			if targetPosition != -1 {
				break
			}
			inTargetFile = strings.HasSuffix(line, " b/"+filePath)
			hunkStarted = false
			hunkLines = nil
			newFileLineNum = 0
			position = 0
			continue
		}

		if !inTargetFile {
			continue
		}

		if strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") ||
			strings.HasPrefix(line, "index ") || strings.HasPrefix(line, "new file") ||
			strings.HasPrefix(line, "deleted file") || strings.HasPrefix(line, "similarity") ||
			strings.HasPrefix(line, "rename") {
			continue
		}

		if strings.HasPrefix(line, "@@ ") {
			if targetPosition != -1 {
				break
			}
			hunkStarted = true
			hunkLines = []string{line}
			position = 1
			newFileLineNum = parseHunkNewStart(line) - 1
			continue
		}

		if !hunkStarted {
			continue
		}

		hunkLines = append(hunkLines, line)
		position++

		switch {
		case strings.HasPrefix(line, "+"):
			newFileLineNum++
			if newFileLineNum == targetLine {
				targetPosition = position
			}
		case strings.HasPrefix(line, "-"):
			// Removed lines don't advance the new file line number
		default:
			newFileLineNum++
			if newFileLineNum == targetLine {
				targetPosition = position
			}
		}

		if targetPosition != -1 {
			break
		}
	}

	if targetPosition == -1 {
		return nil
	}

	return &DiffHunkResult{
		Hunk:     strings.Join(hunkLines, "\n"),
		Position: targetPosition,
	}
}

// parseHunkNewStart extracts the new-file start line number from a unified diff hunk header.
// e.g. "@@ -10,6 +15,8 @@" returns 15.
func parseHunkNewStart(hunkHeader string) int {
	parts := strings.Fields(hunkHeader)
	for _, part := range parts {
		if strings.HasPrefix(part, "+") {
			numPart := strings.TrimPrefix(part, "+")
			if idx := strings.Index(numPart, ","); idx != -1 {
				numPart = numPart[:idx]
			}
			var n int
			if _, err := fmt.Sscanf(numPart, "%d", &n); err == nil {
				return n
			}
		}
	}
	return 1
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
	failedPRs := 0
	prDiffCache := make(map[int]string) // prID -> raw unified diff
	var prDiffMu sync.Mutex
	var wg sync.WaitGroup

	for prID := range prURLMap {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			diff, err := c.GetPullRequestDiff(workspace, repoSlug, id)
			if err != nil {
				c.logger.Debug("Failed to fetch PR diff, inline comments will be demoted if no diff available",
					zap.Int("pr_id", id),
					zap.Error(err))
				return
			}
			prDiffMu.Lock()
			prDiffCache[id] = diff
			prDiffMu.Unlock()
		}(prID)
	}
	wg.Wait()

	for prID := range prURLMap {
		page := 1
		pageLen := 100
		hasMore := true

		// Accumulate inline comments whose real diff hunk can't be found,
		// grouped by thread URL for a single coherent issue comment per thread.
		type demotedEntry struct {
			userUUID  string
			body      string
			createdAt string
		}
		type demotedMeta struct {
			path           string
			lineNumber     int
			firstCommentID int
			prURL          string
		}
		demotedEntries := make(map[string][]demotedEntry) // threadURL -> entries
		demotedMetas := make(map[string]demotedMeta)      // threadURL -> metadata

		for hasMore {
			baseEndpoint := fmt.Sprintf("repositories/%s/%s/pullrequests/%d/comments",
				workspace, repoSlug, prID)

			params := url.Values{}
			params.Add("q", "deleted=false")
			params.Add("page", strconv.Itoa(page))
			params.Add("pagelen", strconv.Itoa(pageLen))

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

				if comment.Inline != nil && comment.Inline.Path != "" {
					if !resolvedSHAs[prID] {
						shortSHA := prCommitMap[prID]
						fullSHA, err := c.GetFullCommitSHA(workspace, repoSlug, shortSHA)
						if err != nil {
							c.logger.Warn("Failed to resolve full SHA for PR",
								zap.Int("pr_id", prID),
								zap.String("short_sha", shortSHA),
								zap.Error(err))
							fullSHA = shortSHA
						}
						prCommitMap[prID] = fullSHA
						resolvedSHAs[prID] = true

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
					prDiff := prDiffCache[prID]

					if prDiff == "" {
						// No diff available (e.g. squash merge with deleted branch) — demote
						// the inline review comment to a regular issue comment so it appears
						// as readable plain text rather than a broken preserved fallback.
						commentURL := formatURL("issue_comment", workspace, repoSlug, prNumber, comment.ID)
						prURL := formatURL("pr", workspace, repoSlug, prNumber)
						demotedBody := fmt.Sprintf("_[Inline comment on `%s` line %d]_\n\n%s",
							comment.Inline.Path, lineNumber, transformedBody)
						regularComments = append(regularComments, data.IssueComment{
							Type:        "issue_comment",
							URL:         commentURL,
							User:        userURL,
							Body:        demotedBody,
							CreatedAt:   createdAt,
							Formatter:   "markdown",
							Reactions:   []string{},
							PullRequest: prURL,
						})
						c.logger.Debug("Demoted inline review comment to issue comment (no diff available)",
							zap.Int("pr_id", prID),
							zap.String("path", comment.Inline.Path),
							zap.Int("line", lineNumber))
					} else {
						// Diff is available — extract the real hunk and position.
						diffHunkResult := parseDiffHunk(prDiff, comment.Inline.Path, lineNumber)

						if diffHunkResult == nil {
							// Real hunk not found (e.g. large PR where Bitbucket truncated
							// the diff). Accumulate into a grouped issue comment per thread.
							uuid := strings.Trim(comment.User.UUID, "{}")
							demotedEntries[threadURL] = append(demotedEntries[threadURL], demotedEntry{
								userUUID:  uuid,
								body:      transformedBody,
								createdAt: createdAt,
							})
							if _, exists := demotedMetas[threadURL]; !exists {
								demotedMetas[threadURL] = demotedMeta{
									path:           comment.Inline.Path,
									lineNumber:     lineNumber,
									firstCommentID: comment.ID,
									prURL:          formatURL("pr", workspace, repoSlug, prNumber),
								}
							}
							c.logger.Debug("Queued inline review comment for grouped demote (hunk not found in diff)",
								zap.Int("pr_id", prID),
								zap.String("path", comment.Inline.Path),
								zap.Int("line", lineNumber))
						} else {
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
								Position:                diffHunkResult.Position,
								OriginalPosition:        diffHunkResult.Position,
								Body:                    transformedBody,
								CreatedAt:               createdAt,
								UpdatedAt:               updatedAt,
								Formatter:               "markdown",
								DiffHunk:                diffHunkResult.Hunk,
								State:                   1,
								InReplyTo:               inReplyTo,
								Reactions:               []string{},
								SubjectType:             "line",
							}
							reviewComments = append(reviewComments, reviewComment)
						}
					}
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

			hasMore = response.Next != ""
			if hasMore {
				page++
			}
		}

		// Emit one grouped issue comment per demoted thread
		for tURL, entries := range demotedEntries {
			meta := demotedMetas[tURL]
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("_[Inline comment thread on `%s` line %d]_\n\n", meta.path, meta.lineNumber))
			for _, e := range entries {
				displayName := e.userUUID
				if user, ok := c.userCache[e.userUUID]; ok {
					if user.DisplayName != "" {
						displayName = user.DisplayName
					} else if user.Nickname != "" {
						displayName = user.Nickname
					}
				}
				sb.WriteString(fmt.Sprintf("**%s**: %s\n\n", displayName, e.body))
			}
			commentURL := formatURL("issue_comment", workspace, repoSlug, fmt.Sprintf("%d", prID), meta.firstCommentID)
			regularComments = append(regularComments, data.IssueComment{
				Type:        "issue_comment",
				URL:         commentURL,
				User:        formatURL("user", workspace, "", entries[0].userUUID),
				Body:        strings.TrimRight(sb.String(), "\n"),
				CreatedAt:   entries[0].createdAt,
				Formatter:   "markdown",
				Reactions:   []string{},
				PullRequest: meta.prURL,
			})
		}
	}

	c.logger.Info("Pull request comments fetched",
		zap.Int("regular_comments", len(regularComments)),
		zap.Int("review_comments", len(reviewComments)),
		zap.Int("failed_prs", failedPRs))

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

	transformedBody = prNumberPattern.ReplaceAllStringFunc(transformedBody, func(match string) string {
		numStr := match[1:] // Remove the # prefix
		return fmt.Sprintf("[%s](%s)", match, fmt.Sprintf("https://bitbucket.org/%s/%s/pull/%s",
			workspace, repoSlug, numStr))
	})

	return transformedBody
}
