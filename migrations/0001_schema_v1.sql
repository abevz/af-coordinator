create table projects (
  id text primary key,
  key text not null unique,
  name text not null,
  description text not null default '',
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
  kind text not null default 'blocks',
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
  issue_id text references issues(id) on delete cascade,
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
