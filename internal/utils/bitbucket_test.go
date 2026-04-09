package utils

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/katiem0/gh-bbc-exporter/internal/data"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// baseDelay is used for rate limiting tests and should match the value in the main code.
var baseDelay = 1 * time.Second

//var maxRetries = 5

// Helper function to safely write to response
func writeResponse(t *testing.T, w http.ResponseWriter, data []byte) {
	_, err := w.Write(data)
	if err != nil {
		t.Fatalf("Failed to write test response: %v", err)
	}
}

func TestNewClient(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := NewClient("https://example.com", "token", "api-token", "email", "user", "pass", logger, "/path/to/export", true)

	assert.NotNil(t, client)
	assert.Equal(t, "https://example.com", client.baseURL)
	assert.Equal(t, "token", client.accessToken)
	assert.Equal(t, "api-token", client.apiToken)
	assert.Equal(t, "email", client.email)
	assert.Equal(t, "user", client.username)
	assert.Equal(t, "pass", client.appPass)
	assert.NotNil(t, client.httpClient)
	assert.NotNil(t, client.logger)
	assert.Equal(t, "/path/to/export", client.exportDir)
	assert.Equal(t, true, client.skipCommitLookup)
}

func TestGetPullRequests(t *testing.T) {
	// Test case 1: Successful request with no pull requests
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		writeResponse(t, w, []byte(`{"values": [], "next": null}`))
	}))
	defer testServer.Close()

	logger, _ := zap.NewDevelopment()
	client := &Client{
		baseURL:        testServer.URL,
		httpClient:     testServer.Client(),
		logger:         logger,
		commitSHACache: make(map[string]string),
	}

	prs, err := client.GetPullRequests("workspace", "repo", false, "")

	assert.NoError(t, err)
	assert.Empty(t, prs)

	// Test case 2: Successful request with pull requests
	testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		writeResponse(t, w, []byte(`{"values": [{"id": 1, "title": "Test PR"}], "next": null}`))
	}))
	defer testServer.Close()

	client.baseURL = testServer.URL
	prs, err = client.GetPullRequests("workspace", "repo", false, "")

	assert.NoError(t, err)
	assert.NotEmpty(t, prs)
	assert.Equal(t, 1, len(prs))
	assert.Equal(t, "Test PR", prs[0].Title)

	// Test case 3: API returns an error
	testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		writeResponse(t, w, []byte(`{"error": "internal server error"}`))
	}))
	defer testServer.Close()

	client.baseURL = testServer.URL
	prs, err = client.GetPullRequests("workspace", "repo", false, "")

	assert.Error(t, err)
	assert.Nil(t, prs)
}

func TestGetPullRequestComments(t *testing.T) {
	// Test case 1: Successful request with no comments
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		writeResponse(t, w, []byte(`{"values": [], "next": null}`))
	}))
	defer testServer.Close()

	logger, _ := zap.NewDevelopment()
	client := &Client{
		baseURL:        testServer.URL,
		httpClient:     testServer.Client(),
		logger:         logger,
		commitSHACache: make(map[string]string),
	}

	prs := []data.PullRequest{{URL: "https://example.com/pr/1", Head: data.PRBranch{SHA: "testsha"}}}
	comments, reviewComments, err := client.GetPullRequestComments("workspace", "repo", prs)

	assert.NoError(t, err)
	assert.Empty(t, comments)
	assert.Empty(t, reviewComments)

	// Test case 2: API returns an error
	testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		writeResponse(t, w, []byte(`{"error": "internal server error"}`))
	}))
	defer testServer.Close()

	client.baseURL = testServer.URL
	comments, reviewComments, err = client.GetPullRequestComments("workspace", "repo", prs)

	assert.NoError(t, err)
	assert.Empty(t, comments)
	assert.Empty(t, reviewComments)
}

func TestGetUsers(t *testing.T) {
	// Test case 1: Successful request with no users
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		writeResponse(t, w, []byte(`{"values": [], "next": null}`))
	}))
	defer testServer.Close()

	logger, _ := zap.NewDevelopment()
	client := &Client{
		baseURL:        testServer.URL,
		httpClient:     testServer.Client(),
		logger:         logger,
		commitSHACache: make(map[string]string),
	}

	users, err := client.GetUsers("workspace", "repo")

	assert.NoError(t, err)
	assert.Len(t, users, 1) // Expecting fallback user

	// Test case 2: Successful request with users
	testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		writeResponse(t, w, []byte(`{"values": [{"user": {"display_name": "Test User", "uuid": "{test-uuid}"}}], "next": null}`))
	}))
	defer testServer.Close()

	client.baseURL = testServer.URL
	users, err = client.GetUsers("workspace", "repo")

	assert.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, "Test User", users[0].Name)

	// Test case 3: API returns an error
	testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		writeResponse(t, w, []byte(`{"error": "internal server error"}`))
	}))
	defer testServer.Close()

	client.baseURL = testServer.URL
	users, err = client.GetUsers("workspace", "repo")

	assert.NoError(t, err) // Expecting fallback user
	assert.Len(t, users, 1)
}

func TestGetRepository(t *testing.T) {
	// Test case 1: Successful request
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		writeResponse(t, w, []byte(`{"name": "Test Repo", "mainbranch": {"name": "main"}}`))
	}))
	defer testServer.Close()

	logger, _ := zap.NewDevelopment()
	client := &Client{
		baseURL:    testServer.URL,
		httpClient: testServer.Client(),
		logger:     logger,
	}

	repo, err := client.GetRepository("workspace", "repo")

	assert.NoError(t, err)
	assert.NotNil(t, repo)
	assert.Equal(t, "Test Repo", repo.Name)
	assert.Equal(t, "main", repo.MainBranch.Name)

	// Test case 2: API returns an error
	testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		writeResponse(t, w, []byte(`{"error": "internal server error"}`))
	}))
	defer testServer.Close()

	client.baseURL = testServer.URL
	repo, err = client.GetRepository("workspace", "repo")

	assert.Error(t, err)
	assert.Nil(t, repo)
}

