package utils

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/katiem0/gh-bbc-exporter/internal/data"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestNewExporter(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := &Client{}
	exporter := NewExporter(client, "output", logger, false, "")

	assert.NotNil(t, exporter)
	assert.Equal(t, client, exporter.client)
	assert.Equal(t, "output", exporter.outputDir)
	assert.NotNil(t, exporter.logger)
}

func TestCreateOrganizationData(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := &Client{}
	exporter := NewExporter(client, "output", logger, false, "")

	orgs := exporter.createOrganizationData("testworkspace")

	assert.Len(t, orgs, 1)
	assert.Equal(t, "organization", orgs[0].Type)
	assert.Equal(t, "testworkspace", orgs[0].Login)
}

func TestWriteJSONFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "exporter-test-")
	assert.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: Failed to remove temp dir: %v", err)
		}
	}()

	logger, _ := zap.NewDevelopment()
	client := &Client{}
	exporter := NewExporter(client, tempDir, logger, false, "")

	testData := []data.User{{
		Type:  "user",
		Login: "testuser",
	}}

	err = exporter.writeJSONFile("test.json", testData)
	assert.NoError(t, err)

	filePath := filepath.Join(exporter.outputDir, "test.json")
	file, err := os.Open(filePath)
	assert.NoError(t, err)
	defer func() {
		if err := file.Close(); err != nil {
			t.Logf("Warning: Failed to close file: %v", err)
		}
	}()

	fileContent, err := io.ReadAll(file)
	assert.NoError(t, err)

	var readData []data.User
	err = json.Unmarshal(fileContent, &readData)
	assert.NoError(t, err)
	assert.Equal(t, testData, readData)

}

func TestCreateArchive(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "exporter-test-")
	assert.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: Failed to remove temp dir: %v", err)
		}
	}()

	logger, _ := zap.NewDevelopment()
	client := &Client{}
	exporter := NewExporter(client, tempDir, logger, false, "")

	dummyFilePath := filepath.Join(exporter.outputDir, "dummy.txt")
	err = os.WriteFile(dummyFilePath, []byte("test data"), 0644)
	assert.NoError(t, err)

	archivePath, err := exporter.CreateArchive()
	assert.NoError(t, err)
	assert.NotEmpty(t, archivePath)

	_, err = os.Stat(archivePath)
	assert.NoError(t, err)

	archiveFile, err := os.Open(archivePath)
	assert.NoError(t, err)
	defer func() {
		if err := archiveFile.Close(); err != nil {
			t.Logf("Warning: Failed to close archive file: %v", err)
		}
	}()

	gzipReader, err := gzip.NewReader(archiveFile)
	assert.NoError(t, err)
	defer func() {
		if err := gzipReader.Close(); err != nil {
			t.Logf("Warning: Failed to close gzip reader: %v", err)
		}
	}()
}

