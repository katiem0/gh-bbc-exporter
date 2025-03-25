package data

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBitbucketRepositoryJSON(t *testing.T) {
	repo := BitbucketRepository{
		Name: "test-repo",
		MainBranch: &struct {
			Name string `json:"name"`
			Type string `json:"type"`
		}{
			Name: "main",
			Type: "branch",
		},
	}
	jsonData, err := json.Marshal(repo)
	assert.NoError(t, err)

	var unmarshaledRepo BitbucketRepository
	err = json.Unmarshal(jsonData, &unmarshaledRepo)
	assert.NoError(t, err)

	assert.Equal(t, repo, unmarshaledRepo)
}

func TestBitbucketPRResponseJSON(t *testing.T) {
	response := BitbucketPRResponse{
		Values: []BitbucketPR{
			{
				ID:    1,
				Title: "Test PR",
				State: "MERGED",
				Author: BitbucketPRUser{
					UUID: "{test-uuid}",
				},
				Source: BitbucketPREndpoint{
					Branch: struct {
						Name string `json:"name"`
					}{
						Name: "feature",
					},
					Commit: struct {
						Hash string `json:"hash"`
					}{
						Hash: "abcdef",
					},
				},
				Destination: BitbucketPREndpoint{
					Branch: struct {
						Name string `json:"name"`
					}{
						Name: "main",
					},
					Commit: struct {
						Hash string `json:"hash"`
					}{
						Hash: "123456",
					},
				},
			},
		},
		Next: "",
	}

	jsonData, err := json.Marshal(response)
	assert.NoError(t, err)

	var unmarshaledResponse BitbucketPRResponse
	err = json.Unmarshal(jsonData, &unmarshaledResponse)
	assert.NoError(t, err)

	assert.Len(t, unmarshaledResponse.Values, 1)
	assert.Equal(t, 1, unmarshaledResponse.Values[0].ID)
	assert.Equal(t, "Test PR", unmarshaledResponse.Values[0].Title)
}

func TestBitbucketUserResponseJSON(t *testing.T) {
	response := BitbucketUserResponse{
		Values: []struct {
			User struct {
				AccountID   string `json:"account_id"`
				DisplayName string `json:"display_name"`
				Nickname    string `json:"nickname"`
				UUID        string `json:"uuid"`
				Links       struct {
					Self struct {
						Href string `json:"href"`
					} `json:"self"`
					HTML struct {
						Href string `json:"href"`
					} `json:"html"`
				} `json:"links"`
			} `json:"user"`
			Workspace struct {
				Slug string `json:"slug"`
				Name string `json:"name"`
			} `json:"workspace"`
		}{
			{
				User: struct {
					AccountID   string `json:"account_id"`
					DisplayName string `json:"display_name"`
					Nickname    string `json:"nickname"`
					UUID        string `json:"uuid"`
					Links       struct {
						Self struct {
							Href string `json:"href"`
						} `json:"self"`
						HTML struct {
							Href string `json:"href"`
						} `json:"html"`
					} `json:"links"`
				}{
					AccountID:   "123",
					DisplayName: "Test User",
					Nickname:    "testuser",
					UUID:        "{test-uuid}",
					Links: struct {
						Self struct {
							Href string `json:"href"`
						} `json:"self"`
						HTML struct {
							Href string `json:"href"`
						} `json:"html"`
					}{
						Self: struct {
							Href string `json:"href"`
						}{
							Href: "https://api.bitbucket.org/2.0/users/{test-uuid}",
						},
						HTML: struct {
							Href string `json:"href"`
						}{
							Href: "https://bitbucket.org/{test-uuid}",
						},
					},
				},
				Workspace: struct {
					Slug string `json:"slug"`
					Name string `json:"name"`
				}{
					Slug: "test-workspace",
					Name: "Test Workspace",
				},
			},
		},
		Next: "",
	}

	jsonData, err := json.Marshal(response)
	assert.NoError(t, err)

	var unmarshaledResponse BitbucketUserResponse
	err = json.Unmarshal(jsonData, &unmarshaledResponse)
	assert.NoError(t, err)

	assert.Len(t, unmarshaledResponse.Values, 1)
	assert.Equal(t, "Test User", unmarshaledResponse.Values[0].User.DisplayName)
	assert.Equal(t, "{test-uuid}", unmarshaledResponse.Values[0].User.UUID)
}

func TestBitBucketCommentResponseJSON(t *testing.T) {
	// Create a to pointer for inline comments
	toValue := 10

	response := BitBucketCommentResponse{
		Values: []BitBucketComment{
			{
				ID:        1,
				CreatedOn: "2023-01-01T00:00:00Z",
				UpdatedOn: "2023-01-01T00:00:00Z",
				Content: struct {
					Raw string `json:"raw"`
				}{
					Raw: "Test comment",
				},
				User: BitbucketPRUser{
					UUID: "{test-uuid}",
				},
				Inline: &Inline{
					Path: "test/path.go",
					From: nil,
					To:   &toValue,
				},
			},
		},
		Next: "",
	}

	jsonData, err := json.Marshal(response)
	assert.NoError(t, err)

	var unmarshaledResponse BitBucketCommentResponse
	err = json.Unmarshal(jsonData, &unmarshaledResponse)
	assert.NoError(t, err)

	assert.Len(t, unmarshaledResponse.Values, 1)
	assert.Equal(t, 1, unmarshaledResponse.Values[0].ID)
	assert.Equal(t, "Test comment", unmarshaledResponse.Values[0].Content.Raw)
	assert.Equal(t, "test/path.go", unmarshaledResponse.Values[0].Inline.Path)
	assert.Equal(t, 10, *unmarshaledResponse.Values[0].Inline.To)
}
