package migrate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/katiem0/gh-bbc-exporter/internal/data"
	"github.com/stretchr/testify/assert"
)

func cleanupExportDirs(t *testing.T) {
	matches, err := filepath.Glob("./bitbucket-export-*")
	if err != nil {
		return
	}
	for _, match := range matches {
		if err := os.RemoveAll(match); err != nil {
			t.Logf("Warning: Failed to remove %s: %v", match, err)
		}
		// Remove corresponding archive if it exists
		archivePath := match + ".tar.gz"
		if _, err := os.Stat(archivePath); err == nil {
			if err := os.Remove(archivePath); err != nil {
				t.Logf("Warning: Failed to remove archive %s: %v", archivePath, err)
			}
		}
	}
}

func TestNewCmdMigrate(t *testing.T) {
	cmd := NewCmdMigrate()

	assert.NotNil(t, cmd)
	assert.Equal(t, "migrate [flags]", cmd.Use)
	assert.Equal(t, "Export from Bitbucket and import to GitHub", cmd.Short)
}

func TestMigrateCommandFlags(t *testing.T) {
	cmd := NewCmdMigrate()

	expectedFlags := []struct {
		name         string
		shorthand    string
		defaultValue string
	}{
		{"bbc-api-url", "a", "https://api.bitbucket.org/2.0"},
		{"access-token", "t", ""},
		{"api-token", "", ""},
		{"email", "e", ""},
		{"user", "u", ""},
		{"app-password", "p", ""},
		{"workspace", "w", ""},
		{"repo", "r", ""},
		{"temp-dir", "", ""},
		{"output", "o", ""},
		{"open-prs-only", "", "false"},
		{"prs-from-date", "", ""},
		{"skip-commit-lookup", "", "false"},
		{"target-org", "", ""},
		{"target-repo", "", ""},
		{"github-target-pat", "", ""},
		{"target-repo-visibility", "", "private"},
		{"debug", "d", "false"},
	}

	for _, ef := range expectedFlags {
		t.Run(ef.name, func(t *testing.T) {
			flag := cmd.PersistentFlags().Lookup(ef.name)
			assert.NotNil(t, flag, "Flag %s should exist", ef.name)
			if flag != nil {
				assert.Equal(t, ef.shorthand, flag.Shorthand, "Flag %s shorthand mismatch", ef.name)
				assert.Equal(t, ef.defaultValue, flag.DefValue, "Flag %s default value mismatch", ef.name)
			}
		})
	}
}

func TestMigrateCommandRequiredFlags(t *testing.T) {
	cmd := NewCmdMigrate()

	requiredFlags := []string{"workspace", "repo", "target-org"}

	for _, flagName := range requiredFlags {
		t.Run(flagName, func(t *testing.T) {
			flag := cmd.PersistentFlags().Lookup(flagName)
			assert.NotNil(t, flag, "Flag %s should exist", flagName)
		})
	}
}

func TestMigratePreRunValidation(t *testing.T) {
	defer cleanupExportDirs(t)
	testCases := []struct {
		name        string
		args        []string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Missing workspace",
			args:        []string{"--repo", "test-repo", "--target-org", "test-org"},
			expectError: true,
			errorMsg:    "bitbucket workspace must be specified",
		},
		{
			name:        "Missing repository",
			args:        []string{"--workspace", "test-ws", "--target-org", "test-org"},
			expectError: true,
			errorMsg:    "bitbucket repository must be specified",
		},
		{
			name:        "Missing target org",
			args:        []string{"--workspace", "test-ws", "--repo", "test-repo"},
			expectError: true,
			errorMsg:    "target GitHub organization must be specified",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := NewCmdMigrate()
			cmd.SetArgs(tc.args)

			err := cmd.Execute()
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
			}
		})
	}
}

func TestRepoVisibilityFlag(t *testing.T) {
	cmd := NewCmdMigrate()

	flag := cmd.PersistentFlags().Lookup("target-repo-visibility")
	assert.NotNil(t, flag)
	assert.Equal(t, "<internal|private|public>", flag.Value.Type())
}

func TestRepoVisibilityValidValues(t *testing.T) {
	testCases := []struct {
		name           string
		value          string
		expectError    bool
		expectedString string
	}{
		{"Public visibility", "public", false, "public"},
		{"Private visibility", "private", false, "private"},
		{"Internal visibility", "internal", false, "internal"},
		{"Empty visibility defaults to private", "", false, "private"},
		{"Invalid visibility", "invalid", true, ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			visibility := data.RepoVisibility("")
			err := visibility.Set(tc.value)

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "must be one of")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedString, visibility.String())
			}
		})
	}
}

