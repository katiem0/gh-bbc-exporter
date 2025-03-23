package utils

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/katiem0/gh-bbc-exporter/internal/data"
	"go.uber.org/zap"
)

type Exporter struct {
	client    *Client
	outputDir string
	logger    *zap.Logger
}

func NewExporter(client *Client, outputDir string, logger *zap.Logger) *Exporter {
	return &Exporter{
		client:    client,
		outputDir: outputDir,
		logger:    logger,
	}
}

func (e *Exporter) Export(workspace, repoSlug string, logger *zap.Logger) error {
	// Create output directory if not specified
	if e.outputDir == "" {
		timestamp := time.Now().Format("20060102-150405")
		e.outputDir = fmt.Sprintf("./bitbucket-export-%s", timestamp)
	}

	e.logger.Info("Creating output directory", zap.String("path", e.outputDir))
	if err := os.MkdirAll(e.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create repositories directory structure
	reposDir := filepath.Join(e.outputDir, "repositories", workspace)
	if err := os.MkdirAll(reposDir, 0755); err != nil {
		return fmt.Errorf("failed to create repositories directory: %w", err)
	}

	// Fetch repository data
	repo, err := e.client.GetRepository(workspace, repoSlug)
	if err != nil {
		return fmt.Errorf("failed to fetch repository data: %w", err)
	}

	// Create schema.json
	schema := data.MigrationArchiveSchema{
		Version: "1.2.0",
	}
	if err := e.writeJSONFile("schema.json", schema); err != nil {
		return err
	}

	// Create urls.json
	urls := e.createURLsTemplate()
	if err := e.writeJSONFile("urls.json", urls); err != nil {
		return err
	}

	defaultLabels := []data.Label{}
	repositories := e.createRepositoriesData(repo, workspace, defaultLabels)
	if err := e.writeJSONFile("repositories_000001.json", repositories); err != nil {
		return err
	}
	cloneURL := fmt.Sprintf("https://%s:%s@bitbucket.org/%s/%s.git",
		e.client.username, e.client.appPass, workspace, repoSlug)
	if err := e.CloneRepository(workspace, repoSlug, cloneURL, logger); err != nil {
		e.logger.Warn("Failed to clone repository, creating empty repository structure",
			zap.String("repo", repoSlug),
			zap.Error(err))
		if err := e.createEmptyRepository(workspace, repoSlug); err != nil {
			return fmt.Errorf("failed to create empty repository structure: %w", err)
		}
	}

	users, err := e.client.GetUsers(workspace, repoSlug)
	if err != nil {
		e.logger.Warn("Failed to fetch users", zap.Error(err))
		users = e.createBasicUsers(workspace)
	}
	if err := e.writeJSONFile("users_000001.json", users); err != nil {
		return err
	}

	orgs := e.createOrganizationData(workspace)
	if err := e.writeJSONFile("organizations_000001.json", orgs); err != nil {
		return err
	}

	issues, err := e.client.GetIssues(workspace, repoSlug)
	if err != nil {
		e.logger.Warn("Failed to fetch issues", zap.Error(err))
		issues = []data.Issue{}
	}
	if len(issues) > 0 {
		for i := range issues {
			issues[i].Repository = fmt.Sprintf("https://bitbucket.org/%s/%s", workspace, repoSlug)
		}
		if err := e.writeJSONFile("issues_000001.json", issues); err != nil {
			return err
		}
	}

	prs, err := e.client.GetPullRequests(workspace, repoSlug)
	if err != nil {
		e.logger.Warn("Failed to fetch pull requests", zap.Error(err))
		prs = []data.PullRequest{}
	} else {
		e.logger.Info("Successfully fetched pull requests",
			zap.Int("count", len(prs)))
	}

	if len(prs) > 0 {
		for i := range prs {
			prs[i].Repository = fmt.Sprintf("https://bitbucket.org/%s/%s", workspace, repoSlug)
		}

		// Write pull requests to file
		if err := e.writeJSONFile("pull_requests_000001.json", prs); err != nil {
			return fmt.Errorf("failed to write pull requests: %w", err)
		}
	}

	// // Fetch issue comments and create issue_comments_000001.json
	// comments, err := e.client.GetComments(workspace, repoSlug)
	// if err != nil {
	// 	e.logger.Warn("Failed to fetch comments", zap.Error(err))
	// 	// Create empty comments array if API call fails
	// 	comments = []data.IssueComment{}
	// }
	// if len(comments) > 0 {
	// 	if err := e.writeJSONFile("issue_comments_000001.json", comments); err != nil {
	// 		return err
	// 	}
	// }

	// Create teams data
	teams := e.createTeamsData(workspace, repoSlug)
	if err := e.writeJSONFile("teams_000001.json", teams); err != nil {
		return err
	}

	protectedBranches, err := e.createProtectedBranchesData(workspace, repoSlug)
	if err != nil {
		e.logger.Warn("Failed to create protected branches data", zap.Error(err))
		// Create empty protected branches array if API call fails
		protectedBranches = []data.ProtectedBranch{}
	}

	if len(protectedBranches) > 0 {
		if err := e.writeJSONFile("protected_branches_000001.json", protectedBranches); err != nil {
			return err
		}
	}

	archivePath, err := e.CreateArchive()
	if err != nil {
		e.logger.Warn("Failed to create archive", zap.Error(err))
		// Continue without creating archive - we still have the directory
	} else {
		e.logger.Info("Created archive of export directory",
			zap.String("archive", archivePath))
		// Update the output path to point to the archive
		e.outputDir = archivePath
	}

	e.logger.Info("Export completed successfully", zap.String("output", e.outputDir))
	return nil
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
		User:            "{scheme}://{+host}{/segments*}/{user}",
		Organization:    "{scheme}://{+host}/{organization}",
		Team:            "{scheme}://{+host}/{owner}/teams/{team}",
		Repository:      "{scheme}://{+host}/{owner}/{repository}",
		ProtectedBranch: "{scheme}://{+host}/{owner}/{repository}/protected_branches/{protected_branch}",
		PullRequest:     "{scheme}://{+host}/{owner}/{repository}/merge_requests/{number}",
		CommitComment:   "{scheme}://{+host}/{owner}/{repository}/commit/{commit}#note_{commit_comment}",
		Release:         "{scheme}://{+host}/{owner}/{repository}/tags/{release}",
		Label:           "{scheme}://{+host}/{owner}/{repository}/labels#/{label}",
	}
}

