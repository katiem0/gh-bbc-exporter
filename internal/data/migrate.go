package data

import (
	"fmt"
)

type RepoVisibility string

type CmdMigrateFlags struct {
	TargetOrg                    string
	TargetRepo                   string
	TargetAPIUrl                 string
	TargetRepoVisibility         RepoVisibility
	GitHubPAT                    string
	UseGitHubStorage             bool
	AzureStorageConnectionString string
	AWSBucketName                string
	AWSAccessKey                 string
	AWSSecretKey                 string
	AWSSessionToken              string
	AWSRegion                    string
	KeepArchive                  bool
}

func (v *RepoVisibility) String() string {
	return string(*v)
}

func (v *RepoVisibility) Set(s string) error {
	switch s {
	case "public", "private", "internal", "":
		*v = RepoVisibility(s)
		return nil
	default:
		return fmt.Errorf("must be one of: internal, private, public")
	}
}

func (v *RepoVisibility) Type() string {
	return "<internal|private|public>"
}