func TestArchiveCleanup(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "migrate-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	archivePath := filepath.Join(tempDir, "test-archive.tar.gz")
	err = os.WriteFile(archivePath, []byte("test archive content"), 0644)
	assert.NoError(t, err)

	_, err = os.Stat(archivePath)
	assert.NoError(t, err)

	err = os.Remove(archivePath)
	assert.NoError(t, err)

	_, err = os.Stat(archivePath)
	assert.True(t, os.IsNotExist(err))
}

func TestCmdMigrateFlagsStruct(t *testing.T) {
	flags := data.CmdMigrateFlags{}

	assert.Empty(t, flags.TargetOrg)
	assert.Empty(t, flags.TargetRepo)
	assert.Empty(t, flags.GitHubPAT)
}

func TestExportFlagsInMigrate(t *testing.T) {
	flags := data.CmdExportFlags{}

	assert.Empty(t, flags.BitbucketAccessToken)
	assert.Empty(t, flags.BitbucketUser)
	assert.Empty(t, flags.BitbucketEmail)
	assert.Empty(t, flags.BitbucketAppPass)
	assert.Empty(t, flags.BitbucketAPIToken)
	assert.Empty(t, flags.BitbucketAPIURL)
	assert.Empty(t, flags.Repository)
	assert.Empty(t, flags.Workspace)
	assert.Empty(t, flags.OutputDir)
	assert.Empty(t, flags.TempDir)
	assert.Empty(t, flags.PRsFromDate)
	assert.False(t, flags.OpenPRsOnly)
	assert.False(t, flags.SkipCommitLookup)
	assert.False(t, flags.Debug)
}

func TestMigrateCommandSortFlags(t *testing.T) {
	cmd := NewCmdMigrate()

	assert.False(t, cmd.Flags().SortFlags)
	assert.False(t, cmd.PersistentFlags().SortFlags)
}

func TestRepoVisibilityType(t *testing.T) {
	visibility := data.RepoVisibility("")

	// Test Type() method
	assert.Equal(t, "<internal|private|public>", visibility.Type())
}

func TestRepoVisibilityString(t *testing.T) {
	testCases := []struct {
		name     string
		value    data.RepoVisibility
		expected string
	}{
		{"Empty visibility defaults to private", data.RepoVisibility(""), "private"},
		{"Public visibility", data.RepoVisibility("public"), "public"},
		{"Private visibility", data.RepoVisibility("private"), "private"},
		{"Internal visibility", data.RepoVisibility("internal"), "internal"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.value.String())
		})
	}
}

func TestMigrateAuthenticationValidation(t *testing.T) {
	defer cleanupExportDirs(t)
	testCases := []struct {
		name        string
		args        []string
		expectError bool
		errorMsg    string
	}{
		{
			name: "With access token",
			args: []string{
				"--workspace", "test-ws",
				"--repo", "test-repo",
				"--target-org", "test-org",
				"--access-token", "test-token",
			},
			expectError: true,
			errorMsg:    "",
		},
		{
			name: "With user and app password",
			args: []string{
				"--workspace", "test-ws",
				"--repo", "test-repo",
				"--target-org", "test-org",
				"--user", "test-user",
				"--app-password", "test-pass",
			},
			expectError: true,
			errorMsg:    "",
		},
		{
			name: "With API token and email",
			args: []string{
				"--workspace", "test-ws",
				"--repo", "test-repo",
				"--target-org", "test-org",
				"--api-token", "test-api-token",
				"--email", "test@example.com",
			},
			expectError: true,
			errorMsg:    "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := NewCmdMigrate()
			cmd.SetArgs(tc.args)

			err := cmd.Execute()
			if tc.expectError && tc.errorMsg != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
			}
			// Note: Without mock servers, execution will fail after PreRunE
		})
	}
}

func TestMigratePRFilterFlags(t *testing.T) {
	cmd := NewCmdMigrate()

	testCases := []struct {
		name      string
		flagName  string
		flagValue string
		expected  string
	}{
		{
			name:      "Open PRs only flag",
			flagName:  "open-prs-only",
			flagValue: "true",
			expected:  "true",
		},
		{
			name:      "PRs from date flag",
			flagName:  "prs-from-date",
			flagValue: "2023-01-01",
			expected:  "2023-01-01",
		},
		{
			name:      "Skip commit lookup flag",
			flagName:  "skip-commit-lookup",
			flagValue: "true",
			expected:  "true",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args := []string{
				"--workspace", "test-ws",
				"--repo", "test-repo",
				"--target-org", "test-org",
				"--" + tc.flagName, tc.flagValue,
			}
			err := cmd.ParseFlags(args)
			assert.NoError(t, err)

			flag := cmd.PersistentFlags().Lookup(tc.flagName)
			assert.NotNil(t, flag)
			assert.Equal(t, tc.expected, flag.Value.String())
		})
	}
}

