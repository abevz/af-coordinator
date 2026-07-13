# Review

Status: planning complete; implementation not started.

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

## Implementation Review Checklist

When the deferred tasks are completed, update this file with:

- migration and compatibility results;
- focused regression-test evidence per task;
- installed daemon/client verification after rebuild and restart;
- reconciliation of `afctl stats` against a sanitized fixture and live export;
- confirmation that lease tokens never appear in events, logs, reports, or
  examples;
- final decision on whether the packet and epic can be marked completed.
