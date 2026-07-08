# Review

Status: active

## 2026-07-07 - Packet initialized

### What shipped

- Created the Aion Forge integration readiness SDD packet.
- Moved active packet guidance away from completed packet 003.
- Updated packet 003 README status to match its completed review outcome.

### What was verified

- `docs/roadmap.md` lists epic `afc-24` and ready issues `afc-25` through
  `afc-30` as the planned work.
- `docs/specs/003-beads-migration/review.md` declares packet 003 completed.

### Open items

- Implementation tasks `afc-25` through `afc-30`, plus `afc-36`, remain open.
- Each implementation task must update this review with shipped behavior,
  verification, and remaining risks.

## 2026-07-07 - Task IDs aligned with af-coordinator

### What shipped

- Replaced the previous parallel spec-task names with the live `afc-N` issue IDs
  used by the coordinator.
- Added `afc-36` for scoped repository resolution, which was previously present
  in this packet but missing from the live `afc` issue set.
- Kept `afc-37` as the cleanup task for this alignment work.

### What was verified

- `aion-forge` uses the same convention in `docs/specs/010-harness-v2`: task
  IDs are the af-coordinator short IDs, with one ID scheme rather than a
  parallel spec-task namespace.
- `afc-36` and `afc-37` exist in the live coordinator.
- `afc-30`, `afc-36`, and `afc-37` are parent-linked to `afc-24`.
- `afc-30` priority is P1 to match the packet ordering.

### Open items

- Keep `tasks.md` aligned with `afctl issue list --project afc` when new leaves
  are added or closed.

## 2026-07-07 - Contract fixes for dependency identity and repo scoping

### What shipped

- Fixed dependency responses so they expose explicit UUID and short-id fields:
  `issue_id`, `issue_short_id`, `depends_on_id`, and
  `depends_on_short_id`.
- Scoped issue repository resolution by project when project context is known,
  while preserving exact UUID lookup.
- Rejected ambiguous unscoped repository logical-name lookups with a validation
  error instead of silently selecting one repository.
- Updated the full `afctl issue get --full` output to show dependency short IDs
  and UUIDs together.
- Moved repository/worktree resolution ahead of `CreateIssue` transaction start
  so single-connection SQLite test/server setups do not deadlock on repo
  lookups.

### What was verified

- `go test ./internal/store/sqlite ./internal/api ./cmd/afctl`
- `go test ./...`
- `make build`
- Added store regression tests for:
  - explicit dependency identifier fields on `GetIssue`
  - ambiguous unscoped repository logical names
  - project-scoped repository resolution in `CreateIssue`, `ListIssues`, and
    ready-issue filtering
- Added API regression tests for:
  - explicit dependency identifier fields on `GET /v1/issues/{issue_id}`
  - project-scoped repository resolution on `GET /v1/issues/ready`
- Added a CLI regression test for `afctl issue get --full` dependency output.

### Open items

- `afc-25`, `afc-26`, `afc-27`, `afc-28`, and `afc-29` remain open in this
  packet.

## 2026-07-08 - afc-25 global events watch API

### What shipped

- Added `GET /v1/events` as a global watch/list endpoint over the append-only
  events table.
- Defined an opaque `since` cursor contract returned as `next_since`, ordered
  by `(created_at, id)` without exposing raw query semantics to clients.
- Added bounded long-polling via `wait_ms`, with a maximum wait of 30000 ms,
  plus a `limit` parameter capped at 500.
- Kept the existing issue-scoped `GET /v1/issues/{issue_id}/events` endpoint
  unchanged.
- Added a client wrapper for the global watch API and refreshed the API docs.

### What was verified

- `go test ./internal/store/sqlite ./internal/api ./internal/client`
- Added store regression tests for:
  - global cursor pagination across multiple events
  - invalid `since` cursor rejection
- Added API regression tests for:
  - `GET /v1/events` initial page and follow-up page behavior
  - invalid `since` cursor rejection
  - bounded long-poll timeout with an empty result set
