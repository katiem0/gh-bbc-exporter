package utils

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/katiem0/gh-bbc-exporter/internal/data"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
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
	}
	exporter := NewExporter(client, tempDir, logger, false, "")

	err = exporter.Export("workspace", "repo")
	assert.NoError(t, err)
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

func TestExportWithFilters(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "exporter-filter-test-")
	assert.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: Failed to remove temp dir: %v", err)
		}
	}()

	// Mock server responses
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)

		// Repository info
		if strings.Contains(r.URL.Path, "/repositories/") && !strings.Contains(r.URL.Path, "pullrequests") {
			writeResponse(t, w, []byte(`{"name": "Test Repo", "mainbranch": {"name": "main"}}`))
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

	// Test 1: No filters
	client := &Client{
		baseURL:        testServer.URL,
		httpClient:     testServer.Client(),
		logger:         logger,
		commitSHACache: make(map[string]string),
	}
	exporter := NewExporter(client, tempDir+"/no-filters", logger, false, "")

	err = exporter.Export("workspace", "repo")
	assert.NoError(t, err)

	// Test 2: Open PRs only
	client = &Client{
		baseURL:        testServer.URL,
		httpClient:     testServer.Client(),
		logger:         logger,
		commitSHACache: make(map[string]string),
	}
	exporter = NewExporter(client, tempDir+"/open-only", logger, true, "")

	err = exporter.Export("workspace", "repo")
	assert.NoError(t, err)

	// Test 3: Date filter
	client = &Client{
		baseURL:        testServer.URL,
		httpClient:     testServer.Client(),
		logger:         logger,
		commitSHACache: make(map[string]string),
	}
	exporter = NewExporter(client, tempDir+"/date-filter", logger, false, "2023-01-01")

	err = exporter.Export("workspace", "repo")
	assert.NoError(t, err)
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

func TestExportErrorPaths(t *testing.T) {
	// Test when output directory doesn't exist
	client := &Client{}
	logger, _ := zap.NewDevelopment()
	exporter := NewExporter(client, "/nonexistent/path", logger, false, "")

	err := exporter.Export("workspace", "repo")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "output directory")

	// Test when workspace/repo parameter validation fails
	exporter = NewExporter(client, ".", logger, false, "")
	err = exporter.Export("", "repo")
	assert.Error(t, err)
	// More assertions
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
