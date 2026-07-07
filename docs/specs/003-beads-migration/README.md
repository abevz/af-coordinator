# 003 — Beads migration pilot

Status: completed. See `review.md` for the migration and soak outcome.

Intent: migrate `~/github/utils` off Beads/shared Dolt as the v1
acceptance test:

- register project, repository, and worktrees in af-coordinator
- import open Beads issues (id mapping `UTL-*` → `utils-*` short ids)
- switch the repo's `AGENTS.md` to the `afctl` protocol (packet 002
  adapter snippet)
- run both systems read-only in parallel for a short soak, then stop
  requiring Beads in that repo

Success criterion (from README/architecture): over the following weeks
the daemon needs less babysitting than `beads-dolt.service` did. If it
does not, v1 has failed its acceptance test and we say so in review.md.

Historical note: this packet was originally blocked by packet 002 because
agents needed the protocol and enforcement before a live repo depended on the
coordinator. That blocker was resolved before implementation.
