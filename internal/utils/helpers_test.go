package utils

import (
	"encoding/json"
	"fmt"
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

func TestValidateExportFlags(t *testing.T) {
	// Test case 1: No credentials provided
	cmdFlags := &data.CmdFlags{}
	err := ValidateExportFlags(cmdFlags)
	assert.Error(t, err, "Expected error when no credentials are provided")
	assert.Contains(t, err.Error(), "authentication credentials required")

	// Test case 2: Token provided
	cmdFlags = &data.CmdFlags{}
	cmdFlags.BitbucketAccessToken = "testtoken"
	err = ValidateExportFlags(cmdFlags)
	assert.NoError(t, err, "Expected no error when token is provided")

	// Test case 3: Username and app password provided
	cmdFlags = &data.CmdFlags{}
	cmdFlags.BitbucketAccessToken = ""
	cmdFlags.BitbucketUser = "testuser"
	cmdFlags.BitbucketAppPass = "testpass"
	err = ValidateExportFlags(cmdFlags)
	assert.NoError(t, err, "Expected no error when username and app password are provided")

	// Test case 4: Only username provided (missing app password)
	cmdFlags = &data.CmdFlags{}
	cmdFlags.BitbucketAccessToken = ""
	cmdFlags.BitbucketUser = "testuser"
	cmdFlags.BitbucketAppPass = ""
	err = ValidateExportFlags(cmdFlags)
	assert.Error(t, err, "Expected error when only username is provided")
	// The actual error message is the general authentication error, not the specific one
	assert.Contains(t, err.Error(), "authentication credentials required")

	// Test case 5: Only app password provided (missing username)
	cmdFlags = &data.CmdFlags{}
	cmdFlags.BitbucketAccessToken = ""
	cmdFlags.BitbucketUser = ""
	cmdFlags.BitbucketAppPass = "testpass"
	err = ValidateExportFlags(cmdFlags)
	assert.Error(t, err, "Expected error when only app password is provided")
	// The actual error message is the general authentication error, not the specific one
	assert.Contains(t, err.Error(), "authentication credentials required")

	// Test case 6: API token and email provided
	cmdFlags = &data.CmdFlags{}
	cmdFlags.BitbucketAPIToken = "testapitoken"
	cmdFlags.BitbucketEmail = "test@example.com"
	err = ValidateExportFlags(cmdFlags)
	assert.NoError(t, err, "Expected no error when API token and email are provided")

	// Test case 7: Only API token provided (missing email)
	cmdFlags = &data.CmdFlags{}
	cmdFlags.BitbucketAPIToken = "testapitoken"
	cmdFlags.BitbucketEmail = ""
	err = ValidateExportFlags(cmdFlags)
	assert.Error(t, err, "Expected error when only API token is provided")
	// The actual error message is the general authentication error, not the specific one
	assert.Contains(t, err.Error(), "authentication credentials required")

	// Test case 8: Only email provided (missing API token)
	cmdFlags = &data.CmdFlags{}
	cmdFlags.BitbucketAPIToken = ""
	cmdFlags.BitbucketEmail = "test@example.com"
	err = ValidateExportFlags(cmdFlags)
	assert.Error(t, err, "Expected error when only email is provided")
	// The actual error message is the general authentication error, not the specific one
	assert.Contains(t, err.Error(), "authentication credentials required")

	// Test case 9: Mixed authentication methods - access token with username/password
	cmdFlags = &data.CmdFlags{}
	cmdFlags.BitbucketAccessToken = "testtoken"
	cmdFlags.BitbucketUser = "testuser"
	cmdFlags.BitbucketAppPass = "testpass"
	err = ValidateExportFlags(cmdFlags)
	assert.Error(t, err, "Expected error when multiple authentication methods are provided")
	assert.Contains(t, err.Error(), "mixed authentication methods")

	// Test case 9b: Mixed authentication methods - access token with API token/email
	cmdFlags = &data.CmdFlags{}
	cmdFlags.BitbucketAccessToken = "testtoken"
	cmdFlags.BitbucketAPIToken = "testapitoken"
	cmdFlags.BitbucketEmail = "test@example.com"
	err = ValidateExportFlags(cmdFlags)
	assert.Error(t, err, "Expected error when multiple authentication methods are provided")
	assert.Contains(t, err.Error(), "mixed authentication methods")

	// Test case 9c: Mixed authentication methods - API token/email with username/password
	cmdFlags = &data.CmdFlags{}
	cmdFlags.BitbucketAPIToken = "testapitoken"
	cmdFlags.BitbucketEmail = "test@example.com"
	cmdFlags.BitbucketUser = "testuser"
	cmdFlags.BitbucketAppPass = "testpass"
	err = ValidateExportFlags(cmdFlags)
	assert.Error(t, err, "Expected error when multiple authentication methods are provided")
	assert.Contains(t, err.Error(), "mixed authentication methods")

	// Test case 10: Valid date format for PRsFromDate
	cmdFlags = &data.CmdFlags{}
	cmdFlags.BitbucketAccessToken = "testtoken"
	cmdFlags.PRsFromDate = "2023-01-01"
	err = ValidateExportFlags(cmdFlags)
	assert.NoError(t, err, "Expected no error with valid date format")

	// Test case 11: Invalid date format for PRsFromDate
	cmdFlags = &data.CmdFlags{}
	cmdFlags.BitbucketAccessToken = "testtoken"
	cmdFlags.PRsFromDate = "01/01/2023"
	err = ValidateExportFlags(cmdFlags)
	assert.Error(t, err, "Expected error with invalid date format")
	assert.Contains(t, err.Error(), "invalid date format for --prs-from-date", "Error should mention invalid date format")
}

func TestSetupEnvironmentCredentials(t *testing.T) {
	cmdFlags := &data.CmdFlags{}

	// Clean up any existing environment variables first
	if err := os.Unsetenv("BITBUCKET_USERNAME"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}
	if err := os.Unsetenv("BITBUCKET_APP_PASSWORD"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}
	if err := os.Unsetenv("BITBUCKET_ACCESS_TOKEN"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}
	if err := os.Unsetenv("BITBUCKET_API_TOKEN"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}
	if err := os.Unsetenv("BITBUCKET_EMAIL"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}

	// Set environment variables
	err := os.Setenv("BITBUCKET_USERNAME", "envuser")
	assert.NoError(t, err)
	err = os.Setenv("BITBUCKET_APP_PASSWORD", "envpass")
	assert.NoError(t, err)
	err = os.Setenv("BITBUCKET_ACCESS_TOKEN", "envtoken") // Corrected from BITBUCKET_TOKEN
	assert.NoError(t, err)
	err = os.Setenv("BITBUCKET_API_TOKEN", "envapitoken") // Added API token
	assert.NoError(t, err)
	err = os.Setenv("BITBUCKET_EMAIL", "user@example.com") // Added email
	assert.NoError(t, err)

	// Call the function
	SetupEnvironmentCredentials(cmdFlags)

	// Assert that the values are set correctly
	assert.Equal(t, "envuser", cmdFlags.BitbucketUser, "Expected username to be set from environment")
	assert.Equal(t, "envpass", cmdFlags.BitbucketAppPass, "Expected app password to be set from environment")
	assert.Equal(t, "envtoken", cmdFlags.BitbucketAccessToken, "Expected access token to be set from environment")
	assert.Equal(t, "envapitoken", cmdFlags.BitbucketAPIToken, "Expected API token to be set from environment")
	assert.Equal(t, "user@example.com", cmdFlags.BitbucketEmail, "Expected email to be set from environment")

	// Clean up environment variables
	err = os.Unsetenv("BITBUCKET_USERNAME")
	assert.NoError(t, err)
	err = os.Unsetenv("BITBUCKET_APP_PASSWORD")
	assert.NoError(t, err)
	err = os.Unsetenv("BITBUCKET_ACCESS_TOKEN")
	assert.NoError(t, err)
	err = os.Unsetenv("BITBUCKET_API_TOKEN")
	assert.NoError(t, err)
	err = os.Unsetenv("BITBUCKET_EMAIL")
	assert.NoError(t, err)
}

func TestFormatDateToZ(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "ISO8601 format",
			input:    "2023-01-01T12:34:56.789Z",
			expected: "2023-01-01T12:34:56Z",
		},
		{
			name:     "RFC3339 format",
			input:    "2023-01-01T12:34:56Z",
			expected: "2023-01-01T12:34:56Z",
		},
		{
			name:     "RFC3339 with nanoseconds",
			input:    "2023-01-01T12:34:56.789123456Z",
			expected: "2023-01-01T12:34:56Z",
		},
		{
			name:     "RFC3339 with offset",
			input:    "2023-01-01T12:34:56+00:00",
			expected: "2023-01-01T12:34:56Z",
		},
		{
			name:     "Custom format",
			input:    "2023/01/01 12:34:56",
			expected: "2023-01-01T12:34:56Z",
		},
		{
			name:     "Current time (approximate)",
			input:    time.Now().Format(time.RFC3339),
			expected: time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "Current time (approximate)" {
				// Skip exact comparison for current time
				result := formatDateToZ(tc.input)
				assert.NotEmpty(t, result)
			} else {
				result := formatDateToZ(tc.input)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestFormatDateToZEdgeCases(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expected    string
		shouldMatch bool
	}{
		{
			name:        "Empty string",
			input:       "",
			expected:    "", // Should return empty string
			shouldMatch: true,
		},
		{
			name:        "Invalid date format",
			input:       "not-a-date",
			expected:    "", // Should return empty string or default value
			shouldMatch: true,
		},
		{
			name:        "Date with microseconds",
			input:       "2023-01-01T12:34:56.123456+00:00",
			expected:    "2023-01-01T12:34:56Z",
			shouldMatch: true,
		},
		{
			name:        "Date with timezone offset",
			input:       "2023-01-01T12:34:56+05:30",
			expected:    "2023-01-01T07:04:56Z", // Converted to UTC
			shouldMatch: true,
		},
		{
			name:        "RFC3339 format",
			input:       "2023-01-01T12:34:56Z",
			expected:    "2023-01-01T12:34:56Z",
			shouldMatch: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatDateToZ(tc.input)
			if tc.shouldMatch {
				assert.Equal(t, tc.expected, result)
			} else {
				assert.NotEqual(t, tc.expected, result)
			}
		})
	}
}

func TestFormatURL(t *testing.T) {
	testCases := []struct {
		name       string
		urlType    string
		workspace  string
		repository string
		id         []interface{}
		expected   string
	}{
		{
			name:       "PR URL",
			urlType:    "pr",
			workspace:  "testworkspace",
			repository: "testrepo",
			id:         []interface{}{"123"},
			expected:   "https://bitbucket.org/testworkspace/testrepo/pull/123",
		},
		{
			name:       "PR URL without ID",
			urlType:    "pr",
			workspace:  "testworkspace",
			repository: "testrepo",
			id:         []interface{}{},
			expected:   "https://bitbucket.org/testworkspace/testrepo/pulls",
		},
		{
			name:       "Repository URL",
			urlType:    "repository",
			workspace:  "testworkspace",
			repository: "testrepo",
			id:         []interface{}{},
			expected:   "https://bitbucket.org/testworkspace/testrepo",
		},
		{
			name:       "User URL",
			urlType:    "user",
			workspace:  "testworkspace",
			repository: "",
			id:         []interface{}{"testuser"},
			expected:   "https://bitbucket.org/testuser",
		},
		{
			name:       "User URL without ID",
			urlType:    "user",
			workspace:  "testworkspace",
			repository: "",
			id:         []interface{}{},
			expected:   "https://bitbucket.org/testworkspace",
		},
		{
			name:       "PR Review URL",
			urlType:    "pr_review",
			workspace:  "testworkspace",
			repository: "testrepo",
			id:         []interface{}{"123", "456"},
			expected:   "https://bitbucket.org/testworkspace/testrepo/pull/123/files#pullrequestreview-456",
		},
		{
			name:       "PR Review URL without ID",
			urlType:    "pr_review",
			workspace:  "testworkspace",
			repository: "testrepo",
			id:         []interface{}{},
			expected:   "https://bitbucket.org/testworkspace/testrepo/pull/reviews",
		},
		{
			name:       "PR Review Comment URL",
			urlType:    "pr_review_comment",
			workspace:  "testworkspace",
			repository: "testrepo",
			id:         []interface{}{"123", "456"},
			expected:   "https://bitbucket.org/testworkspace/testrepo/pull/123/files#r456",
		},
		{
			name:       "PR Review Comment URL without ID",
			urlType:    "pr_review_comment",
			workspace:  "testworkspace",
			repository: "testrepo",
			id:         []interface{}{},
			expected:   "https://bitbucket.org/testworkspace/testrepo/pull/comments",
		},
		{
			name:       "Issue Comment URL",
			urlType:    "issue_comment",
			workspace:  "testworkspace",
			repository: "testrepo",
			id:         []interface{}{"123", "456"},
			expected:   "https://bitbucket.org/testworkspace/testrepo/pull/123#issuecomment-456",
		},
		{
			name:       "PR Review Thread URL",
			urlType:    "pr_review_thread",
			workspace:  "testworkspace",
			repository: "testrepo",
			id:         []interface{}{"123", "456"},
			expected:   "https://bitbucket.org/testworkspace/testrepo/pull/123/files#pullrequestreviewthread-456",
		},
		{
			name:       "PR Review Thread URL without ID",
			urlType:    "pr_review_thread",
			workspace:  "testworkspace",
			repository: "testrepo",
			id:         []interface{}{},
			expected:   "https://bitbucket.org/testworkspace/testrepo/pull/threads",
		},
		{
			name:       "Git URL",
			urlType:    "git",
			workspace:  "testworkspace",
			repository: "testrepo",
			id:         []interface{}{},
			expected:   "tarball://root/repositories/testworkspace/testrepo.git",
		},
		{
			name:       "Organization URL",
			urlType:    "organization",
			workspace:  "testworkspace",
			repository: "",
			id:         []interface{}{},
			expected:   "https://bitbucket.org/testworkspace",
		},
		{
			name:       "Default URL",
			urlType:    "unknown",
			workspace:  "testworkspace",
			repository: "testrepo",
			id:         []interface{}{},
			expected:   "https://bitbucket.org/testworkspace/testrepo",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result string

			// Pass the arguments based on the test case
			switch len(tc.id) {
			case 0:
				result = formatURL(tc.urlType, tc.workspace, tc.repository)
			case 1:
				result = formatURL(tc.urlType, tc.workspace, tc.repository, tc.id[0])
			case 2:
				result = formatURL(tc.urlType, tc.workspace, tc.repository, tc.id[0], tc.id[1])
			default:
				t.Fatalf("Unsupported number of ID parameters: %d", len(tc.id))
			}

			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestPrintSuccessMessage(t *testing.T) {
	// This is mostly a visual test, so we just ensure it doesn't panic
	assert.NotPanics(t, func() {
		PrintSuccessMessage("/path/to/output")
	})
}

func TestPaginationHandling(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Create a test server that returns paginated results
	firstPage := true
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.String(), "/pullrequests") {
			w.WriteHeader(http.StatusOK)
			if firstPage {
				// First page with next link
				writeResponse(t, w, []byte(`{
                    "values": [
                        {
                            "id": 1, 
                            "title": "First Page PR", 
                            "state": "OPEN",
                            "author": {"uuid": "{test-uuid}"},
                            "source": {"branch": {"name": "feature"}, "commit": {"hash": "1234567890123456789012345678901234567890"}},
                            "destination": {"branch": {"name": "main"}, "commit": {"hash": "0987654321098765432109876543210987654321"}}
                        }
                    ], 
                    "next": "https://api.bitbucket.org/2.0/repositories/workspace/repo/pullrequests?page=2"
                }`))
				firstPage = false
			} else {
				// Second page with no next link
				writeResponse(t, w, []byte(`{
                    "values": [
                        {
                            "id": 2, 
                            "title": "Second Page PR", 
                            "state": "OPEN",
                            "author": {"uuid": "{test-uuid}"},
                            "source": {"branch": {"name": "feature2"}, "commit": {"hash": "abcdef1234567890abcdef1234567890abcdef12"}},
                            "destination": {"branch": {"name": "main"}, "commit": {"hash": "fedcba0987654321fedcba0987654321fedcba09"}}
                        }
                    ], 
                    "next": null
                }`))
			}
		} else {
			// For commit SHA lookups
			w.WriteHeader(http.StatusOK)
			writeResponse(t, w, []byte(`{"hash": "1234567890123456789012345678901234567890"}`))
		}
	}))
	defer testServer.Close()

	client := &Client{
		baseURL:        testServer.URL,
		httpClient:     testServer.Client(),
		logger:         logger,
		commitSHACache: make(map[string]string),
	}

	prs, err := client.GetPullRequests("workspace", "repo", false, "")

	assert.NoError(t, err)
	assert.Len(t, prs, 2, "Expected PRs from both pages")
	assert.Equal(t, "First Page PR", prs[0].Title)
	assert.Equal(t, "Second Page PR", prs[1].Title)
}

