This repo is coordinated by [af-coordinator](https://github.com/abevz/af-coordinator).

- **Read the protocol**: `afctl protocol` (or `~/github/af-coordinator/main/docs/agent-protocol-v1.md`)
- **Export your identity**: `export AF_COORDINATOR_ACTOR=<agent-name>`
- **Session cycle**: `ready → claim → heartbeat → note → close`
- **Never** edit files without an active claim.
- **Never** touch the coordinator database.
- **Never** restate specs in issue descriptions — link them.
- **Never** close an issue without a note (`--note`) — the audit trail is for whoever comes after you.