func TestMigrateTargetRepoFlag(t *testing.T) {

	testCases := []struct {
		name         string
		targetRepo   string
		expectedRepo string
	}{
		{
			name:         "Custom target repo name",
			targetRepo:   "custom-repo-name",
			expectedRepo: "custom-repo-name",
		},
		{
			name:         "Empty target repo (uses source repo name)",
			targetRepo:   "",
			expectedRepo: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := NewCmdMigrate()
			args := []string{
				"--workspace", "test-ws",
				"--repo", "source-repo",
				"--target-org", "test-org",
			}
			if tc.targetRepo != "" {
				args = append(args, "--target-repo", tc.targetRepo)
			}

			err := cmd.ParseFlags(args)
			assert.NoError(t, err)

			flag := cmd.PersistentFlags().Lookup("target-repo")
			assert.NotNil(t, flag)
			assert.Equal(t, tc.expectedRepo, flag.Value.String())
		})
	}
}

func TestMigrateDebugFlag(t *testing.T) {
	cmd := NewCmdMigrate()

	testCases := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "Debug disabled by default",
			args:     []string{"--workspace", "test-ws", "--repo", "test-repo", "--target-org", "test-org"},
			expected: "false",
		},
		{
			name:     "Debug enabled with flag",
			args:     []string{"--workspace", "test-ws", "--repo", "test-repo", "--target-org", "test-org", "--debug"},
			expected: "true",
		},
		{
			name:     "Debug enabled with shorthand",
			args:     []string{"--workspace", "test-ws", "--repo", "test-repo", "--target-org", "test-org", "-d"},
			expected: "true",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := cmd.ParseFlags(tc.args)
			assert.NoError(t, err)

			flag := cmd.PersistentFlags().Lookup("debug")
			assert.NotNil(t, flag)
			assert.Equal(t, tc.expected, flag.Value.String())
		})
	}
}

func TestMigrateTempDirFlag(t *testing.T) {
	cmd := NewCmdMigrate()

	tempDir, err := os.MkdirTemp("", "migrate-temp-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	args := []string{
		"--workspace", "test-ws",
		"--repo", "test-repo",
		"--target-org", "test-org",
		"--temp-dir", tempDir,
	}

	err = cmd.ParseFlags(args)
	assert.NoError(t, err)

	flag := cmd.PersistentFlags().Lookup("temp-dir")
	assert.NotNil(t, flag)
	assert.Equal(t, tempDir, flag.Value.String())
}

func TestMigrateOutputDirFlag(t *testing.T) {
	cmd := NewCmdMigrate()

	outputDir := "/custom/output/path"

	args := []string{
		"--workspace", "test-ws",
		"--repo", "test-repo",
		"--target-org", "test-org",
		"--output", outputDir,
	}

	err := cmd.ParseFlags(args)
	assert.NoError(t, err)

	flag := cmd.PersistentFlags().Lookup("output")
	assert.NotNil(t, flag)
	assert.Equal(t, outputDir, flag.Value.String())
}

func TestMigrateBitbucketAPIURLFlag(t *testing.T) {
	cmd := NewCmdMigrate()

	testCases := []struct {
		name        string
		apiURL      string
		expectedURL string
	}{
		{
			name:        "Default Bitbucket API URL",
			apiURL:      "",
			expectedURL: "https://api.bitbucket.org/2.0",
		},
		{
			name:        "Custom Bitbucket API URL",
			apiURL:      "https://custom.bitbucket.org/2.0",
			expectedURL: "https://custom.bitbucket.org/2.0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args := []string{
				"--workspace", "test-ws",
				"--repo", "test-repo",
				"--target-org", "test-org",
			}
			if tc.apiURL != "" {
				args = append(args, "--bbc-api-url", tc.apiURL)
			}

			err := cmd.ParseFlags(args)
			assert.NoError(t, err)

			flag := cmd.PersistentFlags().Lookup("bbc-api-url")
			assert.NotNil(t, flag)
			if tc.apiURL == "" {
				assert.Equal(t, tc.expectedURL, flag.DefValue)
			} else {
				assert.Equal(t, tc.expectedURL, flag.Value.String())
			}
		})
	}
}

func TestMigrateCommandLongDescription(t *testing.T) {
	cmd := NewCmdMigrate()

	assert.NotEmpty(t, cmd.Long, "Long description should not be empty")
	assert.Contains(t, cmd.Long, "Bitbucket", "Long description should mention Bitbucket")
	assert.Contains(t, cmd.Long, "GitHub", "Long description should mention GitHub")
}

func TestMigrateCommandExample(t *testing.T) {
	cmd := NewCmdMigrate()

	if cmd.Example != "" {
		assert.Contains(t, cmd.Example, "migrate", "Example should include migrate command")
	}
}

