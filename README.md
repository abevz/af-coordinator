# af-coordinator

Local coordination service for AI agents working across many projects,
repositories, and worktrees.

## Why this exists

`af-coordinator` is meant to replace the fragile parts of `Beads + shared Dolt`
when the real workload is concurrent agent coordination rather than human issue
tracking.

The core design choice is simple:

- agents do not write to storage directly
- one daemon owns all writes
- clients talk to the daemon over a local API

## Goals

- reliable local-first coordination without internet dependency
- atomic claim, release, update, and close operations
- support for many projects and repositories
- first-class worktree and remote awareness
- first-class links from operational work to SDD artifacts
- clear event log and audit trail
- simple recovery and backup model

## Non-goals for v1

- web UI
- distributed cluster mode
- multi-node replication
- GitHub-first source of truth
- embedded scripting or plugin system

## What we borrow from Beads

`af-coordinator` should borrow workflow and UX ideas from Beads, but not its
shared-Dolt operational model.

Keep from Beads:

- first-class tasks
- dependency tracking
- computed `ready` view
- notes / comments / activity trail
- short stable task ids
- query-oriented CLI, not only flat listing

Do not copy from Beads:

- shared Dolt server as hot-path storage
- auto-sync / auto-push in the write path
- multi-writer semantics through shell commands over a shared VCS-backed store

So the intended split is:

- Beads ideas for task flow and operator UX
- `af-coordinator` storage and concurrency model for correctness

## Architecture

```text
agents / scripts / tools
        |
        | HTTP+JSON over Unix socket
        v
af-coordinatord
        |
        v
SQLite (WAL)
```

The daemon is the single write authority. Clients never open the database
directly.

## SDD methodology

This project is built spec-first.

For any meaningful feature, the canonical flow is:

```text
requirements.md -> design.md -> tasks.md -> implementation -> review.md
```

For `af-coordinator`, that means:

- SDD artifacts define scope, contracts, and acceptance criteria
- `af-coordinator` runtime state tracks execution, claims, blockers, notes, and handoff
- the coordinator does not replace the spec canon; it complements it

Initial SDD workspace:

```text
docs/specs/001-foundation/
  README.md
  requirements.md
  design.md
  tasks.md
  review.md
```

Rules for v1:

- no implementation starts before `requirements.md`, `design.md`, and `tasks.md` exist
- tiny mechanical fixes may skip a full spec packet
- operational issues should link to the relevant spec/task artifact instead of duplicating design intent in issue text

## Main domain model

- `projects`: logical top-level initiatives
- `repositories`: logical repos inside projects
- `repo_remotes`: tracked fetch/push remotes for each repository
- `worktrees`: concrete checkouts on disk, possibly pointing at different remotes
- `artifacts`: SDD files and related design artifacts tracked by path and kind
- `issues`: tasks, bugs, ops work, coordination units
- `dependencies`: blocking relationships between issues
- `leases`: active claims owned by agents
- `notes`: human/agent notes attached to issues
- `events`: append-only audit trail

In practice, this means the product should eventually expose:

- a `ready` view
- dependency-aware task state
- issue notes / comments
- issue activity timeline
- queryable task listings

## Why projects, repositories, and worktrees are separate

This environment has:

- many projects
- multiple repositories per project
- multiple worktrees per repository
- some worktrees tracking different remotes

So identity cannot be based only on a filesystem path.

An issue may belong to:

- a project
- a repository
- optionally a specific worktree when the task is local to that checkout

## Repository layout

```text
cmd/af-coordinatord/   daemon entrypoint
cmd/afctl/             CLI client
docs/                  design docs
internal/api/          transport layer
internal/core/         domain logic
internal/export/       JSONL / markdown / future mirrors
internal/lease/        claim and lease logic
internal/store/sqlite/ SQLite implementation
migrations/            schema migrations
```

## v1 documentation

- [Architecture v1](docs/architecture-v1.md)
- [Schema v1](docs/schema-v1.md)
- [SDD workflow v1](docs/sdd-workflow-v1.md)
- [Foundation spec packet](docs/specs/001-foundation/README.md)

## Recommended implementation order

1. SQLite schema + migrations
2. daemon boot + health endpoint
3. project/repo/worktree registration
4. artifact registration + issue-to-spec links
5. issue create/get/list/ready
6. claim/release/heartbeat lease flow
7. update/close with optimistic concurrency
8. notes + events
9. `afctl`
10. systemd user service
11. export and backup helpers
