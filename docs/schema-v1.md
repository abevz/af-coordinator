# Schema v1

## Notes

- SQLite dialect
- `version` is incremented on each update
- timestamps stored as UTC text or unix epoch, but be consistent
- foreign keys must be enabled

## Tables

### projects

```sql
create table projects (
  id text primary key,
  key text not null unique,
  name text not null,
  description text not null default '',
  created_at text not null,
  updated_at text not null
);
```

### repositories

```sql
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
```

### repo_remotes

```sql
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
```

### worktrees

```sql
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
```

### artifact_roots

```sql
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
```

### artifacts

```sql
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
```

### issues

```sql
create table issues (
  id text primary key,
  project_id text not null references projects(id) on delete cascade,
  repository_id text references repositories(id) on delete set null,
  worktree_id text references worktrees(id) on delete set null,
  scope_kind text not null,
  title text not null,
  description text not null default '',
  status text not null,
  priority integer not null default 3,
  assignee text not null default '',
  version integer not null default 1,
  claimed_at text not null default '',
  closed_at text not null default '',
  created_at text not null,
  updated_at text not null
);
```

### issue_artifacts

```sql
create table issue_artifacts (
  issue_id text not null references issues(id) on delete cascade,
  artifact_id text not null references artifacts(id) on delete cascade,
  relation text not null default 'implements',
  created_at text not null,
  primary key (issue_id, artifact_id, relation)
);
```

### dependencies

```sql
create table dependencies (
  issue_id text not null references issues(id) on delete cascade,
  depends_on_issue_id text not null references issues(id) on delete cascade,
  kind text not null default 'blocks',
  created_at text not null,
  primary key (issue_id, depends_on_issue_id, kind)
);
```

### leases

```sql
create table leases (
  issue_id text primary key references issues(id) on delete cascade,
  holder text not null,
  lease_token text not null unique,
  expires_at text not null,
  created_at text not null,
  updated_at text not null
);
```

### notes

```sql
create table notes (
  id text primary key,
  issue_id text not null references issues(id) on delete cascade,
  author text not null,
  body text not null,
  created_at text not null
);
```

### events

```sql
create table events (
  id text primary key,
  issue_id text references issues(id) on delete cascade,
  actor text not null,
  event_type text not null,
  payload_json text not null default '{}',
  created_at text not null
);
```

## Indexes

```sql
create index idx_issues_project_status_priority
  on issues(project_id, status, priority, updated_at);

create index idx_issues_repo_status
  on issues(repository_id, status, updated_at);

create index idx_issues_worktree_status
  on issues(worktree_id, status, updated_at);

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
```

## Ready query shape

Conceptually, a ready issue is:

- not `done`
- not `cancelled`
- not blocked by another unfinished issue
- not actively leased by someone else

This should be implemented in SQL inside the daemon, not in clients.

## Mutation contract

For mutable issue operations:

- request includes `expected_version`
- request includes valid `lease_token`
- transaction updates issue and increments `version`
- transaction appends an event row

## SDD artifact contract

The schema must support:

- registering repository artifact roots such as `docs/specs/`
- registering concrete spec artifacts such as `requirements.md`, `design.md`,
  `tasks.md`, `review.md`, and ADR files
- linking operational issues to the artifacts they execute or depend on

That preserves the split between:

- SDD as design truth
- coordinator state as execution truth
