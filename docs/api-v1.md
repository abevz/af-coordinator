# API v1

HTTP+JSON over the unix socket. The daemon is the single write authority;
this contract is the product surface. `afctl` and agent wrappers are thin
clients over these endpoints.

Test shape:

```text
curl --unix-socket ~/.local/state/af-coordinator/af-coordinator.sock \
  http://localhost/v1/health
```

## Implementation layout

The API stack is intentionally thin and split into five layers:

- daemon entrypoint: `cmd/af-coordinatord/main.go`
- route registration and JSON helpers: `internal/api/daemon.go`,
  `internal/api/errors.go`
- endpoint handlers: `internal/api/projects.go`, `repos.go`, `worktrees.go`,
  `artifacts.go`, `issues.go`
- typed client over the unix socket: `internal/client/client.go`
- persistence and most business rules: `internal/store/sqlite/*.go`

The effective call path is:

```text
afctl or curl
  -> internal/client (for afctl)
  -> Unix socket HTTP API
  -> internal/api handlers
  -> internal/store/sqlite
  -> SQLite
```

## Conventions

- all bodies are JSON
- all timestamps are RFC 3339 UTC (`YYYY-MM-DDTHH:MM:SSZ`)
- issues are addressable by `short_id` (`afc-42`) everywhere an
  `{issue_id}` appears
- mutating requests carry `actor` (client-asserted identity, see
  architecture doc)
- list endpoints accept query filters, not fixed views

## Error taxonomy

Errors use one envelope:

```json
{
  "error": {
    "code": "version_conflict",
    "message": "expected version 3, current version is 5"
  }
}
```

| HTTP | code               | meaning                                            |
|------|--------------------|----------------------------------------------------|
| 400  | `validation_failed`| malformed body, unknown status, bad scope          |
| 404  | `not_found`        | unknown project/repo/worktree/artifact/issue       |
| 409  | `version_conflict` | `expected_version` does not match current version  |
| 409  | `lease_held`       | another holder has an unexpired lease              |
| 409  | `already_linked`   | artifact is already linked to the issue            |
| 409  | `short_id_taken`   | an issue with this short_id already exists         |
| 410  | `lease_expired`    | supplied `lease_token` is expired or unknown       |
| 422  | `dependency_cycle` | a `blocks` edge would create a cycle               |
| 500  | `internal_error`   | internal daemon/database failure                   |

Clients handle `version_conflict` by rereading and retrying;
`lease_held` by backing off or picking other ready work;
`lease_expired` by re-claiming.

## Health

- `GET /healthz` — liveness, also `GET /v1/health`

## Endpoint map

This is the compact route-to-implementation inventory for the current daemon.

### `internal/api/projects.go`

- `POST /v1/projects` -> `handleCreateProject` -> `sqlite.CreateProject`
- `GET /v1/projects` -> `handleListProjects` -> `sqlite.ListProjects`

### `internal/api/repos.go`

- `POST /v1/repos` -> `handleCreateRepo` -> `sqlite.CreateRepo`
- `GET /v1/repos?project=` -> `handleListRepos` ->
  `sqlite.ListReposByProjectKey` / `sqlite.ListRepos`

### `internal/api/worktrees.go`

- `POST /v1/worktrees` -> `handleRegisterWorktree` -> `sqlite.UpsertWorktree`
- `GET /v1/worktrees?repo=` -> `handleListWorktrees` ->
  `sqlite.ListWorktrees`

### `internal/api/artifacts.go`

- `POST /v1/artifact-roots` -> `handleCreateArtifactRoot` ->
  `sqlite.CreateArtifactRoot`
- `GET /v1/artifact-roots?repo=` -> `handleListArtifactRoots` ->
  `sqlite.ListArtifactRoots`
- `POST /v1/artifacts` -> `handleCreateArtifact` -> `sqlite.CreateArtifact`
- `GET /v1/artifacts?repo=` -> `handleListArtifacts` ->
  `sqlite.ListArtifacts`

### `internal/api/issues.go`

- `POST /v1/issues` -> `handleCreateIssue` -> `sqlite.CreateIssue`
- `GET /v1/issues/{issue_id}` -> `handleGetIssue` -> `sqlite.GetIssue`
- `GET /v1/issues?...` -> `handleListIssues` -> `sqlite.ListIssues`
- `GET /v1/issues/ready?project=&repo=` -> `handleListReadyIssues` ->
  `sqlite.ListReadyIssues`
- `POST /v1/issues/{issue_id}/claim` -> `handleClaimIssue` ->
  `sqlite.ClaimIssue`
- `POST /v1/issues/{issue_id}/heartbeat` -> `handleHeartbeatLease` ->
  `sqlite.HeartbeatLease`
- `POST /v1/issues/{issue_id}/release` -> `handleReleaseLease` ->
  `sqlite.ReleaseLease`
- `PATCH /v1/issues/{issue_id}` -> `handleUpdateIssue` ->
  `sqlite.UpdateIssue`
- `POST /v1/issues/{issue_id}/close` -> `handleCloseIssue` ->
  `sqlite.CloseIssue`
