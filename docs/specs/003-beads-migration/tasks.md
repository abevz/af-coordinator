# Tasks — Beads migration pilot

Numbering continues the global AFC-SDD sequence.

- [x] AFC-SDD-0023 Scout completed — data in `inputs/` (commit b15288a)
- [ ] AFC-SDD-0024 Start af-coordinatord, verify daemon stays up
      (via contrib/systemd/af-coordinatord.service)
- [ ] AFC-SDD-0025 Register utils project, repository, and worktree in
      af-coordinator
  - `afctl project add --key utils --name "Utils repo"`
  - `afctl repo add --project utils --logical-name utils --canonical-git-dir /home/abevz/github/utils`
  - `afctl worktree register --repo utils --absolute-path /home/abevz/github/utils --main`
  - verify: `afctl issue list --project utils` returns empty list
- [ ] AFC-SDD-0026 Replace utils AGENTS.md with afctl protocol adapter
  - commit on a branch in the utils repo
  - let operator review and apply
- [ ] AFC-SDD-0027 Parallel soak: monitor daemon stability for >= 48h
- [ ] AFC-SDD-0028 Decommission beads-dolt (conditional on soak success)
- [ ] AFC-SDD-0029 Write review.md with migration outcome and acceptance
      verdict

Ordering: 0024 (daemon) → 0025 (register) → 0026 (AGENTS.md) → 0027 (soak) → 0028 → 0029.
