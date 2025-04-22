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

	// Configure flags for command
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketToken, "token", "t", "", "Bitbucket access token for authentication")
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketUser, "user", "u", "", "Bitbucket username for basic authentication")
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketAppPass, "app-password", "p", "", "Bitbucket app password for basic authentication")
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.BitbucketAPIURL, "bbc-api-url", "a", "https://api.bitbucket.org/2.0", "Bitbucket API to use")
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.Repository, "repo", "r", "", "Name of the repository to export from Bitbucket Cloud")
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.Workspace, "workspace", "w", "", "Bitbucket workspace name")
	exportCmd.PersistentFlags().StringVarP(&cmdFlags.OutputDir, "output", "o", "", "Output directory for exported data (default: ./bitbucket-export-TIMESTAMP)")
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

	// Validate inputs
	if err := utils.ValidateExportFlags(cmdFlags); err != nil {
		return err
	}

	utils.SetupEnvironmentCredentials(cmdFlags)

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
