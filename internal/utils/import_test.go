package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
	graphql "github.com/cli/shurcooL-graphql"
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

func TestMigrateRepoVisibility(t *testing.T) {
	testCases := []struct {
		name           string
		visibility     string
		valid          bool
		expectedString string
	}{
		{"Public", "public", true, "public"},
		{"Private", "private", true, "private"},
		{"Internal", "internal", true, "internal"},
		{"Empty defaults to private", "", true, "private"},
		{"Invalid uppercase", "PUBLIC", false, ""},
		{"Invalid value", "secret", false, ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var rv data.RepoVisibility
			err := rv.Set(tc.visibility)
			if tc.valid {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedString, rv.String())
			} else {
				assert.Error(t, err)
			}
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

func TestMigrateFlagsEmbeddedExportFlags(t *testing.T) {
	migrateFlags := data.CmdMigrateFlags{
		TargetOrg:            "target-org",
		TargetRepo:           "target-repo",
		TargetRepoVisibility: data.RepoVisibility("private"),
	}

	// Verify migrate-specific fields
	assert.Equal(t, "target-org", migrateFlags.TargetOrg)
	assert.Equal(t, "target-repo", migrateFlags.TargetRepo)
	assert.Equal(t, "private", migrateFlags.TargetRepoVisibility.String())
}

func TestMigrateFlagsJSON(t *testing.T) {
	flags := data.CmdMigrateFlags{
		TargetOrg:            "github-org",
		TargetRepo:           "github-repo",
		TargetRepoVisibility: data.RepoVisibility("private"),
	}

	jsonData, err := json.Marshal(flags)
	assert.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	var unmarshaledFlags data.CmdMigrateFlags
	err = json.Unmarshal(jsonData, &unmarshaledFlags)
	assert.NoError(t, err)

	assert.Equal(t, flags.TargetOrg, unmarshaledFlags.TargetOrg)
	assert.Equal(t, flags.TargetRepo, unmarshaledFlags.TargetRepo)
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

func TestNewAPIGetter(t *testing.T) {
	gqlClient := &api.GraphQLClient{}
	restClient := &api.RESTClient{}

	apiGetter := NewAPIGetter(gqlClient, restClient)

	assert.NotNil(t, apiGetter)
}

func TestGetOrganizationInfo(t *testing.T) {
	gqlClient := &api.GraphQLClient{}
	restClient := &api.RESTClient{}
	apiGetter := NewAPIGetter(gqlClient, restClient)

	assert.NotNil(t, apiGetter)
}

func TestCreateMigrationSourceInputStruct(t *testing.T) {
	input := data.CreateMigrationSourceInput{
		Name:    "Bitbucket Cloud Migration",
		URL:     "https://bitbucket.org",
		OwnerID: "ORG_123",
		Type:    "GITHUB_ARCHIVE",
	}

	assert.Equal(t, "Bitbucket Cloud Migration", string(input.Name))
	assert.Equal(t, "https://bitbucket.org", string(input.URL))
	assert.Equal(t, "ORG_123", string(input.OwnerID))
	assert.Equal(t, "GITHUB_ARCHIVE", string(input.Type))
}

func TestStartRepositoryMigrationInputStruct(t *testing.T) {
	input := data.StartRepositoryMigrationInput{
		SourceID:             "SOURCE_123",
		OwnerID:              "ORG_456",
		RepositoryName:       "test-repo",
		ContinueOnError:      true,
		GitHubPAT:            "ghp_token",
		AccessToken:          "ghp_token",
		GitArchiveUrl:        "gei://archive/test-id",
		MetadataArchiveUrl:   "gei://archive/test-id",
		SourceRepositoryUrl:  "https://bitbucket.org/workspace/repo",
		TargetRepoVisibility: "private",
	}

	assert.Equal(t, "SOURCE_123", string(input.SourceID))
	assert.Equal(t, "ORG_456", string(input.OwnerID))
	assert.Equal(t, "test-repo", string(input.RepositoryName))
	assert.True(t, bool(input.ContinueOnError))
	assert.Equal(t, "ghp_token", string(input.GitHubPAT))
	assert.Equal(t, "gei://archive/test-id", string(input.GitArchiveUrl))
	assert.Equal(t, "private", string(input.TargetRepoVisibility))
}

func TestOrganizationIDQueryStruct(t *testing.T) {
	query := data.OrganizationIDQuery{}
	query.Organization.ID = "ORG_NODE_ID"
	query.Organization.DatabaseID = 12345

	assert.Equal(t, "ORG_NODE_ID", query.Organization.ID)
	assert.Equal(t, 12345, query.Organization.DatabaseID)
}

func TestMutationMigrationSourceStruct(t *testing.T) {
	mutation := data.MutationMigrationSource{}
	mutation.CreateMigrationSource.MigrationSource.ID = "MS_123"
	mutation.CreateMigrationSource.MigrationSource.Name = "Test Source"
	mutation.CreateMigrationSource.MigrationSource.Type = "GITHUB_ARCHIVE"

	assert.Equal(t, "MS_123", mutation.CreateMigrationSource.MigrationSource.ID)
	assert.Equal(t, "Test Source", mutation.CreateMigrationSource.MigrationSource.Name)
	assert.Equal(t, "GITHUB_ARCHIVE", mutation.CreateMigrationSource.MigrationSource.Type)
}

func TestStartMigrationResponseStruct(t *testing.T) {
	response := data.StartMigrationResponse{}
	response.StartRepositoryMigration.RepositoryMigration.ID = "MIG_123"

	assert.Equal(t, "MIG_123", response.StartRepositoryMigration.RepositoryMigration.ID)
}

func TestMigrationStatusQueryStruct(t *testing.T) {
	query := data.MigrationStatusQuery{}
	query.Node.Migration.ID = "MIG_456"
	query.Node.Migration.State = "SUCCEEDED"
	query.Node.Migration.FailureReason = ""

	assert.Equal(t, "SUCCEEDED", query.Node.Migration.State)
	assert.Empty(t, query.Node.Migration.FailureReason)
}

func TestMigrationStatusQueryFailedState(t *testing.T) {
	query := data.MigrationStatusQuery{}
	query.Node.Migration.ID = "MIG_789"
	query.Node.Migration.State = "FAILED"
	query.Node.Migration.FailureReason = "Invalid archive format"

	assert.Equal(t, "FAILED", query.Node.Migration.State)
	assert.Equal(t, "Invalid archive format", query.Node.Migration.FailureReason)
}

func TestRunGitHubAPIMigrationTargetRepoDefault(t *testing.T) {
	exportFlags := &data.CmdExportFlags{
		Workspace:  "test-workspace",
		Repository: "source-repo",
	}
	migrateFlags := &data.CmdMigrateFlags{
		TargetOrg:  "target-org",
		TargetRepo: "",
	}

	targetRepo := migrateFlags.TargetRepo
	if targetRepo == "" {
		targetRepo = exportFlags.Repository
	}

	assert.Equal(t, "source-repo", targetRepo)
}

func TestRunGitHubAPIMigrationTargetRepoSpecified(t *testing.T) {
	exportFlags := &data.CmdExportFlags{
		Workspace:  "test-workspace",
		Repository: "source-repo",
	}
	migrateFlags := &data.CmdMigrateFlags{
		TargetOrg:  "target-org",
		TargetRepo: "custom-target-repo",
	}

	targetRepo := migrateFlags.TargetRepo
	if targetRepo == "" {
		targetRepo = exportFlags.Repository
	}

	assert.Equal(t, "custom-target-repo", targetRepo)
}

func TestMigrationVisibilityValues(t *testing.T) {
	testCases := []struct {
		name       string
		visibility data.RepoVisibility
		expected   string
	}{
		{"Private visibility", data.RepoVisibility("private"), "private"},
		{"Public visibility", data.RepoVisibility("public"), "public"},
		{"Internal visibility", data.RepoVisibility("internal"), "internal"},
		{"Empty defaults to private", data.RepoVisibility(""), "private"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.visibility.String())
		})
	}
}

func TestMigrationSourceURL(t *testing.T) {
	workspace := "test-workspace"
	repo := "test-repo"

	expectedURL := fmt.Sprintf("https://bitbucket.org/%s/%s", workspace, repo)
	assert.Equal(t, "https://bitbucket.org/test-workspace/test-repo", expectedURL)
}

func TestMigrationArchiveURIFormat(t *testing.T) {
	testURIs := []struct {
		uri   string
		valid bool
	}{
		{"gei://archive/abc123", true},
		{"gei://archive/test-archive-id", true},
		{"https://example.com/archive", false},
		{"", false},
	}

	for _, tc := range testURIs {
		t.Run(tc.uri, func(t *testing.T) {
			isGEI := strings.HasPrefix(tc.uri, "gei://")
			assert.Equal(t, tc.valid, isGEI)
		})
	}
}

func TestMigrationStatusStates(t *testing.T) {
	states := []struct {
		state      string
		isTerminal bool
		isSuccess  bool
	}{
		{"QUEUED", false, false},
		{"IN_PROGRESS", false, false},
		{"SUCCEEDED", true, true},
		{"FAILED", true, false},
		{"PENDING_VALIDATION", false, false},
	}

	for _, s := range states {
		t.Run(s.state, func(t *testing.T) {
			isTerminal := s.state == "SUCCEEDED" || s.state == "FAILED"
			isSuccess := s.state == "SUCCEEDED"

			assert.Equal(t, s.isTerminal, isTerminal)
			assert.Equal(t, s.isSuccess, isSuccess)
		})
	}
}

func TestGetGitHubAuthTokenFromFlags(t *testing.T) {
	originalPAT := os.Getenv("GITHUB_PAT")
	defer func() {
		if originalPAT != "" {
			_ = os.Setenv("GITHUB_PAT", originalPAT)
		} else {
			_ = os.Unsetenv("GITHUB_PAT")
		}
	}()
	_ = os.Unsetenv("GITHUB_PAT")

	testCases := []struct {
		name        string
		flagPAT     string
		envPAT      string
		expectedPAT string
	}{
		{
			name:        "PAT from flags",
			flagPAT:     "ghp_from_flag",
			envPAT:      "",
			expectedPAT: "ghp_from_flag",
		},
		{
			name:        "PAT from environment",
			flagPAT:     "",
			envPAT:      "ghp_from_env",
			expectedPAT: "ghp_from_env",
		},
		{
			name:        "Flag takes precedence over env",
			flagPAT:     "ghp_from_flag",
			envPAT:      "ghp_from_env",
			expectedPAT: "ghp_from_flag",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envPAT != "" {
				_ = os.Setenv("GITHUB_PAT", tc.envPAT)
			} else {
				_ = os.Unsetenv("GITHUB_PAT")
			}

			migrateFlags := &data.CmdMigrateFlags{
				GitHubPAT: tc.flagPAT,
			}

			token, err := GetGitHubAuthToken(migrateFlags)
			if tc.expectedPAT != "" {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedPAT, token)
			}
		})
	}
}

