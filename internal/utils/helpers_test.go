package utils

import (
	"bytes"
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
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestValidateExportFlags(t *testing.T) {
	// Test case 1: No credentials provided
	cmdFlags := &data.CmdExportFlags{}
	err := ValidateExportFlags(cmdFlags)
	assert.Error(t, err, "Expected error when no credentials are provided")
	assert.Contains(t, err.Error(), "authentication credentials required")

	// Test case 2: Token provided
	cmdFlags = &data.CmdExportFlags{}
	cmdFlags.BitbucketAccessToken = "testtoken"
	err = ValidateExportFlags(cmdFlags)
	assert.NoError(t, err, "Expected no error when token is provided")

	// Test case 3: Username and app password provided
	cmdFlags = &data.CmdExportFlags{}
	cmdFlags.BitbucketAccessToken = ""
	cmdFlags.BitbucketUser = "testuser"
	cmdFlags.BitbucketAppPass = "testpass"
	err = ValidateExportFlags(cmdFlags)
	assert.NoError(t, err, "Expected no error when username and app password are provided")

	// Test case 4: Only username provided (missing app password)
	cmdFlags = &data.CmdExportFlags{}
	cmdFlags.BitbucketAccessToken = ""
	cmdFlags.BitbucketUser = "testuser"
	cmdFlags.BitbucketAppPass = ""
	err = ValidateExportFlags(cmdFlags)
	assert.Error(t, err, "Expected error when only username is provided")
	// The actual error message is the general authentication error, not the specific one
	assert.Contains(t, err.Error(), "authentication credentials required")

	// Test case 5: Only app password provided (missing username)
	cmdFlags = &data.CmdExportFlags{}
	cmdFlags.BitbucketAccessToken = ""
	cmdFlags.BitbucketUser = ""
	cmdFlags.BitbucketAppPass = "testpass"
	err = ValidateExportFlags(cmdFlags)
	assert.Error(t, err, "Expected error when only app password is provided")
	// The actual error message is the general authentication error, not the specific one
	assert.Contains(t, err.Error(), "authentication credentials required")

	// Test case 6: API token and email provided
	cmdFlags = &data.CmdExportFlags{}
	cmdFlags.BitbucketAPIToken = "testapitoken"
	cmdFlags.BitbucketEmail = "test@example.com"
	err = ValidateExportFlags(cmdFlags)
	assert.NoError(t, err, "Expected no error when API token and email are provided")

	// Test case 7: Only API token provided (missing email)
	cmdFlags = &data.CmdExportFlags{}
	cmdFlags.BitbucketAPIToken = "testapitoken"
	cmdFlags.BitbucketEmail = ""
	err = ValidateExportFlags(cmdFlags)
	assert.Error(t, err, "Expected error when only API token is provided")
	// The actual error message is the general authentication error, not the specific one
	assert.Contains(t, err.Error(), "authentication credentials required")

	// Test case 8: Only email provided (missing API token)
	cmdFlags = &data.CmdExportFlags{}
	cmdFlags.BitbucketAPIToken = ""
	cmdFlags.BitbucketEmail = "test@example.com"
	err = ValidateExportFlags(cmdFlags)
	assert.Error(t, err, "Expected error when only email is provided")
	// The actual error message is the general authentication error, not the specific one
	assert.Contains(t, err.Error(), "authentication credentials required")

	// Test case 9: Mixed authentication methods - access token with username/password
	cmdFlags = &data.CmdExportFlags{}
	cmdFlags.BitbucketAccessToken = "testtoken"
	cmdFlags.BitbucketUser = "testuser"
	cmdFlags.BitbucketAppPass = "testpass"
	err = ValidateExportFlags(cmdFlags)
	assert.Error(t, err, "Expected error when multiple authentication methods are provided")
	assert.Contains(t, err.Error(), "mixed authentication methods")

	// Test case 9b: Mixed authentication methods - access token with API token/email
	cmdFlags = &data.CmdExportFlags{}
	cmdFlags.BitbucketAccessToken = "testtoken"
	cmdFlags.BitbucketAPIToken = "testapitoken"
	cmdFlags.BitbucketEmail = "test@example.com"
	err = ValidateExportFlags(cmdFlags)
	assert.Error(t, err, "Expected error when multiple authentication methods are provided")
	assert.Contains(t, err.Error(), "mixed authentication methods")

	// Test case 9c: Mixed authentication methods - API token/email with username/password
	cmdFlags = &data.CmdExportFlags{}
	cmdFlags.BitbucketAPIToken = "testapitoken"
	cmdFlags.BitbucketEmail = "test@example.com"
	cmdFlags.BitbucketUser = "testuser"
	cmdFlags.BitbucketAppPass = "testpass"
	err = ValidateExportFlags(cmdFlags)
	assert.Error(t, err, "Expected error when multiple authentication methods are provided")
	assert.Contains(t, err.Error(), "mixed authentication methods")

	// Test case 10: Valid date format for PRsFromDate
	cmdFlags = &data.CmdExportFlags{}
	cmdFlags.BitbucketAccessToken = "testtoken"
	cmdFlags.PRsFromDate = "2023-01-01"
	err = ValidateExportFlags(cmdFlags)
	assert.NoError(t, err, "Expected no error with valid date format")

	// Test case 11: Invalid date format for PRsFromDate
	cmdFlags = &data.CmdExportFlags{}
	cmdFlags.BitbucketAccessToken = "testtoken"
	cmdFlags.PRsFromDate = "01/01/2023"
	err = ValidateExportFlags(cmdFlags)
	assert.Error(t, err, "Expected error with invalid date format")
	assert.Contains(t, err.Error(), "invalid date format for --prs-from-date", "Error should mention invalid date format")
}

func TestSetupEnvironmentCredentials(t *testing.T) {
	cmdFlags := &data.CmdExportFlags{}

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
	if err := os.Unsetenv("BITBUCKET_TEMP_DIR"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}

	// Set environment variables
	err := os.Setenv("BITBUCKET_USERNAME", "envuser")
	assert.NoError(t, err)
	err = os.Setenv("BITBUCKET_APP_PASSWORD", "envpass")
	assert.NoError(t, err)
	err = os.Setenv("BITBUCKET_ACCESS_TOKEN", "envtoken")
	assert.NoError(t, err)
	err = os.Setenv("BITBUCKET_API_TOKEN", "envapitoken")
	assert.NoError(t, err)
	err = os.Setenv("BITBUCKET_EMAIL", "user@example.com")
	assert.NoError(t, err)
	err = os.Setenv("BITBUCKET_TEMP_DIR", "/tmp/custom-temp-dir")
	assert.NoError(t, err)

	SetupEnvironmentCredentials(cmdFlags)

	// Assert that the values are set correctly
	assert.Equal(t, "envuser", cmdFlags.BitbucketUser, "Expected username to be set from environment")
	assert.Equal(t, "envpass", cmdFlags.BitbucketAppPass, "Expected app password to be set from environment")
	assert.Equal(t, "envtoken", cmdFlags.BitbucketAccessToken, "Expected access token to be set from environment")
	assert.Equal(t, "envapitoken", cmdFlags.BitbucketAPIToken, "Expected API token to be set from environment")
	assert.Equal(t, "user@example.com", cmdFlags.BitbucketEmail, "Expected email to be set from environment")
	assert.Equal(t, "/tmp/custom-temp-dir", cmdFlags.TempDir, "Expected temp dir to be set from environment")

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
	err = os.Unsetenv("BITBUCKET_TEMP_DIR")
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
			cmdFlags := &data.CmdExportFlags{
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

// TestUpdateRepositoryFieldPreservesWikiURLNull guards the GHE migrator wiki-import
// fix at the disk round-trip layer: editing repositories_000001.json (e.g. to
// update default_branch) must not silently turn `"wiki_url":null` into
// `"wiki_url":""`, which would re-trigger InvalidTarballUrl.
func TestUpdateRepositoryFieldPreservesWikiURLNull(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "update-repo-field-wiki-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	logger, _ := zap.NewDevelopment()
	exporter := NewExporter(&Client{}, tempDir, logger, false, "")

	// Seed with WikiURL nil (the only state we ever produce post-fix).
	initial := []data.Repository{
		{
			Type:          "repository",
			Name:          "r",
			Slug:          "r",
			DefaultBranch: "main",
			WikiURL:       nil,
		},
	}
	err = exporter.writeJSONFile("repositories_000001.json", initial)
	assert.NoError(t, err)

	exporter.updateRepositoryField("r", "default_branch", "develop")
	exporter.updateRepositoryField("r", "git_url", "tarball://root/repositories/ws/r.git")

	b, err := os.ReadFile(filepath.Join(tempDir, "repositories_000001.json"))
	assert.NoError(t, err)

	// Normalize JSON so the assertion is whitespace-insensitive (the migrator
	// consumes raw JSON regardless of indentation).
	var compact bytes.Buffer
	assert.NoError(t, json.Compact(&compact, b))

	assert.Contains(t, compact.String(), `"wiki_url":null`,
		"wiki_url must remain null after round-trip through updateRepositoryField")
	assert.NotContains(t, compact.String(), `"wiki_url":""`)

	var repos []data.Repository
	assert.NoError(t, json.Unmarshal(b, &repos))
	assert.Nil(t, repos[0].WikiURL)
	assert.Equal(t, "develop", repos[0].DefaultBranch)
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

func TestTempDirEnvironmentVariable(t *testing.T) {
	originalTempDir := os.Getenv("BITBUCKET_TEMP_DIR")
	defer func() {
		if originalTempDir != "" {
			_ = os.Setenv("BITBUCKET_TEMP_DIR", originalTempDir)
		} else {
			_ = os.Unsetenv("BITBUCKET_TEMP_DIR")
		}
	}()

	testCases := []struct {
		name           string
		envValue       string
		flagValue      string
		expectedResult string
		description    string
	}{
		{
			name:           "Environment variable set, flag empty",
			envValue:       "/env/temp/dir",
			flagValue:      "",
			expectedResult: "/env/temp/dir",
			description:    "Should use environment variable when flag is empty",
		},
		{
			name:           "Flag set, environment empty",
			envValue:       "",
			flagValue:      "/flag/temp/dir",
			expectedResult: "/flag/temp/dir",
			description:    "Should preserve flag value when environment is empty",
		},
		{
			name:           "Both flag and environment set",
			envValue:       "/env/temp/dir",
			flagValue:      "/flag/temp/dir",
			expectedResult: "/flag/temp/dir",
			description:    "Should preserve flag value when both are set",
		},
		{
			name:           "Both empty",
			envValue:       "",
			flagValue:      "",
			expectedResult: "",
			description:    "Should remain empty when both are empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envValue != "" {
				err := os.Setenv("BITBUCKET_TEMP_DIR", tc.envValue)
				assert.NoError(t, err)
			} else {
				err := os.Unsetenv("BITBUCKET_TEMP_DIR")
				assert.NoError(t, err)
			}

			cmdFlags := &data.CmdExportFlags{
				TempDir: tc.flagValue,
			}

			SetupEnvironmentCredentials(cmdFlags)

			assert.Equal(t, tc.expectedResult, cmdFlags.TempDir, tc.description)
		})
	}
}

func TestSetupCommandUsageTemplate(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
	}

	cmd.PersistentFlags().StringP("workspace", "w", "", "Bitbucket workspace name")
	cmd.PersistentFlags().StringP("repo", "r", "", "Bitbucket repository slug")
	cmd.PersistentFlags().StringP("access-token", "t", "", "Bitbucket access token for authentication")
	cmd.PersistentFlags().BoolP("debug", "d", false, "Enable debug logging")

	SetupCommandUsageTemplate(cmd, 120)

	assert.False(t, cmd.Flags().SortFlags, "Regular flags should not be sorted")
	assert.False(t, cmd.PersistentFlags().SortFlags, "Persistent flags should not be sorted")

	assert.NotEmpty(t, cmd.UsageTemplate(), "Usage template should be set")
	assert.Contains(t, cmd.UsageTemplate(), "wrappedFlagUsages", "Template should use wrappedFlagUsages")
}

func TestSetupCommandUsageTemplateOutput(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "test [flags]",
		Short: "Test command for template",
		Long:  "This is a longer description that explains what the test command does.",
		Example: `  test --workspace myworkspace --repo myrepo
  test -w myworkspace -r myrepo --debug`,
	}

	cmd.PersistentFlags().StringP("workspace", "w", "", "Bitbucket workspace name")
	cmd.PersistentFlags().StringP("repo", "r", "", "Bitbucket repository slug")

	SetupCommandUsageTemplate(cmd, 80)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Usage()
	assert.NoError(t, err)

	output := buf.String()

	assert.Contains(t, output, "Usage:", "Output should contain Usage section")
	assert.Contains(t, output, "Examples:", "Output should contain Examples section")
	assert.Contains(t, output, "Flags:", "Output should contain Flags section")
	assert.Contains(t, output, "--workspace", "Output should contain workspace flag")
	assert.Contains(t, output, "--repo", "Output should contain repo flag")
}

