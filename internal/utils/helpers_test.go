package utils

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/katiem0/gh-bbc-exporter/internal/data"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestValidateExportFlags(t *testing.T) {

	// Test case 1: No credentials provided
	cmdFlags := &data.CmdFlags{}
	err := ValidateExportFlags(cmdFlags)
	assert.Error(t, err, "Expected error when no credentials are provided")

	// Test case 2: Token provided
	cmdFlags = &data.CmdFlags{}
	cmdFlags.BitbucketToken = "testtoken"
	err = ValidateExportFlags(cmdFlags)
	assert.NoError(t, err, "Expected no error when token is provided")

	// Test case 3: Username and app password provided
	cmdFlags = &data.CmdFlags{}
	cmdFlags.BitbucketToken = ""
	cmdFlags.BitbucketUser = "testuser"
	cmdFlags.BitbucketAppPass = "testpass"
	err = ValidateExportFlags(cmdFlags)
	assert.NoError(t, err, "Expected no error when username and app password are provided")

	// Test case 4: Only username provided (missing app password)
	cmdFlags = &data.CmdFlags{}
	cmdFlags.BitbucketToken = ""
	cmdFlags.BitbucketUser = "testuser"
	cmdFlags.BitbucketAppPass = ""
	err = ValidateExportFlags(cmdFlags)
	assert.Error(t, err, "Expected error when only username is provided")

	// Test case 5: Only app password provided (missing username)
	cmdFlags = &data.CmdFlags{}
	cmdFlags.BitbucketToken = ""
	cmdFlags.BitbucketUser = ""
	cmdFlags.BitbucketAppPass = "testpass"
	err = ValidateExportFlags(cmdFlags)
	assert.Error(t, err, "Expected error when only app password is provided")

	// Test case 6: Valid date format for PRsFromDate
	cmdFlags = &data.CmdFlags{}
	cmdFlags.BitbucketToken = "testtoken"
	cmdFlags.PRsFromDate = "2023-01-01"
	err = ValidateExportFlags(cmdFlags)
	assert.NoError(t, err, "Expected no error with valid date format")

	// Test case 7: Invalid date format for PRsFromDate
	cmdFlags = &data.CmdFlags{}
	cmdFlags.BitbucketToken = "testtoken"
	cmdFlags.PRsFromDate = "01/01/2023"
	err = ValidateExportFlags(cmdFlags)
	assert.Error(t, err, "Expected error with invalid date format")
	assert.Contains(t, err.Error(), "invalid date format for --prs-from-date", "Error should mention invalid date format")
}

func TestSetupEnvironmentCredentials(t *testing.T) {
	cmdFlags := &data.CmdFlags{}

	// Set environment variables
	err := os.Setenv("BITBUCKET_USERNAME", "envuser")
	assert.NoError(t, err)
	err = os.Setenv("BITBUCKET_APP_PASSWORD", "envpass")
	assert.NoError(t, err)
	err = os.Setenv("BITBUCKET_TOKEN", "envtoken")
	assert.NoError(t, err)

	// Call the function
	SetupEnvironmentCredentials(cmdFlags)

	// Assert that the values are set correctly
	assert.Equal(t, "envuser", cmdFlags.BitbucketUser, "Expected username to be set from environment")
	assert.Equal(t, "envpass", cmdFlags.BitbucketAppPass, "Expected app password to be set from environment")
	assert.Equal(t, "envtoken", cmdFlags.BitbucketToken, "Expected token to be set from environment")

	// Clean up environment variables
	err = os.Unsetenv("BITBUCKET_USERNAME")
	assert.NoError(t, err)
	err = os.Unsetenv("BITBUCKET_APP_PASSWORD")
	assert.NoError(t, err)
	err = os.Unsetenv("BITBUCKET_TOKEN")
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
				BitbucketToken: "test-token", // Add required auth
				PRsFromDate:    tc.dateStr,
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
