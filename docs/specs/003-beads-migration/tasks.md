# Tasks — Beads migration pilot

Numbering continues the global AFC-SDD sequence.

- [ ] AFC-SDD-0023 Scout Beads issues: list open issues, note count,
      and status distribution
- [ ] AFC-SDD-0024 Write import script:
  - reads `beads issue list --json`
  - for each open issue, maps UTL-N → utils-N and calls
    `afctl issue create` and `afctl issue note add`
  - idempotent: skips existing utils-N short ids
- [ ] AFC-SDD-0025 Register utils project and repository in af-coordinator
  - `afctl project create` and `afctl repo create`
  - verify with `afctl issue list --project utils`
  - `afctl worktree register`
- [ ] AFC-SDD-0026 Run import, verify all issues and notes landed
- [ ] AFC-SDD-0027 Switch utils AGENTS.md to afctl protocol
  - commit on a branch in the utils repo
  - PR/review cycle
- [ ] AFC-SDD-0028 Parallel soak: monitor daemon stability for >= 48h
- [ ] AFC-SDD-0029 Decommission beads-dolt (conditional on soak success)
- [ ] AFC-SDD-0030 Write review.md with migration outcome and acceptance
      verdict

Ordering: 0023 → 0024 → (0025 + 0026) → 0027 → 0028 → 0029 → 0030.
