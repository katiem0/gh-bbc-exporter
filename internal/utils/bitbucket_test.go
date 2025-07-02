package utils

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/katiem0/gh-bbc-exporter/internal/data"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

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

func TestMakeRequest(t *testing.T) {
	// Test case 1: Successful request
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		writeResponse(t, w, []byte(`{"message": "success"}`))
	}))
	defer testServer.Close()

	logger, _ := zap.NewDevelopment()
	client := &Client{
		baseURL:    testServer.URL,
		httpClient: testServer.Client(),
		logger:     logger,
	}

	var result map[string]interface{}
	err := client.makeRequest("GET", "/", &result)

	assert.NoError(t, err)
	assert.Equal(t, "success", result["message"])

	// Test case 2: API returns an error
	testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		writeResponse(t, w, []byte(`{"error": "internal server error"}`))
	}))
	defer testServer.Close()

	client.baseURL = testServer.URL
	err = client.makeRequest("GET", "/", &result)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API request failed with status 500")

	// Test case 3: Rate limited
	testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Limit", "100")
		writeResponse(t, w, []byte(`{"message": "rate limited"}`))
	}))
	defer testServer.Close()
	client.baseURL = testServer.URL
	err = client.makeRequest("GET", "/", &result)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API request failed after 5 retries")
}

func TestGetFullCommitSHA(t *testing.T) {
	// Test case 1: Successful request
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		writeResponse(t, w, []byte(`{"hash": "1234567890123456789012345678901234567890"}`))
	}))
	defer testServer.Close()

	logger, _ := zap.NewDevelopment()
	client := &Client{
		baseURL:    testServer.URL,
		httpClient: testServer.Client(),
		logger:     logger,
	}

	sha, err := client.GetFullCommitSHA("workspace", "repo", "shortsha")

	assert.NoError(t, err)
	assert.Equal(t, "1234567890123456789012345678901234567890", sha)

	// Test case 2: API returns an error
	testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		writeResponse(t, w, []byte(`{"error": "internal server error"}`))
	}))
	defer testServer.Close()

	client.baseURL = testServer.URL
	sha, err = client.GetFullCommitSHA("workspace", "repo", "shortsha")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch full commit SHA")
	assert.Equal(t, "shortsha", sha) // Should return the original short SHA
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

	prs, err := client.GetPullRequests("workspace", "repo", false)

	assert.NoError(t, err)
	assert.Empty(t, prs)

	// Test case 2: Successful request with pull requests
	testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		writeResponse(t, w, []byte(`{"values": [{"id": 1, "title": "Test PR"}], "next": null}`))
	}))
	defer testServer.Close()

	client.baseURL = testServer.URL
	prs, err = client.GetPullRequests("workspace", "repo", false)

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
	prs, err = client.GetPullRequests("workspace", "repo", false)

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

	prs, err := client.GetPullRequests("workspace", "repo", true)
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
	prs, err = client.GetPullRequests("workspace", "repo", false)
	assert.NoError(t, err)
	assert.Len(t, prs, 2)
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

	prs, err := client.GetPullRequests("workspace", "repo", false)
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
	exporter := NewExporter(client, tempDir, logger, false)

	err = exporter.Export("workspace", "non-existent-repo")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Repository not found")
}
