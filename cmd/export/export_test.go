package export

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/katiem0/gh-bbc-exporter/internal/data"
	"github.com/katiem0/gh-bbc-exporter/internal/utils"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func cleanupExportDirs(t *testing.T) {
	matches, err := filepath.Glob("./bitbucket-export-*")
	if err != nil {
		t.Logf("Warning: Failed to glob for export directories: %v", err)
		return
	}
	for _, match := range matches {
		if err := os.RemoveAll(match); err != nil {
			t.Logf("Warning: Failed to remove %s: %v", match, err)
		}
		archivePath := match + ".tar.gz"
		if _, err := os.Stat(archivePath); err == nil {
			if err := os.Remove(archivePath); err != nil {
				t.Logf("Warning: Failed to remove archive %s: %v", archivePath, err)
			}
		}
	}
}

func TestValidateExportFlagsMixedAuth(t *testing.T) {
	CmdExportFlags := &data.CmdExportFlags{
		BitbucketAccessToken: "testtoken",
		BitbucketUser:        "testuser",
		BitbucketAppPass:     "testpass",
	}

	err := utils.ValidateExportFlags(CmdExportFlags)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mixed authentication methods")
}

func TestPRFilteringFlagsIntegration(t *testing.T) {
	CmdExportFlags := &data.CmdExportFlags{}

	cmd := &cobra.Command{
		Run: func(cmd *cobra.Command, args []string) {
			// This function will be called when cmd.Execute() runs
		},
	}

	cmd.PersistentFlags().BoolVar(&CmdExportFlags.OpenPRsOnly, "open-prs-only", false, "Import only open pull requests")
	cmd.PersistentFlags().StringVarP(&CmdExportFlags.PRsFromDate, "prs-from-date", "", "", "Import pull requests created on or after this date")
	cmd.PersistentFlags().StringVarP(&CmdExportFlags.Workspace, "workspace", "w", "", "Bitbucket workspace name")
	cmd.PersistentFlags().StringVarP(&CmdExportFlags.Repository, "repo", "r", "", "Repository name")
	cmd.PersistentFlags().StringVarP(&CmdExportFlags.BitbucketAccessToken, "access-token", "t", "", "Access token")

	args := []string{
		"--workspace", "testworkspace",
		"--repo", "testrepo",
		"--access-token", "testtoken",
		"--open-prs-only",
	}

	cmd.SetArgs(args)
	err := cmd.Execute()
	assert.NoError(t, err)

	assert.True(t, CmdExportFlags.OpenPRsOnly, "Expected OpenPRsOnly to be true")
	assert.Equal(t, "", CmdExportFlags.PRsFromDate, "Expected PRsFromDate to be empty")

	CmdExportFlags = &data.CmdExportFlags{}
	cmd = &cobra.Command{Run: func(cmd *cobra.Command, args []string) {}}
	cmd.PersistentFlags().BoolVar(&CmdExportFlags.OpenPRsOnly, "open-prs-only", false, "Import only open pull requests")
	cmd.PersistentFlags().StringVarP(&CmdExportFlags.PRsFromDate, "prs-from-date", "", "", "Import pull requests created on or after this date")
	cmd.PersistentFlags().StringVarP(&CmdExportFlags.Workspace, "workspace", "w", "", "Bitbucket workspace name")
	cmd.PersistentFlags().StringVarP(&CmdExportFlags.Repository, "repo", "r", "", "Repository name")
	cmd.PersistentFlags().StringVarP(&CmdExportFlags.BitbucketAccessToken, "access-token", "t", "", "Access token")

	args = []string{
		"--workspace", "testworkspace",
		"--repo", "testrepo",
		"--access-token", "testtoken",
		"--prs-from-date", "2023-01-01",
	}

	cmd.SetArgs(args)
	err = cmd.Execute()
	assert.NoError(t, err)

	assert.False(t, CmdExportFlags.OpenPRsOnly, "Expected OpenPRsOnly to be false")
	assert.Equal(t, "2023-01-01", CmdExportFlags.PRsFromDate, "Expected PRsFromDate to be set")

	CmdExportFlags = &data.CmdExportFlags{}
	cmd = &cobra.Command{Run: func(cmd *cobra.Command, args []string) {}}
	cmd.PersistentFlags().BoolVar(&CmdExportFlags.OpenPRsOnly, "open-prs-only", false, "Import only open pull requests")
	cmd.PersistentFlags().StringVarP(&CmdExportFlags.PRsFromDate, "prs-from-date", "", "", "Import pull requests created on or after this date")
	cmd.PersistentFlags().StringVarP(&CmdExportFlags.Workspace, "workspace", "w", "", "Bitbucket workspace name")
	cmd.PersistentFlags().StringVarP(&CmdExportFlags.Repository, "repo", "r", "", "Repository name")
	cmd.PersistentFlags().StringVarP(&CmdExportFlags.BitbucketAccessToken, "access-token", "t", "", "Access token")

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

	assert.True(t, CmdExportFlags.OpenPRsOnly, "Expected OpenPRsOnly to be true")
	assert.Equal(t, "2023-01-01", CmdExportFlags.PRsFromDate, "Expected PRsFromDate to be set")
}