func TestGetPullRequestsWithStateFilter(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Test with openPRsOnly = true
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if this is a request for pull requests (not for commit SHA)
		if strings.Contains(r.URL.String(), "/pullrequests") {
			// Verify the endpoint contains state=OPEN
			if !strings.Contains(r.URL.String(), "state=OPEN") {
				t.Errorf("Expected URL to contain state=OPEN when openPRsOnly is true, but got: %s", r.URL.String())
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
			writeResponse(t, w, []byte(`{
                "values": [
                    {
                        "id": 1, 
                        "title": "Open PR", 
                        "state": "OPEN",
                        "author": {"uuid": "{test-uuid}"},
                        "source": {"branch": {"name": "feature"}, "commit": {"hash": "1234567890123456789012345678901234567890"}},
                        "destination": {"branch": {"name": "main"}, "commit": {"hash": "0987654321098765432109876543210987654321"}}
                    }
                ], 
                "next": null
            }`))
		} else {
			// For any other requests like commit SHA lookups, return success
			w.WriteHeader(http.StatusOK)
			writeResponse(t, w, []byte(`{"hash": "1234567890123456789012345678901234567890"}`))
		}
	}))
	defer testServer.Close()

	client := &Client{
		baseURL:        testServer.URL,
		httpClient:     testServer.Client(),
		logger:         logger,
		commitSHACache: make(map[string]string),
	}

	prs, err := client.GetPullRequests("workspace", "repo", true, "")
	assert.NoError(t, err)
	assert.Len(t, prs, 1)
	assert.Equal(t, "Open PR", prs[0].Title)

	// Test with openPRsOnly = false
	testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if this is a request for pull requests (not for commit SHA)
		if strings.Contains(r.URL.String(), "/pullrequests") {
			// Verify the endpoint contains state=ALL
			if !strings.Contains(r.URL.String(), "state=ALL") {
				t.Errorf("Expected URL to contain state=ALL when openPRsOnly is false, but got: %s", r.URL.String())
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
			writeResponse(t, w, []byte(`{
                "values": [
                    {
                        "id": 1, 
                        "title": "Any PR", 
                        "state": "OPEN",
                        "author": {"uuid": "{test-uuid}"},
                        "source": {"branch": {"name": "feature"}, "commit": {"hash": "1234567890123456789012345678901234567890"}},
                        "destination": {"branch": {"name": "main"}, "commit": {"hash": "0987654321098765432109876543210987654321"}}
                    }, 
                    {
                        "id": 2, 
                        "title": "Closed PR", 
                        "state": "DECLINED",
                        "author": {"uuid": "{test-uuid}"},
                        "source": {"branch": {"name": "bugfix"}, "commit": {"hash": "abcdef1234567890abcdef1234567890abcdef12"}},
                        "destination": {"branch": {"name": "main"}, "commit": {"hash": "fedcba0987654321fedcba0987654321fedcba09"}}
                    }
                ], 
                "next": null
            }`))
		} else {
			// For any other requests like commit SHA lookups, return success
			w.WriteHeader(http.StatusOK)
			writeResponse(t, w, []byte(`{"hash": "1234567890123456789012345678901234567890"}`))
		}
	}))
	defer testServer.Close()

	client.baseURL = testServer.URL
	prs, err = client.GetPullRequests("workspace", "repo", false, "")
	assert.NoError(t, err)
	assert.Len(t, prs, 2)
}

func TestGetPullRequestsWithDateFilter(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return sample PRs with different dates
		w.WriteHeader(http.StatusOK)
		writeResponse(t, w, []byte(`{
            "values": [
                {
                    "id": 1, 
                    "title": "Old PR",
                    "state": "OPEN",
                    "created_on": "2022-01-01T00:00:00+00:00",
                    "author": {"uuid": "{123}"},
                    "source": {"branch": {"name": "source-branch"}, "commit": {"hash": "abc123"}},
                    "destination": {"branch": {"name": "main"}, "commit": {"hash": "def456"}}
                },
                {
                    "id": 2, 
                    "title": "New PR",
                    "state": "OPEN",
                    "created_on": "2023-06-01T00:00:00+00:00",
                    "author": {"uuid": "{123}"},
                    "source": {"branch": {"name": "feature"}, "commit": {"hash": "abc123"}},
                    "destination": {"branch": {"name": "main"}, "commit": {"hash": "def456"}}
                }
            ],
            "next": null
        }`))
	}))
	defer testServer.Close()

	client := &Client{
		baseURL:        testServer.URL,
		httpClient:     testServer.Client(),
		logger:         logger,
		commitSHACache: make(map[string]string),
	}

	// Test with date filter
	prs, err := client.GetPullRequests("workspace", "repo", false, "2023-01-01")
	assert.NoError(t, err)
	assert.Len(t, prs, 1)
	assert.Equal(t, "New PR", prs[0].Title)
}

