package data

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepoVisibilityString(t *testing.T) {
	testCases := []struct {
		name     string
		value    RepoVisibility
		expected string
	}{
		{
			name:     "Public visibility",
			value:    RepoVisibility("public"),
			expected: "public",
		},
		{
			name:     "Private visibility",
			value:    RepoVisibility("private"),
			expected: "private",
		},
		{
			name:     "Internal visibility",
			value:    RepoVisibility("internal"),
			expected: "internal",
		},
		{
			name:     "Empty visibility",
			value:    RepoVisibility(""),
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.value.String()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestRepoVisibilitySet(t *testing.T) {
	testCases := []struct {
		name        string
		value       string
		expectError bool
		expected    RepoVisibility
	}{
		{
			name:        "Set public",
			value:       "public",
			expectError: false,
			expected:    RepoVisibility("public"),
		},
		{
			name:        "Set private",
			value:       "private",
			expectError: false,
			expected:    RepoVisibility("private"),
		},
		{
			name:        "Set internal",
			value:       "internal",
			expectError: false,
			expected:    RepoVisibility("internal"),
		},
		{
			name:        "Set empty string",
			value:       "",
			expectError: false,
			expected:    RepoVisibility(""),
		},
		{
			name:        "Invalid visibility - uppercase",
			value:       "PUBLIC",
			expectError: true,
			expected:    RepoVisibility(""),
		},
		{
			name:        "Invalid visibility - random string",
			value:       "protected",
			expectError: true,
			expected:    RepoVisibility(""),
		},
		{
			name:        "Invalid visibility - numeric",
			value:       "123",
			expectError: true,
			expected:    RepoVisibility(""),
		},
		{
			name:        "Invalid visibility - mixed case",
			value:       "Private",
			expectError: true,
			expected:    RepoVisibility(""),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var rv RepoVisibility
			err := rv.Set(tc.value)

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "must be one of")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, rv)
			}
		})
	}
}

func TestRepoVisibilityType(t *testing.T) {
	var rv RepoVisibility
	typeStr := rv.Type()
	assert.Contains(t, typeStr, "public")
	assert.Contains(t, typeStr, "private")
	assert.Contains(t, typeStr, "internal")
}

func TestCmdMigrateFlagsDefaults(t *testing.T) {
	migrateFlags := CmdMigrateFlags{}

	assert.Empty(t, migrateFlags.TargetOrg)
	assert.Empty(t, migrateFlags.TargetRepo)
	assert.Empty(t, migrateFlags.GitHubPAT)
	assert.False(t, migrateFlags.UseGitHubStorage)
	assert.Empty(t, migrateFlags.AzureStorageConnectionString)
	assert.Empty(t, migrateFlags.AWSBucketName)
	assert.Empty(t, migrateFlags.AWSRegion)
	assert.Empty(t, migrateFlags.AWSAccessKey)
	assert.Empty(t, migrateFlags.AWSSecretKey)
	assert.Empty(t, migrateFlags.AWSSessionToken)
	assert.Equal(t, RepoVisibility(""), migrateFlags.TargetRepoVisibility)
	assert.False(t, migrateFlags.KeepArchive)
}

func TestCmdMigrateFlagsWithValues(t *testing.T) {
	flags := CmdMigrateFlags{
		TargetOrg:                    "github-org",
		TargetRepo:                   "github-repo",
		GitHubPAT:                    "ghp_token",
		UseGitHubStorage:             true,
		AzureStorageConnectionString: "azure-conn-string",
		AWSBucketName:                "my-bucket",
		AWSRegion:                    "us-east-1",
		AWSAccessKey:                 "AKIAIOSFODNN7EXAMPLE",
		AWSSecretKey:                 "secret-key",
		AWSSessionToken:              "session-token",
		TargetRepoVisibility:         RepoVisibility("private"),
		KeepArchive:                  true,
	}

	assert.Equal(t, "github-org", flags.TargetOrg)
	assert.Equal(t, "github-repo", flags.TargetRepo)
	assert.Equal(t, "ghp_token", flags.GitHubPAT)
	assert.True(t, flags.UseGitHubStorage)
	assert.Equal(t, "azure-conn-string", flags.AzureStorageConnectionString)
	assert.Equal(t, "my-bucket", flags.AWSBucketName)
	assert.Equal(t, "us-east-1", flags.AWSRegion)
	assert.Equal(t, "AKIAIOSFODNN7EXAMPLE", flags.AWSAccessKey)
	assert.Equal(t, "secret-key", flags.AWSSecretKey)
	assert.Equal(t, "session-token", flags.AWSSessionToken)
	assert.Equal(t, RepoVisibility("private"), flags.TargetRepoVisibility)
	assert.True(t, flags.KeepArchive)
}

