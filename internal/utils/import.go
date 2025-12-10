package utils

import (
	"fmt"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
	graphql "github.com/cli/shurcooL-graphql"
	"github.com/katiem0/gh-bbc-exporter/internal/data"
	"go.uber.org/zap"
)

type Getter interface {
	getOrganizationInfo(owner string) (*data.OrganizationIDQuery, error)
	createMigrationSource(name string, url string, ownerID string) (string, error)
	UploadArchiveToGitHub(orgID int, archivePath string, logger *zap.Logger) (string, error)
}

type APIGetter struct {
	gqlClient  api.GraphQLClient
	restClient api.RESTClient
}

func NewAPIGetter(gqlClient *api.GraphQLClient, restClient *api.RESTClient) *APIGetter {
	return &APIGetter{
		gqlClient:  *gqlClient,
		restClient: *restClient,
	}
}

func RunGitHubAPIMigration(exportFlags *data.CmdExportFlags, migrateFlags *data.CmdMigrateFlags, archivePath string, g *APIGetter, logger *zap.Logger) error {
	logger.Debug("Starting GitHub API migration process",
		zap.String("workspace", exportFlags.Workspace),
		zap.String("repository", exportFlags.Repository),
		zap.String("targetOrg", migrateFlags.TargetOrg),
		zap.String("archivePath", archivePath))

	targetRepo := migrateFlags.TargetRepo
	if targetRepo == "" {
		targetRepo = exportFlags.Repository
		logger.Debug("Target repo not specified, using source repo name",
			zap.String("targetRepo", targetRepo))
	}

	// Step 1: Get Organization ID
	logger.Info("Getting organization information", zap.String("org", migrateFlags.TargetOrg))
	logger.Debug("Step 1: Fetching organization ID via GraphQL",
		zap.String("orgLogin", migrateFlags.TargetOrg))

	orgInfo, err := g.getOrganizationInfo(migrateFlags.TargetOrg)
	if err != nil {
		logger.Debug("Failed to get organization info",
			zap.String("org", migrateFlags.TargetOrg),
			zap.Error(err))
		return fmt.Errorf("failed to get organization info: %w", err)
	}
	logger.Debug("Organization Info retrieved",
		zap.String("orgID", orgInfo.Organization.ID),
		zap.Int("databaseID", orgInfo.Organization.DatabaseID))

	// Step 2: Create Migration Source
	logger.Info("Creating migration source")
	logger.Debug("Step 2: Creating migration source via GraphQL",
		zap.String("sourceName", "Bitbucket Cloud Migration"),
		zap.String("sourceURL", "https://bitbucket.org"),
		zap.String("ownerID", orgInfo.Organization.ID))

	migrationSourceID, err := g.createMigrationSource("Bitbucket Cloud Migration", "https://bitbucket.org", orgInfo.Organization.ID)
	if err != nil {
		logger.Debug("Failed to create migration source", zap.Error(err))
		return fmt.Errorf("failed to create migration source: %w", err)
	}
	logger.Info("Migration source created", zap.String("sourceID", migrationSourceID))
	logger.Debug("Migration source created successfully",
		zap.String("migrationSourceID", migrationSourceID))

	// Step 3: Upload archive to GitHub-owned storage
	var archiveURI string
	logger.Info("Uploading archive to GitHub-owned storage", zap.String("archive", archivePath))
	logger.Debug("Step 3: Initiating archive upload",
		zap.String("archivePath", archivePath),
		zap.Int("orgDatabaseID", orgInfo.Organization.DatabaseID))

	uploadStartTime := time.Now()
	archiveURI, err = g.UploadArchiveToGitHub(orgInfo.Organization.DatabaseID, archivePath, logger)
	uploadDuration := time.Since(uploadStartTime)
	if err != nil {
		logger.Debug("Failed to upload archive",
			zap.Duration("duration", uploadDuration),
			zap.Error(err))
		return fmt.Errorf("failed to upload archive: %w", err)
	}
	logger.Info("Archive uploaded successfully", zap.String("uri", archiveURI))
	logger.Debug("Archive upload completed",
		zap.String("archiveURI", archiveURI),
		zap.Duration("uploadDuration", uploadDuration))

	// Step 4: Start Repository Migration
	logger.Info("Starting repository migration",
		zap.String("source", fmt.Sprintf("%s/%s", exportFlags.Workspace, exportFlags.Repository)),
		zap.String("target", fmt.Sprintf("%s/%s", migrateFlags.TargetOrg, targetRepo)))

	logger.Debug("Step 4: Retrieving GitHub authentication token")
	authToken, err := GetGitHubAuthToken(migrateFlags)
	if err != nil {
		logger.Debug("Failed to get GitHub authentication token", zap.Error(err))
		return fmt.Errorf("failed to get GitHub authentication token: %w", err)
	}
	logger.Debug("GitHub authentication token retrieved",
		zap.Int("tokenLength", len(authToken)))

	visibility := migrateFlags.TargetRepoVisibility.String()
	logger.Debug("Using repository visibility", zap.String("visibility", visibility))

	sourceURL := fmt.Sprintf("https://bitbucket.org/%s/%s", exportFlags.Workspace, exportFlags.Repository)
	logger.Debug("Preparing to start repository migration",
		zap.String("sourceID", migrationSourceID),
		zap.String("orgID", orgInfo.Organization.ID),
		zap.String("targetRepo", targetRepo),
		zap.String("archiveURI", archiveURI),
		zap.String("sourceURL", sourceURL),
		zap.String("visibility", visibility))

	migrationID, err := g.startRepositoryMigration(migrationSourceID, orgInfo.Organization.ID, targetRepo, archiveURI,
		sourceURL, visibility, authToken)
	if err != nil {
		logger.Debug("Failed to start migration", zap.Error(err))
		return fmt.Errorf("failed to start migration: %w", err)
	}

	// Step 5: Monitor migration status
	logger.Info("Migration started", zap.String("migrationID", migrationID))
	logger.Debug("Step 5: Beginning migration status monitoring",
		zap.String("migrationID", migrationID))

	if err := g.monitorMigrationStatus(migrationID, logger); err != nil {
		logger.Debug("Migration failed during monitoring", zap.Error(err))
		return fmt.Errorf("migration failed: %w", err)
	}

	logger.Debug("GitHub API migration completed successfully",
		zap.String("migrationID", migrationID),
		zap.String("targetRepo", targetRepo))

	return nil
}