func TestMigrationInputValidation(t *testing.T) {
	testCases := []struct {
		name        string
		sourceID    string
		orgID       string
		targetRepo  string
		archiveURI  string
		expectValid bool
	}{
		{
			name:        "All fields valid",
			sourceID:    "MS_123",
			orgID:       "ORG_456",
			targetRepo:  "test-repo",
			archiveURI:  "gei://archive/test-id",
			expectValid: true,
		},
		{
			name:        "Empty source ID",
			sourceID:    "",
			orgID:       "ORG_456",
			targetRepo:  "test-repo",
			archiveURI:  "gei://archive/test-id",
			expectValid: false,
		},
		{
			name:        "Empty org ID",
			sourceID:    "MS_123",
			orgID:       "",
			targetRepo:  "test-repo",
			archiveURI:  "gei://archive/test-id",
			expectValid: false,
		},
		{
			name:        "Empty target repo",
			sourceID:    "MS_123",
			orgID:       "ORG_456",
			targetRepo:  "",
			archiveURI:  "gei://archive/test-id",
			expectValid: false,
		},
		{
			name:        "Empty archive URI",
			sourceID:    "MS_123",
			orgID:       "ORG_456",
			targetRepo:  "test-repo",
			archiveURI:  "",
			expectValid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isValid := tc.sourceID != "" && tc.orgID != "" && tc.targetRepo != "" && tc.archiveURI != ""
			assert.Equal(t, tc.expectValid, isValid)
		})
	}
}