func TestDraftPRHandling(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Create a test server that returns a draft PR
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.String(), "/pullrequests") {
			w.WriteHeader(http.StatusOK)
			writeResponse(t, w, []byte(`{
                "values": [
                    {
                        "id": 1, 
                        "title": "Draft PR", 
                        "state": "OPEN",
                        "draft": true,
                        "author": {"uuid": "{test-uuid}"},
                        "source": {"branch": {"name": "feature"}, "commit": {"hash": "1234567890123456789012345678901234567890"}},
                        "destination": {"branch": {"name": "main"}, "commit": {"hash": "0987654321098765432109876543210987654321"}}
                    },
                    {
                        "id": 2, 
                        "title": "Regular PR", 
                        "state": "OPEN",
                        "draft": false,
                        "author": {"uuid": "{test-uuid}"},
                        "source": {"branch": {"name": "bugfix"}, "commit": {"hash": "abcdef1234567890abcdef1234567890abcdef12"}},
                        "destination": {"branch": {"name": "main"}, "commit": {"hash": "fedcba0987654321fedcba0987654321fedcba09"}}
                    }
                ], 
                "next": null
            }`))
		} else {
			// For commit SHA lookups
			w.WriteHeader(http.StatusOK)
			writeResponse(t, w, []byte(`{"hash": "1234567890123456789012345678901234567890"}`))
		}
	}))
	defer testServer.Close()

	client := &Client{
		baseURL:        testServer.URL,
		httpClient:     testServer.Client(),
		logger:         logger,
		commitSHACache: make(map[string]string),
	}

	prs, err := client.GetPullRequests("workspace", "repo", false, "")
	assert.NoError(t, err)
	assert.Len(t, prs, 2)

	// Check that the draft PR has work_in_progress set to true
	assert.True(t, prs[0].WorkInProgress, "Expected draft PR to have WorkInProgress set to true")
	assert.Equal(t, "Draft PR", prs[0].Title, "Expected title to match")

	// Check that the regular PR has work_in_progress set to false
	assert.False(t, prs[1].WorkInProgress, "Expected non-draft PR to have WorkInProgress set to false")
	assert.Equal(t, "Regular PR", prs[1].Title, "Expected title to match")
}

func TestExportNonExistentRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "exporter-test-")
	assert.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: Failed to remove temp dir: %v", err)
		}
	}()

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return 404 for repository
		if strings.Contains(r.URL.Path, "/repositories/") {
			w.WriteHeader(http.StatusNotFound)
			writeResponse(t, w, []byte(`{"error": "Repository not found"}`))
		} else {
			w.WriteHeader(http.StatusOK)
			writeResponse(t, w, []byte(`{}`))
		}
	}))
	defer testServer.Close()

	logger, _ := zap.NewDevelopment()
	client := &Client{
		baseURL:        testServer.URL,
		httpClient:     testServer.Client(),
		logger:         logger,
		commitSHACache: make(map[string]string),
	}
	exporter := NewExporter(client, tempDir, logger, false, "")

	err = exporter.Export("workspace", "non-existent-repo")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Repository not found")
}

func TestGetPullRequestsWithComprehensiveFilters(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Create a test server that checks the query parameters and returns filtered data
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse the URL to check query parameters
		isOpenOnly := strings.Contains(r.URL.String(), "state=OPEN")
		hasDateFilter := r.URL.Query().Get("prsFromDate") == "2023-01-01" || strings.Contains(r.URL.RawQuery, "prsFromDate=2023-01-01")

		// Base set of all PRs
		allPRs := `{
            "values": [
                {
                    "id": 1, 
                    "title": "Old Open PR",
                    "state": "OPEN",
                    "created_on": "2022-01-01T00:00:00+00:00",
                    "author": {"uuid": "{123}"},
                    "source": {"branch": {"name": "source-branch"}, "commit": {"hash": "abc123"}},
                    "destination": {"branch": {"name": "main"}, "commit": {"hash": "def456"}}
                },
                {
                    "id": 2, 
                    "title": "Old Closed PR",
                    "state": "DECLINED",
                    "created_on": "2022-03-01T00:00:00+00:00",
                    "author": {"uuid": "{123}"},
                    "source": {"branch": {"name": "source-branch"}, "commit": {"hash": "abc123"}},
                    "destination": {"branch": {"name": "main"}, "commit": {"hash": "def456"}}
                },
                {
                    "id": 3, 
                    "title": "New Open PR",
                    "state": "OPEN",
                    "created_on": "2023-06-01T00:00:00+00:00",
                    "author": {"uuid": "{123}"},
                    "source": {"branch": {"name": "feature"}, "commit": {"hash": "abc123"}},
                    "destination": {"branch": {"name": "main"}, "commit": {"hash": "def456"}}
                },
                {
                    "id": 4,
                    "title": "New Merged PR",
                    "state": "MERGED",
                    "created_on": "2023-07-01T00:00:00+00:00",
                    "author": {"uuid": "{123}"},
                    "source": {"branch": {"name": "bugfix"}, "commit": {"hash": "abc123"}},
                    "destination": {"branch": {"name": "main"}, "commit": {"hash": "def456"}}
                }
            ],
            "next": null
        }`

		// Only open PRs
		openPRs := `{
            "values": [
                {
                    "id": 1, 
                    "title": "Old Open PR",
                    "state": "OPEN",
                    "created_on": "2022-01-01T00:00:00+00:00",
                    "author": {"uuid": "{123}"},
                    "source": {"branch": {"name": "source-branch"}, "commit": {"hash": "abc123"}},
                    "destination": {"branch": {"name": "main"}, "commit": {"hash": "def456"}}
                },
                {
                    "id": 3, 
                    "title": "New Open PR",
                    "state": "OPEN",
                    "created_on": "2023-06-01T00:00:00+00:00",
                    "author": {"uuid": "{123}"},
                    "source": {"branch": {"name": "feature"}, "commit": {"hash": "abc123"}},
                    "destination": {"branch": {"name": "main"}, "commit": {"hash": "def456"}}
                }
            ],
            "next": null
        }`

		// PRs from 2023
		prsFrom2023 := `{
            "values": [
                {
                    "id": 3, 
                    "title": "New Open PR",
                    "state": "OPEN",
                    "created_on": "2023-06-01T00:00:00+00:00",
                    "author": {"uuid": "{123}"},
                    "source": {"branch": {"name": "feature"}, "commit": {"hash": "abc123"}},
                    "destination": {"branch": {"name": "main"}, "commit": {"hash": "def456"}}
                },
                {
                    "id": 4,
                    "title": "New Merged PR",
                    "state": "MERGED",
                    "created_on": "2023-07-01T00:00:00+00:00",
                    "author": {"uuid": "{123}"},
                    "source": {"branch": {"name": "bugfix"}, "commit": {"hash": "abc123"}},
                    "destination": {"branch": {"name": "main"}, "commit": {"hash": "def456"}}
                }
            ],
            "next": null
        }`

		// Open PRs from 2023
		openPRsFrom2023 := `{
            "values": [
                {
                    "id": 3, 
                    "title": "New Open PR",
                    "state": "OPEN",
                    "created_on": "2023-06-01T00:00:00+00:00",
                    "author": {"uuid": "{123}"},
                    "source": {"branch": {"name": "feature"}, "commit": {"hash": "abc123"}},
                    "destination": {"branch": {"name": "main"}, "commit": {"hash": "def456"}}
                }
            ],
            "next": null
        }`

		w.WriteHeader(http.StatusOK)

		// Return appropriate response based on filter combination
		if isOpenOnly && hasDateFilter {
			// Open PRs from 2023
			writeResponse(t, w, []byte(openPRsFrom2023))
		} else if isOpenOnly {
			// Only open PRs
			writeResponse(t, w, []byte(openPRs))
		} else if hasDateFilter {
			// PRs from 2023
			writeResponse(t, w, []byte(prsFrom2023))
		} else {
			// All PRs
			writeResponse(t, w, []byte(allPRs))
		}
	}))
	defer testServer.Close()

	client := &Client{
		baseURL:        testServer.URL,
		httpClient:     testServer.Client(),
		logger:         logger,
		commitSHACache: make(map[string]string),
	}

	// Test 1: No filters - should return all PRs
	prs, err := client.GetPullRequests("workspace", "repo", false, "")
	assert.NoError(t, err)
	assert.Len(t, prs, 4, "Expected all 4 PRs with no filters")

	// Test 2: Open PRs only
	prs, err = client.GetPullRequests("workspace", "repo", true, "")
	assert.NoError(t, err)
	assert.Len(t, prs, 2, "Expected 2 open PRs")
	var titles []string
	for _, pr := range prs {
		titles = append(titles, pr.Title)
	}
	assert.Contains(t, titles, "Old Open PR")
	assert.Contains(t, titles, "New Open PR")

	// Test 3: Date filter only - PRs from 2023
	prs, err = client.GetPullRequests("workspace", "repo", false, "2023-01-01")
	assert.NoError(t, err)
	assert.Len(t, prs, 2, "Expected 2 PRs from 2023")
	titles = []string{}
	for _, pr := range prs {
		titles = append(titles, pr.Title)
	}
	assert.Contains(t, titles, "New Open PR")
	assert.Contains(t, titles, "New Merged PR")

	// Test 4: Combined filters - Open PRs from 2023
	prs, err = client.GetPullRequests("workspace", "repo", true, "2023-01-01")
	assert.NoError(t, err)
	assert.Len(t, prs, 1, "Expected 1 open PR from 2023")
	assert.Equal(t, "New Open PR", prs[0].Title)
}

