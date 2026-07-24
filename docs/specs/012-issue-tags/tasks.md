# 012 Issue Tags — Tasks

- [x] T1: Add `migrations/0007_issue_tags.sql` (`issue_tags` table + tag
  index) (R1).
- [x] T2: `internal/core/issue.go`: `Issue.Tags`, `CreateIssueRequest.Tags`,
  `AddTagRequest`, `ValidateTag` (closed charset + reserved namespaces +
  length bound), wire into `ValidateCreateIssue` (R1, R2, R3, R4).
- [x] T3: `internal/core/errors.go`: add `ErrAlreadyTagged` (R5).
- [x] T4: `internal/store/sqlite/issues_tags.go`: `populateTags`, `AddTag`,
  `RemoveTag`, reusing `insertEvent`/`isSQLiteConstraintError` (R1, R5).
- [x] T5: Wire tags into `CreateIssue` (insert rows + fold into
  `issue_created` payload) and call `populateTags` in `GetIssue`,
  `ListIssues`, `ListReadyIssues` (R1, R8).
- [x] T6: `internal/store/store.go` interface + `internal/store/sqlite/store.go`
  wrapper methods for `AddTag`/`RemoveTag` (R5).
- [x] T7: `internal/api/daemon.go` routes + `internal/api/issues.go`
  `handleAddTag`/`handleRemoveTag` (query-param DELETE) (R5).
- [x] T8: `internal/client/client.go` `AddTag`/`RemoveTag` (R6).
- [x] T9: `cmd/afctl/cmd_issue.go`: `--tag` on `issue create`; `runIssueTag`
  dispatcher (`add`/`remove`/`list`) (R4, R6).
- [x] T10: `cmd/afctl/main.go`: `Tags:` line in `printIssueDetailed` and
  `printIssueFull` (R6).
- [x] T11: `internal/mcp/server.go`: `CoordinatorClient` additions,
  `add_tag`/`remove_tag` tool cases + schemas (R7).
- [x] T12: Tests — `ValidateTag` table-driven cases; store tests (create
  with tags, get/list/ready populate tags, add emits `issue_tagged`,
  duplicate add → `already_tagged`, remove emits `issue_untagged`, remove
  nonexistent → `not_found`); API handler tests (add success/validation/
  conflict, remove success/not-found); client round-trip tests; CLI
  parsing tests for `--tag` and the `tag` dispatcher (R1–R7).
