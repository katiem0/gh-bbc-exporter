package utils

// // GetIssues retrieves issues for a repository
// func (c *Client) GetIssues(workspace, repoSlug string) ([]data.Issue, error) {
// 	endpoint := fmt.Sprintf("/repositories/%s/%s/issues", workspace, repoSlug)

// 	c.logger.Debug("Fetching issues",
// 		zap.String("workspace", workspace),
// 		zap.String("repository", repoSlug))

// 	// Implement pagination and response handling for issues

// 	// This is a placeholder that you'd need to implement
// 	return []data.Issue{}, nil
// }

// // GetPullRequests retrieves pull requests for a repository
// func (c *Client) GetPullRequests(workspace, repoSlug string) ([]data.PullRequest, error) {
// 	endpoint := fmt.Sprintf("/repositories/%s/%s/pullrequests", workspace, repoSlug)

// 	c.logger.Debug("Fetching pull requests",
// 		zap.String("workspace", workspace),
// 		zap.String("repository", repoSlug))

// 	// Implement pagination and response handling for pull requests

// 	// This is a placeholder that you'd need to implement
// 	return []data.PullRequest{}, nil
// }

// // Additional API methods would be added here for users, commits, etc.