func TestGetPullRequestCommentsWithThreads(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Create a test server that returns multiple comments on the same line/file
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "comments") {
			w.WriteHeader(http.StatusOK)
			// Return two comments on the same file/line (one parent, one reply)
			writeResponse(t, w, []byte(`{
                "values": [
                    {
                        "id": 123,
                        "created_on": "2023-01-01T12:00:00Z",
                        "updated_on": "2023-01-01T12:00:00Z",
                        "content": {"raw": "This is a parent comment"},
                        "user": {"uuid": "{parent-uuid}"},
                        "inline": {
                            "path": "test/file.txt",
                            "to": 10
                        }
                    },
                    {
                        "id": 456,
                        "created_on": "2023-01-02T12:00:00Z",
                        "updated_on": "2023-01-02T12:00:00Z",
                        "content": {"raw": "This is a reply"},
                        "user": {"uuid": "{reply-uuid}"},
                        "parent": {"id": 123},
                        "inline": {
                            "path": "test/file.txt",
                            "to": 10
                        }
                    }
                ],
                "next": null
            }`))
		} else if strings.Contains(r.URL.Path, "diff") {
			// Return a valid unified diff for test/file.txt where line 10 is modified
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "text/plain")
			diff := "diff --git a/test/file.txt b/test/file.txt\n" +
				"index abc1234..def5678 100644\n" +
				"--- a/test/file.txt\n" +
				"+++ b/test/file.txt\n" +
				"@@ -1,12 +1,12 @@\n" +
				" line 1\n" +
				" line 2\n" +
				" line 3\n" +
				" line 4\n" +
				" line 5\n" +
				" line 6\n" +
				" line 7\n" +
				" line 8\n" +
				" line 9\n" +
				"-old line 10\n" +
				"+new line 10\n" +
				" line 11\n" +
				" line 12\n"
			_, _ = w.Write([]byte(diff))
		} else {
			// For commit SHA lookups
			w.WriteHeader(http.StatusOK)
			writeResponse(t, w, []byte(`{"hash": "1234567890123456789012345678901234567890"}`))
		}
	}))
	defer testServer.Close()

	client := &Client{
		baseURL:        testServer.URL,
		httpClient:     testServer.Client(),
		logger:         logger,
		commitSHACache: make(map[string]string),
	}

	// Test with pull requests
	prs := []data.PullRequest{{
		URL:  "https://bitbucket.org/workspace/repo/pull/1",
		Head: data.PRBranch{SHA: "abcdef"},
	}}

	_, reviewComments, err := client.GetPullRequestComments("workspace", "repo", prs)
	assert.NoError(t, err)
	assert.Len(t, reviewComments, 2, "Expected two review comments")

	// Check that they belong to the same thread
	assert.Equal(t, reviewComments[0].PullRequestReviewThread, reviewComments[1].PullRequestReviewThread,
		"Comments should share the same thread ID")

	// Check parent-child relationship
	assert.Nil(t, reviewComments[0].InReplyTo, "First comment should have no parent")
	assert.NotNil(t, reviewComments[1].InReplyTo, "Second comment should have a parent")
	assert.Equal(t, "123", *reviewComments[1].InReplyTo, "Reply should reference parent ID")

	// Check review IDs - Updated to match the actual format used in the code
	expectedReviewURL := formatURL("pr_review", "workspace", "repo", "1", "review-123")
	assert.Equal(t, expectedReviewURL, reviewComments[0].PullRequestReview, "Parent comment should use own ID for review")
	assert.Equal(t, expectedReviewURL, reviewComments[1].PullRequestReview, "Reply should use parent ID for review")
}