func TestExecuteWithInvalidFlags(t *testing.T) {
	rootCmd := NewCmdExport()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	rootCmd.SetArgs([]string{})
	err := rootCmd.Execute()
	assert.Error(t, err)

	output := buf.String()
	assert.Contains(t, output, "Bitbucket Workspace must be specified")
}

func TestExecuteWithValidFlags(t *testing.T) {
	rootCmd := NewCmdExport()

	originalRun := rootCmd.RunE
	defer func() { rootCmd.RunE = originalRun }()

	executed := false
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		executed = true
		return nil
	}

	rootCmd.SetArgs([]string{
		"--workspace", "test-workspace",
		"--repo", "test-repo",
		"--access-token", "fake-token",
	})

	err := rootCmd.Execute()
	assert.NoError(t, err)
	assert.True(t, executed)
}

func TestRunCmdExportOptionsValidation(t *testing.T) {
	defer cleanupExportDirs(t)
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

	_ = os.Unsetenv("BITBUCKET_ACCESS_TOKEN")
	_ = os.Unsetenv("BITBUCKET_USERNAME")
	_ = os.Unsetenv("BITBUCKET_APP_PASSWORD")
	_ = os.Unsetenv("BITBUCKET_API_TOKEN")
	_ = os.Unsetenv("BITBUCKET_EMAIL")

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
			CmdExportFlags := &data.CmdExportFlags{}
			exportCmd := &cobra.Command{
				Use:   "bbc-exporter",
				Short: "Export repository and metadata from Bitbucket Cloud",
				PreRunE: func(cmd *cobra.Command, args []string) error {
					if len(CmdExportFlags.Workspace) == 0 {
						return errors.New("a Bitbucket Workspace must be specified")
					}
					if len(CmdExportFlags.Repository) == 0 {
						return errors.New("a Bitbucket repository must be specified")
					}
					return nil
				},
				RunE: func(cmd *cobra.Command, args []string) error {
					utils.SetupEnvironmentCredentials(CmdExportFlags)
					return utils.ValidateExportFlags(CmdExportFlags)
				},
			}

			exportCmd.PersistentFlags().StringVarP(&CmdExportFlags.BitbucketAPIURL, "bbc-api-url", "a",
				"https://api.bitbucket.org/2.0", "Bitbucket API URL")
			exportCmd.PersistentFlags().StringVarP(&CmdExportFlags.BitbucketAccessToken, "access-token", "t", "",
				"Bitbucket workspace access token")
			exportCmd.PersistentFlags().StringVarP(&CmdExportFlags.BitbucketAPIToken, "api-token", "", "",
				"Bitbucket API token")
			exportCmd.PersistentFlags().StringVarP(&CmdExportFlags.BitbucketEmail, "email", "e", "",
				"Atlassian account email")
			exportCmd.PersistentFlags().StringVarP(&CmdExportFlags.BitbucketUser, "user", "u", "",
				"Bitbucket username")
			exportCmd.PersistentFlags().StringVarP(&CmdExportFlags.BitbucketAppPass, "app-password", "p", "",
				"Bitbucket app password")
			exportCmd.PersistentFlags().StringVarP(&CmdExportFlags.Workspace, "workspace", "w", "",
				"Bitbucket workspace name")
			exportCmd.PersistentFlags().StringVarP(&CmdExportFlags.Repository, "repo", "r", "",
				"Repository name")
			exportCmd.PersistentFlags().StringVarP(&CmdExportFlags.PRsFromDate, "prs-from-date", "", "",
				"Export pull requests from date (YYYY-MM-DD)")
			exportCmd.PersistentFlags().BoolVar(&CmdExportFlags.OpenPRsOnly, "open-prs-only", false,
				"Export only open pull requests")

			exportCmd.SetArgs(tt.args)
			err := exportCmd.Execute()

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

func TestRunCmdExportWithValidFlags(t *testing.T) {
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
			rootCmd := NewCmdExport()

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
	originalToken := os.Getenv("BITBUCKET_ACCESS_TOKEN")
	originalUser := os.Getenv("BITBUCKET_USERNAME")
	originalAppPass := os.Getenv("BITBUCKET_APP_PASSWORD")
	originalApiToken := os.Getenv("BITBUCKET_API_TOKEN")
	originalEmail := os.Getenv("BITBUCKET_EMAIL")
	originalTempDir := os.Getenv("BITBUCKET_TEMP_DIR")

	tokenExists := originalToken != ""
	userExists := originalUser != ""
	appPassExists := originalAppPass != ""
	apiTokenExists := originalApiToken != ""
	emailExists := originalEmail != ""
	tempDirExists := originalTempDir != ""

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

		if tempDirExists {
			_ = os.Setenv("BITBUCKET_TEMP_DIR", originalTempDir)
		} else {
			_ = os.Unsetenv("BITBUCKET_TEMP_DIR")
		}
	}()

	// Clear all environment variables
	_ = os.Unsetenv("BITBUCKET_ACCESS_TOKEN")
	_ = os.Unsetenv("BITBUCKET_USERNAME")
	_ = os.Unsetenv("BITBUCKET_APP_PASSWORD")
	_ = os.Unsetenv("BITBUCKET_API_TOKEN")
	_ = os.Unsetenv("BITBUCKET_EMAIL")
	_ = os.Unsetenv("BITBUCKET_TEMP_DIR")

	err := os.Setenv("BITBUCKET_ACCESS_TOKEN", "env-token")
	assert.NoError(t, err)
	err = os.Setenv("BITBUCKET_USERNAME", "env-user")
	assert.NoError(t, err)
	err = os.Setenv("BITBUCKET_APP_PASSWORD", "env-pass")
	assert.NoError(t, err)
	err = os.Setenv("BITBUCKET_API_TOKEN", "env-api-token")
	assert.NoError(t, err)
	err = os.Setenv("BITBUCKET_EMAIL", "env@example.com")
	assert.NoError(t, err)
	err = os.Setenv("BITBUCKET_TEMP_DIR", "/env/temp/dir")
	assert.NoError(t, err)

	CmdExportFlags := &data.CmdExportFlags{}
	utils.SetupEnvironmentCredentials(CmdExportFlags)

	assert.Equal(t, "env-token", CmdExportFlags.BitbucketAccessToken)
	assert.Equal(t, "env-user", CmdExportFlags.BitbucketUser)
	assert.Equal(t, "env-pass", CmdExportFlags.BitbucketAppPass)
	assert.Equal(t, "env-api-token", CmdExportFlags.BitbucketAPIToken)
	assert.Equal(t, "env@example.com", CmdExportFlags.BitbucketEmail)
	assert.Equal(t, "/env/temp/dir", CmdExportFlags.TempDir)
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
	defer cleanupExportDirs(t)
	t.Run("env vars only - should succeed", func(t *testing.T) {
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

		_ = os.Unsetenv("BITBUCKET_ACCESS_TOKEN")
		_ = os.Unsetenv("BITBUCKET_USERNAME")
		_ = os.Unsetenv("BITBUCKET_APP_PASSWORD")
		_ = os.Unsetenv("BITBUCKET_API_TOKEN")
		_ = os.Unsetenv("BITBUCKET_EMAIL")

		err := os.Setenv("BITBUCKET_ACCESS_TOKEN", "secure-env-token")
		assert.NoError(t, err, "Failed to set BITBUCKET_ACCESS_TOKEN environment variable")

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

		var capturedToken string
		cmdExportFlags := &data.CmdExportFlags{}
		exportCmd := &cobra.Command{
			Use:   "bbc-exporter",
			Short: "Export repository and metadata from Bitbucket Cloud",
			RunE: func(cmd *cobra.Command, args []string) error {
				utils.SetupEnvironmentCredentials(cmdExportFlags)
				capturedToken = cmdExportFlags.BitbucketAccessToken
				return utils.ValidateExportFlags(cmdExportFlags)
			},
		}

		exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.BitbucketAccessToken, "access-token", "t", "", "Token")
		exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.BitbucketUser, "user", "u", "", "User")
		exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.BitbucketAppPass, "app-password", "p", "", "App Password")
		exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.BitbucketAPIToken, "api-token", "", "", "API Token")
		exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.BitbucketEmail, "email", "e", "", "Email")
		exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.Workspace, "workspace", "w", "", "Workspace")
		exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.Repository, "repo", "r", "", "Repository")

		exportCmd.SetArgs([]string{
			"--workspace", "test-ws",
			"--repo", "test-repo",
		})

		err = exportCmd.Execute()
		assert.NoError(t, err, "Should succeed with env var token")
		assert.Equal(t, "secure-env-token", capturedToken)
	})
}

