# Design

## Layering

```text
agent behavior   docs/agent-protocol-v1.md   (what to do)
machine surface  afctl --json + exit codes   (how to do it programmatically)
enforcement      contrib/hooks/*             (what happens if you don't)
per-repo glue    contrib/agents/* snippet    (one pointer, no restating)
```

## CLI: --json and exit codes

- Global flag parsed before command dispatch; stored on the client
  printer, not threaded through every function signature.
- Success: exactly one JSON document on stdout (object for single
  resources, array for lists). Human format stays the default.
- Errors: the daemon's error envelope passes through to stderr verbatim
  when `--json` is set; the exit code is derived from `error.code`:

| error.code         | exit |
|--------------------|------|
| (success)          | 0    |
| any transport/usage failure | 1 |
| `version_conflict` | 2    |
| `lease_held`       | 3    |
| `lease_expired`    | 4    |
| `not_found`        | 5    |
| `dependency_cycle` | 6    |

- Actor identity: mutating commands resolve actor as flag >
  `AF_COORDINATOR_ACTOR` > error. The protocol doc tells each agent to
  export a stable name (`claude-code`, `codex`, `codewhale-<n>`).

## Protocol document

`docs/agent-protocol-v1.md`, hard cap 150 lines, structured as:

1. Session loop (the normative core):
   - `afctl issue ready --json` → pick highest-priority item you can do
   - `afctl issue claim <id> --ttl 900` → hold the lease token
   - heartbeat every TTL/3 (300s for the default 900s TTL)
   - `afctl issue note add` for material findings; always leave a final
     `HANDOFF:` note when stopping without closing
   - `afctl issue close <id> --resolution done` or `release` at session end
2. Exit-code reaction table (from requirements).
3. Scope rules: claim before mutating files that belong to the issue;
   one issue claimed per agent at a time unless tasks are trivially
   coupled.
4. What not to do: no direct SQLite access, no editing without a claim
   in coordinated repos, no restating specs in issue descriptions.

## Hooks

`contrib/hooks/claude-code/`:

- `check-lease.sh` — called from a PreToolUse hook on Edit/Write/Bash
  mutations; queries `afctl issue list --json --status in_progress` and
  greps for a lease held by `$AF_COORDINATOR_ACTOR` scoped to the current
  repo; exit 0 = allow, exit 2 = block with message.
- `settings-snippet.json` — the one-line hook registration to paste into
  `.claude/settings.json`.
- Mode via env: `AF_HOOK_MODE=warn|block` (default `warn`).
- Positive results cached in `/tmp` keyed by repo+actor for 60s to keep
  overhead off the editing hot path.

`contrib/hooks/codex/` — same check script, Codex hook registration
format.

## Per-repo adapter

`contrib/agents/AGENTS-snippet.md` — ≤15 lines, to paste into a repo's
`AGENTS.md`/`CLAUDE.md`:

- this repo is coordinated by af-coordinator
- export `AF_COORDINATOR_ACTOR`
- follow `docs/agent-protocol-v1.md` (link), summary: ready → claim →
  heartbeat → note → close/release
- never bypass `afctl`/API for writes

## End-to-end test

Go integration test (build tag or `internal/api` test) that starts the
daemon on a temp socket, runs two concurrent claim attempts on one issue,
and asserts exactly one success and one `lease_held`. CLI exit codes get
table-driven unit tests against a stub server.

## Risks

- Hook false negatives (repo not registered, daemon down): hooks must
  fail open in `warn` mode and say why, or agents will disable them.
- Protocol doc drift from CLI reality: the doc quotes real commands, so
  CI or review must re-check it whenever `afctl` flags change.
