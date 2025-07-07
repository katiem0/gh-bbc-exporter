package cmd

import (
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
