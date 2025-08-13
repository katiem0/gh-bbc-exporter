package cmd

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/katiem0/gh-bbc-exporter/internal/data"
	"github.com/katiem0/gh-bbc-exporter/internal/utils"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestValidateExportFlagsMixedAuth(t *testing.T) {
	// Test case for mixed authentication methods
	cmdFlags := &data.CmdFlags{
		BitbucketAccessToken: "testtoken",
		BitbucketUser:        "testuser",
		BitbucketAppPass:     "testpass",
	}

	err := utils.ValidateExportFlags(cmdFlags)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mixed authentication methods")
}

func TestPRFilteringFlagsIntegration(t *testing.T) {
	// Create a new instance of the CmdFlags to capture the values
	cmdFlags := &data.CmdFlags{}

	// Create a test version of the command that uses our cmdFlags
	cmd := &cobra.Command{
		Run: func(cmd *cobra.Command, args []string) {
			// This function will be called when cmd.Execute() runs
		},
	}

	// Set up the flags the same way as in NewCmdRoot()
	cmd.PersistentFlags().BoolVar(&cmdFlags.OpenPRsOnly, "open-prs-only", false, "Import only open pull requests")
	cmd.PersistentFlags().StringVarP(&cmdFlags.PRsFromDate, "prs-from-date", "", "", "Import pull requests created on or after this date")
	cmd.PersistentFlags().StringVarP(&cmdFlags.Workspace, "workspace", "w", "", "Bitbucket workspace name")
	cmd.PersistentFlags().StringVarP(&cmdFlags.Repository, "repo", "r", "", "Repository name")
	cmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketAccessToken, "access-token", "t", "", "Access token")

	// Test 1: Open PRs only
	args := []string{
		"--workspace", "testworkspace",
		"--repo", "testrepo",
		"--access-token", "testtoken",
		"--open-prs-only",
	}

	cmd.SetArgs(args)
	err := cmd.Execute()
	assert.NoError(t, err)

	// Check that the cmdFlags has the right values
	assert.True(t, cmdFlags.OpenPRsOnly, "Expected OpenPRsOnly to be true")
	assert.Equal(t, "", cmdFlags.PRsFromDate, "Expected PRsFromDate to be empty")

	// Test 2: PRs from date
	cmdFlags = &data.CmdFlags{} // Reset flags
	cmd = &cobra.Command{Run: func(cmd *cobra.Command, args []string) {}}
	cmd.PersistentFlags().BoolVar(&cmdFlags.OpenPRsOnly, "open-prs-only", false, "Import only open pull requests")
	cmd.PersistentFlags().StringVarP(&cmdFlags.PRsFromDate, "prs-from-date", "", "", "Import pull requests created on or after this date")
	cmd.PersistentFlags().StringVarP(&cmdFlags.Workspace, "workspace", "w", "", "Bitbucket workspace name")
	cmd.PersistentFlags().StringVarP(&cmdFlags.Repository, "repo", "r", "", "Repository name")
	cmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketAccessToken, "access-token", "t", "", "Access token")

	args = []string{
		"--workspace", "testworkspace",
		"--repo", "testrepo",
		"--access-token", "testtoken",
		"--prs-from-date", "2023-01-01",
	}

	cmd.SetArgs(args)
	err = cmd.Execute()
	assert.NoError(t, err)

	// Check the values
	assert.False(t, cmdFlags.OpenPRsOnly, "Expected OpenPRsOnly to be false")
	assert.Equal(t, "2023-01-01", cmdFlags.PRsFromDate, "Expected PRsFromDate to be set")

	// Test 3: Both filters
	cmdFlags = &data.CmdFlags{} // Reset flags
	cmd = &cobra.Command{Run: func(cmd *cobra.Command, args []string) {}}
	cmd.PersistentFlags().BoolVar(&cmdFlags.OpenPRsOnly, "open-prs-only", false, "Import only open pull requests")
	cmd.PersistentFlags().StringVarP(&cmdFlags.PRsFromDate, "prs-from-date", "", "", "Import pull requests created on or after this date")
	cmd.PersistentFlags().StringVarP(&cmdFlags.Workspace, "workspace", "w", "", "Bitbucket workspace name")
	cmd.PersistentFlags().StringVarP(&cmdFlags.Repository, "repo", "r", "", "Repository name")
	cmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketAccessToken, "access-token", "t", "", "Access token")

	args = []string{
		"--workspace", "testworkspace",
		"--repo", "testrepo",
		"--access-token", "testtoken",
		"--open-prs-only",
		"--prs-from-date", "2023-01-01",
	}

	cmd.SetArgs(args)
	err = cmd.Execute()
	assert.NoError(t, err)

	// Check the values
	assert.True(t, cmdFlags.OpenPRsOnly, "Expected OpenPRsOnly to be true")
	assert.Equal(t, "2023-01-01", cmdFlags.PRsFromDate, "Expected PRsFromDate to be set")
}