func (e *Exporter) CloneRepository(workspace, repoSlug, cloneURL string, logger *zap.Logger) error {
	repoDir := filepath.Join(e.outputDir, "repositories", workspace, repoSlug+".git")

	e.logger.Info("Cloning repository",
		zap.String("repository", repoSlug),
		zap.String("destination", repoDir))

	logger.Debug("Fetching repository details from BitBucket API")
	repoDetails, err := e.client.GetRepository(workspace, repoSlug)
	defaultBranch := "main"

	if err != nil {
		logger.Warn("Failed to get repository details from API, will use 'main' as default branch",
			zap.Error(err))
	} else if repoDetails != nil && repoDetails.MainBranch != nil && repoDetails.MainBranch.Name != "" {
		defaultBranch = repoDetails.MainBranch.Name
		logger.Info("Using mainbranch from BitBucket API",
			zap.String("default_branch", defaultBranch))
	}

	if err := os.MkdirAll(filepath.Dir(repoDir), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	if _, err := os.Stat(repoDir); err == nil {
		if err := os.RemoveAll(repoDir); err != nil {
			return fmt.Errorf("failed to remove existing repository directory: %w", err)
		}
	}
	tempDir, err := os.MkdirTemp("", "bbc-export-")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	logger.Debug("Cloning repository to temporary directory first")
	cmd := exec.Command("git", "clone", "--mirror", cloneURL, tempDir)
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_SSL_NO_VERIFY=true")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to clone repository: %s: %w", string(output), err)
	}

	logger.Debug("Clone to temporary directory successful",
		zap.String("output", string(output)))
	if _, err := os.Stat(repoDir); err == nil {
		if err := os.RemoveAll(repoDir); err != nil {
			return fmt.Errorf("failed to remove existing repository directory: %w", err)
		}
	}

	if err := os.Rename(tempDir, repoDir); err != nil {
		return fmt.Errorf("failed to move repository from temp dir: %w", err)
	}

	logger.Debug("Updating remote URL")
	cmd = exec.Command("git", "remote", "set-url", "origin",
		fmt.Sprintf("https://bitbucket.org/%s/%s.git", workspace, repoSlug))
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		logger.Warn("Failed to update remote URL", zap.Error(err))
	}
	logger.Debug("Verifying default branch exists",
		zap.String("branch", defaultBranch))

	cmd = exec.Command("git", "rev-parse", "--verify", fmt.Sprintf("refs/heads/%s", defaultBranch))
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		logger.Warn("Default branch not found in repository, will attempt fallback methods",
			zap.String("expected_branch", defaultBranch),
			zap.Error(err))

		logger.Debug("Looking for most recent branch")
		cmd = exec.Command("git", "for-each-ref", "--sort=-committerdate", "refs/heads/", "--format=%(refname:short)", "--count=1")
		cmd.Dir = repoDir
		branchOutput, err := cmd.Output()
		if err == nil && len(branchOutput) > 0 {
			mostRecentBranch := strings.TrimSpace(string(branchOutput))
			if mostRecentBranch != "" {
				defaultBranch = mostRecentBranch
				logger.Info("Found default branch by commit date",
					zap.String("branch", defaultBranch))
			}
		} else {
			for _, branch := range []string{"main", "master", "develop", "development"} {
				cmd = exec.Command("git", "rev-parse", "--verify", fmt.Sprintf("refs/heads/%s", branch))
				cmd.Dir = repoDir
				if err := cmd.Run(); err == nil {
					defaultBranch = branch
					logger.Info("Found default branch from common names",
						zap.String("branch", defaultBranch))
					break
				}
			}
		}
	} else {
		logger.Info("Verified default branch exists",
			zap.String("branch", defaultBranch))
	}

	headFile := filepath.Join(repoDir, "HEAD")
	headContent := fmt.Sprintf("ref: refs/heads/%s\n", defaultBranch)

	logger.Info("Setting HEAD file to point to default branch",
		zap.String("branch", defaultBranch))

	if err := os.WriteFile(headFile, []byte(headContent), 0644); err != nil {
		logger.Error("Failed to update HEAD file",
			zap.Error(err),
			zap.String("path", headFile))
		return fmt.Errorf("failed to update HEAD file: %w", err)
	}

	refsHeadsDir := filepath.Join(repoDir, "refs", "heads")
	if _, err := os.Stat(refsHeadsDir); os.IsNotExist(err) {
		if err := os.MkdirAll(refsHeadsDir, 0755); err != nil {
			logger.Warn("Failed to create refs/heads directory", zap.Error(err))
		}
	}

	defaultBranchRef := filepath.Join(refsHeadsDir, defaultBranch)
	if _, err := os.Stat(defaultBranchRef); os.IsNotExist(err) {
		logger.Warn("Default branch reference file doesn't exist",
			zap.String("path", defaultBranchRef))

		cmd = exec.Command("git", "rev-parse", "HEAD")
		cmd.Dir = repoDir
		commitID, err := cmd.Output()
		if err == nil {
			logger.Info("Creating reference file for default branch",
				zap.String("branch", defaultBranch),
				zap.String("commit", strings.TrimSpace(string(commitID))))

			if err := os.WriteFile(defaultBranchRef, []byte(strings.TrimSpace(string(commitID))), 0644); err != nil {
				logger.Warn("Failed to create default branch reference", zap.Error(err))
			}
		}
	}

	// Step 9: Update repositories_000001.json with correct default branch and git URL
	logger.Debug("Updating repositories_000001.json")
	e.updateRepositoryDefaultBranch(repoSlug, defaultBranch)

	gitURL := fmt.Sprintf("tarball://root/repositories/%s/%s.git", workspace, repoSlug)
	e.updateRepositoryGitURL(repoSlug, gitURL)

	logger.Info("Repository clone and setup complete",
		zap.String("default_branch", defaultBranch))

	return nil
}

