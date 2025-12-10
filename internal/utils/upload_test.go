package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestUploadArchiveToGitHub(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "upload-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	archivePath := filepath.Join(tempDir, "test-archive.tar.gz")
	err = os.WriteFile(archivePath, []byte("test archive content"), 0644)
	assert.NoError(t, err)

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer")
		assert.Equal(t, "application/octet-stream", r.Header.Get("Content-Type"))

		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.NotEmpty(t, body)

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uri": "gei://archive/test-archive-id"}`))
	}))
	defer testServer.Close()

	_ = os.Setenv("GITHUB_PAT", "test-token")
	defer func() { _ = os.Unsetenv("GITHUB_PAT") }()

	logger, _ := zap.NewDevelopment()

	gqlClient := &api.GraphQLClient{}
	restClient := &api.RESTClient{}
	apiGetter := NewAPIGetter(gqlClient, restClient)

	oldUploadsBaseURL := uploadsBaseURL
	uploadsBaseURL = testServer.URL + "/organizations/%d/gei/archive"
	defer func() { uploadsBaseURL = oldUploadsBaseURL }()

	uri, err := apiGetter.UploadArchiveToGitHub(12345, archivePath, logger)

	assert.NoError(t, err)
	assert.Equal(t, "gei://archive/test-archive-id", uri)
}

func TestUploadArchiveFileNotFound(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	gqlClient := &api.GraphQLClient{}
	restClient := &api.RESTClient{}
	apiGetter := NewAPIGetter(gqlClient, restClient)

	_, err := apiGetter.UploadArchiveToGitHub(12345, "/nonexistent/path/archive.tar.gz", logger)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open archive file")
}

func TestUploadSingleFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "upload-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	archivePath := filepath.Join(tempDir, "small-archive.tar.gz")
	err = os.WriteFile(archivePath, []byte("small content"), 0644)
	assert.NoError(t, err)

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
            "guid": "test-guid",
            "uri": "gei://archive/test-archive-id",
            "name": "small-archive.tar.gz",
            "size": 13
        }`))
	}))
	defer testServer.Close()

	_ = os.Setenv("GITHUB_PAT", "test-token")
	defer func() { _ = os.Unsetenv("GITHUB_PAT") }()

	logger, _ := zap.NewDevelopment()

	gqlClient := &api.GraphQLClient{}
	restClient := &api.RESTClient{}
	apiGetter := NewAPIGetter(gqlClient, restClient)

	file, err := os.Open(archivePath)
	assert.NoError(t, err)
	defer func() { _ = file.Close() }()

	fileInfo, err := file.Stat()
	assert.NoError(t, err)

	oldUploadsBaseURL := uploadsBaseURL
	uploadsBaseURL = testServer.URL + "/organizations/%d/gei/archive"
	defer func() { uploadsBaseURL = oldUploadsBaseURL }()

	ctx := context.Background()
	uri, err := apiGetter.uploadSingleFile(ctx, 12345, file, "small-archive.tar.gz", fileInfo.Size(), logger)

	assert.NoError(t, err)
	assert.Equal(t, "gei://archive/test-archive-id", uri)
}

func TestUploadMultipartFileIntegration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "upload-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	archivePath := filepath.Join(tempDir, "large-archive.tar.gz")
	testContent := []byte("test archive content for multipart upload")
	err = os.WriteFile(archivePath, testContent, 0644)
	assert.NoError(t, err)

	partNumber := 0
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			if strings.Contains(r.URL.Path, "/blobs/uploads") {
				w.Header().Set("Location", "/organizations/12345/gei/archive/blobs/uploads?part_number=1&guid=test-guid-123&upload_id=test-upload-456")
				w.WriteHeader(http.StatusAccepted)
				return
			}
		case "PATCH":
			partNumber++
			_, _ = io.ReadAll(r.Body)
			w.Header().Set("Location", fmt.Sprintf("/organizations/12345/gei/archive/blobs/uploads?part_number=%d&guid=test-guid-123&upload_id=test-upload-456", partNumber+1))
			w.WriteHeader(http.StatusAccepted)
			return
		case "PUT":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
                "guid": "test-guid-123",
                "node_id": "test-node-id",
                "name": "large-archive.tar.gz",
                "size": 41,
                "uri": "gei://archive/test-multipart-archive-id",
                "created_at": "2023-01-01T00:00:00Z"
            }`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer testServer.Close()

	_ = os.Setenv("GITHUB_PAT", "test-token")
	defer func() { _ = os.Unsetenv("GITHUB_PAT") }()

	logger, _ := zap.NewDevelopment()

	gqlClient := &api.GraphQLClient{}
	restClient := &api.RESTClient{}
	apiGetter := NewAPIGetter(gqlClient, restClient)

	oldUploadsBaseURL := uploadsBaseURL
	oldUploadsHost := uploadsHost
	uploadsBaseURL = testServer.URL + "/organizations/%d/gei/archive"
	uploadsHost = testServer.URL
	defer func() {
		uploadsBaseURL = oldUploadsBaseURL
		uploadsHost = oldUploadsHost
	}()

	oldThreshold := DefaultMultipartThreshold
	oldPartSize := DefaultPartSize
	DefaultMultipartThreshold = 1
	DefaultPartSize = 10
	defer func() {
		DefaultMultipartThreshold = oldThreshold
		DefaultPartSize = oldPartSize
	}()

	uri, err := apiGetter.UploadArchiveToGitHub(12345, archivePath, logger)

	assert.NoError(t, err)
	assert.Equal(t, "gei://archive/test-multipart-archive-id", uri)
	assert.Greater(t, partNumber, 0, "Should have uploaded at least one part")
}

