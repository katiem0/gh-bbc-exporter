package data

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepoVisibilityString(t *testing.T) {
	testCases := []struct {
		name     string
		value    RepoVisibility
		expected string
	}{
		{
			name:     "Public visibility",
			value:    RepoVisibility("public"),
			expected: "public",
		},
		{
			name:     "Private visibility",
			value:    RepoVisibility("private"),
			expected: "private",
		},
		{
			name:     "Internal visibility",
			value:    RepoVisibility("internal"),
			expected: "internal",
		},
		{
			name:     "Empty visibility defaults to private",
			value:    RepoVisibility(""),
			expected: "private",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.value.String()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestRepoVisibilitySet(t *testing.T) {
	testCases := []struct {
		name        string
		value       string
		expectError bool
		expected    RepoVisibility
	}{
		{
			name:        "Set public",
			value:       "public",
			expectError: false,
			expected:    RepoVisibility("public"),
		},
		{
			name:        "Set private",
			value:       "private",
			expectError: false,
			expected:    RepoVisibility("private"),
		},
		{
			name:        "Set internal",
			value:       "internal",
			expectError: false,
			expected:    RepoVisibility("internal"),
		},
		{
			name:        "Set empty string defaults to private",
			value:       "",
			expectError: false,
			expected:    RepoVisibility("private"),
		},
		{
			name:        "Invalid visibility - uppercase",
			value:       "PUBLIC",
			expectError: true,
			expected:    RepoVisibility(""),
		},
		{
			name:        "Invalid visibility - random string",
			value:       "protected",
			expectError: true,
			expected:    RepoVisibility(""),
		},
		{
			name:        "Invalid visibility - numeric",
			value:       "123",
			expectError: true,
			expected:    RepoVisibility(""),
		},
		{
			name:        "Invalid visibility - mixed case",
			value:       "Private",
			expectError: true,
			expected:    RepoVisibility(""),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var rv RepoVisibility
			err := rv.Set(tc.value)

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "must be one of")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, rv)
			}
		})
	}
}

func TestRepoVisibilityAllValidValues(t *testing.T) {
	testCases := []struct {
		input          string
		expectedString string
	}{
		{"public", "public"},
		{"private", "private"},
		{"internal", "internal"},
		{"", "private"}, // Empty defaults to private
	}

	for _, tc := range testCases {
		t.Run("valid_"+tc.input, func(t *testing.T) {
			var rv RepoVisibility
			err := rv.Set(tc.input)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedString, rv.String())
		})
	}
}

func TestRepoVisibilityType(t *testing.T) {
	var rv RepoVisibility
	typeStr := rv.Type()
	assert.Contains(t, typeStr, "public")
	assert.Contains(t, typeStr, "private")
	assert.Contains(t, typeStr, "internal")
}

func TestCmdMigrateFlagsDefaults(t *testing.T) {
	migrateFlags := CmdMigrateFlags{}

	assert.Empty(t, migrateFlags.TargetOrg)
	assert.Empty(t, migrateFlags.TargetRepo)
	assert.Empty(t, migrateFlags.GitHubPAT)
	assert.Equal(t, RepoVisibility(""), migrateFlags.TargetRepoVisibility)
	assert.False(t, migrateFlags.KeepArchive)
}

func TestCmdMigrateFlagsWithValues(t *testing.T) {
	flags := CmdMigrateFlags{
		TargetOrg:            "github-org",
		TargetRepo:           "github-repo",
		GitHubPAT:            "ghp_token",
		TargetRepoVisibility: RepoVisibility("private"),
		KeepArchive:          true,
	}

	assert.Equal(t, "github-org", flags.TargetOrg)
	assert.Equal(t, "github-repo", flags.TargetRepo)
	assert.Equal(t, "ghp_token", flags.GitHubPAT)
	assert.Equal(t, RepoVisibility("private"), flags.TargetRepoVisibility)
	assert.True(t, flags.KeepArchive)
}

func TestCmdMigrateFlagsJSON(t *testing.T) {
	flags := CmdMigrateFlags{
		TargetOrg:            "github-org",
		TargetRepo:           "github-repo",
		TargetRepoVisibility: RepoVisibility("private"),
		KeepArchive:          true,
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(flags)
	assert.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	// Test JSON unmarshaling
	var unmarshaledFlags CmdMigrateFlags
	err = json.Unmarshal(jsonData, &unmarshaledFlags)
	assert.NoError(t, err)

	assert.Equal(t, flags.TargetOrg, unmarshaledFlags.TargetOrg)
	assert.Equal(t, flags.TargetRepo, unmarshaledFlags.TargetRepo)
	assert.Equal(t, flags.KeepArchive, unmarshaledFlags.KeepArchive)
}

func TestRepoVisibilityInvalidValues(t *testing.T) {
	invalidValues := []string{
		"PUBLIC",
		"PRIVATE",
		"INTERNAL",
		"Protected",
		"secret",
		"restricted",
		"open",
		"closed",
		"123",
		"true",
		"false",
		" public",
		"public ",
		" private ",
	}

	for _, value := range invalidValues {
		t.Run("invalid_"+value, func(t *testing.T) {
			var rv RepoVisibility
			err := rv.Set(value)
			assert.Error(t, err)
		})
	}
}

func TestCmdMigrateFlagsTargetRepoInheritance(t *testing.T) {
	// Test case where target repo should default to source repo
	flags := CmdMigrateFlags{
		TargetOrg:  "target-org",
		TargetRepo: "",
	}

	assert.Empty(t, flags.TargetRepo)

	// Test case with explicit target repo
	flagsWithTarget := CmdMigrateFlags{
		TargetOrg:  "target-org",
		TargetRepo: "different-target-repo",
	}

	assert.Equal(t, "different-target-repo", flagsWithTarget.TargetRepo)
}
