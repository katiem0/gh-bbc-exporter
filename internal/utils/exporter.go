package utils

import (
	"archive/tar"
	"compress/gzip"
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

func (e *Exporter) Export(workspace, repoSlug string) error {
	if e.outputDir == "" {
		timestamp := time.Now().Format("20060102-150405")
		e.outputDir = fmt.Sprintf("./bitbucket-export-%s", timestamp)
	}

	e.logger.Debug("Creating output directory", zap.String("path", e.outputDir))
	if err := os.MkdirAll(e.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	reposDir := filepath.Join(e.outputDir, "repositories", workspace, repoSlug+".git")
	if err := os.MkdirAll(reposDir, 0755); err != nil {
		return fmt.Errorf("failed to create repositories directory: %w", err)
	}

	if err := e.createRepositoryInfoFiles(workspace, repoSlug); err != nil {
		return fmt.Errorf("failed to create repository info files: %w", err)
	}

	repo, err := e.client.GetRepository(workspace, repoSlug)
	if err != nil {
		return fmt.Errorf("failed to fetch repository data: %w", err)
	}

	schema := data.MigrationArchiveSchema{
		Version: "1.0.1",
	}
	if err := e.writeJSONFile("schema.json", schema); err != nil {
		return err
	}

	defaultLabels := []data.Label{}
	repositories := e.createRepositoriesData(repo, workspace, defaultLabels)
	if err := e.writeJSONFile("repositories_000001.json", repositories); err != nil {
		return err
	}

	cloneURL := fmt.Sprintf("https://%s:%s@bitbucket.org/%s/%s.git",
		e.client.username, e.client.appPass, workspace, repoSlug)
	if err := e.CloneRepository(workspace, repoSlug, cloneURL); err != nil {
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

	prs, err := e.client.GetPullRequests(workspace, repoSlug)
	if err != nil {
		e.logger.Warn("Failed to fetch pull requests", zap.Error(err))
		prs = []data.PullRequest{}
	} else {
		e.logger.Info("Successfully fetched pull requests",
			zap.Int("count", len(prs)))
	}

	if len(prs) > 0 {
		if err := e.writeJSONFile("pull_requests_000001.json", prs); err != nil {
			return fmt.Errorf("failed to write pull requests: %w", err)
		}
	}

	regularComments, reviewComments, err := e.client.GetPullRequestComments(workspace, repoSlug, prs)
	if err != nil {
		e.logger.Warn("Failed to fetch pull request comments", zap.Error(err))
	} else {
		if len(regularComments) > 0 {
			if err := e.writeJSONFile("issue_comments_000001.json", regularComments); err != nil {
				e.logger.Warn("Failed to write issue comments", zap.Error(err))
			} else {
				e.logger.Debug("Issue comments written", zap.Int("count", len(regularComments)))
			}
		}

		if len(reviewComments) > 0 {
			if err := e.writeJSONFile("pull_request_review_comments_000001.json", reviewComments); err != nil {
				e.logger.Warn("Failed to write pull request review comments", zap.Error(err))
			} else {
				e.logger.Debug("Pull request review comments written", zap.Int("count", len(reviewComments)))
			}

			threads := e.createReviewThreads(reviewComments)
			if err := e.writeJSONFile("pull_request_review_threads_000001.json", threads); err != nil {
				e.logger.Warn("Failed to write review threads", zap.Error(err))
			}

			reviews := e.createReviews(reviewComments)
			if err := e.writeJSONFile("pull_request_reviews_000001.json", reviews); err != nil {
				e.logger.Warn("Failed to write reviews", zap.Error(err))
			}
		}
	}

	archivePath, err := e.CreateArchive()
	if err != nil {
		e.logger.Warn("Failed to create archive", zap.Error(err))
	} else {
		e.logger.Debug("Created archive of export directory",
			zap.String("archive", archivePath))
		e.outputDir = archivePath
	}

	e.logger.Info("Export completed successfully", zap.String("output", e.outputDir))
	return nil
}

func (e *Exporter) CloneRepository(workspace, repoSlug, cloneURL string) error {
	repoDir := filepath.Join(e.outputDir, "repositories", workspace, repoSlug+".git")

	e.logger.Info("Cloning repository",
		zap.String("repository", repoSlug),
		zap.String("destination", repoDir))

	e.logger.Debug("Fetching repository details from BitBucket API")
	repoDetails, err := e.client.GetRepository(workspace, repoSlug)
	defaultBranch := "main"

	if err != nil {
		e.logger.Warn("Failed to get repository details from API, will use 'main' as default branch",
			zap.Error(err))
	} else if repoDetails != nil && repoDetails.MainBranch != nil && repoDetails.MainBranch.Name != "" {
		defaultBranch = repoDetails.MainBranch.Name
		e.logger.Debug("Using mainbranch from BitBucket API",
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

	e.logger.Debug("Cloning repository to temporary directory first")
	cmd := exec.Command("git", "clone", "--mirror", cloneURL, tempDir)
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_SSL_NO_VERIFY=true")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to clone repository: %s: %w", string(output), err)
	}

	e.logger.Debug("Clone to temporary directory successful",
		zap.String("output", string(output)))
	if _, err := os.Stat(repoDir); err == nil {
		if err := os.RemoveAll(repoDir); err != nil {
			return fmt.Errorf("failed to remove existing repository directory: %w", err)
		}
	}

	if err := os.Rename(tempDir, repoDir); err != nil {
		return fmt.Errorf("failed to move repository from temp dir: %w", err)
	}

	e.logger.Debug("Updating remote URL")
	cmd = exec.Command("git", "remote", "set-url", "origin",
		fmt.Sprintf("https://bitbucket.org/%s/%s.git", workspace, repoSlug))
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		e.logger.Warn("Failed to update remote URL", zap.Error(err))
	}
	e.logger.Debug("Verifying default branch exists",
		zap.String("branch", defaultBranch))

	cmd = exec.Command("git", "rev-parse", "--verify", fmt.Sprintf("refs/heads/%s", defaultBranch))
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		e.logger.Warn("Default branch not found in repository, will attempt fallback methods",
			zap.String("expected_branch", defaultBranch),
			zap.Error(err))

		e.logger.Debug("Looking for most recent branch")
		cmd = exec.Command("git", "for-each-ref", "--sort=-committerdate", "refs/heads/", "--format=%(refname:short)", "--count=1")
		cmd.Dir = repoDir
		branchOutput, err := cmd.Output()
		if err == nil && len(branchOutput) > 0 {
			mostRecentBranch := strings.TrimSpace(string(branchOutput))
			if mostRecentBranch != "" {
				defaultBranch = mostRecentBranch
				e.logger.Debug("Found default branch by commit date",
					zap.String("branch", defaultBranch))
			}
		} else {
			for _, branch := range []string{"main", "master", "develop", "development"} {
				cmd = exec.Command("git", "rev-parse", "--verify", fmt.Sprintf("refs/heads/%s", branch))
				cmd.Dir = repoDir
				if err := cmd.Run(); err == nil {
					defaultBranch = branch
					e.logger.Debug("Found default branch from common names",
						zap.String("branch", defaultBranch))
					break
				}
			}
		}
	} else {
		e.logger.Debug("Verified default branch exists",
			zap.String("branch", defaultBranch))
	}

	headFile := filepath.Join(repoDir, "HEAD")
	headContent := fmt.Sprintf("ref: refs/heads/%s\n", defaultBranch)

	e.logger.Debug("Setting HEAD file to point to default branch",
		zap.String("branch", defaultBranch))

	if err := os.WriteFile(headFile, []byte(headContent), 0644); err != nil {
		e.logger.Error("Failed to update HEAD file",
			zap.Error(err),
			zap.String("path", headFile))
		return fmt.Errorf("failed to update HEAD file: %w", err)
	}

	refsHeadsDir := filepath.Join(repoDir, "refs", "heads")
	if _, err := os.Stat(refsHeadsDir); os.IsNotExist(err) {
		if err := os.MkdirAll(refsHeadsDir, 0755); err != nil {
			e.logger.Warn("Failed to create refs/heads directory", zap.Error(err))
		}
	}

	defaultBranchRef := filepath.Join(refsHeadsDir, defaultBranch)
	if _, err := os.Stat(defaultBranchRef); os.IsNotExist(err) {
		e.logger.Warn("Default branch reference file doesn't exist",
			zap.String("path", defaultBranchRef))

		cmd = exec.Command("git", "rev-parse", "HEAD")
		cmd.Dir = repoDir
		commitID, err := cmd.Output()
		if err == nil {
			e.logger.Info("Creating reference file for default branch",
				zap.String("branch", defaultBranch),
				zap.String("commit", strings.TrimSpace(string(commitID))))

			if err := os.WriteFile(defaultBranchRef, []byte(strings.TrimSpace(string(commitID))), 0644); err != nil {
				e.logger.Warn("Failed to create default branch reference", zap.Error(err))
			}
		}
	}

	e.logger.Debug("Updating repositories_000001.json")
	gitURL := fmt.Sprintf("tarball://root/repositories/%s/%s.git", workspace, repoSlug)
	e.updateRepositoryField(repoSlug, "default_branch", defaultBranch)
	e.updateRepositoryField(repoSlug, "git_url", gitURL)

	e.logger.Info("Repository clone and setup complete",
		zap.String("default_branch", defaultBranch))

	return nil
}

func (e *Exporter) createEmptyRepository(workspace, repoSlug string) error {
	repoDir := filepath.Join(e.outputDir, "repositories", workspace, repoSlug+".git")

	e.logger.Debug("Creating empty repository structure",
		zap.String("repository", repoSlug),
		zap.String("path", repoDir))

	// Check if directory already exists and remove it
	if _, err := os.Stat(repoDir); err == nil {
		if err := os.RemoveAll(repoDir); err != nil {
			return fmt.Errorf("failed to remove existing repository directory: %w", err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(repoDir), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	cmd := exec.Command("git", "init", "--bare", repoDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		e.logger.Error("Failed to initialize bare repository",
			zap.String("output", string(output)),
			zap.Error(err))

		if err := os.MkdirAll(repoDir, 0755); err != nil {
			return fmt.Errorf("failed to create repository directory: %w", err)
		}

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
	configContent := `[core]
	repositoryformatversion = 0
	filemode = true
	bare = true
	ignorecase = true
  precomposeunicode = true`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}

	descFile := filepath.Join(repoDir, "description")
	if err := os.WriteFile(descFile, []byte("Unnamed repository; edit this file to name it for gitweb.\n"), 0644); err != nil {
		return fmt.Errorf("failed to create description file: %w", err)
	}

	gitURL := fmt.Sprintf("tarball://root/repositories/%s/%s.git", workspace, repoSlug)
	e.updateRepositoryField(repoSlug, "default_branch", defaultBranch)
	e.updateRepositoryField(repoSlug, "git_url", gitURL)

	e.logger.Debug("Created empty repository structure")
	return nil
}

func (e *Exporter) createBasicUsers(workspace string) []data.User {
	return []data.User{
		{
			Type:      "user",
			URL:       formatURL("user", workspace, ""),
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
			URL:         formatURL("organization", workspace, ""),
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

func (e *Exporter) createRepositoriesData(repo *data.BitbucketRepository, workspace string, labels []data.Label) []data.Repository {

	createdAt := formatDateToZ(repo.CreatedOn)

	return []data.Repository{
		{
			Type:             "repository",
			URL:              formatURL("repository", workspace, repo.Name),
			Owner:            formatURL("user", workspace, ""),
			Name:             repo.Name,
			Description:      repo.Description,
			Private:          repo.IsPrivate,
			HasIssues:        true,
			HasWiki:          true,
			HasDownloads:     true,
			Labels:           []data.Label{},
			Webhooks:         []interface{}{},
			Collaborators:    []interface{}{},
			CreatedAt:        createdAt,
			GitURL:           formatURL("git", workspace, repo.Name),
			DefaultBranch:    "main",
			PublicKeys:       []interface{}{},
			Page:             nil,
			Website:          nil,
			IsArchived:       false,
			RepositoryTopics: []interface{}{},
			SecurityAndAnalysis: map[string]interface{}{
				"dependency_graph":               false,
				"vulnerability_alerts":           false,
				"vulnerability_updates":          false,
				"advanced_security":              false,
				"token_scanning":                 false,
				"token_scanning_push_protection": false,
			},
			Autolinks: []interface{}{},
			GeneralSettings: map[string]interface{}{
				"template":            false,
				"allow_forking":       false,
				"sponsorships":        false,
				"projects":            true,
				"discussions":         false,
				"merge_commit":        true,
				"squash_merge":        true,
				"rebase_merge":        true,
				"auto_merge":          false,
				"delete_branch_heads": false,
				"update_branch":       false,
				"git_lfs_in_archives": false,
			},
			ActionsGeneralSettings: map[string]interface{}{
				"actions_disabled":                 false,
				"allows_all_actions":               true,
				"allows_local_actions_only":        false,
				"allows_github_owned_actions":      false,
				"allows_verified_actions":          false,
				"allows_specific_actions_patterns": false,
				"patterns":                         []interface{}{},
			},
		},
	}
}

func (e *Exporter) createReviewThreads(comments []data.PullRequestReviewComment) []map[string]interface{} {
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

func (e *Exporter) createReviews(comments []data.PullRequestReviewComment) []map[string]interface{} {
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

func (e *Exporter) CreateArchive() (string, error) {
	baseDir := filepath.Dir(e.outputDir)
	exportDirName := filepath.Base(e.outputDir)
	archivePath := filepath.Join(baseDir, exportDirName+".tar.gz")

	e.logger.Debug("Creating archive",
		zap.String("source", e.outputDir),
		zap.String("archive", archivePath))

	archiveFile, err := os.Create(archivePath)
	if err != nil {
		return "", fmt.Errorf("failed to create archive file: %w", err)
	}
	defer archiveFile.Close()

	gzipWriter := gzip.NewWriter(archiveFile)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	if err := e.archiveDirectory(e.outputDir, tarWriter); err != nil {
		return "", fmt.Errorf("failed to build archive: %w", err)
	}

	return archivePath, nil
}

func (e *Exporter) archiveDirectory(sourceDir string, tarWriter *tar.Writer) error {
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		if relPath == "." {
			return nil
		}

		return e.addFileToArchive(tarWriter, path, relPath, info)
	})
}

func (e *Exporter) addFileToArchive(tarWriter *tar.Writer, path, relPath string, info os.FileInfo) error {
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return fmt.Errorf("failed to create tar header: %w", err)
	}

	header.Name = relPath

	if err := tarWriter.WriteHeader(header); err != nil {
		return fmt.Errorf("failed to write tar header: %w", err)
	}

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
}
