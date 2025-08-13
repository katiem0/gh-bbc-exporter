package data

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserJSON(t *testing.T) {
	user := User{
		Type:  "user",
		Login: "testuser",
		Name:  "Test User",
	}

	jsonData, err := json.Marshal(user)
	assert.NoError(t, err)

	var unmarshaledUser User
	err = json.Unmarshal(jsonData, &unmarshaledUser)
	assert.NoError(t, err)

	assert.Equal(t, user, unmarshaledUser)
}

func TestOrganizationJSON(t *testing.T) {
	org := Organization{
		Type:     "organization",
		Login:    "testorg",
		Name:     "Test Org",
		Location: nil,
		Website:  nil,
	}

	jsonData, err := json.Marshal(org)
	assert.NoError(t, err)

	var unmarshaledOrg Organization
	err = json.Unmarshal(jsonData, &unmarshaledOrg)
	assert.NoError(t, err)

	assert.Equal(t, org, unmarshaledOrg)
}

func TestPullRequestJSON(t *testing.T) {
	createdAt := "2023-01-01T00:00:00Z"
	mergedAt := "2023-01-02T00:00:00Z"
	closedAt := "2023-01-02T00:00:00Z"
	mergeCommitSha := "abcdef1234567890"

	pr := PullRequest{
		Type:       "pull_request",
		URL:        "https://example.com/pr/1",
		User:       "https://example.com/user/1",
		Repository: "https://example.com/repo/1",
		Title:      "Test PR",
		Body:       "Test PR body",
		Base: PRBranch{
			Ref:  "main",
			Sha:  "0123456789abcdef",
			User: "https://example.com/user/1",
			Repo: "https://example.com/repo/1",
		},
		Head: PRBranch{
			Ref:  "feature",
			Sha:  "fedcba9876543210",
			User: "https://example.com/user/1",
			Repo: "https://example.com/repo/1",
		},
		Labels:               []string{},
		MergedAt:             &mergedAt,
		ClosedAt:             &closedAt,
		CreatedAt:            createdAt,
		Assignee:             nil,
		Assignees:            []string{},
		Milestone:            nil,
		Reactions:            []string{},
		ReviewRequests:       []string{},
		CloseIssueReferences: []string{},
		WorkInProgress:       false,
		MergeCommitSha:       &mergeCommitSha,
	}

	jsonData, err := json.Marshal(pr)
	assert.NoError(t, err)

	var unmarshaledPR PullRequest
	err = json.Unmarshal(jsonData, &unmarshaledPR)
	assert.NoError(t, err)

	assert.Equal(t, pr, unmarshaledPR)
}

func TestIssueCommentJSON(t *testing.T) {
	comment := IssueComment{
		Type:        "issue_comment",
		URL:         "https://example.com/comment/1",
		User:        "https://example.com/user/1",
		Body:        "Test comment",
		CreatedAt:   "2023-01-01T00:00:00Z",
		Formatter:   "markdown",
		Reactions:   []string{},
		PullRequest: "https://example.com/pr/1",
	}

	jsonData, err := json.Marshal(comment)
	assert.NoError(t, err)

	var unmarshaledComment IssueComment
	err = json.Unmarshal(jsonData, &unmarshaledComment)
	assert.NoError(t, err)

	assert.Equal(t, comment, unmarshaledComment)
}

func TestPullRequestReviewCommentJSON(t *testing.T) {
	comment := PullRequestReviewComment{
		Type:                    "pull_request_review_comment",
		URL:                     "https://example.com/review_comment/1",
		PullRequest:             "https://example.com/pr/1",
		PullRequestReview:       "https://example.com/review/1",
		PullRequestReviewThread: "https://example.com/thread/1",
		User:                    "https://example.com/user/1",
		CommitID:                "0123456789abcdef",
		OriginalCommitId:        "0123456789abcdef",
		Path:                    "test/path.go",
		Position:                10,
		OriginalPosition:        10,
		Body:                    "Test review comment",
		CreatedAt:               "2023-01-01T00:00:00Z",
		UpdatedAt:               "2023-01-01T01:00:00Z",
		Formatter:               "markdown",
		DiffHunk:                "@@ -1,1 +1,1 @@\n Test diff",
		State:                   1,
		InReplyTo:               nil,
		Reactions:               []string{},
		SubjectType:             "line",
	}

	jsonData, err := json.Marshal(comment)
	assert.NoError(t, err)

	var unmarshaledComment PullRequestReviewComment
	err = json.Unmarshal(jsonData, &unmarshaledComment)
	assert.NoError(t, err)

	assert.Equal(t, comment, unmarshaledComment)
}

func TestCmdFlagsDefaults(t *testing.T) {
	flags := CmdFlags{}

	// Test default values
	assert.Empty(t, flags.Workspace)
	assert.Empty(t, flags.Repository)
	assert.Empty(t, flags.OutputDir)
	assert.Empty(t, flags.BitbucketAccessToken)
	assert.Empty(t, flags.BitbucketUser)
	assert.Empty(t, flags.BitbucketAppPass)
	assert.Empty(t, flags.BitbucketAPIURL)
}