- `POST /v1/issues/{issue_id}/dependencies` -> `handleAddDependency` ->
  `sqlite.AddDependency`
- `DELETE /v1/issues/{issue_id}/dependencies/{depends_on}?kind=` ->
  `handleRemoveDependency` -> `sqlite.RemoveDependency`
- `POST /v1/issues/{issue_id}/links` -> `handleLinkArtifact` ->
  `sqlite.LinkArtifact`
- `DELETE /v1/issues/{issue_id}/links?artifact=&relation=&actor=` ->
  `handleUnlinkArtifact` -> `sqlite.UnlinkArtifact`
- `GET /v1/issues/{issue_id}/links` -> `handleListIssueLinks` ->
  `sqlite.ListIssueLinks`
- `POST /v1/issues/{issue_id}/notes` -> `handleCreateNote` ->
  `sqlite.CreateNote`
- `GET /v1/issues/{issue_id}/notes` -> `handleListNotes` ->
  `sqlite.ListNotes`
- `GET /v1/issues/{issue_id}/events` -> `handleListEvents` ->
  `sqlite.ListEvents`

## Registry

- `POST /v1/projects` — create project (`key`, `name`, `description`);
  key must start with a letter, contain only lowercase letters and digits (no
  leading/trailing/double hyphens), max 16 characters
- `GET  /v1/projects` — list
- `POST /v1/repos` — register repository (`project`, `logical_name`,
  `canonical_git_dir`, `default_branch`, remotes)
- `GET  /v1/repos?project=` — list
- `POST /v1/worktrees` — register/update worktree by `absolute_path`
  (upsert: re-registration refreshes branch, HEAD, `last_seen_at`)
- `GET  /v1/worktrees?repo=` — list
- `POST /v1/artifact-roots` — register artifact root (`repo`, `root_path`,
  `kind`)
- `GET  /v1/artifact-roots?repo=` — list registered artifact roots
- `POST /v1/artifacts` — register artifact (`repo`, `relative_path`,
  `kind`, `title`). Performs an upsert: if the artifact already exists by
  `(repo, relative_path)`, updates `title` and `kind` without changing ID.
- `GET  /v1/artifacts?repo=` — list

## Issues

- `POST /v1/issues` — create; daemon allocates `short_id`; body includes
  `project`, `scope_kind`, optional `repo`/`worktree`, `title`,
  `description`, `acceptance_criteria`, `priority`, `issue_type`
  (`task` default, `bug`, `feature`, `epic`, `chore`)
- `GET  /v1/issues/{issue_id}` — fetch one, including current lease if any
- dependency payloads inside issue responses use explicit identity fields:
  `issue_id`, `issue_short_id`, `depends_on_id`, `depends_on_short_id`
- `GET  /v1/issues?project=&repo=&worktree=&status=&assignee=&type=` — query
- `GET  /v1/issues/ready?project=&repo=` — computed ready view; excludes
  epics (they are containers, not units of work). When `repo` is given
  alongside `project`, repository logical-name resolution is scoped to that
  project; without `project`, prefer repository UUIDs over ambiguous names.
- `PATCH /v1/issues/{issue_id}` — edit metadata and status (`title`, `issue_type`,
  `description`, `acceptance_criteria`, `priority`, `assignee`, `status`); requires
  `expected_version`, plus `lease_token` if the issue is claimed
- `POST /v1/issues/{issue_id}/close` — requires `lease_token` +
  `expected_version`; body: `resolution` (`done` | `cancelled`), optional `note` (appends note and closes atomically)

## Leases

- `POST /v1/issues/{issue_id}/claim` — body: `holder`, `ttl_seconds`;
  returns `lease_token`, `expires_at`; fails `lease_held` if an unexpired
  lease exists; moves the issue `open -> in_progress`; epics are rejected
  with `validation_failed`
- `POST /v1/issues/{issue_id}/heartbeat` — body: `lease_token`,
  `ttl_seconds`; extends `expires_at`; appends no event
- `POST /v1/issues/{issue_id}/release` — body: `lease_token`; deletes the
  lease, moves `in_progress -> open` unless left `blocked`

## Notes, links, dependencies, events

- `POST /v1/issues/{issue_id}/notes` — append note (`author`, `body`)
- `GET  /v1/issues/{issue_id}/notes` — list
- `POST /v1/issues/{issue_id}/links` — link artifact (`artifact`,
  `relation`); `artifact` can be a UUID or a repository-relative path
- `DELETE /v1/issues/{issue_id}/links?artifact=&relation=&actor=` — remove a
  link; `artifact` is a UUID or repository-relative path, optional `relation`
  narrows to one relation (omit to remove all); `404 not_found` if absent
- `GET  /v1/issues/{issue_id}/links` — list linked artifacts
- `POST /v1/issues/{issue_id}/dependencies` — add dependency
  (`depends_on`, `kind`); rejects `blocks` cycles with `dependency_cycle`.
  Supported `kind` values: `blocks` (default), `parent`, `related`, `discovered-from`
- `DELETE /v1/issues/{issue_id}/dependencies/{depends_on}?kind=` — remove
- `GET  /v1/issues/{issue_id}/events` — activity timeline
