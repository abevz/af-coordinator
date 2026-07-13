# Review

Status: in progress; `afc-49` and `afc-50` completed.

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

There are no unresolved design blockers for starting `afc-49` or `afc-50` once
the operator chooses this track.

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

## Implementation Review Checklist

When the deferred tasks are completed, update this file with:

- migration and compatibility results;
- focused regression-test evidence per task;
- installed daemon/client verification after rebuild and restart;
- reconciliation of `afctl stats` against a sanitized fixture and live export;
- confirmation that lease tokens never appear in events, logs, reports, or
  examples;
- final decision on whether the packet and epic can be marked completed.