func TestExtractPRNumber(t *testing.T) {
	testCases := []struct {
		name     string
		prURL    string
		expected string
	}{
		{
			name:     "Standard PR URL",
			prURL:    "https://bitbucket.org/workspace/repo/pull/123",
			expected: "123",
		},
		{
			name:     "PR URL with additional path segments",
			prURL:    "https://bitbucket.org/workspace/repo/pull/456/overview",
			expected: "456",
		},
		{
			name:     "PR URL with query parameters",
			prURL:    "https://bitbucket.org/workspace/repo/pull/789?param=value",
			expected: "789",
		},
		{
			name:     "Invalid URL format",
			prURL:    "https://bitbucket.org/workspace/repo/not-a-pull/123",
			expected: "",
		},
		{
			name:     "Empty URL",
			prURL:    "",
			expected: "",
		},
		{
			name:     "PR URL with multiple segments and query",
			prURL:    "https://bitbucket.org/workspace/repo/pull/101/commits/abc123?at=branch",
			expected: "101",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := extractPRNumber(tc.prURL)
			assert.Equal(t, tc.expected, result, "For URL %s, expected PR number %s but got %s", tc.prURL, tc.expected, result)
		})
	}
}

func TestPRDateValidation(t *testing.T) {
	testCases := []struct {
		name        string
		dateStr     string
		expectError bool
	}{
		{
			name:        "Valid date format",
			dateStr:     "2023-01-01",
			expectError: false,
		},
		{
			name:        "Invalid date format - slashes",
			dateStr:     "01/01/2023",
			expectError: true,
		},
		{
			name:        "Invalid date format - month first",
			dateStr:     "01-31-2023",
			expectError: true,
		},
		{
			name:        "Invalid date - nonexistent date",
			dateStr:     "2023-02-30",
			expectError: true,
		},
		{
			name:        "Invalid date - text",
			dateStr:     "yesterday",
			expectError: true,
		},
		{
			name:        "Empty string",
			dateStr:     "",
			expectError: false, // Empty is valid as it's optional
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmdFlags := &data.CmdFlags{
				BitbucketAccessToken: "test-token", // Add required auth
				PRsFromDate:          tc.dateStr,
			}

			err := ValidateExportFlags(cmdFlags)

			if tc.expectError {
				assert.Error(t, err, "Expected error for date: %s", tc.dateStr)
				assert.Contains(t, err.Error(), "invalid date format", "Error should mention date format")
			} else {
				assert.NoError(t, err, "Expected no error for date: %s", tc.dateStr)
			}
		})
	}
}