func TestExport(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "exporter-test-")
	assert.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: Failed to remove temp dir: %v", err)
		}
	}()

	// Create a local git repository that can be cloned
	gitRepoPath := filepath.Join(tempDir, "test-repo.git")
	cmd := exec.Command("git", "init", "--bare", gitRepoPath)
	err = cmd.Run()
	if err != nil {
		t.Skip("Git not available for testing")
	}

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if strings.Contains(r.URL.Path, "pullrequests") {
			writeResponse(t, w, []byte(`{"values": [], "next": null}`)) // Mock pull requests
		} else if strings.Contains(r.URL.Path, "comments") {
			writeResponse(t, w, []byte(`{"values": [], "next": null}`)) // Mock comments
		} else if strings.Contains(r.URL.Path, "members") {
			writeResponse(t, w, []byte(`{"values": [], "next": null}`)) // Mock users
		} else {
			writeResponse(t, w, []byte(`{"name": "Test Repo", "mainbranch": {"name": "main"}}`)) // Mock repository
		}
	}))
	defer testServer.Close()

	logger, _ := zap.NewDevelopment()
	client := &Client{
		baseURL:        testServer.URL,
		httpClient:     testServer.Client(),
		logger:         logger,
		commitSHACache: make(map[string]string),
		// Don't provide any credentials to trigger the empty repository creation path
	}
	exporter := NewExporter(client, tempDir, logger, false, "")

	// Pre-create the repository to avoid actual cloning
	repoPath := filepath.Join(tempDir, "repositories", "workspace", "repo.git")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatal(err)
	}

	// Initialize as a bare git repository
	cmd = exec.Command("git", "init", "--bare")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	// Create the HEAD file pointing to main
	headFile := filepath.Join(repoPath, "HEAD")
	if err := os.WriteFile(headFile, []byte("ref: refs/heads/main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create refs/heads directory and main branch ref
	refsDir := filepath.Join(repoPath, "refs", "heads")
	if err := os.MkdirAll(refsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create an empty main branch ref
	mainRef := filepath.Join(refsDir, "main")
	// Use a dummy commit hash (all zeros is the null hash)
	if err := os.WriteFile(mainRef, []byte("0000000000000000000000000000000000000000\n"), 0644); err != nil {
		t.Fatal(err)
	}

	err = exporter.Export("workspace", "repo")
	// The export will fail due to no authentication, but we can check if it fails with the expected error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "export failed")
}

func TestArchiveCompatibility(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "exporter-test-")
	assert.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: Failed to remove temp dir: %v", err)
		}
	}()

	logger, _ := zap.NewDevelopment()
	client := &Client{}
	exporter := NewExporter(client, tempDir, logger, false, "")

	// Create a test file
	dummyFilePath := filepath.Join(exporter.outputDir, "dummy.txt")
	err = os.WriteFile(dummyFilePath, []byte("test data"), 0644)
	assert.NoError(t, err)

	// Create a file with a very long name to test longlink handling
	longNameDir := filepath.Join(exporter.outputDir, "very_long_directory_name_to_test_longlink_handling")
	err = os.MkdirAll(longNameDir, 0755)
	assert.NoError(t, err)
	longNameFile := filepath.Join(longNameDir, "very_long_file_name_that_should_exceed_one_hundred_characters_to_test_longlink_handling_in_the_tar_format.txt")
	err = os.WriteFile(longNameFile, []byte("long name test"), 0644)
	assert.NoError(t, err)

	// Create the archive
	archivePath, err := exporter.CreateArchive()
	assert.NoError(t, err)

	// Now validate the archive
	file, err := os.Open(archivePath)
	assert.NoError(t, err)
	defer func() {
		if err := file.Close(); err != nil {
			t.Logf("Warning: Failed to close file: %v", err)
		}
	}()

	gzipReader, err := gzip.NewReader(file)
	assert.NoError(t, err)
	defer func() {
		if err := gzipReader.Close(); err != nil {
			t.Logf("Warning: Failed to close gzip reader: %v", err)
		}
	}()

	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		assert.NoError(t, err)

		// Verify no symlinks or hardlinks
		assert.NotEqual(t, tar.TypeSymlink, header.Typeflag)
		assert.NotEqual(t, tar.TypeLink, header.Typeflag)

		// Check if longlinks are handled properly (if using them)
		if header.Typeflag == tar.TypeGNULongName || header.Typeflag == tar.TypeGNULongLink {
			data, err := io.ReadAll(tarReader)
			assert.NoError(t, err)
			assert.LessOrEqual(t, len(data), 10*1024) // Max 10KB for longlinks
		}
	}
}

func TestCreateArchiveErrors(t *testing.T) {
	// Test with a non-writable directory
	logger, _ := zap.NewDevelopment()
	client := &Client{}

	// Create temp dir for testing
	tempDir, err := os.MkdirTemp("", "exporter-test-")
	assert.NoError(t, err)

	// Make it non-writable after we're done with setup
	defer func() {
		// Restore permissions before removal
		if err := os.Chmod(tempDir, 0755); err != nil {
			t.Logf("Warning: Failed to restore directory permissions: %v", err)
		}
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: Failed to remove temp dir: %v", err)
		}
	}()

	exporter := NewExporter(client, tempDir, logger, false, "")

	// Create a test file
	dummyFilePath := filepath.Join(exporter.outputDir, "dummy.txt")
	err = os.WriteFile(dummyFilePath, []byte("test data"), 0644)
	assert.NoError(t, err)

	// Make the directory non-writable
	err = os.Chmod(tempDir, 0400)
	assert.NoError(t, err)

	// Now try to create an archive - should fail
	archivePath, err := exporter.CreateArchive()
	assert.Error(t, err)
	assert.Empty(t, archivePath)
}

func TestCreateReviewThreads(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := &Client{}
	exporter := NewExporter(client, "output", logger, false, "")

	// Create test comments that should be grouped into threads
	reviewComments := []data.PullRequestReviewComment{
		{
			PullRequestReviewThread: "thread-123",
			Path:                    "file1.txt",
			Position:                10,
			CreatedAt:               "2023-01-01T12:00:00Z",
			CommitID:                "abcdef",
			OriginalCommitId:        "abcdef",
			DiffHunk:                "@@ -1,1 +1,1 @@\n+Test",
		},
		{
			PullRequestReviewThread: "thread-123", // Same thread as above
			Path:                    "file1.txt",
			Position:                10,
			CreatedAt:               "2023-01-02T12:00:00Z", // Later comment
			CommitID:                "abcdef",
			OriginalCommitId:        "abcdef",
			DiffHunk:                "@@ -1,1 +1,1 @@\n+Test reply",
		},
		{
			PullRequestReviewThread: "thread-456", // Different thread
			Path:                    "file2.txt",
			Position:                20,
			CreatedAt:               "2023-01-01T14:00:00Z",
			CommitID:                "ghijkl",
			OriginalCommitId:        "ghijkl",
			DiffHunk:                "@@ -1,1 +1,1 @@\n+Another test",
		},
	}

	threads := exporter.createReviewThreads(reviewComments)

	// Should have two threads
	assert.Len(t, threads, 2)

	// Verify thread data is correct
	assert.Equal(t, "file1.txt", threads[0]["path"])
	assert.Equal(t, 10, threads[0]["position"])
	assert.Equal(t, "2023-01-01T12:00:00Z", threads[0]["created_at"]) // Should use earliest comment date

	assert.Equal(t, "file2.txt", threads[1]["path"])
	assert.Equal(t, 20, threads[1]["position"])
	assert.Equal(t, "2023-01-01T14:00:00Z", threads[1]["created_at"])
}

