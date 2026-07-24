# 013 Issue Tag Filters — Tasks

- [x] T1: `core.IssueListParams.Tags`; `ListIssues` per-tag `EXISTS`
  clauses (R1, R2, R8).
- [x] T2: `ListReadyIssues` gains a `tags []string` param + per-tag
  `EXISTS` clause; update `store.CoordinatorStore` and
  `report.Source` interfaces + the `*sqlite.Store` wrapper + the
  `report.go` call site (pass `nil`) (R1, R2, R8).
- [x] T3: `handleListIssues`/`handleListReadyIssues` parse repeatable
  `tag` query param (R3).
- [x] T4: `ListIssuesWithFilters` and `ListReadyIssues` client methods pass
  tag filters (R4).
- [x] T5: `parseIssueListArgs` (`--tag`) and `runIssueReady` (`--tag`); usage
  strings updated (R1).
- [x] T6: `internal/mcp/server.go`: `CoordinatorClient.ListReadyIssues`
  signature + `list_ready_issues` tool `tags` field (R3).
- [x] T7: `docs/agent-protocol-v1.md` `## Tags` section (reserved
  `exec/*` vs free namespaces, no-state-in-tags) + `--tag` mention in
  ready step; sync `cmd/afctl/agent-protocol-v1.md` byte-for-byte (R5, R6).
- [x] T8: `docs/api-v1.md`: afc-91 tag mutation endpoints (previously
  undocumented) + new `tag=` query param on list/ready (R7).
- [x] T9: `docs/mcp-server-v1.md`: `add_tag`/`remove_tag` tools
  (previously undocumented) + `list_ready_issues` `tags` note (R7).
- [x] T10: `docs/api-curl-examples.md`: tags subsection + `?tag=` list/ready
  examples (R7).
- [x] T11: `docs/schema-v1.md`: `issue_tags` table + index (R7).
- [x] T12: `README.md`: tags bullet in "What it does" (R7).
- [x] T13: Tests — store tests (list/ready single-tag, multi-tag AND,
  no-match, untagged issue excluded); API handler tests (repeated `tag`
  query param); client tests (query encoding); CLI tests (`--tag` on
  `list`/`ready`, usage-on-error, help short-circuit);
  `TestEmbeddedProtocolMatchesCanonical` stays green (R1–R8).