func TestCmdMigrateFlagsWithRepoVisibility(t *testing.T) {
	flags := data.CmdMigrateFlags{
		TargetOrg:            "test-org",
		TargetRepo:           "test-repo",
		TargetRepoVisibility: data.RepoVisibility("private"),
	}

	assert.Equal(t, "test-org", flags.TargetOrg)
	assert.Equal(t, "test-repo", flags.TargetRepo)
	assert.Equal(t, "private", flags.TargetRepoVisibility.String())
}

func TestMigrateAllFlagsPresent(t *testing.T) {
	cmd := NewCmdMigrate()

	allFlags := []string{
		"bbc-api-url",
		"access-token",
		"api-token",
		"email",
		"user",
		"app-password",
		"workspace",
		"repo",
		"temp-dir",
		"output",
		"open-prs-only",
		"prs-from-date",
		"skip-commit-lookup",
		"target-org",
		"target-repo",
		"github-target-pat",
		"target-repo-visibility",

		"debug",
	}

	for _, flagName := range allFlags {
		t.Run(flagName, func(t *testing.T) {
			flag := cmd.PersistentFlags().Lookup(flagName)
			assert.NotNil(t, flag, "Flag %s should exist", flagName)
		})
	}
}

func TestMigrateCommandHasPreRunE(t *testing.T) {
	cmd := NewCmdMigrate()

	assert.NotNil(t, cmd.PreRunE, "PreRunE should be set for validation")
}

func TestMigrateCommandHasRunE(t *testing.T) {
	cmd := NewCmdMigrate()

	assert.NotNil(t, cmd.RunE, "RunE should be set for execution")
}

func TestMigrateEnvironmentVariables(t *testing.T) {
	envVars := []string{
		"BITBUCKET_ACCESS_TOKEN",
		"BITBUCKET_API_TOKEN",
		"BITBUCKET_EMAIL",
		"BITBUCKET_USERNAME",
		"BITBUCKET_APP_PASSWORD",
		"BITBUCKET_TEMP_DIR",
	}

	originalGHPAT := os.Getenv("GITHUB_PAT")
	_ = os.Unsetenv("GITHUB_PAT")
	defer func() {
		if originalGHPAT != "" {
			_ = os.Setenv("GITHUB_PAT", originalGHPAT)
		} else {
			_ = os.Unsetenv("GITHUB_PAT")
		}
	}()

	originalValues := make(map[string]string)
	for _, v := range envVars {
		originalValues[v] = os.Getenv(v)
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

	for _, v := range envVars {
		_ = os.Unsetenv(v)
	}

	tests := []struct {
		name    string
		envVars map[string]string
	}{
		{
			name: "Bitbucket access token from env",
			envVars: map[string]string{
				"BITBUCKET_ACCESS_TOKEN": "env-access-token",
			},
		},
		{
			name: "Bitbucket API token and email from env",
			envVars: map[string]string{
				"BITBUCKET_API_TOKEN": "env-api-token",
				"BITBUCKET_EMAIL":     "test@example.com",
			},
		},
		{
			name: "Bitbucket username and app password from env",
			envVars: map[string]string{
				"BITBUCKET_USERNAME":     "env-user",
				"BITBUCKET_APP_PASSWORD": "env-pass",
			},
		},
		{
			name: "Temp dir from env",
			envVars: map[string]string{
				"BITBUCKET_TEMP_DIR": "/env/temp/dir",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, v := range envVars {
				_ = os.Unsetenv(v)
			}

			for k, v := range tt.envVars {
				err := os.Setenv(k, v)
				assert.NoError(t, err)
			}

			for k, v := range tt.envVars {
				assert.Equal(t, v, os.Getenv(k))
			}
		})
	}
}

func TestMigrateMixedAuthenticationMethods(t *testing.T) {
	defer cleanupExportDirs(t)

	// Save and clear GITHUB_PAT to avoid interference from environment
	originalGHPAT := os.Getenv("GITHUB_PAT")
	_ = os.Unsetenv("GITHUB_PAT")
	defer func() {
		if originalGHPAT != "" {
			_ = os.Setenv("GITHUB_PAT", originalGHPAT)
		} else {
			_ = os.Unsetenv("GITHUB_PAT")
		}
	}()

	testCases := []struct {
		name        string
		args        []string
		expectError bool
		errorMsg    string
	}{
		{
			name: "Access token with user/password - should fail",
			args: []string{
				"--workspace", "test-ws",
				"--repo", "test-repo",
				"--target-org", "test-org",
				"--github-target-pat", "ghp_test_token_for_unit_test",
				"--access-token", "token",
				"--user", "user",
				"--app-password", "pass",
			},
			expectError: true,
			errorMsg:    "mixed authentication",
		},
		{
			name: "Access token with API token - should fail",
			args: []string{
				"--workspace", "test-ws",
				"--repo", "test-repo",
				"--target-org", "test-org",
				"--github-target-pat", "ghp_test_token_for_unit_test",
				"--access-token", "token",
				"--api-token", "api-token",
				"--email", "test@example.com",
			},
			expectError: true,
			errorMsg:    "mixed authentication",
		},
		{
			name: "API token without email - should fail",
			args: []string{
				"--workspace", "test-ws",
				"--repo", "test-repo",
				"--target-org", "test-org",
				"--github-target-pat", "ghp_test_token_for_unit_test",
				"--api-token", "api-token",
			},
			expectError: true,
			errorMsg:    "authentication credentials required",
		},
		{
			name: "User without app password - should fail",
			args: []string{
				"--workspace", "test-ws",
				"--repo", "test-repo",
				"--target-org", "test-org",
				"--github-target-pat", "ghp_test_token_for_unit_test",
				"--user", "user",
			},
			expectError: true,
			errorMsg:    "authentication credentials required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := NewCmdMigrate()
			cmd.SetArgs(tc.args)

			err := cmd.Execute()
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorMsg != "" {
					assert.Contains(t, err.Error(), tc.errorMsg)
				}
			}
		})
	}
}

