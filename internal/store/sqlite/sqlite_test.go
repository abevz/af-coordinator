package sqlite

import (
	"os"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// newTestDB creates a temp-file-backed SQLite database with the schema applied.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	f, err := os.CreateTemp("", "af-coordinator-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	dbPath := f.Name()
	f.Close()
	_ = os.Remove(dbPath) // SQLite will keep the file handle open after open

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close(); os.Remove(dbPath) })

	// Enable foreign keys and set pragmas.
	for _, p := range []string{
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := db.Exec(p); err != nil {
			t.Fatal(err)
		}
	}

	// Apply schema matching migrations/0001_schema_v1.sql.
	schema := `
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
`
	_, err = db.Exec(schema)
	if err != nil {
		t.Fatal(err)
	}
	return db
}