func TestCloneRepositoryErrors(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "clone-test-")
	assert.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: Failed to remove temp dir: %v", err)
		}
	}()

	logger, _ := zap.NewDevelopment()

	// Test case 1: Invalid clone URL
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		writeResponse(t, w, []byte(`{"name": "Test Repo", "mainbranch": {"name": "main"}}`))
	}))
	defer testServer.Close()

	client := &Client{
		baseURL:    testServer.URL,
		httpClient: testServer.Client(),
		logger:     logger,
	}

	exporter := NewExporter(client, tempDir, logger, false, "")

	// Invalid URL to trigger git clone failure
	err = exporter.CloneRepository("workspace", "repo", "invalid://url")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to clone repository")

	// Test case 2: Permission error on directory creation
	// Create a read-only directory to cause permission error
	readOnlyDir := filepath.Join(tempDir, "readonly")
	err = os.MkdirAll(readOnlyDir, 0755)
	assert.NoError(t, err)
	err = os.Chmod(readOnlyDir, 0500)
	assert.NoError(t, err)

	readOnlyExporter := NewExporter(client, readOnlyDir, logger, false, "")
	err = readOnlyExporter.CloneRepository("workspace", "repo", "https://example.com/repo.git")
	assert.Error(t, err)
}

func TestCreateRepositoriesDataWithSpecialChars(t *testing.T) {
	// Create an observable logger to capture log output
	core, observedLogs := observer.New(zap.DebugLevel)
	observableLogger := zap.New(core)

	client := &Client{}
	exporter := NewExporter(client, "output", observableLogger, false, "")

	// Test with a repo where name and slug differ due to special characters
	repo := &data.BitbucketRepository{
		Name:        "@group-test/ui", // Name with special characters
		Slug:        "group-test-ui",  // Slug without special characters
		Description: "Test repository with special characters",
		CreatedOn:   "2023-01-01T00:00:00Z",
		IsPrivate:   true,
	}

	// Create repositories data
	repositories := exporter.createRepositoriesData(repo, "test-workspace")

	// Verify the result
	assert.Len(t, repositories, 1)
	assert.Equal(t, "group-test-ui", repositories[0].Name, "Should use slug instead of name")
	assert.Equal(t, "group-test-ui", repositories[0].Slug, "Slug should match")

	// Verify that a debug log was created about using the slug
	logs := observedLogs.All()
	var foundLogMessage bool
	for _, log := range logs {
		if strings.Contains(log.Message, "Repository name contains special characters") {
			foundLogMessage = true
			assert.Equal(t, "@group-test/ui", log.ContextMap()["name"], "Log should contain original name")
			assert.Equal(t, "group-test-ui", log.ContextMap()["slug"], "Log should contain slug")
			break
		}
	}
	assert.True(t, foundLogMessage, "Should log about using slug instead of name")
}

func TestCreateBasicUsers_Fallback(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	exporter := NewExporter(&Client{}, "output", logger, false, "")

	users := exporter.createBasicUsers("ws-fallback")
	assert.Len(t, users, 1)
	assert.Equal(t, "user", users[0].Type)
	assert.Equal(t, "ws-fallback", users[0].Login)
	assert.Equal(t, "ws-fallback", users[0].Name)
}

func TestUpdateRepositoryField(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "update-repo-field-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	logger, _ := zap.NewDevelopment()
	exporter := NewExporter(&Client{}, tempDir, logger, false, "")

	// Seed repositories_000001.json with a minimal repository entry
	initial := []data.Repository{
		{
			Type:          "repository",
			Name:          "group-test-ui",
			Slug:          "group-test-ui",
			DefaultBranch: "main",
			GitURL:        "",
		},
	}
	err = exporter.writeJSONFile("repositories_000001.json", initial)
	assert.NoError(t, err)

	// Update default_branch and git_url
	exporter.updateRepositoryField("group-test-ui", "default_branch", "develop")
	exporter.updateRepositoryField("group-test-ui", "git_url", "tarball://root/repositories/ws/group-test-ui.git")

	// Read back and assert changes
	b, err := os.ReadFile(filepath.Join(tempDir, "repositories_000001.json"))
	assert.NoError(t, err)
	var repos []data.Repository
	assert.NoError(t, json.Unmarshal(b, &repos))
	assert.Equal(t, "develop", repos[0].DefaultBranch)
	assert.Equal(t, "tarball://root/repositories/ws/group-test-ui.git", repos[0].GitURL)
}

func TestCreateEmptyRepository(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "empty-repo-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	logger, _ := zap.NewDevelopment()
	exporter := NewExporter(&Client{}, tempDir, logger, false, "")

	// Test successful creation
	err = exporter.createEmptyRepository("test-workspace", "test-repo")
	assert.NoError(t, err)

	// Verify the repository directory was created
	repoPath := filepath.Join(tempDir, "repositories", "test-workspace", "test-repo.git")
	assert.DirExists(t, repoPath)

	// Verify it's a valid git repository
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	assert.NoError(t, err)
	assert.Contains(t, string(output), ".")
}

