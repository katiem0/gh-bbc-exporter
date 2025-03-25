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
		"2006/01/02 15:04:05",
	}

	for _, format := range formats {
		t, err := time.Parse(format, inputDate)
		if err == nil {
			return t.UTC().Format("2006-01-02T15:04:05Z")
		}
	}
	return inputDate
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

func (e *Exporter) updateRepositoryField(repoSlug string, field string, value interface{}) {
	filePath := filepath.Join(e.outputDir, "repositories_000001.json")
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		e.logger.Warn("Failed to read repositories file", zap.Error(err))
		return
	}

	var repositories []data.Repository
	if err := json.Unmarshal(fileData, &repositories); err != nil {
		e.logger.Warn("Failed to parse repositories file", zap.Error(err))
		return
	}

	repoUpdated := false
	for i, repo := range repositories {
		if repo.Name == repoSlug {
			switch field {
			case "default_branch":
				repositories[i].DefaultBranch = value.(string)
			case "git_url":
				repositories[i].GitURL = value.(string)
				// Add other fields as needed
			}
			repoUpdated = true
			break
		}
	}

	if !repoUpdated {
		e.logger.Warn("Repository not found in repositories file",
			zap.String("repo", repoSlug))
		return
	}

	updatedData, err := json.MarshalIndent(repositories, "", "  ")
	if err != nil {
		e.logger.Warn("Failed to encode repositories data", zap.Error(err))
		return
	}

	if err := os.WriteFile(filePath, updatedData, 0644); err != nil {
		e.logger.Warn("Failed to write updated repositories file", zap.Error(err))
		return
	}

	e.logger.Debug(fmt.Sprintf("Updated %s in repositories file", field),
		zap.String(field, fmt.Sprintf("%v", value)))
}

func ValidateExportFlags(cmdFlags *data.CmdFlags) error {
	hasToken := cmdFlags.BitbucketToken != ""
	hasBasicAuth := cmdFlags.BitbucketUser != "" && cmdFlags.BitbucketAppPass != ""

	if !hasToken && !hasBasicAuth {
		return fmt.Errorf("authentication credentials required: either provide an access token with --token or both username (--user) and app password (--app-password)")
	}

	if hasToken && (cmdFlags.BitbucketUser != "" || cmdFlags.BitbucketAppPass != "") {
		return fmt.Errorf("mixed authentication methods: provide either token OR username/app-password, not both")
	}

	return nil
}

func SetupEnvironmentCredentials(cmdFlags *data.CmdFlags) {
	if cmdFlags.BitbucketUser == "" {
		cmdFlags.BitbucketUser = os.Getenv("BITBUCKET_USERNAME")
	}
	if cmdFlags.BitbucketAppPass == "" {
		cmdFlags.BitbucketAppPass = os.Getenv("BITBUCKET_APP_PASSWORD")
	}
	if cmdFlags.BitbucketToken == "" {
		cmdFlags.BitbucketToken = os.Getenv("BITBUCKET_TOKEN")
	}

	if cmdFlags.BitbucketToken != "" && cmdFlags.BitbucketUser != "" && cmdFlags.BitbucketAppPass != "" {
		fmt.Fprintf(os.Stderr, "Warning: Both token and username/password are set. Token authentication will be used.\n")
	}
}

func PrintSuccessMessage(outputPath string) {
	if strings.HasSuffix(outputPath, ".tar.gz") {
		fmt.Printf("\nExport successful!\nArchive created: %s\n", outputPath)
		fmt.Println("You can use this archive with GitHub's repository importer.")
	} else {
		fmt.Printf("\nExport successful!\nOutput directory: %s\n", outputPath)
	}
}
