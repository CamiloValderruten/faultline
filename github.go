package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
)

// GitHubClient wraps the GitHub REST API.
type GitHubClient struct {
	token  string
	owner  string
	repo   string
	client *http.Client
	logger *slog.Logger
}

// NewGitHubClient creates a new GitHub API client.
func NewGitHubClient(token, owner, repo string, logger *slog.Logger) *GitHubClient {
	return &GitHubClient{
		token:  token,
		owner:  owner,
		repo:   repo,
		client: &http.Client{},
		logger: logger,
	}
}

func (g *GitHubClient) apiURL(path string) string {
	return fmt.Sprintf("https://api.github.com%s", path)
}

func (g *GitHubClient) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, int, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, g.apiURL(path), bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", g.token))
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

// RepoInfo holds basic repository information.
type RepoInfo struct {
	Name           string `json:"name"`
	FullName      string `json:"full_name"`
	Description   string `json:"description"`
	Owner         string `json:"owner"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
	URL           string `json:"html_url"`
}

// GetRepoInfo fetches information about the repository.
func (g *GitHubClient) GetRepoInfo(ctx context.Context) (*RepoInfo, error) {
	path := fmt.Sprintf("/repos/%s/%s", g.owner, g.repo)
	body, status, err := g.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d", path, status)
	}

	var info RepoInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	info.Owner = g.owner
	return &info, nil
}

// Issue represents a GitHub issue or pull request.
type Issue struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	State     string   `json:"state"`
	Author    string   `json:"user"`
	Labels    []string `json:"labels"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
	URL       string   `json:"html_url"`
	IsPR      bool     `json:"pull_request,omitempty"`
}

// CreateIssue creates a new issue in the repository.
func (g *GitHubClient) CreateIssue(ctx context.Context, title, body string, labels []string) (*Issue, error) {
	path := fmt.Sprintf("/repos/%s/%s/issues", g.owner, g.repo)
	input := map[string]interface{}{
		"title": title,
		"body":  body,
	}
	if len(labels) > 0 {
		input["labels"] = labels
	}

	respBody, status, err := g.doRequest(ctx, "POST", path, input)
	if err != nil {
		return nil, err
	}
	if status != http.StatusCreated {
		return nil, fmt.Errorf("POST %s: status %d", path, status)
	}

	var issue Issue
	if err := json.Unmarshal(respBody, &issue); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &issue, nil
}

// ListIssues fetches issues (not PRs) from the repository.
func (g *GitHubClient) ListIssues(ctx context.Context, state, labels string) ([]Issue, error) {
	path := fmt.Sprintf("/repos/%s/%s/issues?state=%s&per_page=30", g.owner, g.repo, state)
	if labels != "" {
		path += "&labels=" + url.QueryEscape(labels)
	}

	body, status, err := g.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d", path, status)
	}

	var rawIssues []json.RawMessage
	if err := json.Unmarshal(body, &rawIssues); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	issues := make([]Issue, 0, len(rawIssues))
	for _, raw := range rawIssues {
		var issue Issue
		if err := json.Unmarshal(raw, &issue); err != nil {
			continue
		}
		if issue.IsPR {
			continue
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

// GetIssue fetches a specific issue or PR by number.
func (g *GitHubClient) GetIssue(ctx context.Context, number int) (*Issue, error) {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d", g.owner, g.repo, number)
	body, status, err := g.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d", path, status)
	}

	var issue Issue
	if err := json.Unmarshal(body, &issue); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &issue, nil
}

// PR represents a GitHub pull request.
type PR struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	State     string   `json:"state"`
	Author    string   `json:"user"`
	Head      string   `json:"head"`
	Base      string   `json:"base"`
	Draft     bool     `json:"draft"`
	Labels    []string `json:"labels"`
	CreatedAt string   `json:"created_at"`
	URL       string   `json:"html_url"`
}

// CreatePR creates a new pull request.
func (g *GitHubClient) CreatePR(ctx context.Context, title, body, head, base string) (*PR, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls", g.owner, g.repo)
	input := map[string]string{
		"title": title,
		"body":  body,
		"head":  head,
		"base":  base,
	}

	respBody, status, err := g.doRequest(ctx, "POST", path, input)
	if err != nil {
		return nil, err
	}
	if status != http.StatusCreated {
		return nil, fmt.Errorf("POST %s: status %d", path, status)
	}

	var pr PR
	if err := json.Unmarshal(respBody, &pr); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &pr, nil
}

// ListPRs fetches pull requests from the repository.
func (g *GitHubClient) ListPRs(ctx context.Context, state string) ([]PR, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls?state=%s&per_page=30", g.owner, g.repo, state)
	body, status, err := g.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d", path, status)
	}

	var prs []PR
	if err := json.Unmarshal(body, &prs); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return prs, nil
}

