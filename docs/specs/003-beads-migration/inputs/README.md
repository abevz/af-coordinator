# Inputs: state of ~/github/utils as of 2026-07-04

Agents working this packet cannot leave the workspace, so the source
data is snapshotted here. Collected by Claude with direct access to
`~/github/utils` and the shared Beads/Dolt server.

## Files

- `beads-snapshot-2026-07-04.json` — full `bd list --all --json` export
- `utils-AGENTS-current.md` — verbatim copy of `~/github/utils/AGENTS.md`
  (the file packet 002's adapter snippet will replace)

## Issue inventory

7 issues total, **all closed, zero open**. Statuses: `closed` ×7.

| id | priority | type | title |
|----|----------|------|-------|
| UTL-iut | P1 | feature | Expose live vector indexing progress |
| UTL-3et | P1 | feature | Add remote-first embeddings with local fallback |
| UTL-m3d | P2 | task | daily-check: fix 'q' key quitting app while typing |
| UTL-bm3 | P2 | task | Fix silent-failure bugs found by audit |
| UTL-jav | P2 | bug | daily-check comment mode: Enter on first comment |
| utils-6pz | P2 | task | Sprint mode toggle |
| utils-cow | P2 | task | TUI tree view in details pane |

Note the id prefixes are already inconsistent (`UTL-*` and `utils-*`) —
Beads changed prefix mid-life. One more argument for daemon-allocated
short ids.

**Implication for requirements:** there is no open-issue import problem.
The migration decision is only whether to import closed history for the
audit trail or start the af-coordinator project empty and keep this
snapshot as the archive. Import tooling, if any, operates on this JSON
file — not on a live Beads database.

## Repo state

`git -C ~/github/utils log --oneline -3`:

```text
88803c9 Add watcher queue visibility to vector status
c62a096 Fix q key quitting daily-check while typing in Metrics field (UTL-m3d)
96ee20f Show live vector indexing progress (UTL-iut)
```

Subtools in the repo: `ai_organizer/`, `ask-vault/`, `daily-check/`,
`exports/`, `llm-limit-watch/`, `vector-indexer/`. Runtime dirs:
`.beads/`, `.agents/`, `.codex/`, `.perles/`.

## Operational evidence (why this migration exists)

Collecting this snapshot required repairing the Beads stack first, which
is the acceptance-test argument in miniature:

- `beads-dolt.service` was in a crash loop, restart counter **914**
- port 3308 was squatted by an orphan Dolt server belonging to a
  *different* project (`englishdrills`), masking the real failure
- underneath, the `utils` Dolt database had a corrupted chunk journal
  (`invalid journal record length`); recovery required
  `dolt fsck --revive-journal-with-data-loss`
- after recovery, `bd stats` still fails with an internal Dolt merge
  conflict (`wisps table: failed to stage dolt_ignore`)
- pre-repair safety copy: `~/.beads/shared-server/dolt-utils-backup-20260704`

The daemon this migration targets has none of these failure modes: one
process, one SQLite file, no ports, no merge state.