func (g *APIGetter) getOrganizationInfo(login string) (*data.OrganizationIDQuery, error) {
	query := new(data.OrganizationIDQuery)
	variables := map[string]interface{}{
		"login": graphql.String(login),
	}

	err := g.gqlClient.Query("GetOrgInfo", &query, variables)
	return query, err
}

func (g *APIGetter) createMigrationSource(name string, url string, ownerID string) (string, error) {
	mutation := new(data.MutationMigrationSource)
	variables := map[string]interface{}{
		"input": data.CreateMigrationSourceInput{
			Name:    graphql.String(name),
			URL:     graphql.String(url),
			OwnerID: graphql.String(ownerID),
			Type:    graphql.String("GITHUB_ARCHIVE"),
		},
	}

	err := g.gqlClient.Mutate("createMigrationSource", &mutation, variables)
	if err != nil {
		return "", fmt.Errorf("failed to create migration source: %w", err)
	}
	migrationSourceID := mutation.CreateMigrationSource.MigrationSource.ID

	return migrationSourceID, nil
}

func (g *APIGetter) startRepositoryMigration(sourceID string, orgID string, targetRepo string, archiveURI string, sourceURL string, visibility string, githubPAT string) (string, error) {
	mutation := new(data.StartMigrationResponse)
	variables := map[string]interface{}{
		"input": data.StartRepositoryMigrationInput{
			SourceID:             graphql.String(sourceID),
			OwnerID:              graphql.String(orgID),
			RepositoryName:       graphql.String(targetRepo),
			ContinueOnError:      graphql.Boolean(true),
			GitHubPAT:            graphql.String(githubPAT),
			AccessToken:          graphql.String(githubPAT),
			GitArchiveUrl:        graphql.String(archiveURI),
			MetadataArchiveUrl:   graphql.String(archiveURI),
			SourceRepositoryUrl:  graphql.String(sourceURL),
			TargetRepoVisibility: graphql.String(visibility),
		},
	}

	err := g.gqlClient.Mutate("startRepositoryMigration", &mutation, variables)
	if err != nil {
		return "", fmt.Errorf("failed to start repository migration: %w", err)
	}
	migrationID := mutation.StartRepositoryMigration.RepositoryMigration.ID

	return migrationID, nil
}

func (g *APIGetter) monitorMigrationStatus(id string, logger *zap.Logger) error {
	logger.Debug("Starting migration status monitoring",
		zap.String("migrationID", id))

	query := new(data.MigrationStatusQuery)
	variables := map[string]interface{}{
		"id": graphql.ID(id),
	}

	pollCount := 0
	startTime := time.Now()

	for {
		pollCount++
		logger.Debug("Polling migration status",
			zap.Int("pollCount", pollCount),
			zap.Duration("elapsed", time.Since(startTime)))

		err := g.gqlClient.Query("GetMigrationStatus", &query, variables)
		if err != nil {
			logger.Debug("Failed to get migration status",
				zap.Int("pollCount", pollCount),
				zap.Error(err))
			return fmt.Errorf("failed to get migration status: %w", err)
		}

		migrationStatus := query.Node.Migration.State
		logger.Info("Monitoring migration status", zap.String("status", migrationStatus))
		logger.Debug("Migration status details",
			zap.String("migrationID", id),
			zap.String("state", migrationStatus),
			zap.Int("pollCount", pollCount),
			zap.Duration("elapsed", time.Since(startTime)))

		switch migrationStatus {
		case "SUCCEEDED":
			logger.Debug("Migration completed successfully",
				zap.String("migrationID", id),
				zap.Int("totalPolls", pollCount),
				zap.Duration("totalDuration", time.Since(startTime)))
			return nil
		case "FAILED":
			failureReason := query.Node.Migration.FailureReason
			logger.Debug("Migration failed",
				zap.String("migrationID", id),
				zap.String("failureReason", failureReason),
				zap.Int("totalPolls", pollCount),
				zap.Duration("totalDuration", time.Since(startTime)))
			return fmt.Errorf("migration failed: %s", failureReason)
		default:
			logger.Debug("Migration still in progress, waiting before next poll",
				zap.String("currentState", migrationStatus),
				zap.Duration("sleepDuration", 10*time.Second))
			time.Sleep(10 * time.Second)
		}
	}
}
