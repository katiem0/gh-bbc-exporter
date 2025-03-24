package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func (e *Exporter) GetOutputPath() string {
	return e.outputDir
}

func (e *Exporter) createRepositoryInfoFiles(workspace, repoSlug string) error {
	repoPath := filepath.Join(e.outputDir, "repositories", workspace, repoSlug+".git")

	// Create info directory
	infoDir := filepath.Join(repoPath, "info")
	if err := os.MkdirAll(infoDir, 0755); err != nil {
		return fmt.Errorf("failed to create info directory: %w", err)
	}

	// Create nwo file (name with owner)
	nwoContent := fmt.Sprintf("%s/%s\n", workspace, repoSlug)
	if err := os.WriteFile(filepath.Join(infoDir, "nwo"), []byte(nwoContent), 0644); err != nil {
		return fmt.Errorf("failed to create nwo file: %w", err)
	}

	// Create last-sync file with current timestamp
	syncTime := time.Now().Format("2006-01-02T15:04:05")
	if err := os.WriteFile(filepath.Join(infoDir, "last-sync"), []byte(syncTime), 0644); err != nil {
		return fmt.Errorf("failed to create last-sync file: %w", err)
	}

	return nil
}

func formatURL(urlType string, workspace, repoSlug string, id ...interface{}) string {
	switch urlType {
	case "repository":
		return fmt.Sprintf("https://bitbucket.org/%s/%s", workspace, repoSlug)
	case "user":
		userID := workspace
		if len(id) > 0 && id[0] != nil {
			userID = fmt.Sprintf("%v", id[0])
		}
		return fmt.Sprintf("https://bitbucket.org/%s", userID)
	case "organization":
		return fmt.Sprintf("https://bitbucket.org/%s", workspace)
	case "pr":
		if len(id) > 0 {
			return fmt.Sprintf("https://bitbucket.org/%s/%s/pull/%v", workspace, repoSlug, id[0])
		}
		return fmt.Sprintf("https://bitbucket.org/%s/%s/pulls", workspace, repoSlug)
	case "issue_comment":
		if len(id) > 0 {
			return fmt.Sprintf("https://bitbucket.org/%s/%s/pull/%v#issuecomment-%v",
				workspace, repoSlug, extractPRNumber(fmt.Sprintf("%v", id[0])), id[1])
		}
		return fmt.Sprintf("https://bitbucket.org/%s/%s/pull/comments", workspace, repoSlug)
	case "pr_review":
		if len(id) > 0 {
			return fmt.Sprintf("https://bitbucket.org/%s/%s/pull/%v/files#pullrequestreview-%v",
				workspace, repoSlug, id[0], id[1])
		}
		return fmt.Sprintf("https://bitbucket.org/%s/%s/pull/reviews", workspace, repoSlug)
	case "pr_review_comment":
		if len(id) > 0 {
			return fmt.Sprintf("https://bitbucket.org/%s/%s/pull/%v/files#r%v",
				workspace, repoSlug, id[0], id[1])
		}
		return fmt.Sprintf("https://bitbucket.org/%s/%s/pull/comments", workspace, repoSlug)
	case "pr_review_thread":
		if len(id) > 0 {
			return fmt.Sprintf("https://bitbucket.org/%s/%s/pull/%v/files#pullrequestreviewthread-%v",
				workspace, repoSlug, id[0], id[1])
		}
		return fmt.Sprintf("https://bitbucket.org/%s/%s/pull/threads", workspace, repoSlug)
	case "git":
		return fmt.Sprintf("tarball://root/repositories/%s/%s.git", workspace, repoSlug)
	default:
		return fmt.Sprintf("https://bitbucket.org/%s/%s", workspace, repoSlug)
	}
}

func extractPRNumber(prURL string) string {
	parts := strings.Split(prURL, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return "1"
}
