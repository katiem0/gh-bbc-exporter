package cmd

import (
	"errors"
	"fmt"

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
		Short: "Export repository and metadata from Bitbucket Cloud",
		Long:  "Export repository and metadata from Bitbucket Cloud for GitHub Cloud import.",
		PreRunE: func(exportCmd *cobra.Command, args []string) error {
			if len(cmdFlags.Workspace) == 0 {
				return errors.New("a Bitbucket Workspace must be specified")
			}
			if len(cmdFlags.Repository) == 0 {
				return errors.New("a Bitbucket repository must be specified")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			logger, err := log.NewLogger(cmdFlags.Debug)
			if err != nil {
				return fmt.Errorf("failed to initialize logger: %w", err)
			}
			defer func() {
				_ = logger.Sync()
			}()
			zap.ReplaceGlobals(logger)
			return runCmdExport(&cmdFlags, logger)
		},
	}

	// Disable alphabetical sorting of flags
	exportCmd.Flags().SortFlags = false
	exportCmd.PersistentFlags().SortFlags = false

	// Configure flags for command
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketAPIURL, "bbc-api-url", "a", "https://api.bitbucket.org/2.0", "Bitbucket API to use")
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketToken, "token", "t", "",
		"Bitbucket access token for authentication (env: BITBUCKET_TOKEN)")
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketUser, "user", "u", "",
		"Bitbucket username for basic authentication (env: BITBUCKET_USERNAME)")
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketAppPass, "app-password", "p", "",
		"Bitbucket app password for basic authentication (env: BITBUCKET_APP_PASSWORD)")
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.Workspace, "workspace", "w", "", "Bitbucket workspace name")
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.Repository, "repo", "r", "", "Name of the repository to export from Bitbucket Cloud")
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.OutputDir, "output", "o", "", "Output directory for exported data (default: ./bitbucket-export-TIMESTAMP)")
	exportCmd.PersistentFlags().BoolVar(&cmdFlags.OpenPRsOnly, "open-prs-only", false, "Export only open pull requests and ignore closed/merged ones")
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.PRsFromDate, "prs-from-date", "", "", "Export pull requests created on or after this date (format: YYYY-MM-DD).")
	exportCmd.PersistentFlags().BoolVarP(&cmdFlags.Debug, "debug", "d", false, "Enable debug logging")
	// Mark required flags
	if err := exportCmd.MarkPersistentFlagRequired("workspace"); err != nil {
		fmt.Printf("Error marking workspace flag as required: %v\n", err)
	}
	if err := exportCmd.MarkPersistentFlagRequired("repo"); err != nil {
		fmt.Printf("Error marking repository flag as required: %v\n", err)
	}
	return exportCmd
}

func runCmdExport(cmdFlags *data.CmdFlags, logger *zap.Logger) error {
	logger.Info("Starting Bitbucket Cloud export",
		zap.String("workspace", cmdFlags.Workspace),
		zap.String("repository", cmdFlags.Repository))

	// Read environment variables
	utils.SetupEnvironmentCredentials(cmdFlags)

	// Validate inputs
	if err := utils.ValidateExportFlags(cmdFlags); err != nil {
		return err
	}

	if cmdFlags.BitbucketToken != "" {
		logger.Info("Using token authentication")
	} else if cmdFlags.BitbucketUser != "" && cmdFlags.BitbucketAppPass != "" {
		logger.Info("Using basic authentication",
			zap.String("username", cmdFlags.BitbucketUser))
	}

	// Create Bitbucket client and exporter
	client := utils.NewClient(
		cmdFlags.BitbucketAPIURL,
		cmdFlags.BitbucketToken,
		cmdFlags.BitbucketUser,
		cmdFlags.BitbucketAppPass,
		logger,
	)

	if cmdFlags.OpenPRsOnly {
		logger.Info("Filtering: Only open PRs will be exported")
	}
	if cmdFlags.PRsFromDate != "" {
		logger.Info("Filtering: PRs from date", zap.String("from_date", cmdFlags.PRsFromDate))
	}

	exporter := utils.NewExporter(client, cmdFlags.OutputDir, logger, cmdFlags.OpenPRsOnly, cmdFlags.PRsFromDate)

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
