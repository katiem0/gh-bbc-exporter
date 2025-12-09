package migrate

import (
	"fmt"
	"os"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/katiem0/gh-bbc-exporter/internal/data"
	"github.com/katiem0/gh-bbc-exporter/internal/log"
	"github.com/katiem0/gh-bbc-exporter/internal/utils"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func NewCmdMigrate() *cobra.Command {
	exportFlags := data.CmdExportFlags{}
	migrateFlags := data.CmdMigrateFlags{}
	var authToken string

	migrateCmd := &cobra.Command{
		Use:   "migrate [flags]",
		Short: "Export from Bitbucket and import to GitHub",
		Long:  "Migrate a repository from Bitbucket Cloud to GitHub Enterprise.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if len(exportFlags.Workspace) == 0 {
				return fmt.Errorf("a Bitbucket Workspace must be specified")
			}
			if len(exportFlags.Repository) == 0 {
				return fmt.Errorf("a Bitbucket repository must be specified")
			}
			if len(migrateFlags.TargetOrg) == 0 {
				return fmt.Errorf("a target GitHub organization must be specified")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var gqlClient *api.GraphQLClient
			var restClient *api.RESTClient
			cmd.SilenceUsage = true
			logger, err := log.NewLogger(exportFlags.Debug)
			if err != nil {
				return fmt.Errorf("failed to initialize logger: %w", err)
			}
			defer func() {
				_ = logger.Sync()
			}()
			zap.ReplaceGlobals(logger)
			host := "github.com"

			authToken, err = utils.GetGitHubAuthToken(&migrateFlags)
			if err != nil {
				return fmt.Errorf("failed to get GitHub authentication token: %w", err)
			}

			gqlClient, err = api.NewGraphQLClient(api.ClientOptions{
				Headers: map[string]string{
					"Accept": "application/vnd.github.hawkgirl-preview+json",
				},
				Host:      host,
				AuthToken: authToken,
			})

			if err != nil {
				zap.S().Errorf("Error arose retrieving graphql client")
				return err
			}

			restClient, err = api.NewRESTClient(api.ClientOptions{
				Headers: map[string]string{
					"Accept": "application/vnd.github+json",
				},
				Host:      host,
				AuthToken: authToken,
			})

			if err != nil {
				zap.S().Errorf("Error arose retrieving rest client")
				return err
			}

			return runCmdMigrate(&exportFlags, &migrateFlags, utils.NewAPIGetter(gqlClient, restClient), logger)
		},
	}

	migrateCmd.PersistentFlags().StringVarP(&exportFlags.BitbucketAPIURL, "bbc-api-url", "a",
		"https://api.bitbucket.org/2.0", "Bitbucket API to use")
	migrateCmd.PersistentFlags().StringVarP(&exportFlags.BitbucketAccessToken, "access-token", "t", "",
		"Bitbucket workspace access token for authentication (env: BITBUCKET_ACCESS_TOKEN)")
	migrateCmd.PersistentFlags().StringVarP(&exportFlags.BitbucketAPIToken, "api-token", "", "",
		"Bitbucket API token for authentication (env: BITBUCKET_API_TOKEN)")
	migrateCmd.PersistentFlags().StringVarP(&exportFlags.BitbucketEmail, "email", "e", "",
		"Atlassian account email for API token authentication (env: BITBUCKET_EMAIL)")
	migrateCmd.PersistentFlags().StringVarP(&exportFlags.BitbucketUser, "user", "u", "",
		"Bitbucket username for basic authentication (env: BITBUCKET_USERNAME)")
	migrateCmd.PersistentFlags().StringVarP(&exportFlags.BitbucketAppPass, "app-password", "p", "",
		"Bitbucket app password for basic authentication (env: BITBUCKET_APP_PASSWORD)")
	migrateCmd.PersistentFlags().StringVarP(&exportFlags.Workspace, "workspace", "w", "",
		"Bitbucket workspace name")
	migrateCmd.PersistentFlags().StringVarP(&exportFlags.Repository, "repo", "r", "",
		"Name of the repository to export from Bitbucket Cloud")
	migrateCmd.PersistentFlags().StringVar(&exportFlags.TempDir, "temp-dir", "",
		"Temporary directory for cloning (env: BITBUCKET_TEMP_DIR)")
	migrateCmd.PersistentFlags().StringVarP(&exportFlags.OutputDir, "output", "o", "",
		"Output directory for exported data (default: ./bitbucket-export-TIMESTAMP)")
	migrateCmd.PersistentFlags().BoolVar(&exportFlags.OpenPRsOnly, "open-prs-only", false,
		"Export only open pull requests")
	migrateCmd.PersistentFlags().StringVar(&exportFlags.PRsFromDate, "prs-from-date", "",
		"Export pull requests created on or after this date (format: YYYY-MM-DD)")
	migrateCmd.PersistentFlags().BoolVar(&exportFlags.SkipCommitLookup, "skip-commit-lookup", false,
		"Skip Bitbucket API lookups to retrieve commit SHAs (use local lookup only)")

	migrateCmd.PersistentFlags().StringVar(&migrateFlags.TargetOrg, "target-org", "",
		"Target GitHub organization (required)")
	migrateCmd.PersistentFlags().StringVar(&migrateFlags.TargetRepo, "target-repo", "",
		"Target repository name (defaults to source repo name)")
	migrateCmd.PersistentFlags().StringVar(&migrateFlags.GitHubPAT, "github-target-pat", "",
		"GitHub Personal Access Token (env: GH_PAT)")
	migrateCmd.PersistentFlags().BoolVar(&migrateFlags.UseGitHubStorage, "use-github-storage", true,
		"Use GitHub-owned storage for migration")
	migrateCmd.PersistentFlags().StringVar(&migrateFlags.AzureStorageConnectionString, "azure-storage-connection-string", "",
		"The connection string for the Azure storage account, used to upload data archives pre-migration.")
	migrateCmd.PersistentFlags().StringVar(&migrateFlags.AWSBucketName, "aws-bucket-name", "",
		"If using AWS, the name of the S3 bucket to upload the data archives to.")
	migrateCmd.PersistentFlags().StringVar(&migrateFlags.AWSRegion, "aws-region", "",
		"If using AWS, required to specify the AWS region. (env: AWS_REGION)")
	migrateCmd.PersistentFlags().StringVar(&migrateFlags.AWSAccessKey, "aws-access-key", "",
		"If uploading to S3, the AWS access key. (env: AWS_ACCESS_KEY_ID)")
	migrateCmd.PersistentFlags().StringVar(&migrateFlags.AWSSecretKey, "aws-secret-key", "",
		"If uploading to S3, the AWS secret key. (env: AWS_SECRET_ACCESS_KEY)")
	migrateCmd.PersistentFlags().StringVar(&migrateFlags.AWSSessionToken, "aws-session-token", "",
		"If using AWS, the AWS session token. (env: AWS_SESSION_TOKEN)")
	migrateCmd.PersistentFlags().Var(&migrateFlags.TargetRepoVisibility, "target-repo-visibility",
		"The visibility of the target repo. Defaults to private. Valid values are public, private, or internal.")
	migrateCmd.PersistentFlags().BoolVar(&migrateFlags.KeepArchive, "keep-archive", false,
		"Keep the migration archive after successful import (default: delete after import)")
	migrateCmd.PersistentFlags().BoolVarP(&exportFlags.Debug, "debug", "d", false, "Enable debug logging")

	if err := migrateCmd.MarkPersistentFlagRequired("workspace"); err != nil {
		fmt.Printf("Error marking workspace flag as required: %v\n", err)
	}
	if err := migrateCmd.MarkPersistentFlagRequired("repo"); err != nil {
		fmt.Printf("Error marking repository flag as required: %v\n", err)
	}
	if err := migrateCmd.MarkPersistentFlagRequired("target-org"); err != nil {
		fmt.Printf("Error marking target-org flag as required: %v\n", err)
	}

	utils.SetupCommandUsageTemplate(migrateCmd, 100)

	return migrateCmd
}