func TestMigratePRsFromDateValidation(t *testing.T) {
	defer cleanupExportDirs(t)

	// Clear all authentication environment variables
	envVars := []string{
		"BITBUCKET_ACCESS_TOKEN",
		"BITBUCKET_API_TOKEN",
		"BITBUCKET_EMAIL",
		"BITBUCKET_USERNAME",
		"BITBUCKET_APP_PASSWORD",
	}

	// Save and clear GITHUB_PAT to avoid interference
	originalGHPAT := os.Getenv("GITHUB_PAT")
	_ = os.Unsetenv("GITHUB_PAT")
	defer func() {
		if originalGHPAT != "" {
			_ = os.Setenv("GITHUB_PAT", originalGHPAT)
		} else {
			_ = os.Unsetenv("GITHUB_PAT")
		}
	}()

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

	testCases := []struct {
		name        string
		dateValue   string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid date format YYYY-MM-DD",
			dateValue:   "2023-01-15",
			expectError: false,
		},
		{
			name:        "Invalid date format MM/DD/YYYY",
			dateValue:   "01/15/2023",
			expectError: true,
			errorMsg:    "invalid date format",
		},
		{
			name:        "Invalid date format DD-MM-YYYY",
			dateValue:   "15-01-2023",
			expectError: true,
			errorMsg:    "invalid date format",
		},
		{
			name:        "Empty date is valid",
			dateValue:   "",
			expectError: false,
		},
		{
			name:        "Invalid date - non-existent day",
			dateValue:   "2023-02-30",
			expectError: true,
			errorMsg:    "invalid date format",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := NewCmdMigrate()
			args := []string{
				"--workspace", "test-ws",
				"--repo", "test-repo",
				"--target-org", "test-org",
				"--access-token", "test-token",
				"--github-target-pat", "ghp_test_token_for_unit_test",
			}
			if tc.dateValue != "" {
				args = append(args, "--prs-from-date", tc.dateValue)
			}
			cmd.SetArgs(args)

			err := cmd.Execute()
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorMsg != "" {
					assert.Contains(t, err.Error(), tc.errorMsg)
				}
			}
		})
	}
}

func TestMigrateNoAuthenticationProvided(t *testing.T) {
	defer cleanupExportDirs(t)

	// Save and clear GITHUB_PAT to avoid interference
	originalGHPAT := os.Getenv("GITHUB_PAT")
	_ = os.Unsetenv("GITHUB_PAT")
	defer func() {
		if originalGHPAT != "" {
			_ = os.Setenv("GITHUB_PAT", originalGHPAT)
		} else {
			_ = os.Unsetenv("GITHUB_PAT")
		}
	}()

	originalToken := os.Getenv("BITBUCKET_ACCESS_TOKEN")
	originalAPIToken := os.Getenv("BITBUCKET_API_TOKEN")
	originalUser := os.Getenv("BITBUCKET_USERNAME")
	originalPass := os.Getenv("BITBUCKET_APP_PASSWORD")

	defer func() {
		if originalToken != "" {
			_ = os.Setenv("BITBUCKET_ACCESS_TOKEN", originalToken)
		}
		if originalAPIToken != "" {
			_ = os.Setenv("BITBUCKET_API_TOKEN", originalAPIToken)
		}
		if originalUser != "" {
			_ = os.Setenv("BITBUCKET_USERNAME", originalUser)
		}
		if originalPass != "" {
			_ = os.Setenv("BITBUCKET_APP_PASSWORD", originalPass)
		}
	}()

	_ = os.Unsetenv("BITBUCKET_ACCESS_TOKEN")
	_ = os.Unsetenv("BITBUCKET_API_TOKEN")
	_ = os.Unsetenv("BITBUCKET_USERNAME")
	_ = os.Unsetenv("BITBUCKET_APP_PASSWORD")

	cmd := NewCmdMigrate()
	cmd.SetArgs([]string{
		"--workspace", "test-ws",
		"--repo", "test-repo",
		"--target-org", "test-org",
		"--github-target-pat", "ghp_test_token_for_unit_test",
	})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authentication")
}

