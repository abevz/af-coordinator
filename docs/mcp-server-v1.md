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
- `add_note`
- `list_notes`
- `list_issue_events`
- `close_issue`

## Design constraints

- tools are thin wrappers over `internal/client`
- daemon API remains the only write authority
- no direct SQLite reads or writes
- no second coordinator protocol beyond the MCP transport wrapper
