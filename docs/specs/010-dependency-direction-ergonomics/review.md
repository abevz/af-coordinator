# 010 Dependency Direction Ergonomics — Review

## Outcome

Complete. PR #15 merged commit `e5d251c` and delivered every task in
`tasks.md`.

## What shipped

- `core.Issue.Blocks` exposes the reverse direction of active blocking edges.
- `afctl issue dependency add/remove` accepts unambiguous `--blocked-by` and
  `--blocks` forms while retaining the compatible `--depends-on` form.
- Full issue output separates `Blocked By:` from `Blocks:` and labels a
  status-only block explicitly.
- Store and CLI tests cover edge direction, terminal blockers, resolver errors,
  and human-readable rendering.

## Verification

- `go test ./...`
- `make build` (`go build -buildvcs=false ./...` in this bare-worktree layout)
- manual inspection of the merged CLI help and directional render contract

## Requirement and design alignment

The implementation preserves the stored dependency model and API compatibility
defined in `design.md`. It changes authoring and presentation semantics only;
readiness continues to depend on the existing `blocks` edge rules.

## Remaining work

None in this packet. Broader operator ergonomics and trust-boundary work stays
in the live `afc` backlog and is prioritized in `docs/roadmap.md`.
