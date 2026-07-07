# Design

## Boundaries

af-coordinator remains a local-first daemon with SQLite WAL persistence and
HTTP+JSON over a Unix socket. Aion Forge and MCP integrations are clients of the
daemon API.

Temporal integration is reference-only from the coordinator perspective:

- coordinator stores workflow ids, mirrored issue keys, PR URLs, commit SHAs,
  and close metadata
- Temporal stores workflow execution state, retries, and runner progress
- integrations never write coordinator state by editing SQLite directly

## Work ordering

1. Fix response and lookup correctness before adding more consumers.
2. Add the event watch API so consumers can react without tight polling.
3. Add external references and structured close metadata.
4. Add JSONL export for audit and bridge use cases.
5. Add the MCP wrapper last, after the daemon API contract is stable enough to
   expose to agent tooling.

## Event stream shape

The watch API should be based on the existing `events` table, but the public API
must define a stable cursor contract rather than exposing table internals. Event
payload JSON must be generated through typed marshaling, not string
construction.

## Identifier policy

Public responses should make UUID and short id fields explicit. If both forms
are useful, return both with distinct names instead of overloading `issue_id`.

## Non-goals

- no web UI
- no distributed coordinator cluster
- no direct GitHub/Gitea source of truth
- no embedded scripting runtime
- no direct SQLite mutations from helper scripts, MCP, or Aion Forge
