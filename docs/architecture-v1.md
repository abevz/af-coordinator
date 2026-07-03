# Architecture v1

## Objective

Build a local coordination backend for multiple AI agents that need safe
concurrent access to shared task state across many projects, repositories, and
worktrees.

## Design principles

- one daemon is the only writer
- clients use an API, not direct DB access
- task ownership is lease-based
- all updates use optimistic concurrency
- local operation must work without internet
- GitHub or other trackers can be mirrors, not the primary source of truth
- SDD artifacts remain the source of truth for feature scope and design
- execution state must link back to exact spec artifacts instead of duplicating them
- borrow proven task semantics from Beads where they improve UX and operator flow
- do not inherit Beads shared-Dolt write-path assumptions

## Transport

Primary transport:

- HTTP+JSON over Unix socket

Optional later:

- localhost TCP for debugging

Reasoning:

- easy to test with `curl --unix-socket`
- simple for shell wrappers and agent tools
- no need for gRPC complexity in v1

## Storage

Primary storage:

- SQLite in WAL mode

Settings expected for v1:

- `journal_mode=WAL`
- `synchronous=NORMAL`
- `busy_timeout`
- foreign keys on

Connection model:

- one dedicated write connection; all mutations serialize through it
- a small pool of read-only connections for queries
- this matches SQLite's single-writer reality instead of fighting it

Driver decision:

- `modernc.org/sqlite` (pure Go, no cgo)
- fits the stdlib-first rule in `AGENTS.md`, keeps cross-compilation and
  static builds trivial
- `mattn/go-sqlite3` is the fallback if a profiled hot path ever demands cgo
  performance, which is not expected at this scale

Migrations:

- plain SQL files in `migrations/`, applied in filename order
- embedded migration runner in the daemon (stdlib only), tracking applied
  migrations in a `schema_migrations` table

Reasoning:

- robust enough for a single local daemon
- transactional
- simple backup story
- much lower operational cost than a separate SQL server

## Process model

```text
many clients -> one daemon -> one SQLite database
```

Clients:

- `afctl`
- agent wrappers
- future editor integrations

Daemon responsibilities:

- validation
- transactional writes
- lease management
- dependency checks
- event creation
- export endpoints

## SDD integration

This project uses a spec-first workflow.

For repository development, the canonical flow is:

```text
requirements.md -> design.md -> tasks.md -> implementation -> review.md
```

That creates a clean boundary:

- spec artifacts own intent, scope, acceptance criteria, and design
- `af-coordinator` owns execution state, claims, blockers, notes, and events

This is important because the coordinator is itself a task system. Without an
explicit boundary, the project would drift into storing design intent inside
issue descriptions and lose the benefits of SDD.

### Repository-level SDD layout

Recommended v1 convention:

```text
docs/specs/001-foundation/
docs/specs/002-next-track/
...
```

Each packet contains:

- `README.md`
- `requirements.md`
- `design.md`
- `tasks.md`
- `review.md`

### Coordinator support for SDD

The product should model SDD artifacts directly enough to answer:

- which spec packet owns this issue
- which `tasks.md` entry is being executed
- which review artifact closes the loop
- which repository/worktree contains the authoritative file

So the domain model needs explicit artifact registration and issue-to-artifact
links, not just free-form text fields.

## Beads-inspired product semantics

Beads is a useful source of task-system ideas even though its shared-Dolt server
is not the right backend for this workload.

### Concepts to adopt

- tasks as first-class objects
- blocking dependencies
- computed `ready` state instead of only stored status
- short stable ids
- per-issue notes and activity trail
- query-oriented CLI ergonomics

These are product semantics and UX patterns. They are worth keeping.

### Concepts not to adopt

- shared Dolt server as the coordination hot path
- auto-sync or auto-push in the mutation path
- multi-agent mutation through shell commands against a VCS-backed shared store

These are operational choices that became fragile under concurrent-agent load.

### Resulting design stance

The intended model for `af-coordinator` is:

```text
Beads-inspired task semantics
        +
single-writer daemon
        +
SQLite WAL
```

This keeps the useful workflow ideas while replacing the fragile write path.

## Identity model

### Project

Top-level initiative. Can span many repositories.

Examples:

- `aion-forge`
- `utils`
- `platform-iac`

### Repository

Logical repository identity. Not just a local checkout path.

Suggested identity sources:

- canonical git dir
- normalized hosting slug when available
- logical name within the project

### Repo remote

Each repository may have many remotes:

- `origin`
- `upstream`
- fork remotes
- custom mirrors

These must be stored explicitly, not inferred ad hoc from one worktree.

### Worktree

Concrete checkout path on disk.

Tracks:

- absolute path
- branch
- HEAD commit
- selected remote
- selected remote branch
- whether it is the main checkout or an ephemeral worktree

### Artifact

An SDD or design artifact inside a repository.

Examples:

- `docs/specs/001-foundation/requirements.md`
- `docs/specs/001-foundation/design.md`
- `docs/specs/001-foundation/tasks.md`
- `docs/specs/001-foundation/review.md`
- `docs/specs/001-foundation/decisions/ADR-001-*.md`

Artifacts should be tracked by:

- repository
- optional worktree observation source
- artifact kind
- relative path
- optional external key or task id

### Issue scope

Issues should support these scopes:

- project-scoped
- repository-scoped
- worktree-scoped

Default expectation:

- most work is repository-scoped
- some operational tasks are worktree-scoped

### Issue identity

Every issue has two ids:

- `id`: opaque internal primary key
- `short_id`: human-facing stable id, format `<project_key>-<N>`,
  for example `afc-42`

