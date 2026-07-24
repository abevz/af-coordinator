# 012 Issue Tags — Requirements

## Problem

The operator wants a way to make a subset of coordinator issues eligible for
autonomous factory execution (the "gate") without standing up a separate
project. A separate project would duplicate every non-factory workflow
(claim, lease, dependencies, notes) for no reason other than visibility
control.

The chosen mechanism is namespaced tags on an issue, mirroring the vault's
own `type/*`, `status/*`, `source/*` convention. This packet (part 1 of 2)
builds the data model and write side; part 2 (afc-92) adds the read/filter
side that the factory will actually gate on.

A tag must never be usable to smuggle first-class issue state (status,
lease, version) through a side channel — the coordinator ledger is the
single source of truth for state (see
`docs/decisions/ADR-029-af-coordinator-as-control-plane.md`).

## Requirements

- R1: `core.Issue` gains a `Tags []string` field; a new `issue_tags` table
  stores `(issue_id, tag)` facts with a uniqueness constraint per issue.
- R2: Tags are namespaced strings `namespace/value`, validated by a closed
  charset (lowercase letters, digits, `-`, exactly one `/`), with a bounded
  maximum length.
- R3: Validation rejects tags whose namespace could masquerade as
  first-class issue state (e.g. `open`, `blocked`, `done`, `in_progress`,
  `status`, `state`) — the no-state-in-tags invariant.
- R4: `issue create` accepts a repeatable `--tag` flag; the API create body
  accepts a `tags` array.
- R5: Dedicated mutation surface for existing issues: `POST
  /v1/issues/{id}/tags` (add) and a remove endpoint, each validating the tag
  and emitting an audit event (`issue_tagged` / `issue_untagged`) into the
  existing event stream. `GET` issue responses include `Tags`.
- R6: CLI: `afctl issue tag add|remove|list <id> [--tag ns/val]`, and `issue
  get`/`get --full` print a `Tags:` line when non-empty. Client library gains
  matching methods.
- R7: MCP `get_issue` includes tags (via the shared `core.Issue` shape); MCP
  gains `add_tag`/`remove_tag` tools mirroring the API.
- R8: No change to stored issue status, lease, or version semantics; tags
  are additive and empty by default, so existing issues and the gate
  workflow (part 2) never need a backfill.