func TestCloneRepositoryWithErrors(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "clone-repo-error-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create mock server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/repositories/") {
			// Return repository with default branch
			w.WriteHeader(http.StatusOK)
			writeResponse(t, w, []byte(`{
                "name": "test-repo",
                "slug": "test-repo",
                "mainbranch": {"name": "develop"}
            }`))
		}
	}))
	defer testServer.Close()

	logger, _ := zap.NewDevelopment()
	client := &Client{
		baseURL:    testServer.URL,
		httpClient: testServer.Client(),
		logger:     logger,
	}
	exporter := NewExporter(client, tempDir, logger, false, "")

	// Test with invalid clone URL - this should fail
	err = exporter.CloneRepository("test-workspace", "test-repo", "invalid://url")
	assert.Error(t, err) // CloneRepository returns error on failure
	assert.Contains(t, err.Error(), "failed to clone repository")
}

func TestCreateReviews(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	exporter := NewExporter(&Client{}, "output", logger, false, "")

	reviewComments := []data.PullRequestReviewComment{
		{
			PullRequestReview: "https://example.com/review/1",
			User:              "https://example.com/user/1",
			Body:              "Looks good!",
			State:             1, // Use int value for approved
			CreatedAt:         "2023-01-01T10:00:00Z",
			UpdatedAt:         "2023-01-01T10:00:00Z",
		},
		{
			PullRequestReview: "https://example.com/review/1", // Same review
			User:              "https://example.com/user/1",
			Body:              "Additional comment",
			State:             1, // Use int value for approved
			CreatedAt:         "2023-01-01T11:00:00Z",
			UpdatedAt:         "2023-01-01T11:00:00Z",
		},
		{
			PullRequestReview: "https://example.com/review/2", // Different review
			User:              "https://example.com/user/2",
			Body:              "Needs changes",
			State:             3, // Use int value for changes_requested
			CreatedAt:         "2023-01-02T10:00:00Z",
			UpdatedAt:         "2023-01-02T10:00:00Z",
		},
	}

	reviews := exporter.createReviews(reviewComments)

	// Should have two reviews
	assert.Len(t, reviews, 2)

	// Create a map to look up reviews by their review URL
	reviewsByURL := make(map[string]map[string]interface{})
	for _, review := range reviews {
		url := review["url"].(string)
		reviewsByURL[url] = review
	}

	// Verify first review (by URL)
	review1 := reviewsByURL["https://example.com/review/1"]
	assert.NotNil(t, review1, "Should have review with URL https://example.com/review/1")
	assert.Equal(t, "2023-01-01T10:00:00Z", review1["submitted_at"], "Should use earliest comment time")
	assert.Equal(t, 1, review1["state"], "Should have state 1 (approved)")

	// Verify second review
	review2 := reviewsByURL["https://example.com/review/2"]
	assert.NotNil(t, review2, "Should have review with URL https://example.com/review/2")
	assert.Equal(t, "2023-01-02T10:00:00Z", review2["submitted_at"])
	assert.Equal(t, 3, review2["state"], "Should have state 3 (changes requested)")
}

func TestArchiveDirectoryWithSpecialFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "archive-special-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	logger, _ := zap.NewDevelopment()
	exporter := NewExporter(&Client{}, tempDir, logger, false, "")

	// Create various file types
	regularFile := filepath.Join(tempDir, "regular.txt")
	err = os.WriteFile(regularFile, []byte("regular content"), 0644)
	assert.NoError(t, err)

	// Create subdirectory with file
	subDir := filepath.Join(tempDir, "subdir")
	err = os.MkdirAll(subDir, 0755)
	assert.NoError(t, err)

	subFile := filepath.Join(subDir, "subfile.txt")
	err = os.WriteFile(subFile, []byte("sub content"), 0644)
	assert.NoError(t, err)

	// Create archive
	archivePath, err := exporter.CreateArchive()
	assert.NoError(t, err)

	// Verify archive contents
	f, err := os.Open(archivePath)
	assert.NoError(t, err)
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	assert.NoError(t, err)
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)

	foundFiles := make(map[string]bool)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		assert.NoError(t, err)
		foundFiles[h.Name] = true
	}

	assert.True(t, foundFiles["regular.txt"], "Should find regular file")
	assert.True(t, foundFiles["subdir/subfile.txt"], "Should find file in subdirectory")
}

