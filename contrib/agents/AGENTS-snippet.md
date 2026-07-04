# af-coordinator for coding agents

This repo is coordinated by [af-coordinator](https://github.com/abevz/af-coordinator).

- **Read the protocol**: `~/github/af-coordinator/main/docs/agent-protocol-v1.md`
  (absolute path — this snippet lands in repos outside af-coordinator, so
  relative links do not survive)
- **Export your identity**: `export AF_COORDINATOR_ACTOR=<agent-name>`
- **Session cycle**: `ready → claim → heartbeat → note → close`
- **Never** edit files without an active claim.
- **Never** touch the coordinator database.
- **Never** restate specs in issue descriptions — link them.
