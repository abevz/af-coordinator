
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
