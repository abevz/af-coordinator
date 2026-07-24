# 012 Issue Tags — Review

## Outcome

Part 1 of 2 shipped: the data model and write side for namespaced issue
tags. All tasks in `tasks.md` are complete.

## What shipped

- `migrations/0007_issue_tags.sql`: `issue_tags(issue_id, tag, created_at)`
  with a composite primary key and a `tag` index for afc-92's future filter.
- `core.Issue.Tags`, `core.CreateIssueRequest.Tags`, `core.AddTagRequest`,
  `core.ValidateTag` (closed charset, 64-char bound, reserved-namespace
  rejection), `core.ErrAlreadyTagged`.
- Store: `populateTags` (batch-load, mirrors `populateDependencies`),
  `AddTag`/`RemoveTag` (both emit `issue_tagged`/`issue_untagged` audit
  events via the existing `insertEvent` helper), tags wired into
  `CreateIssue`, `GetIssue`, `ListIssues`, `ListReadyIssues`.
- API: `POST /v1/issues/{id}/tags`, `DELETE /v1/issues/{id}/tags` (tag and
  actor as query params — see the routing decision below).
- Client: `AddTag`/`RemoveTag`.
- CLI: `issue create --tag` (repeatable), `issue tag add|remove|list`,
  `Tags:` line in `issue get`/`get --full`.
- MCP: `add_tag`/`remove_tag` tools; `get_issue` surfaces tags automatically
  via the shared `core.Issue` shape.

## Routing decision

`DELETE /v1/issues/{id}/tags` takes `tag` (and `actor`) as query
parameters rather than a `{tag}` path segment, diverging deliberately from
the issue text's literal phrasing. The tag value contains `/`
(`namespace/value`), which is not a safe single path segment across all
HTTP tooling (proxies normalizing `%2F`, curl quoting). The existing
`DELETE /v1/issues/{id}/links` endpoint already established the
query-param-for-DELETE precedent for a similarly non-path-safe identifier;
tags follow the same pattern. Confirmed with the user before implementing.

## Verification

- `gofmt -w .` — clean, no diffs.
- `go build -buildvcs=false ./...` — passes.
- `go test -buildvcs=false ./...` and `go test -buildvcs=false -race ./...`
  — all packages pass, including new tests in `internal/core`,
  `internal/store/sqlite`, `internal/api`, `internal/client`,
  `internal/mcp`, and `cmd/afctl`.
- Manual check against a scratch daemon (`AF_COORDINATOR_DB`/
  `AF_COORDINATOR_SOCKET` pointed at `/tmp`, never the live daemon):
  created an issue with `--tag area/frontend --tag theme/dark`; `issue get`
  and `issue get --full` printed `Tags:`; `issue tag add/list/remove`
  round-tripped; adding a reserved-namespace tag (`status/open`) was
  rejected with `validation_failed`; adding a duplicate tag was rejected
  with `already_tagged`; removing a nonexistent tag returned `not_found`;
  `issue events list` showed `issue_tagged`/`issue_untagged` with the tag
  in the payload, and `issue_created`'s payload included the create-time
  `tags` array; `--json issue get` included `"tags"`.
- A cosmetic double-prefix bug (`validation_failed: validation_failed: ...`)
  was found during manual verification and fixed: `core.ValidateTag` no
  longer embeds its own `validation_failed:` prefix — callers (`AddTag` in
  the store, and `ValidateCreateIssue`) each apply exactly one wrapping
  appropriate to their context.

## Requirement and design alignment

Implementation matches `requirements.md` and `design.md`. No change to
issue status, lease, or version semantics (R8); every existing issue has
zero tags until explicitly tagged.

## Remaining work

None in this packet. `--tag` filters on `issue list`/`issue ready`, the
reserved-vs-free namespace convention document, and protocol/reference-doc
updates are afc-92 (part 2), already filed and blocked on this packet.
