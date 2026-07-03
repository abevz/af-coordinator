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
