# Requirements

## R1: Causal Event Order

- Every newly appended event SHALL receive a daemon-assigned monotonic integer
  sequence in the same transaction as the domain mutation.
- Per-issue timelines, the global event feed, JSONL export, and pagination
  cursors SHALL use that sequence as their canonical order.
- Existing events SHALL be migrated into a deterministic legacy order. The
  system SHALL expose a cutoff after which event order is causally exact and
  SHALL NOT present tied legacy timestamps as exact causal history.
- A cursor created by the currently installed client SHALL either remain
  readable during the compatibility window or fail with an explicit typed
  validation error. It SHALL NOT silently skip or duplicate events.

## R2: Close And Reopen Authorization

- The ordinary agent close path SHALL require an active, unexpired lease, the
  matching lease token, and the expected issue version.
- Ordinary close SHALL reject an already terminal issue and SHALL validate the
  source-to-target status transition before changing state.
- Closing an unclaimable epic or performing another deliberate administrative
  close SHALL use an explicit operator command/API path with an actor, expected
  version, reason, and distinct audit event. It SHALL NOT rely on a dummy lease
  token.
- Reopening `done` or `cancelled` work SHALL use an explicit operator command/API
  path with a reason and distinct `issue_reopened` event. Generic metadata
  update SHALL NOT hide a terminal-to-open transition inside `issue_updated`.
- Transition events SHALL include the previous and resulting status without
  copying descriptions, secrets, or other unbounded issue content.

## R3: Lease-Attempt Telemetry

- Each successful claim SHALL create a unique `attempt_id`. The daemon SHALL
  return it and include it in lifecycle event payloads.
- A caller MAY supply a non-secret `session_id` for correlation. Actor identity,
  session identity, and lease-attempt identity SHALL remain separate concepts.
- Claim events SHALL record `attempt_id`, lease TTL, and expiry. Release, close,
  and expiry events SHALL record `attempt_id` and an end reason.
- Replacing a lazily expired lease SHALL append one `lease_expired` event for
  the old attempt before appending the new claim event.
- Lease tokens SHALL never enter events, reports, logs, notes, or exported
  attempt metadata.
- Heartbeats SHALL remain non-events to avoid audit-log flooding. Current lease
  timestamps MAY be updated for diagnostics.
- Lease attempts SHALL remain coordinator ownership records, not replicas of
  Temporal/Aion Forge workflow attempts.

## R4: Atomic HANDOFF

- The API SHALL support adding a HANDOFF note and releasing the active lease in
  one transaction.
- `afctl` SHALL expose an explicit handoff command or equivalent `release
  --note` flow that requires a non-empty note beginning with `HANDOFF:`.
- The event sequence SHALL show `note_added` before `issue_released` for the
  atomic operation.
- A failed note write or failed release SHALL roll back the whole handoff.
- The existing bare release primitive MAY remain for operator recovery, but the
  documented agent protocol SHALL use the atomic handoff path.

## R5: Project Statistics

- `afctl stats` SHALL support project, repository, and time-window filters and
  SHALL provide both human-readable and stable JSON output.
- The report SHALL include inventory/status counts, ready and in-progress work,
  throughput, created-to-closed percentiles, lease-attempt duration and churn,
  release/expiry outcomes, HANDOFF compliance, note coverage, spec-link
  coverage, and structured SCM close-metadata coverage.
- Every duration and ratio SHALL document its numerator, denominator, window,
  and treatment of cancelled, deferred, reopened, and legacy events.
- Reports that include pre-sequence legacy history SHALL expose the ordering
  limitation instead of implying exact attempt reconstruction.
- The first implementation SHALL derive results from coordinator records. It
  SHALL NOT add a mutable analytics database, Prometheus dependency, or remote
  service.
- Statistics SHALL describe system flow. The product SHALL NOT add agent
  leaderboards, productivity scores, or task-count targets.

## R6: Compatibility, Security, And Verification

- New schema migrations SHALL run through the existing embedded migration path
  and SHALL be tested against databases created from the real migration set.
- Existing JSON fields SHALL remain compatible unless the API change is
  explicitly versioned and documented.
- API handlers, store behavior, report aggregation, cursor behavior, and CLI
  rendering SHALL have focused regression tests.
- Documentation SHALL update API, schema, architecture, workflow, protocol, and
  export contracts together with implementation.
- Core operation and reporting SHALL remain local-first and offline-capable.
