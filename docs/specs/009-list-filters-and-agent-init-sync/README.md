# List Filters And Agent Init Sync

Status: complete; `afc-54` implemented this packet.

This packet makes the human issue list useful across projects without changing
the coordinator's local-first control-plane boundary. It also reconciles the
managed `AGENTS.md` summary written by `afctl init` with the canonical agent
protocol after atomic HANDOFF shipped.

The packet owns the CSV list-filter contract, strict CLI parsing, API/client
compatibility, and the managed instruction block. It does not add a reporting
read model, new issue statuses, permissions, remote synchronization, or a new
agent protocol.

The canonical requirements, design, task slice, and shipped evidence are in
`requirements.md`, `design.md`, `tasks.md`, and `review.md`.
