package cmd

import (
	"github.com/katiem0/gh-export-bbc/internal/log"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

type cmdFlags struct {
	bitbucketToken   string
	bitbucketUser    string
	bitbucketAppPass string
	bitbucketAPIURL  string
	repository       string
	workspace        string
	outputDir        string
	debug            bool
}

func NewCmdRoot() *cobra.Command {
	cmdFlags := cmdFlags{}

	exportCmd := &cobra.Command{
		Use:   "export-bbc",
		Short: "Export repository and metadata from BitBucket Cloud",
		Long:  "Export repository and metadata from BitBucket Cloud for GitHub Cloud import.",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger, _ := log.NewLogger(cmdFlags.debug)
			defer logger.Sync()
			zap.ReplaceGlobals(logger)

			return runCmdExport(&cmdFlags, logger)
		},
	}

	// Configure flags for command
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.bitbucketToken, "token", "t", "", "BitBucket access token for authentication")
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.bitbucketUser, "user", "u", "", "BitBucket username for basic authentication")
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.bitbucketAppPass, "app-password", "p", "", "BitBucket app password for basic authentication")
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.bitbucketAPIURL, "bbc-api-url", "a", "https://api.bitbucket.org/2.0", "BitBucket API to use")
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.repository, "repo", "r", "", "Name of the repository to export from BitBucket Cloud")
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.workspace, "workspace", "w", "", "BitBucket workspace (or username for personal accounts)")
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.outputDir, "output", "o", "", "Output directory for exported data (default: ./bitbucket-export-TIMESTAMP)")
	exportCmd.PersistentFlags().BoolVarP(&cmdFlags.debug, "debug", "d", false, "Enable debug logging")

	// Mark required flags
	exportCmd.MarkPersistentFlagRequired("workspace")
	exportCmd.MarkPersistentFlagRequired("repository")

	return exportCmd
}

func runCmdExport(cmdFlags *cmdFlags, logger *zap.Logger) error {
	logger.Info("Starting BitBucket Cloud export")

	return nil
}