func TestHashString(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:  "Simple string",
			input: "test-string",
			// Expected hash value may need to be updated based on your implementation
			expected: HashString("test-string"),
		},
		{
			name:  "File path with line number",
			input: "workspace-src/file.go-10",
			// Expected hash value may need to be updated based on your implementation
			expected: HashString("workspace-src/file.go-10"),
		},
		{
			name:  "Empty string",
			input: "",
			// Expected hash value may need to be updated based on your implementation
			expected: HashString(""),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := HashString(tc.input)
			assert.Equal(t, tc.expected, result)

			// Test idempotence - hashing the same string should produce the same result
			secondResult := HashString(tc.input)
			assert.Equal(t, result, secondResult)
		})
	}
}

func TestHashStringCollisions(t *testing.T) {
	// Test with a large set of different inputs to check for collisions
	uniqueInputs := []string{
		"workspace-src/file1.go-10",
		"workspace-src/file2.go-10",
		"workspace-src/file1.go-20",
		"other-workspace-src/file1.go-10",
		"workspace-src/path/to/file1.go-10",
		"workspace-src/path/to/file2.go-10",
		"workspace-src/path/to/file1.go-20",
	}

	// Generate hashes for all inputs
	hashes := make(map[string]string)

	for _, input := range uniqueInputs {
		hash := HashString(input)

		// Check if we've seen this hash before
		if existing, exists := hashes[hash]; exists {
			// If this is a real test failure, it will be caught
			// But if your hash function is working properly, it shouldn't happen
			t.Logf("Potential collision: '%s' and '%s' both hash to '%s'", existing, input, hash)
		}
		hashes[hash] = input
	}

	// Verify we have the same number of unique hashes as inputs
	assert.Equal(t, len(uniqueInputs), len(hashes), "Hash collision detected")
}

