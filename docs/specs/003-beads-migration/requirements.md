# Requirements — Beads migration pilot

## Purpose

Migrate `~/github/utils` off Beads/shared Dolt as the v1 acceptance test
for `af-coordinator`.

## Constraints

1. **Reversible.** The migration must support a parallel read-only mode
   where both systems stay live. Cutover is a config change, not a data
   migration.

2. **Idempotent.** Registration commands must be safe to re-run (no
   duplicate projects, repos, or worktrees).

3. **Minimal changes to the utils repo.** Only `AGENTS.md` is modified.
   The repo's git history, branch structure, and working-tree layout stay
   the same.

4. **Archive, not migrate.** All 7 Beads issues are closed. Their full
   state is snapshotted in `inputs/beads-snapshot-2026-07-04.json`. The
   coordinator project starts empty; the snapshot IS the archive.

5. **Parallel soak.** For at least 48 hours after switchover, only
   the coordinator accepts mutations. beads-dolt remains running but
   untouched as a rollback safety net. After soak passes, it is
   decommissioned.

6. **No Beads dependency for the coordinator.** The af-coordinator must
   never read, write, or depend on the Beads Dolt database.

7. **No live access to utils repo or beads CLI.** All scouting data is
   pre-collected in `inputs/`. Agents may NOT shell into `~/github/utils`
   or call `bd`/`beads`/`dolt`.

## Functional requirements

- FR-001: Register `utils` project in af-coordinator
- FR-002: Register the `~/github/utils` repository under the utils project
- FR-003: Register the repo's working tree(s) as worktrees
- FR-004: Keep the JSON snapshot as the closed-issue archive; do not
  import closed issues into the coordinator database
- FR-005: Switch `~/github/utils/AGENTS.md` to the afctl protocol
  (referencing the adapter snippet from packet 002)
- FR-006: After soak, stop the beads-dolt service and mark migration
  complete in review.md

## Success criterion

Over the following weeks, the daemon (`af-coordinatord`) needs less
babysitting than `beads-dolt.service` did. If it does not, v1 has failed
its acceptance test. The daemon's initial crash-counter benchmark: 914
(found during snapshot collection).

## Non-goals

- Importing closed/resolved Beads issues into the coordinator database
- Bidirectional sync between Beads and af-coordinator
- Importing Beads user accounts or permission models
- Multi-repo or multi-project migration in this pilot
