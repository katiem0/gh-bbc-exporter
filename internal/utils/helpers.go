package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/katiem0/gh-bbc-exporter/internal/data"
	"go.uber.org/zap"
)

func formatDateToZ(inputDate string) string {
	if inputDate == "" {
		return ""
	}

	// Try parsing with various formats
	formats := []string{
		"2006-01-02T15:04:05.999999+00:00",
		"2006-01-02T15:04:05.999999-07:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.999Z",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02T15:04:05.999999Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		t, err := time.Parse(format, inputDate)
		if err == nil {
			return t.UTC().Format("2006-01-02T15:04:05Z")
		}
	}
	return inputDate
}

func (e *Exporter) updateRepositoryDefaultBranch(repoSlug, defaultBranch string) {
	// Read the current file
	filePath := filepath.Join(e.outputDir, "repositories_000001.json")
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		e.logger.Warn("Failed to read repositories file", zap.Error(err))
		return
	}

	// Parse the JSON
	var repositories []data.Repository
	if err := json.Unmarshal(fileData, &repositories); err != nil {
		e.logger.Warn("Failed to parse repositories file", zap.Error(err))
		return
	}

	// Find the repository and update its default branch
	repoUpdated := false
	for i, repo := range repositories {
		if repo.Name == repoSlug {
			repositories[i].DefaultBranch = defaultBranch
			repoUpdated = true
			break
		}
	}

	if !repoUpdated {
		e.logger.Warn("Repository not found in repositories file",
			zap.String("repo", repoSlug))
		return
	}

	// Write the updated file
	updatedData, err := json.MarshalIndent(repositories, "", "  ")
	if err != nil {
		e.logger.Warn("Failed to encode repositories data", zap.Error(err))
		return
	}

	if err := os.WriteFile(filePath, updatedData, 0644); err != nil {
		e.logger.Warn("Failed to write updated repositories file", zap.Error(err))
		return
	}

	e.logger.Info("Updated default branch in repositories file",
		zap.String("branch", defaultBranch))
}

func (e *Exporter) writeJSONFile(filename string, data interface{}) error {
	filepath := filepath.Join(e.outputDir, filename)
	e.logger.Debug("Writing file", zap.String("path", filepath))

	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filename, err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode data for %s: %w", filename, err)
	}

	return nil
}

func (e *Exporter) createURLsTemplate() data.URLs {
	return data.URLs{
		User:                 "{scheme}://{+host}{/segments*}/{user}",
		Organization:         "{scheme}://{+host}/{organization}",
		Team:                 "{scheme}://{+host}/{owner}/teams/{team}",
		Repository:           "{scheme}://{+host}/{owner}/{repository}",
		ProtectedBranch:      "{scheme}://{+host}/{owner}/{repository}/protected_branches/{protected_branch}",
		PullRequest:          "{scheme}://{+host}/{owner}/{repository}/merge_requests/{number}",
		Release:              "{scheme}://{+host}/{owner}/{repository}/tags/{release}",
		Label:                "{scheme}://{+host}/{owner}/{repository}/labels#/{label}",
		Issue:                "{scheme}://{+host}/{owner}/{repository}/issues/{issue}",
		PullRequestReviewCmt: "{scheme}://{+host}/{owner}/{repository}/merge_requests/{pull_request}/diffs#note_{pull_request_review_comment}",

		IssueComment: data.IssueCommentURLs{
			Issue:       "{scheme}://{+host}/{owner}/{repository}/issues/{issue}#note_{issue_comment}",
			PullRequest: "{scheme}://{+host}/{owner}/{repository}/merge_requests/{pull_request}#note_{issue_comment}",
		},
	}
}

func (e *Exporter) GetOutputPath() string {
	return e.outputDir
}

func formatBitbucketURL(urlType string, workspace, repoSlug string, parts ...string) string {
	baseURL := fmt.Sprintf("https://bitbucket.org/%s", workspace)

	switch urlType {
	case "repository":
		return fmt.Sprintf("%s/%s", baseURL, repoSlug)
	case "pull_request":
		return fmt.Sprintf("%s/%s/merge_requests/%s", baseURL, repoSlug, parts[0])
	case "pull_request_comment":
		return fmt.Sprintf("%s/%s/merge_requests/%s#note_%s", baseURL, repoSlug, parts[0], parts[1])
	case "pull_request_review_comment":
		return fmt.Sprintf("%s/%s/merge_requests/%s/diffs#note_%s", baseURL, repoSlug, parts[0], parts[1])
	case "user":
		return fmt.Sprintf("%s/%s", baseURL, parts[0])
	default:
		return baseURL
	}
}

func (c *Client) paginatedRequest(endpoint string, processor func(data json.RawMessage) error) error {
	page := 1
	pageLen := 100
	hasMore := true

	for hasMore {
		paginatedEndpoint := fmt.Sprintf("%s?page=%d&pagelen=%d", endpoint, page, pageLen)

		var response struct {
			Values json.RawMessage `json:"values"`
			Next   string          `json:"next"`
		}

		if err := c.makeRequest("GET", paginatedEndpoint, &response); err != nil {
			return err
		}

		if err := processor(response.Values); err != nil {
			return err
		}

		hasMore = response.Next != ""
		if hasMore {
			page++
		}
	}

	return nil
}
