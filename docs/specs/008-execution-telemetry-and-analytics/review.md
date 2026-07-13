# Review

Status: in progress; `afc-49` completed.

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

## Implementation Review Checklist

When the deferred tasks are completed, update this file with:

- migration and compatibility results;
- focused regression-test evidence per task;
- installed daemon/client verification after rebuild and restart;
- reconciliation of `afctl stats` against a sanitized fixture and live export;
- confirmation that lease tokens never appear in events, logs, reports, or
  examples;
- final decision on whether the packet and epic can be marked completed.
