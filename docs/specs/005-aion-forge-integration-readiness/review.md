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
