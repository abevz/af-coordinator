# af-coordinator

**A local execution ledger for AI agents working across repositories and git
worktrees.**

`af-coordinator` is a local-first daemon that gives agents one place to decide
what is ready, claim it atomically, renew ownership, hand work off, and leave an
auditable result.

It gives agents a shared execution ledger: what work is ready, who claimed it,
what is blocked, what changed, and how a task was closed. Specs and source code
stay in git; runtime coordination state stays in the local daemon.

Use it when local AI agents need to cooperate safely without all of them
writing directly to a database, editing the same checkout, or duplicating work.

> **Project status: public preview.** The core local workflow is in daily use,
> but the CLI and HTTP API may still change before v1.0. Network clients and
> multi-machine synchronization are not supported yet.

`af-coordinator` is not an agent runtime or a replacement for specs. It is the
execution control plane between planning and execution:

```text
specs / operator intent
          |
          v
af-coordinator: ready -> claim -> heartbeat -> handoff / close
          |
          v
agents / runners -> worktree -> branch / PR / result
```

## What it does

- tracks projects, repositories, worktrees, artifacts, issues, dependencies,
  leases, notes, and events
- exposes a small HTTP+JSON API over a Unix socket
- ships `afctl`, a CLI for agents and humans
- computes a `ready` view from issue status, leases, and blockers
- records an append-only audit trail for claims, notes, updates, and closes
- keeps live runtime data out of git

## The five-minute value

Once a project and repository are registered, the operating loop is small:

```bash
# Human or agent creates work.
afctl issue create --project demo --scope-kind project \
  --title "Document the retry policy"

# Workers ask for work that is open, unblocked, and not leased.
afctl issue ready --project demo

# Exactly one worker receives the lease. Claiming increments the issue's
# version as a side effect — use the Version this prints, not one read
# earlier from `issue get`, as --expected-version below.
afctl issue claim demo-1 --ttl 900

# While working, it renews the lease and records material context.
afctl issue heartbeat demo-1 --lease-token "$LEASE_TOKEN" --ttl 900
afctl issue note add demo-1 --body "Verified the failure path" --actor worker-1

# It closes with evidence, or hands off and releases atomically.
afctl issue close demo-1 --resolution done --expected-version 2 \
  --lease-token "$LEASE_TOKEN" --note "Documented and verified"
```

If two workers try to claim the same issue, one wins and the other receives a
stable `lease_held` error. If a worker disappears, its lease expires and the
work can become ready again — or, to recover it immediately instead of
waiting out the TTL (e.g. a script crashed before it ever persisted its
lease token), `afctl issue operator-release` clears the lease without one.

## Where it fits

| Concern | Source of truth |
|---|---|
| Requirements, design, acceptance criteria | SDD files or another planning system |
| Ready/blocked state, leases, attempts, handoffs | `af-coordinator` |
| Code and review | Git worktrees, branches, commits, and pull requests |
| External visibility | Optional tracker integrations; not implemented yet |

This boundary is deliberate. GitHub Issues, GitLab Issues, Markdown specs, or
native coordinator issues can all describe work; the coordinator owns only the
live execution state used by agents.

## Why this exists

`af-coordinator` is meant to replace the fragile parts of `Beads + shared Dolt`
when the real workload is concurrent agent coordination rather than human issue
tracking.

The core design choice is simple:

- agents do not write to storage directly
- one daemon owns all writes
- clients talk to the daemon over a local API

## Quick start

### Prerequisites

- Go version matching the `go` directive in [go.mod](go.mod)
- `make`
- `git` for clone/worktree workflows

Run the preflight first on a clean laptop or VM:

```bash
make preflight
```

The preflight checks required build tools, the Go version, the install
directory, and the current OS/service-manager situation.

Then install the git hooks once, so the daemon auto-redeploys after every
merge into `main` instead of silently going stale:

```bash
make install-hooks
```

### Build and install

```bash
make build
make build-install
```

This builds `af-coordinatord`, `afctl`, and `afc-mcp` into `~/.local/bin/`.
Make sure `~/.local/bin` is on `PATH`.

To install the latest published GitHub release instead of building from source:

```bash
sh contrib/install/install-release.sh
```

Set `VERSION=vX.Y.Z` to install a specific tag. The script downloads the
matching Linux/macOS archive, verifies it against the published checksum
manifest, and installs the three binaries into `~/.local/bin` by default.

### Test

```bash
make test
```

CI (`.github/workflows/ci.yml`) runs `vet`, a `gofmt -l` check, `golangci-lint`, `test`, and
`build` on every pull request and on push to `main`; a PR must be green before merging.

### Run the daemon

```bash
# Start in the foreground (for testing):
af-coordinatord
```

On Linux with `systemd --user`:

```bash
make install-service
sh contrib/install/systemctl-user.sh enable --now af-coordinatord
```

On macOS with `launchd`:

```bash
make install-launchd
launchctl print gui/$(id -u)/com.abevz.af-coordinatord
```

After the daemon is running:

```bash
afctl health
afctl doctor
```

`afctl doctor` is a post-install/runtime diagnostic. It checks daemon
reachability, client/daemon version skew, whether the daemon binary matches
the local git HEAD (run it from inside this checkout to catch a merge that
was never followed by `make restart-service` — automatic if you've run
`make install-hooks`, a safety net if you haven't), backup setup, duplicate
binaries, and client/daemon config mismatch.

### Platform support

| Platform | Status |
|---|---|
| Linux + systemd user session | Primary supported install path. `make install-service`, `make restart-service`, and `make install-backup` use systemd. |
| macOS | Supported daemon install path via `make install-launchd`; automated backups are available through `make install-backup` / launchd. |
| Other Unix-like OSes | Untested. The daemon relies on Unix sockets and local filesystem paths. |

### Configure

The daemon reads these environment variables:

| Variable | Default | Description |
|---|---|---|
| `AF_COORDINATOR_SOCKET` | `~/.local/state/af-coordinator/af-coordinator.sock` | Unix socket path |
| `AF_COORDINATOR_DB` | `~/.local/share/af-coordinator/af-coordinator.db` | SQLite database path |
| `AF_COORDINATOR_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `AF_OPERATOR_TOKEN` | (unset) | Required by the daemon for `afctl issue operator-close/operator-reopen/operator-release`. Unset means those commands fail with `forbidden: AF_OPERATOR_TOKEN not configured on server`. The client (`afctl`) reads the same variable from its own environment, so the value must match on both sides. Under `systemd --user`, wire it in via an `EnvironmentFile=` drop-in pointing at a `600`-permission file outside the unit — see `contrib/systemd/af-coordinatord.service`. |

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

In practice, the product exposes:

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
internal/store/        API-facing store interface
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
- Privileged operator actions are local-trust operations today; hardening that
  boundary is the next security priority.
- External tracker integrations are planned as optional adapters, not as a new
  source of truth.

## How to release

1. Ensure the `review.md` for the active SDD packet is complete and all related
   `afc` issues are closed.
2. Verify locally:
   ```bash
   go test ./...
   make build
   ```
3. Create and push a tag:
   ```bash
   git tag vX.Y.Z
   git push origin vX.Y.Z
   ```
4. The `Release` GitHub Actions workflow builds Linux and macOS archives for
   `afctl`, `af-coordinatord`, and `afc-mcp`, then uploads
   `checksums.txt` to the GitHub release.
5. Verify the release install path:
   ```bash
   VERSION=vX.Y.Z sh contrib/install/install-release.sh
   afctl health
   ```
