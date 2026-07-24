# 013 Issue Tag Filters — Review

## Outcome

Part 2 of 2 shipped: read/filter support for namespaced issue tags, plus
the convention/protocol/reference-doc updates left by part 1 (afc-91). All
tasks in `tasks.md` are complete.

## What shipped

- Store: `core.IssueListParams.Tags`; `ListIssues` and `ListReadyIssues`
  both AND multiple tags via one `EXISTS (SELECT 1 FROM issue_tags ...)`
  clause per tag. `ListReadyIssues` gained a `tags []string` parameter,
  requiring a signature update across `store.CoordinatorStore`,
  `report.Source`, `*sqlite.Store`, the API handler, the client, the MCP
  `CoordinatorClient` interface, and `cmd/afctl`; the `report.go` stats
  call site passes `nil` (stats don't filter by tag).
- API: `GET /v1/issues` and `GET /v1/issues/ready` accept a repeatable
  `tag` query parameter (AND semantics), normalized the same way as the
  existing `project`/`status`/`type` filters.
- Client: `ListIssuesWithFilters` and `ListReadyIssues` pass tag filters
  through as repeated query params.
- CLI: `issue list --tag` and `issue ready --tag` (both repeatable); usage
  strings updated.
- MCP: `list_ready_issues` accepts an optional `tags` array (`schemaField`
  gained `itemType` support for JSON-Schema `array` fields, since this was
  the first array-typed MCP tool parameter in this codebase).
- Docs: `docs/agent-protocol-v1.md` gained a `## Tags` section (namespace
  convention — `exec/*` reserved for execution routing, `area/*`/`theme/*`
  free for humans — and the no-state-in-tags invariant), synced
  byte-for-byte to `cmd/afctl/agent-protocol-v1.md`. `docs/api-v1.md`,
  `docs/mcp-server-v1.md`, `docs/api-curl-examples.md`,
  `docs/schema-v1.md`, and `README.md` updated — including the afc-91
  write-side surface (`POST`/`DELETE /v1/issues/{id}/tags`, `add_tag`/
  `remove_tag` MCP tools) that part 1 deliberately left undocumented.

## Verification

- `gofmt -w .` — clean, no diffs.
- `go build -buildvcs=false ./...` — passes.
- `go vet ./...` — clean.
- `go test -buildvcs=false ./...` and `go test -buildvcs=false -race ./...`
  — all packages pass, including new AND-semantics tests in
  `internal/store/sqlite`, repeated-query-param tests in `internal/api`,
  query-encoding tests in `internal/client`, and CLI tests in `cmd/afctl`.
- `TestEmbeddedProtocolMatchesCanonical` stays green after the protocol doc
  edit.
- Manual check against a scratch daemon (`AF_COORDINATOR_DB`/
  `AF_COORDINATOR_SOCKET` pointed at `/tmp`, never the live daemon): three
  issues (both tags, one tag, untagged); `issue ready --tag exec/auto`
  returned 2; adding `--tag area/frontend` narrowed it to 1 (AND, not OR);
  an unused tag returned zero; `issue list --tag area/frontend` matched
  correctly; `afctl protocol` output includes the new `## Tags` section.

## Requirement and design alignment

Implementation matches `requirements.md` and `design.md`. No change to
issue status, lease, or version semantics (R8); an untagged issue never
matches a `--tag` filter (no implicit wildcard), confirmed in both the
automated tests and the manual scratch-daemon check.

## Remaining work

None. Both parts of the issue-tags track (afc-91, afc-92) are complete.