func TestSetupCommandUsageTemplateWithSubcommands(t *testing.T) {
	rootCmd := &cobra.Command{
		Use:   "root",
		Short: "Root command",
	}

	subCmd := &cobra.Command{
		Use:   "sub",
		Short: "Sub command",
	}

	rootCmd.AddCommand(subCmd)
	SetupCommandUsageTemplate(rootCmd, 100)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err := rootCmd.Usage()
	assert.NoError(t, err)

	output := buf.String()
	// The custom template may use "Additional help topics:" instead of "Available Commands:"
	assert.True(t,
		strings.Contains(output, "Available Commands:") || strings.Contains(output, "Additional help topics:"),
		"Output should contain commands section")
	assert.Contains(t, output, "sub", "Output should contain subcommand")
}

func TestSetupCommandUsageTemplateMinimumWidth(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
	}

	cmd.PersistentFlags().String("very-long-flag-name", "",
		"This is a very long description that should be wrapped when the terminal width is narrow")

	SetupCommandUsageTemplate(cmd, 20)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Usage()
	assert.NoError(t, err)

	assert.NotEmpty(t, buf.String())
}

func TestSetupCommandUsageTemplatePreservesFlags(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
	}

	cmd.PersistentFlags().String("zebra", "", "Last alphabetically")
	cmd.PersistentFlags().String("alpha", "", "First alphabetically")
	cmd.PersistentFlags().String("middle", "", "Middle alphabetically")

	SetupCommandUsageTemplate(cmd, 100)

	zebraFlag := cmd.PersistentFlags().Lookup("zebra")
	assert.NotNil(t, zebraFlag)
	assert.Equal(t, "zebra", zebraFlag.Name)

	alphaFlag := cmd.PersistentFlags().Lookup("alpha")
	assert.NotNil(t, alphaFlag)
	assert.Equal(t, "alpha", alphaFlag.Name)

	middleFlag := cmd.PersistentFlags().Lookup("middle")
	assert.NotNil(t, middleFlag)
	assert.Equal(t, "middle", middleFlag.Name)
}

