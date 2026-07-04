package api

import (
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

// schema is the full DDL from migrations/0001_schema_v1.sql, inlined for tests.
const schema = `
create table projects (
  id text primary key,
  key text not null unique,
  name text not null,
  description text not null default '',
  next_issue_seq integer not null default 1,
  created_at text not null,
  updated_at text not null
);

create table repositories (
  id text primary key,
  project_id text not null references projects(id) on delete cascade,
  logical_name text not null,
  canonical_git_dir text not null,
  default_branch text not null default 'main',
  hosting_kind text not null default '',
  hosting_slug text not null default '',
  created_at text not null,
  updated_at text not null,
  unique(project_id, logical_name)
);

create table repo_remotes (
  id text primary key,
  repository_id text not null references repositories(id) on delete cascade,
  remote_name text not null,
  fetch_url text not null,
  push_url text not null default '',
  is_primary integer not null default 0,
  created_at text not null,
  updated_at text not null,
  unique(repository_id, remote_name)
);

create table worktrees (
  id text primary key,
  repository_id text not null references repositories(id) on delete cascade,
  absolute_path text not null unique,
  branch text not null default '',
  head_commit text not null default '',
  remote_name text not null default '',
  remote_branch text not null default '',
  is_main integer not null default 0,
  is_ephemeral integer not null default 0,
  last_seen_at text not null,
  created_at text not null,
  updated_at text not null
);

create table artifact_roots (
  id text primary key,
  repository_id text not null references repositories(id) on delete cascade,
  root_path text not null,
  kind text not null default 'sdd',
  is_primary integer not null default 0,
  created_at text not null,
  updated_at text not null,
  unique(repository_id, root_path)
);

create table artifacts (
  id text primary key,
  repository_id text not null references repositories(id) on delete cascade,
  worktree_id text references worktrees(id) on delete set null,
  artifact_root_id text references artifact_roots(id) on delete set null,
  kind text not null,
  relative_path text not null,
  title text not null default '',
  external_key text not null default '',
  status text not null default '',
  created_at text not null,
  updated_at text not null,
  unique(repository_id, relative_path)
);

create table issues (
  id text primary key,
  short_id text not null unique,
  project_id text not null references projects(id) on delete cascade,
  repository_id text references repositories(id) on delete set null,
  worktree_id text references worktrees(id) on delete set null,
  scope_kind text not null
    check (scope_kind in ('project', 'repository', 'worktree')),
  title text not null,
  description text not null default '',
  status text not null
    check (status in ('open', 'in_progress', 'blocked', 'deferred', 'done', 'cancelled')),
  priority integer not null default 3,
  assignee text not null default '',
  version integer not null default 1,
  claimed_at text,
  closed_at text,
  created_at text not null,
  updated_at text not null
);

create table issue_artifacts (
  issue_id text not null references issues(id) on delete cascade,
  artifact_id text not null references artifacts(id) on delete cascade,
  relation text not null default 'implements',
  created_at text not null,
  primary key (issue_id, artifact_id, relation)
);

create table dependencies (
  issue_id text not null references issues(id) on delete cascade,
  depends_on_issue_id text not null references issues(id) on delete cascade,
  kind text not null default 'blocks'
    check (kind in ('blocks', 'parent', 'related', 'discovered-from')),
  created_at text not null,
  primary key (issue_id, depends_on_issue_id, kind)
);

create table leases (
  issue_id text primary key references issues(id) on delete cascade,
  holder text not null,
  lease_token text not null unique,
  expires_at text not null,
  created_at text not null,
  updated_at text not null
);

create table notes (
  id text primary key,
  issue_id text not null references issues(id) on delete cascade,
  author text not null,
  body text not null,
  created_at text not null
);

create table events (
  id text primary key,
  issue_id text references issues(id) on delete set null,
  actor text not null,
  event_type text not null,
  payload_json text not null default '{}',
  created_at text not null
);

create index idx_issues_project_status_priority
  on issues(project_id, status, priority, updated_at);

create index idx_issues_repo_status
  on issues(repository_id, status, updated_at);

create index idx_issues_worktree_status
  on issues(worktree_id, status, updated_at);

create unique index idx_repo_remotes_primary
  on repo_remotes(repository_id) where is_primary = 1;

create unique index idx_artifact_roots_primary
  on artifact_roots(repository_id) where is_primary = 1;

create index idx_artifact_roots_repo_kind
  on artifact_roots(repository_id, kind, root_path);

create index idx_artifacts_repo_kind
  on artifacts(repository_id, kind, relative_path);

create index idx_artifacts_worktree
  on artifacts(worktree_id, kind, relative_path);

create index idx_issue_artifacts_issue
  on issue_artifacts(issue_id, relation);

create index idx_issue_artifacts_artifact
  on issue_artifacts(artifact_id, relation);

create index idx_dependencies_issue
  on dependencies(issue_id);

create index idx_dependencies_depends_on
  on dependencies(depends_on_issue_id);

create index idx_leases_expires_at
  on leases(expires_at);

create index idx_events_issue_created_at
  on events(issue_id, created_at);

create index idx_worktrees_repository_path
  on worktrees(repository_id, absolute_path);
`

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

	if _, err := db.Exec(schema); err != nil {
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

	body := `{"project":"test","scope_kind":"project","title":"My issue"}`
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

	body := `{"project":"nonexistent","scope_kind":"project","title":"My issue"}`
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

	body := `{"title":"Updated","expected_version":1}`
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

	body := `{"resolution":"done","expected_version":1}`
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
	depBody := `{"depends_on":"issue-2","kind":"blocks"}`
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