func TestTransformCommentBody(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := &Client{
		logger: logger,
	}

	testCases := []struct {
		name      string
		body      string
		workspace string
		repoSlug  string
		expected  string
	}{
		{
			name:      "Empty body",
			body:      "",
			workspace: "workspace",
			repoSlug:  "repo",
			expected:  "",
		},
		{
			name:      "Body with PR reference number",
			body:      "Please see #123 for details",
			workspace: "workspace",
			repoSlug:  "repo",
			expected:  "Please see #123 for details",
		},
		{
			name:      "Body with full PR URL",
			body:      "Check out https://bitbucket.org/workspace/repo/pull-requests/456",
			workspace: "workspace",
			repoSlug:  "repo",
			expected:  "Check out https://bitbucket.org/workspace/repo/pull/456",
		},
		{
			name:      "Multiple references",
			body:      "Fix #123, related to #456 and https://bitbucket.org/workspace/repo/pull-requests/789",
			workspace: "workspace",
			repoSlug:  "repo",
			expected:  "Fix #123, related to #456 and https://bitbucket.org/workspace/repo/pull/789",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := client.transformCommentBody(tc.body, tc.workspace, tc.repoSlug)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestTransformCommentBodyEdgeCases(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := &Client{
		logger: logger,
	}

	testCases := []struct {
		name      string
		body      string
		workspace string
		repoSlug  string
		expected  string
	}{
		{
			name:      "Body with nil input",
			body:      "",
			workspace: "",
			repoSlug:  "",
			expected:  "",
		},
		{
			name:      "Body with URL in code block",
			body:      "```\nhttps://bitbucket.org/workspace/repo/pull-requests/123\n```",
			workspace: "workspace",
			repoSlug:  "repo",
			expected:  "```\nhttps://bitbucket.org/workspace/repo/pull/123\n```",
		},
		{
			name:      "Body with multiple URL formats",
			body:      "PR at /workspace/repo/pull-requests/123 and bitbucket.org/workspace/repo/pull-requests/456",
			workspace: "workspace",
			repoSlug:  "repo",
			expected:  "PR at /workspace/repo/pull-requests/123 and bitbucket.org/workspace/repo/pull-requests/456",
		},
		{
			name:      "Body with URL for different repository",
			body:      "See https://bitbucket.org/other-workspace/other-repo/pull-requests/123",
			workspace: "workspace",
			repoSlug:  "repo",
			expected:  "See https://bitbucket.org/other-workspace/other-repo/pull-requests/123",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := client.transformCommentBody(tc.body, tc.workspace, tc.repoSlug)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestMakeRequestWithRateLimiting(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Create a counter to track request attempts
	requestCount := 0

	// Set up a test server that returns rate limit errors for the first few attempts
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		// Return rate limit for first two attempts, then succeed
		if requestCount <= 2 {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Limit", "100")
			w.WriteHeader(http.StatusTooManyRequests) // 429
			writeResponse(t, w, []byte(`{"error": "Too many requests"}`))
		} else {
			w.WriteHeader(http.StatusOK)
			writeResponse(t, w, []byte(`{"success": true}`))
		}
	}))
	defer testServer.Close()

	// Reduce the base delay for faster test execution
	originalBaseDelay := baseDelay // Store original if it's a package variable
	baseDelay = 10 * time.Millisecond
	defer func() {
		baseDelay = originalBaseDelay // Restore after test
	}()

	client := &Client{
		baseURL:    testServer.URL,
		httpClient: testServer.Client(),
		logger:     logger,
	}

	var result struct {
		Success bool `json:"success"`
	}

	startTime := time.Now()
	err := client.makeRequest("GET", "/test-endpoint", &result)
	duration := time.Since(startTime)

	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 3, requestCount, "Expected 3 request attempts")
	assert.GreaterOrEqual(t, duration, 30*time.Millisecond, "Expected some backoff delay")
}

func TestMalformedJSONResponse(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Return incomplete JSON to trigger a JSON parsing error
		w.Header().Set("Content-Type", "application/json")
		writeResponse(t, w, []byte(`{"name": "test-repo", "malformed`))
	}))
	defer testServer.Close()

	client := &Client{
		baseURL:        testServer.URL,
		httpClient:     testServer.Client(),
		logger:         logger,
		commitSHACache: make(map[string]string),
	}

	_, err := client.GetRepository("workspace", "repo")
	assert.Error(t, err)
	// Check for any JSON-related error message
	assert.Contains(t, err.Error(), "EOF")
}

func TestGetFullCommitSHAFastPaths(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Create a test server that should never be called
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This should not be called for the fast paths we're testing
		t.Error("HTTP request was made but should have been avoided")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer testServer.Close()

	client := &Client{
		baseURL:        testServer.URL,
		httpClient:     testServer.Client(),
		logger:         logger,
		commitSHACache: make(map[string]string),
	}

	// 1) Full SHA passthrough (no network)
	full := strings.Repeat("a", 40)
	got, err := client.GetFullCommitSHA("ws", "repo", full)
	assert.NoError(t, err)
	assert.Equal(t, full, got)

	// 2) Cache hit (no network)
	client.commitSHACache["abc123"] = strings.Repeat("b", 40)
	got, err = client.GetFullCommitSHA("ws", "repo", "abc123")
	assert.NoError(t, err)
	assert.Equal(t, strings.Repeat("b", 40), got)
}