func (e *Exporter) createEmptyRepository(workspace, repoSlug string) error {
	repoDir := filepath.Join(e.outputDir, "repositories", workspace, repoSlug+".git")

	e.logger.Info("Creating empty repository structure",
		zap.String("repository", repoSlug),
		zap.String("path", repoDir))

	// Check if directory already exists and remove it
	if _, err := os.Stat(repoDir); err == nil {
		if err := os.RemoveAll(repoDir); err != nil {
			return fmt.Errorf("failed to remove existing repository directory: %w", err)
		}
	}

	// Create parent directory
	if err := os.MkdirAll(filepath.Dir(repoDir), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Initialize empty Git repo
	cmd := exec.Command("git", "init", "--bare", repoDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		e.logger.Error("Failed to initialize bare repository",
			zap.String("output", string(output)),
			zap.Error(err))

		// If git init fails, create directory structure manually
		if err := os.MkdirAll(repoDir, 0755); err != nil {
			return fmt.Errorf("failed to create repository directory: %w", err)
		}

		// Create required directories
		for _, dir := range []string{
			filepath.Join(repoDir, "objects", "info"),
			filepath.Join(repoDir, "objects", "pack"),
			filepath.Join(repoDir, "refs", "heads"),
			filepath.Join(repoDir, "refs", "tags"),
			filepath.Join(repoDir, "hooks"),
			filepath.Join(repoDir, "info"),
		} {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
		}
	}

	defaultBranch := "main"
	emptyTreeSHA := "4b825dc642cb6eb9a060e54bf8d69288fbee4904" // Git's empty tree object
	defaultBranchRef := filepath.Join(repoDir, "refs", "heads", defaultBranch)
	if err := os.WriteFile(defaultBranchRef, []byte(emptyTreeSHA+"\n"), 0644); err != nil {
		e.logger.Warn("Failed to create default branch reference", zap.Error(err))
	}

	headFile := filepath.Join(repoDir, "HEAD")
	headContent := fmt.Sprintf("ref: refs/heads/%s\n", defaultBranch)
	if err := os.WriteFile(headFile, []byte(headContent), 0644); err != nil {
		return fmt.Errorf("failed to create HEAD file: %w", err)
	}

	configFile := filepath.Join(repoDir, "config")
	configContent := fmt.Sprintf(`[core]
	repositoryformatversion = 0
	filemode = true
	bare = true
[remote "origin"]
	url = https://bitbucket.org/%s/%s.git
	fetch = +refs/heads/*:refs/remotes/origin/*
`, workspace, repoSlug)
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}

	descFile := filepath.Join(repoDir, "description")
	if err := os.WriteFile(descFile, []byte("Unnamed repository; edit this file to name it for gitweb.\n"), 0644); err != nil {
		return fmt.Errorf("failed to create description file: %w", err)
	}

	e.updateRepositoryDefaultBranch(repoSlug, defaultBranch)

	gitURL := fmt.Sprintf("tarball://root/repositories/%s/%s.git", workspace, repoSlug)
	e.updateRepositoryGitURL(repoSlug, gitURL)

	e.logger.Info("Created empty repository structure")
	return nil
}

