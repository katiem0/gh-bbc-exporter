package utils

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/katiem0/gh-bbc-exporter/internal/data"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

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
		w.Write([]byte(`{"message": "success"}`))
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
		w.Write([]byte(`{"error": "internal server error"}`))
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
		w.Write([]byte(`{"message": "rate limited"}`))
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
		w.Write([]byte(`{"hash": "1234567890123456789012345678901234567890"}`))
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
		w.Write([]byte(`{"error": "internal server error"}`))
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
		w.Write([]byte(`{"values": [], "next": null}`))
	}))
	defer testServer.Close()

	logger, _ := zap.NewDevelopment()
	client := &Client{
		baseURL:        testServer.URL,
		httpClient:     testServer.Client(),
		logger:         logger,
		commitSHACache: make(map[string]string),
	}

	prs, err := client.GetPullRequests("workspace", "repo")

	assert.NoError(t, err)
	assert.Empty(t, prs)

	// Test case 2: Successful request with pull requests
	testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values": [{"id": 1, "title": "Test PR"}], "next": null}`))
	}))
	defer testServer.Close()

	client.baseURL = testServer.URL
	prs, err = client.GetPullRequests("workspace", "repo")

	assert.NoError(t, err)
	assert.NotEmpty(t, prs)
	assert.Equal(t, 1, len(prs))
	assert.Equal(t, "Test PR", prs[0].Title)

	// Test case 3: API returns an error
	testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer testServer.Close()

	client.baseURL = testServer.URL
	prs, err = client.GetPullRequests("workspace", "repo")

	assert.Error(t, err)
	assert.Nil(t, prs)
}

func TestGetPullRequestComments(t *testing.T) {
	// Test case 1: Successful request with no comments
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values": [], "next": null}`))
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
		w.Write([]byte(`{"error": "internal server error"}`))
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
		w.Write([]byte(`{"values": [], "next": null}`))
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
		w.Write([]byte(`{"values": [{"user": {"display_name": "Test User", "uuid": "{test-uuid}"}}], "next": null}`))
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
		w.Write([]byte(`{"error": "internal server error"}`))
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
		w.Write([]byte(`{"name": "Test Repo", "mainbranch": {"name": "main"}}`))
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
		w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer testServer.Close()

	client.baseURL = testServer.URL
	repo, err = client.GetRepository("workspace", "repo")

	assert.Error(t, err)
	assert.Nil(t, repo)
}
