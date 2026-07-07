# 005 - Aion Forge integration readiness

Status: active.

Intent: prepare af-coordinator to act as the local tracker and control surface
for Aion Forge without turning the coordinator into the execution engine.

Roadmap target:

```text
issue (af-coordinator) -> Temporal workflow -> isolated runner -> branch/PR -> checks -> merge -> issue closed
```

This packet covers the coordinator-side readiness work behind epic `afc-24`:

- `afc-30` - dependency response identity bug
- `afc-25` - events watch API
- `afc-26` - external execution references on issues
- `afc-27` - structured close resolution references
- `afc-29` - JSONL export
- `afc-28` - MCP wrapper over the daemon API

The division of responsibility from `docs/roadmap.md` remains the contract:

- af-coordinator owns issue state, leases, notes, and audit trail
- Temporal owns workflow progress, retries, and runner state
- Aion Forge and MCP integrations talk to the daemon API, not SQLite
