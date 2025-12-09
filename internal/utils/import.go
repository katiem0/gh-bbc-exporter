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
	targetRepo := migrateFlags.TargetRepo
	if targetRepo == "" {
		targetRepo = exportFlags.Repository
	}

	// Step 1: Get Organization ID
	logger.Info("Getting organization information", zap.String("org", migrateFlags.TargetOrg))
	orgInfo, err := g.getOrganizationInfo(migrateFlags.TargetOrg)
	if err != nil {
		return fmt.Errorf("failed to get organization info: %w", err)
	}
	logger.Debug("Organization Info retrieved", zap.String("orgID", orgInfo.Organization.ID))

	// Step 2: Create Migration Source
	logger.Info("Creating migration source")
	migrationSourceID, err := g.createMigrationSource("Bitbucket Cloud Migration", "https://bitbucket.org", orgInfo.Organization.ID)
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}
	logger.Info("Migration source created", zap.String("sourceID", migrationSourceID))

	// Step 3: Upload archive to GitHub-owned storage (if using GitHub storage)
	var archiveURI string
	if migrateFlags.UseGitHubStorage {
		logger.Info("Uploading archive to GitHub-owned storage", zap.String("archive", archivePath))
		archiveURI, err = g.UploadArchiveToGitHub(orgInfo.Organization.DatabaseID, archivePath, logger)
		if err != nil {
			return fmt.Errorf("failed to upload archive: %w", err)
		}
		logger.Info("Archive uploaded successfully", zap.String("uri", archiveURI))
	} else {
		// For other storage providers, construct the appropriate URI
		// This would need implementation based on the storage type
		return fmt.Errorf("non-GitHub storage providers not yet implemented for API migration")
	}

	// Step 4: Start Repository Migration
	logger.Info("Starting repository migration",
		zap.String("source", fmt.Sprintf("%s/%s", exportFlags.Workspace, exportFlags.Repository)),
		zap.String("target", fmt.Sprintf("%s/%s", migrateFlags.TargetOrg, targetRepo)))

	authToken, err := GetGitHubAuthToken(migrateFlags)
	if err != nil {
		return fmt.Errorf("failed to get GitHub authentication token: %w", err)
	}

	visibility := migrateFlags.TargetRepoVisibility.String()
	logger.Debug("Using repository visibility", zap.String("visibility", visibility))

	migrationID, err := g.startRepositoryMigration(migrationSourceID, orgInfo.Organization.ID, targetRepo, archiveURI,
		fmt.Sprintf("https://bitbucket.org/%s/%s", exportFlags.Workspace, exportFlags.Repository),
		visibility, authToken)
	if err != nil {
		return fmt.Errorf("failed to start migration: %w", err)
	}

	// Step 5: Monitor migration status
	logger.Info("Migration started", zap.String("migrationID", migrationID))
	if err := g.monitorMigrationStatus(migrationID, logger); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

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
	query := new(data.MigrationStatusQuery)
	variables := map[string]interface{}{
		"id": graphql.ID(id),
	}

	for {
		err := g.gqlClient.Query("GetMigrationStatus", &query, variables)
		if err != nil {
			return fmt.Errorf("failed to get migration status: %w", err)
		}

		migrationStatus := query.Node.Migration.State
		logger.Info("Monitoring migration status", zap.String("status", migrationStatus))

		if migrationStatus == "SUCCEEDED" {
			return nil
		} else if migrationStatus == "FAILED" {
			return fmt.Errorf("migration failed: %s", query.Node.Migration.FailureReason)
		}
		time.Sleep(10 * time.Second)
	}
}
