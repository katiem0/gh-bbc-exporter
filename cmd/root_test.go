package cmd

import (
	"bytes"
	"testing"

	"github.com/katiem0/gh-bbc-exporter/internal/data"
	"github.com/katiem0/gh-bbc-exporter/internal/utils"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
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
