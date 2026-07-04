# Requirements — Beads migration pilot

## Purpose

Migrate `~/github/utils` off Beads/shared Dolt as the v1 acceptance test
for `af-coordinator`.

## Constraints

1. **Reversible.** The migration must support a parallel read-only mode
   where both systems stay live. Cutover is a config change, not a data
   migration.

2. **Idempotent.** Registration and import commands must be safe to re-run
   (no duplicate projects, repos, or issues).

3. **Minimal changes to the utils repo.** Only `AGENTS.md` is modified.
   The repo's git history, branch structure, and working-tree layout stay
   the same.

4. **Zero data loss.** All open Beads issues with their notes must be
   preserved in af-coordinator before the system becomes the primary.

5. **Parallel soak.** For at least 48 hours after initial import, both
   systems accept mutations. Utils agents may use either system. After
   soak, Beads becomes read-only, then decommissioned.

6. **No Beads dependency for the coordinator.** The af-coordinator must
   never read, write, or depend on the Beads Dolt database.

## Functional requirements

- FR-001: Register `utils` project in af-coordinator
- FR-002: Register the `~/github/utils` repository under the utils project
- FR-003: Register the repo's working tree(s) as worktrees
- FR-004: Import open Beads issues as coordinator issues, mapping
  `UTL-N` short ids to `utils-N`
- FR-005: Preserve issue notes on imported issues
- FR-006: Switch `~/github/utils/AGENTS.md` to the afctl protocol
  (referencing the adapter snippet from packet 002)
- FR-007: After soak, stop the beads-dolt service and mark migration
  complete in review.md

## Success criterion

Over the following weeks, the daemon (`af-coordinatord`) needs less
babysitting than `beads-dolt.service` did. If it does not, v1 has failed
its acceptance test.

## Non-goals

- Importing closed/resolved Beads issues
- Bidirectional sync between Beads and af-coordinator
- Importing Beads user accounts or permission models
- Multi-repo or multi-project migration in this pilot
