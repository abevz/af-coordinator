# 010 Dependency Direction Ergonomics — Requirements

## Problem

The `blocks` dependency has one stored shape — `(issue depends_on target, kind=blocks)`
meaning **issue is blocked by target** — but the tooling exposes it ambiguously:

1. `afctl issue dependency add <A> --depends-on <B> --kind blocks` reads like
   "A blocks B" while it actually records "A is blocked by B". Operators (and
   agents) routinely add the edge in the wrong direction.
2. The `afctl issue get` detail view renders such an edge as `- blocks <B>` in
   the raw `Dependencies:` list, contradicting the correct `Blocked By: <B>`
   line printed just below it.
3. A blocker has no indication of what it blocks — the relationship is only
   visible from the blocked side.

`Blocked` also has a second, unrelated source: an issue whose own `status` is
`blocked`, with no dependency edge. Any change must preserve that distinction.

## Requirements

- R1: Provide unambiguous directional authoring: `--blocked-by <id>` and
  `--blocks <id>` on `dependency add`/`remove`, plus a confirmation line stating
  the resulting relationship in plain language.
- R2: Keep `--depends-on`/`--kind` working for backward compatibility.
- R3: The detail view must never render a `blocks` edge as an ambiguous
  `blocks <target>` line; blocking relationships appear only as `Blocked By:`
  (blocked side) and `Blocks:` (blocking side).
- R4: A blocker issue exposes the reverse relationship (`Blocks`) with the same
  activation rule as `BlockedBy`: a terminal blocker blocks nothing.
- R5: Preserve the status-only block (`status == "blocked"` with no edge) as a
  distinct, clearly labelled state.
- R6: No change to the stored dependency schema or to forward `Blocked`/`BlockedBy`
  semantics.