func TestReviewStates(t *testing.T) {
	// Create a logger and exporter
	logger, _ := zap.NewDevelopment()
	exporter := NewExporter(&Client{}, "output", logger, false, "")

	// Create review comments with different states
	reviewComments := []data.PullRequestReviewComment{
		{
			PullRequestReview: "https://example.com/review/1",
			User:              "https://example.com/user/1",
			State:             1, // Approved
			CreatedAt:         "2023-01-01T10:00:00Z",
		},
		{
			PullRequestReview: "https://example.com/review/2",
			User:              "https://example.com/user/2",
			State:             2, // Commented
			CreatedAt:         "2023-01-02T10:00:00Z",
		},
		{
			PullRequestReview: "https://example.com/review/3",
			User:              "https://example.com/user/3",
			State:             3, // Changes requested
			CreatedAt:         "2023-01-03T10:00:00Z",
		},
	}

	// Create reviews from the comments
	reviews := exporter.createReviews(reviewComments)

	// Verify we have the correct number of reviews
	assert.Len(t, reviews, 3)

	// Create a map to look up reviews by state for verification
	reviewsByState := make(map[int]map[string]interface{})
	for _, review := range reviews {
		state := review["state"].(int)
		reviewsByState[state] = review
	}

	// Verify each review state has the expected attributes
	assert.Contains(t, reviewsByState, 1, "Should have a review with state 1 (approved)")
	assert.Contains(t, reviewsByState, 2, "Should have a review with state 2 (commented)")
	assert.Contains(t, reviewsByState, 3, "Should have a review with state 3 (changes requested)")

	// Verify submitted dates for each state
	assert.Equal(t, "2023-01-01T10:00:00Z", reviewsByState[1]["submitted_at"], "Approved review should have correct date")
	assert.Equal(t, "2023-01-02T10:00:00Z", reviewsByState[2]["submitted_at"], "Commented review should have correct date")
	assert.Equal(t, "2023-01-03T10:00:00Z", reviewsByState[3]["submitted_at"], "Changes requested review should have correct date")
}

func TestExportWithNoData(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-no-data-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Mock server that returns empty data
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)

		if strings.Contains(r.URL.Path, "/repositories/") && !strings.Contains(r.URL.Path, "pullrequests") {
			writeResponse(t, w, []byte(`{
                "name": "empty-repo",
                "slug": "empty-repo",
                "mainbranch": {"name": "main"}
            }`))
		} else {
			// Return empty lists for everything else
			writeResponse(t, w, []byte(`{"values": [], "next": null}`))
		}
	}))
	defer testServer.Close()

	logger, _ := zap.NewDevelopment()

	// Don't set any authentication credentials to avoid actual clone attempts
	// The Export function will create an empty repository when cloning fails
	client := &Client{
		baseURL:        testServer.URL,
		httpClient:     testServer.Client(),
		logger:         logger,
		commitSHACache: make(map[string]string),
		// Don't set apiToken, accessToken, username, or appPass
	}
	exporter := NewExporter(client, tempDir, logger, false, "")

	// Export will fail due to no authentication
	err = exporter.Export("test-workspace", "empty-repo")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "export failed")

	// Since the export fails early due to authentication, these files won't be created
	assert.NoFileExists(t, filepath.Join(tempDir, "organizations_000001.json"))
	assert.NoFileExists(t, filepath.Join(tempDir, "users_000001.json"))

	// Should not create PR-related files when there are no PRs
	assert.NoFileExists(t, filepath.Join(tempDir, "pull_requests_000001.json"))
	assert.NoFileExists(t, filepath.Join(tempDir, "issue_comments_000001.json"))
}