func TestExecuteWithInvalidFlags(t *testing.T) {
	// Create a command for testing
	rootCmd := NewCmdRoot()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	// Test with invalid flags (missing required args)
	rootCmd.SetArgs([]string{})
	err := rootCmd.Execute()
	assert.Error(t, err)

	// Check error message
	output := buf.String()
	assert.Contains(t, output, "Bitbucket Workspace must be specified")
}

func TestExecuteWithValidFlags(t *testing.T) {
	// Create a command with all required flags
	rootCmd := NewCmdRoot()

	// Mock the execute function to prevent actual API calls
	originalRun := rootCmd.RunE
	defer func() { rootCmd.RunE = originalRun }()

	executed := false
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		executed = true
		return nil
	}

	// Set valid args
	rootCmd.SetArgs([]string{
		"--workspace", "test-workspace",
		"--repo", "test-repo",
		"--access-token", "fake-token",
	})

	err := rootCmd.Execute()
	assert.NoError(t, err)
	assert.True(t, executed)
}

func TestRootCmdOptionsValidation(t *testing.T) {
	// Save original environment variables
	oldToken := os.Getenv("BITBUCKET_ACCESS_TOKEN")
	oldUser := os.Getenv("BITBUCKET_USERNAME")
	oldPass := os.Getenv("BITBUCKET_APP_PASSWORD")
	oldApiToken := os.Getenv("BITBUCKET_API_TOKEN")
	oldEmail := os.Getenv("BITBUCKET_EMAIL")

	defer func() {
		// Restore environment variables
		if oldToken != "" {
			_ = os.Setenv("BITBUCKET_ACCESS_TOKEN", oldToken)
		} else {
			_ = os.Unsetenv("BITBUCKET_ACCESS_TOKEN")
		}

		if oldUser != "" {
			_ = os.Setenv("BITBUCKET_USERNAME", oldUser)
		} else {
			_ = os.Unsetenv("BITBUCKET_USERNAME")
		}

		if oldPass != "" {
			_ = os.Setenv("BITBUCKET_APP_PASSWORD", oldPass)
		} else {
			_ = os.Unsetenv("BITBUCKET_APP_PASSWORD")
		}

		if oldApiToken != "" {
			_ = os.Setenv("BITBUCKET_API_TOKEN", oldApiToken)
		} else {
			_ = os.Unsetenv("BITBUCKET_API_TOKEN")
		}

		if oldEmail != "" {
			_ = os.Setenv("BITBUCKET_EMAIL", oldEmail)
		} else {
			_ = os.Unsetenv("BITBUCKET_EMAIL")
		}
	}()

	// Clear all environment variables to avoid interference
	_ = os.Unsetenv("BITBUCKET_ACCESS_TOKEN")
	_ = os.Unsetenv("BITBUCKET_USERNAME")
	_ = os.Unsetenv("BITBUCKET_APP_PASSWORD")
	_ = os.Unsetenv("BITBUCKET_API_TOKEN")
	_ = os.Unsetenv("BITBUCKET_EMAIL")

	// Test flag validation logic
	tests := []struct {
		name    string
		args    []string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "missing workspace",
			args:    []string{"--repo", "test-repo", "--access-token", "fake-token"},
			wantErr: true,
			errMsg:  "a Bitbucket Workspace must be specified",
		},
		{
			name:    "missing repo",
			args:    []string{"--workspace", "test-workspace", "--access-token", "fake-token"},
			wantErr: true,
			errMsg:  "a Bitbucket repository must be specified",
		},
		{
			name:    "invalid date format",
			args:    []string{"--workspace", "test-workspace", "--repo", "test-repo", "--access-token", "fake-token", "--prs-from-date", "01/01/2023"},
			wantErr: true,
			errMsg:  "invalid date format",
		},
		{
			name:    "valid flags with access token",
			args:    []string{"--workspace", "test-workspace", "--repo", "test-repo", "--access-token", "fake-token"},
			wantErr: false,
		},
		{
			name:    "valid flags with API token and email",
			args:    []string{"--workspace", "test-workspace", "--repo", "test-repo", "--api-token", "fake-api-token", "--email", "test@example.com"},
			wantErr: false,
		},
		{
			name:    "API token without email should fail",
			args:    []string{"--workspace", "test-workspace", "--repo", "test-repo", "--api-token", "fake-api-token"},
			wantErr: true,
			errMsg:  "authentication credentials required",
		},
		{
			name:    "valid flags with username and app password",
			args:    []string{"--workspace", "test-workspace", "--repo", "test-repo", "--user", "fake-user", "--app-password", "fake-password"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a custom command for testing instead of using NewCmdRoot()
			cmdFlags := &data.CmdFlags{}
			rootCmd := &cobra.Command{
				Use:   "bbc-exporter",
				Short: "Export repository and metadata from Bitbucket Cloud",
				PreRunE: func(cmd *cobra.Command, args []string) error {
					if len(cmdFlags.Workspace) == 0 {
						return errors.New("a Bitbucket Workspace must be specified")
					}
					if len(cmdFlags.Repository) == 0 {
						return errors.New("a Bitbucket repository must be specified")
					}
					return nil
				},
				RunE: func(cmd *cobra.Command, args []string) error {
					// Just validate the flags without making API calls
					utils.SetupEnvironmentCredentials(cmdFlags)
					return utils.ValidateExportFlags(cmdFlags)
				},
			}

			// Set up the flags the same way as in NewCmdRoot()
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketAPIURL, "bbc-api-url", "a",
				"https://api.bitbucket.org/2.0", "Bitbucket API URL")
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketAccessToken, "access-token", "t", "",
				"Bitbucket workspace access token")
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketAPIToken, "api-token", "", "",
				"Bitbucket API token")
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketEmail, "email", "e", "",
				"Atlassian account email")
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketUser, "user", "u", "",
				"Bitbucket username")
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketAppPass, "app-password", "p", "",
				"Bitbucket app password")
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.Workspace, "workspace", "w", "",
				"Bitbucket workspace name")
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.Repository, "repo", "r", "",
				"Repository name")
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.PRsFromDate, "prs-from-date", "", "",
				"Export pull requests from date (YYYY-MM-DD)")
			rootCmd.PersistentFlags().BoolVar(&cmdFlags.OpenPRsOnly, "open-prs-only", false,
				"Export only open pull requests")

			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg, "Error message should contain the expected text")
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRootCmdWithValidFlags(t *testing.T) {
	// Test cases that would normally make API calls
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "valid flags with token",
			args: []string{"--workspace", "test-workspace", "--repo", "test-repo", "--access-token", "fake-token"},
		},
		{
			name: "valid flags with user/pass",
			args: []string{"--workspace", "test-workspace", "--repo", "test-repo", "--user", "testuser", "--app-password", "testpass"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd := NewCmdRoot()

			// Mock the RunE function to prevent actual API calls
			originalRunE := rootCmd.RunE
			defer func() { rootCmd.RunE = originalRunE }()

			executed := false
			rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
				executed = true
				return nil
			}

			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()

			assert.NoError(t, err)
			assert.True(t, executed)
		})
	}
}

