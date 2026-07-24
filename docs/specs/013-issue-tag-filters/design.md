# 013 Issue Tag Filters — Design

## Store (R1, R2, R8)

`core.IssueListParams` gains `Tags []string`. `ListIssues`
(`internal/store/sqlite/issues.go`) appends one `EXISTS` clause per tag to
its existing `where`/`args` builder, alongside the existing
`appendIssueListInFilter` calls:

```go
for _, tag := range params.Tags {
    where = append(where, `EXISTS (SELECT 1 FROM issue_tags t WHERE t.issue_id = i.id AND t.tag = ?)`)
    args = append(args, tag)
}
```

Each tag's `EXISTS` is ANDed into the existing `WHERE ... AND ...` join
(R1's AND semantics falls out of the existing where-clause join, no special
casing needed) — this differs from `appendIssueListInFilter`'s `IN (...)`
OR-within-one-field pattern, which is why tags need their own loop instead
of reusing that helper.

`ListReadyIssues` gains a `tags []string` parameter (signature becomes
`ListReadyIssues(ctx, db, projectID, repoID string, tags []string)`) and
appends the same per-tag `EXISTS` clause (aliased to the ready query's `i`)
to its raw query string + args before the `ORDER BY`.

Tag values are used as query args (parameterized), not interpolated — no
injection risk, no extra validation needed beyond what already exists on
write.

## Interface signature change (report.Source)

`internal/store/store.go`'s `CoordinatorStore.ListReadyIssues` and
`internal/report/report.go`'s narrower `Source.ListReadyIssues` both change
to `(ctx, projectID, repoID string, tags []string) ([]core.Issue, error)`
so `*sqlite.Store` keeps structurally satisfying both. The single call site
in `report.go` (build ready-count stats) passes `nil` — stats don't filter
by tag.

## API (R3)

`handleListIssues` (`internal/api/issues.go`): read `tags, err :=
core.NormalizeIssueListValues(r.URL.Query()["tag"])` alongside the existing
project/status/type normalization, set `params.Tags = tags`. No new
validation beyond what `NormalizeIssueListValues` already does (empty CSV
element -> `validation_failed`); tag charset was already validated on
write (afc-91) — an unmatched filter tag just yields zero rows, not an
error, matching how an unknown status/type value behaves today.

`handleListReadyIssues`: read `tags := r.URL.Query()["tag"]` (repeatable,
no comma-splitting needed here since ready has no other CSV-style filter
precedent — but reuse `NormalizeIssueListValues` for consistency and to
reject empty elements the same way list does), pass through to
`st.ListReadyIssues(ctx, projectID, repoID, tags)`.

## Client (R4)

`ListIssuesWithFilters`: add `appendValues("tag", params.Tags, "")` to the
existing `appendValues` calls.

`ListReadyIssues` signature changes to `(ctx, project, repo string, tags
[]string) ([]core.Issue, error)`; each tag appended as a repeated `tag=`
query param, same construction style as the existing `project=`/`repo=`
params.

## CLI (R1)

`parseIssueListArgs`: add `"--tag"` to the allowed-flags switch, and a case
that normalizes into `params.Tags` (same shape as `--project`/`--status`/
`--type`).

`runIssueReady`: add a `--tag` case to its existing manual flag loop,
appending into a `[]string`, passed as the new `tags` argument to
`c.ListReadyIssues`.

Usage strings (`issueListUsage`, `issueReadyUsage`) gain a `--tag
<namespace/value[,..]>` line noting repeatable-AND semantics.

## MCP (R3)

`CoordinatorClient.ListReadyIssues` signature updated to match the client.
`list_ready_issues` tool case decodes an optional `tags []string` field and
passes it through; its schema entry in `tools()` gains a `tags` array
field.

## Docs (R5, R6, R7)

- `docs/agent-protocol-v1.md`: add a `## Tags` section (after "Structured
  note conventions") documenting the reserved-vs-free namespace convention
  (`exec/*` reserved for execution/routing — e.g. a factory gate tag;
  `area/*`, `theme/*`, and other non-reserved namespaces free for human
  classification) and the no-state-in-tags invariant. Add a `--tag`
  mention to the "Pick ready work" step. Copy the file byte-for-byte to
  `cmd/afctl/agent-protocol-v1.md` so `TestEmbeddedProtocolMatchesCanonical`
  stays green.
- `docs/api-v1.md`: add the afc-91 `POST`/`DELETE /v1/issues/{id}/tags`
  entries to the endpoint map and the `## Issues` prose (left undocumented
  by part 1), plus the new `tag=` query param on `GET /v1/issues` and
  `GET /v1/issues/ready`.
- `docs/mcp-server-v1.md`: add `add_tag`/`remove_tag` to the tool list (left
  undocumented by part 1); note `list_ready_issues` accepts optional `tags`.
- `docs/api-curl-examples.md`: add a "Tags" subsection under section 5
  (add/remove/list-via-get curl examples) and a `?tag=` example under the
  existing "List all issues"/"List ready issues" sections.
- `docs/schema-v1.md`: add an `### issue_tags` table section (mirroring the
  `### dependencies` section's style) with the migration 0007 DDL, plus its
  index in the `## Indexes` block.
- `README.md`: add a one-line "namespaced tags to classify/route issues" to
  the "What it does" bullet list.

## Compatibility (R8)

Additive only: new optional query param, new optional CLI flag, new
`Tags` field on an existing params struct. No existing filter, route, or
CLI flag changes behavior. An issue with zero tags never matches any
`--tag` filter (the `EXISTS` clause is false), which is the correct
default — no implicit "untagged matches everything" behavior.