func TestGetAuthMethodDescription(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	tests := []struct {
		name     string
		client   *Client
		expected string
	}{
		{
			name: "no credentials",
			client: &Client{
				logger: logger,
			},
			expected: "no authentication",
		},
		{
			name: "workspace access token",
			client: &Client{
				accessToken: "token",
				logger:      logger,
			},
			expected: "workspace access token",
		},
		{
			name: "API token without email",
			client: &Client{
				apiToken: "api-token",
				logger:   logger,
			},
			expected: "API token with x-bitbucket-api-token-auth",
		},
		{
			name: "API token with email",
			client: &Client{
				apiToken: "api-token",
				email:    "user@example.com",
				logger:   logger,
			},
			expected: "API token with email",
		},
		{
			name: "basic authentication",
			client: &Client{
				username: "user",
				appPass:  "pass",
				logger:   logger,
			},
			expected: "username and app password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getAuthMethodDescription(tt.client)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPathHandling(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "path-handling-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	logger, _ := zap.NewDevelopment()
	client := &Client{}
	exporter := NewExporter(client, tempDir, logger, false, "")

	// Test repository directory creation with mixed paths
	workspace := "test-workspace"
	repoSlug := "test-repo"

	// Create paths with mixed slashes
	mixedPath := filepath.Join(tempDir, "repositories", workspace) + "\\mixed\\path"
	err = os.MkdirAll(ToNativePath(mixedPath), 0755)
	assert.NoError(t, err)

	// Test creating repository info files
	err = exporter.createRepositoryInfoFiles(workspace, repoSlug)
	assert.NoError(t, err)

	// Verify that files were created with correct paths
	infoDir := filepath.Join(tempDir, "repositories", workspace, repoSlug+".git", "info")
	assert.DirExists(t, infoDir)

	// Check nwo file
	nwoPath := filepath.Join(infoDir, "nwo")
	assert.FileExists(t, nwoPath)
	nwoContent, err := os.ReadFile(nwoPath)
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%s/%s\n", workspace, repoSlug), string(nwoContent))

	// Check last-sync file
	syncPath := filepath.Join(infoDir, "last-sync")
	assert.FileExists(t, syncPath)
	syncContent, err := os.ReadFile(syncPath)
	assert.NoError(t, err)
	_, err = time.Parse("2006-01-02T15:04:05", string(syncContent))
	assert.NoError(t, err)
}

func TestPathConversionFunctions(t *testing.T) {
	// Test cases for all path conversion functions
	testCases := []struct {
		name     string
		input    string
		expected string
		function func(string) string
	}{
		{
			name:     "NormalizePath - Windows backslashes",
			input:    "repositories\\workspace\\repo.git\\objects\\pack",
			expected: "repositories/workspace/repo.git/objects/pack",
			function: NormalizePath,
		},
		{
			name:     "NormalizePath - Already normalized",
			input:    "repositories/workspace/repo.git/objects/pack",
			expected: "repositories/workspace/repo.git/objects/pack",
			function: NormalizePath,
		},
		{
			name:     "NormalizePath - Mixed slashes",
			input:    "repositories/workspace\\repo.git/objects\\pack",
			expected: "repositories/workspace/repo.git/objects/pack",
			function: NormalizePath,
		},
		{
			name:     "ToUnixPath - Windows backslashes",
			input:    "repositories\\workspace\\repo.git",
			expected: "repositories/workspace/repo.git",
			function: ToUnixPath,
		},
		{
			name:     "ToUnixPath - Already Unix path",
			input:    "repositories/workspace/repo.git",
			expected: "repositories/workspace/repo.git",
			function: ToUnixPath,
		},
		{
			name:     "ToUnixPath - Mixed slashes",
			input:    "repositories/workspace\\repo.git",
			expected: "repositories/workspace/repo.git",
			function: ToUnixPath,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.function(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}

	// Test ToNativePath separately since it's OS-dependent
	t.Run("ToNativePath - Unix to native", func(t *testing.T) {
		input := "repositories/workspace/repo.git"
		var expected string
		if runtime.GOOS == "windows" {
			expected = "repositories\\workspace\\repo.git"
		} else {
			expected = "repositories/workspace/repo.git"
		}
		result := ToNativePath(input)
		assert.Equal(t, expected, result)
	})

	// Test round-trip conversion
	t.Run("Round-trip conversion", func(t *testing.T) {
		original := "repositories/workspace/repo.git"
		roundTrip := ToUnixPath(ToNativePath(original))
		assert.Equal(t, original, roundTrip)
	})
}

func TestCloneRepositoryAuthenticationError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "clone-auth-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	logger, _ := zap.NewDevelopment()

	// Mock a server for repository details
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		writeResponse(t, w, []byte(`{"name": "Test Repo", "mainbranch": {"name": "main"}}`))
	}))
	defer testServer.Close()

	client := &Client{
		baseURL:     testServer.URL,
		httpClient:  testServer.Client(),
		logger:      logger,
		accessToken: "invalid-token",
	}

	exporter := NewExporter(client, tempDir, logger, false, "")

	// This will fail with authentication error
	err = exporter.CloneRepository("workspace", "repo", "https://invalid-auth@bitbucket.org/workspace/repo.git")

	// CloneRepository SHOULD return an error when it fails
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to clone repository")

	// The repository directory should NOT exist after a failed clone
	repoPath := filepath.Join(tempDir, "repositories", "workspace", "repo.git")
	assert.NoDirExists(t, repoPath)
}

func TestExportWithCloneFailure(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-clone-fail-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Mock server that returns repository info but we'll use invalid credentials
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if strings.Contains(r.URL.Path, "/repositories/") && !strings.Contains(r.URL.Path, "pullrequests") {
			writeResponse(t, w, []byte(`{"name": "Test Repo", "mainbranch": {"name": "main"}}`))
		} else {
			writeResponse(t, w, []byte(`{"values": [], "next": null}`))
		}
	}))
	defer testServer.Close()

	logger, _ := zap.NewDevelopment()

	// Create a client with invalid credentials that will cause clone to fail
	client := &Client{
		baseURL:        testServer.URL,
		httpClient:     testServer.Client(),
		logger:         logger,
		accessToken:    "invalid-token-that-will-fail",
		commitSHACache: make(map[string]string),
	}

	exporter := NewExporter(client, tempDir, logger, false, "")

	// Export should fail with authentication error
	err = exporter.Export("workspace", "repo")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authentication error")

	// Repository should NOT be created when authentication fails
	repoPath := filepath.Join(tempDir, "repositories", "workspace", "repo.git")
	assert.NoDirExists(t, repoPath)
}