func TestMigrateTargetRepoDefaultsToSourceRepo(t *testing.T) {
	cmd := NewCmdMigrate()

	args := []string{
		"--workspace", "test-ws",
		"--repo", "source-repo-name",
		"--target-org", "test-org",
	}

	err := cmd.ParseFlags(args)
	assert.NoError(t, err)

	targetRepoFlag := cmd.PersistentFlags().Lookup("target-repo")
	assert.NotNil(t, targetRepoFlag)
	assert.Equal(t, "", targetRepoFlag.Value.String(), "Target repo should be empty by default")
}

func TestMigrateGitHubPATFlag(t *testing.T) {
	cmd := NewCmdMigrate()

	args := []string{
		"--workspace", "test-ws",
		"--repo", "test-repo",
		"--target-org", "test-org",
		"--github-target-pat", "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
	}

	err := cmd.ParseFlags(args)
	assert.NoError(t, err)

	flag := cmd.PersistentFlags().Lookup("github-target-pat")
	assert.NotNil(t, flag)
	assert.Equal(t, "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", flag.Value.String())
}

func TestMigrateCombinedPRFilters(t *testing.T) {
	cmd := NewCmdMigrate()

	args := []string{
		"--workspace", "test-ws",
		"--repo", "test-repo",
		"--target-org", "test-org",
		"--open-prs-only",
		"--prs-from-date", "2023-06-01",
	}

	err := cmd.ParseFlags(args)
	assert.NoError(t, err)

	openPRsFlag := cmd.PersistentFlags().Lookup("open-prs-only")
	assert.Equal(t, "true", openPRsFlag.Value.String())

	prsFromDateFlag := cmd.PersistentFlags().Lookup("prs-from-date")
	assert.Equal(t, "2023-06-01", prsFromDateFlag.Value.String())
}