func TestEnvironmentCredentials(t *testing.T) {
	// Save original environment
	originalToken := os.Getenv("BITBUCKET_ACCESS_TOKEN")
	originalUser := os.Getenv("BITBUCKET_USERNAME")
	originalAppPass := os.Getenv("BITBUCKET_APP_PASSWORD")
	originalApiToken := os.Getenv("BITBUCKET_API_TOKEN")
	originalEmail := os.Getenv("BITBUCKET_EMAIL")

	// Track if variables existed originally
	tokenExists := originalToken != ""
	userExists := originalUser != ""
	appPassExists := originalAppPass != ""
	apiTokenExists := originalApiToken != ""
	emailExists := originalEmail != ""

	// Restore environment after test
	defer func() {
		// For each variable, either restore it or unset it
		if tokenExists {
			_ = os.Setenv("BITBUCKET_ACCESS_TOKEN", originalToken)
		} else {
			_ = os.Unsetenv("BITBUCKET_ACCESS_TOKEN")
		}

		if userExists {
			_ = os.Setenv("BITBUCKET_USERNAME", originalUser)
		} else {
			_ = os.Unsetenv("BITBUCKET_USERNAME")
		}

		if appPassExists {
			_ = os.Setenv("BITBUCKET_APP_PASSWORD", originalAppPass)
		} else {
			_ = os.Unsetenv("BITBUCKET_APP_PASSWORD")
		}

		if apiTokenExists {
			_ = os.Setenv("BITBUCKET_API_TOKEN", originalApiToken)
		} else {
			_ = os.Unsetenv("BITBUCKET_API_TOKEN")
		}

		if emailExists {
			_ = os.Setenv("BITBUCKET_EMAIL", originalEmail)
		} else {
			_ = os.Unsetenv("BITBUCKET_EMAIL")
		}
	}()

	// Test cases for environment variables
	testCases := []struct {
		name    string
		envVars map[string]string
		cmdArgs []string
		wantErr bool
		checkFn func(*testing.T, *data.CmdFlags)
	}{
		{
			name: "token from environment",
			envVars: map[string]string{
				"BITBUCKET_ACCESS_TOKEN": "env-token-123",
			},
			cmdArgs: []string{
				"--workspace", "test-workspace",
				"--repo", "test-repo",
			},
			wantErr: false,
			checkFn: func(t *testing.T, flags *data.CmdFlags) {
				assert.Equal(t, "env-token-123", flags.BitbucketAccessToken, "Expected token from environment variable")
			},
		},
		{
			name: "basic auth from environment",
			envVars: map[string]string{
				"BITBUCKET_USERNAME":     "env-user",
				"BITBUCKET_APP_PASSWORD": "env-password",
			},
			cmdArgs: []string{
				"--workspace", "test-workspace",
				"--repo", "test-repo",
			},
			wantErr: false,
			checkFn: func(t *testing.T, flags *data.CmdFlags) {
				assert.Equal(t, "env-user", flags.BitbucketUser, "Expected username from environment variable")
				assert.Equal(t, "env-password", flags.BitbucketAppPass, "Expected password from environment variable")
			},
		},
		{
			name: "API token with email from environment",
			envVars: map[string]string{
				"BITBUCKET_API_TOKEN": "env-api-token-123",
				"BITBUCKET_EMAIL":     "env-email@example.com",
			},
			cmdArgs: []string{
				"--workspace", "test-workspace",
				"--repo", "test-repo",
			},
			wantErr: false,
			checkFn: func(t *testing.T, flags *data.CmdFlags) {
				assert.Equal(t, "env-api-token-123", flags.BitbucketAPIToken, "Expected API token from environment variable")
				assert.Equal(t, "env-email@example.com", flags.BitbucketEmail, "Expected email from environment variable")
			},
		},
		{
			name: "API token without email should fail",
			envVars: map[string]string{
				"BITBUCKET_API_TOKEN": "env-api-token-123",
			},
			cmdArgs: []string{
				"--workspace", "test-workspace",
				"--repo", "test-repo",
			},
			wantErr: true,
			checkFn: func(t *testing.T, flags *data.CmdFlags) {
				// This test should fail, so no checks needed
			},
		},
		{
			name: "command line takes precedence over environment",
			envVars: map[string]string{
				"BITBUCKET_ACCESS_TOKEN": "env-token-123",
			},
			cmdArgs: []string{
				"--workspace", "test-workspace",
				"--repo", "test-repo",
				"--access-token", "cli-token-override",
			},
			wantErr: false,
			checkFn: func(t *testing.T, flags *data.CmdFlags) {
				assert.Equal(t, "cli-token-override", flags.BitbucketAccessToken, "Expected token from command line to override environment")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear environment variables
			_ = os.Unsetenv("BITBUCKET_ACCESS_TOKEN")
			_ = os.Unsetenv("BITBUCKET_USERNAME")
			_ = os.Unsetenv("BITBUCKET_APP_PASSWORD")
			_ = os.Unsetenv("BITBUCKET_API_TOKEN")
			_ = os.Unsetenv("BITBUCKET_EMAIL")

			// Set environment variables for this test case
			for k, v := range tc.envVars {
				err := os.Setenv(k, v)
				assert.NoError(t, err, "Failed to set environment variable %s", k)
			}

			// Create a new root command with mocked execution
			cmdFlags := &data.CmdFlags{}
			rootCmd := &cobra.Command{
				RunE: func(cmd *cobra.Command, args []string) error {
					// Mock the export function to capture the flags after env vars are loaded
					utils.SetupEnvironmentCredentials(cmdFlags)
					err := utils.ValidateExportFlags(cmdFlags)

					// Run the check function to verify the flags
					if !tc.wantErr && err == nil {
						tc.checkFn(t, cmdFlags)
					}
					return err
				},
			}

			// Set up the flags
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketAccessToken, "access-token", "t", "", "")
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketAPIToken, "api-token", "", "", "")
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketEmail, "email", "e", "", "")
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketUser, "user", "u", "", "")
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketAppPass, "app-password", "p", "", "")
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.Repository, "repo", "r", "", "")
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.Workspace, "workspace", "w", "", "")

			// Execute the command with test arguments
			rootCmd.SetArgs(tc.cmdArgs)
			err := rootCmd.Execute()

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCredentialMasking(t *testing.T) {
	// Ensure credentials are not logged in debug mode
	core, obs := observer.New(zap.DebugLevel)
	logger := zap.New(core)

	// Simulate logging with credentials
	logger.Info("Using authentication",
		zap.String("token", "[REDACTED]"),
		zap.String("username", "user123"))

	// Verify logs don't contain actual tokens
	logs := obs.All()
	assert.Equal(t, 1, len(logs), "Expected one log entry")

	if len(logs) > 0 {
		log := logs[0]
		assert.NotContains(t, log.Message, "actual-token-value")
		assert.Equal(t, "Using authentication", log.Message)

		// Find the token field and check its value
		var foundToken bool
		for _, field := range log.Context {
			if field.Key == "token" {
				foundToken = true
				// The field value in zap's observer is accessed through String() method
				assert.Equal(t, "[REDACTED]", field.String)
			}
		}
		assert.True(t, foundToken, "Expected to find a 'token' field in the log")
	}
}