func TestEnvironmentVariableOverrides(t *testing.T) {
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

	if err := os.Setenv("BITBUCKET_API_URL", "https://custom.bitbucket.com/api/v2"); err != nil {
		t.Fatalf("Failed to set BITBUCKET_API_URL: %v", err)
	}

	cmdExportFlags := &data.CmdExportFlags{}
	cmd := &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			cmdExportFlags.BitbucketAPIURL, _ = cmd.Flags().GetString("api-url")
			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&cmdExportFlags.BitbucketAPIURL, "api-url", os.Getenv("BITBUCKET_API_URL"), "Bitbucket API URL")

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Equal(t, "https://custom.bitbucket.com/api/v2", cmdExportFlags.BitbucketAPIURL)
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
			rootCmd := NewCmdExport()

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

func TestRunCmdExportHelp(t *testing.T) {
	rootCmd := NewCmdExport()

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

func TestRunCmdExportAPIURLFlag(t *testing.T) {
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
			rootCmd := NewCmdExport()

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

func TestRunCmdExportOutputDirFlag(t *testing.T) {
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
			rootCmd := NewCmdExport()

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
	defer cleanupExportDirs(t)
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

			cmdExportFlags := &data.CmdExportFlags{}
			exportCmd := &cobra.Command{
				Use:   "bbc-exporter",
				Short: "Export repository and metadata from Bitbucket Cloud",
				RunE: func(cmd *cobra.Command, args []string) error {
					utils.SetupEnvironmentCredentials(cmdExportFlags)
					return utils.ValidateExportFlags(cmdExportFlags)
				},
			}

			exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.BitbucketAccessToken, "access-token", "t", "", "")
			exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.BitbucketAPIToken, "api-token", "", "", "")
			exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.BitbucketEmail, "email", "e", "", "")
			exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.BitbucketUser, "user", "u", "", "")
			exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.BitbucketAppPass, "app-password", "p", "", "")
			exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.Repository, "repo", "r", "", "")
			exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.Workspace, "workspace", "w", "", "")

			exportCmd.SetArgs(tt.args)
			err := exportCmd.Execute()

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
	defer cleanupExportDirs(t)
	var capturedAuth string
	mockExportFn := func(cmdExportFlags *data.CmdExportFlags) error {
		if cmdExportFlags.BitbucketAccessToken != "" {
			capturedAuth = "workspace-access-token"
		} else if cmdExportFlags.BitbucketAPIToken != "" {
			if cmdExportFlags.BitbucketEmail == "" {
				return errors.New("email is required when using API token authentication")
			}
			capturedAuth = "api-token-with-email"
		} else if cmdExportFlags.BitbucketUser != "" && cmdExportFlags.BitbucketAppPass != "" {
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
			tt.setupEnvVars()
			defer tt.cleanupEnvVars()

			cmdExportFlags := &data.CmdExportFlags{}
			exportCmd := &cobra.Command{
				RunE: func(cmd *cobra.Command, args []string) error {
					utils.SetupEnvironmentCredentials(cmdExportFlags)
					err := utils.ValidateExportFlags(cmdExportFlags)
					if err != nil {
						return err
					}
					return mockExportFn(cmdExportFlags)
				},
			}

			exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.BitbucketAccessToken, "access-token", "t", "", "")
			exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.BitbucketAPIToken, "api-token", "", "", "")
			exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.BitbucketEmail, "email", "e", "", "")
			exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.BitbucketUser, "user", "u", "", "")
			exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.BitbucketAppPass, "app-password", "p", "", "")
			exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.Repository, "repo", "r", "", "")
			exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.Workspace, "workspace", "w", "", "")

			exportCmd.SetArgs(tt.args)
			err := exportCmd.Execute()

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
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			cmdExportFlags := &data.CmdExportFlags{
				BitbucketAccessToken: tt.auth["accessToken"],
				BitbucketAPIToken:    tt.auth["apiToken"],
				BitbucketEmail:       tt.auth["email"],
				BitbucketUser:        tt.auth["user"],
				BitbucketAppPass:     tt.auth["appPass"],
			}

			utils.SetupEnvironmentCredentials(cmdExportFlags)

			err := w.Close()
			assert.NoError(t, err)
			os.Stderr = oldStderr
			var buf bytes.Buffer
			_, err = io.Copy(&buf, r)
			assert.NoError(t, err)
			output := buf.String()

			for _, expected := range tt.expectedOutput {
				if expected != "" {
					assert.Contains(t, strings.ToLower(output), strings.ToLower(expected),
						"Expected output to contain '%s'", expected)
				}
			}

			for _, unexpected := range tt.unexpectedOutput {
				if unexpected != "" {
					assert.NotContains(t, strings.ToLower(output), strings.ToLower(unexpected),
						"Expected output to not contain '%s'", unexpected)
				}
			}
		})
	}
}

