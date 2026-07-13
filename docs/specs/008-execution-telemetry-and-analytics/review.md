# Review

Status: complete; `afc-49` through `afc-53` completed.

## Planning Review

- Live daemon and client were healthy and version matched before task creation.
- The baseline came from the normalized API/CLI export, not direct SQLite
  access.
- Source inspection confirmed the event ordering and close authorization gaps
  described in `evidence.md`.
- The task graph separates audit correctness, ownership telemetry, workflow
  ergonomics, and reporting instead of shipping them as one large change.
- Implementation issues are deferred, so this planning track does not silently
  pre-empt Aion Forge Harness v2 Phase 4.

## Design Conclusions

- Event sequence is a prerequisite for transition-sensitive analytics.
- Legacy tied timestamps can be made deterministic but not retrospectively
  causal; reports must expose that boundary.
- Agent close and operator close/reopen need separate explicit paths.
- A lease attempt is coordinator ownership evidence, not workflow execution
  state.
- HANDOFF plus release should be one transaction.
- The first report should be a local derived read model, not a new telemetry
  stack or mutable analytics store.

There are no unresolved implementation or design blockers in this packet.

## `afc-49` — Preserve Causal Event Order

### What shipped

- Migration `0005_event_sequence.sql` rebuilds `events` with a monotonic
  `sequence` primary key, deterministically migrates legacy rows, and appends
  an `event_ordering_enabled` cutoff marker only when legacy history exists.
- Timelines, global pagination, and JSONL export use sequence order; event
  responses now expose `sequence`.
- New `v2` cursors encode the sequence. Existing `v1` timestamp/ID cursors are
  resolved to the migrated event for the compatibility window and unknown
  cursors fail with `validation_failed` instead of skipping data.
- API, schema, architecture, workflow, protocol, and export documentation now
  describe the ordering contract. The two protocol document copies remain
  byte-identical.

### What was verified

- `go test ./...`
- Migration regression: real embedded pre-0005 migrations, deterministic
  same-second legacy ordering, and cutoff marker.
- Store, API, and export regressions for same-second order, pagination without
  duplicates, v1 cursor compatibility, rejected unknown v2 cursors, and JSONL
  sequence order.

## `afc-50` — Enforce Close And Terminal Transitions

### What shipped

- Ordinary `issue close` now verifies the expected version, requires an active
  matching lease token in the same transaction, and rejects missing, expired,
  stale, and terminal closes.
- Generic issue update cannot move work into a terminal state or reopen it;
  status-changing audit events record `from_status` and `to_status`.
- Explicit local operator close/reopen API, client, CLI, and MCP paths require
  actor, expected version, and a non-empty reason. They are tokenless by
  contract, emit `issue_operator_closed` or `issue_reopened`, and support
  closing unclaimable epics.
- API, schema, architecture, workflow, curl examples, and the byte-identical
  agent-protocol copies describe the split between agent and operator paths.

### What was verified

- Store regressions for missing/expired/wrong leases, already-terminal close,
  blocked generic terminal transitions, operator epic close, and explicit
  reopen.
- API, client, MCP, and CLI regressions verify explicit operator paths and
  reject fake lease-token fields or flags.
- `go test ./...`, `go build -buildvcs=false ./...`, and `make test` (race).
- Scratch daemon end-to-end flow: lease-bound close, duplicate rejection,
  operator epic close/reopen, and tokenless operator CLI rejection.
- Installed through `make restart-service`; health returned `ok`, and a
  non-mutating installed-binary operator-route probe returned typed
  `not_found` as expected.

## `afc-51` — Record Lease Attempts And Outcomes

### What shipped

- Migration `0006_lease_attempts.sql` adds `attempt_id` and `session_id` to
  the current lease. Existing live leases are deterministically backfilled as
  `legacy-<issue-id>` before a unique attempt index is created.
- Each successful claim generates and returns a non-secret `attempt_id`; callers
  may provide an optional non-secret `session_id` without altering actor
  ownership semantics. API, client, CLI, MCP, schema, and protocol surfaces
  expose the compatible optional session field.