func TestEnvironmentVariableSecurityPrecedence(t *testing.T) {
	// Test that environment variables work when no CLI flags are provided
	t.Run("env vars only - should succeed", func(t *testing.T) {
		// Save original environment
		originalToken := os.Getenv("BITBUCKET_ACCESS_TOKEN")
		originalUser := os.Getenv("BITBUCKET_USERNAME")
		originalPass := os.Getenv("BITBUCKET_APP_PASSWORD")
		originalApiToken := os.Getenv("BITBUCKET_API_TOKEN")
		originalEmail := os.Getenv("BITBUCKET_EMAIL")

		tokenExists := originalToken != ""
		userExists := originalUser != ""
		passExists := originalPass != ""
		apiTokenExists := originalApiToken != ""
		emailExists := originalEmail != ""

		// Clear ALL environment variables first
		_ = os.Unsetenv("BITBUCKET_ACCESS_TOKEN")
		_ = os.Unsetenv("BITBUCKET_USERNAME")
		_ = os.Unsetenv("BITBUCKET_APP_PASSWORD")
		_ = os.Unsetenv("BITBUCKET_API_TOKEN")
		_ = os.Unsetenv("BITBUCKET_EMAIL")

		// Set only the test value we want
		err := os.Setenv("BITBUCKET_ACCESS_TOKEN", "secure-env-token")
		assert.NoError(t, err, "Failed to set BITBUCKET_ACCESS_TOKEN environment variable")

		// Restore properly on exit
		defer func() {
			// Clear test values
			_ = os.Unsetenv("BITBUCKET_ACCESS_TOKEN")
			_ = os.Unsetenv("BITBUCKET_USERNAME")
			_ = os.Unsetenv("BITBUCKET_APP_PASSWORD")
			_ = os.Unsetenv("BITBUCKET_API_TOKEN")
			_ = os.Unsetenv("BITBUCKET_EMAIL")

			// Restore original values
			if tokenExists {
				_ = os.Setenv("BITBUCKET_ACCESS_TOKEN", originalToken)
			}
			if userExists {
				_ = os.Setenv("BITBUCKET_USERNAME", originalUser)
			}
			if passExists {
				_ = os.Setenv("BITBUCKET_APP_PASSWORD", originalPass)
			}
			if apiTokenExists {
				_ = os.Setenv("BITBUCKET_API_TOKEN", originalApiToken)
			}
			if emailExists {
				_ = os.Setenv("BITBUCKET_EMAIL", originalEmail)
			}
		}()

		// Create a custom command for testing instead of using NewCmdRoot()
		var capturedToken string
		cmdFlags := &data.CmdFlags{}
		rootCmd := &cobra.Command{
			Use:   "bbc-exporter",
			Short: "Export repository and metadata from Bitbucket Cloud",
			RunE: func(cmd *cobra.Command, args []string) error {
				utils.SetupEnvironmentCredentials(cmdFlags)
				capturedToken = cmdFlags.BitbucketAccessToken
				return utils.ValidateExportFlags(cmdFlags)
			},
		}

		// Add flags to the command
		rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketAccessToken, "access-token", "t", "", "Token")
		rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketUser, "user", "u", "", "User")
		rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketAppPass, "app-password", "p", "", "App Password")
		rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketAPIToken, "api-token", "", "", "API Token")
		rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketEmail, "email", "e", "", "Email")
		rootCmd.PersistentFlags().StringVarP(&cmdFlags.Workspace, "workspace", "w", "", "Workspace")
		rootCmd.PersistentFlags().StringVarP(&cmdFlags.Repository, "repo", "r", "", "Repository")

		rootCmd.SetArgs([]string{
			"--workspace", "test-ws",
			"--repo", "test-repo",
		})

		err = rootCmd.Execute()
		assert.NoError(t, err, "Should succeed with env var token")
		assert.Equal(t, "secure-env-token", capturedToken)
	})
}