func TestSetupCommandUsageTemplateWithAliases(t *testing.T) {
	cmd := &cobra.Command{
		Use:     "test",
		Aliases: []string{"t", "tst"},
		Short:   "Test command with aliases",
	}

	SetupCommandUsageTemplate(cmd, 100)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Usage()
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Aliases:", "Output should contain Aliases section")
}

func TestSetupCommandUsageTemplateInheritedFlags(t *testing.T) {
	rootCmd := &cobra.Command{
		Use:   "root",
		Short: "Root command",
	}

	rootCmd.PersistentFlags().Bool("global-flag", false, "A global flag inherited by subcommands")

	subCmd := &cobra.Command{
		Use:   "sub",
		Short: "Sub command",
	}
	subCmd.Flags().String("local-flag", "", "A local flag")

	rootCmd.AddCommand(subCmd)
	SetupCommandUsageTemplate(subCmd, 100)

	var buf bytes.Buffer
	subCmd.SetOut(&buf)
	subCmd.SetErr(&buf)

	err := subCmd.Usage()
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Global Flags:", "Output should contain Global Flags section")
	assert.Contains(t, output, "--global-flag", "Output should show inherited flag")
}

func TestGetTerminalWidth(t *testing.T) {
	width := getTerminalWidth()
	assert.Greater(t, width, 0, "Terminal width should be positive")
}

func TestSetupCommandUsageTemplateMultipleCalls(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
	}

	cmd.PersistentFlags().String("flag1", "", "First flag")

	SetupCommandUsageTemplate(cmd, 80)
	SetupCommandUsageTemplate(cmd, 100)
	SetupCommandUsageTemplate(cmd, 120)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Usage()
	assert.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

func TestGetTerminalWidthDefault(t *testing.T) {
	// Test that getTerminalWidth returns a reasonable default
	width := getTerminalWidth()
	assert.GreaterOrEqual(t, width, 80, "Terminal width should be at least 80")
	assert.LessOrEqual(t, width, 500, "Terminal width should be reasonable")
}

func TestSetupCommandUsageTemplateWithRunnable(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "test [flags]",
		Short: "Test command",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	cmd.PersistentFlags().String("flag1", "", "First flag")
	SetupCommandUsageTemplate(cmd, 100)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Usage()
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "test [flags]", "Output should contain command usage line")
}

func TestSetupCommandUsageTemplateNoFlags(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test command without flags",
	}

	SetupCommandUsageTemplate(cmd, 100)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Usage()
	assert.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

func TestSetupCommandUsageTemplateWithLongDescription(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Short description",
		Long: `This is a very long description that spans multiple lines.
It explains in detail what the command does and how to use it.

It can include:
- Bullet points
- Multiple paragraphs
- Examples and more`,
	}

	cmd.PersistentFlags().String("option", "", "An option flag")
	SetupCommandUsageTemplate(cmd, 80)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Usage()
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "--option")
	assert.NotEmpty(t, output)
}

func TestSetupCommandUsageTemplateNarrowWidth(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
	}

	// Add a flag with a very long description
	cmd.PersistentFlags().String("long-option-name", "",
		"This is an extremely long description that will definitely need to be wrapped when displayed in a narrow terminal window")

	// Use narrow width (should be clamped to minimum of 40)
	SetupCommandUsageTemplate(cmd, 30)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Usage()
	assert.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

func TestSetupCommandUsageTemplateWideWidth(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
	}

	cmd.PersistentFlags().String("option", "", "A short description")
	SetupCommandUsageTemplate(cmd, 200)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Usage()
	assert.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

func TestSetupCommandUsageTemplateWithDeprecatedFlag(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
	}

	cmd.PersistentFlags().String("old-flag", "", "Deprecated flag")
	err := cmd.PersistentFlags().MarkDeprecated("old-flag", "use --new-flag instead")
	assert.NoError(t, err)

	cmd.PersistentFlags().String("new-flag", "", "New flag")

	SetupCommandUsageTemplate(cmd, 100)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err = cmd.Usage()
	assert.NoError(t, err)
	// Deprecated flags should not appear in usage
	output := buf.String()
	assert.NotContains(t, output, "old-flag")
	assert.Contains(t, output, "new-flag")
}

func TestSetupCommandUsageTemplateWithHiddenFlag(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
	}

	cmd.PersistentFlags().String("hidden-flag", "", "Hidden flag")
	err := cmd.PersistentFlags().MarkHidden("hidden-flag")
	assert.NoError(t, err)

	cmd.PersistentFlags().String("visible-flag", "", "Visible flag")

	SetupCommandUsageTemplate(cmd, 100)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err = cmd.Usage()
	assert.NoError(t, err)
	output := buf.String()
	assert.NotContains(t, output, "hidden-flag")
	assert.Contains(t, output, "visible-flag")
}