func runCmdMigrate(exportFlags *data.CmdExportFlags, migrateFlags *data.CmdMigrateFlags, g *utils.APIGetter, logger *zap.Logger) error {
	logger.Info("Starting Bitbucket to GitHub migration",
		zap.String("source", fmt.Sprintf("%s/%s", exportFlags.Workspace, exportFlags.Repository)),
		zap.String("target", fmt.Sprintf("%s/%s", migrateFlags.TargetOrg, migrateFlags.TargetRepo)))

	logger.Info("Step 1: Exporting from Bitbucket Cloud")

	utils.SetupEnvironmentCredentials(exportFlags)
	if err := utils.ValidateExportFlags(exportFlags); err != nil {
		return fmt.Errorf("export validation failed: %w", err)
	}

	client := utils.NewClient(
		exportFlags.BitbucketAPIURL,
		exportFlags.BitbucketAccessToken,
		exportFlags.BitbucketAPIToken,
		exportFlags.BitbucketEmail,
		exportFlags.BitbucketUser,
		exportFlags.BitbucketAppPass,
		logger,
		exportFlags.OutputDir,
		exportFlags.SkipCommitLookup,
	)

	exporter := utils.NewExporter(client, exportFlags.OutputDir, logger, exportFlags.OpenPRsOnly, exportFlags.PRsFromDate)

	if err := exporter.Export(exportFlags.Workspace, exportFlags.Repository); err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	archivePath := exporter.GetOutputPath()

	logger.Info("Step 2: Importing to GitHub Enterprise Cloud",
		zap.String("archive", archivePath))

	if err := utils.RunGitHubAPIMigration(exportFlags, migrateFlags, archivePath, g, logger); err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	logger.Info("Migration completed successfully")

	if !migrateFlags.KeepArchive {
		logger.Debug("Cleaning up migration archive", zap.String("archive", archivePath))
		if err := os.Remove(archivePath); err != nil {
			logger.Warn("Failed to remove archive file", zap.String("archive", archivePath), zap.Error(err))
		} else {
			logger.Info("Archive cleaned up", zap.String("archive", archivePath))
		}
	} else {
		logger.Info("Archive retained", zap.String("archive", archivePath))
	}
	return nil
}