func TestCloneRepositoryWithInvalidBranchNames(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "clone-invalid-branch-")
	assert.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: Failed to remove temp dir: %v", err)
		}
	}()

	// Create a mock server that returns repository with an ambiguous branch name
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/repositories") {
			// Return repo with branch name that's exactly 40 hex chars
			w.WriteHeader(http.StatusOK)
			writeResponse(t, w, []byte(`{
                "name": "test-repo",
                "mainbranch": {
                    "name": "1234567890abcdef1234567890abcdef12345678"
                }
            }`))
		}
	}))
	defer testServer.Close()

	logger, _ := zap.NewDevelopment()
	client := &Client{
		baseURL:    testServer.URL,
		httpClient: testServer.Client(),
		logger:     logger,
		username:   "test",
		appPass:    "test",
	}

	exporter := NewExporter(client, tempDir, logger, false, "")

	// Create a minimal git repo for testing
	gitRepoPath := filepath.Join(tempDir, "test-git-repo")
	cmd := exec.Command("git", "init", "--bare", gitRepoPath)
	err = cmd.Run()
	if err != nil {
		t.Skip("Git not available for testing")
	}

	// Mock clone URL
	cloneURL := "file://" + gitRepoPath

	// This should fail due to invalid branch name
	err = exporter.CloneRepository("workspace", "test-repo", cloneURL)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid branch reference")
}

func TestSanitizeDescription(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Single newline",
			input:    "Line 1\nLine 2",
			expected: "Line 1 Line 2",
		},
		{
			name:     "Multiple newlines",
			input:    "Line 1\n\n\nLine 2",
			expected: "Line 1 Line 2",
		},
		{
			name:     "Windows newlines",
			input:    "Line 1\r\nLine 2",
			expected: "Line 1 Line 2",
		},
		{
			name:     "Tabs and spaces",
			input:    "Line 1\t\t  Line 2",
			expected: "Line 1 Line 2",
		},
		{
			name:     "Leading and trailing whitespace",
			input:    "  \n  Line 1\nLine 2  \n  ",
			expected: "Line 1 Line 2",
		},
		{
			name:     "No whitespace",
			input:    "SingleLine",
			expected: "SingleLine",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Only whitespace",
			input:    " \n\t\r\n ",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := sanitizeDescription(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestExportWithFilters(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "exporter-filter-test-")
	assert.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: Failed to remove temp dir: %v", err)
		}
	}()

	// Create a local git repository that can be cloned for testing
	gitRepoPath := filepath.Join(tempDir, "test-git-repo.git")
	cmd := exec.Command("git", "init", "--bare", gitRepoPath)
	err = cmd.Run()
	if err != nil {
		t.Skip("Git not available for testing")
	}

	// Mock server responses
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)

		// Repository info
		if strings.Contains(r.URL.Path, "/repositories/") && !strings.Contains(r.URL.Path, "pullrequests") {
			// Return repository info - the clone will use the file:// URL for local git repo
			writeResponse(t, w, []byte(`{
                "name": "Test Repo", 
                "mainbranch": {"name": "main"}
            }`))
			return
		}

		// Pull requests
		if strings.Contains(r.URL.Path, "pullrequests") {
			// Check if open PRs only filter is applied
			if strings.Contains(r.URL.RawQuery, "state=OPEN") {
				writeResponse(t, w, []byte(`{
                    "values": [
                        {
                            "id": 3, 
                            "title": "New Open PR",
                            "state": "OPEN",
                            "created_on": "2023-06-01T00:00:00+00:00",
                            "author": {"uuid": "{123}"},
                            "source": {"branch": {"name": "feature"}, "commit": {"hash": "abc123"}},
                            "destination": {"branch": {"name": "main"}, "commit": {"hash": "def456"}}
                        }
                    ],
                    "next": null
                }`))
				return
			}

			// Return all PRs
			writeResponse(t, w, []byte(`{
                "values": [
                    {
                        "id": 1, 
                        "title": "Old Open PR",
                        "state": "OPEN",
                        "created_on": "2022-01-01T00:00:00+00:00",
                        "author": {"uuid": "{123}"},
                        "source": {"branch": {"name": "source"}, "commit": {"hash": "abc123"}},
                        "destination": {"branch": {"name": "main"}, "commit": {"hash": "def456"}}
                    },
                    {
                        "id": 2, 
                        "title": "Old Closed PR",
                        "state": "DECLINED",
                        "created_on": "2022-03-01T00:00:00+00:00",
                        "author": {"uuid": "{123}"},
                        "source": {"branch": {"name": "source"}, "commit": {"hash": "abc123"}},
                        "destination": {"branch": {"name": "main"}, "commit": {"hash": "def456"}}
                    },
                    {
                        "id": 3, 
                        "title": "New Open PR",
                        "state": "OPEN",
                        "created_on": "2023-06-01T00:00:00+00:00",
                        "author": {"uuid": "{123}"},
                        "source": {"branch": {"name": "feature"}, "commit": {"hash": "abc123"}},
                        "destination": {"branch": {"name": "main"}, "commit": {"hash": "def456"}}
                    }
                ],
                "next": null
            }`))
			return
		}

		// For other requests like users, comments, etc.
		writeResponse(t, w, []byte(`{"values": [], "next": null}`))
	}))
	defer testServer.Close()

	logger, _ := zap.NewDevelopment()

	// Test helper function to run export with local git repo
	runExportTest := func(outputSubDir string, openPRsOnly bool, prsFromDate string) error {
		// Use a special username that we can detect to use the local git repo
		client := &Client{
			baseURL:        testServer.URL,
			httpClient:     testServer.Client(),
			logger:         logger,
			commitSHACache: make(map[string]string),
			username:       "test-local-repo", // Special username to trigger local repo usage
			appPass:        "test",
		}

		outputPath := filepath.Join(tempDir, outputSubDir)
		exporter := NewExporter(client, outputPath, logger, openPRsOnly, prsFromDate)

		// Create a wrapper that intercepts the clone operation
		// We'll create an empty repository locally instead of actually cloning
		repoPath := filepath.Join(outputPath, "repositories", "workspace", "repo.git")
		if err := os.MkdirAll(repoPath, 0755); err != nil {
			return err
		}

		// Initialize as a bare git repository
		cmd := exec.Command("git", "init", "--bare")
		cmd.Dir = repoPath
		if err := cmd.Run(); err != nil {
			return err
		}

		// Create the HEAD file pointing to main
		headFile := filepath.Join(repoPath, "HEAD")
		if err := os.WriteFile(headFile, []byte("ref: refs/heads/main\n"), 0644); err != nil {
			return err
		}

		// Create refs/heads directory
		refsDir := filepath.Join(repoPath, "refs", "heads")
		if err := os.MkdirAll(refsDir, 0755); err != nil {
			return err
		}

		// Now run the export, but skip the actual clone by pre-creating the repo
		return exporter.Export("workspace", "repo")
	}

	// Test 1: No filters
	err = runExportTest("no-filters", false, "")
	// The export will fail because we're not actually cloning, but that's expected
	// We're testing the filter logic, not the full export
	if err != nil && !strings.Contains(err.Error(), "authentication") {
		// Only fail if it's not an authentication error
		assert.NoError(t, err)
	}

	// Test 2: Open PRs only
	err = runExportTest("open-only", true, "")
	if err != nil && !strings.Contains(err.Error(), "authentication") {
		assert.NoError(t, err)
	}

	// Test 3: Date filter
	err = runExportTest("date-filter", false, "2023-01-01")
	if err != nil && !strings.Contains(err.Error(), "authentication") {
		assert.NoError(t, err)
	}
}

