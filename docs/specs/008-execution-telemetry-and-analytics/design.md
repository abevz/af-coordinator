# Design

## Boundaries

af-coordinator remains the control plane for issue state, dependencies, leases,
notes, and audit history. This packet improves that control-plane evidence; it
does not move runner process state, workflow retries, model telemetry, or CI
execution into the coordinator.

The implementation stays inside the current boundaries:

```text
cmd/afctl -> internal/client -> internal/api -> internal/core
                                      |              |
                                      v              v
                               internal/report  internal/store/sqlite
```

`internal/report` is a read-only aggregation layer over a narrow source
interface. It does not import SQLite and does not mutate coordinator state.
The API owns report semantics; CLI and future consumers render the same result.

## 1. Monotonic Event Sequence

The current `created_at` value has one-second precision, while event IDs are
random UUIDs. Ordering by `(created_at, id)` is deterministic but not causal.
Atomic note-plus-close operations therefore appear reversed in part of the
JSONL export.

Add an integer `sequence` to the event contract and make it the ordering key.
The safest SQLite migration is a replacement table whose row identity is an
integer sequence and whose public UUID remains unique:

```sql
create table events_v2 (
  sequence integer primary key autoincrement,
  id text not null unique,
  issue_id text references issues(id) on delete set null,
  actor text not null,
  event_type text not null,
  payload_json text not null default '{}',
  created_at text not null
);
```

Legacy rows are copied in deterministic `(created_at, id)` order. That cannot
recover causality already lost between tied timestamps, so the migration then
appends a system `event_ordering_enabled` marker. Its sequence is the exact
ordering cutoff exposed by statistics and export metadata.

New inserts omit `sequence`; SQLite allocates it in transaction order. Event
models, per-issue queries, JSONL export, and `GET /v1/events` order by sequence.
The global cursor encodes sequence. During one compatibility window, the server
accepts the old `(created_at, id)` cursor by resolving it to the migrated row;
unknown or malformed cursors return `validation_failed`.

## 2. Close And Reopen Paths

Split agent ownership actions from deliberate operator overrides.

### Agent close

`CloseIssue` requires all of the following:

- a current unexpired lease exists;
- the supplied token matches that lease;
- `expected_version` matches;
- the issue is non-terminal and the requested transition is valid.

The close transaction records the active `attempt_id`, optional note, structured
SCM metadata, previous status, resulting status, and close event. It then
removes the lease.

### Operator close and reopen

Add explicit local-socket API operations and CLI commands for operator close
and reopen. They require actor, expected version, and a non-empty reason, but no
lease. This resolves the existing unclaimable-epic gap without weakening agent
close. Because v1 trusts the local Unix-socket operator boundary, this packet
does not introduce RBAC; the distinct command and event make the override
visible and reviewable.

Generic issue update continues to handle metadata and non-terminal routing such
as park/unpark. Terminal reopen is removed from that generic path. Operator
events contain `from_status`, `to_status`, and `reason`.

## 3. Lease Attempts And Sessions

A lease episode becomes a first-class correlation unit without becoming a new
workflow engine.

- Claim generates `attempt_id` in the daemon.
- Claim optionally accepts `session_id` supplied by the caller.
- The live lease row stores both values.
- `issue_claimed` records attempt, session, TTL, and expiry.
- `issue_released`, `issue_closed`, and `lease_expired` record attempt and end
  reason.

When a claim replaces an expired row, the same transaction appends
`lease_expired` for the old attempt, deletes/replaces the lease, updates the
issue, and appends the new claim. Event sequence makes that order exact.

No separate historical attempt table is needed initially. The append-only event
stream is the durable history; the lease table remains current state. A future
read model can be added only if measured report cost justifies it.

Heartbeat continues to update only the live lease. It does not append an event.
The lease token remains excluded from every public model and event payload.

## 4. Atomic HANDOFF

Extend release semantics with an optional note in the request and implement an
explicit `afctl issue handoff` command. The handoff command validates a
`HANDOFF:` prefix and calls a single transaction that:

1. verifies the active lease token;
2. appends the note and `note_added` event;
3. ends the attempt with reason `handoff`;
4. removes the lease, updates issue status, and appends `issue_released`.

Sequence allocation preserves that order. Existing release remains available
for recovery and compatibility, while protocol docs route normal agent stops
through handoff.

## 5. Statistics Read Model

Add `internal/report` with a source interface for issues, events, notes,
dependencies, and artifact references. The SQLite store already exposes most
of these records through the normalized export boundary; the report layer
reuses those contracts instead of opening the database from CLI code.

Expose a versioned report from `GET /v1/stats` and render it with:

```text
afctl stats [--project <key>] [--repo <name>] [--since <RFC3339|duration>]
            [--until <RFC3339>] [--json]
```

The response contains:

- report window and data-quality cutoff;
- issue inventory and current ready/in-progress counts;
- created/closed throughput buckets;
- created-to-closed p50/p75/p90;
- attempt duration p50/p75/p90 and outcome counts;
- issues with multiple attempts and expiry/release counts;
- atomic HANDOFF coverage;
- note, spec-link, and structured close-metadata coverage.

Percentiles use completed records whose relevant timestamps fall inside the
selected window. The response includes sample size for every percentile.
Reopened issues use their latest terminal close for current lead-time reporting,
while transition counts remain visible separately.

The initial report scans coordinator records in memory. The current data set is
small, local, and single-node. Prometheus, rollup tables, dashboards, and remote
analytics are out of scope until a measured need exists.

## Delivery Order

1. `afc-49`: event sequence and cursor/export migration.
2. `afc-50`: close authorization plus explicit operator close/reopen.
3. `afc-51`: attempt/session lifecycle events, after event sequence.
4. `afc-52`: atomic HANDOFF, after sequence and close/lease semantics.
5. `afc-53`: report layer and CLI/API statistics after the event contract is
   trustworthy.

The implementation issues remain deferred until the operator chooses this
track over, or after, Aion Forge Harness v2 Phase 4.
