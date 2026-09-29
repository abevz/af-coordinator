# Review

`afc-174` is implemented in the `feat/afc-174-stats` worktree. The report adds
an additive `by_project` field, the CLI presents a sorted project table and
readable durations, and API/schema docs describe the new field. There is no
schema migration or additional store query.

Verification: `go build ./...` and `go test ./...` passed. A scratch `dibsd`
with isolated `HOME`, `DIBS_DB`, and `DIBS_SOCKET` returned two project rows
whose totals sum to inventory, including an empty project; JSON included the
same rows, and `--project alpha` omitted the human breakdown. An independent
read-only review found a missing blocked column; the final diff includes that
column and a formatting test, and the reviewer confirmed resolution with no
other findings.

Integration and issue closure remain pending. The implementation matches
[requirements.md](requirements.md) and [design.md](design.md).
