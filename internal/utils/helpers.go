package utils

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/katiem0/gh-bbc-exporter/internal/data"
	"go.uber.org/zap"
)

var (
	repoNameInvalidCharsRegex = regexp.MustCompile(`[^a-zA-Z0-9\-\._]|^\.|\.$/`)
	whitespaceRegex           = regexp.MustCompile(`\s+`)
	hexPatternRegex           = regexp.MustCompile(`^[0-9a-f]{40}$`)
	prNumberPattern           = regexp.MustCompile(`\b#(\d+)\b`)
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

func GetFullCommitSHAFromLocalRepo(repoPath string, shortSHA string) (string, error) {
	if len(shortSHA) == 40 {
		return shortSHA, nil
	}

	// Use git rev-parse to convert short SHA to full SHA
	cmd := exec.Command("git", "rev-parse", shortSHA)
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get full commit SHA from local repo: %w", err)
	}

	fullSHA := strings.TrimSpace(string(output))
	if len(fullSHA) == 40 {
		return fullSHA, nil
	}

	return "", fmt.Errorf("unexpected output from git rev-parse: %s", fullSHA)
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

func ToUnixPath(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}

func NormalizePath(path string) string {
	return ToUnixPath(path)
}

func ToNativePath(path string) string {
	if runtime.GOOS == "windows" {
		return strings.ReplaceAll(path, "/", "\\")
	}
	return path
}

func ExecuteCommand(command string, args []string, workingDir string, skipSSLVerify bool) ([]byte, error) {
	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		// Check if the command is an executable or needs cmd.exe
		if strings.HasSuffix(command, ".exe") || isExecutableInPath(command) {
			// Direct executable, no need for cmd.exe wrapper
			cmd = exec.Command(command, args...)
		} else {
			// Use cmd.exe to handle built-in commands and path issues
			allArgs := append([]string{"/C", command}, args...)
			cmd = exec.Command("cmd.exe", allArgs...)
		}
	} else {
		cmd = exec.Command(command, args...)
	}

	cmd.Dir = workingDir

	// Add environment variables
	env := append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	// Only add SSL verification bypass if requested
	if skipSSLVerify {
		env = append(env, "GIT_SSL_NO_VERIFY=true")
	}

	cmd.Env = env

	return cmd.CombinedOutput()
}

