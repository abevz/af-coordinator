# Design

## Architecture

```text
agents / scripts / tools
        |
        | HTTP+JSON over Unix socket
        v
af-coordinatord
        |
        v
SQLite WAL
```

## Core boundary

The design keeps two truths separate:

- SDD artifacts are the truth for scope, requirements, design, and task slicing
- coordinator state is the truth for claims, execution progress, blockers, and notes

The daemon must therefore store references to spec artifacts, but it must not
replace the spec packet itself.

## Domain objects

- project
- repository
- repo remote
- worktree
- artifact root
- artifact
- issue
- dependency
- lease
- note
- event

## Artifact model

`artifact_root` describes a repository-local area such as `docs/specs/`.

`artifact` describes a concrete file such as:

- `docs/specs/001-foundation/requirements.md`
- `docs/specs/001-foundation/design.md`
- `docs/specs/001-foundation/tasks.md`
- `docs/specs/001-foundation/review.md`

Artifacts are linked to issues through `issue_artifacts`.

## Mutation model

Write flow:

1. client reads issue
2. client claims issue and receives lease token
3. client sends update with `expected_version` + `lease_token`
4. daemon validates lease and version
5. daemon writes state change and appends event

## Ready logic

An issue is ready when:

- it is not done or cancelled
- it has no active blocking dependency
- it is not leased by another holder

## Initial command surface

- `afctl project add`
- `afctl repo add`
- `afctl worktree register`
- `afctl artifact-root add`
- `afctl artifact register`
- `afctl issue create`
- `afctl issue claim`
- `afctl issue update`
- `afctl issue link-artifact`
- `afctl issue close`

## Risks

- If artifact registration is too manual, operators will stop maintaining links
- If issue descriptions become the place where design intent lives, SDD value is lost
- If the daemon API is too weak, wrappers will bypass it and recreate unsafe write paths
