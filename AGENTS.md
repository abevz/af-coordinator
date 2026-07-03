# af-coordinator repo instructions

## Purpose

This repository builds a local-first coordination daemon for multi-project,
multi-repository, multi-worktree AI execution.

## Non-negotiable rules

- Never add AI attribution trailers such as `Co-authored-by`.
- Keep SDD as the source of truth for scope and design.
- Keep the coordinator as the source of truth for execution state.
- Do not invent feature behavior in code that is not grounded in the active spec.
- Do not access SQLite directly from helper scripts or agents for mutations.
- Keep real runtime data out of git.

## SDD workflow

Canonical flow for meaningful work:

```text
requirements.md -> design.md -> tasks.md -> implementation -> review.md
```

Repository location:

```text
docs/specs/NNN-feature/
```

Current bootstrap packet:

```text
docs/specs/001-foundation/
```

Rules:

- do not start meaningful implementation before `requirements.md`, `design.md`,
  and `tasks.md` exist
- tiny mechanical fixes may skip a full spec packet
- when implementing a task, reference the concrete artifact path in commit and
  handoff language when useful

## Architecture assumptions for v1

- simple service layout
- stdlib first
- manual dependency wiring
- SQLite WAL backend
- HTTP+JSON over Unix socket

Do not introduce frameworks, DI containers, or plugin systems unless the spec
explicitly calls for them.

## Directory intent

- `cmd/af-coordinatord/` daemon entrypoint only
- `cmd/afctl/` CLI entrypoint only
- `internal/api/` HTTP transport and handlers
- `internal/core/` domain logic and contracts
- `internal/store/sqlite/` SQLite-specific persistence
- `docs/specs/` SDD canon
- `migrations/` schema migrations

Keep `main.go` files thin. Push behavior down into `internal/`.

## Git topology

This repository uses a separate git dir:

```text
/home/abevz/github/af-coordinator/.bare
```

The current working tree is the main checkout.

Rules:

- prefer `git worktree` for parallel tracks instead of ad hoc directory copies
- do not create nested git repositories inside the working tree
- treat `.bare` as repository internals; do not edit it manually
- when additional worktrees are created, keep them tied to the same `.bare`
  repository

## Public repo boundary

This repository is expected to be publishable.

Do commit:

- code
- docs
- migrations
- sanitized examples
- service/unit files without embedded secrets

Do not commit:

- live SQLite databases
- runtime sockets or state files
- logs
- local overrides with secrets
- task exports containing real unsanitized operator data

## Build and verification

Preferred checks:

```text
gofmt -w .
go build ./...
go test ./...
```

Before finishing implementation work:

- run formatting
- build the repo
- run tests relevant to the touched code

## Scope control

- Do not replace spec artifacts with long issue descriptions or ad hoc notes.
- Do not add network-backed dependencies for core local operation.
- Do not widen from bootstrap/service skeleton into full feature delivery unless
  the user asks for that explicitly.
