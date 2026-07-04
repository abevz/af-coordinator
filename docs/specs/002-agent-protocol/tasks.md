# Tasks

Numbering continues the global AFC-SDD sequence.

- [x] AFC-SDD-0017 Add `--json` output and typed exit codes to afctl
  - global flag, JSON success on stdout, error envelope passthrough on
    stderr, exit-code table from design.md
  - actor resolution: flag > `AF_COORDINATOR_ACTOR` > error for mutations
  - table-driven tests for exit-code mapping (per AGENTS.md testing policy)
- [ ] AFC-SDD-0018 Write `docs/agent-protocol-v1.md`
  - session loop, exit-code reaction table, scope rules, prohibitions
  - hard cap 150 lines; every quoted command must actually work
- [ ] AFC-SDD-0019 Claude Code hook snippet (`contrib/hooks/claude-code/`)
  - `check-lease.sh` with warn|block modes, 60s positive cache,
    fail-open-with-reason when daemon is unreachable in warn mode
  - `settings-snippet.json` one-line registration
- [ ] AFC-SDD-0020 Codex hook snippet (`contrib/hooks/codex/`)
  - same check script, Codex registration format
- [ ] AFC-SDD-0021 Per-repo adapter snippet (`contrib/agents/AGENTS-snippet.md`)
  - ≤15 lines, references the protocol doc, never restates it
- [ ] AFC-SDD-0022 End-to-end concurrency test
  - two concurrent claims on one issue: exactly one success, one
    `lease_held` (exit 3 via CLI path or 409 via API path)

Ordering: 0017 blocks everything else (hooks and the protocol doc quote
`--json` commands). 0018 before 0019-0021 so snippets can link to it.