func TestGetFullCommitSHAWithAPICall(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return full SHA for short SHA
		w.WriteHeader(http.StatusOK)
		writeResponse(t, w, []byte(`{"hash": "1234567890abcdef1234567890abcdef12345678"}`))
	}))
	defer testServer.Close()

	client := &Client{
		baseURL:        testServer.URL,
		httpClient:     testServer.Client(),
		logger:         logger,
		accessToken:    "test-token",
		commitSHACache: make(map[string]string),
	}

	// Test with short SHA that needs API call
	shortSHA := "abc123"
	fullSHA, err := client.GetFullCommitSHA("workspace", "repo", shortSHA)

	assert.NoError(t, err)
	assert.Equal(t, "1234567890abcdef1234567890abcdef12345678", fullSHA)

	// Verify it was cached
	assert.Equal(t, fullSHA, client.commitSHACache[shortSHA])

	// Second call should use cache (server won't be called)
	cachedSHA, err := client.GetFullCommitSHA("workspace", "repo", shortSHA)
	assert.NoError(t, err)
	assert.Equal(t, fullSHA, cachedSHA)
}

func TestMakeRequestWithNoAuth(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify no auth header is present
		assert.Empty(t, r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		writeResponse(t, w, []byte(`{"success": true}`))
	}))
	defer testServer.Close()

	client := &Client{
		baseURL:    testServer.URL,
		httpClient: testServer.Client(),
		logger:     logger,
		// No auth credentials
	}

	var result map[string]interface{}
	err := client.makeRequest("GET", "/test", &result)

	assert.NoError(t, err)
	assert.True(t, result["success"].(bool))
}

func TestGetPullRequestCommentsConcurrent(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	requestCount := 0
	mu := sync.Mutex{}

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		// Reduce processing time to make test more reliable
		time.Sleep(50 * time.Millisecond)

		w.WriteHeader(http.StatusOK)
		writeResponse(t, w, []byte(`{
            "values": [
                {
                    "id": 1,
                    "content": {"raw": "Test comment"},
                    "created_on": "2023-01-01T12:00:00Z",
                    "updated_on": "2023-01-01T12:00:00Z",
                    "user": {"uuid": "{test-uuid}"}
                }
            ],
            "next": null
        }`))
	}))
	defer testServer.Close()

	client := &Client{
		baseURL:        testServer.URL,
		httpClient:     testServer.Client(),
		logger:         logger,
		commitSHACache: make(map[string]string),
	}

	// Create multiple PRs to test
	prs := []data.PullRequest{}
	for i := 1; i <= 5; i++ {
		prs = append(prs, data.PullRequest{
			URL:  fmt.Sprintf("https://bitbucket.org/workspace/repo/pull/%d", i),
			Head: data.PRBranch{SHA: "abc123"},
		})
	}

	start := time.Now()
	regularComments, reviewComments, err := client.GetPullRequestComments("workspace", "repo", prs)
	elapsed := time.Since(start)

	assert.NoError(t, err)
	// Each PR now makes 2 requests: one for the diff and one for comments
	assert.Equal(t, 10, requestCount, "Should make two requests per PR (diff + comments)")
	assert.Len(t, regularComments, 5, "Should have one comment per PR")
	assert.Empty(t, reviewComments, "Should have no review comments")

	// Diffs are fetched in parallel; comments are sequential (5 * 50ms = 250ms minimum)
	assert.GreaterOrEqual(t, elapsed, 250*time.Millisecond, "Should take at least 5*50ms for sequential comment fetching")

	// Log the actual time for debugging
	t.Logf("Fetching comments for %d PRs took %v", len(prs), elapsed)
}

func TestExportUpdatesClientExportDir(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "export-dir-test-")
	assert.NoError(t, err)
	defer func() {
		// Clean up the temp directory
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: Failed to remove temp dir: %v", err)
		}

		// Clean up any bitbucket-export-* directories created by the test
		matches, _ := filepath.Glob("./bitbucket-export-*")
		for _, match := range matches {
			// Remove directory
			if err := os.RemoveAll(match); err != nil {
				t.Logf("Warning: Failed to remove %s: %v", match, err)
			}
			// Remove corresponding archive if it exists
			archivePath := match + ".tar.gz"
			if _, err := os.Stat(archivePath); err == nil {
				if err := os.Remove(archivePath); err != nil {
					t.Logf("Warning: Failed to remove archive %s: %v", archivePath, err)
				}
			}
		}
	}()

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if strings.Contains(r.URL.Path, "/repositories/") {
			writeResponse(t, w, []byte(`{"name": "Test Repo", "mainbranch": {"name": "main"}}`))
		} else {
			writeResponse(t, w, []byte(`{"values": [], "next": null}`))
		}
	}))
	defer testServer.Close()

	logger, _ := zap.NewDevelopment()
	client := &Client{
		baseURL:        testServer.URL,
		httpClient:     testServer.Client(),
		logger:         logger,
		commitSHACache: make(map[string]string),
		exportDir:      "", // Start with empty exportDir
		// Don't set any auth credentials to avoid actual clone attempts
	}

	// Test with auto-generated output dir
	exporter := NewExporter(client, "", logger, false, "")

	// Before Export, client.exportDir should be empty
	assert.Empty(t, client.exportDir)

	// The Export will fail due to authentication error during clone, but
	// it should still set the exportDir before attempting the clone
	err = exporter.Export("workspace", "repo")

	// We expect an error due to clone failure (no valid credentials)
	assert.Error(t, err)

	// Use the isAuthenticationError function to check for various authentication-related errors
	assert.True(t, isAuthenticationError(err),
		"Expected authentication or clone related error, got: %v", err)

	// Even though export failed, the client.exportDir should have been set
	// when the export started (before the clone attempt)
	assert.NotEmpty(t, client.exportDir)
	assert.Equal(t, exporter.outputDir, client.exportDir)
	assert.Contains(t, client.exportDir, "bitbucket-export-")
	assert.NotContains(t, client.exportDir, ".tar.gz")
}