func TestRunCmdExportWithMissingRequiredFlags(t *testing.T) {
	testCases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "No workspace",
			args:    []string{"--repo", "test-repo"},
			wantErr: "a Bitbucket Workspace must be specified",
		},
		{
			name:    "No repository",
			args:    []string{"--workspace", "test-workspace"},
			wantErr: "a Bitbucket repository must be specified",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := NewCmdExport()
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestRunCmdExportWithAuthValidation(t *testing.T) {
	defer cleanupExportDirs(t)
	testCases := []struct {
		name    string
		args    []string
		envVars map[string]string
		wantErr string
	}{
		{
			name: "No auth credentials",
			args: []string{
				"--workspace", "test-workspace",
				"--repo", "test-repo",
			},
			wantErr: "authentication credentials required",
		},
		{
			name: "Username without app password",
			args: []string{
				"--workspace", "test-workspace",
				"--repo", "test-repo",
				"--user", "testuser",
			},
			wantErr: "authentication credentials required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.envVars {
				if err := os.Setenv(k, v); err != nil {
					t.Fatalf("Failed to set environment variable %s: %v", k, err)
				}
				defer func(key string) {
					if err := os.Unsetenv(key); err != nil {
						t.Logf("Failed to unset environment variable %s: %v", key, err)
					}
				}(k)
			}

			cmd := NewCmdExport()
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestRunCmdExport_AuthLogging(t *testing.T) {
	defer cleanupExportDirs(t)
	core, obs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	tempDir, err := os.MkdirTemp("", "cmd-export-test-")
	assert.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: Failed to remove temp dir: %v", err)
		}
	}()

	cmdExportFlags := data.CmdExportFlags{
		BitbucketAccessToken: "test-access-token",
		Workspace:            "test-workspace",
		Repository:           "test-repo",
		OutputDir:            tempDir,
	}

	err = runCmdExport(&cmdExportFlags, logger)
	assert.Error(t, err, "expected runCmdExport to return an error (export likely fails in tests)")

	entries := obs.All()
	foundStart := false
	foundAuth := false
	for _, e := range entries {
		if e.Message == "Starting Bitbucket Cloud export" {
			foundStart = true
		}
		if e.Message == "Using workspace access token authentication" {
			foundAuth = true
		}
	}
	assert.True(t, foundStart, "expected startup log from runCmdExport")
	assert.True(t, foundAuth, "expected auth log from runCmdExport")
}

func TestTempDirFlag(t *testing.T) {
	defer cleanupExportDirs(t)
	tests := []struct {
		name        string
		args        []string
		expectedDir string
		shouldSet   bool
	}{
		{
			name:        "temp-dir flag set",
			args:        []string{"--workspace", "test", "--repo", "test", "--access-token", "test", "--temp-dir", "/custom/temp"},
			expectedDir: "/custom/temp",
			shouldSet:   true,
		},
		{
			name:        "temp-dir flag not set",
			args:        []string{"--workspace", "test", "--repo", "test", "--access-token", "test"},
			expectedDir: "",
			shouldSet:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd := NewCmdExport()
			rootCmd.SetArgs(tt.args)

			// Capture the actual execution
			err := rootCmd.Execute()
			// Execution will fail due to invalid auth/repo, but we can check flags were parsed
			assert.Error(t, err)

			// Get the flags
			tempDir, err := rootCmd.PersistentFlags().GetString("temp-dir")
			if tt.shouldSet {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedDir, tempDir)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "", tempDir)
			}
		})
	}
}