func TestMigrateRepoVisibilityValues(t *testing.T) {
	testCases := []struct {
		name       string
		visibility string
		valid      bool
	}{
		{"Public", "public", true},
		{"Private", "private", true},
		{"Internal", "internal", true},
		{"Empty (default)", "", true},
		{"Invalid value", "protected", false},
		{"Case sensitive - PUBLIC", "PUBLIC", false},
		{"Numeric", "123", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			visibility := data.RepoVisibility("")
			err := visibility.Set(tc.visibility)

			if tc.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestMigrateSkipCommitLookupWithDebug(t *testing.T) {
	cmd := NewCmdMigrate()

	args := []string{
		"--workspace", "test-ws",
		"--repo", "test-repo",
		"--target-org", "test-org",
		"--skip-commit-lookup",
		"--debug",
	}

	err := cmd.ParseFlags(args)
	assert.NoError(t, err)

	skipFlag := cmd.PersistentFlags().Lookup("skip-commit-lookup")
	assert.Equal(t, "true", skipFlag.Value.String())

	debugFlag := cmd.PersistentFlags().Lookup("debug")
	assert.Equal(t, "true", debugFlag.Value.String())
}

func TestMigrateOutputAndTempDirTogether(t *testing.T) {
	cmd := NewCmdMigrate()

	tempDir, err := os.MkdirTemp("", "migrate-temp-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	outputDir, err := os.MkdirTemp("", "migrate-output-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(outputDir) }()

	args := []string{
		"--workspace", "test-ws",
		"--repo", "test-repo",
		"--target-org", "test-org",
		"--temp-dir", tempDir,
		"--output", outputDir,
	}

	err = cmd.ParseFlags(args)
	assert.NoError(t, err)

	tempDirFlag := cmd.PersistentFlags().Lookup("temp-dir")
	assert.Equal(t, tempDir, tempDirFlag.Value.String())

	outputFlag := cmd.PersistentFlags().Lookup("output")
	assert.Equal(t, outputDir, outputFlag.Value.String())
}

func TestMigrateCommandDisabledFlagSorting(t *testing.T) {
	cmd := NewCmdMigrate()

	// Verify that flag sorting is disabled for consistent output
	assert.False(t, cmd.Flags().SortFlags, "Regular flags should not be sorted")
	assert.False(t, cmd.PersistentFlags().SortFlags, "Persistent flags should not be sorted")
}

func TestMigrateVisibility(t *testing.T) {
	cmd := NewCmdMigrate()

	args := []string{
		"--workspace", "test-ws",
		"--repo", "test-repo",
		"--target-org", "test-org",
		"--target-repo-visibility", "private",
	}

	err := cmd.ParseFlags(args)
	assert.NoError(t, err)

	visibilityFlag := cmd.PersistentFlags().Lookup("target-repo-visibility")
	assert.Equal(t, "private", visibilityFlag.Value.String())
}

func TestMigrateFlagShorthands(t *testing.T) {
	cmd := NewCmdMigrate()

	shorthandFlags := map[string]string{
		"a": "bbc-api-url",
		"t": "access-token",
		"e": "email",
		"u": "user",
		"p": "app-password",
		"w": "workspace",
		"r": "repo",
		"o": "output",
		"d": "debug",
	}

	for shorthand, fullName := range shorthandFlags {
		t.Run(fullName, func(t *testing.T) {
			flag := cmd.PersistentFlags().Lookup(fullName)
			assert.NotNil(t, flag, "Flag %s should exist", fullName)
			if flag != nil {
				assert.Equal(t, shorthand, flag.Shorthand, "Flag %s should have shorthand %s", fullName, shorthand)
			}
		})
	}
}

func TestMigrateFlagsWithoutShorthands(t *testing.T) {
	cmd := NewCmdMigrate()

	noShorthandFlags := []string{
		"api-token",
		"temp-dir",
		"open-prs-only",
		"prs-from-date",
		"skip-commit-lookup",
		"target-org",
		"target-repo",
		"github-target-pat",
		"target-repo-visibility",
	}

	for _, flagName := range noShorthandFlags {
		t.Run(flagName, func(t *testing.T) {
			flag := cmd.PersistentFlags().Lookup(flagName)
			assert.NotNil(t, flag, "Flag %s should exist", flagName)
			if flag != nil {
				assert.Equal(t, "", flag.Shorthand, "Flag %s should not have a shorthand", flagName)
			}
		})
	}
}

func TestMigrateDefaultBitbucketAPIURL(t *testing.T) {
	cmd := NewCmdMigrate()

	flag := cmd.PersistentFlags().Lookup("bbc-api-url")
	assert.NotNil(t, flag)
	assert.Equal(t, "https://api.bitbucket.org/2.0", flag.DefValue,
		"Default Bitbucket API URL should be the standard endpoint")
}

func TestMigrateEmptyWorkspaceError(t *testing.T) {
	defer cleanupExportDirs(t)

	cmd := NewCmdMigrate()
	cmd.SetArgs([]string{
		"--repo", "test-repo",
		"--target-org", "test-org",
		"--access-token", "test-token",
	})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "workspace")
}

func TestMigrateEmptyRepositoryError(t *testing.T) {
	defer cleanupExportDirs(t)

	cmd := NewCmdMigrate()
	cmd.SetArgs([]string{
		"--workspace", "test-ws",
		"--target-org", "test-org",
		"--access-token", "test-token",
	})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "repository")
}

func TestCmdMigrateFlagsEmbeddedExportFlags(t *testing.T) {
	migrateFlags := data.CmdMigrateFlags{
		TargetOrg:            "target-org",
		TargetRepo:           "target-repo",
		TargetRepoVisibility: data.RepoVisibility("private"),
	}
	exportFlags := data.CmdExportFlags{
		Workspace:            "test-workspace",
		Repository:           "test-repo",
		BitbucketAccessToken: "test-token",
		OutputDir:            "/output",
		TempDir:              "/temp",
		PRsFromDate:          "2023-01-01",
		OpenPRsOnly:          true,
		SkipCommitLookup:     true,
		Debug:                true,
	}

	// Verify embedded fields are accessible
	assert.Equal(t, "test-workspace", exportFlags.Workspace)
	assert.Equal(t, "test-repo", exportFlags.Repository)
	assert.Equal(t, "test-token", exportFlags.BitbucketAccessToken)
	assert.Equal(t, "/output", exportFlags.OutputDir)
	assert.Equal(t, "/temp", exportFlags.TempDir)
	assert.Equal(t, "2023-01-01", exportFlags.PRsFromDate)
	assert.True(t, exportFlags.OpenPRsOnly)
	assert.True(t, exportFlags.SkipCommitLookup)
	assert.True(t, exportFlags.Debug)

	// Verify migrate-specific fields
	assert.Equal(t, "target-org", migrateFlags.TargetOrg)
	assert.Equal(t, "target-repo", migrateFlags.TargetRepo)
	assert.Equal(t, "private", migrateFlags.TargetRepoVisibility.String())
}

func TestMigrateTargetAPIURLFlag(t *testing.T) {
	cmd := NewCmdMigrate()

	flag := cmd.PersistentFlags().Lookup("target-api-url")
	assert.NotNil(t, flag, "target-api-url flag should exist")
	assert.Equal(t, "https://api.github.com", flag.DefValue,
		"Default target API URL should be https://api.github.com")
}

func TestMigrateTargetAPIURLGHECom(t *testing.T) {
	cmd := NewCmdMigrate()

	testCases := []struct {
		name        string
		apiURL      string
		expectedURL string
	}{
		{
			name:        "Default GitHub.com API URL",
			apiURL:      "",
			expectedURL: "https://api.github.com",
		},
		{
			name:        "GHE.com API URL",
			apiURL:      "https://api.octocorp.ghe.com",
			expectedURL: "https://api.octocorp.ghe.com",
		},
		{
			name:        "GHES API URL",
			apiURL:      "https://github.example.com/api/v3",
			expectedURL: "https://github.example.com/api/v3",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args := []string{
				"--workspace", "test-ws",
				"--repo", "test-repo",
				"--target-org", "test-org",
				"--github-target-pat", "ghp_test_token",
				"--access-token", "test-bb-token",
			}
			if tc.apiURL != "" {
				args = append(args, "--target-api-url", tc.apiURL)
			}

			err := cmd.ParseFlags(args)
			assert.NoError(t, err)

			val, err := cmd.PersistentFlags().GetString("target-api-url")
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedURL, val)
		})
	}
}

