# 015 Coordination Safety Review

## Status

Specification and backlog slicing complete; implementation not started.

## Planning outcome

- Preserved the 2026-08-11 evidence-based technical audit in `audit.md`.
- Defined the concurrency, fencing, durability, idempotency, and operational
  requirements in `requirements.md`.
- Chose a SQLite-preserving single-daemon design in `design.md`.
- Created implementation epic `afc-102` and bounded leaves `afc-103` through
  `afc-116` with priority, dependency, factory-routing, scope, out-of-scope, and
  acceptance criteria.
- Gated every implementation leaf on planning issue `afc-101`, so no factory
  worker can start from an unmerged packet.
- Reconciled stale packet 011 and rewrote/gated the pre-existing live backlog
  rather than deleting product ideas.

## What has not shipped

No coordinator correctness, storage, protocol, CLI, or runtime behavior is
changed by this planning slice. In particular, the race classifications in
`audit.md` remain current until their leaves land and are verified.

## Implementation review gate

Packet 015 must not be marked complete until all of the following are recorded
here:

- commits and PRs for every required leaf;
- focused regression tests added in each behavior change;
- deterministic results for all six concurrency races;
- black-box results for all eight crash/recovery cases;
- installed daemon/CLI revision parity and scratch-runtime verification;
- updated maturity assessment and any remaining trust boundary;
- explicit operator decision on whether concurrent factory use is enabled.

## Planning verification

Verified in sibling worktree
`/home/abevz/github/af-coordinator/afc-101-coordination-safety`:

- `git diff --check` — pass;
- `make build` — pass at revision
  `68f4bc67e6ca7067d13bc1c75c7ec3f4204df613`;
- `go test ./... -count=1` — pass;
- `go test -race ./... -count=1` — pass;
- `afctl issue list --project afc --status open,in_progress,deferred --json`
  — confirms the epic, all 14 leaf IDs, parent edges, blocking DAG, priorities,
  and `exec/auto` tags;
- `afctl issue ready --project afc --json` — `[]` while `afc-101` is claimed;
- `afctl issue ready --project afc --tag exec/auto --json` — `[]` while the
  packet is unmerged.

The raw `go build ./...` command in this external `.bare` linked worktree
reported Go VCS-stamping status failure. The repository's canonical `make
build` target deliberately uses `-buildvcs=false` and passed; no source build
or test failed.

After merge and closure of `afc-101`, the expected first manual ready leaf is
`afc-103`; independent `exec/auto` routing may expose only `afc-107` until the
lease-contract blockers close. That post-close view is verified during issue
handoff, not predicted as current live state.
