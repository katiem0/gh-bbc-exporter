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
	"time"

	"github.com/katiem0/gh-bbc-exporter/internal/data"
	"go.uber.org/zap"
)

// Exporter handles exporting data from BitBucket to GitHub migration archive format
type Exporter struct {
	client    *Client
	outputDir string
	logger    *zap.Logger
}

// NewExporter creates a new exporter
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

	// Create default labels for the repository
	defaultLabels := []data.Label{}

	// Create repositories_000001.json
	repositories := e.createRepositoriesData(repo, workspace, defaultLabels)
	if err := e.writeJSONFile("repositories_000001.json", repositories); err != nil {
		return err
	}

	// Clone the Git repository
	cloneURL := fmt.Sprintf("https://%s:%s@bitbucket.org/%s/%s.git",
		e.client.username, e.client.appPass, workspace, repoSlug)
	if err := e.CloneRepository(workspace, repoSlug, cloneURL, logger); err != nil {
		e.logger.Warn("Failed to clone repository, creating empty repository structure",
			zap.String("repo", repoSlug),
			zap.Error(err))

		// Create an empty repository structure if clone fails
		if err := e.createEmptyRepository(workspace, repoSlug); err != nil {
			return fmt.Errorf("failed to create empty repository structure: %w", err)
		}
	}

	// Fetch users and create users_000001.json
	users, err := e.client.GetUsers(workspace, repoSlug)
	if err != nil {
		e.logger.Warn("Failed to fetch users", zap.Error(err))
		// Create a basic set of users if API call fails
		users = e.createBasicUsers(workspace)
	}
	if err := e.writeJSONFile("users_000001.json", users); err != nil {
		return err
	}

	// Create organization data and create organizations_000001.json
	orgs := e.createOrganizationData(workspace)
	if err := e.writeJSONFile("organizations_000001.json", orgs); err != nil {
		return err
	}

	// Fetch issues and create issues_000001.json
	issues, err := e.client.GetIssues(workspace, repoSlug)
	if err != nil {
		e.logger.Warn("Failed to fetch issues", zap.Error(err))
		// Create empty issues array if API call fails
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

	// Fetch pull requests and create pull_requests_000001.json
	prs, err := e.client.GetPullRequests(workspace, repoSlug)
	if err != nil {
		e.logger.Warn("Failed to fetch pull requests", zap.Error(err))
		// Create empty pull requests array if API call fails
		prs = []data.PullRequest{}
	}
	if len(prs) > 0 {
		for i := range prs {
			prs[i].Repository = fmt.Sprintf("https://bitbucket.org/%s/%s", workspace, repoSlug)
		}
		if err := e.writeJSONFile("pull_requests_000001.json", prs); err != nil {
			return err
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

	// // Create protected branches
	// protectedBranches := e.createProtectedBranchesData(workspace, repoSlug)
	// if err := e.writeJSONFile("protected_branches_000001.json", protectedBranches); err != nil {
	// 	return err
	// }

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

// writeJSONFile writes data as JSON to a file
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

// createURLsTemplate creates the URL templates for GitHub resources
func (e *Exporter) createURLsTemplate() data.URLs {
	return data.URLs{
		User:         "{scheme}://{+host}{/segments*}/{user}",
		Organization: "{scheme}://{+host}/{organization}",
		Team:         "{scheme}://{+host}/{owner}/teams/{team}",
		Repository:   "{scheme}://{+host}/{owner}/{repository}",
		PullRequest:  "{scheme}://{+host}/{owner}/{repository}/pull-requests/{number}",
		// Add other URL templates...
		// IssueComment: data.IssueCommentURLs{
		// 	Issue:       "{scheme}://{+host}/{owner}/{repository}/issues/{number}#note_{issue_comment}",
		// 	PullRequest: "{scheme}://{+host}/{owner}/{repository}/merge_requests/{number}#note_{issue_comment}",
		// },
	}
}

// CloneRepository clones a BitBucket repository to the export directory
func (e *Exporter) CloneRepository(workspace, repoSlug, cloneURL string, logger *zap.Logger) error {
	repoDir := filepath.Join(e.outputDir, "repositories", workspace, repoSlug+".git")

	e.logger.Info("Cloning repository",
		zap.String("repository", repoSlug),
		zap.String("destination", repoDir))

	// Create directory
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		return fmt.Errorf("failed to create repository directory: %w", err)
	}

	cmd := exec.Command("git", "clone", "--bare", "--mirror", cloneURL, repoDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to clone repository: %s: %w", string(output), err)
	}

	logger.Info("Successfully cloned repository", zap.String("output", string(output)))

	// Update the remote URL to remove credentials
	cmd = exec.Command("git", "remote", "set-url", "origin",
		fmt.Sprintf("https://bitbucket.org/%s/%s.git", workspace, repoSlug))
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		e.logger.Warn("Failed to update remote URL", zap.Error(err))
	}
	cmd = exec.Command("git", "branch")
	cmd.Dir = repoDir
	branchOutput, err := cmd.CombinedOutput()
	if err != nil {
		e.logger.Warn("Failed to list branches",
			zap.String("output", string(branchOutput)),
			zap.Error(err))
	} else {
		e.logger.Info("Branches in cloned repository",
			zap.String("branches", string(branchOutput)))
	}
	// Fetch all
	cmd = exec.Command("git", "fetch", "--all")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to fetch repository: %w", err)
	}

	// Fetch pull request data and create refs for merge requests
	// prs, err := e.client.GetPullRequests(workspace, repoSlug)
	// if err == nil && len(prs) > 0 {
	// 	for _, pr := range prs {
	// 		// Extract PR ID from URL
	// 		urlParts := strings.Split(pr.URL, "/")
	// 		prID := urlParts[len(urlParts)-1]

	// 		// Create directory for this PR
	// 		prDir := filepath.Join(mrDir, prID)
	// 		if err := os.MkdirAll(prDir, 0755); err != nil {
	// 			e.logger.Warn("Failed to create PR directory",
	// 				zap.String("path", prDir),
	// 				zap.Error(err))
	// 			continue
	// 		}

	// 		// Create head file with PR branch SHA
	// 		headPath := filepath.Join(prDir, "head")
	// 		if err := os.WriteFile(headPath, []byte(pr.Head.Sha+"\n"), 0644); err != nil {
	// 			e.logger.Warn("Failed to create PR head file",
	// 				zap.String("path", headPath),
	// 				zap.Error(err))
	// 		}

	// 		// If PR is merged, create merge file
	// 		if pr.MergedAt != nil {
	// 			mergePath := filepath.Join(prDir, "merge")
	// 			// For merged PRs, use the head SHA as a fallback
	// 			// In a real implementation, you'd need to get the merge commit SHA from BitBucket
	// 			mergeCommit := pr.Head.Sha
	// 			if err := os.WriteFile(mergePath, []byte(mergeCommit+"\n"), 0644); err != nil {
	// 				e.logger.Warn("Failed to create PR merge file",
	// 					zap.String("path", mergePath),
	// 					zap.Error(err))
	// 			}
	// 		}
	// 	}
	// }

	return nil
}