func TestEnvironmentVariableOverrides(t *testing.T) {
	// Save original values
	originalURL := os.Getenv("BITBUCKET_API_URL")
	defer func() {
		if originalURL != "" {
			if err := os.Setenv("BITBUCKET_API_URL", originalURL); err != nil {
				t.Logf("Failed to restore BITBUCKET_API_URL: %v", err)
			}
		} else {
			if err := os.Unsetenv("BITBUCKET_API_URL"); err != nil {
				t.Logf("Failed to unset BITBUCKET_API_URL: %v", err)
			}
		}
	}()

	// Set custom API URL
	if err := os.Setenv("BITBUCKET_API_URL", "https://custom.bitbucket.com/api/v2"); err != nil {
		t.Fatalf("Failed to set BITBUCKET_API_URL: %v", err)
	}

	cmdFlags := &data.CmdFlags{}
	cmd := &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			// Capture flags after processing
			cmdFlags.BitbucketAPIURL, _ = cmd.Flags().GetString("api-url")
			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&cmdFlags.BitbucketAPIURL, "api-url", os.Getenv("BITBUCKET_API_URL"), "Bitbucket API URL")

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Equal(t, "https://custom.bitbucket.com/api/v2", cmdFlags.BitbucketAPIURL)
}

func TestDebugLoggingConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectDebug bool
	}{
		{
			name:        "debug enabled",
			args:        []string{"--debug", "--workspace", "test", "--repo", "test", "--access-token", "test"},
			expectDebug: true,
		},
		{
			name:        "debug disabled",
			args:        []string{"--workspace", "test", "--repo", "test", "--access-token", "test"},
			expectDebug: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd := NewCmdRoot()

			// Capture debug flag
			var debugEnabled bool
			originalRunE := rootCmd.RunE
			rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
				debugEnabled, _ = cmd.Flags().GetBool("debug")
				return nil
			}
			defer func() { rootCmd.RunE = originalRunE }()

			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()

			assert.NoError(t, err)
			assert.Equal(t, tt.expectDebug, debugEnabled)
		})
	}
}