func TestGetFullCommitSHAWithSkipLookup(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("API was called but skipCommitLookup was enabled")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer testServer.Close()

	client := &Client{
		baseURL:          testServer.URL,
		httpClient:       testServer.Client(),
		logger:           logger,
		commitSHACache:   make(map[string]string),
		skipCommitLookup: true,
		exportDir:        "/tmp/test-export",
	}

	// Test 1: Short SHA should be returned as-is
	shortSHA := "abc123"
	result, err := client.GetFullCommitSHA("workspace", "repo", shortSHA)
	assert.NoError(t, err)
	assert.Equal(t, shortSHA, result, "Short SHA should be returned as-is when skip is enabled")

	// Test 2: Full SHA should still be returned unchanged
	fullSHA := strings.Repeat("a", 40)
	result, err = client.GetFullCommitSHA("workspace", "repo", fullSHA)
	assert.NoError(t, err)
	assert.Equal(t, fullSHA, result, "Full SHA should pass through unchanged")

	// Test 3: Verify caching still works
	assert.Equal(t, shortSHA, client.commitSHACache[shortSHA])
}

func TestGetPullRequestCommentsWithShortSHAs(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "comments") {
			w.WriteHeader(http.StatusOK)
			writeResponse(t, w, []byte(`{
                "values": [
                    {
                        "id": 122,
                        "content": {"raw": "Regular comment without inline"},
                        "created_on": "2023-01-01T11:00:00Z",
                        "updated_on": "2023-01-01T11:00:00Z",
                        "user": {"uuid": "{test-uuid}"}
                    },
                    {
                        "id": 123,
                        "content": {"raw": "Review comment with inline"},
                        "created_on": "2023-01-01T12:00:00Z",
                        "updated_on": "2023-01-01T12:00:00Z",
                        "user": {"uuid": "{test-uuid}"},
                        "inline": {
                            "path": "test.txt",
                            "to": 10
                        }
                    }
                ],
                "next": null
            }`))
		} else if strings.Contains(r.URL.Path, "diff") {
			// Return a minimal valid diff so inline comments are not demoted
			w.WriteHeader(http.StatusOK)
			writeResponse(t, w, []byte("diff --git a/test.txt b/test.txt\nindex 0000000..1234567 100644\n--- a/test.txt\n+++ b/test.txt\n@@ -0,0 +1,10 @@\n+line 1\n+line 2\n+line 3\n+line 4\n+line 5\n+line 6\n+line 7\n+line 8\n+line 9\n+line 10\n"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer testServer.Close()

	client := &Client{
		baseURL:          testServer.URL,
		httpClient:       testServer.Client(),
		logger:           logger,
		commitSHACache:   make(map[string]string),
		skipCommitLookup: true,
	}

	prs := []data.PullRequest{
		{
			URL: "https://bitbucket.org/workspace/repo/pull/1",
			Head: data.PRBranch{
				SHA: "abc123",
			},
		},
	}

	regularComments, reviewComments, err := client.GetPullRequestComments("workspace", "repo", prs)
	assert.NoError(t, err)
	assert.Len(t, regularComments, 1, "Should have 1 regular comment")
	assert.Len(t, reviewComments, 1, "Should have 1 review comment")

	for _, comment := range reviewComments {
		if comment.CommitID != "" {
			assert.Equal(t, 6, len(comment.CommitID), "SHA length should remain unchanged when skip is enabled")
			assert.Equal(t, "abc123", comment.CommitID, "Commit ID should match the PR's head SHA")
		}
	}

	for _, comment := range regularComments {
		assert.Equal(t, "https://bitbucket.org/workspace/repo/pull/1", comment.PullRequest)
	}
}

// TestGetPullRequestsSquashMergeHeadSHAFallback verifies that when a squash-merged PR's
// head SHA does not exist as a git object in the local pack (branch was deleted after merge),
// the head SHA is replaced with the merge commit SHA so GEI has a valid object to reference.
func TestGetPullRequestsSquashMergeHeadSHAFallback(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Build the directory structure the production code expects:
	// exportDir/repositories/workspace/repo.git
	exportDir := t.TempDir()
	repoDir := filepath.Join(exportDir, "repositories", "workspace", "repo.git")
	assert.NoError(t, os.MkdirAll(repoDir, 0755))

	// Create a regular repo, make a commit, then clone it as bare into the expected path.
	// git commit doesn't work directly on a bare repo, so we use a temp working tree.
	workDir := t.TempDir()
	gitEnv := append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")

	for _, step := range []struct {
		args []string
		dir  string
	}{
		{[]string{"init", workDir}, ""},
		{[]string{"commit", "--allow-empty", "-m", "initial"}, workDir},
		{[]string{"clone", "--bare", workDir, repoDir}, ""},
	} {
		cmd := exec.Command("git", step.args...)
		cmd.Dir = step.dir
		cmd.Env = gitEnv
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s: %v", step.args, out, err)
		}
	}

	// Get the real commit SHA that exists in the bare repo
	cmd := exec.Command("git", "--git-dir", repoDir, "rev-parse", "HEAD")
	out, err := cmd.Output()
	assert.NoError(t, err)
	existingSHA := strings.TrimSpace(string(out))
	assert.Len(t, existingSHA, 40)

	// A SHA that definitely does not exist in the repo
	missingSHA := strings.Repeat("a", 40)

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Return two merged PRs:
		//   PR 1: head SHA exists in pack (regular merge or squash with branch kept)
		//   PR 2: head SHA does NOT exist in pack (squash merge, branch deleted)
		// Both have a merge_commit that points to existingSHA.
		writeResponse(t, w, []byte(`{
			"values": [
				{
					"id": 1,
					"title": "Regular merge PR",
					"state": "MERGED",
					"updated_on": "2024-01-01T00:00:00+00:00",
					"created_on": "2024-01-01T00:00:00+00:00",
					"author": {"uuid": "{test-uuid}"},
					"source":      {"branch": {"name": "feature"}, "commit": {"hash": "`+existingSHA+`"}},
					"destination": {"branch": {"name": "main"},    "commit": {"hash": "`+existingSHA+`"}},
					"merge_commit": {"hash": "`+existingSHA+`"}
				},
				{
					"id": 2,
					"title": "Squash merge PR - branch deleted",
					"state": "MERGED",
					"updated_on": "2024-01-02T00:00:00+00:00",
					"created_on": "2024-01-02T00:00:00+00:00",
					"author": {"uuid": "{test-uuid}"},
					"source":      {"branch": {"name": "squash-branch"}, "commit": {"hash": "`+missingSHA+`"}},
					"destination": {"branch": {"name": "main"},          "commit": {"hash": "`+existingSHA+`"}},
					"merge_commit": {"hash": "`+existingSHA+`"}
				}
			],
			"next": null
		}`))
	}))
	defer testServer.Close()

	client := &Client{
		baseURL:        testServer.URL,
		httpClient:     testServer.Client(),
		logger:         logger,
		commitSHACache: make(map[string]string),
		exportDir:      exportDir,
	}

	prs, err := client.GetPullRequests("workspace", "repo", false, "")
	assert.NoError(t, err)
	assert.Len(t, prs, 2)

	// PR 1: head SHA exists in pack — should be unchanged
	assert.Equal(t, existingSHA, prs[0].Head.SHA,
		"PR with existing head SHA should not be modified")

	// PR 2: head SHA missing from pack — should fall back to merge commit SHA
	assert.Equal(t, existingSHA, prs[1].Head.SHA,
		"PR with missing head SHA should fall back to merge commit SHA")
	assert.NotEqual(t, missingSHA, prs[1].Head.SHA,
		"Missing head SHA should not be retained")
}

