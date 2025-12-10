package data

import (
	"fmt"

	graphql "github.com/cli/shurcooL-graphql"
)

type RepoVisibility string

type CmdMigrateFlags struct {
	TargetOrg            string
	TargetRepo           string
	TargetRepoVisibility RepoVisibility
	GitHubPAT            string
}

type OrganizationIDQuery struct {
	Organization struct {
		ID         string `json:"id"`
		DatabaseID int    `json:"databaseId"`
	} `graphql:"organization(login: $login)"`
}

type MutationMigrationSource struct {
	CreateMigrationSource struct {
		MigrationSource struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"migrationSource"`
	} `graphql:"createMigrationSource(input: $input)"`
}

type CreateMigrationSourceInput struct {
	Name    graphql.String `json:"name"`
	URL     graphql.String `json:"url"`
	OwnerID graphql.String `json:"ownerId"`
	Type    graphql.String `json:"type"`
}

type StartMigrationResponse struct {
	StartRepositoryMigration struct {
		RepositoryMigration struct {
			ID string `json:"id"`
		} `json:"repositoryMigration"`
	} `graphql:"startRepositoryMigration(input: $input)"`
}

type StartRepositoryMigrationInput struct {
	SourceID             graphql.String  `json:"sourceId"`
	OwnerID              graphql.String  `json:"ownerId"`
	RepositoryName       graphql.String  `json:"repositoryName"`
	ContinueOnError      graphql.Boolean `json:"continueOnError"`
	GitHubPAT            graphql.String  `json:"githubPat"`
	AccessToken          graphql.String  `json:"accessToken"`
	GitArchiveUrl        graphql.String  `json:"gitArchiveUrl"`
	MetadataArchiveUrl   graphql.String  `json:"metadataArchiveUrl"`
	SourceRepositoryUrl  graphql.String  `json:"sourceRepositoryUrl"`
	TargetRepoVisibility graphql.String  `json:"targetRepoVisibility"`
}

type MigrationStatusQuery struct {
	Node struct {
		Migration struct {
			ID            graphql.ID `json:"id"`
			State         string     `json:"state"`
			FailureReason string     `json:"failureReason"`
		} `graphql:"... on Migration"`
	} `graphql:"node(id: $id)"`
}

type UploadResponse struct {
	URI string `json:"uri"`
}

func (v *RepoVisibility) String() string {
	if *v == "" {
		return "private"
	}
	return string(*v)
}

func (v *RepoVisibility) Set(s string) error {
	switch s {
	case "public", "private", "internal":
		*v = RepoVisibility(s)
		return nil
	case "":
		*v = RepoVisibility("private")
		return nil
	default:
		return fmt.Errorf("must be one of: internal, private, public")
	}
}

func (v *RepoVisibility) Type() string {
	return "<internal|private|public>"
}
