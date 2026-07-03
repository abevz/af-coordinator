# Tasks

- [x] AFC-SDD-0001 Create initial Go module, daemon entrypoint, and CLI entrypoint
- [ ] AFC-SDD-0002 Add SQLite migrations for projects, repositories, remotes, worktrees, artifacts, issues, leases, notes, and events
  - schema drafted in `migrations/0001_schema_v1.sql`; embedded migration
    runner and application at daemon boot still pending
- [ ] AFC-SDD-0003 Implement daemon boot, config loading, and health endpoint
  - health endpoint and config defaults exist; DB open, pragmas, and
    migration application at boot still pending
- [x] AFC-SDD-0004 Implement project, repository, and worktree registration APIs
- [x] AFC-SDD-0005 Implement artifact-root and artifact registration APIs
- [x] AFC-SDD-0006 Implement issue create/get/list/ready APIs (short id allocation, cycle-safe ready view)
- [x] AFC-SDD-0007 Implement lease claim/release/heartbeat flow with lazy expiry
- [x] AFC-SDD-0008 Implement issue update/close with optimistic concurrency and the mutation matrix
- [ ] AFC-SDD-0009 Implement issue-to-artifact linking
- [ ] AFC-SDD-0010 Implement notes and issue activity timeline APIs
- [ ] AFC-SDD-0011 Implement query-oriented CLI wrappers for core APIs
- [ ] AFC-SDD-0012 Add systemd user service and basic operational docs (including VACUUM INTO backups)
