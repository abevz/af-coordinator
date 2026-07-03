package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/abevz/af-coordinator/internal/core"
)

// Client is an HTTP client that connects to the daemon over a Unix socket.
type Client struct {
	socketPath string
	httpClient *http.Client
}

// New creates a new Client connected to the given Unix socket path.
func New(socketPath string) *Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}

	return &Client{
		socketPath: socketPath,
		httpClient: &http.Client{
			Timeout:   5 * time.Second,
			Transport: transport,
		},
	}
}

// Health sends a GET /healthz request.
func (c *Client) Health() (core.Health, error) {
	var result core.Health
	if err := c.doJSON(http.MethodGet, "/healthz", nil, &result); err != nil {
		return core.Health{}, err
	}
	return result, nil
}

// CreateProject sends a POST /v1/projects request.
func (c *Client) CreateProject(key, name, description string) (core.Project, error) {
	body := map[string]string{
		"key":         key,
		"name":        name,
		"description": description,
	}

	var result struct {
		Project core.Project `json:"project"`
	}
	if err := c.doJSON(http.MethodPost, "/v1/projects", body, &result); err != nil {
		return core.Project{}, err
	}
	return result.Project, nil
}

// ListProjects sends a GET /v1/projects request.
func (c *Client) ListProjects() ([]core.Project, error) {
	var result struct {
		Projects []core.Project `json:"projects"`
	}
	if err := c.doJSON(http.MethodGet, "/v1/projects", nil, &result); err != nil {
		return nil, err
	}
	return result.Projects, nil
}

// CreateRepo sends a POST /v1/repos request.
func (c *Client) CreateRepo(req core.CreateRepoRequest) (core.Repository, []core.RepoRemote, error) {
	var result struct {
		Repository core.Repository   `json:"repository"`
		Remotes    []core.RepoRemote `json:"remotes"`
	}
	if err := c.doJSON(http.MethodPost, "/v1/repos", req, &result); err != nil {
		return core.Repository{}, nil, err
	}
	return result.Repository, result.Remotes, nil
}

