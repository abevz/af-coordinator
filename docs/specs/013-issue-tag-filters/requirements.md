# 013 Issue Tag Filters — Requirements

## Problem

afc-91 (part 1, `docs/specs/012-issue-tags/`) added namespaced tags as a
write-only surface: an issue can be tagged, but nothing can filter by tag
yet. The operator's actual goal — gate which issues the autonomous factory
picks up, via a configurable tag, without a separate project — needs the
read side: `issue ready`/`issue list` must be able to filter to issues
carrying a given tag (or set of tags).

Because a gate tag and a human-facing classification tag (e.g. `area/*`)
must never be confused, this packet also documents (not enforces in code
beyond afc-91's reserved-state-namespace check) a convention: `exec/*` is
reserved for execution routing, `area/*`/`theme/*` are free for humans.

## Requirements

- R1: `issue ready` and `issue list` accept a repeatable `--tag` flag.
  Multiple `--tag` values are ANDed: an issue must carry every requested
  tag to appear in the result.
- R2: The store computes tag filtering in SQL (`ListIssues`,
  `ListReadyIssues`), not by fetching-then-filtering in application code.
- R3: `GET /v1/issues` and `GET /v1/issues/ready` accept a repeatable `tag`
  query parameter with the same AND semantics.
- R4: Client library methods pass tag filters through to both endpoints.
- R5: A documented convention (protocol doc + reference docs) states:
  `exec/*` is reserved for execution/routing tags (e.g. the factory gate);
  `area/*`, `theme/*`, and other non-reserved namespaces are free for
  human classification; tags never carry state (status/lease/version stay
  first-class) — restating afc-91's invariant in the operator-facing docs.
- R6: `docs/agent-protocol-v1.md` is updated and its embedded copy
  (`cmd/afctl/agent-protocol-v1.md`) stays byte-identical, so
  `TestEmbeddedProtocolMatchesCanonical` stays green.
- R7: Reference docs are updated: `docs/api-v1.md` (routes + query
  params, including afc-91's `POST`/`DELETE /v1/issues/{id}/tags` which
  part 1 left undocumented), `docs/mcp-server-v1.md` (tool list, including
  afc-91's `add_tag`/`remove_tag`), `docs/api-curl-examples.md`,
  `docs/schema-v1.md` (`issue_tags` table), `README.md`.
- R8: No change to stored issue status, lease, or version semantics; an
  issue with no tags never matches a `--tag` filter (no implicit wildcard).
