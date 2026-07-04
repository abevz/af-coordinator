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
- [x] AFC-SDD-0036 Tighten project key validation (it is the short-id prefix)
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
- [x] AFC-SDD-0037 Resolve short_id in every issue endpoint, not only GetIssue
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
- [x] AFC-SDD-0038 Record the real actor in events (audit trail is lying)
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

## Hardening follow-ups (external audit, 2026-07-04)

Source: Go audit by Google Antigravity; full report with the operator.
Mechanical quality work, no behavior changes.

- [x] AFC-SDD-0039 Makefile: `test` runs with `-race`; add `vet` target
- [x] AFC-SDD-0040 Thread `context.Context` through the store and API
  - every exported `internal/store/sqlite` function takes `ctx` first;
    switch to `QueryContext`/`ExecContext`/`BeginTx`; handlers pass
    `r.Context()`
  - why: cancelled clients and graceful shutdown currently cannot
    interrupt in-flight SQL
  - mechanical but wide — single dedicated change, no drive-by edits
- [x] AFC-SDD-0041 `context.Context` in `internal/client` methods
  - `http.NewRequestWithContext`; afctl passes a signal-aware context
    (Ctrl+C cancels instead of waiting out the 5s timeout)
  - depends on 0040 landing first (same signature churn)
- [x] AFC-SDD-0042 Split `cmd/afctl/main.go` (~1700 lines) by domain
  - `main.go` (entry, global flags) + `cmd_issue.go`, `cmd_project.go`,
    `cmd_repo.go`, `cmd_worktree.go`, `cmd_artifact.go`, plus existing
    `init.go`/`protocol.go`
  - pure file moves — no flag-parsing rewrite in this task; adopting
    `flag.NewFlagSet` is a separate decision, not started without a task
- [x] AFC-SDD-0043 README "Getting started" section
  - build (`make build`), run, test (`make test`), configuration env
    vars (`AF_COORDINATOR_SOCKET`/`DB`/`LOG_LEVEL`), install
    (`make build-install install-service`)
- [x] AFC-SDD-0044 Format `cmd/afctl/main.go`
  - file has inconsistent spacing/formatting; run `gofmt -w` to fix
- [x] AFC-SDD-0045 Remove hardcoded DDL schema from tests
  - `internal/api/api_test.go` and `internal/store/sqlite/sqlite_test.go` use an inlined string `schema` with hand-written DDL
  - use `sqlite.Migrate` with `migrations.FS` to initialize in-memory SQLite for tests, matching the AGENTS.md rule
- [x] AFC-SDD-0046 Add golangci-lint target to Makefile
  - add `lint: golangci-lint run`
- [x] AFC-SDD-0047 Update README.md
  - Add "How to release" and "Known limitations" sections
  - Remove outdated "Recommended implementation order" section
- [x] AFC-SDD-0048 Refactor afctl error handling
  - Return `error` from handler functions instead of abruptly calling `os.Exit(1)`. This makes testing easier and the codebase more idiomatic.
- [x] AFC-SDD-0052 Backup automation and verified restore path
  - beads-dolt had `dolt-backup.timer` + `backup-health-check.timer`;
    they are decommissioned and the coordinator has NO automated backup —
    only a manual `VACUUM INTO` recipe in operations.md
  - ship `contrib/systemd/af-coordinator-backup.service` (oneshot) +
    `af-coordinator-backup.timer` (daily, off-peak minute, not :00):
    `VACUUM INTO $BACKUPDIR/af-coordinator-YYYYMMDD-HHMM.db`, then
    `PRAGMA integrity_check` ON THE BACKUP FILE (a backup that was never
    opened is a hope, not a backup), prune to last 14
  - hard rule from the Dolt post-mortems (vault:
    BASE/Beads-Dolt-Sync-Troubleshooting.md): backups must live OUTSIDE
    the live data dir — use `~/backups/af-coordinator/` (Makefile
    `BACKUPDIR` already points there, currently unused)
  - Makefile: `install-backup` target; operations.md: replace the cron
    suggestion with the timer instructions
  - fix operations.md env-var names while there: it documents
    `AF_COORDINATOR_DB_PATH`/`AF_COORDINATOR_SOCKET_PATH`, the real ones
    per internal/config/config.go are `AF_COORDINATOR_DB`/
    `AF_COORDINATOR_SOCKET` (verify against code, not this task)
  - RESTORE DRILL, not just docs: restore latest backup to a scratch
    path, start a second daemon against it with env overrides
    (socket in /tmp — mind the ~108-byte unix socket path limit),
    `afctl health` + `ls --project utils` against that socket must
    return real data; record the drill outcome in review.md
  - vault runbook: new note in Obsidian
    `BASE/AF-Coordinator-Backup-Restore.md` (schedule, paths, restore
    steps, verification, drill date); add a deprecation pointer at the
    top of `BASE/Beads-Dolt-Sync-Troubleshooting.md` (beads-dolt
    decommissioned 2026-07-04 → link to the new runbook)