// createEmptyRepository creates an empty repository structure when cloning fails
func (e *Exporter) createEmptyRepository(workspace, repoSlug string) error {
	repoDir := filepath.Join(e.outputDir, "repositories", workspace, repoSlug+".git")

	e.logger.Info("Creating empty repository structure",
		zap.String("repository", repoSlug),
		zap.String("path", repoDir))

	// Create directory
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		return fmt.Errorf("failed to create repository directory: %w", err)
	}

	// Initialize bare repo
	cmd := exec.Command("git", "init", "--bare")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to initialize bare repository: %w", err)
	}

	// Create empty dirs and required files
	dirsToCreate := []string{
		filepath.Join(repoDir, "objects", "pack"),
		filepath.Join(repoDir, "refs", "heads"),
		filepath.Join(repoDir, "refs", "tags"),
		filepath.Join(repoDir, "refs", "merge-requests"),
	}

	for _, dir := range dirsToCreate {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create HEAD file
	headFile := filepath.Join(repoDir, "HEAD")
	if err := os.WriteFile(headFile, []byte("ref: refs/heads/master\n"), 0644); err != nil {
		return fmt.Errorf("failed to create HEAD file: %w", err)
	}

	// Create description file
	descFile := filepath.Join(repoDir, "description")
	if err := os.WriteFile(descFile, []byte("Unnamed repository; edit this file 'description' to name the repository.\n"), 0644); err != nil {
		return fmt.Errorf("failed to create description file: %w", err)
	}

	// Create config file
	configContent := fmt.Sprintf(`[core]
	bare = true
	repositoryformatversion = 0
	filemode = true
[remote "origin"]
	url = https://bitbucket.org/%s/%s.git
	fetch = +refs/heads/*:refs/remotes/origin/*
`, workspace, repoSlug)
	configFile := filepath.Join(repoDir, "config")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}

	// Create hooks directory with README
	hooksDir := filepath.Join(repoDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}
	readmeContent := `#!/bin/sh
#
# Place appropriately named executable hook scripts into this directory
# to intercept various actions that git takes.  See 'git help hooks' for
# more information.
`
	readmeFile := filepath.Join(hooksDir, "README.sample")
	if err := os.WriteFile(readmeFile, []byte(readmeContent), 0644); err != nil {
		return fmt.Errorf("failed to create hooks README: %w", err)
	}

	// Create info directory with exclude file
	infoDir := filepath.Join(repoDir, "info")
	if err := os.MkdirAll(infoDir, 0755); err != nil {
		return fmt.Errorf("failed to create info directory: %w", err)
	}
	excludeContent := `# File patterns to ignore; see 'git help ignore' for more information.
# Lines that start with '#' are comments.
`
	excludeFile := filepath.Join(infoDir, "exclude")
	if err := os.WriteFile(excludeFile, []byte(excludeContent), 0644); err != nil {
		return fmt.Errorf("failed to create exclude file: %w", err)
	}

	// Create empty FETCH_HEAD
	fetchHeadFile := filepath.Join(repoDir, "FETCH_HEAD")
	if err := os.WriteFile(fetchHeadFile, []byte(""), 0644); err != nil {
		return fmt.Errorf("failed to create FETCH_HEAD file: %w", err)
	}

	return nil
}