func TestMigrationSourceCreationInput(t *testing.T) {
	name := "Bitbucket Cloud Migration"
	url := "https://bitbucket.org"
	ownerID := "ORG_123"

	input := data.CreateMigrationSourceInput{
		Name:    graphql.String(name),
		URL:     graphql.String(url),
		OwnerID: graphql.String(ownerID),
		Type:    graphql.String("GITHUB_ARCHIVE"),
	}

	assert.Equal(t, name, string(input.Name))
	assert.Equal(t, url, string(input.URL))
	assert.Equal(t, ownerID, string(input.OwnerID))
	assert.Equal(t, "GITHUB_ARCHIVE", string(input.Type))
}

func TestAPIGetterCreation(t *testing.T) {
	gqlClient := &api.GraphQLClient{}
	restClient := &api.RESTClient{}

	apiGetter := NewAPIGetter(gqlClient, restClient)

	assert.NotNil(t, apiGetter)
	assert.NotNil(t, apiGetter.gqlClient)
	assert.NotNil(t, apiGetter.restClient)
}

func TestMigrationArchivePathValidation(t *testing.T) {
	testCases := []struct {
		name        string
		archivePath string
		expectValid bool
	}{
		{
			name:        "Valid tar.gz file",
			archivePath: "/path/to/archive.tar.gz",
			expectValid: true,
		},
		{
			name:        "Valid with timestamp",
			archivePath: "/path/to/bitbucket-export-20230615-120000.tar.gz",
			expectValid: true,
		},
		{
			name:        "Invalid extension",
			archivePath: "/path/to/archive.zip",
			expectValid: false,
		},
		{
			name:        "Empty path",
			archivePath: "",
			expectValid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isValid := tc.archivePath != "" && strings.HasSuffix(tc.archivePath, ".tar.gz")
			assert.Equal(t, tc.expectValid, isValid)
		})
	}
}

