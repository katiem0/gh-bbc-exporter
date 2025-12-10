package migrate

import (
	"fmt"
	"time"

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
			if exportFlags.Workspace == "" {
				return fmt.Errorf("bitbucket workspace must be specified")
			}
			if exportFlags.Repository == "" {
				return fmt.Errorf("bitbucket repository must be specified")
			}
			if migrateFlags.TargetOrg == "" {
				return fmt.Errorf("target github organization must be specified")
			}

			if exportFlags.PRsFromDate != "" {
				_, err := time.Parse("2006-01-02", exportFlags.PRsFromDate)
				if err != nil {
					return fmt.Errorf("invalid date format for --prs-from-date: %s (expected YYYY-MM-DD)", exportFlags.PRsFromDate)
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var gqlClient *api.GraphQLClient
			var restClient *api.RESTClient
			var err error

			cmd.SilenceUsage = true
			logger, err := log.NewLogger(exportFlags.Debug)
			if err != nil {
				return fmt.Errorf("failed to initialize logger: %w", err)
			}
			defer func() {
				_ = logger.Sync()
			}()
			zap.ReplaceGlobals(logger)

			logger.Debug("Migrate command initialized",
				zap.Bool("debug", exportFlags.Debug),
				zap.String("workspace", exportFlags.Workspace),
				zap.String("repository", exportFlags.Repository),
				zap.String("targetOrg", migrateFlags.TargetOrg),
				zap.String("targetRepo", migrateFlags.TargetRepo))

			host := "github.com"

			logger.Debug("Retrieving GitHub authentication token")
			authToken, err = utils.GetGitHubAuthToken(&migrateFlags)
			if err != nil {
				logger.Debug("Failed to get GitHub authentication token", zap.Error(err))
				return fmt.Errorf("failed to get GitHub authentication token: %w", err)
			}
			logger.Debug("GitHub authentication token retrieved",
				zap.Int("tokenLength", len(authToken)))

			logger.Debug("Creating GraphQL client",
				zap.String("host", host))
			gqlClient, err = api.NewGraphQLClient(api.ClientOptions{
				Headers: map[string]string{
					"Accept": "application/vnd.github.hawkgirl-preview+json",
				},
				Host:      host,
				AuthToken: authToken,
			})

			if err != nil {
				logger.Debug("Failed to create GraphQL client", zap.Error(err))
				zap.S().Errorf("Error arose retrieving graphql client")
				return err
			}
			logger.Debug("GraphQL client created successfully")

			logger.Debug("Creating REST client",
				zap.String("host", host))
			restClient, err = api.NewRESTClient(api.ClientOptions{
				Headers: map[string]string{
					"Accept": "application/vnd.github+json",
				},
				Host:      host,
				AuthToken: authToken,
			})

			if err != nil {
				logger.Debug("Failed to create REST client", zap.Error(err))
				zap.S().Errorf("Error arose retrieving rest client")
				return err
			}
			logger.Debug("REST client created successfully")

			logger.Debug("Starting migration process")
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
	logger.Debug("runCmdMigrate started",
		zap.String("workspace", exportFlags.Workspace),
		zap.String("repository", exportFlags.Repository),
		zap.String("targetOrg", migrateFlags.TargetOrg),
		zap.String("targetRepo", migrateFlags.TargetRepo),
		zap.String("visibility", migrateFlags.TargetRepoVisibility.String()),
		zap.Bool("keepArchive", migrateFlags.KeepArchive))

	logger.Info("Starting Bitbucket to GitHub migration",
		zap.String("source", fmt.Sprintf("%s/%s", exportFlags.Workspace, exportFlags.Repository)),
		zap.String("target", fmt.Sprintf("%s/%s", migrateFlags.TargetOrg, migrateFlags.TargetRepo)))

	logger.Info("Step 1: Exporting from Bitbucket Cloud")
	logger.Debug("Setting up environment credentials")

	utils.SetupEnvironmentCredentials(exportFlags)

	logger.Debug("Validating export flags",
		zap.String("apiURL", exportFlags.BitbucketAPIURL),
		zap.Bool("hasAccessToken", exportFlags.BitbucketAccessToken != ""),
		zap.Bool("hasAPIToken", exportFlags.BitbucketAPIToken != ""),
		zap.Bool("hasEmail", exportFlags.BitbucketEmail != ""),
		zap.Bool("hasUser", exportFlags.BitbucketUser != ""),
		zap.Bool("hasAppPass", exportFlags.BitbucketAppPass != ""))

	if err := utils.ValidateExportFlags(exportFlags); err != nil {
		logger.Debug("Export validation failed", zap.Error(err))
		return fmt.Errorf("export validation failed: %w", err)
	}
	logger.Debug("Export flags validated successfully")

	logger.Debug("Creating Bitbucket client",
		zap.String("apiURL", exportFlags.BitbucketAPIURL),
		zap.String("outputDir", exportFlags.OutputDir),
		zap.Bool("skipCommitLookup", exportFlags.SkipCommitLookup))

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
	logger.Debug("Bitbucket client created")

	logger.Debug("Creating exporter",
		zap.String("outputDir", exportFlags.OutputDir),
		zap.Bool("openPRsOnly", exportFlags.OpenPRsOnly),
		zap.String("prsFromDate", exportFlags.PRsFromDate))

	exporter := utils.NewExporter(client, exportFlags.OutputDir, logger, exportFlags.OpenPRsOnly, exportFlags.PRsFromDate)

	logger.Debug("Starting export",
		zap.String("workspace", exportFlags.Workspace),
		zap.String("repository", exportFlags.Repository))

	if err := exporter.Export(exportFlags.Workspace, exportFlags.Repository); err != nil {
		logger.Debug("Export failed", zap.Error(err))
		return fmt.Errorf("export failed: %w", err)
	}
	logger.Debug("Export completed successfully")

	archivePath := exporter.GetOutputPath()
	logger.Debug("Archive path determined", zap.String("archivePath", archivePath))

	logger.Info("Step 2: Importing to GitHub Enterprise Cloud",
		zap.String("archive", archivePath))

	logger.Debug("Starting GitHub API migration",
		zap.String("archivePath", archivePath),
		zap.String("targetOrg", migrateFlags.TargetOrg))

	if err := utils.RunGitHubAPIMigration(exportFlags, migrateFlags, archivePath, g, logger); err != nil {
		logger.Debug("Import failed", zap.Error(err))
		return fmt.Errorf("import failed: %w", err)
	}

	logger.Info("Migration completed successfully")
	logger.Debug("Migration process finished",
		zap.String("source", fmt.Sprintf("%s/%s", exportFlags.Workspace, exportFlags.Repository)),
		zap.String("target", fmt.Sprintf("%s/%s", migrateFlags.TargetOrg, migrateFlags.TargetRepo)))

	// if !migrateFlags.KeepArchive {
	// 	logger.Debug("Cleaning up migration archive", zap.String("archive", archivePath))
	// 	if err := os.Remove(archivePath); err != nil {
	// 		logger.Warn("Failed to remove archive file", zap.String("archive", archivePath), zap.Error(err))
	// 		logger.Debug("Archive cleanup failed",
	// 			zap.String("archivePath", archivePath),
	// 			zap.Error(err))
	// 	} else {
	// 		logger.Info("Archive cleaned up", zap.String("archive", archivePath))
	// 		logger.Debug("Archive removed successfully",
	// 			zap.String("archivePath", archivePath))
	// 	}
	// } else {
	// 	logger.Info("Archive retained", zap.String("archive", archivePath))
	// 	logger.Debug("Archive retained per user request",
	// 		zap.String("archivePath", archivePath),
	// 		zap.Bool("keepArchive", migrateFlags.KeepArchive))
	// }
	return nil
}
