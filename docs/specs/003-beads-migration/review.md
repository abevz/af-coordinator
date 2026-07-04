
## AFC-SDD-0025 — Register utils project, repo, worktree

Shipped.

### What shipped

- `afctl project add --key utils --name "Utils repo"` — created
- `afctl repo add --project utils --logical-name utils --canonical-git-dir /home/abevz/github/utils` — created
- `afctl worktree register --repo utils --absolute-path /home/abevz/github/utils --main` — created (`--repo` now accepts logical name via AFC-SDD-0031)
- Verified: `afctl issue list --project utils` → `[]` (empty, correct)

### What was verified

- Daemon starts and stays up via systemd user service
- All three registration commands succeed and are idempotent
- `--repo utils` resolves by logical name (not just UUID)

### Open

- AGENTS.md replacement (0026) — pending
- Long-term soak (0027) — pending

## AFC-SDD-0026 — Replace utils AGENTS.md with afctl protocol adapter

Shipped.

### What shipped

- `~/github/utils/AGENTS.md` replaced: Beads workflow → afctl adapter snippet
  + Session Completion section (mandatory push, no bd dolt push)
- References: `afctl protocol` (go:embed) or absolute path to protocol doc
- Actor identity: `AF_COORDINATOR_ACTOR` with stable names

### What was verified

- Branch `chore/afctl-protocol` created, commit applied by operator
- Session Completion preserved from original (critical git push workflow)
- No Beads references remain

### Open

- Soak period (0027) — pending

## Known limitations

- **AFC-SDD-0031 — GetRepo logical-name ambiguity**:
  `GetRepo` queries `WHERE id = ? OR logical_name = ?`, but `logical_name`
  is only unique per project (`UNIQUE(project_id, logical_name)`). If two
  projects have a repo with the same logical name, `GetRepo` may return
  the wrong one. This is acceptable for the pilot (single project `utils`),
  but before multi-project use a scoped lookup (project_key + logical_name)
  should replace the bare name resolve. Tracked in the 0031 commit message.

## AFC-SDD-0027 — Parallel soak (in progress)

Soak clock started 2026-07-04 ~09:15 CEST: `chore/afctl-protocol`
merged into utils main (99dbeb5), daemon serving.

Baseline:

- `af-coordinatord` NRestarts=0 (ActiveEnterTimestamp 09:10:56 CEST,
  restart at that time was the binary reinstall after 0033)
- Comparison target: beads-dolt.service reached restart counter 914
- Check due: 2026-07-06 (Monday) morning —
  `systemctl --user show af-coordinatord -p NRestarts` plus journal scan

Sequencing note for 0028: `daily-check` still reads Beads. It must be
switched to the coordinator (or its Beads section disabled) BEFORE
beads-dolt is decommissioned, or the soak "succeeds" by breaking a
consumer.

Mid-soak note (2026-07-04 09:51): daemon binary upgraded (0035-0038
fixes) via manual restart. NRestarts still 0 — manual restarts do not
count; crash-free streak unaffected. daily-check migrated off bd on
branch codex/daily-check-afctl (01bcb1f), verified: 99 tests, zero
bd/beads references, -894 lines. Merge pending operator review
(tracked as utils-4).

0028 decommission checklist addition: besides beads-dolt.service,
account for the surrounding automation — `dolt-backup.timer` and
`backup-health-check.timer` (daily Dolt backup + health check). Decide
per-unit: disable with the server, or repoint at the coordinator's
`VACUUM INTO` backups (docs/operations.md).

## Scope extension: job-scout-bot migrated (2026-07-04)

The daily-check board covered 5 Beads projects, not just utils; only
job-scout-bot had live issues (5 open incl. one P0, 1 deferred). To keep
the operator's board whole and unblock full beads-dolt decommission:

- project `jsb` registered (repo + main worktree)
- 6 live issues imported as jsb-3..jsb-8 with provenance notes
  (original Beads ids + timestamps), deferred status preserved;
  9 closed issues stay archived in
  `inputs/beads-snapshot-jsb-2026-07-04.json`
- remaining Beads projects: platform-iac and englishdrills are empty
  (0 open); aion-forge's Beads was already broken before migration
  (its bd points at an orphan Dolt on port 35437 that lacks the
  database) — nothing live to migrate, snapshot impossible until/unless
  the data dir is recovered

Found and fixed during import: 0038 made the API require an actor on
mutations, but afctl issue create/update/close never sent one — every
CLI create failed against the upgraded daemon (fix f50f962). Lesson for
review: an API-side validation change is not done until the shipped
clients are exercised against it.
