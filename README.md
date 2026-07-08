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

## Getting started

### Prerequisites

- Go 1.22 or later

### Build

```bash
make build
```

### Test

```bash
make test
```

### Run the daemon

```bash
# Start in the foreground (for testing):
af-coordinatord

# Or install as a systemd user service:
make install-service
systemctl --user enable --now af-coordinatord
```

### Configure

The daemon reads these environment variables:

| Variable | Default | Description |
|---|---|---|
| `AF_COORDINATOR_SOCKET` | `~/.local/state/af-coordinator/af-coordinator.sock` | Unix socket path |
| `AF_COORDINATOR_DB` | `~/.local/share/af-coordinator/af-coordinator.db` | SQLite database path |
| `AF_COORDINATOR_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |

### Install binaries

```bash
make build-install
```

This builds `af-coordinatord` and `afctl` into `~/.local/bin/`.

Common worktree maintenance commands:

```text
afctl worktree list --repo <repo-id>
afctl worktree unregister --worktree <worktree-id>
afctl worktree prune --repo <repo-id>
```

`unregister` only removes a non-main worktree record when nothing still points
at it. `prune` is the safe cleanup path for stale records whose checkout path
is already gone on disk.

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

## Public repo, private runtime data

This repository is intended to be safe to publish.

The rule is:

- code, docs, schema, migrations, and service definitions may live in git
- real runtime data must stay outside the repository

Expected private runtime locations:

- database: `~/.local/share/af-coordinator/af-coordinator.db`
- socket: `~/.local/state/af-coordinator/af-coordinator.sock`
- logs/state: `~/.local/state/af-coordinator/`

Do not commit:

- live databases
- local runtime state
- logs
- tokens, secrets, or `.env` files
- exports or snapshots containing real task data unless intentionally sanitized

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

Key semantics:

- issues carry human-facing short ids (`<project_key>-<N>`, e.g. `afc-42`)
  allocated by the daemon from a per-project counter
- "claimed" is not a stored status; it is derived from an unexpired lease
- issue statuses: `open`, `in_progress`, `blocked`, `deferred`, `done`,
  `cancelled`
- only `blocks` dependencies affect readiness, and the daemon rejects
  dependency cycles

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
cmd/afc-mcp/           MCP stdio wrapper over the daemon API
docs/                  design docs
internal/api/          transport layer
internal/client/       Go client for the daemon API
internal/mcp/          MCP protocol wrapper over the daemon API
internal/config/       daemon configuration
internal/core/         domain logic (validation, models, lease semantics)
internal/store/sqlite/ SQLite implementation (including lease operations)
migrations/            schema migrations
```

## v1 documentation

- [Architecture v1](docs/architecture-v1.md)
- [Schema v1](docs/schema-v1.md)
- [API v1](docs/api-v1.md)
- [MCP server v1](docs/mcp-server-v1.md)
- [Agent protocol v1](docs/agent-protocol-v1.md)
- [Workflows v1](docs/workflows-v1.md)
- [SDD workflow v1](docs/sdd-workflow-v1.md)
- [Foundation spec packet](docs/specs/001-foundation/README.md)
- [Roadmap](docs/roadmap.md) — direction beyond v1; operational tracking lives in project `afc` inside the coordinator itself

## Known limitations

- The coordinator assumes a single active daemon per machine.
- There is currently no web UI or built-in visualization.
- The project is designed for local-first operations and does not natively synchronize state across multiple machines.

## How to release

1. Ensure the `review.md` for the active SDD packet is complete.
2. Verify that all tests pass (`make test`) and the codebase is clean (`make lint`).
3. Build the binaries using `make build`.
4. Create a new git tag and push it to the upstream repository.