- [x] AFC-SDD-0054 SECURITY: `GET /v1/issues/{id}` leaks the lease_token
  - found live: `afctl --json show afc-2` returned another session's
    full `lease_token`. The protocol declares the token secret ("proves
    your right to mutate"); if any client can read it via GET, any agent
    can hijack any claim — the 0022 concurrency guarantee (B cannot act
    on A's lease) is void
  - fix: issue GET/list responses expose `holder` and `expires_at`
    only; the token appears exactly once, in the claim response.
    Audit every response struct for the field (issue get, list, ready,
    heartbeat response)
  - tests: show/list a claimed issue → token absent, holder present;
    claim response still carries it
- [ ] AFC-SDD-0055 Detect client/daemon version skew
  - found live: agy shipped 0053, installed the new afctl, did not
    restart the daemon — `close --note` silently dropped notes for an
    hour (old server ignored the unknown JSON field). Silent partial
    upgrade is a standing failure mode with several clients and one
    daemon
  - embed a build/schema version in both binaries; `/v1/health` returns
    the daemon's; afctl compares on every invocation (cheap — it
    already opens the socket) and prints one warning line to stderr on
    mismatch: "afctl <v> != daemon <v>; restart af-coordinatord"
  - tests: mismatch → warning on stderr, exit code unaffected;
    match → silent
- [ ] AFC-SDD-0056 Identifier resolution is still inconsistent across endpoints
  - found while linking issues to spec artifacts:
    (a) `issue link <short_id>` → not_found; by UUID → works. 0037
    listed links in scope and is marked done, but the links handler was
    missed — reopen that slice;
    (b) `artifact list --repo <logical-name>` → silent `[]` while
    `artifact register --repo <logical-name>` resolves fine; by UUID
    list works. Same disease as 0031/0037, third occurrence
  - principle to enforce once: every endpoint accepts the human-facing
    identifier (short_id for issues, logical name for repos, key for
    projects); unknown identifier → not_found, NEVER a silent empty list
  - audit ALL handlers against that principle in one pass (issues,
    leases, notes, events, links, dependencies, artifacts, worktrees),
    table-driven test per endpoint × {friendly id, uuid, unknown}
  - bonus: `show --full` prints events and notes but not artifact
    links — add a Links section

Deferred from the audit, deliberately: CI pipeline (GitHub Actions) —
worth doing before the repo is shared, not before Monday's soak
verdict; CLI-layer tests — cmd/ stays thin and untested per AGENTS.md
testing policy until that policy is revisited.

## Next tracks (after the punch list — nothing below starts without its own spec packet)

1. `docs/specs/002-agent-protocol/` — agent working contract + ready-made
   hook snippets (Claude Code, Codex). Prerequisite: punch list done.
2. `docs/specs/003-beads-migration/` — pilot migration of `~/github/utils`
   off Beads/Dolt: register project/repo/worktrees, import open issues,
   switch repo AGENTS.md to `afctl`. This is the v1 acceptance test.
3. daily-check board provider switch (lives in the utils repo, not here).

Explicitly not now: MCP server, GitHub mirror, TUI, JSONL/markdown export
beyond what exists.
