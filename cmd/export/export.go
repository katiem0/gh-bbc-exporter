package export

import (
	"errors"
	"fmt"

	"github.com/katiem0/gh-bbc-exporter/internal/data"
	"github.com/katiem0/gh-bbc-exporter/internal/log"
	"github.com/katiem0/gh-bbc-exporter/internal/utils"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func NewCmdExport() *cobra.Command {
	cmdExportFlags := data.CmdExportFlags{}

	exportCmd := &cobra.Command{
		Use:   "export [flags]",
		Short: "Export repository and metadata from Bitbucket Cloud",
		Long:  "Export repository and metadata from Bitbucket Cloud for GitHub Cloud import.",
		PreRunE: func(exportCmd *cobra.Command, args []string) error {
			if len(cmdExportFlags.Workspace) == 0 {
				return errors.New("a Bitbucket Workspace must be specified")
			}
			if len(cmdExportFlags.Repository) == 0 {
				return errors.New("a Bitbucket repository must be specified")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			logger, err := log.NewLogger(cmdExportFlags.Debug)
			if err != nil {
				return fmt.Errorf("failed to initialize logger: %w", err)
			}
			defer func() {
				_ = logger.Sync()
			}()
			zap.ReplaceGlobals(logger)
			return runCmdExport(&cmdExportFlags, logger)
		},
	}

	exportCmd.Flags().SortFlags = false
	exportCmd.PersistentFlags().SortFlags = false

	utils.SetupCommandUsageTemplate(exportCmd, 100)

	exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.BitbucketAPIURL, "bbc-api-url", "a",
		"https://api.bitbucket.org/2.0", "Bitbucket API to use")
	exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.BitbucketAccessToken, "access-token", "t", "",
		"Bitbucket workspace access token for authentication (env: BITBUCKET_ACCESS_TOKEN)")
	exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.BitbucketAPIToken, "api-token", "", "",
		"Bitbucket API token for authentication (env: BITBUCKET_API_TOKEN)")
	exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.BitbucketEmail, "email", "e", "",
		"Atlassian account email for API token authentication (env: BITBUCKET_EMAIL)")
	exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.BitbucketUser, "user", "u", "",
		"Bitbucket username for basic authentication (env: BITBUCKET_USERNAME)")
	exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.BitbucketAppPass, "app-password", "p", "",
		"Bitbucket app password for basic authentication (env: BITBUCKET_APP_PASSWORD)")
	exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.Workspace, "workspace", "w", "",
		"Bitbucket workspace name")
	exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.Repository, "repo", "r", "",
		"Name of the repository to export from Bitbucket Cloud")
	exportCmd.PersistentFlags().StringVar(&cmdExportFlags.TempDir, "temp-dir", "",
		"Temporary directory for cloning (env: BITBUCKET_TEMP_DIR)")
	exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.OutputDir, "output", "o", "",
		"Output directory for exported data (default: ./bitbucket-export-TIMESTAMP)")
	exportCmd.PersistentFlags().BoolVar(&cmdExportFlags.OpenPRsOnly, "open-prs-only", false,
		"Export only open pull requests and ignore closed/merged ones")
	exportCmd.PersistentFlags().StringVarP(&cmdExportFlags.PRsFromDate, "prs-from-date", "", "",
		"Export pull requests created on or after this date (format: YYYY-MM-DD)")
	exportCmd.PersistentFlags().BoolVar(&cmdExportFlags.SkipCommitLookup, "skip-commit-lookup", false,
		"Skip Bitbucket API lookups to retrieve commit SHAs (use local lookup only)")
	exportCmd.PersistentFlags().BoolVarP(&cmdExportFlags.Debug, "debug", "d", false, "Enable debug logging")

	if err := exportCmd.MarkPersistentFlagRequired("workspace"); err != nil {
		fmt.Printf("Error marking workspace flag as required: %v\n", err)
	}
	if err := exportCmd.MarkPersistentFlagRequired("repo"); err != nil {
		fmt.Printf("Error marking repository flag as required: %v\n", err)
	}
	return exportCmd
}

func runCmdExport(cmdExportFlags *data.CmdExportFlags, logger *zap.Logger) error {
	logger.Info("Starting Bitbucket Cloud export",
		zap.String("workspace", cmdExportFlags.Workspace),
		zap.String("repository", cmdExportFlags.Repository))

	// Read environment variables
	utils.SetupEnvironmentCredentials(cmdExportFlags)

	// Validate inputs
	if err := utils.ValidateExportFlags(cmdExportFlags); err != nil {
		return err
	}

	if cmdExportFlags.BitbucketAccessToken != "" {
		logger.Info("Using workspace access token authentication")
	} else if cmdExportFlags.BitbucketAPIToken != "" {
		logger.Info("Using API token authentication")
	} else if cmdExportFlags.BitbucketUser != "" && cmdExportFlags.BitbucketAppPass != "" {
		logger.Info("Using basic authentication",
			zap.String("username", cmdExportFlags.BitbucketUser))
	}

	// Create Bitbucket client and exporter
	client := utils.NewClient(
		cmdExportFlags.BitbucketAPIURL,
		cmdExportFlags.BitbucketAccessToken,
		cmdExportFlags.BitbucketAPIToken,
		cmdExportFlags.BitbucketEmail,
		cmdExportFlags.BitbucketUser,
		cmdExportFlags.BitbucketAppPass,
		logger,
		cmdExportFlags.OutputDir,
		cmdExportFlags.SkipCommitLookup,
	)

	if cmdExportFlags.OpenPRsOnly {
		logger.Info("Filtering: Only open PRs will be exported")
	}
	if cmdExportFlags.PRsFromDate != "" {
		logger.Info("Filtering: PRs from date", zap.String("from_date", cmdExportFlags.PRsFromDate))
	}

	// Apply commit SHA expansion behavior
	if cmdExportFlags.SkipCommitLookup {
		logger.Info("Skipping Bitbucket API commit SHA lookups (will look locally only)")
	}

	exporter := utils.NewExporter(client, cmdExportFlags.OutputDir, logger, cmdExportFlags.OpenPRsOnly, cmdExportFlags.PRsFromDate)

	if cmdExportFlags.TempDir != "" {
		exporter.SetTempDir(cmdExportFlags.TempDir)
		logger.Debug("Using custom temporary directory", zap.String("temp_dir", cmdExportFlags.TempDir))
	}
	// Run export
	if err := exporter.Export(cmdExportFlags.Workspace, cmdExportFlags.Repository); err != nil {
		logger.Error("Export failed")
		return err
	}

	// Print success message
	outputPath := exporter.GetOutputPath()
	utils.PrintSuccessMessage(outputPath)

	logger.Info("Export completed successfully")
	return nil
}
