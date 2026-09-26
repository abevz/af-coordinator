# 018 GitHub Import Review

## Owner direction (2026-09-26)

- The owner judged that GitHub integration matters for attracting users and
  chose a thin slice over the full stage 4–5 work in the
  [016 plan](../016-adoption/implementation-plan.md): import by URL and one
  result comment, no attachments store, no source-change tracking. Target
  release: `v0.1.0-rc.4`. The public launch post waits for this slice; a small
  circle of testers can start on rc.3.
- `afc-89`, `afc-90`, and `afc-135` remain the full workflow and attachments
  work and are not closed by this packet.

## Design decisions

- **`gh` instead of a built-in HTTP client.** It reuses the user's auth,
  including private repositories and SSO; dibs stores no GitHub token; the
  daemon remains offline and local-only.
- **No schema change.** Idempotent import uses a lookup by external key plus a
  deterministic create operation ID; exactly-once publish uses a marker in the
  GitHub comment instead of a local ledger.
- **Readiness without a probe repository.** `dibs doctor` checks `gh`
  installation, authentication, and a working authenticated API call
  (`rate_limit`). A public probe repository would prove only connectivity;
  per-repository access is checked by `import` and `publish` (owner request,
  2026-09-26).
- **Accepted gap.** Two simultaneous publishes of the same close can post
  twice. A durable ledger is left to `afc-90`.

## Amendment: MCP and protocol (2026-09-26)

- Owner direction: the GitHub workflow must be part of the agent protocol
  and usable through MCP. MCP moved from out of scope into slice `afc-169`
  (R-14, R-15), and the real round trip now covers `dibs-mcp` as well.
- Verified against the code before writing: `dibs-mcp` is a local stdio
  client (`cmd/dibs-mcp/main.go` wires `internal/client`), so it may call
  `gh` without giving the daemon network access. Import and publish live in
  package `main` today, hence the `internal/ghsync` move. The MCP server
  constraint "thin wrappers over `internal/client`" is amended for these two
  tools only. `dibs protocol` embeds `docs/agent-protocol-v1.md`, and a test
  keeps the two identical.
- MCP `import_issue` requires `project`: a stdio server's working
  directory is chosen by the client and is not a reliable repository target.
- Owner approved this amendment on 2026-09-26 (in session, relayed by the
  agent); `afc-169` may start after `afc-164` merges.

## Amendment: Claude Code and Codex integration (2026-09-26)

- Owner direction: the workflow must be visible where Claude Code and Codex
  meet dibs, not only in the protocol and MCP. R-16 extends `afc-169` to the
  SessionStart context and `contrib/hooks/README.md`. R-12 now requires real
  `claude -p` and `codex exec` sessions and one MCP import from Codex.
- Checked against the code: `hookSessionStart` in `cmd/dibs/cmd_hooks.go`
  prints `- <short_id>: <title>` for up to ten ready issues. `core.Issue`
  already carries `external_key`. The context text comes from issue titles,
  which for imported issues are GitHub content; the hook therefore renders
  titles on one line and states the trust rule.
- `issue run --publish` posts from the parent process, so agent sandboxes
  that block network access do not affect publication. Whether Codex applies
  its sandbox to MCP servers is not established; afc-165 verifies it.
- Owner approved this amendment on 2026-09-26 (in session, relayed by the
  agent).

## Approval

- [x] Owner approved this packet on 2026-09-26 (in session, relayed by the
  agent); implementation of `afc-163`–`afc-165` may start.

## Evidence

### afc-169 implementation evidence (2026-09-26)

- Moved target-independent import, close-event selection, publication, and
  token text checks from CLI code into `internal/ghsync`. CLI parsing and
  checkout target resolution remain in `cmd/dibs`; no daemon, API, store, or
  schema changes. Existing afc-163/afc-164 CLI tests were left unchanged.
