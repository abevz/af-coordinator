# Tasks — Beads migration pilot

Numbering continues the global AFC-SDD sequence.

- [ ] AFC-SDD-0023 Scout completed — data in `inputs/` (0 open issues)
- [ ] AFC-SDD-0024 Register utils project, repository, and worktree in
      af-coordinator
  - `afctl project create utils utils "Utils repo"`
  - `afctl repo create utils ~/github/utils`
  - `afctl worktree register --repo utils --path ~/github/utils`
  - verify: `afctl issue list --project utils` returns empty list
- [ ] AFC-SDD-0025 Replace utils AGENTS.md with afctl protocol adapter
  - commit on a branch in the utils repo
  - let operator review and apply
- [ ] AFC-SDD-0026 Start af-coordinatord, verify daemon stays up
- [ ] AFC-SDD-0027 Parallel soak: monitor daemon stability for >= 48h
- [ ] AFC-SDD-0028 Decommission beads-dolt (conditional on soak success)
- [ ] AFC-SDD-0029 Write review.md with migration outcome and acceptance
      verdict

Ordering: 0024 → 0025 → 0026 → 0027 → 0028 → 0029.
