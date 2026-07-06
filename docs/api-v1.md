# API v1

HTTP+JSON over the unix socket. The daemon is the single write authority;
this contract is the product surface. `afctl` and agent wrappers are thin
clients over these endpoints.

Test shape:

```text
curl --unix-socket ~/.local/state/af-coordinator/af-coordinator.sock \
  http://localhost/v1/health
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
- `GET  /v1/artifacts?repo=&kind=` — list

## Issues

- `POST /v1/issues` — create; daemon allocates `short_id`; body includes
  `project`, `scope_kind`, optional `repo`/`worktree`, `title`,
  `description`, `acceptance_criteria`, `priority`, `issue_type`
  (`task` default, `bug`, `feature`, `epic`, `chore`)
- `GET  /v1/issues/{issue_id}` — fetch one, including current lease if any
- `GET  /v1/issues?project=&repo=&worktree=&status=&assignee=&type=` — query
- `GET  /v1/issues/ready?project=&repo=` — computed ready view; excludes
  epics (they are containers, not units of work)
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