The daemon allocates `short_id` from a per-project counter inside the
issue-create transaction, so ids are short, dense, and race-free. All CLI
and API surfaces accept the short id.

### Issue to artifact linkage

An issue may reference one or more artifacts, for example:

- implements `design.md`
- executes task from `tasks.md`
- blocked by missing `requirements.md`
- verified by `review.md`

This keeps operational coordination aligned with the spec canon.

## Concurrency model

### Claim leases

Claiming an issue must create a lease:

- `lease_token`
- `holder`
- `expires_at`

There is no stored `claimed` status. "Claimed" is a derived property: an
issue is claimed if and only if it has an unexpired lease. This keeps one
source of truth and removes the possibility of a status stuck at claimed
after its lease died.

While an unexpired lease exists, only its holder may mutate the issue.

### Lease expiry

Expiry is lazy: any lease with `expires_at` in the past is treated as
absent by every check (claim, mutation, ready). No background process is
required for correctness.

An optional background sweeper may delete expired lease rows and append a
single `lease_expired` event per lease for the activity timeline.

### Heartbeats

Long-running work should periodically extend the lease.

Heartbeats update only `leases.expires_at` and `leases.updated_at`. They
never append events; otherwise a lease renewed every few seconds would
flood the audit log.

If heartbeat stops:

- lease expires
- issue becomes claimable again

### Mutation matrix

Not every operation needs the full proof. Requiring a lease for everything
would make an unclaimed issue uneditable.

| Operation | Requires |
|---|---|
| status transition, close, edits under an active claim | `lease_token` + `expected_version` |
| metadata edit of an unclaimed issue (title, priority, description) | `expected_version` only |
| append note, link artifact | nothing (append-only) |

In all cases: if someone else holds an unexpired lease, mutation is
rejected. Every successful mutation increments `version` and appends an
event in the same transaction.

### Optimistic concurrency

Every mutable row has `version`.

Mutating requests include `expected_version` (plus `lease_token` where the
matrix requires it). If the version mismatches, return conflict and require
reread.

## Issue state machine

Recommended states for v1:

- `open`
- `in_progress`
- `blocked`
- `done`
- `cancelled`

Basic transitions:

```text
open -> in_progress      (claim)
in_progress -> open      (release / lease expiry sweep)
in_progress -> blocked
blocked -> in_progress
blocked -> open
in_progress -> done
any -> cancelled
done -> open             (reopen)
cancelled -> open        (reopen)
```

Claim creates the lease and moves `open -> in_progress` in one transaction.
Release deletes the lease and moves `in_progress -> open` unless the issue
was explicitly left `blocked`.

## Dependency kinds

- `blocks`: the only kind that affects ready computation
- `parent`: hierarchy
- `related`: informational
- `discovered-from`: provenance of follow-up work

The daemon must reject any `blocks` edge that would create a cycle
(reachability check inside the insert transaction). Without this, two
issues blocking each other silently disappear from the ready view forever.

## Ready logic

An issue is ready when:

- state is not `done` or `cancelled`
- no unfinished issue blocks it through a `blocks` dependency
- it has no unexpired lease

## Event log

Every important change should append an event:

- issue created
- issue claimed
- issue updated
- note added
- issue closed
- dependency added or removed
- lease expired (from the sweeper, at most one per lease)

The event log is the audit trail and recovery aid. It is append-only and
must survive its subjects: nothing cascades events away, and issues and
projects are never hard-deleted in v1.

Over time, this should support a user-facing activity timeline comparable to
the good parts of Beads comments/history, but backed by daemon-owned writes.

## CLI model

`afctl` should be a thin client over the same API.

Core commands:

- `afctl issue create`
- `afctl issue get`
- `afctl issue list`
- `afctl issue ready`
- `afctl issue claim`
- `afctl issue release`
- `afctl issue note`
- `afctl issue events`
- `afctl issue close`
- `afctl project add`
- `afctl repo add`
- `afctl worktree register`
- `afctl artifact register`
- `afctl issue link-artifact`

The CLI should eventually support query-style filters, not only fixed list
views.

## Integration model

Possible later integrations:

- markdown export
- JSONL export
- GitHub Issues mirror
- bridge from legacy Beads data
- import or discovery of repo-local SDD packets

But none of these should own the write path.

## Actor identity

In v1, `actor` and `holder` are client-asserted strings (agent name,
username, tool id). The trust boundary is the unix socket itself: mode
`0660`, owned by the operating user. Anything that can connect is trusted
to tell the truth about who it is.

Future hardening option: read peer credentials via `SO_PEERCRED` and record
them alongside the asserted actor. Not required for v1.

## Operational model

Expected runtime assets:

- DB: `~/.local/share/af-coordinator/af-coordinator.db`
- socket: `~/.local/state/af-coordinator/af-coordinator.sock`
- logs/state: `~/.local/state/af-coordinator/`

Suggested service model:

- `systemd --user` service for daemon
- optional timer for exports / snapshots

### Backup

Never copy a live WAL database with `cp`. Supported approaches:

- `VACUUM INTO '<backup path>'` from the daemon (preferred: single
  consistent file, no external tooling)
- `sqlite3 <db> ".backup <path>"` when the daemon is stopped

A `systemd --user` timer should produce periodic backups into
`~/.local/share/af-coordinator/backups/`, plus optional JSONL exports for
greppable history.

## v1 implementation constraints

- no direct DB access from agents
- no shared mutable files as source of truth
- no server-side dependence on internet
- no automatic remote sync in the hot path
