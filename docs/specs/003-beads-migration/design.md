# Design — Beads migration pilot

## Phases

### Phase 1: registration

```text
afctl project add --key utils --name "Utils repo"
afctl repo add --project utils --logical-name utils --canonical-git-dir /home/abevz/github/utils
afctl worktree register --repo utils --absolute-path /home/abevz/github/utils --main
```

The project key is `utils`. The repo is registered with its on-disk path.
The main working tree is registered as the default worktree.

### Phase 2: archive decision

All 7 Beads issues are closed. Full state is in
`inputs/beads-snapshot-2026-07-04.json`.

Decision: **the coordinator project starts empty.** The JSON snapshot is
kept in git as the permanent archive. No import script, no migration of
closed issues.

Rationale: importing 7 closed issues into the coordinator adds zero
operational value — they will never be claimed, heartbeated, or closed
again. The snapshot is sufficient for audit.

### Phase 3: switchover

Replace `~/github/utils/AGENTS.md` content:

- Remove the Beads workflow section (including the BEADS INTEGRATION
  block)
- Run `afctl init` to install the managed coordinator block
- Reference `docs/agent-protocol-v1.md` for the full session loop

The AGENTS.md changes are committed to the utils repo on a branch,
not on main directly. The commit is prepared by the agent but applied
by the operator.

### Phase 4: parallel soak

Only the coordinator accepts mutations. beads-dolt stays running
untouched as a rollback safety net for >= 48 hours.

Monitor:

- Daemon uptime and crash frequency
- Lease enforcement in check-lease.sh
- Issue claim/release latency
- Compare to the 914-restart benchmark from beads-dolt

### Phase 5: decommission (conditional)

If the daemon has required less babysitting than beads-dolt:

1. Stop beads-dolt service
2. Document the outcome in review.md
3. Declare v1 acceptance test passed

If the daemon HAS required more babysitting, note failures in review.md
and defer decommission.

## Data model

The utils project uses the default coordinator schema:

- Project: `utils`, key `utils`
- Repository: `~/github/utils`, id mapped to project
- Issues: none (project starts empty)
- Archive: `docs/specs/003-beads-migration/inputs/beads-snapshot-*.json`

## Rollback

If the migration needs to be reversed:

1. Keep beads-dolt running (it was never stopped)
2. Restore the original `AGENTS.md` from git (`git checkout HEAD~1 -- AGENTS.md`)
3. The coordinator database can be dropped or ignored

No data is lost in either direction because the coordinator never writes
to Beads and Beads never reads the coordinator.
