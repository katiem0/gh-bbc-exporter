package utils

import (
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
	exporter := NewExporter(client, "output", logger)

	assert.NotNil(t, exporter)
	assert.Equal(t, client, exporter.client)
	assert.Equal(t, "output", exporter.outputDir)
	assert.NotNil(t, exporter.logger)
}

func TestCreateBasicUsers(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := &Client{}
	exporter := NewExporter(client, "output", logger)

	users := exporter.createBasicUsers("testworkspace")

	assert.Len(t, users, 1)
	assert.Equal(t, "user", users[0].Type)
	assert.Equal(t, "testworkspace", users[0].Login)
}

func TestCreateOrganizationData(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := &Client{}
	exporter := NewExporter(client, "output", logger)

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
	exporter := NewExporter(client, tempDir, logger)

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
	exporter := NewExporter(client, tempDir, logger)

	dummyFilePath := filepath.Join(exporter.outputDir, "dummy.txt")
	err = os.WriteFile(dummyFilePath, []byte("test data"), 0644)
	assert.NoError(t, err)

	archivePath, err := exporter.CreateArchive()
	assert.NoError(t, err)
	assert.NotEmpty(t, archivePath)

	_, err = os.Stat(archivePath)
	assert.NoError(t, err)
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
	exporter := NewExporter(client, tempDir, logger)

	err = exporter.Export("workspace", "repo")
	assert.NoError(t, err)
}