// ListRepos sends a GET /v1/repos request with an optional project filter.
func (c *Client) ListRepos(project string) ([]core.Repository, error) {
	path := "/v1/repos"
	if project != "" {
		path += "?project=" + project
	}

	var result struct {
		Repositories []core.Repository `json:"repositories"`
	}
	if err := c.doJSON(http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return result.Repositories, nil
}

// RegisterWorktree sends a POST /v1/worktrees request.
func (c *Client) RegisterWorktree(req core.CreateWorktreeRequest) (core.Worktree, error) {
	var result struct {
		Worktree core.Worktree `json:"worktree"`
	}
	if err := c.doJSON(http.MethodPost, "/v1/worktrees", req, &result); err != nil {
		return core.Worktree{}, err
	}
	return result.Worktree, nil
}

// ListWorktrees sends a GET /v1/worktrees request with an optional repo filter.
func (c *Client) ListWorktrees(repo string) ([]core.Worktree, error) {
	path := "/v1/worktrees"
	if repo != "" {
		path += "?repo=" + repo
	}

	var result struct {
		Worktrees []core.Worktree `json:"worktrees"`
	}
	if err := c.doJSON(http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return result.Worktrees, nil
}

// CreateArtifactRoot sends a POST /v1/artifact-roots request.
func (c *Client) CreateArtifactRoot(req core.CreateArtifactRootRequest) (core.ArtifactRoot, error) {
	var result struct {
		ArtifactRoot core.ArtifactRoot `json:"artifact_root"`
	}
	if err := c.doJSON(http.MethodPost, "/v1/artifact-roots", req, &result); err != nil {
		return core.ArtifactRoot{}, err
	}
	return result.ArtifactRoot, nil
}

// ListArtifactRoots sends a GET /v1/artifact-roots request with an optional repo filter.
func (c *Client) ListArtifactRoots(repo string) ([]core.ArtifactRoot, error) {
	path := "/v1/artifact-roots"
	if repo != "" {
		path += "?repo=" + repo
	}

	var result struct {
		ArtifactRoots []core.ArtifactRoot `json:"artifact_roots"`
	}
	if err := c.doJSON(http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return result.ArtifactRoots, nil
}

// CreateArtifact sends a POST /v1/artifacts request.
func (c *Client) CreateArtifact(req core.CreateArtifactRequest) (core.Artifact, error) {
	var result struct {
		Artifact core.Artifact `json:"artifact"`
	}
	if err := c.doJSON(http.MethodPost, "/v1/artifacts", req, &result); err != nil {
		return core.Artifact{}, err
	}
	return result.Artifact, nil
}

// ListArtifacts sends a GET /v1/artifacts request with an optional repo filter.
func (c *Client) ListArtifacts(repo string) ([]core.Artifact, error) {
	path := "/v1/artifacts"
	if repo != "" {
		path += "?repo=" + repo
	}

	var result struct {
		Artifacts []core.Artifact `json:"artifacts"`
	}
	if err := c.doJSON(http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return result.Artifacts, nil
}

// CreateIssue sends a POST /v1/issues request.
func (c *Client) CreateIssue(req core.CreateIssueRequest) (core.Issue, error) {
	var result struct {
		Issue core.Issue `json:"issue"`
	}
	if err := c.doJSON(http.MethodPost, "/v1/issues", req, &result); err != nil {
		return core.Issue{}, err
	}
	return result.Issue, nil
}

// GetIssue sends a GET /v1/issues/{issueID} request.
func (c *Client) GetIssue(issueID string) (core.Issue, *core.IssueLease, error) {
	var result struct {
		Issue core.Issue       `json:"issue"`
		Lease *core.IssueLease `json:"lease,omitempty"`
	}
	if err := c.doJSON(http.MethodGet, "/v1/issues/"+issueID, nil, &result); err != nil {
		return core.Issue{}, nil, err
	}
	return result.Issue, result.Lease, nil
}

// ListIssues sends a GET /v1/issues request with optional query params.
func (c *Client) ListIssues(project, repo, worktree, status, assignee string) ([]core.Issue, error) {
	path := "/v1/issues"
	sep := "?"
	appendParam := func(key, val string) {
		if val != "" {
			path += sep + key + "=" + val
			sep = "&"
		}
	}
	appendParam("project", project)
	appendParam("repo", repo)
	appendParam("worktree", worktree)
	appendParam("status", status)
	appendParam("assignee", assignee)

	var result struct {
		Issues []core.Issue `json:"issues"`
	}
	if err := c.doJSON(http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return result.Issues, nil
}

// ClaimIssue sends a POST /v1/issues/{issueID}/claim request.
func (c *Client) ClaimIssue(issueID, holder string, ttlSeconds int) (core.ClaimResponse, error) {
	body := core.ClaimRequest{Holder: holder, TTLSeconds: ttlSeconds}
	var result core.ClaimResponse
	if err := c.doJSON(http.MethodPost, "/v1/issues/"+issueID+"/claim", body, &result); err != nil {
		return core.ClaimResponse{}, err
	}
	return result, nil
}

// HeartbeatLease sends a POST /v1/issues/{issueID}/heartbeat request.
func (c *Client) HeartbeatLease(issueID, leaseToken string, ttlSeconds int) (string, error) {
	body := core.HeartbeatRequest{LeaseToken: leaseToken, TTLSeconds: ttlSeconds}
	var result struct {
		ExpiresAt string `json:"expires_at"`
	}
	if err := c.doJSON(http.MethodPost, "/v1/issues/"+issueID+"/heartbeat", body, &result); err != nil {
		return "", err
	}
	return result.ExpiresAt, nil
}

// ReleaseLease sends a POST /v1/issues/{issueID}/release request.
func (c *Client) ReleaseLease(issueID, leaseToken string) error {
	body := core.ReleaseRequest{LeaseToken: leaseToken}
	return c.doJSON(http.MethodPost, "/v1/issues/"+issueID+"/release", body, nil)
}

// UpdateIssue sends a PATCH /v1/issues/{issueID} request.
func (c *Client) UpdateIssue(issueID string, req core.UpdateIssueRequest) (core.Issue, error) {
	var result struct {
		Issue core.Issue `json:"issue"`
	}
	if err := c.doJSON(http.MethodPatch, "/v1/issues/"+issueID, req, &result); err != nil {
		return core.Issue{}, err
	}
	return result.Issue, nil
}

// CloseIssue sends a POST /v1/issues/{issueID}/close request.
func (c *Client) CloseIssue(issueID string, req core.CloseIssueRequest) error {
	return c.doJSON(http.MethodPost, "/v1/issues/"+issueID+"/close", req, nil)
}

// LinkArtifact sends a POST /v1/issues/{issueID}/links request.
func (c *Client) LinkArtifact(issueID string, req core.LinkArtifactRequest) error {
	return c.doJSON(http.MethodPost, "/v1/issues/"+issueID+"/links", req, nil)
}

// AddDependency sends a POST /v1/issues/{issueID}/dependencies request.
func (c *Client) AddDependency(issueID string, req core.AddDependencyRequest) error {
	return c.doJSON(http.MethodPost, "/v1/issues/"+issueID+"/dependencies", req, nil)
}

// RemoveDependency sends a DELETE /v1/issues/{issueID}/dependencies/{dependsOn} request.
func (c *Client) RemoveDependency(issueID, dependsOn, kind string) error {
	path := "/v1/issues/" + issueID + "/dependencies/" + dependsOn
	if kind != "" {
		path += "?kind=" + kind
	}
	return c.doJSON(http.MethodDelete, path, nil, nil)
}

// ListReadyIssues sends a GET /v1/issues/ready request with an optional project filter.
func (c *Client) ListReadyIssues(project string) ([]core.Issue, error) {
	path := "/v1/issues/ready"
	if project != "" {
		path += "?project=" + project
	}

	var result struct {
		Issues []core.Issue `json:"issues"`
	}
	if err := c.doJSON(http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return result.Issues, nil
}

// doJSON performs an HTTP request and decodes the JSON response.
func (c *Client) doJSON(method, path string, body any, target any) error {
	var reqBody []byte
	if body != nil {
		var err error
		reqBody, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
	}

	req, err := http.NewRequest(method, "http://af-coordinator"+path, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errResp core.APIErrorResponse
		if decodeErr := json.NewDecoder(resp.Body).Decode(&errResp); decodeErr == nil && errResp.Error.Code != "" {
			return fmt.Errorf("api error: %s: %s", errResp.Error.Code, errResp.Error.Message)
		}
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	if target != nil {
		if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}
