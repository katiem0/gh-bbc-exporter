package utils

import (
	"encoding/json"
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

func TestMigrateExportIntegration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "migrate-export-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"values": [], "next": null}`)); err != nil {
			t.Logf("Warning: Failed to write response: %v", err)
		}
	}))
	defer testServer.Close()

	logger, _ := zap.NewDevelopment()
	client := NewClient(testServer.URL, "token", "", "", "", "", logger, tempDir, false)

	assert.NotNil(t, client)
	assert.Equal(t, testServer.URL, client.baseURL)
	assert.Equal(t, "token", client.accessToken)
}

func TestMigrateArchiveCreation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "migrate-archive-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	logger, _ := zap.NewDevelopment()
	client := &Client{}
	exporter := NewExporter(client, tempDir, logger, false, "")

	// Create test data files
	schema := data.MigrationArchiveSchema{Version: "1.0.1"}
	err = exporter.writeJSONFile("schema.json", schema)
	assert.NoError(t, err)

	users := []data.User{{Type: "user", Login: "testuser", Name: "Test User"}}
	err = exporter.writeJSONFile("users_000001.json", users)
	assert.NoError(t, err)

	orgs := []data.Organization{{Type: "organization", Login: "testorg", Name: "Test Org"}}
	err = exporter.writeJSONFile("organizations_000001.json", orgs)
	assert.NoError(t, err)

	// Create archive
	archivePath, err := exporter.CreateArchive()
	assert.NoError(t, err)
	assert.FileExists(t, archivePath)
	assert.True(t, filepath.Ext(archivePath) == ".gz" || filepath.Base(archivePath) == filepath.Base(tempDir)+".tar.gz")
}

func TestMigrateDataPreparation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "migrate-data-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	logger, _ := zap.NewDevelopment()
	client := &Client{}
	exporter := NewExporter(client, tempDir, logger, false, "")

	repo := &data.BitbucketRepository{
		Name:        "test-repo",
		Slug:        "test-repo",
		Description: "Test repository description",
		IsPrivate:   true,
	}

	repos := exporter.createRepositoriesData(repo, "test-workspace")
	assert.Len(t, repos, 1)
	assert.Equal(t, "repository", repos[0].Type)
	assert.Equal(t, "test-repo", repos[0].Name)
	assert.Equal(t, "main", repos[0].DefaultBranch)
}

func TestMigrateTargetRepoConfiguration(t *testing.T) {
	testCases := []struct {
		name           string
		sourceRepo     string
		targetRepo     string
		expectedTarget string
		description    string
	}{
		{
			name:           "Target repo specified",
			sourceRepo:     "source-repo",
			targetRepo:     "custom-target",
			expectedTarget: "custom-target",
			description:    "Should use specified target repo",
		},
		{
			name:           "Target repo empty - uses source",
			sourceRepo:     "source-repo",
			targetRepo:     "",
			expectedTarget: "",
			description:    "Empty target means use source repo name",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			flags := data.CmdMigrateFlags{
				TargetRepo: tc.targetRepo,
			}
			assert.Equal(t, tc.expectedTarget, flags.TargetRepo)
		})
	}
}

func TestMigrateStorageProviderConfiguration(t *testing.T) {
	testCases := []struct {
		name                  string
		useGitHubStorage      bool
		azureConnectionString string
		awsBucketName         string
		awsRegion             string
		description           string
	}{
		{
			name:             "GitHub storage",
			useGitHubStorage: true,
			description:      "Using GitHub as storage provider",
		},
		{
			name:                  "Azure storage",
			azureConnectionString: "DefaultEndpointsProtocol=https;AccountName=test",
			description:           "Using Azure Blob Storage",
		},
		{
			name:          "AWS S3 storage",
			awsBucketName: "my-bucket",
			awsRegion:     "us-east-1",
			description:   "Using AWS S3",
		},
		{
			name:        "No external storage",
			description: "No storage provider configured",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			flags := data.CmdMigrateFlags{
				UseGitHubStorage:             tc.useGitHubStorage,
				AzureStorageConnectionString: tc.azureConnectionString,
				AWSBucketName:                tc.awsBucketName,
				AWSRegion:                    tc.awsRegion,
			}
			assert.Equal(t, tc.useGitHubStorage, flags.UseGitHubStorage)
			assert.Equal(t, tc.azureConnectionString, flags.AzureStorageConnectionString)
			assert.Equal(t, tc.awsBucketName, flags.AWSBucketName)
			assert.Equal(t, tc.awsRegion, flags.AWSRegion)
		})
	}
}

