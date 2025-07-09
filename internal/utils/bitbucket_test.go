package utils

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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
	client := NewClient("https://example.com", "token", "user", "pass", logger)

	assert.NotNil(t, client)
	assert.Equal(t, "https://example.com", client.baseURL)
	assert.Equal(t, "token", client.token)
	assert.Equal(t, "user", client.username)
	assert.Equal(t, "pass", client.appPass)
	assert.NotNil(t, client.httpClient)
	assert.NotNil(t, client.logger)
}

// func TestMakeRequest(t *testing.T) {
// 	// Test case 1: Successful request
// 	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		w.WriteHeader(http.StatusOK)
// 		writeResponse(t, w, []byte(`{"message": "success"}`))
// 	}))
// 	defer testServer.Close()

// 	logger, _ := zap.NewDevelopment()
// 	client := &Client{
// 		baseURL:    testServer.URL,
// 		httpClient: testServer.Client(),
// 		logger:     logger,
// 	}

// 	var result map[string]interface{}
// 	err := client.makeRequest("GET", "/", &result)

// 	assert.NoError(t, err)
// 	assert.Equal(t, "success", result["message"])

// 	// Test case 2: API returns an error
// 	testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		w.WriteHeader(http.StatusInternalServerError)
// 		writeResponse(t, w, []byte(`{"error": "internal server error"}`))
// 	}))
// 	defer testServer.Close()

// 	client.baseURL = testServer.URL
// 	err = client.makeRequest("GET", "/", &result)

// 	assert.Error(t, err)
// 	assert.Contains(t, err.Error(), "API request failed with status 500")

// 	// Test case 3: Rate limited - using a more controlled approach
// 	// Create a counter to track request attempts
// 	requestCount := 0

// 	// Use a lower retry count for the test to make it faster
// 	originalMaxRetries := maxRetries // Store original value if it's accessible
// 	testMaxRetries := 3              // Set a smaller value just for the test
// 	maxRetries = testMaxRetries      // Override the global value temporarily

// 	// Reduce delay time for faster test execution
// 	originalBaseDelay := baseDelay
// 	baseDelay = 10 * time.Millisecond

// 	testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		requestCount++
// 		// Always return rate limit error
// 		w.WriteHeader(http.StatusTooManyRequests) // 429
// 		w.Header().Set("X-RateLimit-Remaining", "0")
// 		w.Header().Set("X-RateLimit-Limit", "100")
// 		writeResponse(t, w, []byte(`{"message": "rate limited"}`))
// 	}))
// 	defer testServer.Close()

// 	client.baseURL = testServer.URL
// 	err = client.makeRequest("GET", "/", &result)

// 	// Restore original values
// 	baseDelay = originalBaseDelay
// 	maxRetries = originalMaxRetries // Restore the original value

// 	assert.Error(t, err)
// 	assert.Equal(t, maxRetries+1, requestCount, "Expected exactly maxRetries+1 requests")
// 	assert.Contains(t, err.Error(), "API request failed after")
// }

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

	prs := []data.PullRequest{{URL: "https://example.com/pr/1", Head: data.PRBranch{Sha: "testsha"}}}
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
		Head: data.PRBranch{Sha: "abcdef"},
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