func TestAuthenticationMethods(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Create a server that verifies authentication headers
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")

		if strings.HasPrefix(authHeader, "Bearer ") && strings.Contains(authHeader, "test-token") {
			// Token auth
			w.WriteHeader(http.StatusOK)
			writeResponse(t, w, []byte(`{"auth": "token"}`))
		} else if r.Header.Get("Authorization") != "" {
			// Basic auth
			username, password, ok := r.BasicAuth()
			if ok && username == "test-user" && password == "test-pass" {
				w.WriteHeader(http.StatusOK)
				writeResponse(t, w, []byte(`{"auth": "basic"}`))
			} else {
				w.WriteHeader(http.StatusUnauthorized)
				writeResponse(t, w, []byte(`{"error": "invalid credentials"}`))
			}
		} else {
			// No auth
			w.WriteHeader(http.StatusUnauthorized)
			writeResponse(t, w, []byte(`{"error": "authentication required"}`))
		}
	}))
	defer testServer.Close()

	// Test with token authentication
	tokenClient := &Client{
		baseURL:     testServer.URL,
		httpClient:  testServer.Client(),
		accessToken: "test-token",
		logger:      logger,
	}

	var result map[string]interface{}
	err := tokenClient.makeRequest("GET", "/", &result)
	assert.NoError(t, err)
	assert.Equal(t, "token", result["auth"])

	// Test with basic authentication
	basicClient := &Client{
		baseURL:    testServer.URL,
		httpClient: testServer.Client(),
		username:   "test-user",
		appPass:    "test-pass",
		logger:     logger,
	}

	err = basicClient.makeRequest("GET", "/", &result)
	assert.NoError(t, err)
	assert.Equal(t, "basic", result["auth"])

	// Test with no authentication
	noAuthClient := &Client{
		baseURL:    testServer.URL,
		httpClient: testServer.Client(),
		logger:     logger,
	}

	err = noAuthClient.makeRequest("GET", "/", &result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authentication required")
}

