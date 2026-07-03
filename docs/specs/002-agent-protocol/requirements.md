# Requirements

## Problem

The daemon and CLI exist, but there is no shared working contract for
agents. Each agent session improvises: whether to claim before editing,
how often to heartbeat, how to hand off. The architecture doc names the
risk: an agent that silently skips claiming is worse than no coordinator.

Also, `afctl` output is human-formatted text only. Hooks and agent
wrappers cannot react to outcomes without parsing prose, which violates
the agent integration model in `docs/architecture-v1.md`.

## In scope

- machine-readable CLI: `--json` on every `afctl` command, typed exit
  codes, actor identity convention
- canonical protocol document `docs/agent-protocol-v1.md`
- enforcement hook snippets for Claude Code and Codex under `contrib/hooks/`
- per-repo adapter snippet template under `contrib/agents/`
- an end-to-end test proving two concurrent agents cannot hold the same
  issue

## Out of scope

- MCP server or editor integrations
- Beads data migration (packet 003)
- GitHub mirror, exports
- changes to daemon API semantics (transport contract is `docs/api-v1.md`)

## Functional requirements

- THE CLI SHALL accept a global `--json` flag on every command; with it,
  success output is a single JSON document on stdout and errors are the
  API error envelope (`{"error":{"code","message"}}`) on stderr.
- THE CLI SHALL exit with typed codes: `0` success, `1` hard failure
  (daemon unreachable, invalid usage), `2` version_conflict, `3`
  lease_held, `4` lease_expired, `5` not_found, `6` dependency_cycle.
- THE CLI SHALL read the acting identity from `AF_COORDINATOR_ACTOR` when
  an explicit `--actor`/`--holder` flag is absent, and SHALL fail with a
  clear message when a mutating command has neither.
- THE protocol document SHALL define the full session loop: pick from
  `ready`, claim with TTL, heartbeat cadence, progress notes, handoff
  note on stop, release or close at session end.
- THE protocol document SHALL define reactions to each typed exit code
  (conflict → reread and retry; lease held → pick other work; lease
  expired → re-claim before continuing).
- Hook snippets SHALL check for an active lease held by the current
  actor before file mutations and warn or block; enabling a hook SHALL
  cost one config line per agent.
- The per-repo adapter snippet SHALL reference the canonical protocol
  document and SHALL NOT restate its rules.

## Non-functional requirements

- Hook overhead: a lease check adds no more than ~50ms per invocation
  (one CLI call over the unix socket); hooks MAY cache a positive result
  briefly.
- The protocol document must be short enough that an agent can hold it
  in context: target one screen of rules, hard cap 150 lines.

## Acceptance criteria

- WHEN an agent follows only `docs/agent-protocol-v1.md` and `afctl
  --json`, THEN it can complete a full ready → claim → heartbeat → note →
  close cycle without parsing human-formatted text.
- WHEN two agents claim the same issue concurrently, THEN exactly one
  succeeds and the other observes exit code `3` (lease_held).
- WHEN an update carries a stale version, THEN the CLI exits `2` and the
  JSON error names the expected and actual versions.
- WHEN the Claude Code hook is installed and a file edit is attempted in
  a registered repo without an active lease held by the current actor,
  THEN the hook warns or blocks according to its configured mode.
