package utils

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

	// Return empty string for invalid date formats
	// instead of returning the input string unchanged
	return ""
}

func (e *Exporter) writeJSONFile(filename string, data interface{}) error {
	filepath := filepath.Join(e.outputDir, filename)
	e.logger.Debug("Writing file", zap.String("path", filepath))

	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filename, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			e.logger.Warn("Failed to close file", zap.String("file", filepath), zap.Error(err))
		}
	}()
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
	// Always use forward slashes for the repository path
	repoPath := ToUnixPath(filepath.Join(e.outputDir, "repositories", workspace, repoSlug+".git"))

	// Create native path for file operations
	nativePath := ToNativePath(repoPath)

	// Create info directory
	infoDir := filepath.Join(nativePath, "info")
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
		if len(id) > 1 {
			return fmt.Sprintf("https://bitbucket.org/%s/%s/pull/%v#issuecomment-%v",
				workspace, repoSlug, id[0], id[1])
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
	// Extract the PR number from a Bitbucket PR URL
	// Example: https://bitbucket.org/workspace/repo/pull/123

	// First check if this is a PR URL
	if !strings.Contains(prURL, "/pull/") {
		return ""
	}

	// Split the URL by "/" and find the part after "pull"
	parts := strings.Split(prURL, "/")
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "pull" {
			// Get the next part which should be the PR number
			prNumber := parts[i+1]
			// Remove any query parameters
			prNumber = strings.Split(prNumber, "?")[0]
			// Remove any additional path segments
			prNumber = strings.Split(prNumber, "/")[0]
			return prNumber
		}
	}
	return ""
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
		// Case-insensitive comparison for name, exact match for slug (always lowercase)
		if strings.EqualFold(repo.Name, repoSlug) || repo.Slug == repoSlug {
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
	hasToken := cmdFlags.BitbucketAccessToken != ""
	hasAPIToken := cmdFlags.BitbucketAPIToken != ""
	hasEmail := cmdFlags.BitbucketEmail != ""
	hasBasicAuth := cmdFlags.BitbucketUser != "" && cmdFlags.BitbucketAppPass != ""

	hasValidAuth := hasToken || (hasAPIToken && hasEmail) || hasBasicAuth
	if !hasValidAuth {
		return fmt.Errorf("authentication credentials required: either provide a workspace access token with --access-token, an API token with email (--api-token and --email), or both username (--user) and app password (--app-password)")
	}

	// Check for mixed auth methods
	authMethodsCount := 0
	if hasToken {
		authMethodsCount++
	}
	if hasAPIToken && hasEmail {
		authMethodsCount++
	}
	if hasBasicAuth {
		authMethodsCount++
	}

	if authMethodsCount > 1 {
		return fmt.Errorf("mixed authentication methods: provide either workspace token OR (API token + email) OR (username + app-password), not multiple types")
	}

	// Validate that API token comes with email
	if hasAPIToken && !hasEmail {
		return fmt.Errorf("email is required when using API token authentication. Please provide it with --email or BITBUCKET_EMAIL environment variable")
	}

	// Validate that email comes with API token
	if hasEmail && !hasAPIToken {
		return fmt.Errorf("API token is required when using email authentication. Please provide it with --api-token or BITBUCKET_API_TOKEN environment variable")
	}

	// Validate that username comes with app password
	if cmdFlags.BitbucketUser != "" && cmdFlags.BitbucketAppPass == "" {
		return fmt.Errorf("app password is required when using username authentication. Please provide it with --app-password or BITBUCKET_APP_PASSWORD environment variable")
	}

	// Validate that app password comes with username
	if cmdFlags.BitbucketAppPass != "" && cmdFlags.BitbucketUser == "" {
		return fmt.Errorf("username is required when using app password authentication. Please provide it with --user or BITBUCKET_USERNAME environment variable")
	}

	// Validate PRsFromDate format if provided
	if cmdFlags.PRsFromDate != "" {
		if _, err := time.Parse("2006-01-02", cmdFlags.PRsFromDate); err != nil {
			return fmt.Errorf("invalid date format for --prs-from-date: %v (expected format: YYYY-MM-DD)", err)
		}
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
	if cmdFlags.BitbucketAccessToken == "" {
		cmdFlags.BitbucketAccessToken = os.Getenv("BITBUCKET_ACCESS_TOKEN")
	}
	if cmdFlags.BitbucketAPIToken == "" {
		cmdFlags.BitbucketAPIToken = os.Getenv("BITBUCKET_API_TOKEN")
	}
	if cmdFlags.BitbucketEmail == "" {
		cmdFlags.BitbucketEmail = os.Getenv("BITBUCKET_EMAIL")
	}

	// Add warning for multiple auth methods
	if cmdFlags.BitbucketAccessToken != "" &&
		((cmdFlags.BitbucketUser != "" && cmdFlags.BitbucketAppPass != "") || (cmdFlags.BitbucketAPIToken != "" && cmdFlags.BitbucketEmail != "")) {
		fmt.Fprintf(os.Stderr, "Warning: Multiple authentication methods detected. Workspace access token authentication will be used.\n")
	} else if (cmdFlags.BitbucketAPIToken != "" && cmdFlags.BitbucketEmail != "") &&
		(cmdFlags.BitbucketUser != "" && cmdFlags.BitbucketAppPass != "") {
		fmt.Fprintf(os.Stderr, "Warning: Both API token and username/password are set. API token authentication will be used.\n")
	}

	// Add deprecation warning for app passwords
	if cmdFlags.BitbucketAppPass != "" {
		fmt.Fprintf(os.Stderr, "Warning: Bitbucket app passwords are deprecated and will be discontinued after September 9, 2025.\n"+
			"Please consider switching to Bitbucket API tokens instead. See https://support.atlassian.com/bitbucket-cloud/docs/create-an-api-token/\n")
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

func HashString(s string) string {
	h := fnv.New32a()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum32())
}

func NormalizePath(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}

func ToUnixPath(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}

func ToNativePath(path string) string {
	if runtime.GOOS == "windows" {
		return strings.ReplaceAll(path, "/", "\\")
	}
	return path
}

func ExecuteCommand(command string, args []string, workingDir string) ([]byte, error) {
	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		// Use cmd.exe on Windows to handle path issues
		allArgs := append([]string{"/C", command}, args...)
		cmd = exec.Command("cmd.exe", allArgs...)
	} else {
		cmd = exec.Command(command, args...)
	}

	cmd.Dir = workingDir

	// Add environment variables
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_SSL_NO_VERIFY=true")

	return cmd.CombinedOutput()
}