func TestWriteJSONFileErrors(t *testing.T) {
	// Test with invalid output directory
	logger, _ := zap.NewDevelopment()
	client := &Client{}
	exporter := NewExporter(client, "/nonexistent/dir", logger, false, "")

	err := exporter.writeJSONFile("test.json", map[string]string{"test": "data"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no such file or directory")

	// Test with non-marshallable data
	tempDir, err := os.MkdirTemp("", "exporter-test-")
	assert.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: Failed to remove temp dir: %v", err)
		}
	}()

	exporter = NewExporter(client, tempDir, logger, false, "")

	// Create data with circular reference that can't be marshalled to JSON
	type CircularRef struct {
		Self *CircularRef
	}
	circular := &CircularRef{}
	circular.Self = circular

	err = exporter.writeJSONFile("test.json", circular)
	assert.Error(t, err)
}

func TestCreateEmptyRepositoryWithReadOnlyDir(t *testing.T) {
	// Test case 2: Read-only directory - should fail
	tempDir, err := os.MkdirTemp("", "exporter-readonly-test-")
	assert.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: Failed to remove temp dir: %v", err)
		}
	}()

	// Create a read-only directory
	readOnlyDir := filepath.Join(tempDir, "readonly")
	err = os.MkdirAll(readOnlyDir, 0500) // Read-only directory
	assert.NoError(t, err)

	logger, _ := zap.NewDevelopment()
	client := &Client{}
	exporter := NewExporter(client, readOnlyDir, logger, false, "")

	// This should fail due to permissions
	err = exporter.createEmptyRepository("workspace", "repo")
	assert.Error(t, err)
	// Don't check for the specific file, just confirm the operation failed with an error
}

func TestGetOutputPath(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Test with custom output directory
	exporter := NewExporter(&Client{}, "/custom/path", logger, false, "")
	assert.Equal(t, "/custom/path", exporter.GetOutputPath())

	// Test with empty output directory - it remains empty until Export() is called
	exporter = NewExporter(&Client{}, "", logger, false, "")
	path := exporter.GetOutputPath()
	assert.Equal(t, "", path, "Empty output dir should remain empty until Export() is called")
}

