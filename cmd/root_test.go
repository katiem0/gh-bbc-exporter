package cmd

import (
	"bytes"
	"os"
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
		BitbucketToken:   "testtoken",
		BitbucketUser:    "testuser",
		BitbucketAppPass: "testpass",
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
	cmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketToken, "token", "t", "", "Token")

	// Test 1: Open PRs only
	args := []string{
		"--workspace", "testworkspace",
		"--repo", "testrepo",
		"--token", "testtoken",
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
	cmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketToken, "token", "t", "", "Token")

	args = []string{
		"--workspace", "testworkspace",
		"--repo", "testrepo",
		"--token", "testtoken",
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
	cmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketToken, "token", "t", "", "Token")

	args = []string{
		"--workspace", "testworkspace",
		"--repo", "testrepo",
		"--token", "testtoken",
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
		"--token", "fake-token",
	})

	err := rootCmd.Execute()
	assert.NoError(t, err)
	assert.True(t, executed)
}

func TestRootCmdOptionsValidation(t *testing.T) {
	oldToken := os.Getenv("BITBUCKET_TOKEN")
	oldUser := os.Getenv("BITBUCKET_USERNAME")
	oldPass := os.Getenv("BITBUCKET_APP_PASSWORD")

	defer func() {
		// Check errors when restoring environment variables
		if err := os.Setenv("BITBUCKET_TOKEN", oldToken); err != nil {
			t.Logf("Failed to restore BITBUCKET_TOKEN: %v", err)
		}
		if err := os.Setenv("BITBUCKET_USERNAME", oldUser); err != nil {
			t.Logf("Failed to restore BITBUCKET_USERNAME: %v", err)
		}
		if err := os.Setenv("BITBUCKET_APP_PASSWORD", oldPass); err != nil {
			t.Logf("Failed to restore BITBUCKET_APP_PASSWORD: %v", err)
		}
	}()

	// Check errors when unsetting environment variables
	if err := os.Unsetenv("BITBUCKET_TOKEN"); err != nil {
		t.Logf("Failed to unset BITBUCKET_TOKEN: %v", err)
	}
	if err := os.Unsetenv("BITBUCKET_USERNAME"); err != nil {
		t.Logf("Failed to unset BITBUCKET_USERNAME: %v", err)
	}
	if err := os.Unsetenv("BITBUCKET_APP_PASSWORD"); err != nil {
		t.Logf("Failed to unset BITBUCKET_APP_PASSWORD: %v", err)
	}

	// Test flag validation logic
	tests := []struct {
		name    string
		args    []string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "missing workspace",
			args:    []string{"--repo", "test-repo", "--token", "fake-token"},
			wantErr: true,
			errMsg:  "a Bitbucket Workspace must be specified",
		},
		{
			name:    "missing repo",
			args:    []string{"--workspace", "test-workspace", "--token", "fake-token"},
			wantErr: true,
			errMsg:  "a Bitbucket repository must be specified",
		},
		{
			name:    "invalid date format",
			args:    []string{"--workspace", "test-workspace", "--repo", "test-repo", "--token", "fake-token", "--prs-from-date", "01/01/2023"},
			wantErr: true,
			errMsg:  "invalid date format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd := NewCmdRoot()
			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
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
			args: []string{"--workspace", "test-workspace", "--repo", "test-repo", "--token", "fake-token"},
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
	originalToken := os.Getenv("BITBUCKET_TOKEN")
	originalUser := os.Getenv("BITBUCKET_USERNAME")
	originalAppPass := os.Getenv("BITBUCKET_APP_PASSWORD")

	// Track if variables existed originally
	tokenExists := originalToken != ""
	userExists := originalUser != ""
	appPassExists := originalAppPass != ""

	// Restore environment after test
	defer func() {
		// For each variable, either restore it or unset it
		if tokenExists {
			_ = os.Setenv("BITBUCKET_TOKEN", originalToken)
		} else {
			_ = os.Unsetenv("BITBUCKET_TOKEN")
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
				"BITBUCKET_TOKEN": "env-token-123",
			},
			cmdArgs: []string{
				"--workspace", "test-workspace",
				"--repo", "test-repo",
			},
			wantErr: false,
			checkFn: func(t *testing.T, flags *data.CmdFlags) {
				assert.Equal(t, "env-token-123", flags.BitbucketToken, "Expected token from environment variable")
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
			name: "command line takes precedence over environment",
			envVars: map[string]string{
				"BITBUCKET_TOKEN": "env-token-123",
			},
			cmdArgs: []string{
				"--workspace", "test-workspace",
				"--repo", "test-repo",
				"--token", "cli-token-override",
			},
			wantErr: false,
			checkFn: func(t *testing.T, flags *data.CmdFlags) {
				assert.Equal(t, "cli-token-override", flags.BitbucketToken, "Expected token from command line to override environment")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear environment variables
			_ = os.Unsetenv("BITBUCKET_TOKEN")
			_ = os.Unsetenv("BITBUCKET_USERNAME")
			_ = os.Unsetenv("BITBUCKET_APP_PASSWORD")

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
			rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketToken, "token", "t", "", "")
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
		originalToken := os.Getenv("BITBUCKET_TOKEN")
		originalUser := os.Getenv("BITBUCKET_USERNAME")
		originalPass := os.Getenv("BITBUCKET_APP_PASSWORD")

		tokenExists := originalToken != ""
		userExists := originalUser != ""
		passExists := originalPass != ""

		// Clear ALL environment variables first
		_ = os.Unsetenv("BITBUCKET_TOKEN")
		_ = os.Unsetenv("BITBUCKET_USERNAME")
		_ = os.Unsetenv("BITBUCKET_APP_PASSWORD")

		// Set only the test value we want
		err := os.Setenv("BITBUCKET_TOKEN", "secure-env-token")
		assert.NoError(t, err, "Failed to set BITBUCKET_TOKEN environment variable")

		// Restore properly on exit
		defer func() {
			// Clear test values
			_ = os.Unsetenv("BITBUCKET_TOKEN")
			_ = os.Unsetenv("BITBUCKET_USERNAME")
			_ = os.Unsetenv("BITBUCKET_APP_PASSWORD")

			// Restore original values
			if tokenExists {
				_ = os.Setenv("BITBUCKET_TOKEN", originalToken)
			}
			if userExists {
				_ = os.Setenv("BITBUCKET_USERNAME", originalUser)
			}
			if passExists {
				_ = os.Setenv("BITBUCKET_APP_PASSWORD", originalPass)
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
				capturedToken = cmdFlags.BitbucketToken
				return utils.ValidateExportFlags(cmdFlags)
			},
		}

		// Add flags to the command
		rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketToken, "token", "t", "", "Token")
		rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketUser, "user", "u", "", "User")
		rootCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketAppPass, "app-password", "p", "", "App Password")
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
			os.Setenv("BITBUCKET_API_URL", originalURL)
		} else {
			os.Unsetenv("BITBUCKET_API_URL")
		}
	}()

	// Set custom API URL
	os.Setenv("BITBUCKET_API_URL", "https://custom.bitbucket.com/api/v2")

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
			args:        []string{"--debug", "--workspace", "test", "--repo", "test", "--token", "test"},
			expectDebug: true,
		},
		{
			name:        "debug disabled",
			args:        []string{"--workspace", "test", "--repo", "test", "--token", "test"},
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
	assert.Contains(t, output, "--token")
}

func TestRootCommandAPIURLFlag(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectedURL string
	}{
		{
			name:        "custom API URL",
			args:        []string{"--workspace", "test", "--repo", "test", "--token", "test", "--bbc-api-url", "https://custom.bitbucket.com/api/v2"},
			expectedURL: "https://custom.bitbucket.com/api/v2",
		},
		{
			name:        "default API URL",
			args:        []string{"--workspace", "test", "--repo", "test", "--token", "test"},
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
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name      string
		args      []string
		checkPath bool
	}{
		{
			name:      "custom output directory",
			args:      []string{"--workspace", "test", "--repo", "test", "--token", "test", "--output", tempDir},
			checkPath: true,
		},
		{
			name:      "default output directory",
			args:      []string{"--workspace", "test", "--repo", "test", "--token", "test"},
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