func isExecutableInPath(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

func validateGitReference(reference string) error {
	if reference == "" {
		return fmt.Errorf("empty reference")
	}

	// Check if the reference is exactly 40 hex characters (SHA-1 format)
	// This is ambiguous because Git can't determine if it's a branch name or commit SHA
	if hexPatternRegex.MatchString(reference) {
		return fmt.Errorf("ambiguous git reference: %s (exactly 40 hex characters)", reference)
	}

	// Check for other invalid characters in branch names
	// Git branch names cannot contain: space, ~, ^, :, ?, *, [, \, .., @{, //
	invalidPatterns := []string{
		" ", "~", "^", ":", "?", "*", "[", "\\", "..", "@{", "//",
	}
	for _, pattern := range invalidPatterns {
		if strings.Contains(reference, pattern) {
			return fmt.Errorf("invalid git reference: %s (contains '%s')", reference, pattern)
		}
	}

	// Check if reference starts or ends with invalid characters
	if strings.HasPrefix(reference, ".") || strings.HasSuffix(reference, ".") {
		return fmt.Errorf("invalid git reference: %s (cannot start or end with '.')", reference)
	}

	if strings.HasPrefix(reference, "/") || strings.HasSuffix(reference, "/") {
		return fmt.Errorf("invalid git reference: %s (cannot start or end with '/')", reference)
	}

	if strings.HasSuffix(reference, ".lock") {
		return fmt.Errorf("invalid git reference: %s (cannot end with '.lock')", reference)
	}

	return nil
}

func (e *Exporter) validateGitReferences(repoPath string) error {
	var ambiguousRefs []string

	// Track all reference names to detect duplicates across different types
	refNameMap := make(map[string][]string) // name -> [ref types]

	// 1. Get all branches
	branchCmd := exec.Command("git", "for-each-ref", "--format=%(refname)", "refs/heads/")
	branchCmd.Dir = repoPath
	branchOutput, err := branchCmd.Output()
	if err != nil {
		e.logger.Warn("Failed to list branches", zap.Error(err))
	} else {
		branchRefs := strings.Split(strings.TrimSpace(string(branchOutput)), "\n")
		for _, fullRef := range branchRefs {
			fullRef = strings.TrimSpace(fullRef)
			if fullRef == "" {
				continue
			}

			// Extract short name and store with type
			refName := strings.TrimPrefix(fullRef, "refs/heads/")
			refNameMap[refName] = append(refNameMap[refName], "branch")

			// Continue with existing validation for SHA-like patterns
			if err := validateGitReference(refName); err != nil {
				if strings.Contains(err.Error(), "ambiguous git reference") {
					ambiguousRefs = append(ambiguousRefs, fmt.Sprintf("branch '%s'", refName))
					e.logger.Error("Found ambiguous branch name", zap.String("branch", refName))
				} else {
					e.logger.Warn("Branch name has validation issues",
						zap.String("branch", refName), zap.Error(err))
				}
			}
		}
	}

	// 2. Get all tags
	tagCmd := exec.Command("git", "for-each-ref", "--format=%(refname)", "refs/tags/")
	tagCmd.Dir = repoPath
	tagOutput, err := tagCmd.Output()
	if err != nil {
		e.logger.Warn("Failed to list tags", zap.Error(err))
	} else {
		tagRefs := strings.Split(strings.TrimSpace(string(tagOutput)), "\n")
		for _, fullRef := range tagRefs {
			fullRef = strings.TrimSpace(fullRef)
			if fullRef == "" {
				continue
			}

			// Extract short name and store with type
			refName := strings.TrimPrefix(fullRef, "refs/tags/")
			refNameMap[refName] = append(refNameMap[refName], "tag")

			// Continue with existing validation for SHA-like patterns
			if err := validateGitReference(refName); err != nil {
				if strings.Contains(err.Error(), "ambiguous git reference") {
					ambiguousRefs = append(ambiguousRefs, fmt.Sprintf("tag '%s'", refName))
					e.logger.Error("Found ambiguous tag name", zap.String("tag", refName))
				} else {
					e.logger.Warn("Tag name has validation issues",
						zap.String("tag", refName), zap.Error(err))
				}
			}
		}
	}

	// 3. Get all remote references
	remoteCmd := exec.Command("git", "for-each-ref", "--format=%(refname)", "refs/remotes/")
	remoteCmd.Dir = repoPath
	remoteOutput, err := remoteCmd.Output()
	if err != nil {
		e.logger.Warn("Failed to list remote references", zap.Error(err))
	} else {
		remoteRefs := strings.Split(strings.TrimSpace(string(remoteOutput)), "\n")
		for _, fullRef := range remoteRefs {
			fullRef = strings.TrimSpace(fullRef)
			if fullRef == "" {
				continue
			}

			// Extract short name and store with type
			refName := strings.TrimPrefix(fullRef, "refs/remotes/")

			// Store the remote name (e.g., "origin/master")
			refNameMap[refName] = append(refNameMap[refName], "remote")

			// Also store just the branch part for checking against local branches
			parts := strings.SplitN(refName, "/", 2)
			if len(parts) == 2 {
				localBranchName := parts[1]
				if _, exists := refNameMap[localBranchName]; exists {
					// This remote branch name exists as another reference
					refNameMap[localBranchName] = append(refNameMap[localBranchName], "remote/"+parts[0])
				}
			}

			// Check for SHA patterns in remote refs too
			if err := validateGitReference(refName); err != nil {
				if strings.Contains(err.Error(), "ambiguous git reference") {
					ambiguousRefs = append(ambiguousRefs, fmt.Sprintf("remote ref '%s'", refName))
					e.logger.Error("Found ambiguous remote reference", zap.String("ref", refName))
				}
			}
		}
	}

	// 4. Check for incorrectly named remote references (refs/origin/* instead of refs/remotes/origin/*)
	badRefsCmd := exec.Command("git", "for-each-ref", "--format=%(refname)", "refs/origin/")
	badRefsCmd.Dir = repoPath
	if badRefsOutput, err := badRefsCmd.Output(); err == nil && len(badRefsOutput) > 0 {
		badRefs := strings.Split(strings.TrimSpace(string(badRefsOutput)), "\n")
		for _, ref := range badRefs {
			if ref == "" {
				continue
			}
			refName := strings.TrimPrefix(ref, "refs/origin/")
			ambiguousRefs = append(ambiguousRefs, fmt.Sprintf("incorrectly named remote ref 'refs/origin/%s' (should be 'refs/remotes/origin/%s')", refName, refName))
			e.logger.Error("Found incorrectly named remote reference",
				zap.String("actual", ref),
				zap.String("expected", "refs/remotes/origin/"+refName))
		}
	}

	// 5. Check for HEAD reference issues
	if _, hasHead := refNameMap["HEAD"]; hasHead {
		if len(refNameMap["HEAD"]) > 1 {
			// Special warning for HEAD in multiple places
			ambiguousRefs = append(ambiguousRefs, fmt.Sprintf("ambiguous 'HEAD' reference exists as both %s",
				strings.Join(refNameMap["HEAD"], " and ")))
			e.logger.Error("Found ambiguous HEAD reference",
				zap.Strings("types", refNameMap["HEAD"]))
		}
	}

	// 6. Check for duplicate reference names across different types
	for name, types := range refNameMap {
		if len(types) > 1 {
			ambiguousRefs = append(ambiguousRefs, fmt.Sprintf("ambiguous reference '%s' exists as both %s",
				name, strings.Join(types, " and ")))
			e.logger.Error("Found name used for multiple reference types",
				zap.String("name", name),
				zap.Strings("types", types))
		}
	}

	// 7. Continue with the existing filesystem check for additional validation
	refsDir := filepath.Join(repoPath, "refs")
	if _, err := os.Stat(refsDir); err == nil {
		err = filepath.Walk(refsDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Continue walking even if there's an error
			}

			if !info.IsDir() {
				refName := info.Name()
				// Check for ambiguous hex patterns in file-based refs
				if hexPatternRegex.MatchString(refName) {
					refType := "ref"
					if strings.Contains(path, "refs/heads") {
						refType = "branch"
					} else if strings.Contains(path, "refs/tags") {
						refType = "tag"
					} else if strings.Contains(path, "refs/remotes") {
						refType = "remote branch"
					}

					// Check if we already found this ref
					refStr := fmt.Sprintf("%s '%s'", refType, refName)
					found := false
					for _, ref := range ambiguousRefs {
						if ref == refStr {
							found = true
							break
						}
					}
					if !found {
						ambiguousRefs = append(ambiguousRefs, refStr)
						e.logger.Error("Found ambiguous reference in filesystem",
							zap.String("path", path),
							zap.String("ref", refName))
					}
				}
			}
			return nil
		})

		if err != nil {
			e.logger.Warn("Error walking refs directory", zap.Error(err))
		}
	}

	// 8. Check for working directory file conflicts with references
	// First, get the list of files in the working directory
	workingDirCmd := exec.Command("git", "ls-files")
	workingDirCmd.Dir = repoPath
	if workingDirOutput, err := workingDirCmd.Output(); err == nil {
		workingFiles := strings.Split(strings.TrimSpace(string(workingDirOutput)), "\n")
		for _, file := range workingFiles {
			file = strings.TrimSpace(file)
			if file == "" {
				continue
			}

			// Skip directories and focus on top-level files only
			if !strings.Contains(file, "/") && refNameMap[file] != nil {
				ambiguousRefs = append(ambiguousRefs, fmt.Sprintf("file '%s' conflicts with reference name", file))
				e.logger.Error("Working directory file conflicts with reference name",
					zap.String("file", file),
					zap.Strings("reference_types", refNameMap[file]))
			}
		}
	}

	if len(ambiguousRefs) > 0 {
		return fmt.Errorf("ambiguous Git references detected:\n%s\n\nPlease resolve these reference issues in Bitbucket before exporting",
			strings.Join(ambiguousRefs, "\n"))
	}

	return nil
}