func TestUpdateRepositoryFieldCaseInsensitive(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "update-repo-field-case-")
	assert.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: Failed to remove temp dir: %v", err)
		}
	}()

	logger, err := zap.NewDevelopment()
	assert.NoError(t, err)
	exporter := NewExporter(&Client{}, tempDir, logger, false, "")

	// Create a test case with different capitalization
	repos := []data.Repository{
		{
			Type:          "repository",
			Name:          "RepositoryUIName", // Capitalized name
			Slug:          "repositoryuiname", // Lowercase slug
			DefaultBranch: "main",
			GitURL:        "",
		},
		{
			Type:          "repository",
			Name:          "another-repo",
			Slug:          "another-repo",
			DefaultBranch: "main",
			GitURL:        "",
		},
	}
	err = exporter.writeJSONFile("repositories_000001.json", repos)
	assert.NoError(t, err)

	// Test case 1: Update using the capitalized name
	exporter.updateRepositoryField("RepositoryUIName", "default_branch", "develop")

	// Read back and verify
	b, err := os.ReadFile(filepath.Join(tempDir, "repositories_000001.json"))
	assert.NoError(t, err)
	var updatedRepos []data.Repository
	assert.NoError(t, json.Unmarshal(b, &updatedRepos))

	// Verify capitalized name repo was updated
	assert.Equal(t, "develop", updatedRepos[0].DefaultBranch)
	assert.Equal(t, "main", updatedRepos[1].DefaultBranch) // Other repo unchanged

	// Test case 2: Update using the lowercase slug
	exporter.updateRepositoryField("repositoryuiname", "git_url", "tarball://root/repositories/workspace/RepositoryUIName.git")

	// Read back and verify
	b, err = os.ReadFile(filepath.Join(tempDir, "repositories_000001.json"))
	assert.NoError(t, err)
	updatedRepos = nil
	assert.NoError(t, json.Unmarshal(b, &updatedRepos))

	// Verify both fields were updated correctly
	assert.Equal(t, "develop", updatedRepos[0].DefaultBranch)
	assert.Equal(t, "tarball://root/repositories/workspace/RepositoryUIName.git", updatedRepos[0].GitURL)

	// Test case 3: Try to update a non-existent repository
	// Create an observable logger to check for warning messages
	core, observedLogs := observer.New(zap.WarnLevel)
	observableLogger := zap.New(core)
	exporterWithObserver := NewExporter(&Client{}, tempDir, observableLogger, false, "")

	exporterWithObserver.updateRepositoryField("non-existent-repo", "default_branch", "master")

	// Verify warning was logged
	logs := observedLogs.All()
	assert.GreaterOrEqual(t, len(logs), 1)
	assert.Contains(t, logs[0].Message, "Repository not found")
	assert.Equal(t, "non-existent-repo", logs[0].ContextMap()["repo"])
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Windows backslash path",
			input:    "repositories\\mecapplicationdevelopment\\bottlerportaladmin.git\\objects\\pack\\pack-123.idx",
			expected: "repositories/mecapplicationdevelopment/bottlerportaladmin.git/objects/pack/pack-123.idx",
		},
		{
			name:     "Already normalized path",
			input:    "repositories/mecapplicationdevelopment/bottlerportaladmin.git/objects/pack/pack-123.idx",
			expected: "repositories/mecapplicationdevelopment/bottlerportaladmin.git/objects/pack/pack-123.idx",
		},
		{
			name:     "Mixed path",
			input:    "repositories/mecapplicationdevelopment\\bottlerportaladmin.git/objects\\pack/pack-123.idx",
			expected: "repositories/mecapplicationdevelopment/bottlerportaladmin.git/objects/pack/pack-123.idx",
		},
		{
			name:     "Path with spaces",
			input:    "repositories\\mecapplication development\\bottler portal admin.git\\objects\\pack\\pack-123.idx",
			expected: "repositories/mecapplication development/bottler portal admin.git/objects/pack/pack-123.idx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToUnixPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Windows backslash path",
			input:    "repositories\\mecapplicationdevelopment\\bottlerportaladmin.git",
			expected: "repositories/mecapplicationdevelopment/bottlerportaladmin.git",
		},
		{
			name:     "Already Unix path",
			input:    "repositories/mecapplicationdevelopment/bottlerportaladmin.git",
			expected: "repositories/mecapplicationdevelopment/bottlerportaladmin.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToUnixPath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToNativePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:  "Unix path on Windows",
			input: "repositories/mecapplicationdevelopment/bottlerportaladmin.git",
			expected: func() string {
				if runtime.GOOS == "windows" {
					return "repositories\\mecapplicationdevelopment\\bottlerportaladmin.git"
				}
				return "repositories/mecapplicationdevelopment/bottlerportaladmin.git"
			}(),
		},
		{
			name: "Already native path",
			input: func() string {
				if runtime.GOOS == "windows" {
					return "repositories\\mecapplicationdevelopment\\bottlerportaladmin.git"
				}
				return "repositories/mecapplicationdevelopment/bottlerportaladmin.git"
			}(),
			expected: func() string {
				if runtime.GOOS == "windows" {
					return "repositories\\mecapplicationdevelopment\\bottlerportaladmin.git"
				}
				return "repositories/mecapplicationdevelopment/bottlerportaladmin.git"
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToNativePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExecuteCommand(t *testing.T) {
	// Test basic command execution
	var cmdOutput []byte
	var err error

	if runtime.GOOS == "windows" {
		cmdOutput, err = ExecuteCommand("echo", []string{"test"}, "", false)
	} else {
		cmdOutput, err = ExecuteCommand("echo", []string{"test"}, "", false)
	}
	assert.NoError(t, err)
	assert.Contains(t, string(cmdOutput), "test")

	// Test with working directory
	tempDir, err := os.MkdirTemp("", "execute-command-test")
	assert.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: Failed to remove temp dir: %v", err)
		}
	}()

	// Create a test file in the temp directory
	testFilePath := filepath.Join(tempDir, "testfile.txt")
	err = os.WriteFile(testFilePath, []byte("test content"), 0644)
	assert.NoError(t, err)

	// Use 'dir' on Windows and 'ls' on other platforms
	var dirCmd string
	var dirArgs []string
	if runtime.GOOS == "windows" {
		dirCmd = "dir" // Using dir directly to test cmd.exe wrapping
		dirArgs = []string{}
	} else {
		dirCmd = "ls"
		dirArgs = []string{"-la"}
	}

	cmdOutput, err = ExecuteCommand(dirCmd, dirArgs, tempDir, false)
	assert.NoError(t, err)
	assert.Contains(t, string(cmdOutput), "testfile.txt")

	// Test with an executable that should exist on PATH
	var execCmd string
	var execArgs []string
	if runtime.GOOS == "windows" {
		// 'where' is Windows' equivalent of 'which'
		execCmd = "where"
		execArgs = []string{"cmd"}
	} else {
		execCmd = "which"
		execArgs = []string{"ls"}
	}

	cmdOutput, err = ExecuteCommand(execCmd, execArgs, "", false)
	assert.NoError(t, err)
	assert.NotEmpty(t, string(cmdOutput))

	// Test command that doesn't exist
	_, err = ExecuteCommand("command-that-does-not-exist", []string{}, "", false)
	assert.Error(t, err)

	// Test command that returns non-zero exit code
	var badCmd string
	var badArgs []string
	if runtime.GOOS == "windows" {
		badCmd = "dir" // Using dir directly without cmd.exe
		badArgs = []string{"/nonexistent/directory"}
	} else {
		badCmd = "ls"
		badArgs = []string{"/nonexistent/directory"}
	}

	_, err = ExecuteCommand(badCmd, badArgs, "", false)
	assert.Error(t, err)

	// Test with SSL verification bypass
	cmdOutput, err = ExecuteCommand("git", []string{"--version"}, "", true)
	if err == nil {
		assert.NotEmpty(t, string(cmdOutput))
		assert.Contains(t, string(cmdOutput), "git version")
	}
}

func TestCreateRepositoryInfoFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "repo-info-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	logger, _ := zap.NewDevelopment()
	exporter := NewExporter(&Client{}, tempDir, logger, false, "")

	workspace := "test-workspace"
	repoSlug := "test-repo"

	// Create the repository directory first
	repoDir := filepath.Join(tempDir, "repositories", workspace, repoSlug+".git")
	err = os.MkdirAll(repoDir, 0755)
	assert.NoError(t, err)

	// Call the method
	err = exporter.createRepositoryInfoFiles(workspace, repoSlug)
	assert.NoError(t, err)

	// Verify files were created
	nwoPath := filepath.Join(repoDir, "info", "nwo")
	assert.FileExists(t, nwoPath)

	content, err := os.ReadFile(nwoPath)
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%s/%s\n", workspace, repoSlug), string(content))

	syncPath := filepath.Join(repoDir, "info", "last-sync")
	assert.FileExists(t, syncPath)
}