- Added MCP `import_issue`, `publish_issue`, and `close_issue.publish` with a
  fake-injectable GitHub client. A repeated close operation still attempts
  publication; the existing close marker prevents a second comment. CLI and
  MCP share GitHub error codes and retain the concise `gh` stderr text.
- Added the GitHub protocol section, matching embedded protocol, MCP tool
  documentation, managed AGENTS line, and Claude Code/Codex hook guide.
  SessionStart adds the source label and guidance only when a displayed task
  has a GitHub key. Owner clarified that output without GitHub sources is
  byte-for-byte unchanged for titles without control characters; titles with
  control characters are sanitized even without GitHub sources.
- The CLI reports a successful local close before a publication failure.
  Exact token text refusal has code `secret_in_text` and no retry hint, since
  the recorded note or branch cannot be changed after close.
- Final review found that replaying an old MCP `close_issue` operation after
  reopen and reclose could publish the newer close. Owner chose to preserve
  the local replay result, return `already` if the old close marker exists,
  or return `stale_close` without posting. The implementation matches the
  replayed `closed_at` and lease generation against close events; a same-second
  reclose is therefore also disambiguated. A second review found that a later
  operator close needed the same stale handling and that rereading events
  after selecting the replayed close could publish a newer close. Publication
  now renders from the selected event using one event-list read. Focused tests
  cover both stale outcomes, later operator-close, same-second closes, and the
  MCP test daemon replay after a newer close.
- Focused tests covered shared import/publish mapping, moved GitHub comment
  URL, idempotent marker, token refusal, MCP import and close replay against a
  temporary test daemon, missing-key preflight, locked source, GitHub error
  classes, SessionStart output, and CLI close output order. `go build ./...`
  and `go test ./...` passed before final review; final validation logs are
  `/tmp/dibs-afc169-build-final.log` and `/tmp/dibs-afc169-test-final.log`.
  `cmp` confirmed the protocol source and embedded copy are identical;
  `git diff --check` passed.
- PR #117 follow-up: CI found an unused CLI wrapper after the move to
  `internal/ghsync`; it was removed. Missing GitHub keys now report
  `validation_failed` in MCP and CLI JSON before local close. The human
  `secret_in_text` message states once that the close cannot be published.
  Focused regression tests and the CI lint command passed locally with a
  temporary Go 1.27-built `golangci-lint` v2.14.0.
- A subsequent PR #117 CI run found a data race in the CLI publish test
  fixture. Its HTTP handler and tests now synchronize mutable issue, event,
  note, and request-count state. `go test -race ./cmd/dibs/` passed locally;
  the run log is `/tmp/dibs-afc169-pr117-race-cmd.log`.
- Intentionally not run: real GitHub API, real Claude Code/Codex sessions,
  cross-compilation, release dry-run, and rc.4 publication. Those belong to
  `afc-165` or the owner release step.

### afc-164 owner decisions (2026-09-26)

- The CLI-only `hooks complete` marker may carry PR URL, commit SHA, branch,
  and note. Nonempty values override `issue run` launch flags; no flags keep
  the legacy marker. This is an approved extension so an agent can record a
  PR created during its run.
- Publication markers use `issue_closed` event ID, not its second-resolution
  timestamp. Comment list and POST use the fetched issue's `comments_url` to
  tolerate repository transfer or rename.
- Missing `github:` keys stop `issue close --publish` before close and
  `issue run --publish` before claim. After a successful local close, a
  publication failure does not change close success; JSON carries structured
  publication status. Explicit publish fails nonzero when it cannot post.
- `locked` always stops preflight, even if the caller has write access. This
  is an accepted simplification for this slice.
- An explicit publish of an operator-closed issue is outside R-07 for this
  slice. The reader rejects a latest `issue_operator_closed` event so it
  cannot accidentally publish a preceding ordinary close after reopen.