func (e *Exporter) createBasicUsers(workspace string) []data.User {
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
			CreatedAt: formatDateToZ(time.Now().Format(time.RFC3339)),
		},
	}
}

func (e *Exporter) createOrganizationData(workspace string) []data.Organization {
	return []data.Organization{
		{
			Type:        "organization",
			URL:         fmt.Sprintf("https://bitbucket.org/%s", workspace),
			Login:       workspace,
			Name:        workspace,
			Description: "",
			Website:     nil,
			Location:    nil,
			Email:       nil,
			Members:     []data.Member{},
		},
	}
}

func (e *Exporter) createTeamsData(workspace, repoSlug string) []data.Team {
	now := formatDateToZ(time.Now().Format(time.RFC3339))
	description := ""
	return []data.Team{
		{
			Type:         "team",
			URL:          fmt.Sprintf("https://bitbucket.org/%s/teams/%s-admin-access", workspace, workspace),
			Organization: fmt.Sprintf("https://bitbucket.org/%s", workspace),
			Name:         fmt.Sprintf("%s Admin Access", workspace),
			Description:  &description,
			Permissions: []data.Permission{
				{
					Repository: fmt.Sprintf("https://bitbucket.org/%s/%s", workspace, repoSlug),
					Access:     "admin",
				},
			},
			Members:   []data.TeamMember{},
			CreatedAt: now,
		},
	}
}