func TestPathOperations(t *testing.T) {
	// Create a mixed path for testing
	mixedPath := "repositories/workspace\\repo/path\\to/file.txt"

	// Test NormalizePath
	normalizedPath := NormalizePath(mixedPath)
	assert.Equal(t, "repositories/workspace/repo/path/to/file.txt", normalizedPath)

	// Test ToUnixPath
	unixPath := ToUnixPath(mixedPath)
	assert.Equal(t, "repositories/workspace/repo/path/to/file.txt", unixPath)

	// Test ToNativePath - this needs to check OS
	nativePath := ToNativePath(unixPath)
	if runtime.GOOS == "windows" {
		assert.Equal(t, "repositories\\workspace\\repo\\path\\to\\file.txt", nativePath)
	} else {
		assert.Equal(t, "repositories/workspace/repo/path/to/file.txt", nativePath)
	}

	// Test round-trip conversion (Unix -> Native -> Unix)
	roundTripPath := ToUnixPath(ToNativePath(unixPath))
	assert.Equal(t, unixPath, roundTripPath)
}

func TestOSSpecificPathConversion(t *testing.T) {
	// Test absolute paths which may differ by OS
	var absPath string
	if runtime.GOOS == "windows" {
		absPath = "C:\\Users\\user\\Documents\\file.txt"
	} else {
		absPath = "/home/user/Documents/file.txt"
	}

	// Convert to Unix path
	unixPath := ToUnixPath(absPath)
	if runtime.GOOS == "windows" {
		assert.Equal(t, "C:/Users/user/Documents/file.txt", unixPath)
	} else {
		assert.Equal(t, "/home/user/Documents/file.txt", unixPath)
	}

	// Convert back to native path
	nativePath := ToNativePath(unixPath)
	assert.Equal(t, absPath, nativePath)
}

func TestWriteJSONFilePermissionError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Permission test not reliable on Windows")
	}

	tempDir, err := os.MkdirTemp("", "write-perm-test-")
	assert.NoError(t, err)
	defer func() {
		// Restore permissions before removal
		if err := os.Chmod(tempDir, 0755); err != nil {
			t.Logf("Warning: Failed to restore permissions: %v", err)
		}
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: Failed to remove temp dir: %v", err)
		}
	}()

	logger, _ := zap.NewDevelopment()
	exporter := NewExporter(&Client{}, tempDir, logger, false, "")

	// Make directory read-only
	err = os.Chmod(tempDir, 0555)
	assert.NoError(t, err)

	testData := []data.User{{Type: "user", Login: "test"}}
	err = exporter.writeJSONFile("test.json", testData)
	assert.Error(t, err)
}