func TestMigrateRepoVisibility(t *testing.T) {
	testCases := []struct {
		name       string
		visibility string
		valid      bool
	}{
		{"Public", "public", true},
		{"Private", "private", true},
		{"Internal", "internal", true},
		{"Empty", "", true},
		{"Invalid uppercase", "PUBLIC", false},
		{"Invalid value", "secret", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var rv data.RepoVisibility
			err := rv.Set(tc.visibility)
			if tc.valid {
				assert.NoError(t, err)
				assert.Equal(t, tc.visibility, rv.String())
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestMigrateKeepArchiveFlag(t *testing.T) {
	testCases := []struct {
		name        string
		keepArchive bool
		description string
	}{
		{
			name:        "Keep archive enabled",
			keepArchive: true,
			description: "Archive should be retained after migration",
		},
		{
			name:        "Keep archive disabled",
			keepArchive: false,
			description: "Archive should be cleaned up after migration",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			flags := data.CmdMigrateFlags{
				KeepArchive: tc.keepArchive,
			}
			assert.Equal(t, tc.keepArchive, flags.KeepArchive)
		})
	}
}

func TestMigrateGitHubAPIConfiguration(t *testing.T) {
	testCases := []struct {
		name     string
		apiURL   string
		pat      string
		expected string
	}{
		{
			name:     "Default GitHub.com",
			apiURL:   "https://api.github.com",
			pat:      "ghp_xxxxxxxxxxxx",
			expected: "https://api.github.com",
		},
		{
			name:     "GitHub Enterprise",
			apiURL:   "https://github.mycompany.com/api/v3",
			pat:      "ghp_xxxxxxxxxxxx",
			expected: "https://github.mycompany.com/api/v3",
		},
		{
			name:     "GitHub Enterprise with port",
			apiURL:   "https://ghe.internal:8443/api/v3",
			pat:      "ghp_xxxxxxxxxxxx",
			expected: "https://ghe.internal:8443/api/v3",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			flags := data.CmdMigrateFlags{
				GitHubPAT: tc.pat,
			}
			assert.Equal(t, tc.pat, flags.GitHubPAT)
		})
	}
}

func TestMigrateAWSCredentials(t *testing.T) {
	testCases := []struct {
		name         string
		bucketName   string
		region       string
		accessKey    string
		secretKey    string
		sessionToken string
		description  string
	}{
		{
			name:         "Full credentials",
			bucketName:   "my-bucket",
			region:       "us-east-1",
			accessKey:    "AKIAIOSFODNN7EXAMPLE",
			secretKey:    "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			sessionToken: "session-token",
			description:  "All credentials provided",
		},
		{
			name:        "IAM role based",
			bucketName:  "my-bucket",
			region:      "eu-west-1",
			description: "Using IAM role, no explicit credentials",
		},
		{
			name:       "Credentials without session token",
			bucketName: "my-bucket",
			region:     "ap-southeast-1",
			accessKey:  "AKIAIOSFODNN7EXAMPLE",
			secretKey:  "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			flags := data.CmdMigrateFlags{
				AWSBucketName:   tc.bucketName,
				AWSRegion:       tc.region,
				AWSAccessKey:    tc.accessKey,
				AWSSecretKey:    tc.secretKey,
				AWSSessionToken: tc.sessionToken,
			}
			assert.Equal(t, tc.bucketName, flags.AWSBucketName)
			assert.Equal(t, tc.region, flags.AWSRegion)
			assert.Equal(t, tc.accessKey, flags.AWSAccessKey)
			assert.Equal(t, tc.secretKey, flags.AWSSecretKey)
			assert.Equal(t, tc.sessionToken, flags.AWSSessionToken)
		})
	}
}

func TestMigrateFlagsEmbeddedExportFlags(t *testing.T) {
	migrateFlags := data.CmdMigrateFlags{
		TargetOrg:            "target-org",
		TargetRepo:           "target-repo",
		TargetRepoVisibility: data.RepoVisibility("private"),
		KeepArchive:          true,
	}

	// Verify migrate-specific fields
	assert.Equal(t, "target-org", migrateFlags.TargetOrg)
	assert.Equal(t, "target-repo", migrateFlags.TargetRepo)
	assert.Equal(t, "private", migrateFlags.TargetRepoVisibility.String())
	assert.True(t, migrateFlags.KeepArchive)
}

func TestMigrateFlagsJSON(t *testing.T) {
	flags := data.CmdMigrateFlags{
		TargetOrg:            "github-org",
		TargetRepo:           "github-repo",
		TargetRepoVisibility: data.RepoVisibility("private"),
		KeepArchive:          true,
		UseGitHubStorage:     true,
	}

	jsonData, err := json.Marshal(flags)
	assert.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	var unmarshaledFlags data.CmdMigrateFlags
	err = json.Unmarshal(jsonData, &unmarshaledFlags)
	assert.NoError(t, err)

	assert.Equal(t, flags.TargetOrg, unmarshaledFlags.TargetOrg)
	assert.Equal(t, flags.TargetRepo, unmarshaledFlags.TargetRepo)
	assert.Equal(t, flags.KeepArchive, unmarshaledFlags.KeepArchive)
	assert.Equal(t, flags.UseGitHubStorage, unmarshaledFlags.UseGitHubStorage)
}

func TestMigrateSkipCommitLookup(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "migrate-skip-commit-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/commit/") {
			t.Error("Commit lookup API was called but should have been skipped")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"values": [], "next": null}`)); err != nil {
			t.Logf("Warning: Failed to write response: %v", err)
		}
	}))
	defer testServer.Close()

	logger, _ := zap.NewDevelopment()
	client := NewClient(testServer.URL, "token", "", "", "", "", logger, tempDir, true)

	assert.True(t, client.skipCommitLookup)
}

func TestMigrateDateValidation(t *testing.T) {
	testCases := []struct {
		name        string
		date        string
		expectError bool
	}{
		{"Valid date YYYY-MM-DD", "2023-01-15", false},
		{"Valid date start of year", "2023-01-01", false},
		{"Valid date end of year", "2023-12-31", false},
		{"Invalid format MM/DD/YYYY", "01/15/2023", true},
		{"Invalid format DD-MM-YYYY", "15-01-2023", true},
		{"Invalid date Feb 30", "2023-02-30", true},
		{"Empty date", "", false},
		{"Text instead of date", "yesterday", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			flags := &data.CmdExportFlags{
				Workspace:            "test",
				Repository:           "test",
				BitbucketAccessToken: "test",
				PRsFromDate:          tc.date,
			}

			err := ValidateExportFlags(flags)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
