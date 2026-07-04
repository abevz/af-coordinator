# Tasks

Numbering continues the global AFC-SDD sequence.

- [x] AFC-SDD-0017 Add `--json` output and typed exit codes to afctl
  - global flag, JSON success on stdout, error envelope passthrough on
    stderr, exit-code table from design.md
  - actor resolution: flag > `AF_COORDINATOR_ACTOR` > error for mutations
  - table-driven tests for exit-code mapping (per AGENTS.md testing policy)
- [x] AFC-SDD-0018 Write `docs/agent-protocol-v1.md`
  - session loop, exit-code reaction table, scope rules, prohibitions
  - hard cap 150 lines; every quoted command must actually work
- [x] AFC-SDD-0019 Claude Code hook snippet (`contrib/hooks/claude-code/`)
  - `check-lease.sh` with warn|block modes, 60s positive cache,
    fail-open-with-reason when daemon is unreachable in warn mode
  - `settings-snippet.json` one-line registration
- [x] AFC-SDD-0020 Codex hook snippet (`contrib/hooks/codex/`)
  - same check script, Codex registration format
- [x] AFC-SDD-0021 Per-repo adapter snippet (`contrib/agents/AGENTS-snippet.md`)
  - ≤15 lines, references the protocol doc, never restates it
- [x] AFC-SDD-0022 End-to-end concurrency test
  - two concurrent claims on one issue: exactly one success, one
    `lease_held` (exit 3 via CLI path or 409 via API path)
- [x] AFC-SDD-0032 Add `afctl protocol` subcommand (go:embed agent-protocol-v1.md,
      so the contract travels with the binary)
- [x] AFC-SDD-0033 Add `afctl init` — wire a repo's AGENTS.md into the coordinator
  - writes a managed block into `./AGENTS.md` between markers
    `<!-- BEGIN AF-COORDINATOR INTEGRATION v:1 -->` /
    `<!-- END AF-COORDINATOR INTEGRATION -->`; block content is the
    embedded adapter snippet from 0032 (single source, no second copy)
  - idempotent: no AGENTS.md → create with the block; block absent →
    append; block present → replace in place (this is how a repo picks
    up a newer protocol after `afctl` upgrade); text outside the
    markers is never touched
  - prior art: Beads' `BEGIN/END BEADS INTEGRATION v:1 hash:` block
  - supports `--json`; standard exit codes; prints what it did
    (created | updated | unchanged)
  - out of scope: project/repo/worktree registration (those commands
    exist), hook installation, touching any file other than AGENTS.md
  - tests (per AGENTS.md testing policy): table-driven over the four
    states — missing file, file without block, stale block, current
    block (must be a no-op)
- [ ] AFC-SDD-0034 `afctl init --dry-run` output must be distinguishable
      from a real run
  - found by operator: `--dry-run` prints `updated: AGENTS.md` — byte-identical
    to the real run; `runInit` ignores `dryRun` in both text and `--json` output
  - text mode: prefix with `would ` (`would update: AGENTS.md`);
    json mode: add `"dry_run": true`
  - regression test: dry-run output differs from real output AND leaves
    the file untouched (assert file bytes unchanged)
- [ ] AFC-SDD-0035 Show active lease holder in issue lists
  - found by operator: `afctl issue list` renders an ASSIGNEE column
    (advisory, almost always empty) but not the lease holder — the one
    thing you actually want to know about an in_progress issue
  - store: list queries LEFT JOIN unexpired leases, expose
    `holder`/`lease_expires_at` on list items (lazy-expiry rule applies:
    expired lease = absent)
  - API: include the fields in `GET /v1/issues` and `/v1/issues/ready`
    items; update docs/api-v1.md
  - CLI: `issue list`/`ls`/`issue ready` print a CLAIMED column
    (`codex` or empty); keep ASSIGNEE
  - tests: listed issue with active lease shows holder; with expired
    lease shows empty (regression for the lazy-expiry contract)

Ordering: 0017 blocks everything else (hooks and the protocol doc quote
`--json` commands). 0018 before 0019-0021 so snippets can link to it.
0032 blocks 0033 (shared embedded snippet).