func TestParseHunkNewStart(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected int
	}{
		{
			name:     "standard hunk header",
			header:   "@@ -10,6 +15,8 @@",
			expected: 15,
		},
		{
			name:     "new file starting at line 1",
			header:   "@@ -0,0 +1,5 @@",
			expected: 1,
		},
		{
			name:     "no comma in new-file range",
			header:   "@@ -1 +1 @@",
			expected: 1,
		},
		{
			name:     "hunk header with function context",
			header:   "@@ -10,6 +15,8 @@ func foo() {",
			expected: 15,
		},
		{
			name:     "first line of file",
			header:   "@@ -1,3 +1,3 @@",
			expected: 1,
		},
		{
			name:     "empty string falls back to 1",
			header:   "",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, parseHunkNewStart(tt.header))
		})
	}
}

func TestParseDiffHunk(t *testing.T) {
	// A simple two-file diff used across several cases.
	simpleDiff := strings.Join([]string{
		"diff --git a/foo.go b/foo.go",
		"index 0000001..0000002 100644",
		"--- a/foo.go",
		"+++ b/foo.go",
		"@@ -1,4 +1,5 @@",
		" line1",
		" line2",
		"+added line",
		" line3",
		" line4",
		"diff --git a/bar.go b/bar.go",
		"index 0000003..0000004 100644",
		"--- a/bar.go",
		"+++ b/bar.go",
		"@@ -1,3 +1,3 @@",
		" alpha",
		"-removed",
		"+replaced",
		" beta",
	}, "\n")

	tests := []struct {
		name             string
		diff             string
		filePath         string
		targetLine       int
		expectNil        bool
		expectedPosition int
		expectHunkPrefix string // @@ header the hunk should start with
	}{
		{
			name:             "added line matched",
			diff:             simpleDiff,
			filePath:         "foo.go",
			targetLine:       3, // "+added line" is the 3rd new-file line
			expectedPosition: 4, // position 1=@@, 2=line1, 3=line2, 4=+added line
			expectHunkPrefix: "@@ -1,4 +1,5 @@",
		},
		{
			name:             "context line matched",
			diff:             simpleDiff,
			filePath:         "foo.go",
			targetLine:       4, // " line3" is the 4th new-file line
			expectedPosition: 5,
			expectHunkPrefix: "@@ -1,4 +1,5 @@",
		},
		{
			name:             "replaced line in second file",
			diff:             simpleDiff,
			filePath:         "bar.go",
			targetLine:       2, // "+replaced" is the 2nd new-file line
			expectedPosition: 4, // position 1=@@, 2=alpha, 3=-removed, 4=+replaced
			expectHunkPrefix: "@@ -1,3 +1,3 @@",
		},
		{
			name:      "file not in diff returns nil",
			diff:      simpleDiff,
			filePath:  "missing.go",
			targetLine: 1,
			expectNil: true,
		},
		{
			name:      "target line beyond end of hunk returns nil",
			diff:      simpleDiff,
			filePath:  "foo.go",
			targetLine: 99,
			expectNil: true,
		},
		{
			name: "multi-hunk file: target in second hunk",
			diff: strings.Join([]string{
				"diff --git a/multi.go b/multi.go",
				"index 0000001..0000002 100644",
				"--- a/multi.go",
				"+++ b/multi.go",
				"@@ -1,3 +1,3 @@",
				" a",
				"-old",
				"+new",
				"@@ -10,3 +10,4 @@",
				" x",
				"+inserted",
				" y",
				" z",
			}, "\n"),
			filePath:         "multi.go",
			targetLine:       11, // "+inserted" is new-file line 11
			expectedPosition: 3,  // position 1=@@, 2=x, 3=+inserted
			expectHunkPrefix: "@@ -10,3 +10,4 @@",
		},
		{
			name: "deleted-only hunk has no new-file lines for target",
			diff: strings.Join([]string{
				"diff --git a/gone.go b/gone.go",
				"index 0000001..0000002 100644",
				"--- a/gone.go",
				"+++ b/gone.go",
				"@@ -1,3 +0,0 @@",
				"-line1",
				"-line2",
				"-line3",
			}, "\n"),
			filePath:   "gone.go",
			targetLine: 1,
			expectNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDiffHunk(tt.diff, tt.filePath, tt.targetLine)
			if tt.expectNil {
				assert.Nil(t, result)
				return
			}
			assert.NotNil(t, result)
			assert.Equal(t, tt.expectedPosition, result.Position)
			assert.True(t, strings.HasPrefix(result.Hunk, tt.expectHunkPrefix),
				"hunk should start with %q, got %q", tt.expectHunkPrefix, result.Hunk)
		})
	}
}
