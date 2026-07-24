# 012 Issue Tags

Status: complete. Part 1 of 2 (afc-91); see `review.md`. Part 2 (afc-92,
blocked on this packet) adds read/filter support and the convention/protocol
docs.

Let the operator mark which coordinator issues are eligible for autonomous
factory execution, without a separate project, via namespaced tags
(`namespace/value`, mirroring the vault's `type/*`, `status/*`, `source/*`
convention).

This packet (part 1) ships the data model and write side only:

- `issue_tags` table, `core.Issue.Tags`, namespaced-tag validation with a
  closed charset and reserved-namespace guard.
- Create-time tags (`issue create --tag`) and dedicated add/remove mutation
  API + CLI + MCP surface, each emitting an audit event.
- The invariant that tags classify/route issues and never carry state —
  status, lease, and version stay first-class (see
  `docs/decisions/ADR-029-af-coordinator-as-control-plane.md`, a reference
  copy of the aion-forge ADR that frames the coordinator as the sole state
  ledger).

Explicitly out of scope here (afc-92): `--tag` filters on `issue list`/
`issue ready`, tag columns/sorting in list output, the reserved-vs-free
namespace convention document, and protocol/reference-doc updates.

See `requirements.md`, `design.md`, `tasks.md`.