func TestStartMultipartUpload(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/blobs/uploads")

		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)

		var reqBody map[string]interface{}
		err = json.Unmarshal(body, &reqBody)
		assert.NoError(t, err)
		assert.Equal(t, "application/octet-stream", reqBody["content_type"])
		assert.Equal(t, "test-file.tar.gz", reqBody["name"])

		w.Header().Set("Location", "/organizations/12345/gei/archive/blobs/uploads?part_number=1&guid=test-guid-123&upload_id=test-upload-456")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer testServer.Close()

	_ = os.Setenv("GITHUB_PAT", "test-token")
	defer func() { _ = os.Unsetenv("GITHUB_PAT") }()

	logger, _ := zap.NewDevelopment()

	gqlClient := &api.GraphQLClient{}
	restClient := &api.RESTClient{}
	apiGetter := NewAPIGetter(gqlClient, restClient)

	oldUploadsBaseURL := uploadsBaseURL
	uploadsBaseURL = testServer.URL + "/organizations/%d/gei/archive"
	defer func() { uploadsBaseURL = oldUploadsBaseURL }()

	ctx := context.Background()
	guid, uploadID, location, err := apiGetter.startMultipartUpload(ctx, 12345, "test-file.tar.gz", 1000000, logger)

	assert.NoError(t, err)
	assert.Equal(t, "test-guid-123", guid)
	assert.Equal(t, "test-upload-456", uploadID)
	assert.Contains(t, location, "part_number=1")
}

func TestUploadPartIntegration(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)

		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.Equal(t, []byte("test part data"), body)

		w.Header().Set("Location", "/organizations/12345/gei/archive/blobs/uploads?part_number=2&guid=test-guid-123&upload_id=test-upload-456")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer testServer.Close()

	_ = os.Setenv("GITHUB_PAT", "test-token")
	defer func() { _ = os.Unsetenv("GITHUB_PAT") }()

	logger, _ := zap.NewDevelopment()

	gqlClient := &api.GraphQLClient{}
	restClient := &api.RESTClient{}
	apiGetter := NewAPIGetter(gqlClient, restClient)

	// Override the uploads host for testing
	oldUploadsHost := uploadsHost
	uploadsHost = testServer.URL
	defer func() { uploadsHost = oldUploadsHost }()

	ctx := context.Background()
	location := "/organizations/12345/gei/archive/blobs/uploads?part_number=1&guid=test-guid-123&upload_id=test-upload-456"
	nextLocation, err := apiGetter.uploadPart(ctx, location, []byte("test part data"), 1, logger)

	assert.NoError(t, err)
	assert.Contains(t, nextLocation, "part_number=2")
}

