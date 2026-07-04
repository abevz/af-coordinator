# Tasks

- [x] AFC-SDD-0001 Create initial Go module, daemon entrypoint, and CLI entrypoint
- [x] AFC-SDD-0002 Add SQLite migrations for projects, repositories, remotes, worktrees, artifacts, issues, leases, notes, and events
- [x] AFC-SDD-0003 Implement daemon boot, config loading, and health endpoint
- [x] AFC-SDD-0004 Implement project, repository, and worktree registration APIs
- [x] AFC-SDD-0005 Implement artifact-root and artifact registration APIs
- [x] AFC-SDD-0006 Implement issue create/get/list/ready APIs (short id allocation, cycle-safe ready view)
- [x] AFC-SDD-0007 Implement lease claim/release/heartbeat flow with lazy expiry
- [x] AFC-SDD-0008 Implement issue update/close with optimistic concurrency and the mutation matrix
- [x] AFC-SDD-0009 Implement issue-to-artifact linking
- [x] AFC-SDD-0010 Implement notes and issue activity timeline APIs
- [x] AFC-SDD-0011 Implement query-oriented CLI wrappers for core APIs
- [x] AFC-SDD-0012 Add systemd user service and basic operational docs (including VACUUM INTO backups)

## Closure punch list (correctness gaps found in review)

- [x] AFC-SDD-0013 Fix lease expiry comparison in ready view (BUG)
  - `ListReadyIssues` compares `expires_at > datetime('now')`, but leases
    write `expires_at` as RFC 3339 (`2026-07-03T19:35:00Z`) while
    `datetime('now')` yields `2026-07-03 19:35:00`. Lexicographically
    `'T' > ' '`, so a lease that expired earlier the same day still counts
    as active and hides the issue from ready.
  - Fix: pass `time.Now().UTC().Format(time.RFC3339)` as a query parameter
    (or use `strftime('%Y-%m-%dT%H:%M:%SZ','now')`); audit the whole store
    for other `datetime('now')` comparisons against RFC 3339 columns.
  - Also fix test fixtures that seed timestamps with `datetime('now')`:
    they violate the schema time contract and are exactly why the suite
    did not catch this. Fixtures must write RFC 3339. Add a regression
    test: lease expired same-day → issue is ready again.
- [x] AFC-SDD-0014 Implement `blocks` dependency filtering in the ready view
  - `ListReadyIssues` still has "deferred to SDD-0008" in a comment, but
    SDD-0008 shipped without it. Per spec, an issue with an unfinished
    `blocks` dependency must not be ready. Add a `NOT EXISTS` subquery
    joining `dependencies` (kind = 'blocks') to blocker issues whose
    status is not `done`/`cancelled`, plus tests for blocked/unblocked
    transitions.
- [x] AFC-SDD-0015 Complete event coverage per architecture spec
  - Events are currently appended only by `UpdateIssue` and `CloseIssue`.
    The spec requires: issue created, issue claimed, note added,
    dependency added/removed. Release and lease-expiry sweep should also
    emit. Heartbeats must stay event-free.
- [x] AFC-SDD-0016 Enforce artifact kind validation on write
  - `ValidateArtifactKind` exists but is not called on the write path.
- [ ] AFC-SDD-0036 Tighten project key validation (it is the short-id prefix)
  - found by operator: `validProjectKey` (`^[a-z][a-z0-9-]*$`) accepts a
    51-char repo name as a key, so every issue id would carry it; it also
    accepts trailing and doubled hyphens (`saa-` → short_id `saa--1`)
  - cap length (max 16), reject trailing hyphen and `--` runs:
    `^[a-z][a-z0-9]*(-[a-z0-9]+)*$` plus length check
  - error message must say WHY: "key becomes the issue prefix
    (<key>-<n>); keep it short"
  - document in schema-v1.md and api-v1.md; table-driven tests incl. the
    51-char and `saa-` cases
  - existing keys are unaffected (validation is create-time only)
- [ ] AFC-SDD-0037 Resolve short_id in every issue endpoint, not only GetIssue
  - api-v1.md promises "issues are addressable by short_id everywhere an
    {issue_id} appears"; today only `GetIssue` has the short_id fallback.
    Claim/heartbeat/release/close/update/notes/links/dependencies/events
    query the raw UUID — hit live: codex's claim of `utils-1` returned
    not_found and it fell back to the UUID (see utils-1 note)
  - fix once: `ResolveIssueID(db, idOrShortID) (uuid, error)` in the
    store, called at the top of every handler with an `{issue_id}`;
    remove the ad-hoc fallback inside GetIssue
  - also: `ListNotes`/`ListEvents` on an unknown issue must return
    not_found, not a silent empty list
  - tests: table-driven across every endpoint — short_id accepted,
    unknown id → not_found
- [ ] AFC-SDD-0038 Record the real actor in events (audit trail is lying)
  - `issue_created`, `issue_updated`, `issue_closed` events hardcode
    actor `"unknown"` (issues.go lines ~87/~595/~683); only
    `issue_claimed` records the real holder. The event log currently
    cannot answer "who did what" — its entire purpose
  - afctl already resolves and sends actor (0017); thread `req.Actor`
    from the handlers through the store insert paths into the events
  - reject mutations with empty actor at the API layer
    (validation_failed), matching the protocol's actor requirement
  - tests: each mutation's event carries the submitted actor; mutation
    without actor → 400

## Next tracks (after the punch list — nothing below starts without its own spec packet)

1. `docs/specs/002-agent-protocol/` — agent working contract + ready-made
   hook snippets (Claude Code, Codex). Prerequisite: punch list done.
2. `docs/specs/003-beads-migration/` — pilot migration of `~/github/utils`
   off Beads/Dolt: register project/repo/worktrees, import open issues,
   switch repo AGENTS.md to `afctl`. This is the v1 acceptance test.
3. daily-check board provider switch (lives in the utils repo, not here).

Explicitly not now: MCP server, GitHub mirror, TUI, JSONL/markdown export
beyond what exists.
