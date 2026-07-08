package api

import (
	"context"
	"github.com/abevz/af-coordinator/internal/core"
	"github.com/abevz/af-coordinator/internal/store/sqlite"
	"github.com/abevz/af-coordinator/migrations"

	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// registerRoutes duplicates the route setup from daemon.go for testing.
func registerRoutes(mux *http.ServeMux, db *sql.DB, logger *slog.Logger) {
	// Health endpoints
	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
	mux.HandleFunc("/healthz", healthHandler)
	mux.HandleFunc("/v1/health", healthHandler)

	// Projects
	mux.HandleFunc("POST /v1/projects", handleCreateProject(db, logger))
	mux.HandleFunc("GET /v1/projects", handleListProjects(db, logger))

	// Repos
	mux.HandleFunc("POST /v1/repos", handleCreateRepo(db, logger))
	mux.HandleFunc("GET /v1/repos", handleListRepos(db, logger))

	// Worktrees
	mux.HandleFunc("POST /v1/worktrees", handleRegisterWorktree(db, logger))
	mux.HandleFunc("GET /v1/worktrees", handleListWorktrees(db, logger))
	mux.HandleFunc("DELETE /v1/worktrees/{worktree_id}", handleDeleteWorktree(db, logger))
	mux.HandleFunc("GET /v1/events", handleWatchEvents(db, logger))

	// Artifact roots
	mux.HandleFunc("POST /v1/artifact-roots", handleCreateArtifactRoot(db, logger))
	mux.HandleFunc("GET /v1/artifact-roots", handleListArtifactRoots(db, logger))

	// Artifacts
	mux.HandleFunc("POST /v1/artifacts", handleCreateArtifact(db, logger))
	mux.HandleFunc("GET /v1/artifacts", handleListArtifacts(db, logger))

	// Issues
	mux.HandleFunc("POST /v1/issues", handleCreateIssue(db, logger))
	mux.HandleFunc("GET /v1/issues/ready", handleListReadyIssues(db, logger))
	mux.HandleFunc("GET /v1/issues/{issue_id}", handleGetIssue(db, logger))
	mux.HandleFunc("POST /v1/issues/{issue_id}/claim", handleClaimIssue(db, logger))
	mux.HandleFunc("POST /v1/issues/{issue_id}/heartbeat", handleHeartbeatLease(db, logger))
	mux.HandleFunc("POST /v1/issues/{issue_id}/release", handleReleaseLease(db, logger))
	mux.HandleFunc("PATCH /v1/issues/{issue_id}", handleUpdateIssue(db, logger))
	mux.HandleFunc("POST /v1/issues/{issue_id}/close", handleCloseIssue(db, logger))
	mux.HandleFunc("POST /v1/issues/{issue_id}/dependencies", handleAddDependency(db, logger))
	mux.HandleFunc("DELETE /v1/issues/{issue_id}/dependencies/{depends_on}", handleRemoveDependency(db, logger))
	mux.HandleFunc("POST /v1/issues/{issue_id}/links", handleLinkArtifact(db, logger))
	mux.HandleFunc("POST /v1/issues/{issue_id}/notes", handleCreateNote(db, logger))
	mux.HandleFunc("GET /v1/issues/{issue_id}/notes", handleListNotes(db, logger))
	mux.HandleFunc("GET /v1/issues/{issue_id}/events", handleListEvents(db, logger))
	mux.HandleFunc("GET /v1/issues", handleListIssues(db, logger))
}

// newTestServer creates an in-memory SQLite DB, initializes the schema,
// creates an HTTP test server with all routes, and returns the server + db.
func newTestServer(t *testing.T) (*httptest.Server, *sql.DB) {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	// Single connection to keep the in-memory database shared.
	db.SetMaxOpenConns(1)
	// Busy timeout so concurrent readers on the single connection
	// block and retry instead of returning SQLITE_BUSY.
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		t.Fatal(err)
	}

	if err := sqlite.Migrate(context.Background(), db, migrations.FS); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	mux := http.NewServeMux()
	registerRoutes(mux, db, logger)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return server, db
}

// decodeJSON decodes a JSON response body into the target type T.
func decodeJSON[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()
	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decodeJSON: %v", err)
	}
	return result
}

// ---------------------------------------------------------------------------
// Projects
// ---------------------------------------------------------------------------

func TestCreateProject(t *testing.T) {
	server, _ := newTestServer(t)

	body := `{"name":"Test Project","key":"test"}`
	resp, err := http.Post(server.URL+"/v1/projects", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", resp.StatusCode)
	}

	var result struct {
		Project struct {
			ID   string `json:"id"`
			Key  string `json:"key"`
			Name string `json:"name"`
		} `json:"project"`
	}
	result = decodeJSON[struct {
		Project struct {
			ID   string `json:"id"`
			Key  string `json:"key"`
			Name string `json:"name"`
		} `json:"project"`
	}](t, resp)

	if result.Project.Key != "test" {
		t.Errorf("expected key 'test', got %q", result.Project.Key)
	}
	if result.Project.Name != "Test Project" {
		t.Errorf("expected name 'Test Project', got %q", result.Project.Name)
	}
	if result.Project.ID == "" {
		t.Error("expected non-empty project ID")
	}
}

