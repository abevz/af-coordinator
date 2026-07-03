# Requirements

## Problem

Concurrent AI agents need a local coordination backend that is more reliable
than shared Dolt server workflows and that still fits a local-first homelab
environment.

The system must coordinate work across:

- many projects
- many repositories
- many worktrees
- repositories with multiple remotes
- spec-first development flows

## In scope

- local daemon as single write authority
- SQLite WAL storage
- issue lifecycle with claims, leases, and notes
- explicit project/repository/worktree identity
- SDD artifact registration and issue linkage
- CLI client for basic operations

## Out of scope

- distributed clustering
- internet-backed source of truth
- web UI
- plugin runtime
- direct DB writes from agents

## Functional requirements

- THE system SHALL expose a local API for all write operations.
- THE system SHALL prevent concurrent writers from mutating task state directly
  in the database.
- THE system SHALL support projects, repositories, repo remotes, and worktrees
  as separate identity layers.
- THE system SHALL support registering repository artifact roots such as
  `docs/specs/`.
- THE system SHALL support registering spec artifacts such as
  `requirements.md`, `design.md`, `tasks.md`, `review.md`, and ADR files.
- THE system SHALL support linking issues to one or more registered artifacts.
- WHEN an issue is created, THE system SHALL allocate a unique short id of the
  form `<project_key>-<N>` from a per-project counter.
- WHEN an issue is claimed, THE system SHALL create a lease token with expiry.
- THE system SHALL treat "claimed" as derived from an unexpired lease, not as
  a stored issue status.
- WHEN an issue with an unexpired lease is mutated, THE system SHALL require
  both a valid lease token and the expected row version.
- WHEN an unclaimed issue's metadata is edited, THE system SHALL require the
  expected row version.
- WHEN a lease expires, THE system SHALL allow the issue to be claimed again
  without operator intervention.
- WHEN a `blocks` dependency would create a cycle, THE system SHALL reject it.
- THE system SHALL append an event record for important state changes, and
  SHALL NOT append events for lease heartbeats.
- THE system SHALL NOT hard-delete issues or projects in v1; terminal issue
  states are `done` and `cancelled`.
- THE system SHALL store all timestamps as RFC 3339 UTC text.

## Non-functional requirements

- Reliability: local recovery must be simpler than shared SQL/Dolt server recovery
- Performance: single-node laptop or homelab operation must be sufficient
- Operability: systemd user service must be a supported runtime model
- Auditability: important changes must be reconstructable from the event log
- Offline use: core operation must not require internet access

## Acceptance criteria

- WHEN a repository has multiple worktrees, THE system SHALL distinguish them
  without collapsing identity to one path.
- WHEN two agents try to update the same issue, THEN the second update SHALL
  fail with a conflict if the version or lease token is stale.
- WHEN an issue references an SDD task, THE system SHALL be able to link it to
  the exact tracked artifact path.
- WHILE the daemon is healthy, clients SHALL not need direct SQLite access.