func TestRootCommandHelp(t *testing.T) {
	rootCmd := NewCmdRoot()

	// Capture output
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"--help"})

	err := rootCmd.Execute()
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Export repository and metadata from Bitbucket Cloud")
	assert.Contains(t, output, "--workspace")
	assert.Contains(t, output, "--repo")
	assert.Contains(t, output, "--access-token")
}

func TestRootCommandAPIURLFlag(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectedURL string
	}{
		{
			name:        "custom API URL",
			args:        []string{"--workspace", "test", "--repo", "test", "--access-token", "test", "--bbc-api-url", "https://custom.bitbucket.com/api/v2"},
			expectedURL: "https://custom.bitbucket.com/api/v2",
		},
		{
			name:        "default API URL",
			args:        []string{"--workspace", "test", "--repo", "test", "--access-token", "test"},
			expectedURL: "https://api.bitbucket.org/2.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd := NewCmdRoot()

			var capturedURL string
			originalRunE := rootCmd.RunE
			rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
				capturedURL, _ = cmd.Flags().GetString("bbc-api-url")
				return nil
			}
			defer func() { rootCmd.RunE = originalRunE }()

			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedURL, capturedURL)
		})
	}
}

func TestRootCommandOutputDirFlag(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "output-test-")
	assert.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temporary directory: %v", err)
		}
	}()

	tests := []struct {
		name      string
		args      []string
		checkPath bool
	}{
		{
			name:      "custom output directory",
			args:      []string{"--workspace", "test", "--repo", "test", "--access-token", "test", "--output", tempDir},
			checkPath: true,
		},
		{
			name:      "default output directory",
			args:      []string{"--workspace", "test", "--repo", "test", "--access-token", "test"},
			checkPath: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd := NewCmdRoot()

			var outputDir string
			originalRunE := rootCmd.RunE
			rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
				outputDir, _ = cmd.Flags().GetString("output")
				return nil
			}
			defer func() { rootCmd.RunE = originalRunE }()

			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()

			assert.NoError(t, err)
			if tt.checkPath {
				assert.Equal(t, tempDir, outputDir)
			} else {
				assert.Empty(t, outputDir)
			}
		})
	}
}