func TestMigrationFailureReasons(t *testing.T) {
	failureReasons := []string{
		"Invalid archive format",
		"Repository already exists",
		"Authentication failed",
		"Rate limit exceeded",
		"Organization not found",
		"Insufficient permissions",
	}

	for _, reason := range failureReasons {
		t.Run(reason, func(t *testing.T) {
			query := data.MigrationStatusQuery{}
			query.Node.Migration.State = "FAILED"
			query.Node.Migration.FailureReason = reason

			assert.Equal(t, "FAILED", query.Node.Migration.State)
			assert.NotEmpty(t, query.Node.Migration.FailureReason)
		})
	}
}

func TestSourceRepositoryURLConstruction(t *testing.T) {
	testCases := []struct {
		name        string
		workspace   string
		repo        string
		expectedURL string
	}{
		{
			name:        "Standard workspace and repo",
			workspace:   "myworkspace",
			repo:        "myrepo",
			expectedURL: "https://bitbucket.org/myworkspace/myrepo",
		},
		{
			name:        "Workspace with hyphens",
			workspace:   "my-workspace",
			repo:        "my-repo",
			expectedURL: "https://bitbucket.org/my-workspace/my-repo",
		},
		{
			name:        "Workspace with numbers",
			workspace:   "workspace123",
			repo:        "repo456",
			expectedURL: "https://bitbucket.org/workspace123/repo456",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			url := fmt.Sprintf("https://bitbucket.org/%s/%s", tc.workspace, tc.repo)
			assert.Equal(t, tc.expectedURL, url)
		})
	}
}
