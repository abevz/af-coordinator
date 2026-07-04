# Tasks — Beads migration pilot

Numbering continues the global AFC-SDD sequence.

- [x] AFC-SDD-0023 Scout completed — data in `inputs/` (commit b15288a)
- [x] AFC-SDD-0024 Start af-coordinatord, verify daemon stays up
      (via contrib/systemd/af-coordinatord.service)
- [x] AFC-SDD-0031 Fix `worktree register --repo` to accept logical name
      (`utils`) in addition to UUID — `GetRepo` now queries both `id`
      and `logical_name`
      Files: internal/store/sqlite/repos.go
- [x] AFC-SDD-0025 Register utils project, repository, and worktree in
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

Ordering: 0024 → 0031 → 0025 → 0026 → 0027 → 0028 → 0029.
