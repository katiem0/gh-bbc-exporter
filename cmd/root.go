package cmd

import (
	"fmt"
	"strings"

	"github.com/katiem0/gh-bbc-exporter/internal/log"
	"github.com/katiem0/gh-bbc-exporter/internal/utils"
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
		Use:   "bbc-exporter",
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
	logger.Info("Starting BitBucket Cloud export",
		zap.String("workspace", cmdFlags.workspace),
		zap.String("repository", cmdFlags.repository))

	// Validate authentication
	if cmdFlags.bitbucketToken == "" && (cmdFlags.bitbucketUser == "" || cmdFlags.bitbucketAppPass == "") {
		return fmt.Errorf("either token or both username and app password must be provided")
	}

	// Create BitBucket client
	client := utils.NewClient(
		cmdFlags.bitbucketAPIURL,
		cmdFlags.bitbucketToken,
		cmdFlags.bitbucketUser,
		cmdFlags.bitbucketAppPass,
		logger,
	)
	exporter := utils.NewExporter(client, cmdFlags.outputDir, logger)

	// Run export
	if err := exporter.Export(cmdFlags.workspace, cmdFlags.repository, logger); err != nil {
		logger.Error("Export failed", zap.Error(err))
		return err
	}

	// Check if output is an archive
	outputPath := exporter.GetOutputPath()
	if strings.HasSuffix(outputPath, ".tar.gz") {
		fmt.Printf("\nExport successful!\nArchive created: %s\n", outputPath)
		fmt.Println("You can use this archive with GitHub's repository importer.")
	} else {
		fmt.Printf("\nExport successful!\nOutput directory: %s\n", outputPath)
	}

	logger.Info("Export completed successfully")
	return nil
}