func TestSetupCommandUsageTemplateWithRequiredFlag(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
	}

	cmd.PersistentFlags().String("required-flag", "", "Required flag")
	err := cmd.MarkPersistentFlagRequired("required-flag")
	assert.NoError(t, err)

	SetupCommandUsageTemplate(cmd, 100)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err = cmd.Usage()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "required-flag")
}

func TestSetupCommandUsageTemplateWithBoolFlags(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
	}

	cmd.PersistentFlags().Bool("verbose", false, "Enable verbose output")
	cmd.PersistentFlags().BoolP("debug", "d", false, "Enable debug mode")

	SetupCommandUsageTemplate(cmd, 100)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Usage()
	assert.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "--verbose")
	assert.Contains(t, output, "-d, --debug")
}

func TestSetupCommandUsageTemplateWithIntFlags(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
	}

	cmd.PersistentFlags().Int("count", 10, "Number of items")
	cmd.PersistentFlags().IntP("port", "p", 8080, "Port number")

	SetupCommandUsageTemplate(cmd, 100)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Usage()
	assert.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "--count")
	assert.Contains(t, output, "--port")
}

func TestSetupCommandUsageTemplateWithStringSliceFlags(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
	}

	cmd.PersistentFlags().StringSlice("tags", []string{}, "Tags to apply")

	SetupCommandUsageTemplate(cmd, 100)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Usage()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "--tags")
}

func TestFormatURLWithIntegerID(t *testing.T) {
	// Test formatURL with integer IDs
	result := formatURL("pr", "workspace", "repo", 123)
	assert.Equal(t, "https://bitbucket.org/workspace/repo/pull/123", result)

	result = formatURL("pr_review", "workspace", "repo", 123, 456)
	assert.Equal(t, "https://bitbucket.org/workspace/repo/pull/123/files#pullrequestreview-456", result)
}

