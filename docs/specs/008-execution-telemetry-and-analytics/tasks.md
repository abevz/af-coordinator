# Tasks

Task IDs are live af-coordinator short IDs in project `afc`. This file owns
scope and dependency intent; use `afctl issue get afc-<N> --full` for current
status, claims, notes, links, and closure audit.

The operator selected this track on 2026-07-13. `afc-49` is complete; later
implementation tasks remain deferred until their dependency order is reached.

| ID | Type | Pri | Status | Title | Depends on |
|----|------|-----|--------|-------|------------|
| `afc-47` | epic | P1 | open | Execution telemetry and analytics hardening | - |
| `afc-48` | chore | P1 | in_progress | Specify execution telemetry and analytics track | parent `afc-47` |
| `afc-49` | bug | P1 | done | Preserve causal event order with monotonic sequence | `afc-48` |
| `afc-50` | bug | P1 | deferred | Enforce close authorization and terminal transitions | `afc-48` |
| `afc-51` | feature | P2 | deferred | Record execution attempts and lease outcomes | `afc-48`, `afc-49` |
| `afc-52` | feature | P2 | deferred | Add atomic HANDOFF and release flow | `afc-48`, `afc-49`, `afc-50` |
| `afc-53` | feature | P2 | deferred | Add project execution statistics report | `afc-48`, `afc-49`, `afc-50`, `afc-51`, `afc-52` |

All child tasks also carry a `parent` relationship to `afc-47`.

## `afc-48`: Specify The Track

Scope:

- capture the live baseline and its limitations;
- define requirements, boundaries, migration strategy, task slices, and
  verification expectations;
- create the live epic/task graph and link it to this packet;
- keep implementation tasks deferred until the operator chooses the track.

Verification:

- packet contains README, requirements, design, tasks, review, and evidence;
- every live task links to the packet;
- live dependency graph matches the delivery order above;
- docs do not claim that implementation has shipped.

## `afc-49`: Preserve Causal Event Order

Scope:

- add and migrate a monotonic event sequence;
- preserve a deterministic legacy order and expose the exact-order cutoff;
- update event models, per-issue timeline, global cursor/watch API, and JSONL
  export to use sequence;
- retain a bounded old-cursor compatibility path;
- update schema/API/export documentation.

Verification:

- migration tests use the real embedded migration set;
- same-second events retain insertion order;
- note-before-close and expiry-before-reclaim order have regression tests;
- cursor pagination has no duplicates or gaps across tied timestamps;
- JSONL export is deterministic and sequence ordered.

## `afc-50`: Enforce Close And Terminal Transitions

Scope:

- require an active matching lease for ordinary close;
- reject expired, missing, stale, and already-terminal closes;
- apply transition validation and include from/to status in audit events;
- add explicit operator close for unclaimable epics/admin resolution;
- add explicit operator reopen with reason and distinct event;
- remove hidden terminal reopen from generic update.

Verification:

- regression tests cover no lease, expired lease, wrong token, duplicate close,
  cancelled-to-done, explicit reopen, and operator epic close;
- API, client, MCP behavior, CLI exit codes, and docs agree;
- no operator path accepts or emits a fake lease token.

## `afc-51`: Record Lease Attempts And Outcomes

Scope:

- generate and return `attempt_id` on claim;
- accept optional non-secret `session_id` without changing actor semantics;
- store attempt/session on the current lease;
- enrich claim/release/close events and append `lease_expired` on lazy reclaim;
- keep heartbeats out of the event stream and tokens out of all evidence.

Verification:

- claim, release, close, expiry, reclaim, and heartbeat store/API tests;
- concurrent claim test still produces exactly one winner;
- event payload tests assert useful fields and absence of lease tokens;
- old clients remain compatible with omitted session IDs.

## `afc-52`: Add Atomic HANDOFF

Scope:

- add atomic note-plus-release store/API/client behavior;
- expose `afctl issue handoff` and JSON output;
- validate the required `HANDOFF:` note contract;
- update protocol and both byte-identical protocol document copies;
- preserve bare release for recovery/compatibility.

Verification:

- rollback tests prove no partial note or release;
- sequence tests prove `note_added` precedes `issue_released`;
- wrong/expired token and empty/malformed note tests;
- CLI and MCP coverage if the MCP surface exposes release/handoff;
- protocol copies remain byte-identical.

## `afc-53`: Add Project Statistics

Scope:

- add the read-only report package and versioned response model;
- add `/v1/stats`, client support, and `afctl stats` filters/rendering;
- compute the metrics and data-quality fields from requirements;
- document definitions, denominators, legacy cutoff, and examples;
- avoid rankings, external telemetry systems, and mutable rollups.

Verification:

- table-driven aggregation fixtures cover empty, legacy, reopened, cancelled,
  deferred, multi-attempt, expired, handoff, and SCM/spec coverage cases;
- API/client/CLI JSON contract tests;
- human output snapshot or focused rendering tests;
- report results reconcile with a sanitized export fixture;
- `go test ./...` and `go build -buildvcs=false ./...` pass.
