# 013 Issue Tag Filters

Status: complete. Part 2 of 2 (afc-92); see `review.md`. Part 1 (afc-91,
complete) shipped the tag data model and write side.

Let `issue list`/`issue ready` (and the underlying API/MCP surface) filter
by tag, so the factory can gate its ready view to a configurable subset of
issues (e.g. `exec/auto`) without a separate project. Multiple `--tag`
values are ANDed — an issue must carry every requested tag to match.

- Store: tag filtering in `ListIssues` and `ListReadyIssues` via an
  `issue_tags` join/EXISTS per tag.
- API: repeatable `tag` query param on `GET /v1/issues` and
  `GET /v1/issues/ready`.
- CLI: `issue list --tag ns/val` (repeatable), `issue ready --tag ns/val`
  (repeatable).
- Docs: reserved-vs-free namespace convention (`exec/*` reserved for
  execution routing; `area/*`, `theme/*` free for humans), the
  no-state-in-tags invariant restated in the protocol doc, and reference
  docs (`api-v1.md`, `mcp-server-v1.md`, `api-curl-examples.md`,
  `schema-v1.md`, `README.md`) updated for the afc-91 write-side surface
  this packet's predecessor left undocumented, plus this packet's
  read-side additions.

See `requirements.md`, `design.md`, `tasks.md`.
