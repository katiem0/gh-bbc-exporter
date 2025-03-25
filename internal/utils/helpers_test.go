package utils

import (
	"os"
	"testing"
	"time"

	"github.com/katiem0/gh-bbc-exporter/internal/data"
	"github.com/stretchr/testify/assert"
)

func TestValidateExportFlags(t *testing.T) {
	cmdFlags := &data.CmdFlags{}

	// Test case 1: No credentials provided
	err := ValidateExportFlags(cmdFlags)
	assert.Error(t, err, "Expected error when no credentials are provided")

	// Test case 2: Token provided
	cmdFlags.BitbucketToken = "testtoken"
	err = ValidateExportFlags(cmdFlags)
	assert.NoError(t, err, "Expected no error when token is provided")

	// Test case 3: Username and app password provided
	cmdFlags.BitbucketToken = ""
	cmdFlags.BitbucketUser = "testuser"
	cmdFlags.BitbucketAppPass = "testpass"
	err = ValidateExportFlags(cmdFlags)
	assert.NoError(t, err, "Expected no error when username and app password are provided")

	// Test case 4: Only username provided (missing app password)
	cmdFlags.BitbucketToken = ""
	cmdFlags.BitbucketUser = "testuser"
	cmdFlags.BitbucketAppPass = ""
	err = ValidateExportFlags(cmdFlags)
	assert.Error(t, err, "Expected error when only username is provided")

	// Test case 5: Only app password provided (missing username)
	cmdFlags.BitbucketToken = ""
	cmdFlags.BitbucketUser = ""
	cmdFlags.BitbucketAppPass = "testpass"
	err = ValidateExportFlags(cmdFlags)
	assert.Error(t, err, "Expected error when only app password is provided")
}

func TestSetupEnvironmentCredentials(t *testing.T) {
	cmdFlags := &data.CmdFlags{}

	// Set environment variables
	os.Setenv("BITBUCKET_USERNAME", "envuser")
	os.Setenv("BITBUCKET_APP_PASSWORD", "envpass")
	os.Setenv("BITBUCKET_TOKEN", "envtoken")

	// Call the function
	SetupEnvironmentCredentials(cmdFlags)

	// Assert that the values are set correctly
	assert.Equal(t, "envuser", cmdFlags.BitbucketUser, "Expected username to be set from environment")
	assert.Equal(t, "envpass", cmdFlags.BitbucketAppPass, "Expected app password to be set from environment")
	assert.Equal(t, "envtoken", cmdFlags.BitbucketToken, "Expected token to be set from environment")

	// Clean up environment variables
	os.Unsetenv("BITBUCKET_USERNAME")
	os.Unsetenv("BITBUCKET_APP_PASSWORD")
	os.Unsetenv("BITBUCKET_TOKEN")
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
		id         string
		id2        string
		expected   string
	}{
		{
			name:       "PR URL",
			urlType:    "pr",
			workspace:  "testworkspace",
			repository: "testrepo",
			id:         "123",
			expected:   "https://bitbucket.org/testworkspace/testrepo/pull/123",
		},
		{
			name:       "Repository URL",
			urlType:    "repository",
			workspace:  "testworkspace",
			repository: "testrepo",
			expected:   "https://bitbucket.org/testworkspace/testrepo",
		},
		{
			name:      "User URL",
			urlType:   "user",
			workspace: "testworkspace",
			id:        "testuser",
			expected:  "https://bitbucket.org/testuser",
		},
		{
			name:       "PR Review URL",
			urlType:    "pr_review",
			workspace:  "testworkspace",
			repository: "testrepo",
			id:         "123",
			id2:        "456",
			expected:   "https://bitbucket.org/testworkspace/testrepo/pull/123/files#pullrequestreview-456",
		},
		{
			name:       "PR Review Comment URL",
			urlType:    "pr_review_comment",
			workspace:  "testworkspace",
			repository: "testrepo",
			id:         "123",
			id2:        "456",
			expected:   "https://bitbucket.org/testworkspace/testrepo/pull/123/files#r456",
		},
		{
			name:       "Issue Comment URL",
			urlType:    "issue_comment",
			workspace:  "testworkspace",
			repository: "testrepo",
			id:         "123",
			id2:        "456",
			expected:   "https://bitbucket.org/testworkspace/testrepo/pull/123#issuecomment-456",
		},
		{
			name:       "PR Review Thread URL",
			urlType:    "pr_review_thread",
			workspace:  "testworkspace",
			repository: "testrepo",
			id:         "123",
			id2:        "456",
			expected:   "https://bitbucket.org/testworkspace/testrepo/pull/123/files#pullrequestreviewthread-456",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatURL(tc.urlType, tc.workspace, tc.repository, tc.id, tc.id2)
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