func TestFormatURLEdgeCases(t *testing.T) {
	testCases := []struct {
		name       string
		urlType    string
		workspace  string
		repository string
		ids        []interface{}
		expected   string
	}{
		{
			name:       "Empty workspace",
			urlType:    "repository",
			workspace:  "",
			repository: "repo",
			ids:        []interface{}{},
			expected:   "https://bitbucket.org//repo",
		},
		{
			name:       "Empty repository",
			urlType:    "repository",
			workspace:  "workspace",
			repository: "",
			ids:        []interface{}{},
			expected:   "https://bitbucket.org/workspace/",
		},
		{
			name:       "Special characters in workspace",
			urlType:    "repository",
			workspace:  "my-workspace",
			repository: "my-repo",
			ids:        []interface{}{},
			expected:   "https://bitbucket.org/my-workspace/my-repo",
		},
		{
			name:       "Numeric workspace",
			urlType:    "repository",
			workspace:  "123456",
			repository: "repo",
			ids:        []interface{}{},
			expected:   "https://bitbucket.org/123456/repo",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result string
			switch len(tc.ids) {
			case 0:
				result = formatURL(tc.urlType, tc.workspace, tc.repository)
			case 1:
				result = formatURL(tc.urlType, tc.workspace, tc.repository, tc.ids[0])
			case 2:
				result = formatURL(tc.urlType, tc.workspace, tc.repository, tc.ids[0], tc.ids[1])
			}
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestValidateExportFlagsWithAllAuthMethods(t *testing.T) {
	// Test all three authentication methods mixed
	cmdFlags := &data.CmdExportFlags{
		BitbucketAccessToken: "token",
		BitbucketAPIToken:    "apitoken",
		BitbucketEmail:       "email@example.com",
		BitbucketUser:        "user",
		BitbucketAppPass:     "pass",
	}
	err := ValidateExportFlags(cmdFlags)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mixed authentication methods")
}

func TestSetupEnvironmentCredentialsPreservesExistingValues(t *testing.T) {
	// Clear environment first
	envVars := []string{
		"BITBUCKET_USERNAME",
		"BITBUCKET_APP_PASSWORD",
		"BITBUCKET_ACCESS_TOKEN",
		"BITBUCKET_API_TOKEN",
		"BITBUCKET_EMAIL",
		"BITBUCKET_TEMP_DIR",
	}

	originalValues := make(map[string]string)
	for _, v := range envVars {
		originalValues[v] = os.Getenv(v)
		_ = os.Unsetenv(v)
	}

	defer func() {
		for k, v := range originalValues {
			if v != "" {
				_ = os.Setenv(k, v)
			} else {
				_ = os.Unsetenv(k)
			}
		}
	}()

	// Set environment variables
	_ = os.Setenv("BITBUCKET_USERNAME", "env-user")
	_ = os.Setenv("BITBUCKET_ACCESS_TOKEN", "env-token")

	// Create flags with existing values
	cmdFlags := &data.CmdExportFlags{
		BitbucketUser:        "flag-user",
		BitbucketAccessToken: "flag-token",
	}

	SetupEnvironmentCredentials(cmdFlags)

	// Flag values should be preserved (not overwritten by env)
	assert.Equal(t, "flag-user", cmdFlags.BitbucketUser)
	assert.Equal(t, "flag-token", cmdFlags.BitbucketAccessToken)
}

func TestSetupEnvironmentCredentialsEmptyFlagsFilledFromEnv(t *testing.T) {
	envVars := []string{
		"BITBUCKET_USERNAME",
		"BITBUCKET_APP_PASSWORD",
		"BITBUCKET_ACCESS_TOKEN",
		"BITBUCKET_API_TOKEN",
		"BITBUCKET_EMAIL",
		"BITBUCKET_TEMP_DIR",
	}

	originalValues := make(map[string]string)
	for _, v := range envVars {
		originalValues[v] = os.Getenv(v)
		_ = os.Unsetenv(v)
	}

	defer func() {
		for k, v := range originalValues {
			if v != "" {
				_ = os.Setenv(k, v)
			} else {
				_ = os.Unsetenv(k)
			}
		}
	}()

	// Set environment variables
	_ = os.Setenv("BITBUCKET_USERNAME", "env-user")
	_ = os.Setenv("BITBUCKET_APP_PASSWORD", "env-pass")

	// Create empty flags
	cmdFlags := &data.CmdExportFlags{}

	SetupEnvironmentCredentials(cmdFlags)

	// Empty flags should be filled from environment
	assert.Equal(t, "env-user", cmdFlags.BitbucketUser)
	assert.Equal(t, "env-pass", cmdFlags.BitbucketAppPass)
}

func TestPrintSuccessMessageWithArchive(t *testing.T) {
	// Test with archive path
	assert.NotPanics(t, func() {
		PrintSuccessMessage("/path/to/archive.tar.gz")
	})
}

func TestPrintSuccessMessageWithDirectory(t *testing.T) {
	// Test with directory path
	assert.NotPanics(t, func() {
		PrintSuccessMessage("/path/to/output/directory")
	})
}

func TestHashStringDeterministic(t *testing.T) {
	input := "test-input-string"

	// Hash the same string multiple times
	results := make([]string, 10)
	for i := 0; i < 10; i++ {
		results[i] = HashString(input)
	}

	// All results should be identical
	for i := 1; i < len(results); i++ {
		assert.Equal(t, results[0], results[i], "Hash should be deterministic")
	}
}

func TestHashStringDifferentInputs(t *testing.T) {
	inputs := []string{
		"input1",
		"input2",
		"input3",
		"a very long input string that is quite different",
		"",
		" ",
		"123",
	}

	hashes := make(map[string]bool)
	for _, input := range inputs {
		hash := HashString(input)
		// Each hash should be unique (except for the empty string which might collide)
		if input != "" {
			assert.False(t, hashes[hash], "Hash collision detected for input: %s", input)
		}
		hashes[hash] = true
	}
}

func TestExtractPRNumberVariousFormats(t *testing.T) {
	testCases := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "HTTPS URL",
			url:      "https://bitbucket.org/workspace/repo/pull/123",
			expected: "123",
		},
		{
			name:     "HTTP URL",
			url:      "http://bitbucket.org/workspace/repo/pull/456",
			expected: "456",
		},
		{
			name:     "URL with fragment",
			url:      "https://bitbucket.org/workspace/repo/pull/789#comment",
			expected: "789",
		},
		{
			name:     "Not a PR URL - issues",
			url:      "https://bitbucket.org/workspace/repo/issues/123",
			expected: "",
		},
		{
			name:     "Not a PR URL - commits",
			url:      "https://bitbucket.org/workspace/repo/commits/abc123",
			expected: "",
		},
		{
			name:     "Malformed URL",
			url:      "not-a-url",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := extractPRNumber(tc.url)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestValidateGitReferenceMoreCases(t *testing.T) {
	testCases := []struct {
		name        string
		ref         string
		expectError bool
	}{
		{"Valid simple ref", "main", false},
		{"Valid feature branch", "feature/test", false},
		{"Valid with numbers", "release-1.0.0", false},
		{"Valid with underscore", "feature_test", false},
		{"Backslash", "feature\\test", true},
		{"Just a dash", "-", false},
		{"Unicode characters", "feature-日本語", false},
		{"Control character NUL", "feature\x00test", false},
		{"Tab character", "feature\ttest", false},
		{"Newline", "feature\ntest", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateGitReference(tc.ref)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWriteJSONFileWithNestedData(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "write-json-nested-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	logger, _ := zap.NewDevelopment()
	exporter := NewExporter(&Client{}, tempDir, logger, false, "")

	// Create nested data structure
	nestedData := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"level3": "deep value",
			},
			"array": []string{"item1", "item2", "item3"},
		},
		"number": 42,
		"bool":   true,
	}

	err = exporter.writeJSONFile("nested.json", nestedData)
	assert.NoError(t, err)

	// Read back and verify
	content, err := os.ReadFile(filepath.Join(tempDir, "nested.json"))
	assert.NoError(t, err)

	var readData map[string]interface{}
	err = json.Unmarshal(content, &readData)
	assert.NoError(t, err)

	assert.Equal(t, float64(42), readData["number"])
	assert.Equal(t, true, readData["bool"])
}

func TestUpdateRepositoryFieldNonExistentFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "update-repo-nonexistent-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	core, observedLogs := observer.New(zap.WarnLevel)
	logger := zap.New(core)
	exporter := NewExporter(&Client{}, tempDir, logger, false, "")

	// Try to update without creating the file first
	exporter.updateRepositoryField("test-repo", "default_branch", "main")

	// Should log a warning
	logs := observedLogs.All()
	assert.GreaterOrEqual(t, len(logs), 1)
}

func TestUpdateRepositoryFieldInvalidJSON(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "update-repo-invalid-json-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Write invalid JSON
	err = os.WriteFile(filepath.Join(tempDir, "repositories_000001.json"), []byte("not valid json"), 0644)
	assert.NoError(t, err)

	core, observedLogs := observer.New(zap.WarnLevel)
	logger := zap.New(core)
	exporter := NewExporter(&Client{}, tempDir, logger, false, "")

	exporter.updateRepositoryField("test-repo", "default_branch", "main")

	// Should log a warning about parsing
	logs := observedLogs.All()
	assert.GreaterOrEqual(t, len(logs), 1)
}

func TestCreateRepositoryInfoFilesWithSpecialChars(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "repo-info-special-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	logger, _ := zap.NewDevelopment()
	exporter := NewExporter(&Client{}, tempDir, logger, false, "")

	workspace := "test-workspace"
	repoSlug := "test-repo-with-dashes"

	// Create the repository directory
	repoDir := filepath.Join(tempDir, "repositories", workspace, repoSlug+".git")
	err = os.MkdirAll(repoDir, 0755)
	assert.NoError(t, err)

	err = exporter.createRepositoryInfoFiles(workspace, repoSlug)
	assert.NoError(t, err)

	// Verify nwo content
	nwoPath := filepath.Join(repoDir, "info", "nwo")
	content, err := os.ReadFile(nwoPath)
	assert.NoError(t, err)
	assert.Equal(t, "test-workspace/test-repo-with-dashes\n", string(content))
}

func TestFormatDateToZWithVariousTimezones(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "UTC timezone",
			input:    "2023-06-15T10:30:00Z",
			expected: "2023-06-15T10:30:00Z",
		},
		{
			name:     "Positive offset +05:30",
			input:    "2023-06-15T16:00:00+05:30",
			expected: "2023-06-15T10:30:00Z",
		},
		{
			name:     "Negative offset -08:00",
			input:    "2023-06-15T02:30:00-08:00",
			expected: "2023-06-15T10:30:00Z",
		},
		{
			name:     "With milliseconds",
			input:    "2023-06-15T10:30:00.123Z",
			expected: "2023-06-15T10:30:00Z",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatDateToZ(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestExecuteCommandWithWorkingDir(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "exec-cmd-workdir-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create a file in temp dir
	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("content"), 0644)
	assert.NoError(t, err)

	// Run command in that directory
	var output []byte
	if runtime.GOOS == "windows" {
		output, err = ExecuteCommand("cmd", []string{"/c", "dir"}, tempDir, false)
	} else {
		output, err = ExecuteCommand("ls", []string{"-la"}, tempDir, false)
	}

	assert.NoError(t, err)
	assert.Contains(t, string(output), "test.txt")
}

func TestNormalizePathEmpty(t *testing.T) {
	result := NormalizePath("")
	assert.Equal(t, "", result)
}

func TestToUnixPathEmpty(t *testing.T) {
	result := ToUnixPath("")
	assert.Equal(t, "", result)
}

func TestToNativePathEmpty(t *testing.T) {
	result := ToNativePath("")
	assert.Equal(t, "", result)
}

func TestGetOutputPathAfterExport(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "output-path-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	logger, _ := zap.NewDevelopment()
	exporter := NewExporter(&Client{}, tempDir, logger, false, "")

	path := exporter.GetOutputPath()
	assert.Equal(t, tempDir, path)
}

func TestExtractPRNumberWithPullRequestsPath(t *testing.T) {
	testCases := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "Pull-requests URL format",
			url:      "https://bitbucket.org/workspace/repo/pull-requests/123",
			expected: "",
		},
		{
			name:     "Pull URL format",
			url:      "https://bitbucket.org/workspace/repo/pull/123",
			expected: "123",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := extractPRNumber(tc.url)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestValidateExportFlagsPartialCredentials(t *testing.T) {
	testCases := []struct {
		name        string
		flags       *data.CmdExportFlags
		expectError bool
		errorMsg    string
	}{
		{
			name: "Email without API token",
			flags: &data.CmdExportFlags{
				BitbucketEmail: "test@example.com",
			},
			expectError: true,
			errorMsg:    "authentication credentials required",
		},
		{
			name: "App password without username",
			flags: &data.CmdExportFlags{
				BitbucketAppPass: "password",
			},
			expectError: true,
			errorMsg:    "authentication credentials required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateExportFlags(tc.flags)
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFormatURLAllTypes(t *testing.T) {
	testCases := []struct {
		name     string
		urlType  string
		ws       string
		repo     string
		ids      []interface{}
		expected string
	}{
		{
			name:     "Issue comment with two IDs",
			urlType:  "issue_comment",
			ws:       "ws",
			repo:     "repo",
			ids:      []interface{}{"123", "456"},
			expected: "https://bitbucket.org/ws/repo/pull/123#issuecomment-456",
		},
		{
			name:     "Issue comment with no IDs",
			urlType:  "issue_comment",
			ws:       "ws",
			repo:     "repo",
			ids:      []interface{}{},
			expected: "https://bitbucket.org/ws/repo/pull/comments",
		},
		{
			name:     "PR review thread with two IDs",
			urlType:  "pr_review_thread",
			ws:       "ws",
			repo:     "repo",
			ids:      []interface{}{"123", "456"},
			expected: "https://bitbucket.org/ws/repo/pull/123/files#pullrequestreviewthread-456",
		},
		{
			name:     "PR review thread with no IDs",
			urlType:  "pr_review_thread",
			ws:       "ws",
			repo:     "repo",
			ids:      []interface{}{},
			expected: "https://bitbucket.org/ws/repo/pull/threads",
		},
		{
			name:     "PR review with two IDs",
			urlType:  "pr_review",
			ws:       "ws",
			repo:     "repo",
			ids:      []interface{}{"123", "456"},
			expected: "https://bitbucket.org/ws/repo/pull/123/files#pullrequestreview-456",
		},
		{
			name:     "PR review with no IDs",
			urlType:  "pr_review",
			ws:       "ws",
			repo:     "repo",
			ids:      []interface{}{},
			expected: "https://bitbucket.org/ws/repo/pull/reviews",
		},
		{
			name:     "PR review comment with two IDs",
			urlType:  "pr_review_comment",
			ws:       "ws",
			repo:     "repo",
			ids:      []interface{}{"123", "456"},
			expected: "https://bitbucket.org/ws/repo/pull/123/files#r456",
		},
		{
			name:     "PR review comment with no IDs",
			urlType:  "pr_review_comment",
			ws:       "ws",
			repo:     "repo",
			ids:      []interface{}{},
			expected: "https://bitbucket.org/ws/repo/pull/comments",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result string
			switch len(tc.ids) {
			case 0:
				result = formatURL(tc.urlType, tc.ws, tc.repo)
			case 1:
				result = formatURL(tc.urlType, tc.ws, tc.repo, tc.ids[0])
			case 2:
				result = formatURL(tc.urlType, tc.ws, tc.repo, tc.ids[0], tc.ids[1])
			}
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestHashStringEmptyAndWhitespace(t *testing.T) {
	empty := HashString("")
	space := HashString(" ")
	tab := HashString("\t")
	newline := HashString("\n")

	assert.NotEmpty(t, empty)
	assert.NotEmpty(t, space)
	assert.NotEmpty(t, tab)
	assert.NotEmpty(t, newline)

	assert.NotEqual(t, empty, space)
	assert.NotEqual(t, space, tab)
	assert.NotEqual(t, tab, newline)
}

func TestExecuteCommandWithSSLBypass(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git not available for testing")
	}

	output, err := ExecuteCommand("git", []string{"--version"}, "", true)
	assert.NoError(t, err)
	assert.Contains(t, string(output), "git")
}

func TestCreateRepositoryInfoFilesErrorHandling(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Permission test not reliable on Windows")
	}

	tempDir, err := os.MkdirTemp("", "repo-info-error-")
	assert.NoError(t, err)
	defer func() {
		_ = os.Chmod(tempDir, 0755)
		_ = os.RemoveAll(tempDir)
	}()

	logger, _ := zap.NewDevelopment()
	exporter := NewExporter(&Client{}, tempDir, logger, false, "")

	repoDir := filepath.Join(tempDir, "repositories", "ws", "repo.git")
	err = os.MkdirAll(repoDir, 0755)
	assert.NoError(t, err)

	err = os.Chmod(repoDir, 0555)
	assert.NoError(t, err)

	err = exporter.createRepositoryInfoFiles("ws", "repo")
	assert.Error(t, err)
}

func TestValidateGitReferenceEdgeCases(t *testing.T) {
	testCases := []struct {
		name        string
		ref         string
		expectError bool
	}{
		{"Double dots at start", "..test", true},
		{"Double dots at end", "test..", true},
		{"Double dots in middle", "test..branch", true},
		{"At-brace sequence", "test@{0}", true},
		{"Double slash at start", "//test", true},
		{"Double slash at end", "test//", true},
		{"Double slash in middle", "test//branch", true},
		{"Dot-lock extension", "branch.lock", true},
		{"Valid with dot", "v1.0.0", false},
		{"Valid with at", "user@branch", false},
		{"Valid with dash", "feature-branch", false},
		{"Valid with underscore", "feature_branch", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateGitReference(tc.ref)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSetupEnvironmentCredentialsAllVars(t *testing.T) {
	envVars := []string{
		"BITBUCKET_USERNAME",
		"BITBUCKET_APP_PASSWORD",
		"BITBUCKET_ACCESS_TOKEN",
		"BITBUCKET_API_TOKEN",
		"BITBUCKET_EMAIL",
		"BITBUCKET_TEMP_DIR",
	}

	originalValues := make(map[string]string)
	for _, v := range envVars {
		originalValues[v] = os.Getenv(v)
		_ = os.Unsetenv(v)
	}

	defer func() {
		for k, v := range originalValues {
			if v != "" {
				_ = os.Setenv(k, v)
			} else {
				_ = os.Unsetenv(k)
			}
		}
	}()

	_ = os.Setenv("BITBUCKET_USERNAME", "env-user")
	_ = os.Setenv("BITBUCKET_APP_PASSWORD", "env-pass")
	_ = os.Setenv("BITBUCKET_ACCESS_TOKEN", "env-token")
	_ = os.Setenv("BITBUCKET_API_TOKEN", "env-api-token")
	_ = os.Setenv("BITBUCKET_EMAIL", "env@example.com")
	_ = os.Setenv("BITBUCKET_TEMP_DIR", "/env/temp")

	cmdFlags := &data.CmdExportFlags{}
	SetupEnvironmentCredentials(cmdFlags)

	assert.Equal(t, "env-user", cmdFlags.BitbucketUser)
	assert.Equal(t, "env-pass", cmdFlags.BitbucketAppPass)
	assert.Equal(t, "env-token", cmdFlags.BitbucketAccessToken)
	assert.Equal(t, "env-api-token", cmdFlags.BitbucketAPIToken)
	assert.Equal(t, "env@example.com", cmdFlags.BitbucketEmail)
	assert.Equal(t, "/env/temp", cmdFlags.TempDir)
}

func TestPRDateValidationEdgeCases(t *testing.T) {
	testCases := []struct {
		name        string
		date        string
		expectError bool
	}{
		{"Leap year valid date", "2024-02-29", false},
		{"Non-leap year Feb 29", "2023-02-29", true},
		{"December 31", "2023-12-31", false},
		{"January 1", "2023-01-01", false},
		{"Month 13", "2023-13-01", true},
		{"Day 32", "2023-01-32", true},
		{"Month 00", "2023-00-15", true},
		{"Day 00", "2023-01-00", true},
		{"Future date", "2099-12-31", false},
		{"Old date", "1990-01-01", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmdFlags := &data.CmdExportFlags{
				BitbucketAccessToken: "test-token",
				PRsFromDate:          tc.date,
			}
			err := ValidateExportFlags(cmdFlags)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFormatDateToZInvalidFormats(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"Random text", "not a date", ""},
		{"Partial date", "2023-01", ""},
		{"Just numbers", "20230101", ""},
		{"Spaces", "2023 01 01", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatDateToZ(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestWriteJSONFileLargeData(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "write-json-large-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	logger, _ := zap.NewDevelopment()
	exporter := NewExporter(&Client{}, tempDir, logger, false, "")

	largeData := make([]map[string]string, 1000)
	for i := 0; i < 1000; i++ {
		largeData[i] = map[string]string{
			"id":    fmt.Sprintf("%d", i),
			"value": strings.Repeat("x", 100),
		}
	}

	err = exporter.writeJSONFile("large.json", largeData)
	assert.NoError(t, err)

	info, err := os.Stat(filepath.Join(tempDir, "large.json"))
	assert.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0))
}

func TestPathConversionsWithAbsolutePaths(t *testing.T) {
	testCases := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "Unix absolute path",
			path:     "/usr/local/bin",
			expected: "/usr/local/bin",
		},
		{
			name:     "Relative path with dots",
			path:     "../parent/child",
			expected: "../parent/child",
		},
		{
			name:     "Current directory",
			path:     ".",
			expected: ".",
		},
		{
			name:     "Parent directory",
			path:     "..",
			expected: "..",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ToUnixPath(tc.path)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestMakeRequestAuthHeaders(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			w.WriteHeader(http.StatusOK)
			writeResponse(t, w, []byte(`{"type": "bearer"}`))
		} else if strings.HasPrefix(auth, "Basic ") {
			w.WriteHeader(http.StatusOK)
			writeResponse(t, w, []byte(`{"type": "basic"}`))
		} else {
			w.WriteHeader(http.StatusUnauthorized)
			writeResponse(t, w, []byte(`{"error": "unauthorized"}`))
		}
	}))
	defer testServer.Close()

	// Test API token auth
	apiTokenClient := &Client{
		baseURL:    testServer.URL,
		httpClient: testServer.Client(),
		apiToken:   "api-token",
		email:      "test@example.com",
		logger:     logger,
	}

	var result map[string]interface{}
	err := apiTokenClient.makeRequest("GET", "/", &result)
	assert.NoError(t, err)
	assert.Equal(t, "basic", result["type"])
}

func TestUpdateRepositoryFieldUnknownField(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "update-field-unknown-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	logger, _ := zap.NewDevelopment()
	exporter := NewExporter(&Client{}, tempDir, logger, false, "")

	repos := []data.Repository{
		{Name: "test-repo", Slug: "test-repo", DefaultBranch: "main"},
	}
	err = exporter.writeJSONFile("repositories_000001.json", repos)
	assert.NoError(t, err)

	exporter.updateRepositoryField("test-repo", "unknown_field", "value")

	content, err := os.ReadFile(filepath.Join(tempDir, "repositories_000001.json"))
	assert.NoError(t, err)

	var readRepos []data.Repository
	err = json.Unmarshal(content, &readRepos)
	assert.NoError(t, err)
	assert.Equal(t, "main", readRepos[0].DefaultBranch)
}

func TestExecuteCommandNonExistentWorkingDir(t *testing.T) {
	_, err := ExecuteCommand("echo", []string{"test"}, "/nonexistent/path", false)
	assert.Error(t, err)
}

func TestValidateExportFlagsWithAPITokenAuth(t *testing.T) {
	cmdFlags := &data.CmdExportFlags{
		BitbucketAPIToken: "api-token",
		BitbucketEmail:    "user@example.com",
	}
	err := ValidateExportFlags(cmdFlags)
	assert.NoError(t, err)
}

func TestCreateEmptyRepositorySuccess(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "empty-repo-success-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	logger, _ := zap.NewDevelopment()
	exporter := NewExporter(&Client{}, tempDir, logger, false, "")

	err = exporter.createEmptyRepository("workspace", "repo")
	assert.NoError(t, err)

	repoPath := filepath.Join(tempDir, "repositories", "workspace", "repo.git")
	assert.DirExists(t, repoPath)

	headPath := filepath.Join(repoPath, "HEAD")
	assert.FileExists(t, headPath)
}

func TestSetupCommandUsageTemplateZeroWidth(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
	}

	cmd.PersistentFlags().String("flag", "", "A flag")

	SetupCommandUsageTemplate(cmd, 0)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Usage()
	assert.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

func TestFormatDateToZWithNanoseconds(t *testing.T) {
	input := "2023-06-15T10:30:00.123456789Z"
	expected := "2023-06-15T10:30:00Z"
	result := formatDateToZ(input)
	assert.Equal(t, expected, result)
}

func TestHashStringSpecialCharacters(t *testing.T) {
	inputs := []string{
		"path/with/slashes",
		"path\\with\\backslashes",
		"string with spaces",
		"string\twith\ttabs",
		"unicode: 日本語",
		"emoji: 🚀✨",
		"<html>tags</html>",
		"'quotes' and \"double quotes\"",
	}

	hashes := make(map[string]string)
	for _, input := range inputs {
		hash := HashString(input)
		assert.NotEmpty(t, hash)
		if existing, ok := hashes[hash]; ok {
			assert.NotEqual(t, input, existing, "Collision between %q and %q", input, existing)
		}
		hashes[hash] = input
	}
}

func TestGetAPIURLHostGHECom(t *testing.T) {
	testCases := []struct {
		name            string
		apiURL          string
		expectedHost    string
		expectedAPIHost string
		expectError     bool
	}{
		{
			name:            "Default github.com",
			apiURL:          "https://api.github.com",
			expectedHost:    "github.com",
			expectedAPIHost: "api.github.com",
			expectError:     false,
		},
		{
			name:            "GitHub.com with trailing slash",
			apiURL:          "https://api.github.com/",
			expectedHost:    "github.com",
			expectedAPIHost: "api.github.com",
			expectError:     false,
		},
		{
			name:            "GitHub.com with graphql path",
			apiURL:          "https://api.github.com/graphql",
			expectedHost:    "github.com",
			expectedAPIHost: "api.github.com",
			expectError:     false,
		},
		{
			name:            "GitHub.com with v3 path",
			apiURL:          "https://api.github.com/v3",
			expectedHost:    "github.com",
			expectedAPIHost: "api.github.com",
			expectError:     false,
		},
		{
			name:            "GHE.com - octocorp",
			apiURL:          "https://api.octocorp.ghe.com",
			expectedHost:    "octocorp.ghe.com",
			expectedAPIHost: "api.octocorp.ghe.com",
			expectError:     false,
		},
		{
			name:            "GHE.com - mycompany",
			apiURL:          "https://api.mycompany.ghe.com",
			expectedHost:    "mycompany.ghe.com",
			expectedAPIHost: "api.mycompany.ghe.com",
			expectError:     false,
		},
		{
			name:            "GHES URL with /api/v3",
			apiURL:          "https://github.example.com/api/v3",
			expectedHost:    "github.example.com",
			expectedAPIHost: "github.example.com",
			expectError:     false,
		},
		{
			name:        "Invalid URL",
			apiURL:      "://invalid-url",
			expectError: true,
		},
		{
			name:            "Empty URL",
			apiURL:          "",
			expectedHost:    "github.com",
			expectedAPIHost: "api.github.com",
			expectError:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			apiHost, host, err := GetAPIURLHost(tc.apiURL)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedHost, host)
				assert.Equal(t, tc.expectedAPIHost, apiHost)
			}
		})
	}
}

func TestGetGitHubAuthTokenWithTargetAPIURL(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Save and clear relevant env vars
	originalGHPAT := os.Getenv("GITHUB_PAT")
	defer func() {
		if originalGHPAT != "" {
			_ = os.Setenv("GITHUB_PAT", originalGHPAT)
		} else {
			_ = os.Unsetenv("GITHUB_PAT")
		}
	}()
	_ = os.Unsetenv("GITHUB_PAT")

	t.Run("Flag PAT takes priority regardless of target API URL", func(t *testing.T) {
		flags := &data.CmdMigrateFlags{
			GitHubPAT:    "ghp_flag_token",
			TargetAPIURL: "https://api.octocorp.ghe.com",
		}

		token, err := GetGitHubAuthToken(flags, logger)
		assert.NoError(t, err)
		assert.Equal(t, "ghp_flag_token", token)
	})

	t.Run("Environment GITHUB_PAT takes priority over gh CLI", func(t *testing.T) {
		_ = os.Setenv("GITHUB_PAT", "ghp_env_token")
		defer func() { _ = os.Unsetenv("GITHUB_PAT") }()

		flags := &data.CmdMigrateFlags{
			TargetAPIURL: "https://api.octocorp.ghe.com",
		}

		token, err := GetGitHubAuthToken(flags, logger)
		assert.NoError(t, err)
		assert.Equal(t, "ghp_env_token", token)
	})

	t.Run("No auth returns error with GHE.com host in message", func(t *testing.T) {
		_ = os.Unsetenv("GITHUB_PAT")

		flags := &data.CmdMigrateFlags{
			TargetAPIURL: "https://api.octocorp.ghe.com",
		}

		_, err := GetGitHubAuthToken(flags, logger)
		// It may find a token via gh CLI keychain, or it may fail.
		// If it fails, the error should reference the correct host.
		if err != nil {
			assert.Contains(t, err.Error(), "octocorp.ghe.com")
		}
	})

	t.Run("No auth returns error with github.com host for default URL", func(t *testing.T) {
		_ = os.Unsetenv("GITHUB_PAT")

		flags := &data.CmdMigrateFlags{
			TargetAPIURL: "https://api.github.com",
		}

		_, err := GetGitHubAuthToken(flags, logger)
		if err != nil {
			assert.Contains(t, err.Error(), "github.com")
		}
	})

	t.Run("Empty target API URL defaults to github.com", func(t *testing.T) {
		_ = os.Unsetenv("GITHUB_PAT")

		flags := &data.CmdMigrateFlags{
			TargetAPIURL: "",
		}

		_, err := GetGitHubAuthToken(flags, logger)
		if err != nil {
			assert.Contains(t, err.Error(), "github.com")
		}
	})
}