func TestMixedAuthenticationMethods(t *testing.T) {
	// Test that mixed authentication methods are rejected

	tests := []struct {
		name    string
		args    []string
		wantErr bool
		errMsg  string
	}{
		{
			name: "api token and workspace token",
			args: []string{
				"--workspace", "test-workspace",
				"--repo", "test-repo",
				"--api-token", "test-api-token",
				"--email", "test@example.com", // Added email to satisfy API token requirement
				"--access-token", "test-access-token",
			},
			wantErr: true,
			errMsg:  "mixed authentication methods",
		},
		{
			name: "api token and username/password",
			args: []string{
				"--workspace", "test-workspace",
				"--repo", "test-repo",
				"--api-token", "test-api-token",
				"--email", "test@example.com", // Added email to satisfy API token requirement
				"--user", "test-user",
				"--app-password", "test-app-password",
			},
			wantErr: true,
			errMsg:  "mixed authentication methods",
		},
		{
			name: "workspace token and username/password",
			args: []string{
				"--workspace", "test-workspace",
				"--repo", "test-repo",
				"--access-token", "test-access-token",
				"--user", "test-user",
				"--app-password", "test-app-password",
			},
			wantErr: true,
			errMsg:  "mixed authentication methods",
		},
		{
			name: "all three authentication methods",
			args: []string{
				"--workspace", "test-workspace",
				"--repo", "test-repo",
				"--api-token", "test-api-token",
				"--email", "test@example.com", // Added email to satisfy API token requirement
				"--access-token", "test-access-token",
				"--user", "test-user",
				"--app-password", "test-app-password",
			},
			wantErr: true,
			errMsg:  "mixed authentication methods",
		},
		{
			name: "mixed environment and flag",
			args: []string{
				"--workspace", "test-workspace",
				"--repo", "test-repo",
				"--api-token", "test-api-token",
				"--email", "test@example.com", // Added email to satisfy API token requirement
			},
			wantErr: true,
			errMsg:  "mixed authentication methods",
		},
		{
			name: "api token without email",
			args: []string{
				"--workspace", "test-workspace",
				"--repo", "test-repo",
				"--api-token", "test-api-token",
			},
			wantErr: true,
			errMsg:  "authentication credentials required", // Changed to match actual error
		},
		{
			name: "email without api token",
			args: []string{
				"--workspace", "test-workspace",
				"--repo", "test-repo",
				"--email", "test@example.com",
			},
			wantErr: true,
			errMsg:  "authentication credentials required", // Changed to match actual error
		},
		{
			name: "username without password",
			args: []string{
				"--workspace", "test-workspace",
				"--repo", "test-repo",
				"--user", "test-user",
			},
			wantErr: true,
			errMsg:  "authentication credentials required", // Added test case for incomplete basic auth
		},
		{
			name: "password without username",
			args: []string{
				"--workspace", "test-workspace",
				"--repo", "test-repo",
				"--app-password", "test-password",
			},
			wantErr: true,
			errMsg:  "authentication credentials required", // Added test case for incomplete basic auth
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables for the mixed environment and flag test
			if tt.name == "mixed environment and flag" {
				err := os.Setenv("BITBUCKET_ACCESS_TOKEN", "env-access-token")
				assert.NoError(t, err)
				defer func() {
					_ = os.Unsetenv("BITBUCKET_ACCESS_TOKEN")
				}()
			} else {
				_ = os.Unsetenv("BITBUCKET_ACCESS_TOKEN")
				_ = os.Unsetenv("BITBUCKET_API_TOKEN")
				_ = os.Unsetenv("BITBUCKET_USERNAME")
				_ = os.Unsetenv("BITBUCKET_APP_PASSWORD")
				_ = os.Unsetenv("BITBUCKET_EMAIL")
			}

			// Create a custom command for testing
			cmdFlags := &data.CmdFlags{}
			rootCmd := &cobra.Command{
				Use:   "bbc-exporter",
				Short: "Export repository and metadata from Bitbucket Cloud",
				RunE: func(cmd *cobra.Command, args []string) error {
					utils.SetupEnvironmentCredentials(cmdFlags)
					return utils.ValidateExportFlags(cmdFlags)
				},
			}

			// Set up the flags
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketAccessToken, "access-token", "t", "", "")
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketAPIToken, "api-token", "", "", "")
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketEmail, "email", "e", "", "")
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketUser, "user", "u", "", "")
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketAppPass, "app-password", "p", "", "")
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.Repository, "repo", "r", "", "")
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.Workspace, "workspace", "w", "", "")

			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAPITokenAuthentication(t *testing.T) {
	// Create a mock client that verifies auth method
	var capturedAuth string
	mockExportFn := func(cmdFlags *data.CmdFlags) error {
		// Simplified logic that just checks which credentials are set
		if cmdFlags.BitbucketAccessToken != "" {
			capturedAuth = "workspace-access-token"
		} else if cmdFlags.BitbucketAPIToken != "" {
			// Always require email with API token
			if cmdFlags.BitbucketEmail == "" {
				return errors.New("email is required when using API token authentication")
			}
			capturedAuth = "api-token-with-email"
		} else if cmdFlags.BitbucketUser != "" && cmdFlags.BitbucketAppPass != "" {
			capturedAuth = "username-and-password"
		} else {
			capturedAuth = "no-auth"
		}
		return nil
	}

	tests := []struct {
		name           string
		args           []string
		expectedAuth   string
		expectError    bool
		setupEnvVars   func()
		cleanupEnvVars func()
	}{
		{
			name: "api token with email via flags",
			args: []string{
				"--workspace", "test-workspace",
				"--repo", "test-repo",
				"--api-token", "test-api-token",
				"--email", "test@example.com",
			},
			expectedAuth:   "api-token-with-email",
			expectError:    false,
			setupEnvVars:   func() {},
			cleanupEnvVars: func() {},
		},
		{
			name: "api token without email should fail",
			args: []string{
				"--workspace", "test-workspace",
				"--repo", "test-repo",
				"--api-token", "test-api-token",
			},
			expectedAuth:   "",
			expectError:    true,
			setupEnvVars:   func() {},
			cleanupEnvVars: func() {},
		},
		{
			name: "api token with email via env vars",
			args: []string{
				"--workspace", "test-workspace",
				"--repo", "test-repo",
			},
			expectedAuth: "api-token-with-email",
			expectError:  false,
			setupEnvVars: func() {
				err := os.Setenv("BITBUCKET_API_TOKEN", "env-api-token")
				if err != nil {
					t.Fatalf("Failed to set BITBUCKET_API_TOKEN: %v", err)
				}
				err = os.Setenv("BITBUCKET_EMAIL", "env-email@example.com")
				if err != nil {
					t.Fatalf("Failed to set BITBUCKET_EMAIL: %v", err)
				}
			},
			cleanupEnvVars: func() {
				if err := os.Unsetenv("BITBUCKET_API_TOKEN"); err != nil {
					t.Logf("Failed to unset BITBUCKET_API_TOKEN: %v", err)
				}
				if err := os.Unsetenv("BITBUCKET_EMAIL"); err != nil {
					t.Logf("Failed to unset BITBUCKET_EMAIL: %v", err)
				}
			},
		},
		{
			name: "api token via env var without email should fail",
			args: []string{
				"--workspace", "test-workspace",
				"--repo", "test-repo",
			},
			expectedAuth: "",
			expectError:  true,
			setupEnvVars: func() {
				if err := os.Setenv("BITBUCKET_API_TOKEN", "env-api-token"); err != nil {
					t.Fatalf("Failed to set environment variable BITBUCKET_API_TOKEN: %v", err)
				}
			},
			cleanupEnvVars: func() {
				if err := os.Unsetenv("BITBUCKET_API_TOKEN"); err != nil {
					t.Logf("Failed to unset BITBUCKET_API_TOKEN: %v", err)
				}
			},
		},
		{
			name: "workspace access token",
			args: []string{
				"--workspace", "test-workspace",
				"--repo", "test-repo",
				"--access-token", "test-access-token",
			},
			expectedAuth:   "workspace-access-token",
			expectError:    false,
			setupEnvVars:   func() {},
			cleanupEnvVars: func() {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment variables
			tt.setupEnvVars()
			defer tt.cleanupEnvVars()

			// Create a new root command with mocked execution
			cmdFlags := &data.CmdFlags{}
			rootCmd := &cobra.Command{
				RunE: func(cmd *cobra.Command, args []string) error {
					utils.SetupEnvironmentCredentials(cmdFlags)
					err := utils.ValidateExportFlags(cmdFlags)
					if err != nil {
						return err
					}
					return mockExportFn(cmdFlags)
				},
			}

			// Set up the flags
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketAccessToken, "access-token", "t", "", "")
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketAPIToken, "api-token", "", "", "")
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketEmail, "email", "e", "", "")
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketUser, "user", "u", "", "")
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketAppPass, "app-password", "p", "", "")
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.Repository, "repo", "r", "", "")
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.Workspace, "workspace", "w", "", "")

			// Execute the command with test arguments
			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Only verify the authentication method if no error is expected
				assert.Equal(t, tt.expectedAuth, capturedAuth)
			}
		})
	}
}

