# 010 Dependency Direction Ergonomics — Design

## Stored model (unchanged)

A dependency row is `(issue_id, depends_on_issue_id, kind)`. For `kind=blocks`,
it means **issue_id is blocked by depends_on_issue_id** while the target is
non-terminal. This shape and the forward `Blocked`/`BlockedBy` computation in
`populateDependencies` are unchanged (R6).

## Reverse edges (R4)

`core.Issue` gains `Blocks []string`. In `populateDependencies` a second query
selects rows whose `depends_on_issue_id` is one of the loaded issues with
`kind=blocks`; for each, the loaded issue (the blocker) appends the dependent's
short ID to `Blocks`, gated on the blocker being non-terminal. This yields the
symmetry `A.BlockedBy ∋ B ⇔ B.Blocks ∋ A`, both governed solely by the blocker's
terminal status — matching the existing forward rule.

## CLI (R1, R2)

`resolveDependencyEdge(issueID, args)` maps flags onto the stored edge:

| flag on issue A | stored edge | confirmation |
|---|---|---|
| `--blocked-by B` | A depends_on B (blocks) | `A is now blocked by B` |
| `--blocks B` | B depends_on A (blocks) | `B is now blocked by A` |
| `--depends-on B [--kind K]` | A depends_on B (K) | kind-specific |

Exactly one form is allowed; `--kind` may not combine with a directional flag.
`remove` mirrors the same direction resolution.

## Detail render (R3, R5)

`printIssueFull` omits `kind=blocks` edges from the raw `Dependencies:` list
(they are shown as `Blocked By:`), adds a `Blocks:` line from `Issue.Blocks`, and
labels a status-only block as `Blocked:       yes (status)` (no `BlockedBy`). The
table view already separated these and is untouched.

## Compatibility

The API serialises `core.Issue` directly, so `Blocks` flows to every consumer
without endpoint changes. Legacy `--depends-on/--kind` authoring keeps working.