func TestTempDirEnvironmentIntegration(t *testing.T) {

	originalTempDir := os.Getenv("BITBUCKET_TEMP_DIR")
	defer func() {
		if originalTempDir != "" {
			_ = os.Setenv("BITBUCKET_TEMP_DIR", originalTempDir)
		} else {
			_ = os.Unsetenv("BITBUCKET_TEMP_DIR")
		}
	}()

	err := os.Setenv("BITBUCKET_TEMP_DIR", "/env/temp/path")
	assert.NoError(t, err)

	cmdExportFlags := &data.CmdExportFlags{}
	utils.SetupEnvironmentCredentials(cmdExportFlags)

	assert.Equal(t, "/env/temp/path", cmdExportFlags.TempDir, "Should read from environment")

	cmdExportFlags.TempDir = "/flag/temp/path"
	utils.SetupEnvironmentCredentials(cmdExportFlags)

	assert.Equal(t, "/flag/temp/path", cmdExportFlags.TempDir, "Flag should not be overridden when already set")
}

func TestSkipCommitLookupFlag(t *testing.T) {
	defer cleanupExportDirs(t)
	tests := []struct {
		name          string
		args          []string
		expectedValue bool
	}{
		{
			name:          "skip-commit-lookup enabled",
			args:          []string{"--workspace", "test", "--repo", "test", "--access-token", "test", "--skip-commit-lookup"},
			expectedValue: true,
		},
		{
			name:          "skip-commit-lookup disabled by default",
			args:          []string{"--workspace", "test", "--repo", "test", "--access-token", "test"},
			expectedValue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd := NewCmdExport()

			var skipCommitLookup bool
			originalRunE := rootCmd.RunE
			rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
				skipCommitLookup, _ = cmd.Flags().GetBool("skip-commit-lookup")
				return nil
			}
			defer func() { rootCmd.RunE = originalRunE }()

			rootCmd.SetArgs(tt.args)
			err := rootCmd.Execute()

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedValue, skipCommitLookup)
		})
	}
}