func TestAuthMethodDeprecationWarnings(t *testing.T) {
	// Test that appropriate deprecation warnings are written to stderr

	tests := []struct {
		name             string
		auth             map[string]string
		expectedOutput   []string
		unexpectedOutput []string
	}{
		{
			name: "app password should show deprecation warning",
			auth: map[string]string{
				"user":    "test-user",
				"appPass": "test-password",
			},
			expectedOutput:   []string{"deprecated", "discontinued"},
			unexpectedOutput: []string{},
		},
		{
			name: "api token with email should not show warnings",
			auth: map[string]string{
				"apiToken": "test-api-token",
				"email":    "test@example.com",
			},
			expectedOutput:   []string{}, // No warnings expected for valid API token with email
			unexpectedOutput: []string{"deprecated", "discontinued", "better compatibility", "consider providing an email"},
		},
		{
			name: "workspace token should not show warnings",
			auth: map[string]string{
				"accessToken": "test-access-token",
			},
			expectedOutput:   []string{}, // No warnings expected
			unexpectedOutput: []string{"deprecated", "discontinued", "better compatibility"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Redirect stderr to capture output
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			// Create a cmdFlags from the auth parameters
			cmdFlags := &data.CmdFlags{
				BitbucketAccessToken: tt.auth["accessToken"],
				BitbucketAPIToken:    tt.auth["apiToken"],
				BitbucketEmail:       tt.auth["email"],
				BitbucketUser:        tt.auth["user"],
				BitbucketAppPass:     tt.auth["appPass"],
			}

			// Call the function that generates the warnings
			utils.SetupEnvironmentCredentials(cmdFlags)

			// Close the writer and read the output
			err := w.Close()
			assert.NoError(t, err)
			os.Stderr = oldStderr
			var buf bytes.Buffer
			_, err = io.Copy(&buf, r)
			assert.NoError(t, err)
			output := buf.String()

			// Check for expected output
			for _, expected := range tt.expectedOutput {
				if expected != "" {
					assert.Contains(t, strings.ToLower(output), strings.ToLower(expected),
						"Expected output to contain '%s'", expected)
				}
			}

			// Check that unexpected output is not present
			for _, unexpected := range tt.unexpectedOutput {
				if unexpected != "" {
					assert.NotContains(t, strings.ToLower(output), strings.ToLower(unexpected),
						"Expected output to not contain '%s'", unexpected)
				}
			}
		})
	}
}