// createBasicUsers creates a basic set of users when API calls fail
func (e *Exporter) createBasicUsers(workspace string) []data.User {
	// Create a single user representing the workspace
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
			CreatedAt: time.Now().Format("2006-01-02T15:04:05.000Z"),
		},
	}
}

// createOrganizationData creates organization data
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

// createTeamsData creates team data
func (e *Exporter) createTeamsData(workspace, repoSlug string) []data.Team {
	now := time.Now().Format("2006-01-02 15:04:05 -0700")
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

// // createProtectedBranchesData creates protected branches data
// func (e *Exporter) createProtectedBranchesData(workspace, repoSlug string) []data.ProtectedBranch {
// 	return []data.ProtectedBranch{
// 		{
// 			Type:                                 "protected_branch",
// 			Name:                                 "main",
// 			URL:                                  fmt.Sprintf("https://bitbucket.org/%s/%s/protected_branches/main", workspace, repoSlug),
// 			CreatorURL:                           fmt.Sprintf("https://bitbucket.org/%s", workspace),
// 			RepositoryURL:                        fmt.Sprintf("https://bitbucket.org/%s/%s", workspace, repoSlug),
// 			AdminEnforced:                        true,
// 			BlockDeletionsEnforcementLevel:       2,
// 			BlockForcePushesEnforcementLevel:     2,
// 			DismissStaleReviewsOnPush:            false,
// 			PullRequestReviewsEnforcementLevel:   "off",
// 			RequireCodeOwnerReview:               false,
// 			RequiredStatusChecksEnforcementLevel: "off",
// 			StrictRequiredStatusChecksPolicy:     false,
// 			AuthorizedActorsOnly:                 false,
// 			AuthorizedUserURLs:                   []string{},
// 			AuthorizedTeamURLs:                   []string{},
// 			DismissalRestrictedUserURLs:          []string{},
// 			DismissalRestrictedTeamURLs:          []string{},
// 			RequiredStatusChecks:                 []string{},
// 		},
// 	}
// }

// Update the createRepositoriesData function to accept labels
func (e *Exporter) createRepositoriesData(repo *data.BitbucketRepository, workspace string, labels []data.Label) []data.Repository {
	// Format creation date
	createdAt := repo.CreatedOn

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
			Collaborators: []interface{}{},
			CreatedAt:     createdAt,
			GitURL:        fmt.Sprintf("tarball://root/repositories/%s/%s.git", workspace, repo.Name),
			DefaultBranch: "main",
			PublicKeys:    []interface{}{}, // You might need to fetch this from the API
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