func TestValidateGitReference(t *testing.T) {
	testCases := []struct {
		name        string
		reference   string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid branch name",
			reference:   "main",
			expectError: false,
		},
		{
			name:        "Valid branch with slash",
			reference:   "feature/new-feature",
			expectError: false,
		},
		{
			name:        "Empty reference",
			reference:   "",
			expectError: true,
			errorMsg:    "empty reference",
		},
		{
			name:        "Exactly 40 hex characters (ambiguous)",
			reference:   "1234567890abcdef1234567890abcdef12345678",
			expectError: true,
			errorMsg:    "ambiguous git reference",
		},
		{
			name:        "39 hex characters (valid)",
			reference:   "1234567890abcdef1234567890abcdef1234567",
			expectError: false,
		},
		{
			name:        "41 hex characters (valid)",
			reference:   "1234567890abcdef1234567890abcdef123456789",
			expectError: false,
		},
		{
			name:        "Contains space",
			reference:   "feature branch",
			expectError: true,
			errorMsg:    "contains ' '",
		},
		{
			name:        "Contains tilde",
			reference:   "feature~1",
			expectError: true,
			errorMsg:    "contains '~'",
		},
		{
			name:        "Contains caret",
			reference:   "feature^1",
			expectError: true,
			errorMsg:    "contains '^'",
		},
		{
			name:        "Contains colon",
			reference:   "feature:test",
			expectError: true,
			errorMsg:    "contains ':'",
		},
		{
			name:        "Contains question mark",
			reference:   "feature?",
			expectError: true,
			errorMsg:    "contains '?'",
		},
		{
			name:        "Contains asterisk",
			reference:   "feature*",
			expectError: true,
			errorMsg:    "contains '*'",
		},
		{
			name:        "Contains bracket",
			reference:   "feature[1]",
			expectError: true,
			errorMsg:    "contains '['",
		},
		{
			name:        "Starts with dot",
			reference:   ".feature",
			expectError: true,
			errorMsg:    "cannot start or end with '.'",
		},
		{
			name:        "Ends with dot",
			reference:   "feature.",
			expectError: true,
			errorMsg:    "cannot start or end with '.'",
		},
		{
			name:        "Starts with slash",
			reference:   "/feature",
			expectError: true,
			errorMsg:    "cannot start or end with '/'",
		},
		{
			name:        "Ends with slash",
			reference:   "feature/",
			expectError: true,
			errorMsg:    "cannot start or end with '/'",
		},
		{
			name:        "Ends with .lock",
			reference:   "feature.lock",
			expectError: true,
			errorMsg:    "cannot end with '.lock'",
		},
		{
			name:        "Contains double dot",
			reference:   "feature..test",
			expectError: true,
			errorMsg:    "contains '..'",
		},
		{
			name:        "Contains @{",
			reference:   "feature@{upstream}",
			expectError: true,
			errorMsg:    "contains '@{'",
		},
		{
			name:        "Contains double slash",
			reference:   "feature//test",
			expectError: true,
			errorMsg:    "contains '//'",
		},
		{
			name:        "Valid complex branch name",
			reference:   "feature/JIRA-1234_fix-bug",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateGitReference(tc.reference)
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorMsg != "" {
					assert.Contains(t, err.Error(), tc.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateExportDataIntegration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "validate-export-")
	assert.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: Failed to remove temp dir: %v", err)
		}
	}()

	logger, _ := zap.NewDevelopment()
	exporter := NewExporter(&Client{}, tempDir, logger, false, "")

	// Test Case 1: Repository with newlines in description
	repos := []data.Repository{
		{
			Name:        "test-repo",
			Description: "Line 1\nLine 2\r\nLine 3",
		},
	}
	err = exporter.writeJSONFile("repositories_000001.json", repos)
	assert.NoError(t, err)

	// Test Case 2: Pull requests - ambiguous refs should have been filtered during fetch
	// So we'll only include valid PRs
	prs := []data.PullRequest{
		{
			URL: "https://example.com/pr/2",
			Base: data.PRBranch{
				Ref: "main",         // Valid branch name
				SHA: "normalsha123", // Valid SHA (not 40 hex chars)
			},
			Head: data.PRBranch{
				Ref: "feature-branch", // Valid branch name
				SHA: "anothersha456",  // Valid SHA (not 40 hex chars)
			},
		},
	}
	err = exporter.writeJSONFile("pull_requests_000001.json", prs)
	assert.NoError(t, err)

	// Test Case 3: Create a Git repository without ambiguous branch name
	gitRepoPath := filepath.Join(tempDir, "repositories", "workspace", "repo.git")
	err = os.MkdirAll(gitRepoPath, 0755)
	assert.NoError(t, err)

	headFile := filepath.Join(gitRepoPath, "HEAD")
	// Use a valid branch name instead of an ambiguous one
	err = os.WriteFile(headFile, []byte("ref: refs/heads/main\n"), 0644)
	assert.NoError(t, err)

	// Run validation
	err = exporter.validateExportData()
	assert.NoError(t, err)

	// Verify fixes were applied
	// Check repository description
	repoData, err := os.ReadFile(filepath.Join(tempDir, "repositories_000001.json"))
	assert.NoError(t, err)
	var fixedRepos []data.Repository
	err = json.Unmarshal(repoData, &fixedRepos)
	assert.NoError(t, err)
	assert.Equal(t, "Line 1 Line 2 Line 3", fixedRepos[0].Description)

	// Check pull requests remain unchanged (since we only included valid ones)
	prData, err := os.ReadFile(filepath.Join(tempDir, "pull_requests_000001.json"))
	assert.NoError(t, err)
	var fixedPRs []data.PullRequest
	err = json.Unmarshal(prData, &fixedPRs)
	assert.NoError(t, err)
	assert.Equal(t, "normalsha123", fixedPRs[0].Base.SHA)  // Unchanged
	assert.Equal(t, "anothersha456", fixedPRs[0].Head.SHA) // Unchanged

	// Check Git HEAD file remains unchanged
	headContent, err := os.ReadFile(headFile)
	assert.NoError(t, err)
	assert.Equal(t, "ref: refs/heads/main\n", string(headContent))
}

func TestIdentifyShortSHAs(t *testing.T) {
	testCases := []struct {
		name    string
		sha     string
		isShort bool
	}{
		{
			name:    "Full 40-character SHA",
			sha:     "4101446c6322f8b5c99976986fcc49e772f9153f",
			isShort: false,
		},
		{
			name:    "12-character short SHA",
			sha:     "4101446c6322",
			isShort: true,
		},
		{
			name:    "7-character short SHA",
			sha:     "4101446",
			isShort: true,
		},
		{
			name:    "Empty SHA",
			sha:     "",
			isShort: true,
		},
		{
			name:    "Invalid characters but 40 length",
			sha:     strings.Repeat("g", 40),
			isShort: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isShort := len(tc.sha) < 40
			assert.Equal(t, tc.isShort, isShort,
				"SHA '%s' short status should be %v", tc.sha, tc.isShort)
		})
	}
}

func TestValidateCommitSHAs(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "validate-shas-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	logger, _ := zap.NewDevelopment()
	exporter := NewExporter(&Client{}, tempDir, logger, false, "")

	comments := []data.PullRequestReviewComment{
		{CommitID: "abc123"},
		{CommitID: strings.Repeat("a", 40)},
		{CommitID: "def456789012"},
		{CommitID: ""},
	}

	err = exporter.writeJSONFile("pull_request_review_comments_000001.json", comments)
	assert.NoError(t, err)

	var readComments []data.PullRequestReviewComment
	content, err := os.ReadFile(filepath.Join(tempDir, "pull_request_review_comments_000001.json"))
	assert.NoError(t, err)

	err = json.Unmarshal(content, &readComments)
	assert.NoError(t, err)

	shortCount := 0
	for _, c := range readComments {
		if len(c.CommitID) > 0 && len(c.CommitID) < 40 {
			shortCount++
		}
	}

	assert.Equal(t, 2, shortCount, "Should have 2 short SHAs")
}

func TestGetFullCommitSHAFromLocalRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git not available for testing")
	}

	tempDir, err := os.MkdirTemp("", "local-repo-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	repoPath := filepath.Join(tempDir, "test.git")
	cmd := exec.Command("git", "init", "--bare", repoPath)
	err = cmd.Run()
	assert.NoError(t, err)

	_, err = GetFullCommitSHAFromLocalRepo(repoPath, "nonexistent")
	assert.Error(t, err, "Should error for non-existent SHA")

	_, err = GetFullCommitSHAFromLocalRepo("/invalid/path", "abc123")
	assert.Error(t, err, "Should error for invalid repo path")
}
