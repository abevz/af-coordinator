# MCP Server v1

`afc-mcp` is a stdio MCP wrapper over the daemon API. It is a client of the
unix-socket HTTP API and never talks to SQLite directly.

## Launch

```bash
AF_COORDINATOR_SOCKET=~/.local/state/af-coordinator/af-coordinator.sock \
AF_COORDINATOR_ACTOR=codex-1234 \
afc-mcp
```

`AF_COORDINATOR_ACTOR` is optional for read-only tools, but mutating tools use
it as the default actor/holder/author when the request does not pass one.

## Exposed tools

- `health`
- `get_issue`
- `list_ready_issues`
- `claim_issue`
- `heartbeat_issue`
- `handoff_issue`
- `add_note`
- `list_notes`
- `list_issue_events`
- `close_issue`
- `operator_close_issue`
- `operator_reopen_issue`
- `operator_release_issue`

`claim_issue` accepts optional non-secret `session_id` correlation metadata
and returns the daemon-generated `attempt_id` and `version` with the secret
lease token. Claiming increments the issue's version as a side effect, so
callers must use this returned `version` — not one read earlier via
`get_issue` — as `expected_version` on the eventual close/handoff.
`handoff_issue` requires that active token plus a non-empty `note` beginning
`HANDOFF:`; it invokes the daemon's atomic note-and-release path.
`operator_release_issue` never accepts a lease token; it recovers an issue
stuck `in_progress` because its lease token was lost before TTL expiry,
clearing the lease and returning the issue directly to `open` without a
terminal transition.

## Design constraints

- tools are thin wrappers over `internal/client`
- daemon API remains the only write authority
- no direct SQLite reads or writes
- no second coordinator protocol beyond the MCP transport wrapper