func TestCreateProjectMissingName(t *testing.T) {
	server, _ := newTestServer(t)

	body := `{"key":"test"}`
	resp, err := http.Post(server.URL+"/v1/projects", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", resp.StatusCode)
	}

	var errResp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	errResp = decodeJSON[struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}](t, resp)

	if errResp.Error.Code != "validation_failed" {
		t.Errorf("expected code validation_failed, got %q", errResp.Error.Code)
	}
}

func TestListProjects(t *testing.T) {
	server, db := newTestServer(t)

	// Insert two projects directly
	now := time.Now().UTC().Format(time.RFC3339)
	for _, p := range []struct {
		id, key, name string
	}{
		{"p1", "alpha", "Alpha"},
		{"p2", "beta", "Beta"},
	} {
		_, err := db.Exec(
			`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
			 VALUES (?, ?, ?, '', 1, ?, ?)`,
			p.id, p.key, p.name, now, now,
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	resp, err := http.Get(server.URL + "/v1/projects")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	var result struct {
		Projects []struct {
			ID   string `json:"id"`
			Key  string `json:"key"`
			Name string `json:"name"`
		} `json:"projects"`
	}
	result = decodeJSON[struct {
		Projects []struct {
			ID   string `json:"id"`
			Key  string `json:"key"`
			Name string `json:"name"`
		} `json:"projects"`
	}](t, resp)

	if len(result.Projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(result.Projects))
	}
}

// ---------------------------------------------------------------------------
// Repos
// ---------------------------------------------------------------------------

func TestCreateRepo(t *testing.T) {
	server, db := newTestServer(t)

	// Create a project first
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test Project', '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	body := `{"project":"test","logical_name":"main","canonical_git_dir":"/tmp/repo","default_branch":"main"}`
	resp, err := http.Post(server.URL+"/v1/repos", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", resp.StatusCode)
	}

	var result struct {
		Repository struct {
			ID              string `json:"id"`
			LogicalName     string `json:"logical_name"`
			CanonicalGitDir string `json:"canonical_git_dir"`
		} `json:"repository"`
		Remotes []any `json:"remotes"`
	}
	result = decodeJSON[struct {
		Repository struct {
			ID              string `json:"id"`
			LogicalName     string `json:"logical_name"`
			CanonicalGitDir string `json:"canonical_git_dir"`
		} `json:"repository"`
		Remotes []any `json:"remotes"`
	}](t, resp)

	if result.Repository.LogicalName != "main" {
		t.Errorf("expected logical_name 'main', got %q", result.Repository.LogicalName)
	}
}

func TestListRepos(t *testing.T) {
	server, db := newTestServer(t)

	// Create a project and a repo
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(
		`INSERT INTO repositories (id, project_id, logical_name, canonical_git_dir, default_branch, created_at, updated_at)
		 VALUES ('repo-1', 'proj-1', 'main', '/tmp/repo', 'main', ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(server.URL + "/v1/repos")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	var result struct {
		Repositories []struct {
			ID string `json:"id"`
		} `json:"repositories"`
	}
	result = decodeJSON[struct {
		Repositories []struct {
			ID string `json:"id"`
		} `json:"repositories"`
	}](t, resp)

	if len(result.Repositories) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(result.Repositories))
	}
}

// ---------------------------------------------------------------------------
// Issues
// ---------------------------------------------------------------------------

func TestCreateIssue(t *testing.T) {
	server, db := newTestServer(t)

	// Create a project
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	body := `{"project":"test","scope_kind":"project","title":"My issue","actor":"test"}`
	resp, err := http.Post(server.URL+"/v1/issues", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", resp.StatusCode)
	}

	var result struct {
		Issue struct {
			ID        string `json:"id"`
			ShortID   string `json:"short_id"`
			Title     string `json:"title"`
			Status    string `json:"status"`
			ScopeKind string `json:"scope_kind"`
		} `json:"issue"`
	}
	result = decodeJSON[struct {
		Issue struct {
			ID        string `json:"id"`
			ShortID   string `json:"short_id"`
			Title     string `json:"title"`
			Status    string `json:"status"`
			ScopeKind string `json:"scope_kind"`
		} `json:"issue"`
	}](t, resp)

	if result.Issue.Title != "My issue" {
		t.Errorf("expected title 'My issue', got %q", result.Issue.Title)
	}
	if result.Issue.Status != "open" {
		t.Errorf("expected status 'open', got %q", result.Issue.Status)
	}
	if result.Issue.ScopeKind != "project" {
		t.Errorf("expected scope_kind 'project', got %q", result.Issue.ScopeKind)
	}
	if !strings.HasPrefix(result.Issue.ShortID, "test-") {
		t.Errorf("expected short_id to start with 'test-', got %q", result.Issue.ShortID)
	}
}

func TestCreateIssueMissingTitle(t *testing.T) {
	server, db := newTestServer(t)

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	body := `{"project":"test","scope_kind":"project"}`
	resp, err := http.Post(server.URL+"/v1/issues", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", resp.StatusCode)
	}
}

func TestCreateIssueProjectNotFound(t *testing.T) {
	server, _ := newTestServer(t)

	body := `{"project":"nonexistent","scope_kind":"project","title":"My issue","actor":"test"}`
	resp, err := http.Post(server.URL+"/v1/issues", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d", resp.StatusCode)
	}
}

func TestGetIssue(t *testing.T) {
	server, db := newTestServer(t)

	// Create project + issue directly
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	issueID := "issue-1"
	_, err = db.Exec(
		`INSERT INTO issues (id, short_id, project_id, scope_kind, title, description, status, priority, assignee, version, created_at, updated_at)
		 VALUES (?, 'test-1', 'proj-1', 'project', 'Test issue', '', 'open', 3, '', 1, ?, ?)`,
		issueID, now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(server.URL + "/v1/issues/" + issueID)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	var result struct {
		Issue struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"issue"`
	}
	result = decodeJSON[struct {
		Issue struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"issue"`
	}](t, resp)

	if result.Issue.ID != issueID {
		t.Errorf("expected issue ID %q, got %q", issueID, result.Issue.ID)
	}
	if result.Issue.Title != "Test issue" {
		t.Errorf("expected title 'Test issue', got %q", result.Issue.Title)
	}
}

func TestGetIssueIncludesExplicitDependencyIdentifiers(t *testing.T) {
	server, db := newTestServer(t)

	if _, err := sqlite.CreateProject(context.Background(), db, "test", "Test", ""); err != nil {
		t.Fatal(err)
	}
	source, err := sqlite.CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{
		ScopeKind: "project",
		Title:     "Source",
	})
	if err != nil {
		t.Fatal(err)
	}
	target, err := sqlite.CreateIssue(context.Background(), db, "test", core.CreateIssueRequest{
		ScopeKind: "project",
		Title:     "Target",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlite.AddDependency(context.Background(), db, source.ID, core.AddDependencyRequest{
		DependsOn: target.ID,
		Kind:      "blocks",
	}); err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(server.URL + "/v1/issues/" + source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	var result struct {
		Issue struct {
			Dependencies []struct {
				IssueID          string `json:"issue_id"`
				IssueShortID     string `json:"issue_short_id"`
				DependsOnID      string `json:"depends_on_id"`
				DependsOnShortID string `json:"depends_on_short_id"`
				Kind             string `json:"kind"`
			} `json:"dependencies"`
		} `json:"issue"`
	}
	result = decodeJSON[struct {
		Issue struct {
			Dependencies []struct {
				IssueID          string `json:"issue_id"`
				IssueShortID     string `json:"issue_short_id"`
				DependsOnID      string `json:"depends_on_id"`
				DependsOnShortID string `json:"depends_on_short_id"`
				Kind             string `json:"kind"`
			} `json:"dependencies"`
		} `json:"issue"`
	}](t, resp)

	if len(result.Issue.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(result.Issue.Dependencies))
	}
	dep := result.Issue.Dependencies[0]
	if dep.IssueID != source.ID || dep.IssueShortID != source.ShortID {
		t.Fatalf("unexpected source dependency identity: %+v", dep)
	}
	if dep.DependsOnID != target.ID || dep.DependsOnShortID != target.ShortID {
		t.Fatalf("unexpected dependency target identity: %+v", dep)
	}
}

func TestGetIssueLeaseTokenLeak(t *testing.T) {
	server, db := newTestServer(t)

	// Create project + issue
	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	)
	issueID := "issue-1"
	_, _ = db.Exec(
		`INSERT INTO issues (id, short_id, project_id, scope_kind, title, description, status, priority, assignee, version, created_at, updated_at)
		 VALUES (?, 'test-1', 'proj-1', 'project', 'Test issue', '', 'open', 3, '', 1, ?, ?)`,
		issueID, now, now,
	)

	// Claim it
	claimBody := `{"holder":"agent-1","ttl_seconds":3600}`
	claimReq, _ := http.NewRequest("POST", server.URL+"/v1/issues/"+issueID+"/claim", strings.NewReader(claimBody))
	claimReq.Header.Set("Content-Type", "application/json")
	claimRespHTTP, err := http.DefaultClient.Do(claimReq)
	if err != nil || claimRespHTTP.StatusCode != http.StatusOK {
		t.Fatalf("failed to claim issue")
	}

	// Get it
	resp, err := http.Get(server.URL + "/v1/issues/" + issueID)
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	leaseObj, ok := result["lease"].(map[string]any)
	if !ok {
		t.Fatalf("expected lease object in response, got %v", result["lease"])
	}

	if _, hasToken := leaseObj["lease_token"]; hasToken {
		t.Errorf("SECURITY LEAK: lease_token must not be returned in GET issue response")
	}
	if holder, ok := leaseObj["holder"].(string); !ok || holder != "agent-1" {
		t.Errorf("expected holder agent-1, got %v", leaseObj["holder"])
	}
}

func TestGetIssueNotFound(t *testing.T) {
	server, _ := newTestServer(t)

	resp, err := http.Get(server.URL + "/v1/issues/nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d", resp.StatusCode)
	}
}

func TestClaimIssue(t *testing.T) {
	server, db := newTestServer(t)

	// Create project and issue
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	issueID := "issue-1"
	_, err = db.Exec(
		`INSERT INTO issues (id, short_id, project_id, scope_kind, title, description, status, priority, assignee, version, created_at, updated_at)
		 VALUES (?, 'test-1', 'proj-1', 'project', 'Claimable', '', 'open', 3, '', 1, ?, ?)`,
		issueID, now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	body := `{"holder":"agent-1","ttl_seconds":3600}`
	req, err := http.NewRequest("POST", server.URL+"/v1/issues/"+issueID+"/claim", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	var claimResp struct {
		LeaseToken string `json:"lease_token"`
		ExpiresAt  string `json:"expires_at"`
	}
	claimResp = decodeJSON[struct {
		LeaseToken string `json:"lease_token"`
		ExpiresAt  string `json:"expires_at"`
	}](t, resp)

	if claimResp.LeaseToken == "" {
		t.Error("expected non-empty lease_token")
	}
	if claimResp.ExpiresAt == "" {
		t.Error("expected non-empty expires_at")
	}
}

func TestClaimIssueNotFound(t *testing.T) {
	server, _ := newTestServer(t)

	body := `{"holder":"agent-1","ttl_seconds":3600}`
	req, err := http.NewRequest("POST", server.URL+"/v1/issues/nonexistent/claim", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d", resp.StatusCode)
	}
}

func TestListIssues(t *testing.T) {
	server, db := newTestServer(t)

	// Create project and issues
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	for i, title := range []string{"Issue A", "Issue B"} {
		shortID := "test-" + string(rune('1'+i))
		_, err := db.Exec(
			`INSERT INTO issues (id, short_id, project_id, scope_kind, title, description, status, priority, assignee, version, created_at, updated_at)
			 VALUES (?, ?, 'proj-1', 'project', ?, '', 'open', 3, '', 1, ?, ?)`,
			"issue-"+string(rune('a'+i)), shortID, title, now, now,
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	resp, err := http.Get(server.URL + "/v1/issues?project=test")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	var result struct {
		Issues []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"issues"`
	}
	result = decodeJSON[struct {
		Issues []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"issues"`
	}](t, resp)

	if len(result.Issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(result.Issues))
	}
}

func TestListIssuesShowsHolder(t *testing.T) {
	server, db := newTestServer(t)

	// Create project and an issue.
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	issueID := "issue-1"
	_, err = db.Exec(
		`INSERT INTO issues (id, short_id, project_id, scope_kind, title, description, status, priority, assignee, version, created_at, updated_at)
		 VALUES (?, 'test-1', 'proj-1', 'project', 'Claimed', '', 'open', 3, '', 1, ?, ?)`,
		issueID, now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Claim the issue via the API.
	body := `{"holder":"agent-99","ttl_seconds":3600}`
	req, err := http.NewRequest("POST", server.URL+"/v1/issues/"+issueID+"/claim", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on claim, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// List issues — the holder should appear in the response.
	resp, err = http.Get(server.URL + "/v1/issues?project=test")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	var result struct {
		Issues []struct {
			ID             string `json:"id"`
			Holder         string `json:"holder"`
			LeaseExpiresAt string `json:"lease_expires_at"`
		} `json:"issues"`
	}
	result = decodeJSON[struct {
		Issues []struct {
			ID             string `json:"id"`
			Holder         string `json:"holder"`
			LeaseExpiresAt string `json:"lease_expires_at"`
		} `json:"issues"`
	}](t, resp)

	if len(result.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result.Issues))
	}
	if result.Issues[0].Holder != "agent-99" {
		t.Errorf("expected holder 'agent-99', got %q", result.Issues[0].Holder)
	}
	if result.Issues[0].LeaseExpiresAt == "" {
		t.Error("expected non-empty lease_expires_at")
	}
}

// ---------------------------------------------------------------------------
// Health
// ---------------------------------------------------------------------------

func TestHealth(t *testing.T) {
	server, _ := newTestServer(t)

	resp, err := http.Get(server.URL + "/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	var health struct {
		Status string `json:"status"`
	}
	health = decodeJSON[struct {
		Status string `json:"status"`
	}](t, resp)

	if health.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", health.Status)
	}
}

func TestHealthz(t *testing.T) {
	server, _ := newTestServer(t)

	resp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
}

func TestListReadyIssuesScopesRepoByProject(t *testing.T) {
	server, db := newTestServer(t)

	if _, err := sqlite.CreateProject(context.Background(), db, "p1", "Project 1", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlite.CreateProject(context.Background(), db, "p2", "Project 2", ""); err != nil {
		t.Fatal(err)
	}

	repo1, _, err := sqlite.CreateRepo(context.Background(), db, "p1", core.CreateRepoRequest{
		Project:         "p1",
		LogicalName:     "shared",
		CanonicalGitDir: "/repos/p1-shared.git",
	})
	if err != nil {
		t.Fatal(err)
	}
	repo2, _, err := sqlite.CreateRepo(context.Background(), db, "p2", core.CreateRepoRequest{
		Project:         "p2",
		LogicalName:     "shared",
		CanonicalGitDir: "/repos/p2-shared.git",
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := sqlite.CreateIssue(context.Background(), db, "p1", core.CreateIssueRequest{
		ScopeKind: "repository",
		Repo:      repo1.ID,
		Title:     "P1 issue",
	}); err != nil {
		t.Fatal(err)
	}
	want, err := sqlite.CreateIssue(context.Background(), db, "p2", core.CreateIssueRequest{
		ScopeKind: "repository",
		Repo:      repo2.ID,
		Title:     "P2 issue",
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(server.URL + "/v1/issues/ready?project=p2&repo=shared")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	var result struct {
		Issues []struct {
			ID string `json:"id"`
		} `json:"issues"`
	}
	result = decodeJSON[struct {
		Issues []struct {
			ID string `json:"id"`
		} `json:"issues"`
	}](t, resp)

	if len(result.Issues) != 1 {
		t.Fatalf("expected 1 ready issue, got %d", len(result.Issues))
	}
	if result.Issues[0].ID != want.ID {
		t.Fatalf("expected issue %q, got %q", want.ID, result.Issues[0].ID)
	}
}

// ---------------------------------------------------------------------------
// Worktrees
// ---------------------------------------------------------------------------

func TestCreateWorktree(t *testing.T) {
	server, db := newTestServer(t)

	// Create project and repo
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(
		`INSERT INTO repositories (id, project_id, logical_name, canonical_git_dir, default_branch, created_at, updated_at)
		 VALUES ('repo-1', 'proj-1', 'main', '/tmp/repo', 'main', ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	body := `{"repo":"repo-1","absolute_path":"/tmp/worktree","branch":"feature-1"}`
	resp, err := http.Post(server.URL+"/v1/worktrees", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", resp.StatusCode)
	}

	var result struct {
		Worktree struct {
			ID           string `json:"id"`
			AbsolutePath string `json:"absolute_path"`
			Branch       string `json:"branch"`
		} `json:"worktree"`
	}
	result = decodeJSON[struct {
		Worktree struct {
			ID           string `json:"id"`
			AbsolutePath string `json:"absolute_path"`
			Branch       string `json:"branch"`
		} `json:"worktree"`
	}](t, resp)

	if result.Worktree.AbsolutePath != "/tmp/worktree" {
		t.Errorf("expected absolute_path '/tmp/worktree', got %q", result.Worktree.AbsolutePath)
	}
}

// ---------------------------------------------------------------------------
// Artifact Roots
// ---------------------------------------------------------------------------

func TestCreateArtifactRoot(t *testing.T) {
	server, db := newTestServer(t)

	// Create project and repo
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(
		`INSERT INTO repositories (id, project_id, logical_name, canonical_git_dir, default_branch, created_at, updated_at)
		 VALUES ('repo-1', 'proj-1', 'main', '/tmp/repo', 'main', ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	body := `{"repo":"repo-1","root_path":"docs/specs","kind":"spec"}`
	resp, err := http.Post(server.URL+"/v1/artifact-roots", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", resp.StatusCode)
	}

	var result struct {
		ArtifactRoot struct {
			ID       string `json:"id"`
			RootPath string `json:"root_path"`
			Kind     string `json:"kind"`
		} `json:"artifact_root"`
	}
	result = decodeJSON[struct {
		ArtifactRoot struct {
			ID       string `json:"id"`
			RootPath string `json:"root_path"`
			Kind     string `json:"kind"`
		} `json:"artifact_root"`
	}](t, resp)

	if result.ArtifactRoot.RootPath != "docs/specs" {
		t.Errorf("expected root_path 'docs/specs', got %q", result.ArtifactRoot.RootPath)
	}
}

func TestDeleteWorktree(t *testing.T) {
	server, db := newTestServer(t)

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(
		`INSERT INTO repositories (id, project_id, logical_name, canonical_git_dir, default_branch, created_at, updated_at)
		 VALUES ('repo-1', 'proj-1', 'main', '/tmp/repo', 'main', ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(
		`INSERT INTO worktrees (id, repository_id, absolute_path, branch, head_commit, remote_name, remote_branch, is_main, is_ephemeral, last_seen_at, created_at, updated_at)
		 VALUES ('wt-1', 'repo-1', '/tmp/worktree', 'feature-1', '', '', '', 0, 0, ?, ?, ?)`,
		now, now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodDelete, server.URL+"/v1/worktrees/wt-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	result := decodeJSON[struct {
		Worktree struct {
			ID string `json:"id"`
		} `json:"worktree"`
	}](t, resp)
	if result.Worktree.ID != "wt-1" {
		t.Fatalf("expected deleted worktree id wt-1, got %q", result.Worktree.ID)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(1) FROM worktrees WHERE id = 'wt-1'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected worktree row to be deleted, found %d row(s)", count)
	}
}

func TestDeleteWorktreeRejectsMain(t *testing.T) {
	server, db := newTestServer(t)

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(
		`INSERT INTO repositories (id, project_id, logical_name, canonical_git_dir, default_branch, created_at, updated_at)
		 VALUES ('repo-1', 'proj-1', 'main', '/tmp/repo', 'main', ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(
		`INSERT INTO worktrees (id, repository_id, absolute_path, branch, head_commit, remote_name, remote_branch, is_main, is_ephemeral, last_seen_at, created_at, updated_at)
		 VALUES ('wt-main', 'repo-1', '/tmp/main', 'main', '', '', '', 1, 0, ?, ?, ?)`,
		now, now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodDelete, server.URL+"/v1/worktrees/wt-main", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 Conflict, got %d", resp.StatusCode)
	}

	errResp := decodeJSON[struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}](t, resp)
	if errResp.Error.Code != core.ErrConflict {
		t.Fatalf("expected error code %q, got %q", core.ErrConflict, errResp.Error.Code)
	}
}

// ---------------------------------------------------------------------------
// Artifacts
// ---------------------------------------------------------------------------

func TestCreateArtifact(t *testing.T) {
	server, db := newTestServer(t)

	// Create project and repo
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(
		`INSERT INTO repositories (id, project_id, logical_name, canonical_git_dir, default_branch, created_at, updated_at)
		 VALUES ('repo-1', 'proj-1', 'main', '/tmp/repo', 'main', ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	body := `{"repo":"repo-1","kind":"spec","relative_path":"docs/api.md"}`
	resp, err := http.Post(server.URL+"/v1/artifacts", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", resp.StatusCode)
	}

	var result struct {
		Artifact struct {
			ID           string `json:"id"`
			RelativePath string `json:"relative_path"`
			Kind         string `json:"kind"`
		} `json:"artifact"`
	}
	result = decodeJSON[struct {
		Artifact struct {
			ID           string `json:"id"`
			RelativePath string `json:"relative_path"`
			Kind         string `json:"kind"`
		} `json:"artifact"`
	}](t, resp)

	if result.Artifact.RelativePath != "docs/api.md" {
		t.Errorf("expected relative_path 'docs/api.md', got %q", result.Artifact.RelativePath)
	}
}

// ---------------------------------------------------------------------------
// Update Issue
// ---------------------------------------------------------------------------

func TestUpdateIssue(t *testing.T) {
	server, db := newTestServer(t)

	// Create project and issue
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	issueID := "issue-1"
	_, err = db.Exec(
		`INSERT INTO issues (id, short_id, project_id, scope_kind, title, description, status, priority, assignee, version, created_at, updated_at)
		 VALUES (?, 'test-1', 'proj-1', 'project', 'Original', '', 'open', 3, '', 1, ?, ?)`,
		issueID, now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	body := `{"title":"Updated","expected_version":1,"actor":"test"}`
	req, err := http.NewRequest("PATCH", server.URL+"/v1/issues/"+issueID, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	var result struct {
		Issue struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"issue"`
	}
	result = decodeJSON[struct {
		Issue struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"issue"`
	}](t, resp)

	if result.Issue.Title != "Updated" {
		t.Errorf("expected title 'Updated', got %q", result.Issue.Title)
	}
}

// ---------------------------------------------------------------------------
// Close Issue
// ---------------------------------------------------------------------------

func TestCloseIssue(t *testing.T) {
	server, db := newTestServer(t)

	// Create project and issue
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	issueID := "issue-1"
	_, err = db.Exec(
		`INSERT INTO issues (id, short_id, project_id, scope_kind, title, description, status, priority, assignee, version, created_at, updated_at)
		 VALUES (?, 'test-1', 'proj-1', 'project', 'Closable', '', 'open', 3, '', 1, ?, ?)`,
		issueID, now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	body := `{"resolution":"done","expected_version":1,"actor":"test"}`
	req, err := http.NewRequest("POST", server.URL+"/v1/issues/"+issueID+"/close", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Notes
// ---------------------------------------------------------------------------

func TestCreateAndListNotes(t *testing.T) {
	server, db := newTestServer(t)

	// Create project and issue
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	issueID := "issue-1"
	_, err = db.Exec(
		`INSERT INTO issues (id, short_id, project_id, scope_kind, title, description, status, priority, assignee, version, created_at, updated_at)
		 VALUES (?, 'test-1', 'proj-1', 'project', 'Noted', '', 'open', 3, '', 1, ?, ?)`,
		issueID, now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Create a note
	noteBody := `{"author":"tester","body":"This is a note"}`
	req, err := http.NewRequest("POST", server.URL+"/v1/issues/"+issueID+"/notes", strings.NewReader(noteBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created for note, got %d", resp.StatusCode)
	}

	// List notes
	resp, err = http.Get(server.URL + "/v1/issues/" + issueID + "/notes")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for note list, got %d", resp.StatusCode)
	}

	var notesResult struct {
		Notes []struct {
			ID     string `json:"id"`
			Author string `json:"author"`
			Body   string `json:"body"`
		} `json:"notes"`
	}
	notesResult = decodeJSON[struct {
		Notes []struct {
			ID     string `json:"id"`
			Author string `json:"author"`
			Body   string `json:"body"`
		} `json:"notes"`
	}](t, resp)

	if len(notesResult.Notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notesResult.Notes))
	}
	if notesResult.Notes[0].Body != "This is a note" {
		t.Errorf("expected note body 'This is a note', got %q", notesResult.Notes[0].Body)
	}
}

// ---------------------------------------------------------------------------
// Events
// ---------------------------------------------------------------------------

func TestListEvents(t *testing.T) {
	server, db := newTestServer(t)

	// Create project and issue
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	issueID := "issue-1"
	_, err = db.Exec(
		`INSERT INTO issues (id, short_id, project_id, scope_kind, title, description, status, priority, assignee, version, created_at, updated_at)
		 VALUES (?, 'test-1', 'proj-1', 'project', 'Eventful', '', 'open', 3, '', 1, ?, ?)`,
		issueID, now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Insert an event directly
	_, err = db.Exec(
		`INSERT INTO events (id, issue_id, actor, event_type, payload_json, created_at)
		 VALUES ('evt-1', ?, 'system', 'issue.created', '{}', ?)`,
		issueID, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(server.URL + "/v1/issues/" + issueID + "/events")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	var eventsResult struct {
		Events []struct {
			ID        string `json:"id"`
			EventType string `json:"event_type"`
		} `json:"events"`
	}
	eventsResult = decodeJSON[struct {
		Events []struct {
			ID        string `json:"id"`
			EventType string `json:"event_type"`
		} `json:"events"`
	}](t, resp)

	if len(eventsResult.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(eventsResult.Events))
	}
	if eventsResult.Events[0].EventType != "issue.created" {
		t.Errorf("expected event_type 'issue.created', got %q", eventsResult.Events[0].EventType)
	}
}

func TestWatchEvents(t *testing.T) {
	server, db := newTestServer(t)

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(
		`INSERT INTO issues (id, short_id, project_id, scope_kind, title, description, status, priority, assignee, version, created_at, updated_at)
		 VALUES ('issue-1', 'test-1', 'proj-1', 'project', 'Eventful', '', 'open', 3, '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(
		`INSERT INTO events (id, issue_id, actor, event_type, payload_json, created_at)
		 VALUES ('evt-1', 'issue-1', 'system', 'issue.created', '{"title":"Eventful"}', ?)`,
		now,
	)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(server.URL + "/v1/events")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	result := decodeJSON[core.EventPage](t, resp)
	if len(result.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(result.Events))
	}
	if result.NextSince == "" {
		t.Fatal("expected non-empty next_since")
	}
	if !json.Valid([]byte(result.Events[0].PayloadJSON)) {
		t.Fatalf("expected valid payload_json, got %q", result.Events[0].PayloadJSON)
	}

	resp, err = http.Get(server.URL + "/v1/events?since=" + result.NextSince)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on cursor follow-up, got %d", resp.StatusCode)
	}
	followUp := decodeJSON[core.EventPage](t, resp)
	if len(followUp.Events) != 0 {
		t.Fatalf("expected 0 follow-up events, got %d", len(followUp.Events))
	}
	if followUp.NextSince != result.NextSince {
		t.Fatalf("expected next_since to stay at %q, got %q", result.NextSince, followUp.NextSince)
	}
}

func TestWatchEventsInvalidCursor(t *testing.T) {
	server, _ := newTestServer(t)

	resp, err := http.Get(server.URL + "/v1/events?since=bad-cursor")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", resp.StatusCode)
	}
}

func TestWatchEventsLongPollTimeout(t *testing.T) {
	server, _ := newTestServer(t)

	start := time.Now()
	resp, err := http.Get(server.URL + "/v1/events?wait_ms=50")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Fatalf("expected long-poll to wait at least ~40ms, got %v", elapsed)
	}

	result := decodeJSON[core.EventPage](t, resp)
	if len(result.Events) != 0 {
		t.Fatalf("expected no events, got %d", len(result.Events))
	}
	if result.NextSince != "" {
		t.Fatalf("expected empty next_since, got %q", result.NextSince)
	}
}

// ---------------------------------------------------------------------------
// Dependencies
// ---------------------------------------------------------------------------

func TestAddRemoveDependency(t *testing.T) {
	server, db := newTestServer(t)

	// Create project and two issues
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(
		`INSERT INTO issues (id, short_id, project_id, scope_kind, title, description, status, priority, assignee, version, created_at, updated_at)
		 VALUES ('issue-1', 'test-1', 'proj-1', 'project', 'First', '', 'open', 3, '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(
		`INSERT INTO issues (id, short_id, project_id, scope_kind, title, description, status, priority, assignee, version, created_at, updated_at)
		 VALUES ('issue-2', 'test-2', 'proj-1', 'project', 'Second', '', 'open', 3, '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Add dependency
	depBody := `{"depends_on":"issue-2","kind":"blocks","actor":"test"}`
	req, err := http.NewRequest("POST", server.URL+"/v1/issues/issue-1/dependencies", strings.NewReader(depBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created for dependency, got %d", resp.StatusCode)
	}

	// Remove dependency
	req, err = http.NewRequest("DELETE", server.URL+"/v1/issues/issue-1/dependencies/issue-2", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 No Content for dependency removal, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Release lease
// ---------------------------------------------------------------------------

func TestReleaseLease(t *testing.T) {
	server, db := newTestServer(t)

	// Create project and issue
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	issueID := "issue-1"
	_, err = db.Exec(
		`INSERT INTO issues (id, short_id, project_id, scope_kind, title, description, status, priority, assignee, version, created_at, updated_at)
		 VALUES (?, 'test-1', 'proj-1', 'project', 'Leased', '', 'open', 3, '', 1, ?, ?)`,
		issueID, now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Claim first to get a token
	claimBody := `{"holder":"agent-1","ttl_seconds":3600}`
	req, _ := http.NewRequest("POST", server.URL+"/v1/issues/"+issueID+"/claim", strings.NewReader(claimBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var claimResp struct {
		LeaseToken string `json:"lease_token"`
		ExpiresAt  string `json:"expires_at"`
	}
	claimResp = decodeJSON[struct {
		LeaseToken string `json:"lease_token"`
		ExpiresAt  string `json:"expires_at"`
	}](t, resp)

	// Release the lease
	releaseBody := `{"lease_token":"` + claimResp.LeaseToken + `"}`
	req, _ = http.NewRequest("POST", server.URL+"/v1/issues/"+issueID+"/release", strings.NewReader(releaseBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 No Content, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Heartbeat lease
// ---------------------------------------------------------------------------

func TestHeartbeatLease(t *testing.T) {
	server, db := newTestServer(t)

	// Create project and issue
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-1', 'test', 'Test', '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	issueID := "issue-1"
	_, err = db.Exec(
		`INSERT INTO issues (id, short_id, project_id, scope_kind, title, description, status, priority, assignee, version, created_at, updated_at)
		 VALUES (?, 'test-1', 'proj-1', 'project', 'Heartbeatable', '', 'open', 3, '', 1, ?, ?)`,
		issueID, now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Claim first to get a token
	claimBody := `{"holder":"agent-1","ttl_seconds":3600}`
	req, _ := http.NewRequest("POST", server.URL+"/v1/issues/"+issueID+"/claim", strings.NewReader(claimBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var claimResp struct {
		LeaseToken string `json:"lease_token"`
		ExpiresAt  string `json:"expires_at"`
	}
	claimResp = decodeJSON[struct {
		LeaseToken string `json:"lease_token"`
		ExpiresAt  string `json:"expires_at"`
	}](t, resp)

	// Heartbeat
	hbBody := `{"lease_token":"` + claimResp.LeaseToken + `","ttl_seconds":3600}`
	req, _ = http.NewRequest("POST", server.URL+"/v1/issues/"+issueID+"/heartbeat", strings.NewReader(hbBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	var hbResp struct {
		ExpiresAt string `json:"expires_at"`
	}
	hbResp = decodeJSON[struct {
		ExpiresAt string `json:"expires_at"`
	}](t, resp)

	if hbResp.ExpiresAt == "" {
		t.Error("expected non-empty expires_at in heartbeat response")
	}
}

// ─── Concurrency ────────────────────────────────────────────────────────────

func TestConcurrentClaimSameIssue(t *testing.T) {
	t.Parallel()
	server, db := newTestServer(t)

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO projects (id, key, name, description, next_issue_seq, created_at, updated_at)
		 VALUES ('proj-concur', 'concur', 'Concur', '', 1, ?, ?)`,
		now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	issueID := "issue-concur-1"
	_, err = db.Exec(
		`INSERT INTO issues (id, short_id, project_id, scope_kind, title, description, status, priority, assignee, version, created_at, updated_at)
		 VALUES (?, 'concur-1', 'proj-concur', 'project', 'Concurrent claim', '', 'open', 3, '', 1, ?, ?)`,
		issueID, now, now,
	)
	if err != nil {
		t.Fatal(err)
	}

	const goroutines = 2
	results := make(chan int, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			body := `{"holder":"agent-` + fmt.Sprintf("%d", id) + `","ttl_seconds":3600}`
			req, reqErr := http.NewRequest("POST", server.URL+"/v1/issues/"+issueID+"/claim", strings.NewReader(body))
			if reqErr != nil {
				t.Errorf("goroutine %d: build request: %v", id, reqErr)
				results <- 0
				return
			}
			req.Header.Set("Content-Type", "application/json")
			resp, reqErr := http.DefaultClient.Do(req)
			if reqErr != nil {
				t.Errorf("goroutine %d: do request: %v", id, reqErr)
				results <- 0
				return
			}
			resp.Body.Close()
			results <- resp.StatusCode
		}(i)
	}

	statuses := make([]int, 0, goroutines)
	for i := 0; i < goroutines; i++ {
		statuses = append(statuses, <-results)
	}

	successCount := 0
	conflictCount := 0
	for _, s := range statuses {
		switch s {
		case http.StatusOK:
			successCount++
		case http.StatusConflict:
			conflictCount++
		default:
			t.Errorf("unexpected status: %d", s)
		}
	}

	if successCount != 1 {
		t.Errorf("expected exactly 1 success, got %d", successCount)
	}
	if conflictCount != 1 {
		t.Errorf("expected exactly 1 conflict (409), got %d", conflictCount)
	}
}