func (e *Exporter) createProtectedBranchesData(workspace string, repoSlug string) ([]data.ProtectedBranch, error) {
	filePath := filepath.Join(e.outputDir, "repositories_000001.json")
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read repositories file: %w", err)
	}

	// Parse repositories JSON - fixing the unmarshaling to use a slice
	var repositories []data.Repository
	if err := json.Unmarshal(fileData, &repositories); err != nil {
		return nil, fmt.Errorf("failed to parse repositories file: %w", err)
	}

	defaultBranch := "main"
	for _, repo := range repositories {
		if repo.Name == repoSlug {
			defaultBranch = repo.DefaultBranch
			e.logger.Debug("Found default branch for repository",
				zap.String("repo", repoSlug),
				zap.String("branch", defaultBranch))
			break
		}
	}

	restrictions, err := e.client.GetBranchRestrictions(workspace, repoSlug)
	if err != nil {
		e.logger.Warn("Failed to fetch branch restrictions",
			zap.String("repo", repoSlug),
			zap.Error(err))
	}

	branchProtections := make(map[string]*data.ProtectedBranch)

	for _, restriction := range restrictions {
		branchName := restriction.BranchPattern
		if branchName == "" || branchName == "**" {
			branchName = defaultBranch
		}

		if _, exists := branchProtections[branchName]; !exists {
			branchProtections[branchName] = &data.ProtectedBranch{
				Type: "protected_branch",
				Name: branchName,
				URL: fmt.Sprintf("https://bitbucket.org/%s/%s/protected_branches/%s",
					workspace, repoSlug, branchName),
				CreatorURL:                           fmt.Sprintf("https://bitbucket.org/%s", workspace),
				RepositoryURL:                        fmt.Sprintf("https://bitbucket.org/%s/%s", workspace, repoSlug),
				AdminEnforced:                        true,
				BlockDeletionsEnforcementLevel:       0,
				BlockForcePushesEnforcementLevel:     0,
				DismissStaleReviewsOnPush:            false,
				PullRequestReviewsEnforcementLevel:   "off",
				RequireCodeOwnerReview:               false,
				RequiredStatusChecksEnforcementLevel: "off",
				StrictRequiredStatusChecksPolicy:     false,
				AuthorizedActorsOnly:                 false,
				AuthorizedUserURLs:                   []string{},
				AuthorizedTeamURLs:                   []string{},
				DismissalRestrictedUserURLs:          []string{},
				DismissalRestrictedTeamURLs:          []string{},
				RequiredStatusChecks:                 []string{},
			}
		}

		// Update protection based on the restriction type
		protection := branchProtections[branchName]
		switch restriction.Type {
		case "push":
			protection.AuthorizedActorsOnly = true
			protection.AuthorizedUserURLs = append(protection.AuthorizedUserURLs, restriction.Users...)
		case "force_push":
			protection.BlockForcePushesEnforcementLevel = 2
		case "delete":
			protection.BlockDeletionsEnforcementLevel = 2
		case "require_reviews":
			protection.PullRequestReviewsEnforcementLevel = "off"
		case "require_code_owner_review":
			protection.RequireCodeOwnerReview = true
		}
	}

	// Always protect the default branch if no other protections exist
	if len(branchProtections) == 0 {
		branchProtections[defaultBranch] = &data.ProtectedBranch{
			Type: "protected_branch",
			Name: defaultBranch,
			URL: fmt.Sprintf("https://bitbucket.org/%s/%s/protected_branches/%s",
				workspace, repoSlug, defaultBranch),
			CreatorURL:                           fmt.Sprintf("https://bitbucket.org/%s", workspace),
			RepositoryURL:                        fmt.Sprintf("https://bitbucket.org/%s/%s", workspace, repoSlug),
			AdminEnforced:                        true,
			BlockDeletionsEnforcementLevel:       2, // Default protection for default branch
			BlockForcePushesEnforcementLevel:     2, // Default protection for default branch
			DismissStaleReviewsOnPush:            false,
			PullRequestReviewsEnforcementLevel:   "off",
			RequireCodeOwnerReview:               false,
			RequiredStatusChecksEnforcementLevel: "off",
			StrictRequiredStatusChecksPolicy:     false,
			AuthorizedActorsOnly:                 false,
			AuthorizedUserURLs:                   []string{},
			AuthorizedTeamURLs:                   []string{},
			DismissalRestrictedUserURLs:          []string{},
			DismissalRestrictedTeamURLs:          []string{},
			RequiredStatusChecks:                 []string{},
		}
	}

	// Convert map to slice
	protectedBranches := make([]data.ProtectedBranch, 0, len(branchProtections))
	for _, protection := range branchProtections {
		protectedBranches = append(protectedBranches, *protection)
	}

	e.logger.Info("Created protected branches data",
		zap.Int("count", len(protectedBranches)))

	return protectedBranches, nil
}

