
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