- Verified watched `payload_json` stays valid JSON in the API contract.

### Open items

- `afc-26`, `afc-27`, `afc-28`, and `afc-29` remain open in this packet.

## 2026-07-08 - afc-26 external issue references

### What shipped

- Added optional issue-level `external_key` storage for mirrored tracker keys
  and external workflow identifiers.
- Added migration `0004_issue_external_key.sql` because migration `0003` was
  already occupied by acceptance criteria.
- Exposed `external_key` through create/get/update/list API surfaces, store
  models, and `afctl` create/update/list flows.
- Added exact-match `external_key` filtering on `GET /v1/issues`.
- Switched `issue_created` event payload generation to typed JSON marshaling so
  titles and external keys with special characters remain valid JSON.

### What was verified

- `go test ./internal/store/sqlite ./internal/api ./internal/client ./cmd/afctl`
- Added store regression tests for:
  - create/get round-trip of issue `external_key`
  - exact-match issue listing by `external_key`
  - update event payload change tracking for `external_key`
  - valid JSON `issue_created` payloads with quoted titles and external keys
- Added API regression tests for:
  - `POST /v1/issues` returning `external_key`
  - `GET /v1/issues?external_key=` exact-match filtering
  - `PATCH /v1/issues/{issue_id}` updating `external_key`
- Added client regression coverage for URL-encoded `external_key` query values.
- Added CLI output coverage for displaying issue `external_key`.

### Open items

- `afc-27`, `afc-28`, and `afc-29` remain open in this packet.

## 2026-07-08 - afc-27 structured close metadata

### What shipped

- Extended `POST /v1/issues/{issue_id}/close` to accept structured close
  metadata: `branch`, `pr_url`, and `commit_sha`.
- Changed the close API response to return the structured close metadata,
  `resolution`, and `closed_at` instead of a bare status string.
- Persisted structured close metadata in the `issue_closed` event payload.
- When an issue already has an `external_key`, included it in the close response
  and `issue_closed` event so the audit trail points to the external execution
  reference as well.
- Switched `issue_closed` event payload generation to typed JSON marshaling.
- Updated agent protocol and workflow examples so common close commands show the
  structured refs explicitly.

### What was verified

- `go test ./internal/store/sqlite ./internal/api ./internal/client ./cmd/afctl`
- Added store regression tests for:
  - structured close metadata in the returned close result
  - valid JSON `issue_closed` payloads containing `branch`, `pr_url`, and
    `commit_sha`
  - inclusion of issue `external_key` in close result and event payload
- Added API regression tests for:
  - `POST /v1/issues/{issue_id}/close` returning structured close metadata and
    `closed_at`
- Added client regression coverage for structured close responses.

### Open items

- `afc-28` and `afc-29` remain open in this packet.

## 2026-07-08 - afc-28 MCP wrapper over daemon API

### What shipped

- Added `afc-mcp`, a small stdio MCP server wrapper over the daemon API.
- Kept the wrapper thin by routing every tool through `internal/client`; the
  wrapper does not access SQLite directly.
- Exposed a focused tool surface for coordinator agent workflows:
  `health`, `get_issue`, `list_ready_issues`, `claim_issue`,
  `heartbeat_issue`, `add_note`, `list_notes`, `list_issue_events`, and
  `close_issue`.
- Added MCP docs plus repo layout references, and installed `afc-mcp` through
  `make build-install`.

### What was verified

- `go test ./internal/mcp ./internal/client ./cmd/afc-mcp`
- `go test ./...`
- `go build -buildvcs=false ./...`
- `make build-install`
- Installed-binary scratch verification of `afc-mcp` over stdio framing:
  - `initialize`
  - `tools/call` for `list_ready_issues`
  - `tools/call` for `claim_issue`
- Added MCP regression tests for:
  - `initialize`
  - `tools/list`
  - structured tool dispatch for `claim_issue` and `close_issue`
  - stdio message framing with `Content-Length`

### Open items

- `afc-29` remains open in this packet.