func (e *Exporter) createRepositoriesData(repo *data.BitbucketRepository, workspace string, labels []data.Label) []data.Repository {
	// Format creation date to ISO 8601
	createdAt := formatDateToZ(repo.CreatedOn)

	// Create repository entry
	return []data.Repository{
		{
			Type:          "repository",
			URL:           fmt.Sprintf("https://bitbucket.org/%s/%s", workspace, repo.Name),
			Owner:         fmt.Sprintf("https://bitbucket.org/%s", workspace),
			Name:          repo.Name,
			Description:   repo.Description,
			Private:       repo.IsPrivate,
			HasIssues:     true,
			HasWiki:       false,
			HasDownloads:  true,
			Labels:        labels,
			Webhooks:      []interface{}{},
			Collaborators: []interface{}{},
			CreatedAt:     createdAt,
			GitURL:        fmt.Sprintf("tarball://root/repositories/%s/%s.git", workspace, repo.Name),
			DefaultBranch: "main",
			PublicKeys:    []interface{}{},
			WikiURL:       "",
		},
	}
}

func (e *Exporter) GetOutputPath() string {
	return e.outputDir
}

func (e *Exporter) CreateArchive() (string, error) {
	// Get the base directory and export directory name
	baseDir := filepath.Dir(e.outputDir)
	exportDirName := filepath.Base(e.outputDir)

	// Create archive name using the export directory name
	archivePath := filepath.Join(baseDir, exportDirName+".tar.gz")

	e.logger.Info("Creating archive",
		zap.String("source", e.outputDir),
		zap.String("archive", archivePath))

	// Create the archive file
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		return "", fmt.Errorf("failed to create archive file: %w", err)
	}
	defer archiveFile.Close()

	// Create a gzip writer
	gzipWriter := gzip.NewWriter(archiveFile)
	defer gzipWriter.Close()

	// Create a tar writer
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Walk through all files in the export directory
	err = filepath.Walk(e.outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get the relative path from the export directory, not the parent dir
		// This is crucial for GSM Actions compatibility
		relPath, err := filepath.Rel(e.outputDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		// Create header for the file
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("failed to create tar header: %w", err)
		}

		// Use relative path for the file name
		header.Name = relPath

		// Write the header to the archive
		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header: %w", err)
		}

		// If it's a regular file, write its contents
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("failed to open file %s: %w", path, err)
			}
			defer file.Close()

			if _, err := io.Copy(tarWriter, file); err != nil {
				return fmt.Errorf("failed to copy file contents: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to build archive: %w", err)
	}

	return archivePath, nil
}

func (e *Exporter) updateRepositoryGitURL(repoSlug, gitURL string) {
	// Read the current repositories file
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

	// Find and update the repository
	repoUpdated := false
	for i, repo := range repositories {
		if repo.Name == repoSlug {
			repositories[i].GitURL = gitURL
			repoUpdated = true
			break
		}
	}

	if !repoUpdated {
		e.logger.Warn("Repository not found in repositories file",
			zap.String("repo", repoSlug))
		return
	}

	// Write the updated JSON back to the file
	updatedData, err := json.MarshalIndent(repositories, "", "  ")
	if err != nil {
		e.logger.Warn("Failed to marshal updated repositories data", zap.Error(err))
		return
	}

	if err := os.WriteFile(filePath, updatedData, 0644); err != nil {
		e.logger.Warn("Failed to write updated repositories file", zap.Error(err))
		return
	}

	e.logger.Info("Updated git_url in repositories file",
		zap.String("git_url", gitURL))
}