- Claim, release, ordinary close, and lazy expiry/reclaim events carry attempt
  outcome evidence. Lazy reclaim records `lease_expired` for the old attempt
  before appending the replacement claim; heartbeats do not append events.
- Event payloads deliberately exclude lease tokens. The active lease exposes
  attempt/session identifiers only, never the secret token.

### What was verified

- Store, API, client, MCP, CLI, migration, and compatibility regressions cover
  claim/release/close/expiry/reclaim/heartbeat behavior, omitted session IDs,
  and absence of lease tokens from public evidence.
- Concurrent claim regression verifies exactly one winner and one durable claim
  event.
- `go test ./...`, `go build -buildvcs=false ./...`, `make test` (race), and
  `git diff --check` pass; the two protocol copies remain byte-identical.
- Scratch-daemon flow verified attempt/session persistence, close and release
  outcomes, no-event heartbeat, and ordered lazy reclaim. The installed daemon
  was rebuilt and restarted successfully; a read-only live check confirmed
  migration backfill and no public lease token.

## `afc-52` — Add Atomic HANDOFF

### What shipped

- `POST /v1/issues/{issue_id}/handoff` records a required `HANDOFF:` note and
  releases the active lease in one SQLite transaction. The store derives the
  note author from the lease holder and rejects missing, wrong, or expired
  leases without changing state.
- The durable sequence is `note_added` followed by `issue_released`; the
  release records the existing attempt with `end_reason: handoff`, while lease
  tokens remain absent from notes, public results, and event payloads.
- API, client, `afctl issue handoff`, MCP `handoff_issue`, curl/schema/
  architecture docs, and both byte-identical agent protocol copies expose the
  same contract. Bare release remains available only for recovery and
  compatibility.

### What was verified

- Store regressions cover successful lifecycle ordering, malformed/wrong/
  expired credentials, and forced note-write or lease-delete failures with no
  partial note, event, status, or lease mutation.
- API, client, MCP, and CLI regressions validate the endpoint, request shape,
  required `HANDOFF:` prefix, and MCP forwarding.
- `go test ./...`, `go build -buildvcs=false ./...`, `make test` (race),
  scratch-daemon flow, and installed-service verification passed; protocol
  copies remain byte-identical.

## `afc-53` — Add Project Statistics

### What shipped

- Added the versioned, read-only `internal/report` aggregation layer and
  `GET /v1/stats`. It derives inventory, throughput, lead-time percentiles,
  attempt duration/churn/outcomes, HANDOFF compliance, note/spec/SCM coverage,
  and legacy-ordering data quality directly from coordinator records.
- Added `client.GetStats` plus `afctl stats` with project, repository, RFC 3339
  or duration `since`, RFC 3339 `until`, stable JSON, and concise human
  rendering. The report owns no database connection, mutable rollup, remote
  dependency, productivity score, or agent ranking.
- Defined report window, nearest-rank percentile, ratio, snapshot-coverage,
  reopened, cancelled, deferred, and legacy-cutoff treatment in API, schema,
  architecture, workflow, operations, curl, protocol, and export-facing docs.
  The two protocol document copies remain byte-identical.

### What was verified

- Table-driven aggregation fixtures cover empty data, legacy cutoff, reopened
  and cancelled work, deferred inventory, multi-attempt churn, expiry,
  HANDOFF, SCM/spec coverage, ambiguous filters, and a sanitized normalized
  export fixture.
- API, client, CLI parser/JSON path, and human-rendering regressions cover the
  versioned endpoint, filters, invalid windows, stable report decoding, and
  visible metric/data-quality output.
- `go test ./...`, `go build -buildvcs=false ./...`, `make test` (race), and
  `git diff --check` passed.
- `make build-install` and `make restart-service` installed the daemon and
  CLI. The active user service passed `afctl health` (version `0055`) and both
  JSON and human `afctl stats --project afc --since 24h` reads against live
  coordinator data without mutation.
- The report response and fixtures contain no lease tokens or note bodies.

## Implementation Review Checklist

All packet tasks are complete. The parent epic may be closed through the
explicit local operator path after the `afc-53` closure audit is recorded.