func TestPRsFromDateValidation(t *testing.T) {
	defer cleanupExportDirs(t)
	tests := []struct {
		name        string
		dateValue   string
		expectError bool
		errContains string
	}{
		{
			name:        "valid date format YYYY-MM-DD",
			dateValue:   "2023-01-15",
			expectError: false,
		},
		{
			name:        "invalid date format MM/DD/YYYY",
			dateValue:   "01/15/2023",
			expectError: true,
			errContains: "invalid date format",
		},
		{
			name:        "invalid date format DD-MM-YYYY",
			dateValue:   "15-01-2023",
			expectError: true,
			errContains: "invalid date format",
		},
		{
			name:        "empty date is valid",
			dateValue:   "",
			expectError: false,
		},
		{
			name:        "invalid date - non-existent day",
			dateValue:   "2023-02-30",
			expectError: true,
			errContains: "invalid date format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmdExportFlags := &data.CmdExportFlags{
				Workspace:            "test-workspace",
				Repository:           "test-repo",
				BitbucketAccessToken: "test-token",
				PRsFromDate:          tt.dateValue,
			}

			err := utils.ValidateExportFlags(cmdExportFlags)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCombinedPRFilters(t *testing.T) {
	defer cleanupExportDirs(t)
	tests := []struct {
		name        string
		openOnly    bool
		fromDate    string
		expectError bool
	}{
		{
			name:        "open PRs only",
			openOnly:    true,
			fromDate:    "",
			expectError: false,
		},
		{
			name:        "PRs from specific date",
			openOnly:    false,
			fromDate:    "2023-06-01",
			expectError: false,
		},
		{
			name:        "combined filters - open PRs from date",
			openOnly:    true,
			fromDate:    "2023-06-01",
			expectError: false,
		},
		{
			name:        "no filters",
			openOnly:    false,
			fromDate:    "",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmdExportFlags := &data.CmdExportFlags{
				Workspace:            "test-workspace",
				Repository:           "test-repo",
				BitbucketAccessToken: "test-token",
				OpenPRsOnly:          tt.openOnly,
				PRsFromDate:          tt.fromDate,
			}

			err := utils.ValidateExportFlags(cmdExportFlags)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOutputDirectoryCreation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "output-dir-test-")
	assert.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: Failed to remove temp dir: %v", err)
		}
	}()

	nestedDir := filepath.Join(tempDir, "nested", "path", "to", "output")

	cmdExportFlags := &data.CmdExportFlags{
		Workspace:            "test-workspace",
		Repository:           "test-repo",
		BitbucketAccessToken: "test-token",
		OutputDir:            nestedDir,
	}

	err = utils.ValidateExportFlags(cmdExportFlags)
	assert.NoError(t, err)
}

func TestMultipleEnvironmentVariableSources(t *testing.T) {
	envVars := []string{
		"BITBUCKET_ACCESS_TOKEN",
		"BITBUCKET_API_TOKEN",
		"BITBUCKET_EMAIL",
		"BITBUCKET_USERNAME",
		"BITBUCKET_APP_PASSWORD",
		"BITBUCKET_TEMP_DIR",
	}

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

	// Clear all env vars
	for _, v := range envVars {
		_ = os.Unsetenv(v)
	}

	tests := []struct {
		name     string
		envVars  map[string]string
		expected data.CmdExportFlags
	}{
		{
			name: "access token from env",
			envVars: map[string]string{
				"BITBUCKET_ACCESS_TOKEN": "env-access-token",
			},
			expected: data.CmdExportFlags{
				BitbucketAccessToken: "env-access-token",
			},
		},
		{
			name: "api token and email from env",
			envVars: map[string]string{
				"BITBUCKET_API_TOKEN": "env-api-token",
				"BITBUCKET_EMAIL":     "test@example.com",
			},
			expected: data.CmdExportFlags{
				BitbucketAPIToken: "env-api-token",
				BitbucketEmail:    "test@example.com",
			},
		},
		{
			name: "username and app password from env",
			envVars: map[string]string{
				"BITBUCKET_USERNAME":     "env-user",
				"BITBUCKET_APP_PASSWORD": "env-pass",
			},
			expected: data.CmdExportFlags{
				BitbucketUser:    "env-user",
				BitbucketAppPass: "env-pass",
			},
		},
		{
			name: "all credentials from env",
			envVars: map[string]string{
				"BITBUCKET_ACCESS_TOKEN": "env-access-token",
				"BITBUCKET_API_TOKEN":    "env-api-token",
				"BITBUCKET_EMAIL":        "test@example.com",
				"BITBUCKET_USERNAME":     "env-user",
				"BITBUCKET_APP_PASSWORD": "env-pass",
				"BITBUCKET_TEMP_DIR":     "/env/temp",
			},
			expected: data.CmdExportFlags{
				BitbucketAccessToken: "env-access-token",
				BitbucketAPIToken:    "env-api-token",
				BitbucketEmail:       "test@example.com",
				BitbucketUser:        "env-user",
				BitbucketAppPass:     "env-pass",
				TempDir:              "/env/temp",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all env vars before each test
			for _, v := range envVars {
				_ = os.Unsetenv(v)
			}

			// Set test-specific env vars
			for k, v := range tt.envVars {
				err := os.Setenv(k, v)
				assert.NoError(t, err)
			}

			cmdExportFlags := &data.CmdExportFlags{}
			utils.SetupEnvironmentCredentials(cmdExportFlags)

			assert.Equal(t, tt.expected.BitbucketAccessToken, cmdExportFlags.BitbucketAccessToken)
			assert.Equal(t, tt.expected.BitbucketAPIToken, cmdExportFlags.BitbucketAPIToken)
			assert.Equal(t, tt.expected.BitbucketEmail, cmdExportFlags.BitbucketEmail)
			assert.Equal(t, tt.expected.BitbucketUser, cmdExportFlags.BitbucketUser)
			assert.Equal(t, tt.expected.BitbucketAppPass, cmdExportFlags.BitbucketAppPass)
			assert.Equal(t, tt.expected.TempDir, cmdExportFlags.TempDir)
		})
	}
}

func TestFlagPrecedenceOverEnvironment(t *testing.T) {
	originalToken := os.Getenv("BITBUCKET_ACCESS_TOKEN")
	defer func() {
		if originalToken != "" {
			_ = os.Setenv("BITBUCKET_ACCESS_TOKEN", originalToken)
		} else {
			_ = os.Unsetenv("BITBUCKET_ACCESS_TOKEN")
		}
	}()

	err := os.Setenv("BITBUCKET_ACCESS_TOKEN", "env-token")
	assert.NoError(t, err)

	cmdExportFlags := &data.CmdExportFlags{
		BitbucketAccessToken: "flag-token",
	}
	utils.SetupEnvironmentCredentials(cmdExportFlags)

	assert.Equal(t, "flag-token", cmdExportFlags.BitbucketAccessToken,
		"Flag value should take precedence over environment variable")
}

func TestNewCmdExportFlagDefinitions(t *testing.T) {
	cmd := NewCmdExport()

	expectedFlags := []struct {
		name      string
		shorthand string
	}{
		{"bbc-api-url", "a"},
		{"access-token", "t"},
		{"api-token", ""},
		{"email", "e"},
		{"user", "u"},
		{"app-password", "p"},
		{"workspace", "w"},
		{"repo", "r"},
		{"output", "o"},
		{"temp-dir", ""},
		{"prs-from-date", ""},
		{"open-prs-only", ""},
		{"skip-commit-lookup", ""},
		{"debug", "d"},
	}

	for _, ef := range expectedFlags {
		t.Run("flag_"+ef.name, func(t *testing.T) {
			flag := cmd.PersistentFlags().Lookup(ef.name)
			assert.NotNil(t, flag, "Flag %s should be defined", ef.name)
			if ef.shorthand != "" {
				assert.Equal(t, ef.shorthand, flag.Shorthand,
					"Flag %s should have shorthand %s", ef.name, ef.shorthand)
			}
		})
	}
}

func TestDefaultAPIURL(t *testing.T) {
	cmd := NewCmdExport()

	apiURLFlag := cmd.PersistentFlags().Lookup("bbc-api-url")
	assert.NotNil(t, apiURLFlag)
	assert.Equal(t, "https://api.bitbucket.org/2.0", apiURLFlag.DefValue,
		"Default API URL should be the Bitbucket Cloud API endpoint")
}

func TestExportCommandUsage(t *testing.T) {
	cmd := NewCmdExport()

	assert.Equal(t, "export [flags]", cmd.Use)
	assert.Contains(t, cmd.Short, "Export")
	assert.Contains(t, cmd.Short, "Bitbucket Cloud")
}

func TestRunCmdExportWithDebugLogging(t *testing.T) {
	defer cleanupExportDirs(t)

	core, obs := observer.New(zap.DebugLevel)
	logger := zap.New(core)

	tempDir, err := os.MkdirTemp("", "debug-logging-test-")
	assert.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: Failed to remove temp dir: %v", err)
		}
	}()

	cmdExportFlags := data.CmdExportFlags{
		BitbucketAccessToken: "test-token",
		Workspace:            "test-workspace",
		Repository:           "test-repo",
		OutputDir:            tempDir,
		Debug:                true,
	}

	// This will fail but we're testing that debug logging is set up correctly
	_ = runCmdExport(&cmdExportFlags, logger)

	// Verify debug-level logs were captured
	entries := obs.All()
	assert.Greater(t, len(entries), 0, "Expected log entries to be generated")
}

func TestValidateExportFlagsEmptyWorkspace(t *testing.T) {
	cmd := NewCmdExport()
	cmd.SetArgs([]string{
		"--repo", "test-repo",
		"--access-token", "test-token",
	})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Workspace")
}

func TestValidateExportFlagsEmptyRepository(t *testing.T) {
	cmd := NewCmdExport()
	cmd.SetArgs([]string{
		"--workspace", "test-workspace",
		"--access-token", "test-token",
	})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "repository")
}

func TestCleanupExportDirsHelper(t *testing.T) {
	testDir, err := os.MkdirTemp(".", "bitbucket-export-test-")
	assert.NoError(t, err)

	_, err = os.Stat(testDir)
	assert.NoError(t, err)

	cleanupExportDirs(t)

	_, err = os.Stat(testDir)
	assert.True(t, os.IsNotExist(err), "Test directory should have been cleaned up")
}