func TestCompleteMultipartUploadIntegration(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
            "guid": "test-guid-123",
            "node_id": "test-node-id",
            "name": "test-archive.tar.gz",
            "size": 1000000,
            "uri": "gei://archive/test-archive-id",
            "created_at": "2023-01-01T00:00:00Z"
        }`))
	}))
	defer testServer.Close()

	_ = os.Setenv("GITHUB_PAT", "test-token")
	defer func() { _ = os.Unsetenv("GITHUB_PAT") }()

	logger, _ := zap.NewDevelopment()

	gqlClient := &api.GraphQLClient{}
	restClient := &api.RESTClient{}
	apiGetter := NewAPIGetter(gqlClient, restClient)

	// Override the uploads host for testing
	oldUploadsHost := uploadsHost
	uploadsHost = testServer.URL
	defer func() { uploadsHost = oldUploadsHost }()

	ctx := context.Background()
	lastLocation := "/organizations/12345/gei/archive/blobs/uploads?part_number=3&guid=test-guid-123&upload_id=test-upload-456"
	uri, err := apiGetter.completeMultipartUpload(ctx, lastLocation, logger)

	assert.NoError(t, err)
	assert.Equal(t, "gei://archive/test-archive-id", uri)
}

func TestUploadArchiveServerError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "upload-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	archivePath := filepath.Join(tempDir, "test-archive.tar.gz")
	err = os.WriteFile(archivePath, []byte("test archive content"), 0644)
	assert.NoError(t, err)

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer testServer.Close()

	_ = os.Setenv("GITHUB_PAT", "test-token")
	defer func() { _ = os.Unsetenv("GITHUB_PAT") }()

	logger, _ := zap.NewDevelopment()

	gqlClient := &api.GraphQLClient{}
	restClient := &api.RESTClient{}
	apiGetter := NewAPIGetter(gqlClient, restClient)

	oldUploadsBaseURL := uploadsBaseURL
	uploadsBaseURL = testServer.URL + "/organizations/%d/gei/archive"
	defer func() { uploadsBaseURL = oldUploadsBaseURL }()

	_, err = apiGetter.UploadArchiveToGitHub(12345, archivePath, logger)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected response status")
}

func TestUploadLargeFileForceMultipart(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "upload-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	archivePath := filepath.Join(tempDir, "test-archive.tar.gz")
	err = os.WriteFile(archivePath, []byte("test"), 0644)
	assert.NoError(t, err)

	_ = os.Setenv("GITHUB_PAT", "invalid-token-for-test")
	defer func() { _ = os.Unsetenv("GITHUB_PAT") }()

	logger, _ := zap.NewDevelopment()

	gqlClient := &api.GraphQLClient{}
	restClient := &api.RESTClient{}
	apiGetter := NewAPIGetter(gqlClient, restClient)

	oldThreshold := DefaultMultipartThreshold
	DefaultMultipartThreshold = 1
	defer func() {
		DefaultMultipartThreshold = oldThreshold
	}()

	_, err = apiGetter.UploadArchiveToGitHub(12345, archivePath, logger)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start multipart upload")
}

func TestUploadEmptyFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "upload-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	archivePath := filepath.Join(tempDir, "empty-archive.tar.gz")
	err = os.WriteFile(archivePath, []byte{}, 0644)
	assert.NoError(t, err)

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if len(body) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error": "empty file"}`))
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uri": "gei://archive/test-archive-id"}`))
	}))
	defer testServer.Close()

	_ = os.Setenv("GITHUB_PAT", "test-token")
	defer func() { _ = os.Unsetenv("GITHUB_PAT") }()

	logger, _ := zap.NewDevelopment()

	gqlClient := &api.GraphQLClient{}
	restClient := &api.RESTClient{}
	apiGetter := NewAPIGetter(gqlClient, restClient)

	oldUploadsBaseURL := uploadsBaseURL
	uploadsBaseURL = testServer.URL + "/organizations/%d/gei/archive"
	defer func() { uploadsBaseURL = oldUploadsBaseURL }()

	_, err = apiGetter.UploadArchiveToGitHub(12345, archivePath, logger)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected response status")
}

