# Design — Beads migration pilot

## Phases

### Phase 1: registration

```text
afctl project create utils utils "Utils repo"
afctl repo create utils ~/github/utils
afctl worktree register utils --repo utils --path ~/github/utils
```

The project key is `utils`. The repo is registered with its on-disk path.
The main working tree is registered as the default worktree.

### Phase 2: import

For each open Beads issue (from `beads issue list --json`):

1. Map `UTL-N` → `utils-N` as the short id
2. Create issue via `afctl issue create` with the original title,
   description, status (mapped to open/in_progress), and priority
3. For each note on the Beads issue, create a coordinator note via
   `afctl issue note add`

Mapping:

| Beads status | Coordinator status |
|---|---|
| open, triage | `open` |
| in_progress | `in_progress` |
| blocked | `blocked` |
| closed, done, cancelled | skip (non-goal) |

The issue ID in the coordinator is `utils-N` where N is the Beads
sequence number. The import script must check for duplicate short ids
before creating (idempotency).

### Phase 3: switchover

Replace `~/github/utils/AGENTS.md` content:

- Remove/replace Beads-specific instructions
- Add the coordinator adapter snippet from `contrib/agents/AGENTS-snippet.md`
- Reference `docs/agent-protocol-v1.md` for the full protocol

The AGENTS.md changes are committed to the utils repo on a branch,
not on main directly.

### Phase 4: parallel soak

Both systems stay writable for >= 48 hours. The coordinator daemon
runs alongside beads-dolt. Agents may use either system.

Monitor:
- Daemon uptime and crash frequency
- Lease enforcement in check-lease.sh
- Issue claim/release latency

### Phase 5: decommission (conditional)

If the daemon has required less babysitting than beads-dolt:

1. Set beads-dolt to read-only mode (service stop)
2. Document the outcome in review.md
3. Declare v1 acceptance test passed

If the daemon HAS required more babysitting, note failures in review.md
and defer decommission.

## Data model

The utils project uses the default coordinator schema:

- Project: `utils`, key `utils`
- Repository: `~/github/utils`, id mapped to project
- Issues: short ids `utils-N`, fields map from Beads schema
- Notes: preserved from Beads, imported sequentially

## Rollback

If the migration needs to be reversed:

1. Keep beads-dolt running (it was never stopped)
2. Restore the original `AGENTS.md` from git
3. The coordinator database can be dropped or ignored

No data is lost in either direction because the coordinator never writes
to Beads and Beads never reads the coordinator.