func TestCmdMigrateFlagsJSON(t *testing.T) {
	flags := CmdMigrateFlags{
		TargetOrg:            "github-org",
		TargetRepo:           "github-repo",
		TargetRepoVisibility: RepoVisibility("private"),
		KeepArchive:          true,
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(flags)
	assert.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	// Test JSON unmarshaling
	var unmarshaledFlags CmdMigrateFlags
	err = json.Unmarshal(jsonData, &unmarshaledFlags)
	assert.NoError(t, err)

	assert.Equal(t, flags.TargetOrg, unmarshaledFlags.TargetOrg)
	assert.Equal(t, flags.TargetRepo, unmarshaledFlags.TargetRepo)
	assert.Equal(t, flags.KeepArchive, unmarshaledFlags.KeepArchive)
}

func TestRepoVisibilityAllValidValues(t *testing.T) {
	validValues := []string{"public", "private", "internal", ""}

	for _, value := range validValues {
		t.Run("valid_"+value, func(t *testing.T) {
			var rv RepoVisibility
			err := rv.Set(value)
			assert.NoError(t, err)
			assert.Equal(t, value, rv.String())
		})
	}
}

func TestRepoVisibilityInvalidValues(t *testing.T) {
	invalidValues := []string{
		"PUBLIC",
		"PRIVATE",
		"INTERNAL",
		"Protected",
		"secret",
		"restricted",
		"open",
		"closed",
		"123",
		"true",
		"false",
		" public",
		"public ",
		" private ",
	}

	for _, value := range invalidValues {
		t.Run("invalid_"+value, func(t *testing.T) {
			var rv RepoVisibility
			err := rv.Set(value)
			assert.Error(t, err)
		})
	}
}

func TestCmdMigrateFlagsStorageProviderCombinations(t *testing.T) {
	testCases := []struct {
		name                  string
		useGitHubStorage      bool
		azureConnectionString string
		awsBucketName         string
		description           string
	}{
		{
			name:                  "GitHub storage only",
			useGitHubStorage:      true,
			azureConnectionString: "",
			awsBucketName:         "",
			description:           "Using GitHub as storage provider",
		},
		{
			name:                  "Azure storage only",
			useGitHubStorage:      false,
			azureConnectionString: "DefaultEndpointsProtocol=https;AccountName=test",
			awsBucketName:         "",
			description:           "Using Azure as storage provider",
		},
		{
			name:                  "AWS storage only",
			useGitHubStorage:      false,
			azureConnectionString: "",
			awsBucketName:         "my-s3-bucket",
			description:           "Using AWS S3 as storage provider",
		},
		{
			name:                  "No storage provider",
			useGitHubStorage:      false,
			azureConnectionString: "",
			awsBucketName:         "",
			description:           "No external storage configured",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			flags := CmdMigrateFlags{
				UseGitHubStorage:             tc.useGitHubStorage,
				AzureStorageConnectionString: tc.azureConnectionString,
				AWSBucketName:                tc.awsBucketName,
			}

			assert.Equal(t, tc.useGitHubStorage, flags.UseGitHubStorage)
			assert.Equal(t, tc.azureConnectionString, flags.AzureStorageConnectionString)
			assert.Equal(t, tc.awsBucketName, flags.AWSBucketName)
		})
	}
}

func TestCmdMigrateFlagsAWSCredentialsCombinations(t *testing.T) {
	testCases := []struct {
		name         string
		bucketName   string
		region       string
		accessKey    string
		secretKey    string
		sessionToken string
		description  string
	}{
		{
			name:         "Full credentials with session token",
			bucketName:   "my-bucket",
			region:       "us-east-1",
			accessKey:    "AKIAIOSFODNN7EXAMPLE",
			secretKey:    "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			sessionToken: "FwoGZXIvYXdzEBY...",
			description:  "All AWS credentials including session token",
		},
		{
			name:         "Credentials without session token",
			bucketName:   "my-bucket",
			region:       "eu-west-1",
			accessKey:    "AKIAIOSFODNN7EXAMPLE",
			secretKey:    "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			sessionToken: "",
			description:  "AWS credentials without session token",
		},
		{
			name:         "IAM role based (no credentials)",
			bucketName:   "my-bucket",
			region:       "ap-southeast-1",
			accessKey:    "",
			secretKey:    "",
			sessionToken: "",
			description:  "Using IAM role, no explicit credentials",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			flags := CmdMigrateFlags{
				AWSBucketName:   tc.bucketName,
				AWSRegion:       tc.region,
				AWSAccessKey:    tc.accessKey,
				AWSSecretKey:    tc.secretKey,
				AWSSessionToken: tc.sessionToken,
			}

			assert.Equal(t, tc.bucketName, flags.AWSBucketName)
			assert.Equal(t, tc.region, flags.AWSRegion)
			assert.Equal(t, tc.accessKey, flags.AWSAccessKey)
			assert.Equal(t, tc.secretKey, flags.AWSSecretKey)
			assert.Equal(t, tc.sessionToken, flags.AWSSessionToken)
		})
	}
}

func TestCmdMigrateFlagsTargetRepoInheritance(t *testing.T) {
	// Test case where target repo should default to source repo
	flags := CmdMigrateFlags{
		TargetOrg:  "target-org",
		TargetRepo: "",
	}

	assert.Empty(t, flags.TargetRepo)

	// Test case with explicit target repo
	flagsWithTarget := CmdMigrateFlags{
		TargetOrg:  "target-org",
		TargetRepo: "different-target-repo",
	}

	assert.Equal(t, "different-target-repo", flagsWithTarget.TargetRepo)
}