func TestUploadWithMissingToken(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "upload-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	archivePath := filepath.Join(tempDir, "test-archive.tar.gz")
	err = os.WriteFile(archivePath, []byte("test content"), 0644)
	assert.NoError(t, err)

	originalToken := os.Getenv("GITHUB_PAT")
	defer func() { _ = os.Unsetenv("GITHUB_PAT") }()
	defer func() {
		if originalToken != "" {
			_ = os.Setenv("GITHUB_PAT", originalToken)
		}
	}()

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" || auth == "Bearer " || !strings.HasPrefix(auth, "Bearer ") || len(strings.TrimPrefix(auth, "Bearer ")) == 0 {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message": "Bad credentials"}`))
			return
		}
		w.WriteHeader(http.StatusCreated)
		response := map[string]string{"uri": "gei://archive/test-archive-id"}
		responseBytes, _ := json.Marshal(response)
		_, _ = w.Write(responseBytes)
	}))
	defer testServer.Close()

	logger, _ := zap.NewDevelopment()

	gqlClient := &api.GraphQLClient{}
	restClient := &api.RESTClient{}
	apiGetter := NewAPIGetter(gqlClient, restClient)

	oldUploadsBaseURL := uploadsBaseURL
	uploadsBaseURL = testServer.URL + "/organizations/%d/gei/archive"
	defer func() { uploadsBaseURL = oldUploadsBaseURL }()

	_, err = apiGetter.UploadArchiveToGitHub(12345, archivePath, logger)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected response status")
}

func TestUploadWithRetry(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "upload-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	archivePath := filepath.Join(tempDir, "test-archive.tar.gz")
	err = os.WriteFile(archivePath, []byte("test archive content"), 0644)
	assert.NoError(t, err)

	requestCount := 0
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error": "service unavailable"}`))
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uri": "gei://archive/test-archive-id"}`))
	}))
	defer testServer.Close()

	_ = os.Setenv("GITHUB_PAT", "test-token")
	defer func() { _ = os.Unsetenv("GITHUB_PAT") }()

	logger, _ := zap.NewDevelopment()

	gqlClient := &api.GraphQLClient{}
	restClient := &api.RESTClient{}
	apiGetter := NewAPIGetter(gqlClient, restClient)

	oldUploadsBaseURL := uploadsBaseURL
	uploadsBaseURL = testServer.URL + "/organizations/%d/gei/archive"
	defer func() { uploadsBaseURL = oldUploadsBaseURL }()

	_, err = apiGetter.UploadArchiveToGitHub(12345, archivePath, logger)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected response status")
}

func TestUploadPartSize(t *testing.T) {
	assert.Equal(t, int64(100*1024*1024), DefaultPartSize, "Default part size should be 100 MB")
	assert.Equal(t, int64(5000*1024*1024), DefaultMultipartThreshold, "Default multipart threshold should be 5 GB")
}

func TestUploadURLConstruction(t *testing.T) {
	orgID := 12345
	expectedURL := fmt.Sprintf("https://uploads.github.com/organizations/%d/gei/archive", orgID)
	actualURL := fmt.Sprintf(uploadsBaseURL, orgID)
	assert.Equal(t, expectedURL, actualURL)
}