// FileContent holds the content of a file.
type FileContent struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Content string `json:"content"` // base64-encoded
	SHA     string `json:"sha"`
	Size    int    `json:"size"`
}

// GetFile fetches a file from the repository.
func (g *GitHubClient) GetFile(ctx context.Context, path string) (*FileContent, error) {
	apiPath := fmt.Sprintf("/repos/%s/%s/contents/%s", g.owner, g.repo, path)
	body, status, err := g.doRequest(ctx, "GET", apiPath, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d", apiPath, status)
	}

	var file FileContent
	if err := json.Unmarshal(body, &file); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &file, nil
}

// CreateOrUpdateFile creates or updates a file in the repository.
func (g *GitHubClient) CreateOrUpdateFile(ctx context.Context, path, message, content, branch, sha string) (string, error) {
	apiPath := fmt.Sprintf("/repos/%s/%s/contents/%s", g.owner, g.repo, path)

	reqBody := map[string]string{
		"message": message,
		"content": content,
	}
	if branch != "" {
		reqBody["branch"] = branch
	}
	if sha != "" {
		reqBody["sha"] = sha
	}

	body, status, err := g.doRequest(ctx, "PUT", apiPath, reqBody)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return "", fmt.Errorf("PUT %s: status %d", apiPath, status)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if commit, ok := response["commit"].(map[string]interface{}); ok {
		if sha, ok := commit["sha"].(string); ok {
			return sha, nil
		}
	}
	return "", nil
}

// Branch represents a git branch.
type Branch struct {
	Name      string `json:"name"`
	Protected bool   `json:"protected"`
	SHA       string `json:"commit"`
}

// ListBranches fetches all branches in the repository.
func (g *GitHubClient) ListBranches(ctx context.Context) ([]Branch, error) {
	path := fmt.Sprintf("/repos/%s/%s/branches", g.owner, g.repo)
	body, status, err := g.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d", path, status)
	}

	var branches []Branch
	if err := json.Unmarshal(body, &branches); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return branches, nil
}

// CreateBranch creates a new branch from a base branch.
func (g *GitHubClient) CreateBranch(ctx context.Context, branchName, baseBranch string) error {
	refPath := fmt.Sprintf("/repos/%s/%s/git/ref/heads/%s", g.owner, g.repo, baseBranch)
	body, status, err := g.doRequest(ctx, "GET", refPath, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("GET %s: status %d", refPath, status)
	}

	var refResponse map[string]interface{}
	if err := json.Unmarshal(body, &refResponse); err != nil {
		return fmt.Errorf("unmarshal ref response: %w", err)
	}

	object, ok := refResponse["object"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("unexpected ref response format")
	}
	sha, ok := object["sha"].(string)
	if !ok {
		return fmt.Errorf("missing sha in ref response")
	}

	createRefPath := fmt.Sprintf("/repos/%s/%s/git/refs", g.owner, g.repo)
	createBody := map[string]string{
		"ref": fmt.Sprintf("refs/heads/%s", branchName),
		"sha": sha,
	}
	_, status, err = g.doRequest(ctx, "POST", createRefPath, createBody)
	if err != nil {
		return err
	}
	if status != http.StatusCreated {
		return fmt.Errorf("POST %s: status %d", createRefPath, status)
	}

	return nil
}

// RepoSearchInput holds parameters for searching repositories.
type RepoSearchInput struct {
	Query   string `json:"q"`
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
}

// RepoSearchResult represents a repository search result item.
type RepoSearchResult struct {
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	Owner       string `json:"owner"`
	Stars       int    `json:"stargazers_count"`
	Language    string `json:"language"`
	URL         string `json:"html_url"`
}

// SearchRepos searches for repositories.
func (g *GitHubClient) SearchRepos(ctx context.Context, query string, page, perPage int) ([]RepoSearchResult, error) {
	path := fmt.Sprintf("/search/repositories?q=%s", url.QueryEscape(query))
	if page > 0 {
		path += fmt.Sprintf("&page=%d", page)
	}
	if perPage > 0 {
		path += fmt.Sprintf("&per_page=%d", perPage)
	} else {
		path += "&per_page=10"
	}

	body, status, err := g.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d", path, status)
	}

	var response struct {
		Items []RepoSearchResult `json:"items"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return response.Items, nil
}

// DecodeBase64FileContent decodes base64-encoded file content.
func DecodeBase64FileContent(encoded string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}
	return string(decoded), nil
}

// EncodeToBase64 encodes a string to base64.
func EncodeToBase64(content string) string {
	return base64.StdEncoding.EncodeToString([]byte(content))
}