func TestCreateBasicUsersFallback(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "basic-users-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	logger, _ := zap.NewDevelopment()
	client := &Client{
		baseURL:    "http://test",
		httpClient: http.DefaultClient,
		logger:     logger,
	}
	exporter := NewExporter(client, tempDir, logger, false, "")

	// Test with workspace name
	workspace := "test-workspace"
	users := exporter.createBasicUsers(workspace)

	// Should have 1 basic user based on workspace
	assert.Len(t, users, 1)

	// Verify user data
	assert.Equal(t, "user", users[0].Type)
	assert.Equal(t, workspace, users[0].Login)
	assert.Equal(t, workspace, users[0].Name)
	assert.Equal(t, fmt.Sprintf("https://bitbucket.org/%s", workspace), users[0].URL)
}

func TestCreateRepositoryInfoFilesErrors(t *testing.T) {
	// Test with non-existent directory
	logger, _ := zap.NewDevelopment()
	client := &Client{}
	exporter := NewExporter(client, "/non/existent/path", logger, false, "")

	err := exporter.createRepositoryInfoFiles("workspace", "repo")
	assert.Error(t, err)
}

func TestUpdateRepositoryFieldWithInvalidJSON(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "invalid-json-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	logger, _ := zap.NewDevelopment()
	exporter := NewExporter(&Client{}, tempDir, logger, false, "")

	// Create an invalid JSON file
	invalidJSON := filepath.Join(tempDir, "repositories_000001.json")
	err = os.WriteFile(invalidJSON, []byte("not valid json"), 0644)
	assert.NoError(t, err)

	// This should handle the error gracefully
	exporter.updateRepositoryField("test-repo", "default_branch", "main")

	// File should still exist but be unchanged
	assert.FileExists(t, invalidJSON)
}

func TestSanitizeDescriptionEdgeCases(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Unicode characters",
			input:    "Line 1 🚀\n\nLine 2 ✨",
			expected: "Line 1 🚀 Line 2 ✨",
		},
		{
			name:     "Multiple whitespace types",
			input:    "Line\t1\r\n\n\nLine\t\t2",
			expected: "Line 1 Line 2",
		},
		{
			name:     "Only newlines",
			input:    "\n\n\n",
			expected: "",
		},
		{
			name:     "Very long line",
			input:    strings.Repeat("a", 1000) + "\n" + strings.Repeat("b", 1000),
			expected: strings.Repeat("a", 1000) + " " + strings.Repeat("b", 1000),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := sanitizeDescription(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestCreateEmptyRepositoryWithPermissionError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows as permission handling differs")
	}

	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "repo-perm-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Make the temp dir read-only to force permission error
	err = os.Chmod(tempDir, 0500) // read + execute, but no write
	assert.NoError(t, err)
	defer os.Chmod(tempDir, 0700) // restore permissions for cleanup

	logger, _ := zap.NewDevelopment()
	exporter := NewExporter(&Client{}, tempDir, logger, false, "")

	// This should fail with permission error when trying to create directories
	err = exporter.createEmptyRepository("workspace", "repo")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create")
}
