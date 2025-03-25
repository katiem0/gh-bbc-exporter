package cmd

import (
	"github.com/katiem0/gh-bbc-exporter/internal/data"
	"github.com/katiem0/gh-bbc-exporter/internal/log"
	"github.com/katiem0/gh-bbc-exporter/internal/utils"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func NewCmdRoot() *cobra.Command {
	cmdFlags := data.CmdFlags{}

	exportCmd := &cobra.Command{
		Use:   "bbc-exporter [flags]",
		Short: "Export repository and metadata from BitBucket Cloud",
		Long:  "Export repository and metadata from BitBucket Cloud for GitHub Cloud import.",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger, _ := log.NewLogger(cmdFlags.Debug)
			defer logger.Sync()
			zap.ReplaceGlobals(logger)

			return runCmdExport(&cmdFlags, logger)
		},
	}

	// Configure flags for command
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketToken, "token", "t", "", "BitBucket access token for authentication")
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketUser, "user", "u", "", "BitBucket username for basic authentication")
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketAppPass, "app-password", "p", "", "BitBucket app password for basic authentication")
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketAPIURL, "bbc-api-url", "a", "https://api.bitbucket.org/2.0", "BitBucket API to use")
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.Repository, "repo", "r", "", "Name of the repository to export from BitBucket Cloud")
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.Workspace, "workspace", "w", "", "BitBucket workspace (or username for personal accounts)")
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.OutputDir, "output", "o", "", "Output directory for exported data (default: ./bitbucket-export-TIMESTAMP)")
	exportCmd.PersistentFlags().BoolVarP(&cmdFlags.Debug, "debug", "d", false, "Enable debug logging")
	// Mark required flags
	exportCmd.MarkPersistentFlagRequired("workspace")
	exportCmd.MarkPersistentFlagRequired("repository")

	return exportCmd
}

func runCmdExport(cmdFlags *data.CmdFlags, logger *zap.Logger) error {
	logger.Info("Starting BitBucket Cloud export",
		zap.String("workspace", cmdFlags.Workspace),
		zap.String("repository", cmdFlags.Repository))

	// Validate inputs
	if err := utils.ValidateExportFlags(cmdFlags); err != nil {
		return err
	}

	utils.SetupEnvironmentCredentials(cmdFlags)

	// Create BitBucket client and exporter
	client := utils.NewClient(
		cmdFlags.BitbucketAPIURL,
		cmdFlags.BitbucketToken,
		cmdFlags.BitbucketUser,
		cmdFlags.BitbucketAppPass,
		logger,
	)
	exporter := utils.NewExporter(client, cmdFlags.OutputDir, logger)

	// Run export
	if err := exporter.Export(cmdFlags.Workspace, cmdFlags.Repository); err != nil {
		logger.Error("Export failed", zap.Error(err))
		return err
	}

	// Print success message
	outputPath := exporter.GetOutputPath()
	utils.PrintSuccessMessage(outputPath)

	logger.Info("Export completed successfully")
	return nil
}