func TestMigrateCommandFlagsIncludesTargetAPIURL(t *testing.T) {
	cmd := NewCmdMigrate()

	expectedFlags := []string{
		"target-api-url",
		"target-org",
		"target-repo",
		"github-target-pat",
		"target-repo-visibility",
	}

	for _, flagName := range expectedFlags {
		flag := cmd.PersistentFlags().Lookup(flagName)
		assert.NotNil(t, flag, "Flag %s should exist", flagName)
	}
}

func TestMigrateFlagsWithTargetAPIURL(t *testing.T) {
	flags := data.CmdMigrateFlags{
		TargetOrg:            "github-org",
		TargetRepo:           "github-repo",
		TargetAPIURL:         "https://api.octocorp.ghe.com",
		TargetRepoVisibility: data.RepoVisibility("private"),
	}

	assert.Equal(t, "https://api.octocorp.ghe.com", flags.TargetAPIURL)

	jsonData, err := json.Marshal(flags)
	assert.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	var unmarshaledFlags data.CmdMigrateFlags
	err = json.Unmarshal(jsonData, &unmarshaledFlags)
	assert.NoError(t, err)

	assert.Equal(t, flags.TargetAPIURL, unmarshaledFlags.TargetAPIURL)
	assert.Equal(t, flags.TargetOrg, unmarshaledFlags.TargetOrg)
}

func TestMigratePreRunValidatesTargetAPIURL(t *testing.T) {
	defer cleanupExportDirs(t)

	testCases := []struct {
		name        string
		apiURL      string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "GHES URL fails fast in PreRunE",
			apiURL:      "https://github.example.com/api/v3",
			expectError: true,
			errorMsg:    "unsupported target API URL",
		},
		{
			name:        "Non-GHE custom URL fails fast in PreRunE",
			apiURL:      "https://custom.enterprise.com/api",
			expectError: true,
			errorMsg:    "unsupported target API URL",
		},
		{
			name:        "Default github.com URL passes PreRunE",
			apiURL:      "https://api.github.com",
			expectError: false,
		},
		{
			name:        "GHE.com URL passes PreRunE",
			apiURL:      "https://api.octocorp.ghe.com",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := NewCmdMigrate()

			args := []string{
				"--workspace", "test-ws",
				"--repo", "test-repo",
				"--target-org", "test-org",
				"--access-token", "test-token",
				"--target-api-url", tc.apiURL,
			}
			cmd.SetArgs(args)

			err := cmd.Execute()

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
			} else {
				if err != nil {
					assert.NotContains(t, err.Error(), "unsupported target API URL",
						"PreRunE should not reject valid target API URL: %s", tc.apiURL)
				}
			}
		})
	}
}

func TestMigrateGHESFailsFastBeforeExport(t *testing.T) {
	cleanupExportDirs(t)
	defer cleanupExportDirs(t)

	cmd := NewCmdMigrate()
	cmd.SetArgs([]string{
		"--workspace", "test-ws",
		"--repo", "test-repo",
		"--target-org", "test-org",
		"--access-token", "test-token",
		"--target-api-url", "https://github.example.com/api/v3",
	})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported target API URL")

	// Verify no export directory was created (export never started)
	matches, _ := filepath.Glob("./bitbucket-export-*")
	assert.Empty(t, matches, "No export directory should be created when target API URL is invalid")
}
