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

Active packet selection:

- treat the active SDD packet as the lowest-numbered `docs/specs/NNN-*` packet
  that is not explicitly declared complete by its packet-local artifacts
- today that is the bootstrap packet:

```text
docs/specs/001-foundation/
```

Packet shape:

- minimum v1 packet:
  - `README.md`
  - `requirements.md`
  - `design.md`
  - `tasks.md`
  - `review.md`
- supporting artifacts are first-class when present:
  - `decisions/`
  - `traceability.md`
  - `glossary.md`
  - `schemas/`

Rules:

- do not start meaningful implementation before `requirements.md`, `design.md`,
  and `tasks.md` exist
- tiny mechanical fixes may skip a full spec packet
- when implementing a task, reference the concrete artifact path in commit and
  handoff language when useful
- treat `tasks.md` as the canonical task-slicing artifact for the packet
- do not mark a `tasks.md` entry done just because supporting code or documents
  exist; mark it done only when the described slice is actually complete and
  verified
- do not assume task status is inferred automatically from commits or file
  presence; update `tasks.md` and `review.md` deliberately as part of finishing
  the slice
- use `review.md` to capture what shipped, what was verified, what remains open,
  and whether implementation still matches requirements and design

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

## CodeWhale agent conventions

This project uses a `.bare` git worktree model. All implementation work
MUST go through a git worktree, never write directly to the `main/` checkout.

CodeWhale upstream supports first-class sub-agent worktree isolation through
`worktree`, `worktree_branch`, `worktree_base`, and `worktree_path` on the
sub-agent spawn call. For this repository:

- when opening a sub-agent with `worktree: true`, always set
  `worktree_path` explicitly
- use an absolute sibling path at the repo root; relative paths still resolve
  under CodeWhale's default `.codewhale-worktrees/` root
- do not rely on CodeWhale defaults for this repository; without an explicit
  absolute `worktree_path`, CodeWhale will create the child checkout under
  `.codewhale-worktrees/...`

Example:

```
# CORRECT — sibling worktree at repo root, alongside main/
agent(
    worktree=True,
    worktree_path="/home/abevz/github/af-coordinator/<branch-name>",
)
```

```
# WRONG — default path creates .codewhale-worktrees/ subdirectory
agent(
    worktree=True,  # missing worktree_path
)
```

Every feature commit MUST land via a worktree branch that is merged into
main. Do not write or commit directly from the main checkout.

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