- A close note is used only when its `note_added` event immediately precedes
  the latest `issue_closed` with the same actor and timestamp. The API gives
  notes second-resolution times and no link to the close event, so another
  note by that author in the same second remains ambiguous. Adding `note_id`
  to the close payload is follow-up work for `afc-90`.
- The note is public, line quoted, capped at 2,000 runes with a final `…`.
  Markdown and @mentions remain as written. The branch is also public.
  Exact nonempty values of the three current lease/operator token env vars
  are rejected if present in either field. Close/run also reject their exact
  active lease token even when the parent environment does not hold it; no
  heuristic cleaning is applied.
- `gh api --slurp` first appears in [GitHub CLI 2.48.0](https://github.com/cli/cli/releases/tag/v2.48.0);
  doctor warns for older versions. `cancelled` closes publish; handoffs and
  lease expiry do not.

### afc-164 implementation evidence

- CLI-only `issue publish`, `issue close --publish`, `issue run --publish`,
  extended `hooks complete`, GitHub comment client, and doctor version guard
  were implemented without daemon, API, store, or schema changes.
- Focused tests covered preflight before close/claim, event and note selection,
  marker idempotency across two closes in one second, moved-repository
  `comments_url`, locked/not-found, token rejection, line quoting and rune
  truncation, hook metadata override, JSON result shape, and publish failure
  after successful close for both close and run. Both close/run retained a
  successful exit; explicit publish exited nonzero on the same fake `gh`
  failure.
- `gofmt` and `go build ./...` passed; `go test ./...` passed. Full logs:
  `/tmp/dibs-afc164-full-build.log` and `/tmp/dibs-afc164-full-test.log`.
- A temporary real `dibsd` with isolated HOME, DIBS_DB, DIBS_SOCKET and fake
  `gh` imported `o/r#1`, ran `issue run --require-complete --publish` with
  PR URL, branch and note set by `hooks complete`, then repeated
  `issue publish --json`. The fake GitHub state held one comment with the
  PR, note, and close-event marker; repeat reported `already: true`; local
  issue status was `done`. Scratch state: `/tmp/dibs-afc164.erBTHy`.
- A real GitHub API round trip, release binaries, and owner-created rc.4 tag
  were intentionally not tested here; they belong to `afc-165`.

### afc-163 (implementation in review)

- Owner clarifications: with explicit `--project`, scope follows presence of
  `--repo`; descriptions remain complete because the server has no limit;
  UUIDv5 uses `uuid.NameSpaceURL` and the name
  `dibs:issue-import:<project_id>:<external_key>`.
- Added a CLI-only GitHub client, source parser, import command, and optional
  doctor check. Daemon, API, store, and migrations are unchanged.
- Focused tests: `go test ./cmd/dibs -run 'TestIssueImport|TestImportOperationID|TestMapExitCodeErr'`
  and `go test ./internal/github ./internal/doctor` passed. Full verification:
  `go build ./...` and `go test ./...` passed.
- Scratch daemon with temporary HOME, DIBS_DB, and short DIBS_SOCKET plus fake
  `gh`: new and repeated import, two concurrent imports (one dibs issue),
  closed source rejection and `--allow-closed`, PR rejection, current-checkout
  target resolution, doctor success and all three warning points, and GitHub
  error-class JSON output passed. No test called the real GitHub API.
- Real GitHub round trip, publish, README/protocol updates, tag, and release
  remain in `afc-164`/`afc-165` or owner work as described in tasks.md.
- PR #113 follow-up: current-checkout target resolution now reuses
  `firstuse.SameRegisteredGitDir` and recognizes legacy checkout paths;
  `ParseIssueRef` accepts one trailing slash; typed `github.Error` includes a
  short, one-line `gh` stderr in human and JSON messages. Focused regression
  tests cover all three changes. `go build ./...` and `go test ./...` passed
  after the fix. A scratch daemon recognized a legacy checkout registration
  from a linked worktree and returned the expected JSON import and error.
