# 010 Dependency Direction Ergonomics — Tasks

- [x] T1: Add `Blocks []string` to `core.Issue` (R4).
- [x] T2: Populate reverse `blocks` edges in `populateDependencies`, gated on the
  blocker being non-terminal (R4, R6).
- [x] T3: Add `--blocked-by` / `--blocks` to `dependency add`/`remove` via a pure
  `resolveDependencyEdge` resolver; keep `--depends-on/--kind`; print a
  plain-language confirmation (R1, R2).
- [x] T4: In `printIssueFull`, drop `blocks` edges from the raw `Dependencies:`
  list, add a `Blocks:` line, and label status-only blocks distinctly (R3, R5).
- [x] T5: Tests — store reverse population + terminal-blocker rule; CLI direction
  resolver (all forms + errors); detail-view directional rendering. Update the
  stale test that asserted the old ambiguous `blocks <target>` render.
